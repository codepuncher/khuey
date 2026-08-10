package main

import (
	"fmt"
	"log"

	"github.com/codepuncher/khuey/internal/common"
	"github.com/codepuncher/khuey/internal/config"
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
	fmt.Printf("Username: %s\n\n", common.SanitizeForLog(cfg.Key))

	if cfg.ClientKey == "" {
		fmt.Println("No Entertainment client key is configured.")
		fmt.Println("Run 'go run ./cmd/register-entertainment' to obtain one from the bridge,")
		fmt.Println("then add it to ~/.openhue/config.yaml:")
		fmt.Println("  clientkey: YOUR_CLIENT_KEY")
		fmt.Println()
	} else {
		// Printed unmasked: the user needs to copy this into config.yaml.
		fmt.Println("Client Key (for Entertainment API):")
		fmt.Printf("  %s\n\n", cfg.ClientKey)
	}

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
