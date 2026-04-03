package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/openhue/openhue-go"
)

func main() {
	fmt.Println("Hue Entertainment API Registration")
	fmt.Println("===================================")
	fmt.Println()

	bridgeIP := "192.168.0.9"
	
	fmt.Printf("Registering with bridge: %s\n", bridgeIP)
	fmt.Println("\n⚠️  PLEASE PRESS THE LINK BUTTON ON YOUR HUE BRIDGE NOW!")
	fmt.Println("    Attempting registration in 3 seconds...")
	fmt.Println()

	// Short countdown to give user time to read
	for i := 3; i > 0; i-- {
		fmt.Printf("\r   Starting in... %d seconds   ", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("\r                                ")

	// Create HTTP client with insecure TLS (Hue bridge uses self-signed cert)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// Use the lower-level API to get full response including clientkey
	client, err := openhue.NewClientWithResponses("https://"+bridgeIP, openhue.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("\n📡 Attempting registration...")
	
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
