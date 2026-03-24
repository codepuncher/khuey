package main

import (
	"fmt"
	"log"
	"time"

	"github.com/openhue/openhue-go"
)

func main() {
	fmt.Println("Hue Entertainment API Registration")
	fmt.Println("===================================\n")

	bridgeIP := "192.168.0.9"
	
	fmt.Printf("Registering with bridge: %s\n", bridgeIP)
	fmt.Println("\n⚠️  PLEASE PRESS THE LINK BUTTON ON YOUR HUE BRIDGE NOW!")
	fmt.Println("    You have 30 seconds...")
	fmt.Println()

	// Wait for user to press the button
	for i := 30; i > 0; i-- {
		fmt.Printf("\r   Waiting... %d seconds remaining   ", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println()

	// Create authenticator with ClientKey generation enabled
	authenticator, err := openhue.NewAuthenticator(
		bridgeIP,
		openhue.WithDeviceType("khuey#desktop"),
		openhue.WithGenerateClientKey(true), // This is the key part!
	)
	if err != nil {
		log.Fatalf("Failed to create authenticator: %v", err)
	}

	fmt.Println("\n📡 Attempting registration...")
	username, clientKey, err := authenticator.Authenticate()
	if err != nil {
		log.Fatalf("Failed to authenticate: %v\n\nDid you press the link button?", err)
	}

	fmt.Println("\n✅ Registration successful!")
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("SAVE THESE CREDENTIALS:")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Username:   %s\n", username)
	fmt.Printf("ClientKey:  %s\n", clientKey)
	fmt.Println("════════════════════════════════════════")

	fmt.Println("\nUpdate your ~/.openhue/config.yaml with:")
	fmt.Printf("  key: %s\n", username)
	fmt.Printf("  clientkey: %s\n", clientKey)
	
	fmt.Println("\nNote: The clientkey is HEX-encoded and used for")
	fmt.Println("      Entertainment API DTLS authentication.")
}
