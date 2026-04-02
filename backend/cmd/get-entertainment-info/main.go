package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
)

func main() {
	fmt.Println("Entertainment Area Information Tool")
	fmt.Println("====================================\n")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v\n", err)
	}

	fmt.Printf("Bridge: %s\n", cfg.Bridge)
	fmt.Printf("Username: %s\n\n", cfg.Key)

	// Create Hue client
	client, err := hue.NewClient(cfg.Bridge, cfg.Key)
	if err != nil {
		log.Fatalf("Failed to create client: %v\n", err)
	}

	// Get client key (needed for Entertainment API)
	clientKey := client.GetClientKey()
	fmt.Println("Client Key (for Entertainment API):")
	fmt.Printf("  %s\n\n", clientKey)

	// TODO: Add method to fetch Entertainment Areas
	fmt.Println("Note: You'll need to create an Entertainment Area in the Hue app first.")
	fmt.Println("Then we can query it via the API.")
}
