package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // Register PNG decoder
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// Constants for screen capture
const (
	MinFPS = 10 // Minimum frames per second for capture (matches config.MinFPS)
	MaxFPS = 60 // Maximum frames per second (matches config.MaxFPS)
)

// Pool for RGBA image buffers used in frame copying to reduce GC pressure.
// Reusing buffers avoids allocating/deallocating large RGBA images every frame
// (14.7MB at 2560x1440). Pool size is managed automatically by the runtime.
//
// New returns an empty image rather than a pre-sized one: GetImageBuffer
// allocates at the caller's bounds whenever the fetched buffer is the wrong
// size, so pre-sizing here is only ever right for one display and costs a
// full-resolution allocation that is discarded immediately on every other.
var imageBufferPool = sync.Pool{
	New: func() any {
		return image.NewRGBA(image.Rectangle{})
	},
}

// GetImageBuffer gets an image buffer from the pool or creates a new one if size doesn't match
func GetImageBuffer(bounds image.Rectangle) *image.RGBA {
	img := imageBufferPool.Get().(*image.RGBA)

	// Check if pooled buffer matches required size
	if img.Bounds() != bounds {
		// Size mismatch - create new buffer with correct size
		img = image.NewRGBA(bounds)
	}

	return img
}

// PutImageBuffer returns an image buffer to the pool
func PutImageBuffer(img *image.RGBA) {
	if img != nil {
		imageBufferPool.Put(img)
	}
}

// ScreenCapture handles screen capture via Wayland/Pipewire
type ScreenCapture struct {
	conn          *dbus.Conn
	sessionHandle string
	streamNode    uint32
	fps           int
	fpsMutex      sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc

	// Real Pipewire capture
	gstCmd           *exec.Cmd
	nativeCapture    *NativePipeWireCapture
	frameBuffer      *image.RGBA
	frameMutex       sync.RWMutex
	useMockFrames    bool
	useNativeCapture bool
	frameReaderWg    sync.WaitGroup // Tracks frame reader goroutine lifecycle
	consecutiveErrs  int            // Attempts in the current failure streak, reported when capture gives up
	captureErr       error          // Why capture gave up; guarded by frameMutex, cleared by Start

	// Screenshot-based capture
	useScreenshot  bool
	screenshotTool string
	captureWidth   int
	captureHeight  int

	// Restore token for permission persistence
	restoreToken string     // Current restore token from config, guarded by tokenMutex
	tokenMutex   sync.Mutex // Start reads the token on the sync goroutine while DBus clears it

	// Bumped by ClearRestoreToken so a start already waiting on the portal
	// dialog cannot save its grant over a reset the user was told had worked.
	tokenGeneration uint64

	onTokenUpdate func(newToken string) // Callback to save new token
}

// Config holds screen capture configuration
type Config struct {
	FPS              int             // Target frames per second (10-60)
	UseMockFrames    bool            // Use mock gradient instead of real capture (for testing)
	UseNativeCapture bool            // Use native CGo PipeWire capture (default: true)
	UseScreenshot    bool            // Use screenshot method for real capture (simple, higher CPU)
	ScreenshotTool   string          // Screenshot tool to use: "spectacle", "import", etc (auto-detect if empty)
	CaptureWidth     int             // Downsample width (0 = full resolution, e.g. 640 for faster)
	CaptureHeight    int             // Downsample height (0 = full resolution, e.g. 360 for faster)
	Context          context.Context // Parent context for cancellation (optional, defaults to Background)
	RestoreToken     string          // Portal restore token from previous session (optional, empty = show dialog)
	OnTokenUpdate    func(string)    // Callback when new restore token is available (optional)
}

// NewScreenCapture creates a new screen capture instance
func NewScreenCapture(cfg Config) (*ScreenCapture, error) {
	// Validate FPS
	if cfg.FPS < MinFPS || cfg.FPS > MaxFPS {
		return nil, fmt.Errorf("FPS must be between %d and %d, got %d", MinFPS, MaxFPS, cfg.FPS)
	}

	// Connect to session bus for XDG Desktop Portal
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	// Use provided context or default to Background
	parentCtx := cfg.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)

	// Auto-detect screenshot tool if UseScreenshot is enabled
	screenshotTool := cfg.ScreenshotTool
	if cfg.UseScreenshot && screenshotTool == "" {
		screenshotTool = detectScreenshotTool()
		if screenshotTool == "" {
			cancel() // Clean up context on error
			return nil, fmt.Errorf("no screenshot tool available (need spectacle, grim, or import)")
		}
	}

	return &ScreenCapture{
		conn:             conn,
		fps:              cfg.FPS,
		ctx:              ctx,
		cancel:           cancel,
		useMockFrames:    cfg.UseMockFrames,
		useNativeCapture: cfg.UseNativeCapture,
		useScreenshot:    cfg.UseScreenshot,
		screenshotTool:   screenshotTool,
		captureWidth:     cfg.CaptureWidth,
		captureHeight:    cfg.CaptureHeight,
		restoreToken:     cfg.RestoreToken,
		onTokenUpdate:    cfg.OnTokenUpdate,
	}, nil
}

