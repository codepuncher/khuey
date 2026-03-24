package capture

import (
	"context"
	"fmt"
	"image"
	"image/color"
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
}

// Config holds screen capture configuration
type Config struct {
	FPS     int  // Target frames per second (10-60)
	Monitor int  // Monitor index (-1 for all monitors)
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
		conn:   conn,
		fps:    cfg.FPS,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start begins screen capture
func (sc *ScreenCapture) Start() error {
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
	return nil
}

// CaptureFrame captures a single frame
func (sc *ScreenCapture) CaptureFrame() (*image.RGBA, error) {
	// TODO: Implement real Pipewire frame reading
	// For now, return a mock gradient frame for testing the pipeline
	return sc.generateMockFrame(), nil
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
	if sc.cancel != nil {
		sc.cancel()
	}
	if sc.conn != nil {
		sc.conn.Close()
	}
}

// GetFrameInterval returns the time between frames based on FPS
func (sc *ScreenCapture) GetFrameInterval() time.Duration {
	return time.Second / time.Duration(sc.fps)
}
