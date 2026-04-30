package dbus

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/common"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/gaming"
	"github.com/codepuncher/khuey/internal/hue"
	syncengine "github.com/codepuncher/khuey/internal/sync"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	dbusName      = "org.kde.plasma.hue"
	dbusPath      = "/org/kde/plasma/hue"
	dbusInterface = "org.kde.plasma.hue"
)

// Service provides DBus interface for the tray application
type Service struct {
	conn             *dbus.Conn
	config           *config.Config
	hueClient        *hue.Client
	syncEngine       *syncengine.Engine
	gamingDetector   *gaming.Detector
	gamingModeActive bool         // Track if gaming mode triggered sync
	mu               sync.RWMutex // Protects config access from concurrent DBus calls
	ownerUID         uint32       // UID of the service owner for access control
}

// NewService creates a new DBus service
func NewService(cfg *config.Config, client *hue.Client) (*Service, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	// Create sync engine if Entertainment API is configured
	var engine *syncengine.Engine
	if cfg.EntertainmentConfigurationID != "" && cfg.ClientKey != "" {
		engine, err = syncengine.NewEngine(cfg)
		if err != nil {
			log.Printf("[WARN] Failed to create sync engine: %v", err)
			log.Println("   Screen sync will be unavailable")
		} else {
			log.Println("[INFO] Sync engine initialized")
		}
	} else {
		log.Println("[INFO] Entertainment API not configured - screen sync unavailable")
	}

	return &Service{
		conn:       conn,
		config:     cfg,
		hueClient:  client,
		syncEngine: engine,
		ownerUID:   uint32(os.Getuid()), // Store owner UID for access control
	}, nil
}

// Start begins the DBus service
func (s *Service) Start() error {
	var success bool
	defer func() {
		if !success && s.conn != nil {
			s.conn.Close()
		}
	}()

	// Request the bus name
	reply, err := s.conn.RequestName(dbusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("failed to request bus name: %w", err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("name already taken")
	}

	// Export the service
	if err := s.conn.Export(s, dbusPath, dbusInterface); err != nil {
		return fmt.Errorf("failed to export service: %w", err)
	}

	// Export introspection data
	node := &introspect.Node{
		Name: dbusPath,
		Interfaces: []introspect.Interface{
			{
				Name:    dbusInterface,
				Methods: s.introspectionMethods(),
			},
			introspect.IntrospectData,
		},
	}

	if err := s.conn.Export(introspect.NewIntrospectable(node), dbusPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("failed to export introspection: %w", err)
	}

	log.Printf("DBus service started: %s at %s", dbusName, dbusPath)
	success = true
	return nil
}

// Stop stops the DBus service
func (s *Service) Stop() {
	// Stop gaming mode detector if running
	s.StopGamingMode()

	if s.conn != nil {
		// Release the DBus name before closing connection to prevent resource leak
		s.conn.ReleaseName(dbusName)
		s.conn.Close()
	}
}

// getCallerUID retrieves the UID of the DBus caller for access control
func (s *Service) getCallerUID(sender dbus.Sender) (uint32, error) {
	obj := s.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	var uid uint32
	err := obj.Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, sender).Store(&uid)
	if err != nil {
		return 0, fmt.Errorf("failed to get caller UID: %w", err)
	}
	return uid, nil
}

// getGroupedLightID retrieves the grouped light ID from config
// Returns error if not configured or if hue client is not available
func (s *Service) getGroupedLightID() (string, error) {
	if s.hueClient == nil {
		return "", fmt.Errorf("hue client not initialized")
	}

	s.mu.RLock()
	groupedLightID := s.config.GroupedLightID
	s.mu.RUnlock()

	if groupedLightID == "" {
		return "", fmt.Errorf("no grouped light configured")
	}

	return groupedLightID, nil
}