// detectScreenshotTool finds an available screenshot tool
func detectScreenshotTool() string {
	tools := []string{"spectacle", "grim", "import"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			return tool
		}
	}
	return ""
}

// Start begins screen capture
func (sc *ScreenCapture) Start() error {
	// The field outlives the reader goroutine, so without this a session
	// that gave up would poison every session after it.
	sc.frameMutex.Lock()
	sc.captureErr = nil
	sc.frameMutex.Unlock()

	// If using mock frames or screenshots, skip portal setup
	if sc.useMockFrames || sc.useScreenshot {
		// Mock/screenshot modes don't need portal setup
		return nil
	}

	// Recreate DBus connection if it was closed (e.g., after Stop())
	if sc.conn == nil {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			return fmt.Errorf("failed to connect to session bus: %w", err)
		}
		sc.conn = conn
	}

	// Recreate context if it was cancelled (e.g., after Stop())
	var contextCreated bool
	if sc.ctx.Err() != nil {
		ctx, cancel := context.WithCancel(context.Background())
		sc.ctx = ctx
		sc.cancel = cancel
		contextCreated = true
	}

	// Cleanup context on error if we created it
	var success bool
	defer func() {
		if !success && contextCreated && sc.cancel != nil {
			sc.cancel()
		}
	}()

	// Step 1: Create session
	sessionHandle, err := sc.createSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sc.sessionHandle = sessionHandle

	// Step 2: Select sources (monitors) with restore token
	sc.tokenMutex.Lock()
	currentToken := sc.restoreToken
	startGeneration := sc.tokenGeneration
	sc.tokenMutex.Unlock()

	if err := sc.selectSources(sessionHandle, currentToken); err != nil {
		return fmt.Errorf("failed to select sources: %w", err)
	}

	// Step 3: Start Pipewire stream and get NEW restore token
	streamNode, newToken, err := sc.startStream(sessionHandle)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	sc.streamNode = streamNode

	log.Printf("Screen capture started: session=%s, node=%d", sessionHandle, streamNode)

	// Save new restore token for next session (if callback provided)
	sc.tokenMutex.Lock()
	reset := startGeneration != sc.tokenGeneration
	tokenChanged := !reset && newToken != "" && newToken != sc.restoreToken
	if tokenChanged {
		sc.restoreToken = newToken
	}
	sc.tokenMutex.Unlock()

	if reset && newToken != "" {
		log.Printf("[INFO] Screen share was reset while this session started, its grant is not saved")
	}

	if tokenChanged && sc.onTokenUpdate != nil {
		// CRITICAL: Run callback in goroutine to avoid blocking capture startup
		// The callback saves config which acquires locks that caller (engine.Start) may hold
		// Running synchronously caused a deadlock where capture.Start blocked indefinitely
		// Async execution allows capture to complete startup before config save
		go sc.onTokenUpdate(newToken)
	}

	// Start real Pipewire capture if not using mock frames
	if !sc.useMockFrames {
		if err := sc.startPipewireCapture(); err != nil {
			return fmt.Errorf("failed to start Pipewire capture: %w", err)
		}
	}

	success = true
	return nil
}

// ErrCaptureStopped wraps the reason capture gave up. Once CaptureFrame
// returns an error matching it, no further frame will arrive without a new
// Start, so a caller that keeps polling is streaming a frozen image.
var ErrCaptureStopped = errors.New("screen capture stopped")

// failCapture records why the frame reader gave up. Without this the reader
// goroutine can exit while CaptureFrame keeps handing back the last frame it
// published, which leaves the caller streaming one still image with nothing
// reported.
func (sc *ScreenCapture) failCapture(cause error) {
	sc.frameMutex.Lock()
	defer sc.frameMutex.Unlock()

	if sc.captureErr == nil {
		sc.captureErr = fmt.Errorf("%w: %w", ErrCaptureStopped, cause)
	}
}

