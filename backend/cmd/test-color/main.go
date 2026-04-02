package main

import (
	"fmt"
	"log"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/color"
)

func main() {
	fmt.Println("Color Extraction Test - Milestone 2")
	fmt.Println("====================================\n")

	// Step 1: Get a test frame
	fmt.Println("📸 Generating test frame...")
	cap, err := capture.NewScreenCapture(capture.Config{FPS: 30, Monitor: -1})
	if err != nil {
		log.Fatalf("Failed to create capture: %v", err)
	}
	defer cap.Stop()

	frame, err := cap.CaptureFrame()
	if err != nil {
		log.Fatalf("Failed to capture frame: %v", err)
	}
	fmt.Printf("✅ Frame: %dx%d\n\n", frame.Bounds().Dx(), frame.Bounds().Dy())

	// Step 2: Define test zones
	// Simulate a typical 3-light setup: left, center, right
	zones := []color.Zone{
		{
			ID:   1,
			U1:   0.0,  // Left edge
			V1:   0.0,  // Top
			U2:   0.33, // 1/3 width
			V2:   1.0,  // Bottom
			Name: "Left Light",
		},
		{
			ID:   2,
			U1:   0.33, // 1/3 width
			V1:   0.0,  // Top
			U2:   0.66, // 2/3 width
			V2:   1.0,  // Bottom
			Name: "Center Light",
		},
		{
			ID:   3,
			U1:   0.66, // 2/3 width
			V1:   0.0,  // Top
			U2:   1.0,  // Right edge
			V2:   1.0,  // Bottom
			Name: "Right Light",
		},
	}

	fmt.Println("🎯 Test zones:")
	for _, zone := range zones {
		fmt.Printf("  Zone %d (%s): UV=(%.2f,%.2f)-(%.2f,%.2f)\n",
			zone.ID, zone.Name, zone.U1, zone.V1, zone.U2, zone.V2)
	}
	fmt.Println()

	// Step 3: Create color extractor
	// Using 64px subsample width and gamma 2.2 (typical display gamma)
	fmt.Println("🎨 Creating color extractor...")
	extractor, err := color.NewExtractor(64, 2.2)
	if err != nil {
		log.Fatalf("Failed to create extractor: %v", err)
	}
	fmt.Println("   Subsample width: 64px")
	fmt.Println("   Gamma correction: 2.2")
	fmt.Println()

	// Step 4: Extract colors
	fmt.Println("⚡ Extracting colors from zones...")
	colors, err := extractor.ExtractColors(frame, zones)
	if err != nil {
		log.Fatalf("Failed to extract colors: %v", err)
	}

	// Step 5: Display results
	fmt.Println("✅ Colors extracted!\n")
	fmt.Println("Results:")
	fmt.Println("--------")
	for i, zoneColor := range colors {
		zone := zones[i]
		fmt.Printf("Zone %d (%s):\n", zone.ID, zone.Name)
		fmt.Printf("  RGB: (%3d, %3d, %3d)\n", zoneColor.R, zoneColor.G, zoneColor.B)
		fmt.Printf("  Hex: #%02X%02X%02X\n", zoneColor.R, zoneColor.G, zoneColor.B)
		fmt.Printf("  %s\n\n", colorBar(zoneColor.R, zoneColor.G, zoneColor.B))
	}

	// Step 6: Performance test
	fmt.Println("🔥 Performance test (100 extractions)...")
	iterations := 100
	
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := extractor.ExtractColors(frame, zones)
		if err != nil {
			log.Fatalf("Extraction failed: %v", err)
		}
	}
	elapsed := time.Since(start)
	
	avgTime := elapsed / time.Duration(iterations)
	fps := float64(time.Second) / float64(avgTime)
	
	fmt.Printf("✅ %d iterations in %v\n", iterations, elapsed)
	fmt.Printf("   Average: %v per extraction\n", avgTime)
	fmt.Printf("   Max FPS: %.1f\n", fps)
	
	if fps >= 30 {
		fmt.Println("   ✅ Performance: Excellent (can support 30+ FPS)")
	} else if fps >= 15 {
		fmt.Println("   ⚠️  Performance: Acceptable (15-30 FPS)")
	} else {
		fmt.Println("   ❌ Performance: Poor (<15 FPS)")
	}
}

// colorBar returns a visual representation of the color
func colorBar(r, g, b uint8) string {
	// Determine dominant channel
	dominant := "Mixed"
	if r > g && r > b {
		dominant = "Red"
	} else if g > r && g > b {
		dominant = "Green"
	} else if b > r && b > g {
		dominant = "Blue"
	}
	
	// Create visual bar
	bar := "█████████████████████"
	return fmt.Sprintf("  Dominant: %s  %s", dominant, bar)
}
