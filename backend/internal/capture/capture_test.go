package capture

import (
	"fmt"
	"image"
	"sync"
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

// TestNewScreenCapture tests ScreenCapture creation and validation
func TestNewScreenCapture(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config with mock frames",
			cfg: Config{
				FPS:           30,
				UseMockFrames: true,
			},
			expectError: false,
		},
		{
			name: "Valid config at min FPS",
			cfg: Config{
				FPS:           MinFPS,
				UseMockFrames: true,
			},
			expectError: false,
		},
		{
			name: "Valid config at max FPS",
			cfg: Config{
				FPS:           MaxFPS,
				UseMockFrames: true,
			},
			expectError: false,
		},
		{
			name: "FPS too low",
			cfg: Config{
				FPS:           MinFPS - 1,
				UseMockFrames: true,
			},
			expectError: true,
			errorMsg:    "FPS must be between",
		},
		{
			name: "FPS too high",
			cfg: Config{
				FPS:           MaxFPS + 1,
				UseMockFrames: true,
			},
			expectError: true,
			errorMsg:    "FPS must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := NewScreenCapture(tt.cfg)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if sc == nil {
					t.Error("Expected non-nil ScreenCapture")
				} else {
					// Clean up
					sc.Stop()
				}
			}
		})
	}
}

// TestCaptureFrame_MockMode tests capturing frames in mock mode
func TestCaptureFrame_MockMode(t *testing.T) {
	cfg := Config{
		FPS:           30,
		UseMockFrames: true,
	}

	sc, err := NewScreenCapture(cfg)
	if err != nil {
		t.Fatalf("Failed to create ScreenCapture: %v", err)
	}
	defer sc.Stop()

	// Start capture (mock mode doesn't need portal)
	if err := sc.Start(); err != nil {
		t.Fatalf("Failed to start capture: %v", err)
	}

	// Capture frame
	frame, err := sc.CaptureFrame()
	if err != nil {
		t.Errorf("CaptureFrame() failed: %v", err)
	}
	if frame == nil {
		t.Error("CaptureFrame() returned nil frame")
	}

	// Verify frame dimensions
	if frame.Bounds().Dx() != 1920 || frame.Bounds().Dy() != 1080 {
		t.Errorf("Expected 1920x1080, got %dx%d", frame.Bounds().Dx(), frame.Bounds().Dy())
	}

	// Return buffer to pool
	PutImageBuffer(frame)
}

// TestStop tests that Stop cleans up resources
func TestStop(t *testing.T) {
	cfg := Config{
		FPS:           30,
		UseMockFrames: true,
	}

	sc, err := NewScreenCapture(cfg)
	if err != nil {
		t.Fatalf("Failed to create ScreenCapture: %v", err)
	}

	// Start and stop
	if err := sc.Start(); err != nil {
		t.Fatalf("Failed to start capture: %v", err)
	}

	sc.Stop()

	// Verify cleanup
	if sc.sessionHandle != "" {
		t.Error("Expected sessionHandle to be cleared")
	}
	if sc.streamNode != 0 {
		t.Error("Expected streamNode to be cleared")
	}
	if sc.conn != nil {
		t.Error("Expected conn to be closed")
	}
}

// TestPutImageBuffer_Nil tests that nil buffers don't crash
func TestPutImageBuffer_Nil(t *testing.T) {
	// Should not panic
	PutImageBuffer(nil)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// fillPix paints every byte of img with v so a reader can tell in one pass
// whether it copied a single frame or a buffer that changed under it.
func fillPix(img *image.RGBA, v byte) {
	for i := range img.Pix {
		img.Pix[i] = v
	}
}

// TestPublishFrameRecyclingIsSafeUnderConcurrentCapture drives publishFrame
// the way nativeFrameReaderLoop does while several CaptureFrame readers run
// concurrently. Each published frame is uniformly painted with its own marker
// byte, so a buffer recycled before its readers finished shows up as a frame
// containing two different markers. Weakening publishFrame's write lock, or
// pooling the old buffer before the swap is visible, fails this test and
// trips the race detector.
func TestPublishFrameRecyclingIsSafeUnderConcurrentCapture(t *testing.T) {
	const (
		frames  = 2000
		readers = 4
	)

	bounds := image.Rect(0, 0, 32, 32)
	sc := &ScreenCapture{}

	seed := GetImageBuffer(bounds)
	fillPix(seed, 1)
	sc.frameBuffer = seed

	done := make(chan struct{})
	var wg sync.WaitGroup

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				frame, err := sc.CaptureFrame()
				if err != nil {
					t.Errorf("CaptureFrame: %v", err)
					return
				}

				want := frame.Pix[0]
				for i, got := range frame.Pix {
					if got != want {
						t.Errorf("torn frame: byte %d is %d, want %d (a recycled buffer was rewritten mid-copy)", i, got, want)
						PutImageBuffer(frame)
						return
					}
				}
				PutImageBuffer(frame)
			}
		}()
	}

	for i := 0; i < frames; i++ {
		next := GetImageBuffer(bounds)
		fillPix(next, byte(i%254)+1)
		sc.publishFrame(next)
	}

	close(done)
	wg.Wait()
}