// CaptureFrame captures a single frame
func (sc *ScreenCapture) CaptureFrame() (*image.RGBA, error) {
	if sc.useMockFrames {
		return sc.generateMockFrame(), nil
	}

	if sc.useScreenshot {
		return sc.captureScreenshot()
	}

	// Return latest frame from Pipewire capture
	sc.frameMutex.RLock()
	defer sc.frameMutex.RUnlock()

	// Checked before frameBuffer: the buffer outlives the reader goroutine,
	// so a stale frame is exactly what is on offer once capture has failed.
	if sc.captureErr != nil {
		return nil, sc.captureErr
	}

	if sc.frameBuffer == nil {
		return nil, fmt.Errorf("no frame available yet")
	}

	bounds := sc.frameBuffer.Bounds()
	frame := GetImageBuffer(bounds)
	copy(frame.Pix, sc.frameBuffer.Pix)

	return frame, nil
}

// generateMockFrame creates a test frame with a gradient for pipeline testing
func (sc *ScreenCapture) generateMockFrame() *image.RGBA {
	// Create a test image with solid colors per zone for easy testing
	// Left = RED, Center = GREEN, Right = BLUE
	width, height := 1920, 1080
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Divide into 3 zones
	leftBoundary := width / 3
	rightBoundary := 2 * width / 3

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var c color.RGBA
			if x < leftBoundary {
				// Left zone: RED
				c = color.RGBA{R: 255, G: 0, B: 0, A: 255}
			} else if x < rightBoundary {
				// Center zone: GREEN
				c = color.RGBA{R: 0, G: 255, B: 0, A: 255}
			} else {
				// Right zone: BLUE
				c = color.RGBA{R: 0, G: 0, B: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}

	return img
}

// captureScreenshot captures a screenshot using the configured tool
func (sc *ScreenCapture) captureScreenshot() (*image.RGBA, error) {
	var cmd *exec.Cmd
	var output []byte
	var err error

	switch sc.screenshotTool {
	case "spectacle":
		// Spectacle doesn't support stdout, use temp file
		tmpfile := "/tmp/hue-screenshot.png"
		defer func() {
			if err := os.Remove(tmpfile); err != nil {
				log.Printf("warn: failed to remove temp screenshot file: %v", err)
			}
		}()

		cmd = exec.CommandContext(sc.ctx, "spectacle", "-b", "-n", "-o", tmpfile)
		if err = cmd.Run(); err != nil {
			return nil, fmt.Errorf("spectacle failed: %w", err)
		}

		// If downsampling requested, use ImageMagick convert
		if sc.captureWidth > 0 && sc.captureHeight > 0 {
			// Use convert to resize: convert input.png -resize WxH output.png
			resizeCmd := exec.CommandContext(sc.ctx, "convert", tmpfile,
				"-resize", fmt.Sprintf("%dx%d!", sc.captureWidth, sc.captureHeight),
				tmpfile)
			if err = resizeCmd.Run(); err != nil {
				return nil, fmt.Errorf("resize failed: %w", err)
			}
		}

		output, err = os.ReadFile(tmpfile)
		if err != nil {
			return nil, fmt.Errorf("failed to read screenshot: %w", err)
		}

	case "grim":
		// Grim (Wayland): capture to stdout with optional scale
		args := []string{}
		if sc.captureWidth > 0 && sc.captureHeight > 0 {
			args = append(args, "-s", fmt.Sprintf("%d,%d", sc.captureWidth, sc.captureHeight))
		}
		args = append(args, "-")
		cmd = exec.CommandContext(sc.ctx, "grim", args...)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("grim failed: %w", err)
		}

	case "import":
		// ImageMagick import: capture root window with optional resize
		args := []string{"-window", "root"}
		if sc.captureWidth > 0 && sc.captureHeight > 0 {
			args = append(args, "-resize", fmt.Sprintf("%dx%d!", sc.captureWidth, sc.captureHeight))
		}
		args = append(args, "png:-")
		cmd = exec.CommandContext(sc.ctx, "import", args...)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("import failed: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported screenshot tool: %s", sc.screenshotTool)
	}

	// Decode PNG image
	img, _, err := image.Decode(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	// Convert to RGBA using image/draw for better performance
	rgba, ok := img.(*image.RGBA)
	if !ok {
		bounds := img.Bounds()
		rgba = image.NewRGBA(bounds)
		// Use draw.Draw for efficient pixel copying instead of nested loops
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	return rgba, nil
}

// Stop stops the capture session
func (sc *ScreenCapture) Stop() {
	// Cancel context to signal goroutines to stop
	if sc.cancel != nil {
		sc.cancel()
	}

	// Wait for frame reader goroutine to exit (prevents goroutine leak)
	sc.frameReaderWg.Wait()

	// Stop native capture if active
	if sc.nativeCapture != nil {
		sc.nativeCapture.Stop()
		sc.nativeCapture = nil
	}

	// Stop gstreamer pipeline
	if sc.gstCmd != nil && sc.gstCmd.Process != nil {
		if err := sc.gstCmd.Process.Kill(); err != nil {
			log.Printf("warn: failed to kill gstreamer process: %v", err)
		}
		// Wait for process to exit to prevent zombie process
		if err := sc.gstCmd.Wait(); err != nil {
			log.Printf("warn: gstreamer process wait error: %v", err)
		}
	}

	if sc.conn != nil {
		if err := sc.conn.Close(); err != nil {
			log.Printf("warn: failed to close DBus connection: %v", err)
		}
	}

	// Reset portal state so it can be recreated on next Start()
	sc.sessionHandle = ""
	sc.streamNode = 0
	sc.conn = nil

	sc.publishFrame(nil)
}

// publishFrame installs frame as the latest captured frame and returns the
// buffer it replaced to the pool. Recycling is only safe because the write
// lock waits for every outstanding RLock holder to release before it is
// granted: any CaptureFrame call that read the old pointer has therefore
// finished copying out of it by the time Lock returns. Downgrading this to
// an RLock, or dropping it, hands a buffer back to the pool while a reader
// is still copying from it, and the next writer scribbles over the read.
func (sc *ScreenCapture) publishFrame(frame *image.RGBA) {
	sc.frameMutex.Lock()
	old := sc.frameBuffer
	sc.frameBuffer = frame
	sc.frameMutex.Unlock()

	PutImageBuffer(old)
}

// startPipewireCapture starts capturing frames from Pipewire
func (sc *ScreenCapture) startPipewireCapture() error {
	// Use native CGo capture if enabled (default)
	if sc.useNativeCapture {
		return sc.startNativePipewireCapture()
	}

	// Fall back to GStreamer-based capture
	return sc.startGStreamerCapture()
}

// startNativePipewireCapture starts native CGo-based PipeWire capture
func (sc *ScreenCapture) startNativePipewireCapture() error {
	// Create native capture
	nativeCapture, err := NewNativePipeWireCapture(sc.streamNode)
	if err != nil {
		return fmt.Errorf("failed to create native capture: %w", err)
	}

	// Stop() is the only complete unwinder for a running instance: it also
	// cancels the context and joins nativeFrameReaderLoop, which a teardown
	// here could not do without tearing down the context this Start just set
	// up. A partial stop here would leave the old reader polling and racing
	// the write below, so rely on the engine's Start/Stop pairing instead.
	sc.nativeCapture = nativeCapture

	// Start capture (runs in background)
	if err := sc.nativeCapture.Start(); err != nil {
		// NewNativePipeWireCapture already connected the stream and created
		// the loop, and no caller unwinds this: ScreenCapture.Start's defer
		// only cancels the context, and the engine returns without calling
		// Stop. Release it here or every retry leaks a loop thread.
		sc.nativeCapture.Stop()
		sc.nativeCapture = nil
		return fmt.Errorf("failed to start native capture: %w", err)
	}

	// Start frame polling goroutine with WaitGroup tracking
	sc.frameReaderWg.Add(1)
	go sc.nativeFrameReaderLoop()

	log.Printf("Native PipeWire capture started (CGo + libpipewire)")
	return nil
}

// captureBreakerTripped reports whether a run of failed polls that began at
// firstErrAt has lasted long enough to give up on capture.
func captureBreakerTripped(firstErrAt, now time.Time, grace time.Duration) bool {
	if firstErrAt.IsZero() {
		return false
	}
	return now.Sub(firstErrAt) >= grace
}

// nativeFrameReaderLoop polls frames from native capture
func (sc *ScreenCapture) nativeFrameReaderLoop() {
	defer sc.frameReaderWg.Done()
	interval := sc.GetFrameInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// A deadline, not an error count. This loop's period follows the
	// configured FPS, so counting ticks would give 0.5s of grace at 60 and 3s
	// at 10: raising the frame rate alone could stop capture before the
	// compositor delivered its first buffer.
	//
	// Tripping it now ends the sync session rather than just the reader loop,
	// so it has to outlast a cold compositor start on a loaded machine.
	const captureGrace = 5 * time.Second

	// The whole budget for getting a first frame, measured from here rather
	// than from whenever the stream reached the streaming state. Both waits
	// draw on it, including the generic one below, because a stream can bounce
	// between the two states before any buffer arrives and timing each stretch
	// on its own lets them add up. No frame means the engine has nothing to
	// send, and a bridge left with no packets long enough drops out of
	// streaming mode, which would leave the lights unresponsive for the rest
	// of a session that still reports itself live.
	//
	// Rests on a frame never returning the loop to this state: ready_idx is
	// set to -1 once at allocation and never again, so ErrAwaitingFirstFrame
	// cannot reappear mid-session. Reset it on renegotiation and this deadline
	// fires immediately on any session older than the grace.
	const firstFrameGrace = 6 * time.Second
	startedAt := time.Now()

	var firstErrAt time.Time
	var haveFrame bool

	// The field outlives this loop, so a previous session's streak would be
	// added to the attempt count the breaker reports.
	sc.consecutiveErrs = 0

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-ticker.C:
			if current := sc.GetFrameInterval(); current != interval {
				interval = current
				ticker.Reset(interval)
			}

			// GetFrame's conversion runs unlocked: it draws its destination
			// from the shared image buffer pool (see convertToRGBA), so it
			// never touches whatever sc.frameBuffer currently points to.
			frame, err := sc.nativeCapture.GetFrame()

			// The compositor publishes on damage, so a still screen produces
			// no frames at all. Holding the last one is the correct output
			// here, and arming the breaker would kill a healthy session.
			// The streak clears because a frame is there to be read: leaving
			// it armed lets an idle spell join two unrelated errors into one
			// run long enough to trip the breaker.
			if errors.Is(err, ErrNoNewFrame) {
				sc.consecutiveErrs = 0
				firstErrAt = time.Time{}
				continue
			}

			// The stream is up and simply has not sent anything yet, which on
			// a still screen is normal. Bounded all the same, so a producer
			// that connects and never delivers is still reported.
			if errors.Is(err, ErrAwaitingFirstFrame) {
				sc.consecutiveErrs = 0
				firstErrAt = time.Time{}
				now := time.Now()
				if captureBreakerTripped(startedAt, now, firstFrameGrace) {
					elapsed := now.Sub(startedAt).Round(time.Millisecond)
					log.Printf("[ERROR] Stream up but no first frame after %v, stopping capture", elapsed)
					sc.failCapture(fmt.Errorf("stream delivered no frame in %v", elapsed))
					if sc.cancel != nil {
						sc.cancel()
					}
					return
				}
				continue
			}

			// A failed stream never recovers, so there is nothing to wait out.
			if errors.Is(err, ErrStreamFailed) {
				log.Printf("[ERROR] Stopping capture: %v", err)
				sc.failCapture(err)
				if sc.cancel != nil {
					sc.cancel()
				}
				return
			}

			if err != nil {
				sc.consecutiveErrs++
				now := time.Now()
				if firstErrAt.IsZero() {
					firstErrAt = now
				}
				// Before the first frame this shares the budget above. Its own
				// clock restarts every time the stream drops back out of the
				// streaming state, so on a stream that bounces it would never
				// run out while the total dead air kept growing.
				if !haveFrame && captureBreakerTripped(startedAt, now, firstFrameGrace) {
					elapsed := now.Sub(startedAt).Round(time.Millisecond)
					log.Printf("[ERROR] No first frame after %v, stopping capture", elapsed)
					sc.failCapture(fmt.Errorf("stream delivered no frame in %v", elapsed))
					if sc.cancel != nil {
						sc.cancel()
					}
					return
				}
				if captureBreakerTripped(firstErrAt, now, captureGrace) {
					elapsed := now.Sub(firstErrAt).Round(time.Millisecond)
					log.Printf("[ERROR] Frame capture failing for %v (%d attempts), stopping capture (possible permission denial or PipeWire issue)",
						elapsed, sc.consecutiveErrs)
					sc.failCapture(fmt.Errorf("no frame for %v (%d attempts): %w", elapsed, sc.consecutiveErrs, err))
					if sc.cancel != nil {
						sc.cancel() // Stop the capture to prevent wasting CPU
					}
					return
				}
				// No frame available yet, continue trying
				continue
			}

			// Frame captured successfully - reset the failure streak
			sc.consecutiveErrs = 0
			firstErrAt = time.Time{}
			haveFrame = true

			sc.publishFrame(frame)
		}
	}
}

