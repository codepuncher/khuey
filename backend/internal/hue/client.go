package hue

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/codepuncher/khuey/internal/common"
	"github.com/openhue/openhue-go"
	"golang.org/x/time/rate"
)

// ConnectionStatus represents the current bridge connection state
type ConnectionStatus struct {
	Connected   bool
	LastError   string
	LastAttempt time.Time
	BridgeAddr  string
}

// Client wraps the openhue-go client for basic Hue operations
type Client struct {
	client     *openhue.ClientWithResponses
	bridgeAddr string
	apiKey     string
	ctx        context.Context
	// Connection tracking
	connStatus ConnectionStatus
	connMutex  sync.RWMutex
	// Rate limiting (10 req/sec, burst 20)
	limiter *rate.Limiter
}

// Scene represents a Hue scene
type Scene struct {
	ID             string
	Name           string
	Room           string
	RoomName       string
	GroupedLightID string
}

// GroupedLight represents a room or zone with grouped lights
type GroupedLight struct {
	ID   string
	Name string
	Type string // "room" or "zone"
}

// roomOrZone holds a room or zone's identity and its associated grouped_light
// service ID (if any), resolved once by getRoomsAndZones for reuse across callers.
type roomOrZone struct {
	ID             string
	Name           string
	Type           string // "room" or "zone"
	GroupedLightID string
}

