package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
)

func main() {
	fmt.Println("Screen Capture Test - Milestone 1")
	fmt.Println("==================================")

	// Create capture instance with 30 FPS
	cfg := capture.Config{
		FPS:     30,
		Monitor: -1, // All monitors
	}

	cap, err := capture.NewScreenCapture(cfg)
	if err != nil {
		log.Fatalf("Failed to create screen capture: %v", err)
	}
	defer cap.Stop()

	// Start capture session
	fmt.Println("Starting screen capture...")
	fmt.Println("You may see a system permission dialog - please allow screen capture.")

	if err := cap.Start(); err != nil {
		log.Fatalf("Failed to start capture: %v", err)
	}

	fmt.Println("✅ Screen capture started successfully!")
	fmt.Printf("📊 FPS: %d (frame interval: %v)\n", 30, cap.GetFrameInterval())

	// Capture a test frame
	fmt.Println("\n📸 Capturing test frame...")
	frame, err := cap.CaptureFrame()
	if err != nil {
		log.Fatalf("Failed to capture frame: %v", err)
	}

	fmt.Printf("✅ Frame captured: %dx%d\n", frame.Bounds().Dx(), frame.Bounds().Dy())
	fmt.Println("   (Mock gradient frame for pipeline testing)")

	fmt.Println("\nPress Ctrl+C to stop...")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Simulate frame capture (we'll implement actual frame reading later)
	ticker := time.NewTicker(cap.GetFrameInterval())
	defer ticker.Stop()

	frameCount := 0
	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			frameCount++
			elapsed := time.Since(startTime)

			// Capture frame (currently returns mock gradient)
			_, err := cap.CaptureFrame()
			if err != nil {
				log.Printf("Frame capture error: %v", err)
			}

			if frameCount%30 == 0 {
				fps := float64(frameCount) / elapsed.Seconds()
				fmt.Printf("📸 Captured %d frames (%.1f FPS actual)\n", frameCount, fps)
			}

		case <-sigChan:
			elapsed := time.Since(startTime)
			fps := float64(frameCount) / elapsed.Seconds()
			fmt.Printf("\n\n📊 Final stats:\n")
			fmt.Printf("   Total frames: %d\n", frameCount)
			fmt.Printf("   Duration: %.1fs\n", elapsed.Seconds())
			fmt.Printf("   Average FPS: %.1f\n", fps)
			fmt.Println("\n✅ Stopped gracefully")
			return
		}
	}
}