// TestSourceBytesPerPixel pins the format table validateFrameGeometry sizes
// its bounds check against. A format listed here but not in convertToRGBA's
// switch (or vice versa) is what the switch's default case guards against.
func TestSourceBytesPerPixel(t *testing.T) {
	fourByte := []uint32{
		SPA_VIDEO_FORMAT_BGRx, SPA_VIDEO_FORMAT_BGRA,
		SPA_VIDEO_FORMAT_RGBx, SPA_VIDEO_FORMAT_RGBA,
		SPA_VIDEO_FORMAT_xBGR, SPA_VIDEO_FORMAT_xRGB,
	}
	for _, format := range fourByte {
		if got := sourceBytesPerPixel(format); got != 4 {
			t.Errorf("sourceBytesPerPixel(%d) = %d, want 4", format, got)
		}
	}

	for _, format := range []uint32{SPA_VIDEO_FORMAT_BGR, SPA_VIDEO_FORMAT_RGB} {
		if got := sourceBytesPerPixel(format); got != 3 {
			t.Errorf("sourceBytesPerPixel(%d) = %d, want 3", format, got)
		}
	}

	for _, format := range []uint32{0, 1, 99, 4294967295} {
		if got := sourceBytesPerPixel(format); got != 0 {
			t.Errorf("sourceBytesPerPixel(%d) = %d, want 0 for an unhandled format", format, got)
		}
	}
}

// TestValidateFrameGeometry covers the geometry a malformed PipeWire frame can
// report. Every rejected case would otherwise index past the end of the C
// mapping in convertToRGBA and panic the daemon, so these are crash cases, not
// merely invalid input.
func TestValidateFrameGeometry(t *testing.T) {
	tests := []struct {
		name                                 string
		width, height, stride, bytesPerPixel int
		wantErr                              bool
	}{
		{"typical 1440p BGRx", 2560, 1440, 10240, 4, false},
		{"stride padded beyond the row", 1920, 1080, 8192, 4, false},
		{"tightly packed 24-bit", 1920, 1080, 5760, 3, false},
		{"zero stride on a data-less frame", 1920, 1080, 0, 4, true},
		{"stride one byte short of a row", 1920, 1080, 7679, 4, true},
		{"stride sized for 3 bytes but format needs 4", 1920, 1080, 5760, 4, true},
		{"zero width", 0, 1080, 0, 4, true},
		{"zero height", 1920, 0, 7680, 4, true},
		{"negative height", 1920, -1, 7680, 4, true},
		{"negative stride", 1920, 1080, -7680, 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFrameGeometry(tt.width, tt.height, tt.stride, tt.bytesPerPixel)
			if tt.wantErr && err == nil {
				t.Errorf("validateFrameGeometry(%d, %d, %d, %d) = nil, want an error",
					tt.width, tt.height, tt.stride, tt.bytesPerPixel)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateFrameGeometry(%d, %d, %d, %d) = %v, want nil",
					tt.width, tt.height, tt.stride, tt.bytesPerPixel, err)
			}
		})
	}
}

// TestCaptureBreakerTripped covers the frame-capture grace period. It has to
// be a deadline rather than a count of failed polls: the reader loop ticks at
// the configured FPS, so a count would give 0.5s of grace at 60 FPS and 3s at
// 10, and raising the frame rate alone could stop capture before the
// compositor delivered its first buffer.
func TestCaptureBreakerTripped(t *testing.T) {
	const grace = time.Second
	start := time.Now()

	tests := []struct {
		name       string
		firstErrAt time.Time
		now        time.Time
		want       bool
	}{
		{name: "no failures yet", firstErrAt: time.Time{}, now: start, want: false},
		{name: "within the grace period", firstErrAt: start, now: start.Add(999 * time.Millisecond), want: false},
		{name: "at the grace period", firstErrAt: start, now: start.Add(grace), want: true},
		{name: "past the grace period", firstErrAt: start, now: start.Add(5 * time.Second), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := captureBreakerTripped(tt.firstErrAt, tt.now, grace); got != tt.want {
				t.Errorf("captureBreakerTripped(%v, %v, %v) = %v, want %v", tt.firstErrAt, tt.now, grace, got, tt.want)
			}
		})
	}
}

// TestCaptureBreakerGraceIsIndependentOfFPS pins the deadline's semantics: the
// same wall-clock streak trips at every supported frame rate, while the number
// of polls to get there tracks the rate. It drives captureBreakerTripped
// directly and simulates the cadence, so it does not catch nativeFrameReaderLoop
// going back to counting errors; testing that needs a live PipeWire stream.
func TestCaptureBreakerGraceIsIndependentOfFPS(t *testing.T) {
	const grace = time.Second

	for _, fps := range []int{MinFPS, 30, MaxFPS} {
		t.Run(fmt.Sprintf("%d FPS", fps), func(t *testing.T) {
			interval := time.Second / time.Duration(fps)
			now := time.Now()
			firstErrAt := now

			polls := 0
			for !captureBreakerTripped(firstErrAt, now, grace) {
				now = now.Add(interval)
				polls++
				if polls > 10*fps {
					t.Fatalf("breaker never tripped after %d polls", polls)
				}
			}

			if elapsed := now.Sub(firstErrAt); elapsed < grace || elapsed > grace+interval {
				t.Errorf("tripped after %v, want about %v", elapsed, grace)
			}
			// time.Second/fps truncates, so the streak can need one extra poll
			// to clear the deadline. The point is that the poll count tracks
			// the frame rate while the wall-clock deadline does not.
			if polls < fps || polls > fps+1 {
				t.Errorf("tripped after %d polls at %d FPS, want %d or %d", polls, fps, fps, fps+1)
			}
		})
	}
}