// NewClient creates a new Hue client with a cancellable context
// The context should be the application's main context so API calls can be
// cancelled during shutdown
func NewClient(ctx context.Context, bridgeAddr, apiKey string) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if bridgeAddr == "" {
		return nil, fmt.Errorf("bridge address is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	httpClient := common.NewHueHTTPClient()

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

	c := &Client{
		client:     client,
		bridgeAddr: bridgeAddr,
		apiKey:     apiKey,
		ctx:        ctx,
		connStatus: ConnectionStatus{
			Connected:  false,
			BridgeAddr: bridgeAddr,
		},
		limiter: rate.NewLimiter(rate.Limit(10), 20),
	}

	// Initial connection check
	if err := c.updateConnectionStatus(); err != nil {
		// Don't fail initialization, just log the error
		c.setConnectionError(err)
	}

	return c, nil
}

// waitForRateLimit waits for rate limiter before making API call
func (c *Client) waitForRateLimit() error {
	return c.limiter.Wait(c.ctx)
}

// SetLightPower turns a grouped light (room) on or off
func (c *Client) SetLightPower(groupID string, on bool) error {
	if err := c.waitForRateLimit(); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

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

// GetGroupedLightState gets the current power and brightness state of a grouped light
func (c *Client) GetGroupedLightState(groupID string) (power bool, brightness float32, err error) {
	if err := c.waitForRateLimit(); err != nil {
		return false, 0, fmt.Errorf("rate limit error: %w", err)
	}

	resp, err := c.client.GetGroupedLightWithResponse(c.ctx, groupID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get grouped light: %w", err)
	}

	if resp.StatusCode() != 200 {
		return false, 0, fmt.Errorf("failed to get grouped light: status %d", resp.StatusCode())
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
		return false, 0, fmt.Errorf("no data in response")
	}

	data := (*resp.JSON200.Data)[0]

	power = false
	if data.On != nil && data.On.On != nil {
		power = *data.On.On
	}

	brightness = 0
	if data.Dimming != nil && data.Dimming.Brightness != nil {
		brightness = *data.Dimming.Brightness
	}

	return power, brightness, nil
}

// SetLightBrightness sets the brightness of a grouped light (0-100)
// When brightness > 0, also turns on all lights in the group to ensure
// the brightness change applies to all lights (not just those currently on)
func (c *Client) SetLightBrightness(groupID string, brightness float32) error {
	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}

	if err := c.waitForRateLimit(); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	body := openhue.UpdateGroupedLightJSONRequestBody{
		Dimming: &openhue.Dimming{
			Brightness: &brightness,
		},
	}

	// When setting brightness > 0, also turn on the lights
	// This ensures all lights in the group respond to the brightness change,
	// not just the ones that are currently on (e.g., after a scene activation).
	// brightness == 0 intentionally leaves on/off state untouched (asymmetric).
	if brightness > 0 {
		on := true
		body.On = &openhue.On{
			On: &on,
		}
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
	if err := c.waitForRateLimit(); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	action := openhue.SceneRecallActionActive

	body := openhue.UpdateSceneJSONRequestBody{
		Recall: &openhue.SceneRecall{
			Action: &action,
		},
	}

	resp, err := c.client.UpdateSceneWithResponse(c.ctx, sceneID, body)
	if err != nil {
		c.setConnectionError(err)
		return fmt.Errorf("failed to activate scene: %w", err)
	}

	if resp.StatusCode() != 200 {
		err := fmt.Errorf("failed to activate scene: status %d", resp.StatusCode())
		c.setConnectionError(err)
		return err
	}

	c.setConnectionSuccess()
	return nil
}

// GetScenes lists all available scenes with their room names
func (c *Client) GetScenes() ([]Scene, error) {
	if err := c.waitForRateLimit(); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	resp, err := c.client.GetScenesWithResponse(c.ctx)
	if err != nil {
		c.setConnectionError(err)
		return nil, fmt.Errorf("failed to get scenes: %w", err)
	}

	if resp.StatusCode() != 200 {
		err := fmt.Errorf("failed to get scenes: status %d", resp.StatusCode())
		c.setConnectionError(err)
		return nil, err
	}

	c.setConnectionSuccess()

	// Resolve room/zone names and grouped_light IDs for display and activation
	roomsByID := make(map[string]roomOrZone)
	if roomsAndZones, _, err := c.getRoomsAndZones(); err == nil {
		for _, rz := range roomsAndZones {
			roomsByID[rz.ID] = rz
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

		// Get room/zone name and grouped_light ID if available
		if item.Group != nil && item.Group.Rid != nil {
			scene.Room = *item.Group.Rid
			if rz, ok := roomsByID[scene.Room]; ok {
				scene.RoomName = rz.Name
				scene.GroupedLightID = rz.GroupedLightID
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
	if err := c.waitForRateLimit(); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	resp, err := c.client.GetBridgesWithResponse(c.ctx)
	if err != nil {
		c.setConnectionError(err)
		return fmt.Errorf("bridge unreachable: %w", err)
	}

	if resp.StatusCode() != 200 {
		err := fmt.Errorf("bridge returned status %d", resp.StatusCode())
		c.setConnectionError(err)
		return err
	}

	c.setConnectionSuccess()
	return nil
}

// getRoomsAndZones fetches all rooms and zones from the bridge, resolving each
// one's name and associated grouped_light service ID. It is the single source
// of rooms/zones data shared by GetScenes and GetGroupedLights. fetchErrs
// holds per-endpoint failures (rooms and/or zones); it is not folded into the
// returned error because a partial fetch (e.g. zones failed but rooms
// succeeded) is not fatal - callers that care whether they got usable data
// decide that for themselves. The returned error is only set for a hard
// failure (rate limiting) before any fetch runs.
func (c *Client) getRoomsAndZones() (result []roomOrZone, fetchErrs []error, err error) {
	if err := c.waitForRateLimit(); err != nil {
		return nil, nil, fmt.Errorf("rate limit error: %w", err)
	}

	// Get rooms (don't fail if this fails, we can still try zones)
	roomsResp, roomsErr := c.client.GetRoomsWithResponse(c.ctx)
	if roomsErr != nil {
		fetchErrs = append(fetchErrs, fmt.Errorf("failed to get rooms: %w", roomsErr))
	} else if roomsResp.StatusCode() != 200 {
		fetchErrs = append(fetchErrs, fmt.Errorf("failed to get rooms: status %d", roomsResp.StatusCode()))
	} else if roomsResp.JSON200 != nil && roomsResp.JSON200.Data != nil {
		for _, room := range *roomsResp.JSON200.Data {
			if room.Id == nil || room.Metadata == nil || room.Metadata.Name == nil {
				continue
			}
			result = append(result, roomOrZone{
				ID:             *room.Id,
				Name:           *room.Metadata.Name,
				Type:           "room",
				GroupedLightID: findGroupedLightService(room.Services),
			})
		}
	}

	// Get zones (don't fail if this fails, we might have rooms)
	zonesResp, zonesErr := c.client.GetZonesWithResponse(c.ctx)
	if zonesErr != nil {
		fetchErrs = append(fetchErrs, fmt.Errorf("failed to get zones: %w", zonesErr))
	} else if zonesResp.StatusCode() != 200 {
		fetchErrs = append(fetchErrs, fmt.Errorf("failed to get zones: status %d", zonesResp.StatusCode()))
	} else if zonesResp.JSON200 != nil && zonesResp.JSON200.Data != nil {
		for _, zone := range *zonesResp.JSON200.Data {
			if zone.Id == nil || zone.Metadata == nil || zone.Metadata.Name == nil {
				continue
			}
			result = append(result, roomOrZone{
				ID:             *zone.Id,
				Name:           *zone.Metadata.Name,
				Type:           "zone",
				GroupedLightID: findGroupedLightService(zone.Services),
			})
		}
	}

	return result, fetchErrs, nil
}

// findGroupedLightService returns the grouped_light resource ID from a
// room/zone's services list, or "" if none is present.
func findGroupedLightService(services *[]openhue.ResourceIdentifier) string {
	if services == nil {
		return ""
	}
	for _, svc := range *services {
		if svc.Rtype != nil && *svc.Rtype == "grouped_light" && svc.Rid != nil {
			return *svc.Rid
		}
	}
	return ""
}

// GetGroupedLights lists all available rooms and zones with grouped lights
func (c *Client) GetGroupedLights() ([]GroupedLight, error) {
	roomsAndZones, fetchErrs, err := c.getRoomsAndZones()
	if err != nil {
		return nil, err
	}

	var groupedLights []GroupedLight
	for _, rz := range roomsAndZones {
		if rz.GroupedLightID == "" {
			continue
		}
		groupedLights = append(groupedLights, GroupedLight{
			ID:   rz.GroupedLightID,
			Name: rz.Name,
			Type: rz.Type,
		})
	}

	// Only fail if we got no lights at all
	if len(groupedLights) == 0 && len(fetchErrs) > 0 {
		return nil, fmt.Errorf("failed to get any grouped lights: %v", fetchErrs)
	}

	// Sort by name
	sort.Slice(groupedLights, func(i, j int) bool {
		return groupedLights[i].Name < groupedLights[j].Name
	})

	return groupedLights, nil
}

// GetConnectionStatus returns the current connection status
func (c *Client) GetConnectionStatus() ConnectionStatus {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connStatus
}

// IsReachable checks if the bridge is currently reachable
func (c *Client) IsReachable() (bool, error) {
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	// Note: No rate limiting on health check to avoid blocking status checks
	// Try to get bridge info
	resp, err := c.client.GetBridgesWithResponse(ctx)
	if err != nil {
		c.setConnectionError(err)
		return false, err
	}

	if resp.StatusCode() != 200 {
		err := fmt.Errorf("bridge returned status %d", resp.StatusCode())
		c.setConnectionError(err)
		return false, err
	}

	c.setConnectionSuccess()
	return true, nil
}

// updateConnectionStatus checks and updates the connection status
func (c *Client) updateConnectionStatus() error {
	c.connMutex.Lock()
	c.connStatus.LastAttempt = time.Now()
	c.connMutex.Unlock()

	_, err := c.IsReachable()
	return err
}

// setConnectionError marks the connection as failed with an error
func (c *Client) setConnectionError(err error) {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	c.connStatus.Connected = false
	c.connStatus.LastError = err.Error()
	c.connStatus.LastAttempt = time.Now()
}

// setConnectionSuccess marks the connection as successful
func (c *Client) setConnectionSuccess() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	c.connStatus.Connected = true
	c.connStatus.LastError = ""
	c.connStatus.LastAttempt = time.Now()
}