// startGStreamerCapture starts capturing frames from Pipewire using gstreamer (fallback)
func (sc *ScreenCapture) startGStreamerCapture() error {
	// Use gstreamer to capture frames from Pipewire node
	// Simplified pipeline: pipewiresrc -> videoconvert -> jpegenc -> filesink
	// We capture snapshots at the target FPS rate

	pipeline := fmt.Sprintf(
		"pipewiresrc path=%d ! "+
			"videorate ! video/x-raw,framerate=%d/1 ! "+
			"videoconvert ! "+
			"jpegenc ! "+
			"multifilesink location=/tmp/hue-frame-%%05d.jpg post-messages=true max-files=2",
		sc.streamNode,
		int(time.Second/sc.GetFrameInterval()),
	)
	// The pipeline's framerate is fixed for the life of the process, so a
	// later SetFPS only changes how often this path samples the files gst
	// writes. Applying a new rate fully would mean respawning gst-launch.

	// Start gstreamer pipeline
	sc.gstCmd = exec.CommandContext(sc.ctx, "gst-launch-1.0", "-q", pipeline)

	if err := sc.gstCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gstreamer: %w", err)
	}

	// Tracked so Stop's Wait covers this path too, not just the native one
	sc.frameReaderWg.Add(1)
	go sc.frameReaderLoop()

	log.Printf("GStreamer Pipewire capture started (fallback)")
	return nil
}

