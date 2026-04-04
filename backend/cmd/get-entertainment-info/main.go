package main

import (
	"context"
	"fmt"
	"log"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
)

func main() {
	fmt.Println("Entertainment Area Information Tool")
	fmt.Println("====================================")
	fmt.Println()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v\n", err)
	}

	fmt.Printf("Bridge: %s\n", cfg.Bridge)
	fmt.Printf("Username: %s\n\n", cfg.Key)

	// Create Hue client
	client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
	if err != nil {
		log.Fatalf("Failed to create client: %v\n", err)
	}

	// Get client key (needed for Entertainment API)
	clientKey := client.GetClientKey()
	fmt.Println("Client Key (for Entertainment API):")
	fmt.Printf("  %s\n\n", clientKey)

	// Note: Entertainment Areas must be created in the Hue app
	// The Entertainment Configuration API endpoint is: /clip/v2/resource/entertainment_configuration
	// However, creating/modifying Entertainment Areas via API is complex and best done through the Hue app
	fmt.Println("Note: Entertainment Areas must be created in the Hue app first.")
	fmt.Println("To query Entertainment Areas, use the openhue-cli tool:")
	fmt.Println("  openhue-cli entertainment list")
	fmt.Println()
	fmt.Println("After creating an Entertainment Area, add its ID to ~/.openhue/config.yaml:")
	fmt.Println("  entertainmentConfigurationId: YOUR_ENTERTAINMENT_ID")
}
