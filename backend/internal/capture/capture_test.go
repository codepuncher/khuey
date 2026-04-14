package capture

import (
	"image"
	"testing"
	"time"
)

// TestGetFrameInterval tests frame interval calculation via the real method
func TestGetFrameInterval(t *testing.T) {
	tests := []struct {
		name     string
		fps      int
		expected time.Duration
	}{
		{name: "10 FPS", fps: 10, expected: 100 * time.Millisecond},
		{name: "30 FPS", fps: 30, expected: 33333333 * time.Nanosecond},
		{name: "60 FPS", fps: 60, expected: 16666666 * time.Nanosecond},
		{name: "1 FPS", fps: 1, expected: 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &ScreenCapture{fps: tt.fps}
			interval := sc.GetFrameInterval()

			diff := interval - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Nanosecond {
				t.Errorf("Expected interval %v, got %v (diff: %v)", tt.expected, interval, diff)
			}
		})
	}
}

// TestGetImageBuffer tests that the pool returns correctly-sized buffers
func TestGetImageBuffer(t *testing.T) {
	bounds := image.Rect(0, 0, 640, 480)
	buf := GetImageBuffer(bounds)

	if buf.Bounds() != bounds {
		t.Errorf("Expected bounds %v, got %v", bounds, buf.Bounds())
	}

	// Return to pool
	PutImageBuffer(buf)

	// Get again - should get a buffer (may or may not be the same one)
	buf2 := GetImageBuffer(bounds)
	if buf2.Bounds() != bounds {
		t.Errorf("Expected bounds %v, got %v", bounds, buf2.Bounds())
	}
	PutImageBuffer(buf2)
}

// TestGetImageBuffer_SizeMismatch tests that pool creates new buffer for different sizes
func TestGetImageBuffer_SizeMismatch(t *testing.T) {
	// Pool defaults to 1920x1080, request different size
	bounds := image.Rect(0, 0, 320, 240)
	buf := GetImageBuffer(bounds)

	if buf.Bounds() != bounds {
		t.Errorf("Expected bounds %v, got %v", bounds, buf.Bounds())
	}
	PutImageBuffer(buf)
}

// TestDetectScreenshotTool tests screenshot tool detection
func TestDetectScreenshotTool(t *testing.T) {
	// We can't control which tools are installed, but we can test
	// that the function returns a valid tool or empty string
	tool := detectScreenshotTool()
	validTools := map[string]bool{"spectacle": true, "grim": true, "import": true, "": true}
	if !validTools[tool] {
		t.Errorf("detectScreenshotTool() returned unexpected tool: %q", tool)
	}
}

// TestMockFrame tests that mock frame generation produces correct dimensions
func TestMockFrame(t *testing.T) {
	sc := &ScreenCapture{useMockFrames: true}
	frame := sc.generateMockFrame()

	if frame == nil {
		t.Fatal("Expected non-nil frame")
	}

	bounds := frame.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("Expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Left zone should be red
	r, g, b, _ := frame.At(100, 540).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("Left zone expected red, got RGB(%d,%d,%d)", r>>8, g>>8, b>>8)
	}

	// Center zone should be green
	r, g, b, _ = frame.At(960, 540).RGBA()
	if r>>8 != 0 || g>>8 != 255 || b>>8 != 0 {
		t.Errorf("Center zone expected green, got RGB(%d,%d,%d)", r>>8, g>>8, b>>8)
	}

	// Right zone should be blue
	r, g, b, _ = frame.At(1800, 540).RGBA()
	if r>>8 != 0 || g>>8 != 0 || b>>8 != 255 {
		t.Errorf("Right zone expected blue, got RGB(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}
