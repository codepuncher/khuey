package capture

/*
#cgo pkg-config: libpipewire-0.3
#cgo CFLAGS: -Wno-deprecated-declarations
#include <pipewire/pipewire.h>
#include <spa/param/video/format-utils.h>
#include <spa/param/video/type-info.h>
#include <spa/debug/types.h>
#include <spa/debug/format.h>
#include <stdlib.h>
#include <string.h>

// User data passed to callbacks
struct user_data {
	struct pw_thread_loop *loop;
	struct pw_stream *stream;

	// Frame data
	uint8_t *frame_data;
	int frame_width;
	int frame_height;
	int frame_stride;
	uint32_t frame_format;
	int frame_ready;
	int process_count;  // Debug counter

	// For Go callbacks
	void *go_context;
};

// Forward declaration of the on_process callback
void on_process_cb(void *userdata);

// Stream event handlers
static void on_stream_state_changed(void *data, enum pw_stream_state old,
				    enum pw_stream_state state, const char *error)
{
	struct user_data *ud = data;
	printf("[PipeWire] Stream state changed: %s -> %s\n",
	       pw_stream_state_as_string(old),
	       pw_stream_state_as_string(state));

	if (state == PW_STREAM_STATE_ERROR) {
		printf("[PipeWire] Stream error: %s\n", error);
		pw_thread_loop_signal(ud->loop, false);
	}
}

static void on_stream_param_changed(void *data, uint32_t id, const struct spa_pod *param)
{
	struct user_data *ud = data;

	if (param == NULL || id != SPA_PARAM_Format)
		return;

	// Parse video format
	struct spa_video_info_raw format;
	if (spa_format_video_raw_parse(param, &format) < 0) {
		printf("[PipeWire] Failed to parse video format\n");
		return;
	}

	// Store format info
	ud->frame_width = format.size.width;
	ud->frame_height = format.size.height;
	ud->frame_format = format.format;

	printf("[PipeWire] Video format: %dx%d, format=%s\n",
	       format.size.width, format.size.height,
	       spa_debug_type_find_name(spa_type_video_format, format.format));
}

static void on_stream_process(void *data)
{
	struct user_data *ud = data;
	struct pw_buffer *b;
	struct spa_buffer *buf;

	ud->process_count++;

	// Dequeue buffer
	b = pw_stream_dequeue_buffer(ud->stream);
	if (b == NULL) {
		return;
	}

	buf = b->buffer;

	// Get video data
	if (buf->datas[0].data != NULL) {
		uint8_t *src = buf->datas[0].data;
		int stride = buf->datas[0].chunk->stride;
		int size = buf->datas[0].chunk->size;

		// Calculate expected size
		int height = ud->frame_height;
		int expected_size = stride * height;

		// Allocate/reallocate frame buffer if needed
		if (ud->frame_data == NULL || ud->frame_stride != stride) {
			if (ud->frame_data != NULL) {
				free(ud->frame_data);
			}
			ud->frame_data = malloc(expected_size);
			ud->frame_stride = stride;
		}

		// Copy frame data (thread-safe)
		if (ud->frame_data != NULL && size > 0) {
			memcpy(ud->frame_data, src, size < expected_size ? size : expected_size);
			ud->frame_ready = 1;
		}
	}

	// Return buffer to PipeWire
	pw_stream_queue_buffer(ud->stream, b);
}

static const struct pw_stream_events stream_events = {
	PW_VERSION_STREAM_EVENTS,
	.state_changed = on_stream_state_changed,
	.param_changed = on_stream_param_changed,
	.process = on_stream_process,
};

// Helper function to create and connect stream
struct user_data* pw_stream_connect_to_node(uint32_t node_id) {
	struct pw_thread_loop *loop;
	struct pw_stream *stream;
	struct user_data *ud;
	const struct spa_pod *params[1];
	uint8_t buffer[1024];
	struct spa_pod_builder b = SPA_POD_BUILDER_INIT(buffer, sizeof(buffer));

	// Initialize PipeWire
	pw_init(NULL, NULL);

	// Create thread loop (for multi-threaded applications)
	loop = pw_thread_loop_new("khuey-pipewire", NULL);
	if (!loop) {
		printf("[PipeWire] Failed to create thread loop\n");
		return NULL;
	}

	// Allocate user data
	ud = calloc(1, sizeof(struct user_data));
	if (!ud) {
		pw_thread_loop_destroy(loop);
		return NULL;
	}
	ud->loop = loop;

	// Lock the loop before creating stream
	pw_thread_loop_lock(loop);

	// Create stream
	stream = pw_stream_new_simple(
		pw_thread_loop_get_loop(loop),
		"khuey-screen-capture",
		pw_properties_new(
			PW_KEY_MEDIA_TYPE, "Video",
			PW_KEY_MEDIA_CATEGORY, "Capture",
			PW_KEY_MEDIA_ROLE, "Screen",
			NULL),
		&stream_events,
		ud);

	if (!stream) {
		printf("[PipeWire] Failed to create stream\n");
		pw_thread_loop_unlock(loop);
		free(ud);
		pw_thread_loop_destroy(loop);
		return NULL;
	}
	ud->stream = stream;

	// Build format parameters - accept any video format
	params[0] = spa_pod_builder_add_object(&b,
		SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat,
		SPA_FORMAT_mediaType, SPA_POD_Id(SPA_MEDIA_TYPE_video),
		SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
		SPA_FORMAT_VIDEO_format, SPA_POD_CHOICE_ENUM_Id(5,
			SPA_VIDEO_FORMAT_BGRx,
			SPA_VIDEO_FORMAT_RGBx,
			SPA_VIDEO_FORMAT_BGRA,
			SPA_VIDEO_FORMAT_RGBA,
			SPA_VIDEO_FORMAT_BGR),
		0);

	// Connect stream to specific node
	if (pw_stream_connect(stream,
			      PW_DIRECTION_INPUT,
			      node_id,
			      PW_STREAM_FLAG_AUTOCONNECT |
			      PW_STREAM_FLAG_MAP_BUFFERS,
			      params, 1) < 0) {
		printf("[PipeWire] Failed to connect stream to node %u\n", node_id);
		pw_stream_destroy(stream);
		pw_thread_loop_unlock(loop);
		free(ud);
		pw_thread_loop_destroy(loop);
		return NULL;
	}

	// Unlock before starting the loop
	pw_thread_loop_unlock(loop);

	printf("[PipeWire] Stream connected to node %u\n", node_id);
	return ud;
}

// Start the thread loop (non-blocking)
int pw_start_loop(struct user_data *ud) {
	if (ud && ud->loop) {
		return pw_thread_loop_start(ud->loop);
	}
	return -1;
}

// Stop the thread loop
void pw_stop_loop(struct user_data *ud) {
	if (ud && ud->loop) {
		pw_thread_loop_stop(ud->loop);
	}
}

// Cleanup
void pw_cleanup(struct user_data *ud) {
	if (ud) {
		if (ud->loop) {
			pw_thread_loop_lock(ud->loop);
		}
		if (ud->stream) {
			pw_stream_destroy(ud->stream);
		}
		if (ud->loop) {
			pw_thread_loop_unlock(ud->loop);
			pw_thread_loop_destroy(ud->loop);
		}
		if (ud->frame_data) {
			free(ud->frame_data);
		}
		free(ud);
	}
	pw_deinit();
}

// Get frame data (returns 0 if no frame ready, 1 if frame available)
int pw_get_frame(struct user_data *ud, uint8_t **data, int *width, int *height,
                 int *stride, uint32_t *format) {
	if (ud == NULL) {
		return 0;
	}

	// Lock the thread loop for thread-safe access
	pw_thread_loop_lock(ud->loop);

	if (!ud->frame_ready) {
		pw_thread_loop_unlock(ud->loop);
		return 0;
	}

	*data = ud->frame_data;
	*width = ud->frame_width;
	*height = ud->frame_height;
	*stride = ud->frame_stride;
	*format = ud->frame_format;

	pw_thread_loop_unlock(ud->loop);

	return 1;
}
*/
import "C"
import (
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// NativePipeWireCapture handles native PipeWire capture via CGo
type NativePipeWireCapture struct {
	userData    *C.struct_user_data
	stopChan    chan struct{}
	running     atomic.Bool
	latestFrame *image.RGBA
	frameMutex  sync.RWMutex
	rgbaBuffer  *image.RGBA // Reused RGBA buffer to avoid allocations every frame
	bufferMutex sync.Mutex  // Protects rgbaBuffer during conversion
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
		stopChan: make(chan struct{}),
	}

	return capture, nil
}