// checkAccess verifies that the caller is the service owner
// This prevents other users or processes from controlling your lights
func (s *Service) checkAccess(sender dbus.Sender) error {
	callerUID, err := s.getCallerUID(sender)
	if err != nil {
		log.Printf("[WARN] Failed to get caller UID: %v", err)
		return fmt.Errorf("access denied: unable to verify caller identity")
	}

	if callerUID != s.ownerUID {
		log.Printf("[WARN] Access denied: caller UID %d != owner UID %d", callerUID, s.ownerUID)
		return fmt.Errorf("access denied: only the service owner can perform this operation")
	}

	return nil
}

// Introspection methods
func (s *Service) introspectionMethods() []introspect.Method {
	return []introspect.Method{
		{
			Name: "GetStatus",
			Args: []introspect.Arg{
				{Name: "status", Type: "s", Direction: "out"},
			},
		},
		{
			Name: "SetPower",
			Args: []introspect.Arg{
				{Name: "on", Type: "b", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "SetBrightness",
			Args: []introspect.Arg{
				{Name: "brightness", Type: "i", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "ActivateScene",
			Args: []introspect.Arg{
				{Name: "sceneName", Type: "s", Direction: "in"},
				{Name: "result", Type: "s", Direction: "out"},
			},
		},
		{
			Name: "GetScenes",
			Args: []introspect.Arg{
				{Name: "scenes", Type: "as", Direction: "out"},
			},
		},
		{
			Name: "StartSync",
			Args: []introspect.Arg{
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "StopSync",
			Args: []introspect.Arg{
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "IsSyncing",
			Args: []introspect.Arg{
				{Name: "syncing", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetGroupedLights",
			Args: []introspect.Arg{
				{Name: "lights", Type: "a(sss)", Direction: "out"}, // Array of (ID, Name, Type)
			},
		},
		{
			Name: "GetState",
			Args: []introspect.Arg{
				{Name: "power", Type: "b", Direction: "out"},
				{Name: "brightness", Type: "i", Direction: "out"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "SetGroupedLight",
			Args: []introspect.Arg{
				{Name: "groupedLightID", Type: "s", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetConnectionStatus",
			Args: []introspect.Arg{
				{Name: "status", Type: "a{sv}", Direction: "out"}, // Map of string to variant
			},
		},
		{
			Name: "RetryConnection",
			Args: []introspect.Arg{
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetSyncSettings",
			Args: []introspect.Arg{
				{Name: "settings", Type: "a{sv}", Direction: "out"}, // Map of string to variant
			},
		},
		{
			Name: "SetSyncSettings",
			Args: []introspect.Arg{
				{Name: "fps", Type: "i", Direction: "in"},
				{Name: "subsampleWidth", Type: "i", Direction: "in"},
				{Name: "monitor", Type: "s", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "SetGamingMode",
			Args: []introspect.Arg{
				{Name: "enabled", Type: "b", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "IsGamingModeEnabled",
			Args: []introspect.Arg{
				{Name: "enabled", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "IsGamingModeActive",
			Args: []introspect.Arg{
				{Name: "active", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetBridgeSettings",
			Args: []introspect.Arg{
				{Name: "settings", Type: "a{sv}", Direction: "out"}, // Map of string to variant
			},
		},
		{
			Name: "TestBridgeConnection",
			Args: []introspect.Arg{
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetSelectedRoom",
			Args: []introspect.Arg{
				{Name: "roomID", Type: "s", Direction: "out"},
			},
		},
		{
			Name: "SetSelectedRoom",
			Args: []introspect.Arg{
				{Name: "roomID", Type: "s", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetTrayIcons",
			Args: []introspect.Arg{
				{Name: "gaming", Type: "s", Direction: "out"},
				{Name: "syncing", Type: "s", Direction: "out"},
				{Name: "idle", Type: "s", Direction: "out"},
			},
		},
		{
			Name: "SetTrayIcons",
			Args: []introspect.Arg{
				{Name: "gaming", Type: "s", Direction: "in"},
				{Name: "syncing", Type: "s", Direction: "in"},
				{Name: "idle", Type: "s", Direction: "in"},
				{Name: "success", Type: "b", Direction: "out"},
			},
		},
	}
}

// DBus Methods (exported to DBus)

// GetStatus returns the current status
func (s *Service) GetStatus() (string, *dbus.Error) {
	if !s.config.IsConfigured() {
		return "Not configured", nil
	}
	return "Ready", nil
}

// SetPower turns lights on or off
func (s *Service) SetPower(on bool, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can control lights
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetPower access denied")
		return false, dbus.MakeFailedError(err)
	}

	groupedLightID, err := s.getGroupedLightID()
	if err != nil {
		return false, dbus.MakeFailedError(err)
	}

	err = s.hueClient.SetLightPower(groupedLightID, on)
	if err != nil {
		log.Printf("[ERROR] Failed to set power: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("Power set to %v", on)
	return true, nil
}

// SetBrightness sets the brightness (0-100)
func (s *Service) SetBrightness(brightness int32, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can control lights
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetBrightness access denied")
		return false, dbus.MakeFailedError(err)
	}

	if brightness < 0 || brightness > 100 {
		return false, dbus.MakeFailedError(fmt.Errorf("brightness must be 0-100"))
	}

	groupedLightID, err := s.getGroupedLightID()
	if err != nil {
		return false, dbus.MakeFailedError(err)
	}

	err = s.hueClient.SetLightBrightness(groupedLightID, float32(brightness))
	if err != nil {
		log.Printf("[ERROR] Failed to set brightness: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("Brightness set to %d%%", brightness)
	return true, nil
}

// ActivateScene activates a scene by name (with optional room prefix)
func (s *Service) ActivateScene(displayName string, sender dbus.Sender) (string, *dbus.Error) {
	if err := common.ValidateDBusString("displayName", displayName, 255); err != nil {
		log.Printf("[WARN] ActivateScene invalid input: %v", err)
		return "", dbus.MakeFailedError(err)
	}

	// Access control: only service owner can control lights
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] ActivateScene access denied")
		return "", dbus.MakeFailedError(err)
	}

	if s.hueClient == nil {
		return "", dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	// Get all scenes and find matching one
	scenes, err := s.hueClient.GetScenes()
	if err != nil {
		log.Printf("Failed to get scenes: %v", err)
		return "", dbus.MakeFailedError(err)
	}

	// displayName might be "Room - SceneName" or just "SceneName"
	// Try to match against both the full display name and just the scene name
	for _, scene := range scenes {
		// Build the display name for this scene
		sceneDisplayName := scene.Name
		if scene.RoomName != "" {
			sceneDisplayName = scene.RoomName + " - " + scene.Name
		}

		// Match against either the full display name or just the scene name
		if sceneDisplayName == displayName || scene.Name == displayName {
			if err := s.hueClient.ActivateScene(scene.ID); err != nil {
				log.Printf("Failed to activate scene '%s': %v", displayName, err)
				return "", dbus.MakeFailedError(err)
			}
			log.Printf("Activated scene: %s (ID: %s)", displayName, scene.ID)

			// Auto-update GroupedLightID to match the scene's room so that
			// brightness/power controls target the same lights as the scene
			if scene.Room != "" {
				if glID, err := s.hueClient.GetGroupedLightIDForRoom(scene.Room); err == nil {
					s.mu.Lock()
					s.config.GroupedLightID = glID
					saveErr := s.config.Save()
					s.mu.Unlock()
					if saveErr != nil {
						log.Printf("[WARN] Failed to save config after scene activation: %v", saveErr)
					}
					log.Printf("Auto-updated GroupedLightID to %s (room: %s)", glID, scene.Room)
				} else {
					log.Printf("[WARN] Could not find grouped light for scene room %s: %v", scene.Room, err)
				}
			}

			return "Scene activated: " + displayName, nil
		}
	}

	log.Printf("Scene not found: %s", displayName)
	return "", dbus.MakeFailedError(fmt.Errorf("scene not found: %s", displayName))
}

// GetScenes returns list of available scenes with room names
func (s *Service) GetScenes() ([]string, *dbus.Error) {
	if s.hueClient == nil {
		return nil, dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	scenes, err := s.hueClient.GetScenes()
	if err != nil {
		return nil, dbus.MakeFailedError(err)
	}

	names := make([]string, len(scenes))
	for i, scene := range scenes {
		// Format: "RoomName - SceneName" or just "SceneName" if no room
		if scene.RoomName != "" {
			names[i] = scene.RoomName + " - " + scene.Name
		} else {
			names[i] = scene.Name
		}
	}

	return names, nil
}

// GetGroupedLights returns available rooms and zones with grouped lights
func (s *Service) GetGroupedLights() ([]struct{ ID, Name, Type string }, *dbus.Error) {
	if s.hueClient == nil {
		return nil, dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	lights, err := s.hueClient.GetGroupedLights()
	if err != nil {
		log.Printf("[ERROR] Failed to get grouped lights: %v", err)
		return nil, dbus.MakeFailedError(err)
	}

	// Convert to DBus-friendly struct format
	result := make([]struct{ ID, Name, Type string }, len(lights))
	for i, light := range lights {
		result[i] = struct{ ID, Name, Type string }{
			ID:   light.ID,
			Name: light.Name,
			Type: light.Type,
		}
	}

	log.Printf("[INFO] Found %d grouped lights", len(result))
	return result, nil
}

// StartSync starts screen synchronization
func (s *Service) StartSync(sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can start sync
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] StartSync access denied")
		return false, dbus.MakeFailedError(err)
	}

	if s.syncEngine == nil {
		return false, dbus.MakeFailedError(fmt.Errorf("sync engine not available - check Entertainment API configuration"))
	}

	// Pass context.Background() since DBus service runs for application lifetime
	if err := s.syncEngine.Start(context.Background()); err != nil {
		log.Printf("[ERROR] Failed to start sync: %v", err)

		// Check if it's a portal error and provide better error message
		if portalErr, ok := err.(*capture.PortalError); ok {
			// Format: "PortalError:TYPE:HINT" for easy parsing in tray app
			errMsg := "PortalError:" + portalErr.Type + ":" + portalErr.Hint
			return false, dbus.MakeFailedError(fmt.Errorf("%s", errMsg))
		}

		return false, dbus.MakeFailedError(err)
	}

	log.Println("[INFO] Screen sync started")
	return true, nil
}

// StopSync stops screen synchronization
func (s *Service) StopSync(sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can stop sync
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] StopSync access denied")
		return false, dbus.MakeFailedError(err)
	}

	if s.syncEngine == nil {
		return false, dbus.MakeFailedError(fmt.Errorf("sync engine not available"))
	}

	if err := s.syncEngine.Stop(); err != nil {
		log.Printf("[ERROR] Failed to stop sync: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Println("[INFO] Screen sync stopped")
	return true, nil
}

// IsSyncing returns whether sync is active
func (s *Service) IsSyncing() (bool, *dbus.Error) {
	if s.syncEngine == nil {
		return false, nil
	}
	return s.syncEngine.IsRunning(), nil
}

// GetState returns the current power and brightness state
func (s *Service) GetState() (bool, int32, bool, *dbus.Error) {
	groupedLightID, err := s.getGroupedLightID()
	if err != nil {
		return false, 0, false, dbus.MakeFailedError(err)
	}

	power, brightness, err := s.hueClient.GetGroupedLightState(groupedLightID)
	if err != nil {
		log.Printf("[ERROR] Failed to get state: %v", err)
		return false, 0, false, dbus.MakeFailedError(err)
	}

	log.Printf("[INFO] Current state: power=%v, brightness=%.1f", power, brightness)
	// Round brightness to nearest integer instead of truncating
	roundedBrightness := int32(brightness + 0.5)
	return power, roundedBrightness, true, nil
}

// SetGroupedLight sets the grouped light ID in the config
func (s *Service) SetGroupedLight(groupedLightID string, sender dbus.Sender) (bool, *dbus.Error) {
	if err := common.ValidateDBusString("groupedLightID", groupedLightID, 255); err != nil {
		log.Printf("[WARN] SetGroupedLight invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetGroupedLight access denied")
		return false, dbus.MakeFailedError(err)
	}

	if groupedLightID == "" {
		return false, dbus.MakeFailedError(fmt.Errorf("grouped light ID cannot be empty"))
	}

	s.mu.Lock()
	s.config.GroupedLightID = groupedLightID
	err := s.config.Save()
	s.mu.Unlock()

	if err != nil {
		log.Printf("[ERROR] Failed to save config: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("Grouped light ID set to: %s", groupedLightID)
	return true, nil
}

// GetConnectionStatus returns the current bridge connection status
func (s *Service) GetConnectionStatus() (map[string]interface{}, *dbus.Error) {
	if s.hueClient == nil {
		return map[string]interface{}{
			"connected":   false,
			"lastError":   "Hue client not initialized",
			"bridgeIP":    s.config.Bridge,
			"lastAttempt": "",
		}, nil
	}

	status := s.hueClient.GetConnectionStatus()

	lastAttemptStr := ""
	if !status.LastAttempt.IsZero() {
		lastAttemptStr = status.LastAttempt.Format("2006-01-02 15:04:05")
	}

	return map[string]interface{}{
		"connected":   status.Connected,
		"lastError":   status.LastError,
		"bridgeIP":    status.BridgeAddr,
		"lastAttempt": lastAttemptStr,
	}, nil
}

// RetryConnection attempts to reconnect to the bridge
func (s *Service) RetryConnection() (bool, *dbus.Error) {
	if s.hueClient == nil {
		return false, dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	log.Println("[INFO] Retrying bridge connection...")

	reachable, err := s.hueClient.IsReachable()
	if err != nil {
		log.Printf("[ERROR] Retry failed: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	if reachable {
		log.Println("[INFO] Bridge connection restored")
		return true, nil
	}

	return false, dbus.MakeFailedError(fmt.Errorf("bridge still unreachable"))
}

// GetSyncSettings returns current Screen Sync configuration
func (s *Service) GetSyncSettings() (map[string]interface{}, *dbus.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"fps":            s.config.Sync.FPS,
		"subsampleWidth": s.config.Sync.SubsampleWidth,
		"monitor":        s.config.Sync.Monitor,
		"enabled":        s.config.Sync.Enabled,
	}, nil
}

// SetSyncSettings updates Screen Sync configuration
// Note: Changes require restarting sync for them to take effect
func (s *Service) SetSyncSettings(fps int32, subsampleWidth int32, monitor string, sender dbus.Sender) (bool, *dbus.Error) {
	if err := common.ValidateDBusString("monitor", monitor, 255); err != nil {
		log.Printf("[WARN] SetSyncSettings invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetSyncSettings access denied")
		return false, dbus.MakeFailedError(err)
	}

	// Validate FPS
	if fps < capture.MinFPS || fps > capture.MaxFPS {
		return false, dbus.MakeFailedError(fmt.Errorf("FPS must be between %d and %d (got %d)", capture.MinFPS, capture.MaxFPS, fps))
	}

	// Validate subsample width
	if subsampleWidth < config.MinSubsampleWidth || subsampleWidth > config.MaxSubsampleWidth {
		return false, dbus.MakeFailedError(fmt.Errorf("subsample width must be between %d and %d (got %d)", config.MinSubsampleWidth, config.MaxSubsampleWidth, subsampleWidth))
	}

	s.mu.Lock()
	s.config.Sync.FPS = int(fps)
	s.config.Sync.SubsampleWidth = int(subsampleWidth)
	s.config.Sync.Monitor = monitor
	err := s.config.Save()
	s.mu.Unlock()

	if err != nil {
		log.Printf("[ERROR] Failed to save sync settings: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("[INFO] Sync settings updated: FPS=%d, SubsampleWidth=%d, Monitor=%s", fps, subsampleWidth, monitor)
	return true, nil
}

// GetBridgeSettings returns bridge connection information
func (s *Service) GetBridgeSettings() (map[string]interface{}, *dbus.Error) {
	s.mu.RLock()
	bridgeIP := s.config.Bridge
	s.mu.RUnlock()

	var connected bool
	var lastError string

	if s.hueClient != nil {
		status := s.hueClient.GetConnectionStatus()
		connected = status.Connected
		lastError = status.LastError
	} else {
		connected = false
		lastError = "Hue client not initialized"
	}

	return map[string]interface{}{
		"bridgeIP":  bridgeIP,
		"connected": connected,
		"lastError": lastError,
	}, nil
}

// TestBridgeConnection tests connectivity to the bridge
func (s *Service) TestBridgeConnection() (bool, *dbus.Error) {
	if s.hueClient == nil {
		return false, dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	log.Println("[INFO] Testing bridge connection...")

	reachable, err := s.hueClient.IsReachable()
	if err != nil {
		log.Printf("[ERROR] Bridge test failed: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	if reachable {
		log.Println("[INFO] Bridge is reachable")
		return true, nil
	}

	return false, dbus.MakeFailedError(fmt.Errorf("bridge is not reachable"))
}

// GetSelectedRoom returns the currently selected room/zone for light control
func (s *Service) GetSelectedRoom() (string, *dbus.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.GroupedLightID, nil
}

// SetSelectedRoom updates the room/zone selection
func (s *Service) SetSelectedRoom(roomID string, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetSelectedRoom access denied")
		return false, dbus.MakeFailedError(err)
	}

	if roomID == "" {
		return false, dbus.MakeFailedError(fmt.Errorf("room ID cannot be empty"))
	}

	s.mu.Lock()
	s.config.GroupedLightID = roomID
	err := s.config.Save()
	s.mu.Unlock()

	if err != nil {
		log.Printf("[ERROR] Failed to save room selection: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("[INFO] Selected room set to: %s", roomID)
	return true, nil
}

// SetGamingMode enables or disables gaming mode auto-sync
func (s *Service) SetGamingMode(sender dbus.Sender, enabled bool) (bool, *dbus.Error) {
	// Check access control
	if err := s.checkAccess(sender); err != nil {
		return false, dbus.MakeFailedError(err)
	}

	s.mu.Lock()

	// Update config
	s.config.GamingMode.Enabled = enabled

	// Save config
	if err := s.config.Save(); err != nil {
		s.mu.Unlock()
		log.Printf("❌ Failed to save gaming mode config: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	s.mu.Unlock()

	// IMPORTANT: Actually start/stop the detector!
	if enabled {
		// Stop existing detector if running
		s.StopGamingMode()

		// Start new detector
		s.InitGamingMode()
		log.Println("🎮 Gaming mode enabled - detector started")
	} else {
		// Stop detector
		s.StopGamingMode()
		log.Println("🎮 Gaming mode disabled - detector stopped")
	}

	return true, nil
}

// IsGamingModeEnabled returns whether gaming mode is enabled in config
func (s *Service) IsGamingModeEnabled() (bool, *dbus.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.GamingMode.Enabled, nil
}

// IsGamingModeActive returns whether gaming mode is currently detecting gaming activity
func (s *Service) IsGamingModeActive() (bool, *dbus.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.gamingDetector == nil {
		return false, nil
	}

	return s.gamingDetector.IsGaming(), nil
}

// InitGamingMode initializes the gaming detector if enabled in config
// Should be called from main() after service is created
func (s *Service) InitGamingMode() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.config.GamingMode.Enabled {
		log.Println("ℹ️  Gaming mode disabled in config")
		return
	}

	if s.syncEngine == nil {
		log.Println("⚠️  Gaming mode requires Entertainment API configuration")
		return
	}

	// Create gaming detector config
	gamingCfg := gaming.Config{
		PollInterval:      time.Duration(s.config.GamingMode.PollInterval) * time.Second,
		DebounceDelay:     time.Duration(s.config.GamingMode.DebounceDelay) * time.Second,
		UseSystemdInhibit: s.config.GamingMode.UseSystemdInhibit,
		UsePowerProfile:   s.config.GamingMode.UsePowerProfile,
		UseSteamAppId:     s.config.GamingMode.UseSteamAppId,
		UseGameMode:       s.config.GamingMode.UseGameMode,
		UseFullscreen:     s.config.GamingMode.UseFullscreen,
	}

	// Create detector with callback
	detector, err := gaming.NewDetector(gamingCfg, func(isGaming bool) {
		s.onGamingStateChanged(isGaming)
	})

	if err != nil {
		log.Printf("❌ Failed to create gaming detector: %v", err)
		return
	}

	if detector == nil {
		log.Println("⚠️  No gaming detection methods available")
		return
	}

	s.gamingDetector = detector
	s.gamingDetector.Start()
	log.Println("✅ Gaming mode detector started")
}

// StopGamingMode stops the gaming detector
func (s *Service) StopGamingMode() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.gamingDetector != nil {
		s.gamingDetector.Close()
		s.gamingDetector = nil
		log.Println("🎮 Gaming mode detector stopped")
	}
}

// onGamingStateChanged is called when gaming state changes
func (s *Service) onGamingStateChanged(isGaming bool) {
	// Check state and determine action while holding lock
	s.mu.Lock()
	shouldStart := isGaming && s.syncEngine != nil && !s.gamingModeActive
	shouldStop := !isGaming && s.gamingModeActive && s.syncEngine != nil

	if isGaming {
		s.gamingModeActive = true
	} else {
		s.gamingModeActive = false
	}
	engine := s.syncEngine
	s.mu.Unlock()

	// Perform sync operations WITHOUT holding lock to avoid deadlock
	if shouldStart {
		log.Println("🎮 Gaming detected - starting screen sync")
		// Use Background context - sync engine manages its own lifecycle via Stop()
		if err := engine.Start(context.Background()); err != nil {
			log.Printf("❌ Failed to start sync for gaming mode: %v", err)
		} else {
			log.Println("✅ Screen sync enabled for immersive gaming")
		}
	} else if shouldStop {
		log.Println("🎮 Gaming stopped - stopping screen sync")
		engine.Stop()
		log.Println("✅ Screen sync disabled")
	}
}

// GetTrayIcons returns the configured tray icon names
func (s *Service) GetTrayIcons() (string, string, string, *dbus.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.UI.Icons.Gaming, s.config.UI.Icons.Syncing, s.config.UI.Icons.Idle, nil
}

// SetTrayIcons updates the tray icon configuration
func (s *Service) SetTrayIcons(gaming string, syncing string, idle string, sender dbus.Sender) (bool, *dbus.Error) {
	// SEC-007: Validate DBus string inputs
	if err := common.ValidateDBusString("gaming", gaming, 255); err != nil {
		log.Printf("🚫 SetTrayIcons invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}
	if err := common.ValidateDBusString("syncing", syncing, 255); err != nil {
		log.Printf("🚫 SetTrayIcons invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}
	if err := common.ValidateDBusString("idle", idle, 255); err != nil {
		log.Printf("🚫 SetTrayIcons invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update config
	s.config.UI.Icons.Gaming = gaming
	s.config.UI.Icons.Syncing = syncing
	s.config.UI.Icons.Idle = idle

	// Save to file
	if err := s.config.Save(); err != nil {
		log.Printf("❌ Failed to save tray icon settings: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("✅ Tray icons updated: Gaming=%s, Syncing=%s, Idle=%s", gaming, syncing, idle)
	return true, nil
}
