package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	"github.com/codepuncher/khuey/internal/capture"
	hucolor "github.com/codepuncher/khuey/internal/color"
)

func main() {
	fmt.Println("Visual Zone Test")
	fmt.Println("================")
	fmt.Println()

	// Get test frame
	cap, _ := capture.NewScreenCapture(capture.Config{FPS: 30, Monitor: -1})
	defer cap.Stop()

	frame, _ := cap.CaptureFrame()
	fmt.Printf("📸 Frame: %dx%d\n", frame.Bounds().Dx(), frame.Bounds().Dy())

	// Define zones
	zones := []hucolor.Zone{
		{ID: 1, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0, Name: "Left"},
		{ID: 2, U1: 0.33, V1: 0.0, U2: 0.66, V2: 1.0, Name: "Center"},
		{ID: 3, U1: 0.66, V1: 0.0, U2: 1.0, V2: 1.0, Name: "Right"},
	}

	// Extract colors
	extractor, _ := hucolor.NewExtractor(64, 2.2)
	colors, err := extractor.ExtractColors(frame, zones)
	if err != nil {
		log.Fatal(err)
	}

	// Create visualization
	viz := visualizeZones(frame, zones, colors)

	// Save to file
	outFile := "/tmp/zone-visualization.png"
	f, err := os.Create(outFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	if err := png.Encode(f, viz); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n✅ Visualization saved to: %s\n", outFile)
	fmt.Println("\nZone colors:")
	for _, c := range colors {
		fmt.Printf("  Zone %d: RGB(%3d, %3d, %3d) #%02X%02X%02X\n",
			c.ZoneID, c.R, c.G, c.B, c.R, c.G, c.B)
	}

	fmt.Println("\nOpen the image with:")
	fmt.Println("  xdg-open /tmp/zone-visualization.png")
}

func visualizeZones(frame image.Image, zones []hucolor.Zone, colors []hucolor.ZoneColor) image.Image {
	bounds := frame.Bounds()
	viz := image.NewRGBA(bounds)

	// Draw original frame
	draw.Draw(viz, bounds, frame, bounds.Min, draw.Src)

	// Draw zone boundaries and labels
	for i, zone := range zones {
		c := colors[i]

		// Calculate pixel coordinates
		x1 := int(zone.U1 * float64(bounds.Dx()))
		y1 := int(zone.V1 * float64(bounds.Dy()))
		x2 := int(zone.U2 * float64(bounds.Dx()))
		y2 := int(zone.V2 * float64(bounds.Dy()))

		// Draw border (white lines)
		borderColor := color.RGBA{255, 255, 255, 255}
		drawRect(viz, x1, y1, x2, y2, borderColor, 3)

		// Draw color swatch in corner of zone
		swatchSize := 60
		swatchX := x1 + 10
		swatchY := y1 + 10
		swatchColor := color.RGBA{c.R, c.G, c.B, 255}
		fillRect(viz, swatchX, swatchY, swatchX+swatchSize, swatchY+swatchSize, swatchColor)
		drawRect(viz, swatchX, swatchY, swatchX+swatchSize, swatchY+swatchSize, borderColor, 2)
	}

	return viz
}

func drawRect(img *image.RGBA, x1, y1, x2, y2 int, c color.Color, thickness int) {
	// Top
	for x := x1; x <= x2; x++ {
		for t := 0; t < thickness; t++ {
			img.Set(x, y1+t, c)
		}
	}
	// Bottom
	for x := x1; x <= x2; x++ {
		for t := 0; t < thickness; t++ {
			img.Set(x, y2-t, c)
		}
	}
	// Left
	for y := y1; y <= y2; y++ {
		for t := 0; t < thickness; t++ {
			img.Set(x1+t, y, c)
		}
	}
	// Right
	for y := y1; y <= y2; y++ {
		for t := 0; t < thickness; t++ {
			img.Set(x2-t, y, c)
		}
	}
}

func fillRect(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			img.Set(x, y, c)
		}
	}
}
