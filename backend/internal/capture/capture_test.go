package capture

import (
	"errors"
	"fmt"
	"image"
	"strings"
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

// TestCaptureFrameReportsFailureInsteadOfStaleFrame covers the case where the
// reader goroutine gives up: the last published frame is still sitting in
// frameBuffer, and handing it back would leave the caller streaming one still
// image with nothing reported.
func TestCaptureFrameReportsFailureInsteadOfStaleFrame(t *testing.T) {
	sc := &ScreenCapture{}
	sc.publishFrame(GetImageBuffer(image.Rect(0, 0, 4, 4)))

	frame, err := sc.CaptureFrame()
	if err != nil {
		t.Fatalf("CaptureFrame before the failure: %v", err)
	}
	PutImageBuffer(frame)

	sc.failCapture(errors.New("portal denied"))

	frame, err = sc.CaptureFrame()
	if frame != nil {
		t.Error("CaptureFrame handed back a frame after capture failed")
	}
	if !errors.Is(err, ErrCaptureStopped) {
		t.Fatalf("err = %v, want it to match ErrCaptureStopped", err)
	}
	if !strings.Contains(err.Error(), "portal denied") {
		t.Errorf("err = %v, want it to name the cause", err)
	}
}

// TestFailCaptureKeepsFirstCause makes sure the reason capture gave up is the
// original one, not whatever error the loop happened to see last.
func TestFailCaptureKeepsFirstCause(t *testing.T) {
	sc := &ScreenCapture{}
	sc.failCapture(errors.New("portal denied"))
	sc.failCapture(errors.New("something later"))

	_, err := sc.CaptureFrame()
	if !strings.Contains(err.Error(), "portal denied") {
		t.Errorf("err = %v, want the first cause", err)
	}
}

// TestStartClearsPreviousFailure guards against a session that gave up
// poisoning every session after it, the way consecutiveErrs once did.
func TestStartClearsPreviousFailure(t *testing.T) {
	sc := &ScreenCapture{useMockFrames: true}
	sc.failCapture(errors.New("portal denied"))

	if err := sc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	sc.frameMutex.RLock()
	got := sc.captureErr
	sc.frameMutex.RUnlock()

	if got != nil {
		t.Errorf("captureErr = %v after Start, want nil", got)
	}
}

// TestStreamStalled covers the distinction the capture loop depends on: a
// static screen produces no frames while perfectly healthy, so only a stream
// that has left the streaming state long enough counts as dead.
func TestStreamStalled(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		leftStreamingAt time.Time
		want            bool
	}{
		{"still streaming", time.Time{}, false},
		{"just left the streaming state", now, false},
		{"gone, still inside the deadline", now.Add(-streamStallDeadline + time.Millisecond), false},
		{"gone, past the deadline", now.Add(-streamStallDeadline), true},
		{"gone for an hour", now.Add(-time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := streamStalled(tt.leftStreamingAt, now); got != tt.want {
				t.Errorf("streamStalled(%v) = %v, want %v", tt.leftStreamingAt, got, tt.want)
			}
		})
	}
}

// TestFaultSinceClearsOnRecovery covers the clearing half of the fault clock:
// a stream that comes back has to start from a clean slate, or a later fault
// inherits a deadline that has already expired.
func TestFaultSinceClearsOnRecovery(t *testing.T) {
	start := time.Now()

	since := faultSince(false, time.Time{}, start)
	if !since.Equal(start) {
		t.Fatalf("faultSince stamped %v, want %v", since, start)
	}

	// A fault that persists keeps its original timestamp.
	if got := faultSince(false, since, start.Add(time.Minute)); !got.Equal(start) {
		t.Errorf("a continuing fault moved its clock to %v, want %v", got, start)
	}

	if got := faultSince(true, since, start.Add(time.Minute)); !got.IsZero() {
		t.Errorf("recovery left the clock at %v, want it cleared", got)
	}
}

// TestStaleFrameVerdict covers what the same frame coming back can mean. A
// screen with nothing to send produces no frames while perfectly healthy, so
// only a stream that has also left the streaming state is a failure.
func TestStaleFrameVerdict(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		health streamHealth
		want   error
	}{
		{"idle screen", streamHealth{}, ErrNoNewFrame},
		{"paused briefly to renegotiate", streamHealth{leftStreamingAt: now.Add(-time.Millisecond)}, ErrNoNewFrame},
		{"stopped streaming", streamHealth{leftStreamingAt: now.Add(-streamStallDeadline)}, ErrStreamFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.health.staleVerdict(now, unusableRun{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("staleVerdict = %v, want it to match %v", err, tt.want)
			}
		})
	}
}

