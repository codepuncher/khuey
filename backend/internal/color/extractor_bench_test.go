// Benchmark tests for color extraction
// Run with: go test -bench=. -benchmem -benchtime=10s
package color

import (
	"image"
	"image/color"
	"testing"
)

// BenchmarkExtractColors_TwoZones_1440p benchmarks typical setup
func BenchmarkExtractColors_TwoZones_1440p(b *testing.B) {
	img := createBenchImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0}, // Left half
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0}, // Right half
	}

	extractor, err := NewExtractor(64, 2.2)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractColors(img, zones)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractColors_FourZones_1440p benchmarks four-zone setup
func BenchmarkExtractColors_FourZones_1440p(b *testing.B) {
	img := createBenchImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 0.5}, // Top-left
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 0.5}, // Top-right
		{U1: 0.0, V1: 0.5, U2: 0.5, V2: 1.0}, // Bottom-left
		{U1: 0.5, V1: 0.5, U2: 1.0, V2: 1.0}, // Bottom-right
	}

	extractor, err := NewExtractor(64, 2.2)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractColors(img, zones)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractColors_TwoZones_4K benchmarks 4K resolution
func BenchmarkExtractColors_TwoZones_4K(b *testing.B) {
	img := createBenchImage(3840, 2160, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
	}

	extractor, err := NewExtractor(64, 2.2)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractColors(img, zones)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractColors_SubsampleWidth benchmarks different subsample widths
func BenchmarkExtractColors_Subsample32(b *testing.B) {
	benchmarkSubsample(b, 32)
}

func BenchmarkExtractColors_Subsample64(b *testing.B) {
	benchmarkSubsample(b, 64)
}

func BenchmarkExtractColors_Subsample128(b *testing.B) {
	benchmarkSubsample(b, 128)
}

func benchmarkSubsample(b *testing.B, subsampleWidth int) {
	img := createBenchImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

	zones := []Zone{
		{U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		{U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
	}

	extractor, err := NewExtractor(subsampleWidth, 2.2)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractColors(img, zones)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function
func createBenchImage(width, height int, fillColor color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
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
