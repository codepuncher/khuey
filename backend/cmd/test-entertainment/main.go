package main

import (
	"fmt"
	"log"
	"time"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/entertainment"
)

func main() {
	fmt.Println("Entertainment API Test - Milestone 3")
	fmt.Println("=====================================")
	fmt.Println()

	appCfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if !appCfg.HasEntertainmentConfig() {
		log.Fatal("Entertainment API is not configured: the config needs Bridge, Key, clientkey and entertainmentConfigurationId")
	}

	var channelIDs []int
	for _, ch := range appCfg.Channels {
		if !ch.Active {
			continue
		}
		channelIDs = append(channelIDs, int(ch.ID))
	}
	if len(channelIDs) == 0 {
		log.Fatal("No active channels in the config's 'channels' list")
	}

	cfg := entertainment.Config{
		BridgeIP:        appCfg.Bridge,
		Username:        appCfg.Key,
		ClientKey:       appCfg.ClientKey,
		EntertainmentID: appCfg.EntertainmentConfigurationID,
		ChannelCount:    len(channelIDs),
	}

	// Create client
	fmt.Println("Creating Entertainment API client...")
	client, err := entertainment.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close() //nolint:errcheck

	// Connect
	fmt.Println("Connecting to Entertainment API...")
	fmt.Printf("   Bridge: %s:%d\n", cfg.BridgeIP, entertainment.EntertainmentAPIPort)
	fmt.Println("   Entertainment ID: " + cfg.EntertainmentID)

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v\n\nNote: Make sure your Entertainment Area is activated in the Hue app!", err)
	}
	fmt.Println("Connected via DTLS!")
	fmt.Println()

	// Test 1: Send red to all lights
	fmt.Printf("Test 1: Sending RED to all %d lights...\n", len(channelIDs))
	redColors := solidColors(channelIDs, 65535, 0, 0)

	for i := 0; i < 30; i++ { // Stream for 1 second at 30 FPS
		if err := client.StreamColors(redColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond) // ~30 FPS
	}
	fmt.Println("   Red sent (1 second)")

	time.Sleep(500 * time.Millisecond)

	// Test 2: Send green to all lights
	fmt.Printf("Test 2: Sending GREEN to all %d lights...\n", len(channelIDs))
	greenColors := solidColors(channelIDs, 0, 65535, 0)

	for i := 0; i < 30; i++ {
		if err := client.StreamColors(greenColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond)
	}
	fmt.Println("   Green sent (1 second)")

	time.Sleep(500 * time.Millisecond)

	// Test 3: Send blue to all lights
	fmt.Printf("Test 3: Sending BLUE to all %d lights...\n", len(channelIDs))
	blueColors := solidColors(channelIDs, 0, 0, 65535)

	for i := 0; i < 30; i++ {
		if err := client.StreamColors(blueColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond)
	}
	fmt.Println("   Blue sent (1 second)")

	time.Sleep(500 * time.Millisecond)

	// Test 4: Different color per light
	fmt.Println("Test 4: Different colors per light...")
	primaries := [][3]uint16{{65535, 0, 0}, {0, 65535, 0}, {0, 0, 65535}}
	rainbowColors := make([]entertainment.ChannelColor, len(channelIDs))
	for i, id := range channelIDs {
		p := primaries[i%len(primaries)]
		rainbowColors[i] = entertainment.ChannelColor{ChannelID: id, R: p[0], G: p[1], B: p[2]}
	}

	for i := 0; i < 30; i++ {
		if err := client.StreamColors(rainbowColors); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(33 * time.Millisecond)
	}
	fmt.Println("   Rainbow sent (1 second)")

	fmt.Println("\nAll tests complete!")
	fmt.Println("\nNote: Lights will revert to previous state after disconnection.")
}

func solidColors(channelIDs []int, r, g, b uint16) []entertainment.ChannelColor {
	colors := make([]entertainment.ChannelColor, len(channelIDs))
	for i, id := range channelIDs {
		colors[i] = entertainment.ChannelColor{ChannelID: id, R: r, G: g, B: b}
	}
	return colors
}