// TestStreamHealthIdleScreenStaysHealthy drives the poll sequence a still
// screen produces: the stream stays up and the same frame comes back every
// time. That must never be read as a failure, however long it lasts.
func TestStreamHealthIdleScreenStaysHealthy(t *testing.T) {
	start := time.Now()
	var h streamHealth

	for i := 1; i <= 600; i++ {
		now := start.Add(time.Duration(i) * time.Second)
		h.observe(true, now)
		if err := h.staleVerdict(now, unusableRun{}); !errors.Is(err, ErrNoNewFrame) {
			t.Fatalf("after %ds idle: staleVerdict = %v, want ErrNoNewFrame", i, err)
		}
	}
}

// TestStreamHealthStoppedStreamFails drives what a revoked screen share looks
// like: the stream leaves the streaming state while the producer is already
// quiet, so nothing but that state change separates it from an idle screen.
// The grace period has to start there too, not at the last frame, or a
// renegotiation after a still screen dies on its first poll.
func TestStreamHealthStoppedStreamFails(t *testing.T) {
	start := time.Now()
	var h streamHealth

	for i := 1; i <= 10; i++ {
		now := start.Add(time.Duration(i) * time.Second)
		h.observe(true, now)
		if err := h.staleVerdict(now, unusableRun{}); !errors.Is(err, ErrNoNewFrame) {
			t.Fatalf("idle poll %d: staleVerdict = %v, want ErrNoNewFrame", i, err)
		}
	}

	stop := start.Add(11 * time.Second)
	h.observe(false, stop)
	if err := h.staleVerdict(stop, unusableRun{}); !errors.Is(err, ErrNoNewFrame) {
		t.Fatalf("the instant the stream stopped: %v, want its full grace", err)
	}

	late := stop.Add(streamStallDeadline)
	h.observe(false, late)
	err := h.staleVerdict(late, unusableRun{})
	if !errors.Is(err, ErrStreamFailed) {
		t.Fatalf("staleVerdict = %v, want ErrStreamFailed", err)
	}
	if !strings.Contains(err.Error(), "stopped streaming") {
		t.Errorf("err = %v, want it to name the stream state", err)
	}
}

// TestStreamHealthRenegotiationSurvivesAnIdleScreen is the false positive this
// has to avoid. A resolution change pauses the stream, and if the screen is
// still afterwards no frame follows to certify recovery, so returning to the
// streaming state has to be enough on its own.
func TestStreamHealthRenegotiationSurvivesAnIdleScreen(t *testing.T) {
	start := time.Now()
	var h streamHealth

	h.observe(true, start)

	pause := start.Add(time.Hour)
	h.observe(false, pause)
	h.observe(true, pause.Add(300*time.Millisecond))

	for i := 1; i <= 600; i++ {
		now := pause.Add(time.Duration(i) * time.Second)
		h.observe(true, now)
		if err := h.staleVerdict(now, unusableRun{}); !errors.Is(err, ErrNoNewFrame) {
			t.Fatalf("%ds after the renegotiation: staleVerdict = %v, want ErrNoNewFrame", i, err)
		}
	}
}

// TestIsPermissionDenied separates the user declining the screen-share dialog,
// which nothing should ask about again, from the portal failing, which is worth
// retrying. It has to see through the wrapping Start adds on the way up.
func TestIsPermissionDenied(t *testing.T) {
	denied := &PortalError{Type: "permission_denied", Msg: "declined"}
	failed := &PortalError{Type: "request_failed", Msg: "code 2"}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"declined", denied, true},
		{"declined, wrapped twice", fmt.Errorf("start: %w", fmt.Errorf("select sources: %w", denied)), true},
		{"portal failed", fmt.Errorf("start: %w", failed), false},
		{"other error", errors.New("bus gone"), false},
		{"no error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermissionDenied(tt.err); got != tt.want {
				t.Errorf("IsPermissionDenied = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnusableRunVerdict(t *testing.T) {
	now := time.Now()
	const reason = "buffer memory is not mapped"

	tests := []struct {
		name string
		run  unusableRun
		want error
	}{
		{"no run", unusableRun{}, ErrNoNewFrame},
		{"renegotiation burst", unusableRun{count: 50, span: 100 * time.Millisecond, reason: reason}, ErrNoNewFrame},
		{"stray drops on a still screen", unusableRun{count: 2, span: 10 * time.Minute, reason: reason}, ErrNoNewFrame},
		{"one drop short of the limit", unusableRun{count: unusableRunLimit - 1, span: time.Hour, reason: reason}, ErrNoNewFrame},
		{"just short of the span", unusableRun{count: 1000, span: unusableRunSpan - time.Nanosecond, reason: reason}, ErrNoNewFrame},
		{"every buffer unusable", unusableRun{count: unusableRunLimit, span: unusableRunSpan, reason: reason}, ErrStreamFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h streamHealth
			h.observe(true, now)
			err := h.staleVerdict(now, tt.run)
			if !errors.Is(err, tt.want) {
				t.Fatalf("staleVerdict = %v, want it to match %v", err, tt.want)
			}
			if tt.want == ErrStreamFailed && !strings.Contains(err.Error(), reason) {
				t.Errorf("error %q does not name the drop reason", err)
			}
		})
	}
}
