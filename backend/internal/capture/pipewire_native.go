package capture

/*
#cgo pkg-config: libpipewire-0.3
#cgo CFLAGS: -Wno-deprecated-declarations
#include "pipewire_native.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// ErrNoNewFrame reports that the stream has published nothing since the
// previous poll. A screen with no damage produces no frames, so this is the
// normal idle case rather than a failure.
var ErrNoNewFrame = errors.New("no new frame since last poll")

// ErrStreamFailed reports that the PipeWire stream has stopped producing for
// good: it errored, left the streaming state and never came back, or kept
// sending buffers none of which could be used. Revoking a screen share shows
// up as the second of these.
var ErrStreamFailed = errors.New("pipewire stream failed")

// ErrAwaitingFirstFrame reports that the stream is up but the compositor has
// not sent its first buffer. Distinct from a stream that never connected,
// because a screen with no damage can take a while to produce one.
var ErrAwaitingFirstFrame = errors.New("awaiting the first frame")

// streamStallDeadline is how long a stream may sit outside the streaming state
// before it counts as dead. Crossing it now ends the sync session with no
// retry, so it is set well past the renegotiation it has to tolerate: a
// resolution change clears it in under a second, but a monitor hotplug or a
// display reconfiguration on a loaded machine is the case that matters. The
// cost of the margin is only that the lights hold their last colour for that
// much longer before sync gives up.
const streamStallDeadline = 5 * time.Second

// streamStalled reports whether a stream producing no new frames counts as
// dead rather than idle. Frames only arrive on damage, so a static screen
// produces none while perfectly healthy; what separates the two is that a
// healthy stream stays in the streaming state.
//
// The deadline runs from the moment the stream left that state, not from the
// last frame: on a still screen the last frame is already arbitrarily old, so
// measuring from it would give a renegotiation no grace at all.
func streamStalled(leftStreamingAt, now time.Time) bool {
	if leftStreamingAt.IsZero() {
		return false
	}
	return now.Sub(leftStreamingAt) >= streamStallDeadline
}

// streamHealth tracks whether a stream that is producing no frames has also
// left the streaming state, which is the only thing separating a dead producer
// from a screen with nothing to send.
type streamHealth struct {
	leftStreamingAt time.Time
}

// observe folds one poll into the fault state.
func (h *streamHealth) observe(streaming bool, now time.Time) {
	h.leftStreamingAt = faultSince(streaming, h.leftStreamingAt, now)
}

// staleVerdict decides what being handed the same frame again means. A screen
// with nothing to send produces no frames while perfectly healthy, so only a
// stream that has also left the streaming state, or a producer whose every
// recent buffer was unusable, counts as dead.
func (h *streamHealth) staleVerdict(now time.Time, run unusableRun) error {
	if streamStalled(h.leftStreamingAt, now) {
		return fmt.Errorf("%w: no frame in the %v since the stream stopped streaming",
			ErrStreamFailed, now.Sub(h.leftStreamingAt).Round(time.Millisecond))
	}
	if err := run.failure(); err != nil {
		return err
	}
	return ErrNoNewFrame
}

// unusableRunLimit and unusableRunSpan are how much evidence it takes to call
// a producer broken, and both have to be met: the count alone would let a
// renegotiation's burst of drops reach a verdict, and the span alone would let
// two stray drops a few seconds apart on a still screen.
const (
	unusableRunLimit = 10
	unusableRunSpan  = 2 * time.Second
)

// unusableRun is the current run of consecutive buffers the producer sent
// that could not be used.
type unusableRun struct {
	count  uint64
	span   time.Duration // from the first of them to the last
	reason string        // why the latest one was dropped
}

// failure reports the run as a failed stream once every buffer the producer
// sent recently was unusable, and is nil until then. It is judged by what the
// producer sent rather than by the clock, because a still screen sends nothing
// at all, and the span runs to the last drop rather than to now, so an idle
// screen never grows a burst into a verdict. The C side splits the run on a
// long gap between drops, so neither does a stray drop long after one.
func (r unusableRun) failure() error {
	if r.count < unusableRunLimit || r.span < unusableRunSpan {
		return nil
	}
	return fmt.Errorf("%w: the last %d buffers, over %v, were all unusable: %s",
		ErrStreamFailed, r.count, r.span.Round(time.Millisecond), r.reason)
}

// faultSince tracks how long a fault has held: the zero time while the stream
// is healthy in that respect, otherwise the first time it was not.
//
// Returning to the streaming state clears the clock even though no frame has
// arrived yet. That is deliberate: a renegotiation on a still screen produces
// no damage and so no frame, and requiring one to certify recovery would kill
// exactly that healthy session. The cost is that a producer flapping in and
// out of streaming faster than the deadline goes unnoticed.
func faultSince(healthy bool, since, now time.Time) time.Time {
	if healthy {
		return time.Time{}
	}
	if since.IsZero() {
		return now
	}
	return since
}

// NativePipeWireCapture handles native PipeWire capture via CGo
type NativePipeWireCapture struct {
	userData *C.struct_user_data
	running  atomic.Bool

	// Sequence of the last frame handed to the caller, compared against the
	// stream's current sequence to spot a slot that has already been served.
	// Guarded by shutdownMutex along with every other GetFrame access.
	lastSeq uint64

	// Guarded by shutdownMutex along with every other GetFrame access.
	health streamHealth

	// shutdownMutex closes the teardown TOCTOU: Stop takes this lock before
	// touching userData, so it can never free the C-side state out from
	// under a GetFrame call that already passed the running check and is
	// using userData. It is a plain Mutex, not an RWMutex: the C-side
	// double buffer (see struct user_data in pipewire_native.c) only
	// tolerates a single outstanding pw_get_frame call at a time, so
	// GetFrame must serialize against itself too, not just against Stop.
	shutdownMutex sync.Mutex
}

// NewNativePipeWireCapture creates a new native PipeWire capture instance
func NewNativePipeWireCapture(nodeID uint32) (*NativePipeWireCapture, error) {
	// Connect to PipeWire node
	userData := C.pw_stream_connect_to_node(C.uint32_t(nodeID))
	if userData == nil {
		return nil, fmt.Errorf("failed to connect to PipeWire node %d", nodeID)
	}

	capture := &NativePipeWireCapture{
		userData: userData,
	}

	return capture, nil
}

// Start begins the capture loop in a separate goroutine
func (npc *NativePipeWireCapture) Start() error {
	// Same lock as GetFrame and Stop: userData is guarded by it, and the
	// running check below is only a check-then-set under it.
	npc.shutdownMutex.Lock()
	defer npc.shutdownMutex.Unlock()

	if npc.userData == nil {
		return fmt.Errorf("capture not initialized")
	}
	if npc.running.Load() {
		return fmt.Errorf("capture already running")
	}

	// Start the PipeWire thread loop (non-blocking)
	result := C.pw_start_loop(npc.userData)
	if result < 0 {
		return fmt.Errorf("failed to start PipeWire thread loop")
	}

	npc.running.Store(true)
	return nil
}

// GetFrame retrieves the latest captured frame. The returned *image.RGBA
// comes from the shared image buffer pool (see GetImageBuffer in
// capture.go): it belongs to the caller, not to this NativePipeWireCapture,
// so the caller is free to hand it to another goroutine and should return it
// to the pool with PutImageBuffer once done. GetFrame must not be called
// concurrently from more than one goroutine (shutdownMutex enforces this).
func (npc *NativePipeWireCapture) GetFrame() (*image.RGBA, error) {
	// Also closes the teardown TOCTOU: Stop cannot free userData while a
	// GetFrame call is between this check and its use of userData.
	npc.shutdownMutex.Lock()
	defer npc.shutdownMutex.Unlock()

	if npc.userData == nil || !npc.running.Load() {
		return nil, fmt.Errorf("capture not initialized or stopped")
	}

	// Get frame data from C
	var data *C.uint8_t
	var width, height, stride C.int
	var format C.uint32_t
	var status C.struct_frame_status

	result := C.pw_get_frame(npc.userData, &data, &width, &height, &stride, &format, &status)
	if result < 0 {
		return nil, ErrStreamFailed
	}

	now := time.Now()
	npc.health.observe(status.streaming != 0, now)

	if result == 0 {
		// Named here rather than left to the first-frame deadline, which
		// would end capture without saying why.
		if err := unusableRunFrom(&status).failure(); err != nil {
			return nil, err
		}
		if status.streaming != 0 {
			return nil, ErrAwaitingFirstFrame
		}
		return nil, fmt.Errorf("no frame available yet")
	}

	// Converting the same pixels again would cost a full frame's work and
	// publish a buffer identical to the one already installed.
	if uint64(status.seq) == npc.lastSeq {
		return nil, npc.health.staleVerdict(now, unusableRunFrom(&status))
	}

	// Recorded as consumed only once it converts. A slot that never converts
	// has not produced a frame, and counting it as one would clear the very
	// clocks that are meant to notice capture has stopped working.
	frame, err := npc.convertToRGBA(data, int(width), int(height), int(stride), uint32(format))
	if err != nil {
		return nil, err
	}

	npc.lastSeq = uint64(status.seq)
	return frame, nil
}

func unusableRunFrom(status *C.struct_frame_status) unusableRun {
	if status.unusable_run == 0 {
		return unusableRun{}
	}
	return unusableRun{
		count:  uint64(status.unusable_run),
		span:   time.Duration(status.unusable_span_ns),
		reason: C.GoString(status.unusable_reason),
	}
}

// Format constants from spa/param/video/format.h
const (
	SPA_VIDEO_FORMAT_BGRx = 8
	SPA_VIDEO_FORMAT_RGBx = 7
	SPA_VIDEO_FORMAT_BGRA = 12
	SPA_VIDEO_FORMAT_RGBA = 11
	SPA_VIDEO_FORMAT_BGR  = 16
	SPA_VIDEO_FORMAT_RGB  = 15
	SPA_VIDEO_FORMAT_xBGR = 10
	SPA_VIDEO_FORMAT_xRGB = 9
)

// sourceBytesPerPixel reports how many source bytes convertToRGBA reads per
// pixel for a SPA video format, or 0 for formats it cannot convert.
func sourceBytesPerPixel(format uint32) int {
	switch format {
	case SPA_VIDEO_FORMAT_BGRx, SPA_VIDEO_FORMAT_BGRA,
		SPA_VIDEO_FORMAT_RGBx, SPA_VIDEO_FORMAT_RGBA,
		SPA_VIDEO_FORMAT_xBGR, SPA_VIDEO_FORMAT_xRGB:
		return 4
	case SPA_VIDEO_FORMAT_BGR, SPA_VIDEO_FORMAT_RGB:
		return 3
	}
	return 0
}

// validateFrameGeometry rejects frame dimensions that convertToRGBA's row
// indexing would read out of bounds. Its source is C memory whose extent the
// converter is told rather than can check, and the loops index it as
// stride*y + x*bytesPerPixel, so a frame reporting a stride too short for its
// own width (chunk->stride is zero on a frame the producer published without
// data) would walk straight off the end of the mapping. That read is not
// recoverable: it panics on nativeFrameReaderLoop, which nothing recovers
// from, taking the whole daemon down.
func validateFrameGeometry(width, height, stride, bytesPerPixel int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid frame dimensions: %dx%d", width, height)
	}
	if stride < width*bytesPerPixel {
		return fmt.Errorf("frame stride %d is too short for %d pixels at %d bytes each", stride, width, bytesPerPixel)
	}
	return nil
}

// convertToRGBA converts PipeWire buffer to image.RGBA
func (npc *NativePipeWireCapture) convertToRGBA(data *C.uint8_t, width, height, stride int, format uint32) (*image.RGBA, error) {
	bytesPerPixel := sourceBytesPerPixel(format)
	if bytesPerPixel == 0 {
		return nil, fmt.Errorf("unsupported video format: %d", format)
	}

	if err := validateFrameGeometry(width, height, stride, bytesPerPixel); err != nil {
		return nil, err
	}

	// Pull the destination from the shared pool instead of reusing one
	// persistent buffer: each call gets an image nothing else references
	// yet, so this conversion never has to coordinate with whatever a
	// previous call's buffer is being read by elsewhere.
	bounds := image.Rect(0, 0, width, height)
	img := GetImageBuffer(bounds)

	// Convert C buffer to Go slice (no copy, just reference)
	bufferSize := stride * height
	srcSlice := unsafe.Slice((*byte)(unsafe.Pointer(data)), bufferSize)

	// Convert pixel format
	switch format {
	case SPA_VIDEO_FORMAT_BGRx, SPA_VIDEO_FORMAT_BGRA:
		// BGRx/BGRA -> RGBA: swap R and B (4 bytes per pixel)
		for y := 0; y < height; y++ {
			srcOffset := y * stride
			dstOffset := y * img.Stride
			for x := 0; x < width; x++ {
				b := srcSlice[srcOffset+x*4+0]
				g := srcSlice[srcOffset+x*4+1]
				r := srcSlice[srcOffset+x*4+2]

				img.Pix[dstOffset+x*4+0] = r
				img.Pix[dstOffset+x*4+1] = g
				img.Pix[dstOffset+x*4+2] = b
				img.Pix[dstOffset+x*4+3] = 255
			}
		}

	case SPA_VIDEO_FORMAT_xBGR:
		// xBGR -> RGBA: swap R and B, alpha is first byte (4 bytes per pixel)
		for y := 0; y < height; y++ {
			srcOffset := y * stride
			dstOffset := y * img.Stride
			for x := 0; x < width; x++ {
				// xBGR format: X B G R
				r := srcSlice[srcOffset+x*4+3]
				g := srcSlice[srcOffset+x*4+2]
				b := srcSlice[srcOffset+x*4+1]

				img.Pix[dstOffset+x*4+0] = r
				img.Pix[dstOffset+x*4+1] = g
				img.Pix[dstOffset+x*4+2] = b
				img.Pix[dstOffset+x*4+3] = 255
			}
		}

	case SPA_VIDEO_FORMAT_RGBx, SPA_VIDEO_FORMAT_RGBA:
		// RGBx/RGBA -> RGBA: direct copy (4 bytes per pixel)
		for y := 0; y < height; y++ {
			srcOffset := y * stride
			dstOffset := y * img.Stride
			for x := 0; x < width; x++ {
				img.Pix[dstOffset+x*4+0] = srcSlice[srcOffset+x*4+0]
				img.Pix[dstOffset+x*4+1] = srcSlice[srcOffset+x*4+1]
				img.Pix[dstOffset+x*4+2] = srcSlice[srcOffset+x*4+2]
				img.Pix[dstOffset+x*4+3] = 255
			}
		}

	case SPA_VIDEO_FORMAT_xRGB:
		// xRGB -> RGBA: alpha is first byte (4 bytes per pixel)
		for y := 0; y < height; y++ {
			srcOffset := y * stride
			dstOffset := y * img.Stride
			for x := 0; x < width; x++ {
				// xRGB format: X R G B
				r := srcSlice[srcOffset+x*4+1]
				g := srcSlice[srcOffset+x*4+2]
				b := srcSlice[srcOffset+x*4+3]

				img.Pix[dstOffset+x*4+0] = r
				img.Pix[dstOffset+x*4+1] = g
				img.Pix[dstOffset+x*4+2] = b
				img.Pix[dstOffset+x*4+3] = 255
			}
		}

	case SPA_VIDEO_FORMAT_BGR:
		// BGR -> RGBA (3 bytes per pixel)
		for y := 0; y < height; y++ {
			srcOffset := y * stride
			dstOffset := y * img.Stride
			for x := 0; x < width; x++ {
				b := srcSlice[srcOffset+x*3+0]
				g := srcSlice[srcOffset+x*3+1]
				r := srcSlice[srcOffset+x*3+2]

				img.Pix[dstOffset+x*4+0] = r
				img.Pix[dstOffset+x*4+1] = g
				img.Pix[dstOffset+x*4+2] = b
				img.Pix[dstOffset+x*4+3] = 255
			}
		}

	case SPA_VIDEO_FORMAT_RGB:
		// RGB -> RGBA (3 bytes per pixel)
		for y := 0; y < height; y++ {
			srcOffset := y * stride
			dstOffset := y * img.Stride
			for x := 0; x < width; x++ {
				r := srcSlice[srcOffset+x*3+0]
				g := srcSlice[srcOffset+x*3+1]
				b := srcSlice[srcOffset+x*3+2]

				img.Pix[dstOffset+x*4+0] = r
				img.Pix[dstOffset+x*4+1] = g
				img.Pix[dstOffset+x*4+2] = b
				img.Pix[dstOffset+x*4+3] = 255
			}
		}

	default:
		// Unreachable while this switch and sourceBytesPerPixel agree; kept
		// so that adding a format to one but not the other cannot return an
		// unwritten image. img came from the pool, so give it back.
		PutImageBuffer(img)
		return nil, fmt.Errorf("unsupported video format: %d", format)
	}

	return img, nil
}

// Stop stops the capture
func (npc *NativePipeWireCapture) Stop() {
	npc.running.Store(false)

	// Waits for any GetFrame call already past the running check to finish
	// (it holds this same lock for its whole body) before userData is freed
	// below, and blocks new GetFrame calls from starting until this
	// function returns.
	npc.shutdownMutex.Lock()
	defer npc.shutdownMutex.Unlock()

	// Keyed on userData rather than on running: NewNativePipeWireCapture
	// connects the stream and spawns the loop thread, so a Start that fails
	// after that leaves all of it allocated with running still false. Gating
	// teardown on running leaked the stream, the loop and both frame slots on
	// exactly that path, and again on every retry.
	if npc.userData == nil {
		return
	}

	C.pw_stop_loop(npc.userData)
	C.pw_cleanup(npc.userData)
	npc.userData = nil
}

// IsRunning returns whether capture is running
func (npc *NativePipeWireCapture) IsRunning() bool {
	return npc.running.Load()
}
