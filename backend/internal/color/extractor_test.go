package color

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// TestNewExtractor tests the Extractor constructor
func TestNewExtractor(t *testing.T) {
	tests := []struct {
		name           string
		subsampleWidth int
		gamma          float64
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Valid parameters",
			subsampleWidth: 64,
			gamma:          2.2,
			expectError:    false,
		},
		{
			name:           "Minimum subsample width",
			subsampleWidth: 16,
			gamma:          2.2,
			expectError:    false,
		},
		{
			name:           "Maximum subsample width",
			subsampleWidth: 256,
			gamma:          2.2,
			expectError:    false,
		},
		{
			name:           "Subsample width too small",
			subsampleWidth: 15,
			gamma:          2.2,
			expectError:    true,
			errorContains:  "must be between 16 and 256",
		},
		{
			name:           "Subsample width too large",
			subsampleWidth: 257,
			gamma:          2.2,
			expectError:    true,
			errorContains:  "must be between 16 and 256",
		},
		{
			name:           "Zero gamma",
			subsampleWidth: 64,
			gamma:          0,
			expectError:    true,
			errorContains:  "gamma must be positive",
		},
		{
			name:           "Negative gamma",
			subsampleWidth: 64,
			gamma:          -1.0,
			expectError:    true,
			errorContains:  "gamma must be positive",
		},
		{
			name:           "Minimum valid gamma",
			subsampleWidth: 64,
			gamma:          0.1,
			expectError:    false,
		},
		{
			name:           "Large gamma",
			subsampleWidth: 64,
			gamma:          10.0,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := NewExtractor(tt.subsampleWidth, tt.gamma)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if ext == nil {
					t.Errorf("Expected extractor but got nil")
				} else {
					if ext.subsampleWidth != tt.subsampleWidth {
						t.Errorf("Expected subsampleWidth %d, got %d", tt.subsampleWidth, ext.subsampleWidth)
					}
					if ext.gammaCorrection != tt.gamma {
						t.Errorf("Expected gamma %f, got %f", tt.gamma, ext.gammaCorrection)
					}
				}
			}
		})
	}
}

// TestExtractColors_NilImage tests that ExtractColors handles nil images
func TestExtractColors_NilImage(t *testing.T) {
	ext, err := NewExtractor(64, 2.2)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	zones := []Zone{{ID: 0, U1: 0, V1: 0, U2: 1, V2: 1}}
	_, err = ext.ExtractColors(nil, zones)

	if err == nil {
		t.Error("Expected error for nil image")
	}
	if !strings.Contains(err.Error(), "image is nil") {
		t.Errorf("Expected 'image is nil' error, got: %v", err)
	}
}

// TestExtractColors_EmptyZones tests extracting from zero zones
func TestExtractColors_EmptyZones(t *testing.T) {
	ext, err := NewExtractor(64, 2.2)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	img := createSolidColorImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	colors, err := ext.ExtractColors(img, []Zone{})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(colors) != 0 {
		t.Errorf("Expected 0 colors, got %d", len(colors))
	}
}