// frameReaderLoop reads frames written by gstreamer
func (sc *ScreenCapture) frameReaderLoop() {
	defer sc.frameReaderWg.Done()

	interval := sc.GetFrameInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	frameNum := 0

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-ticker.C:
			if current := sc.GetFrameInterval(); current != interval {
				interval = current
				ticker.Reset(interval)
			}

			// Read the latest frame file
			// Gstreamer writes to /tmp/hue-frame-00000.jpg, hue-frame-00001.jpg in rotation
			framePath := fmt.Sprintf("/tmp/hue-frame-%05d.jpg", frameNum%2)

			// Try to read the frame
			if data, err := os.ReadFile(framePath); err == nil {
				if frame, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
					// Convert to RGBA using image/draw for better performance
					rgba, ok := frame.(*image.RGBA)
					if !ok {
						bounds := frame.Bounds()
						rgba = image.NewRGBA(bounds)
						// Use draw.Draw for efficient pixel copying instead of nested loops
						draw.Draw(rgba, bounds, frame, bounds.Min, draw.Src)
					}

					sc.publishFrame(rgba)
				}
			}

			frameNum++
		}
	}
}

// GetFrameInterval returns the time between frames based on FPS
func (sc *ScreenCapture) GetFrameInterval() time.Duration {
	sc.fpsMutex.RLock()
	defer sc.fpsMutex.RUnlock()
	return time.Second / time.Duration(sc.fps)
}

