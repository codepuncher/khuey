package main

import (
	"fmt"
	"log"
	"time"

	"github.com/codepuncher/khuey/internal/gaming"
)

func main() {
	log.Println("🎮 Gaming Detection Test Utility")
	log.Println("=================================")
	log.Println()
	log.Println("This utility tests all gaming detection methods in real-time.")
	log.Println("Launch a game and watch the detection status update every 2 seconds.")
	log.Println()

	// Initialize all detectors
	log.Println("Initializing detectors...")
	systemd := gaming.NewSystemdDetector()
	log.Println("✅ systemd-inhibit detector ready")

	power := gaming.NewPowerProfileDetector()
	log.Println("✅ Power profile detector ready")

	steam := gaming.NewSteamDetector()
	log.Println("✅ Steam AppId detector ready")

	log.Println()
	log.Println("Monitoring gaming activity (Ctrl+C to exit)...")
	log.Println()

	// Monitor every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		// Check all detection methods
		systemdActive := systemd.IsActive()
		powerPerf := power.IsPerformanceMode()
		steamActive := steam.IsActive()

		// Overall detection result (matches main detector logic)
		gaming := systemdActive || (powerPerf && steamActive)

		// Format timestamp
		timestamp := time.Now().Format("15:04:05")

		// Print status
		fmt.Printf("[%s]\n", timestamp)
		fmt.Printf("  systemd-inhibit: %v", systemdActive)
		if systemdActive {
			fmt.Printf(" ✅")
		}
		fmt.Println()

		fmt.Printf("  Power profile:   %v", powerPerf)
		if powerPerf {
			fmt.Printf(" ✅")
		}
		fmt.Println()

		fmt.Printf("  Steam AppId:     %v", steamActive)
		if steamActive {
			if appId := steam.GetAppId(); appId != "" {
				fmt.Printf(" (AppId: %s)", appId)
			}
			fmt.Printf(" ✅")
		}
		fmt.Println()

		// Overall result with emoji
		if gaming {
			fmt.Printf("  🎮 Gaming Active: %v ✅\n", gaming)
		} else {
			fmt.Printf("  🎮 Gaming Active: %v\n", gaming)
		}
		fmt.Println()
	}
}