// TestExtractColors_FullScreen tests extracting the entire screen
func TestExtractColors_FullScreen(t *testing.T) {
	ext, err := NewExtractor(64, 1.0) // Use gamma=1.0 to simplify testing
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	// Create a solid red image
	img := createSolidColorImage(100, 100, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	zones := []Zone{
		{ID: 0, U1: 0, V1: 0, U2: 1, V2: 1, Name: "Full Screen"},
	}

	colors, err := ext.ExtractColors(img, zones)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(colors) != 1 {
		t.Fatalf("Expected 1 color, got %d", len(colors))
	}

	// Check zone ID
	if colors[0].ZoneID != 0 {
		t.Errorf("Expected zone ID 0, got %d", colors[0].ZoneID)
	}

	// Check color values (should be close to original)
	if !colorClose(colors[0].R, 200, 5) {
		t.Errorf("Expected R ~200, got %d", colors[0].R)
	}
	if !colorClose(colors[0].G, 100, 5) {
		t.Errorf("Expected G ~100, got %d", colors[0].G)
	}
	if !colorClose(colors[0].B, 50, 5) {
		t.Errorf("Expected B ~50, got %d", colors[0].B)
	}
}

// TestExtractColors_MultipleZones tests extracting from multiple zones
func TestExtractColors_MultipleZones(t *testing.T) {
	ext, err := NewExtractor(64, 1.0)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	// Create an image with different colored quadrants
	img := createQuadrantImage(200, 200)

	zones := []Zone{
		{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 0.5, Name: "Top-Left (Red)"},
		{ID: 1, U1: 0.5, V1: 0.0, U2: 1.0, V2: 0.5, Name: "Top-Right (Green)"},
		{ID: 2, U1: 0.0, V1: 0.5, U2: 0.5, V2: 1.0, Name: "Bottom-Left (Blue)"},
		{ID: 3, U1: 0.5, V1: 0.5, U2: 1.0, V2: 1.0, Name: "Bottom-Right (White)"},
	}

	colors, err := ext.ExtractColors(img, zones)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(colors) != 4 {
		t.Fatalf("Expected 4 colors, got %d", len(colors))
	}

	// Top-left should be red
	if !colorClose(colors[0].R, 255, 10) || !colorClose(colors[0].G, 0, 10) || !colorClose(colors[0].B, 0, 10) {
		t.Errorf("Zone 0 expected red, got RGB(%d,%d,%d)", colors[0].R, colors[0].G, colors[0].B)
	}

	// Top-right should be green
	if !colorClose(colors[1].R, 0, 10) || !colorClose(colors[1].G, 255, 10) || !colorClose(colors[1].B, 0, 10) {
		t.Errorf("Zone 1 expected green, got RGB(%d,%d,%d)", colors[1].R, colors[1].G, colors[1].B)
	}

	// Bottom-left should be blue
	if !colorClose(colors[2].R, 0, 10) || !colorClose(colors[2].G, 0, 10) || !colorClose(colors[2].B, 255, 10) {
		t.Errorf("Zone 2 expected blue, got RGB(%d,%d,%d)", colors[2].R, colors[2].G, colors[2].B)
	}

	// Bottom-right should be white
	if !colorClose(colors[3].R, 255, 10) || !colorClose(colors[3].G, 255, 10) || !colorClose(colors[3].B, 255, 10) {
		t.Errorf("Zone 3 expected white, got RGB(%d,%d,%d)", colors[3].R, colors[3].G, colors[3].B)
	}
}

// TestExtractColors_EdgeCases tests edge cases in zone extraction
func TestExtractColors_EdgeCases(t *testing.T) {
	ext, err := NewExtractor(64, 1.0)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	img := createSolidColorImage(100, 100, color.RGBA{R: 128, G: 128, B: 128, A: 255})

	tests := []struct {
		name string
		zone Zone
	}{
		{
			name: "Zone outside bounds (clamped)",
			zone: Zone{ID: 0, U1: 1.5, V1: 1.5, U2: 2.0, V2: 2.0},
		},
		{
			name: "Zone partially outside bounds",
			zone: Zone{ID: 1, U1: 0.8, V1: 0.8, U2: 1.2, V2: 1.2},
		},
		{
			name: "Very small zone",
			zone: Zone{ID: 2, U1: 0.5, V1: 0.5, U2: 0.51, V2: 0.51},
		},
		{
			name: "Inverted coordinates (U2 < U1)",
			zone: Zone{ID: 3, U1: 0.8, V1: 0.8, U2: 0.2, V2: 0.2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colors, err := ext.ExtractColors(img, []Zone{tt.zone})
			// Should not error - clamping handles edge cases
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(colors) != 1 {
				t.Errorf("Expected 1 color, got %d", len(colors))
			}
		})
	}
}

// TestSubsampleImage tests image subsampling
func TestSubsampleImage(t *testing.T) {
	tests := []struct {
		name           string
		subsampleWidth int
		origWidth      int
		origHeight     int
	}{
		{
			name:           "Downsample large image",
			subsampleWidth: 64,
			origWidth:      1920,
			origHeight:     1080,
		},
		{
			name:           "Already small image (no change)",
			subsampleWidth: 64,
			origWidth:      50,
			origHeight:     50,
		},
		{
			name:           "Exact match",
			subsampleWidth: 64,
			origWidth:      64,
			origHeight:     64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := NewExtractor(tt.subsampleWidth, 2.2)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			img := createSolidColorImage(tt.origWidth, tt.origHeight, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			subsampled := ext.subsampleImage(img)

			bounds := subsampled.Bounds()
			newWidth := bounds.Dx()
			newHeight := bounds.Dy()

			// If original is smaller or equal, should not change
			if tt.origWidth <= tt.subsampleWidth {
				if newWidth != tt.origWidth || newHeight != tt.origHeight {
					t.Errorf("Expected size %dx%d (no change), got %dx%d", tt.origWidth, tt.origHeight, newWidth, newHeight)
				}
			} else {
				// Should be downsampled to target width
				if newWidth != tt.subsampleWidth {
					t.Errorf("Expected width %d, got %d", tt.subsampleWidth, newWidth)
				}

				// Check aspect ratio is maintained (within rounding)
				expectedHeight := int(float64(tt.subsampleWidth) * float64(tt.origHeight) / float64(tt.origWidth))
				if abs(newHeight-expectedHeight) > 1 {
					t.Errorf("Expected height ~%d, got %d (aspect ratio not maintained)", expectedHeight, newHeight)
				}
			}
		})
	}
}

// TestCalculateMeanColor tests the mean color calculation
func TestCalculateMeanColor(t *testing.T) {
	ext, _ := NewExtractor(64, 1.0)

	tests := []struct {
		name      string
		img       image.Image
		expectedR uint8
		expectedG uint8
		expectedB uint8
	}{
		{
			name:      "Solid red",
			img:       createSolidColorImage(10, 10, color.RGBA{R: 200, G: 0, B: 0, A: 255}),
			expectedR: 200,
			expectedG: 0,
			expectedB: 0,
		},
		{
			name:      "Solid gray",
			img:       createSolidColorImage(10, 10, color.RGBA{R: 128, G: 128, B: 128, A: 255}),
			expectedR: 128,
			expectedG: 128,
			expectedB: 128,
		},
		{
			name:      "Two-tone image (should average)",
			img:       createHalfColorImage(10, 10, color.RGBA{R: 0, G: 0, B: 0, A: 255}, color.RGBA{R: 255, G: 255, B: 255, A: 255}),
			expectedR: 127, // Average of 0 and 255
			expectedG: 127,
			expectedB: 127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := ext.calculateMeanColor(tt.img)

			if !colorClose(r, tt.expectedR, 2) {
				t.Errorf("Expected R ~%d, got %d", tt.expectedR, r)
			}
			if !colorClose(g, tt.expectedG, 2) {
				t.Errorf("Expected G ~%d, got %d", tt.expectedG, g)
			}
			if !colorClose(b, tt.expectedB, 2) {
				t.Errorf("Expected B ~%d, got %d", tt.expectedB, b)
			}
		})
	}
}

