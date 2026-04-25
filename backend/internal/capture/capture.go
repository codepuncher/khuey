package capture

import (
	"bytes"
	"context"
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
	MinFPS = 10 // Minimum frames per second for capture (stricter than config.MinFPS)
	MaxFPS = 60 // Maximum frames per second
)

// Pool for RGBA image buffers used in frame copying to reduce GC pressure
// Reusing buffers avoids allocating/deallocating large RGBA images every frame (14.7MB at 2560x1440)
// Pool size is managed automatically by Go runtime based on usage patterns
// Default size covers common high-res displays (2560x1440 and below)
var imageBufferPool = sync.Pool{
	New: func() any {
		return image.NewRGBA(image.Rect(0, 0, 2560, 1440))
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
	ctx           context.Context
	cancel        context.CancelFunc

	// Real Pipewire capture
	gstCmd           *exec.Cmd
	nativeCapture    *NativePipeWireCapture
	frameBuffer      *image.RGBA
	frameMutex       sync.RWMutex
	useMockFrames    bool
	useNativeCapture bool

	// Screenshot-based capture
	useScreenshot  bool
	screenshotTool string
	captureWidth   int
	captureHeight  int

	// Restore token for permission persistence
	restoreToken  string                // Current restore token from config
	onTokenUpdate func(newToken string) // Callback to save new token
}

// Config holds screen capture configuration
type Config struct {
	FPS              int             // Target frames per second (10-60)
	Monitor          int             // Monitor index (-1 for all monitors)
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
	if err := sc.selectSources(sessionHandle, sc.restoreToken); err != nil {
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
	if newToken != "" && newToken != sc.restoreToken {
		sc.restoreToken = newToken
		if sc.onTokenUpdate != nil {
			// CRITICAL: Run callback in goroutine to avoid blocking capture startup
			// The callback saves config which acquires locks that caller (engine.Start) may hold
			// Running synchronously caused a deadlock where capture.Start blocked indefinitely
			// Async execution allows capture to complete startup before config save
			go sc.onTokenUpdate(newToken)
		}
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
		defer os.Remove(tmpfile) // Ensure cleanup in all paths

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
	// Stop native capture if active
	if sc.nativeCapture != nil {
		sc.nativeCapture.Stop()
		sc.nativeCapture = nil
	}

	// Stop gstreamer pipeline
	if sc.gstCmd != nil && sc.gstCmd.Process != nil {
		sc.gstCmd.Process.Kill()
		// Wait for process to exit to prevent zombie process
		sc.gstCmd.Wait()
	}

	if sc.cancel != nil {
		sc.cancel()
	}
	if sc.conn != nil {
		sc.conn.Close()
	}

	// Reset portal state so it can be recreated on next Start()
	sc.sessionHandle = ""
	sc.streamNode = 0
	sc.conn = nil

	sc.frameMutex.Lock()
	sc.frameBuffer = nil
	sc.frameMutex.Unlock()
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

	sc.nativeCapture = nativeCapture

	// Start capture (runs in background)
	if err := sc.nativeCapture.Start(); err != nil {
		return fmt.Errorf("failed to start native capture: %w", err)
	}

	// Start frame polling goroutine
	go sc.nativeFrameReaderLoop()

	log.Printf("Native PipeWire capture started (CGo + libpipewire)")
	return nil
}

// nativeFrameReaderLoop polls frames from native capture
func (sc *ScreenCapture) nativeFrameReaderLoop() {
	ticker := time.NewTicker(sc.GetFrameInterval())
	defer ticker.Stop()

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-ticker.C:
			// Get frame from native capture
			frame, err := sc.nativeCapture.GetFrame()
			if err != nil {
				// No frame available yet, skip
				continue
			}

			// Update frame buffer
			sc.frameMutex.Lock()
			sc.frameBuffer = frame
			sc.frameMutex.Unlock()
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
		sc.fps,
	)

	// Start gstreamer pipeline
	sc.gstCmd = exec.CommandContext(sc.ctx, "gst-launch-1.0", "-q", pipeline)

	if err := sc.gstCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gstreamer: %w", err)
	}

	// Start goroutine to read frames
	go sc.frameReaderLoop()

	log.Printf("GStreamer Pipewire capture started (fallback)")
	return nil
}

// frameReaderLoop reads frames written by gstreamer
func (sc *ScreenCapture) frameReaderLoop() {
	ticker := time.NewTicker(sc.GetFrameInterval())
	defer ticker.Stop()

	frameNum := 0

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-ticker.C:
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

					// Update frame buffer
					sc.frameMutex.Lock()
					sc.frameBuffer = rgba
					sc.frameMutex.Unlock()
				}
			}

			frameNum++
		}
	}
}

// GetFrameInterval returns the time between frames based on FPS
func (sc *ScreenCapture) GetFrameInterval() time.Duration {
	return time.Second / time.Duration(sc.fps)
}
