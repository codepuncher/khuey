package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/codepuncher/khuey/internal/common"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/openhue/openhue-go"
	"github.com/spf13/viper"
)

func main() {
	bridgeFlag := flag.String("bridge", "", "Hue bridge IP address (default: the Bridge in the config)")
	flag.Parse()

	fmt.Println("Hue Entertainment API Registration")
	fmt.Println("===================================")
	fmt.Println()

	bridgeIP, err := bridgeAddress(*bridgeFlag)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Printf("Registering with bridge: %s\n", bridgeIP)
	fmt.Println("\nPLEASE PRESS THE LINK BUTTON ON YOUR HUE BRIDGE NOW!")
	fmt.Println("    Attempting registration in 3 seconds...")
	fmt.Println()

	// Short countdown to give user time to read
	for i := 3; i > 0; i-- {
		fmt.Printf("\r   Starting in... %d seconds   ", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("\r                                ")

	// QUAL-004: Use common HTTP client instead of creating new one
	httpClient := common.NewHueHTTPClient()

	// Use the lower-level API to get full response including clientkey
	client, err := openhue.NewClientWithResponses("https://"+bridgeIP, openhue.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("\nAttempting registration...")

	// Make authentication request with generateclientkey
	deviceType := "khuey#desktop"
	generateKey := true
	resp, err := client.AuthenticateWithResponse(context.Background(), openhue.AuthenticateJSONRequestBody{
		Devicetype:        &deviceType,
		Generateclientkey: &generateKey,
	})
	if err != nil {
		log.Fatalf("Failed to authenticate: %v\n\nDid you press the link button?", err)
	}

	// Check for errors in response
	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		log.Fatalf("Invalid response from bridge")
	}

	result := (*resp.JSON200)[0]

	// Check for error
	if result.Error != nil {
		desc := "Unknown error"
		if result.Error.Description != nil {
			desc = *result.Error.Description
		}
		log.Fatalf("Bridge error: %s\n\nDid you press the link button?", desc)
	}

	// Check for success
	if result.Success == nil {
		log.Fatalf("No success data in response")
	}

	username := ""
	if result.Success.Username != nil {
		username = *result.Success.Username
	}

	clientKey := ""
	if result.Success.Clientkey != nil {
		clientKey = *result.Success.Clientkey
	}

	if username == "" || clientKey == "" {
		log.Fatalf("Missing username or clientkey in response")
	}

	fmt.Println("\nRegistration successful!")
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("SAVE THESE CREDENTIALS:")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Username:   %s\n", username)
	fmt.Printf("ClientKey:  %s\n", clientKey)
	fmt.Println("════════════════════════════════════════")

	fmt.Printf("\nReplace these values in %s (the keys may be capitalised there):\n", config.File())
	fmt.Printf("  key: %s\n", username)
	fmt.Printf("  clientkey: %s\n", clientKey)

	fmt.Println("\nNote: The clientkey is HEX-encoded and used for")
	fmt.Println("      Entertainment API DTLS authentication.")
}

// bridgeAddress returns the bridge to register with. The config is read
// directly instead of through config.Load, which validates: a config with a
// Bridge and no Key is the case this tool exists to fix.
func bridgeAddress(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	v := viper.New()
	v.SetConfigFile(config.File())
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return "", fmt.Errorf("no -bridge given and %s could not be read: %w", config.File(), err)
	}

	bridge := v.GetString("bridge")
	if bridge == "" {
		return "", fmt.Errorf("no -bridge given and no Bridge in %s", config.File())
	}
	return bridge, nil
}