// SetFPS changes the capture rate. The reader loop picks it up on its next
// tick; without this the loop would keep producing frames at the rate set in
// NewScreenCapture and cap the sync engine no matter what it was told.
func (sc *ScreenCapture) SetFPS(fps int) error {
	if fps < MinFPS || fps > MaxFPS {
		return fmt.Errorf("FPS must be between %d and %d, got %d", MinFPS, MaxFPS, fps)
	}
	sc.fpsMutex.Lock()
	sc.fps = fps
	sc.fpsMutex.Unlock()
	return nil
}

// ClearRestoreToken forgets the portal's saved screen-share grant, so the next
// Start shows the picker again. The portal has no option for naming an output,
// so re-picking in its dialog is the only way to change which screen is cast.
func (sc *ScreenCapture) ClearRestoreToken() {
	sc.tokenMutex.Lock()
	sc.restoreToken = ""
	sc.tokenGeneration++
	sc.tokenMutex.Unlock()
}

// TokenIsCurrent reports whether token is still the one this session holds.
// The portal's grant is saved from a goroutine, so a reset can land in between;
// callers check this under whatever lock guards the destination.
func (sc *ScreenCapture) TokenIsCurrent(token string) bool {
	sc.tokenMutex.Lock()
	defer sc.tokenMutex.Unlock()
	return sc.restoreToken == token
}
