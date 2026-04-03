package main

import (
"fmt"
"log"
"time"

"github.com/codepuncher/khuey/internal/capture"
"github.com/codepuncher/khuey/internal/color"
)

func main() {
fmt.Println("Color Extraction Test - Screen Sync Feature")
fmt.Println("============================================")
fmt.Println()

// Create capture with native PipeWire
fmt.Println("📸 Initializing screen capture (native PipeWire)...")
cap, err := capture.NewScreenCapture(capture.Config{
FPS:              30,
Monitor:          -1,
UseNativeCapture: true,
})
if err != nil {
log.Fatalf("Failed to create capture: %v", err)
}
defer cap.Stop()

// Start capture
if err := cap.Start(); err != nil {
log.Fatalf("Failed to start capture: %v", err)
}

// Wait for first frame to be available
fmt.Println("⏳ Waiting for first frame (5 seconds)...")
time.Sleep(5 * time.Second)

// Capture a frame
fmt.Println("📸 Capturing frame...")
frame, err := cap.CaptureFrame()
if err != nil {
log.Fatalf("Failed to capture frame: %v", err)
}
fmt.Printf("✅ Frame captured: %dx%d\n\n", frame.Bounds().Dx(), frame.Bounds().Dy())

// Define test zones (3 channels as per config)
zones := []color.Zone{
{
ID:   0,
U1:   0.0,
V1:   0.0,
U2:   0.33,
V2:   1.0,
Name: "Left",
},
{
ID:   1,
U1:   0.33,
V1:   0.0,
U2:   0.66,
V2:   1.0,
Name: "Center",
},
{
ID:   2,
U1:   0.66,
V1:   0.0,
U2:   1.0,
V2:   1.0,
Name: "Right",
},
}

fmt.Println("🎯 Test zones:")
for _, zone := range zones {
fmt.Printf("  Zone %d (%s): UV=(%.2f,%.2f)-(%.2f,%.2f)\n",
zone.ID, zone.Name, zone.U1, zone.V1, zone.U2, zone.V2)
}
fmt.Println()

// Create color extractor
fmt.Println("🎨 Creating color extractor (subsample=64px, gamma=2.2)...")
extractor, err := color.NewExtractor(64, 2.2)
if err != nil {
log.Fatalf("Failed to create extractor: %v", err)
}
fmt.Println()

// Extract colors
fmt.Println("⚡ Extracting colors from zones...")
colors, err := extractor.ExtractColors(frame, zones)
if err != nil {
log.Fatalf("Failed to extract colors: %v", err)
}

// Display results
fmt.Println("✅ Colors extracted!")
fmt.Println()
fmt.Println("Results:")
fmt.Println("--------")
for i, zoneColor := range colors {
zone := zones[i]
fmt.Printf("Zone %d (%s):\n", zone.ID, zone.Name)
fmt.Printf("  RGB: (%3d, %3d, %3d)\n", zoneColor.R, zoneColor.G, zoneColor.B)
fmt.Printf("  Hex: #%02X%02X%02X\n\n", zoneColor.R, zoneColor.G, zoneColor.B)
}

// Performance test
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

fmt.Println()
fmt.Println("✅ Color extraction test completed successfully!")
}
