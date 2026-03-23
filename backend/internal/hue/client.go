package hue

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/openhue/openhue-go"
)

// Client wraps the openhue-go client for basic Hue operations
type Client struct {
	client     *openhue.ClientWithResponses
	bridgeAddr string
	apiKey     string
	ctx        context.Context
}

// Scene represents a Hue scene
type Scene struct {
	ID       string
	Name     string
	Room     string
	RoomName string
}

// NewClient creates a new Hue client
func NewClient(bridgeAddr, apiKey string) (*Client, error) {
	if bridgeAddr == "" {
		return nil, fmt.Errorf("bridge address is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Create HTTP client that accepts self-signed certificates
	// (Hue bridges use self-signed certs)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 10 * time.Second,
	}

	// Create API key auth function
	apiKeyAuth := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("hue-application-key", apiKey)
		return nil
	}

	// Create openhue client
	client, err := openhue.NewClientWithResponses(
		"https://"+bridgeAddr,
		openhue.WithHTTPClient(httpClient),
		openhue.WithRequestEditorFn(apiKeyAuth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Hue client: %w", err)
	}

	return &Client{
		client:     client,
		bridgeAddr: bridgeAddr,
		apiKey:     apiKey,
		ctx:        context.Background(),
	}, nil
}

// SetLightPower turns a grouped light (room) on or off
func (c *Client) SetLightPower(groupID string, on bool) error {
	body := openhue.UpdateGroupedLightJSONRequestBody{
		On: &openhue.On{
			On: &on,
		},
	}

	resp, err := c.client.UpdateGroupedLightWithResponse(c.ctx, groupID, body)
	if err != nil {
		return fmt.Errorf("failed to set light power: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to set light power: status %d", resp.StatusCode())
	}

	return nil
}

// SetLightBrightness sets the brightness of a grouped light (0-100)
func (c *Client) SetLightBrightness(groupID string, brightness float32) error {
	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}

	body := openhue.UpdateGroupedLightJSONRequestBody{
		Dimming: &openhue.Dimming{
			Brightness: &brightness,
		},
	}

	resp, err := c.client.UpdateGroupedLightWithResponse(c.ctx, groupID, body)
	if err != nil {
		return fmt.Errorf("failed to set brightness: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to set brightness: status %d", resp.StatusCode())
	}

	return nil
}

// ActivateScene activates a scene
func (c *Client) ActivateScene(sceneID string) error {
	action := openhue.SceneRecallActionActive

	body := openhue.UpdateSceneJSONRequestBody{
		Recall: &openhue.SceneRecall{
			Action: &action,
		},
	}

	resp, err := c.client.UpdateSceneWithResponse(c.ctx, sceneID, body)
	if err != nil {
		return fmt.Errorf("failed to activate scene: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to activate scene: status %d", resp.StatusCode())
	}

	return nil
}

// GetScenes lists all available scenes with their room names
func (c *Client) GetScenes() ([]Scene, error) {
	resp, err := c.client.GetScenesWithResponse(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get scenes: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to get scenes: status %d", resp.StatusCode())
	}

	// Get rooms to resolve names
	roomsResp, err := c.client.GetRoomsWithResponse(c.ctx)
	roomMap := make(map[string]string)
	if err == nil && roomsResp.JSON200 != nil && roomsResp.JSON200.Data != nil {
		for _, room := range *roomsResp.JSON200.Data {
			if room.Id != nil && room.Metadata != nil && room.Metadata.Name != nil {
				roomMap[*room.Id] = *room.Metadata.Name
			}
		}
	}

	// Get zones to resolve names
	zonesResp, err := c.client.GetZonesWithResponse(c.ctx)
	if err == nil && zonesResp.JSON200 != nil && zonesResp.JSON200.Data != nil {
		for _, zone := range *zonesResp.JSON200.Data {
			if zone.Id != nil && zone.Metadata != nil && zone.Metadata.Name != nil {
				roomMap[*zone.Id] = *zone.Metadata.Name
			}
		}
	}

	scenes := []Scene{}
	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return scenes, nil
	}

	for _, item := range *resp.JSON200.Data {
		if item.Id == nil || item.Metadata == nil || item.Metadata.Name == nil {
			continue
		}

		scene := Scene{
			ID:   *item.Id,
			Name: *item.Metadata.Name,
		}

		// Get room/zone name if available
		if item.Group != nil && item.Group.Rid != nil {
			scene.Room = *item.Group.Rid
			if roomName, ok := roomMap[scene.Room]; ok {
				scene.RoomName = roomName
			}
		}

		scenes = append(scenes, scene)
	}

	// Sort scenes alphabetically by room name first, then scene name
	sort.Slice(scenes, func(i, j int) bool {
		if scenes[i].RoomName != scenes[j].RoomName {
			return scenes[i].RoomName < scenes[j].RoomName
		}
		return scenes[i].Name < scenes[j].Name
	})

	return scenes, nil
}

// Ping checks if the bridge is reachable
func (c *Client) Ping() error {
	resp, err := c.client.GetBridgesWithResponse(c.ctx)
	if err != nil {
		return fmt.Errorf("bridge unreachable: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("bridge returned status %d", resp.StatusCode())
	}

	return nil
}

// GetClientKey returns the API key (for Entertainment API setup)
func (c *Client) GetClientKey() string {
	return c.apiKey
}
