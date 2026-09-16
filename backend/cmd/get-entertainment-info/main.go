package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/codepuncher/khuey/internal/common"
	"github.com/codepuncher/khuey/internal/config"
)

type entertainmentArea struct {
	ID       string `json:"id"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Channels []json.RawMessage `json:"channels"`
}

func main() {
	fmt.Println("Entertainment Area Information Tool")
	fmt.Println("====================================")
	fmt.Println()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v\n", err)
	}
	if !cfg.IsConfigured() {
		log.Fatalf("No bridge configured in %s. Run 'openhue setup' first.", config.File())
	}

	fmt.Printf("Bridge: %s\n", cfg.Bridge)
	fmt.Printf("Username: %s\n\n", common.SanitizeForLog(cfg.Key))

	if cfg.ClientKey == "" {
		fmt.Println("No Entertainment client key is configured.")
		fmt.Println("Run 'go run ./cmd/register-entertainment' to register with the bridge.")
		fmt.Println("It registers a new user, so replace both the key and the clientkey in")
		fmt.Printf("%s with the values it prints.\n", config.File())
		fmt.Println("The key is the DTLS identity for that clientkey.")
		fmt.Println()
	} else {
		// Printed unmasked: the user needs to copy this into config.yaml.
		fmt.Println("Client Key (for Entertainment API):")
		fmt.Printf("  %s\n\n", cfg.ClientKey)
	}

	areas, err := listEntertainmentAreas(cfg.Bridge, cfg.Key)
	if err != nil {
		log.Fatalf("Failed to list Entertainment Areas: %v", err)
	}
	if len(areas) == 0 {
		fmt.Println("The bridge has no Entertainment Areas. Create one in the Hue app.")
		return
	}

	fmt.Println("Entertainment Areas:")
	for _, area := range areas {
		configured := ""
		if area.ID == cfg.EntertainmentConfigurationID {
			configured = " (configured)"
		}
		fmt.Printf("  %s  %s, %d channels%s\n", area.ID, area.Metadata.Name, len(area.Channels), configured)
	}
	fmt.Println()
	fmt.Printf("Set the one to use as entertainmentConfigurationId in %s\n", config.File())
}

func listEntertainmentAreas(bridge, key string) ([]entertainmentArea, error) {
	req, err := http.NewRequest(http.MethodGet, "https://"+bridge+"/clip/v2/resource/entertainment_configuration", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("hue-application-key", key)

	resp, err := common.NewHueHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bridge returned %s", resp.Status)
	}

	var body struct {
		Errors []json.RawMessage   `json:"errors"`
		Data   []entertainmentArea `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(body.Errors) > 0 {
		return nil, fmt.Errorf("bridge returned errors: %s", body.Errors)
	}
	return body.Data, nil
}
