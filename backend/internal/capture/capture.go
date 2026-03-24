package capture

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// ScreenCapture handles screen capture via Wayland/Pipewire
type ScreenCapture struct {
	conn          *dbus.Conn
	sessionHandle string
	streamNode    uint32
	fps           int
	ctx           context.Context
	cancel        context.CancelFunc
	
	// Real Pipewire capture
	gstCmd        *exec.Cmd
	frameBuffer   *image.RGBA
	frameMutex    sync.RWMutex
	useMockFrames bool
}

// Config holds screen capture configuration
type Config struct {
	FPS           int  // Target frames per second (10-60)
	Monitor       int  // Monitor index (-1 for all monitors)
	UseMockFrames bool // Use mock gradient instead of real capture (for testing)
}

// NewScreenCapture creates a new screen capture instance
func NewScreenCapture(cfg Config) (*ScreenCapture, error) {
	// Validate FPS
	if cfg.FPS < 10 || cfg.FPS > 60 {
		return nil, fmt.Errorf("FPS must be between 10 and 60, got %d", cfg.FPS)
	}

	// Connect to session bus for XDG Desktop Portal
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ScreenCapture{
		conn:          conn,
		fps:           cfg.FPS,
		ctx:           ctx,
		cancel:        cancel,
		useMockFrames: cfg.UseMockFrames,
	}, nil
}

// Start begins screen capture
func (sc *ScreenCapture) Start() error {
	// If using mock frames, skip portal setup
	if sc.useMockFrames {
		fmt.Println("Using mock frames - skipping XDG Portal setup")
		return nil
	}
	
	// Step 1: Create session
	sessionHandle, err := sc.createSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sc.sessionHandle = sessionHandle

	// Step 2: Select sources (monitors)
	if err := sc.selectSources(sessionHandle); err != nil {
		return fmt.Errorf("failed to select sources: %w", err)
	}

	// Step 3: Start Pipewire stream
	streamNode, err := sc.startStream(sessionHandle)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	sc.streamNode = streamNode

	fmt.Printf("Screen capture started: session=%s, node=%d\n", sessionHandle, streamNode)
	
	// Start real Pipewire capture if not using mock frames
	if !sc.useMockFrames {
		if err := sc.startPipewireCapture(); err != nil {
			return fmt.Errorf("failed to start Pipewire capture: %w", err)
		}
	}
	
	return nil
}

// CaptureFrame captures a single frame
func (sc *ScreenCapture) CaptureFrame() (*image.RGBA, error) {
	if sc.useMockFrames {
		return sc.generateMockFrame(), nil
	}
	
	// Return latest frame from Pipewire capture
	sc.frameMutex.RLock()
	defer sc.frameMutex.RUnlock()
	
	if sc.frameBuffer == nil {
		return nil, fmt.Errorf("no frame available yet")
	}
	
	// Return a copy to avoid race conditions
	bounds := sc.frameBuffer.Bounds()
	frame := image.NewRGBA(bounds)
	copy(frame.Pix, sc.frameBuffer.Pix)
	
	return frame, nil
}

// generateMockFrame creates a test frame with a gradient for pipeline testing
func (sc *ScreenCapture) generateMockFrame() *image.RGBA {
	// Create a test image with gradient colors
	// This simulates screen content for testing color extraction
	width, height := 1920, 1080
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Generate gradient: red->green->blue across width
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Horizontal gradient
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(((width - x) * 255) / width)
			
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	
	return img
}

// Stop stops the capture session
func (sc *ScreenCapture) Stop() {
	// Stop gstreamer pipeline
	if sc.gstCmd != nil && sc.gstCmd.Process != nil {
		sc.gstCmd.Process.Kill()
	}
	
	if sc.cancel != nil {
		sc.cancel()
	}
	if sc.conn != nil {
		sc.conn.Close()
	}
}

// startPipewireCapture starts capturing frames from Pipewire using gstreamer
func (sc *ScreenCapture) startPipewireCapture() error {
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
	
	fmt.Printf("✅ Real Pipewire capture started (gstreamer pipeline)\n")
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
					// Convert to RGBA if needed
					rgba, ok := frame.(*image.RGBA)
					if !ok {
						bounds := frame.Bounds()
						rgba = image.NewRGBA(bounds)
						for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
							for x := bounds.Min.X; x < bounds.Max.X; x++ {
								rgba.Set(x, y, frame.At(x, y))
							}
						}
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
