package main

import (
	"fmt"
	"log"
	"time"

	"github.com/codepuncher/khuey/internal/entertainment"
)

func main() {
	fmt.Println("Entertainment API Test - Milestone 3")
	fmt.Println("=====================================")
	fmt.Println()

	// Configuration (from your ~/.openhue/config.yaml)
	cfg := entertainment.Config{
		BridgeIP:        "192.168.0.9",
		Username:        "IxwdSPVqUEkomWphEq7nHHD246IGPCPAGqQpAt0T",
		ClientKey:       "20197970C01CB43EF516C6923A55ECB5",
		EntertainmentID: "6a941316-2219-4063-937e-cf0886eecb46",
		ChannelCount:    3, // You have 3 lights in your Entertainment Area
	}

	// Create client
	fmt.Println("📡 Creating Entertainment API client...")
	client, err := entertainment.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close() //nolint:errcheck

	// Connect
	fmt.Println("🔌 Connecting to Entertainment API...")
	fmt.Printf("   Bridge: %s:%d\n", cfg.BridgeIP, entertainment.EntertainmentAPIPort)
	fmt.Println("   Entertainment ID: " + cfg.EntertainmentID[:8] + "...")

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v\n\nNote: Make sure your Entertainment Area is activated in the Hue app!", err)
	}
	fmt.Println("✅ Connected via DTLS!")
	fmt.Println()

	// Test 1: Send red to all lights
	fmt.Println("🔴 Test 1: Sending RED to all 3 lights...")
	redColors := []entertainment.ChannelColor{
		{ChannelID: 0, R: 65535, G: 0, B: 0},
		{ChannelID: 1, R: 65535, G: 0, B: 0},
		{ChannelID: 2, R: 65535, G: 0, B: 0},
	}

	for i := 0; i < 30; i++ { // Stream for 1 second at 30 FPS
		if err := client.StreamColors(redColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond) // ~30 FPS
	}
	fmt.Println("   ✅ Red sent (1 second)")

	time.Sleep(500 * time.Millisecond)

	// Test 2: Send green to all lights
	fmt.Println("🟢 Test 2: Sending GREEN to all 3 lights...")
	greenColors := []entertainment.ChannelColor{
		{ChannelID: 0, R: 0, G: 65535, B: 0},
		{ChannelID: 1, R: 0, G: 65535, B: 0},
		{ChannelID: 2, R: 0, G: 65535, B: 0},
	}

	for i := 0; i < 30; i++ {
		if err := client.StreamColors(greenColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond)
	}
	fmt.Println("   ✅ Green sent (1 second)")

	time.Sleep(500 * time.Millisecond)

	// Test 3: Send blue to all lights
	fmt.Println("🔵 Test 3: Sending BLUE to all 3 lights...")
	blueColors := []entertainment.ChannelColor{
		{ChannelID: 0, R: 0, G: 0, B: 65535},
		{ChannelID: 1, R: 0, G: 0, B: 65535},
		{ChannelID: 2, R: 0, G: 0, B: 65535},
	}

	for i := 0; i < 30; i++ {
		if err := client.StreamColors(blueColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond)
	}
	fmt.Println("   ✅ Blue sent (1 second)")

	time.Sleep(500 * time.Millisecond)

	// Test 4: Different color per light
	fmt.Println("🌈 Test 4: Different colors per light...")
	rainbowColors := []entertainment.ChannelColor{
		{ChannelID: 0, R: 65535, G: 0, B: 0}, // Red
		{ChannelID: 1, R: 0, G: 65535, B: 0}, // Green
		{ChannelID: 2, R: 0, G: 0, B: 65535}, // Blue
	}

	for i := 0; i < 30; i++ {
		if err := client.StreamColors(rainbowColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond)
	}
	fmt.Println("   ✅ Rainbow sent (1 second)")

	fmt.Println("\n🎉 All tests complete!")
	fmt.Println("\nNote: Lights will revert to previous state after disconnection.")
}
