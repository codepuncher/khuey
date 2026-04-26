// Performance regression tests for color extraction
// These tests ensure performance doesn't regress below acceptable thresholds
// Run with: go test -v -run TestPerformanceRegression
package color

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// TestPerformanceRegression_ColorExtraction validates color extraction performance
func TestPerformanceRegression_ColorExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	// Performance threshold: extraction must complete in <12ms for typical 2-zone setup
	// This is based on PR #42 optimization results (11ms average)
	const maxExtractTime = 12 * time.Millisecond

	// Create 1440p test image (typical monitor resolution)
	img := createPerfTestImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	// Two zones (left and right halves) - typical setup
	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0}, // Left half
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0}, // Right half
	}

	extractor, err := NewExtractor(64, 2.2)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	// Warm-up run
	_, err = extractor.ExtractColors(img, zones)
	if err != nil {
		t.Fatalf("Warmup extraction failed: %v", err)
	}

	// Measure extraction time over multiple iterations
	const iterations = 100
	var totalTime time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := extractor.ExtractColors(img, zones)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Extraction failed on iteration %d: %v", i, err)
		}

		totalTime += elapsed
	}

	avgTime := totalTime / iterations

	t.Logf("Color Extraction Performance (n=%d):", iterations)
	t.Logf("  Resolution: 2560x1440")
	t.Logf("  Zones: 2")
	t.Logf("  Subsample width: 64")
	t.Logf("  Average time: %v", avgTime)
	t.Logf("  Threshold: %v", maxExtractTime)
	t.Logf("  Max FPS theoretical: %.0f", float64(time.Second)/float64(avgTime))

	if avgTime > maxExtractTime {
		t.Errorf("PERFORMANCE REGRESSION: Average extraction time %v exceeds threshold %v",
			avgTime, maxExtractTime)
		t.Errorf("This indicates a performance regression from PR #42 optimization (target: 11ms)")
	}
}

// TestPerformanceRegression_FourZones validates performance doesn't degrade with more zones
func TestPerformanceRegression_FourZones(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	// Four zones should still complete in <18ms
	const maxExtractTime = 18 * time.Millisecond

	img := createPerfTestImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	// Four zones (quadrants)
	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 0.5}, // Top-left
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 0.5}, // Top-right
		{U1: 0.0, V1: 0.5, U2: 0.5, V2: 1.0}, // Bottom-left
		{U1: 0.5, V1: 0.5, U2: 1.0, V2: 1.0}, // Bottom-right
	}

	extractor, err := NewExtractor(64, 2.2)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	// Measure
	const iterations = 100
	var totalTime time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := extractor.ExtractColors(img, zones)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Extraction failed: %v", err)
		}

		totalTime += elapsed
	}

	avgTime := totalTime / iterations

	t.Logf("Four-Zone Extraction Performance:")
	t.Logf("  Zones: 4")
	t.Logf("  Average time: %v", avgTime)
	t.Logf("  Threshold: %v", maxExtractTime)

	if avgTime > maxExtractTime {
		t.Errorf("PERFORMANCE REGRESSION: Four-zone extraction %v exceeds threshold %v",
			avgTime, maxExtractTime)
	}
}

// TestPerformanceRegression_4KResolution validates 4K performance
func TestPerformanceRegression_4KResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	// 4K should complete in <20ms (higher resolution = slightly slower, but still fast)
	const maxExtractTime = 20 * time.Millisecond

	img := createPerfTestImage(3840, 2160, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
	}

	extractor, err := NewExtractor(64, 2.2)
	if err != nil {
		t.Fatalf("Failed to create extractor: %v", err)
	}

	const iterations = 50 // Fewer iterations for 4K (larger images)
	var totalTime time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := extractor.ExtractColors(img, zones)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Extraction failed: %v", err)
		}

		totalTime += elapsed
	}

	avgTime := totalTime / iterations

	t.Logf("4K Resolution Performance:")
	t.Logf("  Resolution: 3840x2160")
	t.Logf("  Average time: %v", avgTime)
	t.Logf("  Threshold: %v", maxExtractTime)

	if avgTime > maxExtractTime {
		t.Errorf("PERFORMANCE REGRESSION: 4K extraction %v exceeds threshold %v",
			avgTime, maxExtractTime)
	}
}

// Helper function to create test images for performance tests
func createPerfTestImage(width, height int, fillColor color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Create gradient for more realistic testing
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * int(fillColor.R)) / width)
			g := uint8((y * int(fillColor.G)) / height)
			b := fillColor.B
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}