// Start begins the capture loop in a separate goroutine
func (npc *NativePipeWireCapture) Start() error {
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

// GetFrame retrieves the latest captured frame
func (npc *NativePipeWireCapture) GetFrame() (*image.RGBA, error) {
	if npc.userData == nil || !npc.running.Load() {
		return nil, fmt.Errorf("capture not initialized or stopped")
	}

	// Get frame data from C
	var data *C.uint8_t
	var width, height, stride C.int
	var format C.uint32_t

	result := C.pw_get_frame(npc.userData, &data, &width, &height, &stride, &format)
	if result == 0 {
		return nil, fmt.Errorf("no frame available yet")
	}

	// Convert C data to Go image
	return npc.convertToRGBA(data, int(width), int(height), int(stride), uint32(format))
}

// convertToRGBA converts PipeWire buffer to image.RGBA
func (npc *NativePipeWireCapture) convertToRGBA(data *C.uint8_t, width, height, stride int, format uint32) (*image.RGBA, error) {
	// Reuse RGBA buffer to eliminate per-frame allocations
	npc.bufferMutex.Lock()
	bounds := image.Rect(0, 0, width, height)
	if npc.rgbaBuffer == nil || npc.rgbaBuffer.Bounds() != bounds {
		// First frame or resolution changed - allocate new buffer
		npc.rgbaBuffer = image.NewRGBA(bounds)
	}
	img := npc.rgbaBuffer
	npc.bufferMutex.Unlock()

	// Convert C buffer to Go slice (no copy, just reference)
	bufferSize := stride * height
	srcSlice := unsafe.Slice((*byte)(unsafe.Pointer(data)), bufferSize)

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
		return nil, fmt.Errorf("unsupported video format: %d", format)
	}

	// Store latest frame
	npc.frameMutex.Lock()
	npc.latestFrame = img
	npc.frameMutex.Unlock()

	return img, nil
}

// Stop stops the capture
func (npc *NativePipeWireCapture) Stop() {
	if !npc.running.CompareAndSwap(true, false) {
		return
	}

	// Give the frame reader loop time to exit
	time.Sleep(100 * time.Millisecond)

	// Stop the thread loop
	C.pw_stop_loop(npc.userData)

	// Cleanup
	C.pw_cleanup(npc.userData)
	npc.userData = nil
}

// IsRunning returns whether capture is running
func (npc *NativePipeWireCapture) IsRunning() bool {
	return npc.running.Load()
}
