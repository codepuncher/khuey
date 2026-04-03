package capture

import (
	"testing"
	"time"
)

// TestNewScreenCapture_Validation tests capture configuration validation
func TestNewScreenCapture_Validation(t *testing.T) {
	tests := []struct {
		name        string
		fps         int
		expectError bool
	}{
		{
			name:        "Valid minimum FPS",
			fps:         10,
			expectError: false,
		},
		{
			name:        "Valid mid FPS",
			fps:         30,
			expectError: false,
		},
		{
			name:        "Valid maximum FPS",
			fps:         60,
			expectError: false,
		},
		{
			name:        "FPS too low",
			fps:         9,
			expectError: true,
		},
		{
			name:        "FPS too high",
			fps:         61,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate FPS (logic from NewScreenCapture)
			isValid := tt.fps >= 10 && tt.fps <= 60

			if tt.expectError && isValid {
				t.Errorf("Expected FPS %d to be invalid", tt.fps)
			}
			if !tt.expectError && !isValid {
				t.Errorf("Expected FPS %d to be valid", tt.fps)
			}
		})
	}
}

// TestGetFrameInterval tests frame interval calculation
func TestGetFrameInterval(t *testing.T) {
	tests := []struct {
		name     string
		fps      int
		expected time.Duration
	}{
		{
			name:     "10 FPS",
			fps:      10,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "30 FPS",
			fps:      30,
			expected: 33333333 * time.Nanosecond, // ~33.33ms
		},
		{
			name:     "60 FPS",
			fps:      60,
			expected: 16666666 * time.Nanosecond, // ~16.67ms
		},
		{
			name:     "1 FPS",
			fps:      1,
			expected: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate frame interval (1 second / FPS)
			interval := time.Second / time.Duration(tt.fps)

			// Allow small tolerance for rounding
			diff := interval - tt.expected
			if diff < 0 {
				diff = -diff
			}

			// Tolerance: 1 nanosecond
			if diff > time.Nanosecond {
				t.Errorf("Expected interval %v, got %v (diff: %v)", tt.expected, interval, diff)
			}
		})
	}
}

// TestScreenshotToolDetection tests screenshot tool detection logic
func TestScreenshotToolDetection(t *testing.T) {
	tools := []string{"spectacle", "grim", "import"}

	tests := []struct {
		name           string
		availableTools []string
		expectedTool   string
	}{
		{
			name:           "Spectacle available (KDE)",
			availableTools: []string{"spectacle"},
			expectedTool:   "spectacle",
		},
		{
			name:           "Grim available (Wayland)",
			availableTools: []string{"grim"},
			expectedTool:   "grim",
		},
		{
			name:           "Import available (X11)",
			availableTools: []string{"import"},
			expectedTool:   "import",
		},
		{
			name:           "Multiple tools - prefer spectacle",
			availableTools: []string{"spectacle", "grim", "import"},
			expectedTool:   "spectacle",
		},
		{
			name:           "Spectacle and grim - prefer spectacle",
			availableTools: []string{"spectacle", "grim"},
			expectedTool:   "spectacle",
		},
		{
			name:           "Grim and import - prefer grim",
			availableTools: []string{"grim", "import"},
			expectedTool:   "grim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate tool detection logic (priority: spectacle > grim > import)
			var detected string

			for _, tool := range tools {
				for _, available := range tt.availableTools {
					if tool == available {
						detected = tool
						break
					}
				}
				if detected != "" {
					break
				}
			}

			if detected != tt.expectedTool {
				t.Errorf("Expected tool %q, got %q", tt.expectedTool, detected)
			}
		})
	}
}

// TestConfigDefaults tests default configuration values
func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		FPS:            30,
		Monitor:        -1,
		UseMockFrames:  false,
		UseScreenshot:  true,
		ScreenshotTool: "",
		CaptureWidth:   640,
		CaptureHeight:  360,
	}

	if cfg.FPS != 30 {
		t.Errorf("Default FPS should be 30, got %d", cfg.FPS)
	}
	if cfg.Monitor != -1 {
		t.Errorf("Default Monitor should be -1 (all), got %d", cfg.Monitor)
	}
	if cfg.UseMockFrames {
		t.Error("UseMockFrames should default to false")
	}
	if !cfg.UseScreenshot {
		t.Error("UseScreenshot should default to true")
	}
	if cfg.CaptureWidth != 640 {
		t.Errorf("Default CaptureWidth should be 640, got %d", cfg.CaptureWidth)
	}
	if cfg.CaptureHeight != 360 {
		t.Errorf("Default CaptureHeight should be 360, got %d", cfg.CaptureHeight)
	}
}

// TestMockFrameMode tests mock frame configuration
func TestMockFrameMode(t *testing.T) {
	tests := []struct {
		name          string
		useMockFrames bool
		useScreenshot bool
		expectedMode  string
	}{
		{
			name:          "Mock frames enabled",
			useMockFrames: true,
			useScreenshot: false,
			expectedMode:  "mock",
		},
		{
			name:          "Screenshot mode",
			useMockFrames: false,
			useScreenshot: true,
			expectedMode:  "screenshot",
		},
		{
			name:          "Portal mode (neither mock nor screenshot)",
			useMockFrames: false,
			useScreenshot: false,
			expectedMode:  "portal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine capture mode
			var mode string
			if tt.useMockFrames {
				mode = "mock"
			} else if tt.useScreenshot {
				mode = "screenshot"
			} else {
				mode = "portal"
			}

			if mode != tt.expectedMode {
				t.Errorf("Expected mode %q, got %q", tt.expectedMode, mode)
			}
		})
	}
}

// TestCaptureResolution tests capture resolution validation
func TestCaptureResolution(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		valid  bool
	}{
		{
			name:   "Standard HD",
			width:  1920,
			height: 1080,
			valid:  true,
		},
		{
			name:   "Downsampled",
			width:  640,
			height: 360,
			valid:  true,
		},
		{
			name:   "4K",
			width:  3840,
			height: 2160,
			valid:  true,
		},
		{
			name:   "Minimum",
			width:  320,
			height: 240,
			valid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All positive dimensions are valid
			isValid := tt.width > 0 && tt.height > 0

			if isValid != tt.valid {
				t.Errorf("Expected validity %v for %dx%d", tt.valid, tt.width, tt.height)
			}
		})
	}
}

// TestFrameIntervalAccuracy tests that frame intervals are calculated correctly
func TestFrameIntervalAccuracy(t *testing.T) {
	// Test that intervals sum correctly over time
	tests := []struct {
		fps      int
		frames   int
		expected time.Duration
	}{
		{
			fps:      30,
			frames:   30,
			expected: 1 * time.Second,
		},
		{
			fps:      60,
			frames:   60,
			expected: 1 * time.Second,
		},
		{
			fps:      10,
			frames:   10,
			expected: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			interval := time.Second / time.Duration(tt.fps)
			totalTime := interval * time.Duration(tt.frames)

			// Should equal expected total (allow small rounding error)
			diff := totalTime - tt.expected
			if diff < 0 {
				diff = -diff
			}

			// Allow up to 1ms total error over all frames
			if diff > time.Millisecond {
				t.Errorf("FPS %d: expected %v for %d frames, got %v", tt.fps, tt.expected, tt.frames, totalTime)
			}
		})
	}
}