// TestApplyGamma tests gamma correction
func TestApplyGamma(t *testing.T) {
	tests := []struct {
		name      string
		gamma     float64
		input     uint8
		expected  uint8
		tolerance uint8
	}{
		{
			name:      "Gamma 1.0 (no change)",
			gamma:     1.0,
			input:     128,
			expected:  128,
			tolerance: 1,
		},
		{
			name:      "Gamma 2.2 (standard)",
			gamma:     2.2,
			input:     128,
			expected:  186, // 128 with gamma 1/2.2 ≈ 186
			tolerance: 2,
		},
		{
			name:      "Gamma 2.2 with black",
			gamma:     2.2,
			input:     0,
			expected:  0,
			tolerance: 0,
		},
		{
			name:      "Gamma 2.2 with white",
			gamma:     2.2,
			input:     255,
			expected:  255,
			tolerance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := NewExtractor(64, tt.gamma)
			if err != nil {
				t.Fatalf("Failed to create extractor: %v", err)
			}

			result := ext.applyGamma(tt.input)

			if !colorClose(result, tt.expected, tt.tolerance) {
				t.Errorf("Expected ~%d, got %d", tt.expected, result)
			}
		})
	}
}

// Helper functions

func createSolidColorImage(width, height int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func createQuadrantImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	halfW := width / 2
	halfH := height / 2

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var c color.RGBA
			if x < halfW && y < halfH {
				c = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red
			} else if x >= halfW && y < halfH {
				c = color.RGBA{R: 0, G: 255, B: 0, A: 255} // Green
			} else if x < halfW && y >= halfH {
				c = color.RGBA{R: 0, G: 0, B: 255, A: 255} // Blue
			} else {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255} // White
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func createHalfColorImage(width, height int, c1, c2 color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	half := width / 2

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < half {
				img.Set(x, y, c1)
			} else {
				img.Set(x, y, c2)
			}
		}
	}
	return img
}

func colorClose(a, b, tolerance uint8) bool {
	diff := int(a) - int(b)
	if diff < 0 {
		diff = -diff
	}
	return diff <= int(tolerance)
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
