package dbus

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/color"
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
	gamingModeActive bool // Track if gaming mode triggered sync
	// Cancels the supervisor watching the sync session gaming mode started.
	// Only ever set while gamingModeActive is true, and cleared whenever that
	// goes false, all under mu, so a supervisor never outlives its game.
	stopGamingSupervisor context.CancelCauseFunc
	mu                   sync.RWMutex // Guards the gaming state; config has its own lock
	ownerUID             uint32       // UID of the service owner for access control
	// callerUID resolves a DBus sender to its UID. Set to getCallerUID by
	// NewService; a nil value (e.g. a directly-constructed Service in tests)
	// makes checkAccess fail closed rather than allow or fall back.
	callerUID func(dbus.Sender) (uint32, error)
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

	s := &Service{
		conn:       conn,
		config:     cfg,
		hueClient:  client,
		syncEngine: engine,
		ownerUID:   uint32(os.Getuid()), // Store owner UID for access control
	}
	s.callerUID = s.getCallerUID
	return s, nil
}

// Start begins the DBus service
func (s *Service) Start() error {
	var success bool
	defer func() {
		if !success && s.conn != nil {
			if err := s.conn.Close(); err != nil {
				log.Printf("warn: failed to close DBus connection on startup failure: %v", err)
			}
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
		if _, err := s.conn.ReleaseName(dbusName); err != nil {
			log.Printf("warn: failed to release DBus name: %v", err)
		}
		if err := s.conn.Close(); err != nil {
			log.Printf("warn: failed to close DBus connection: %v", err)
		}
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

	var groupedLightID string
	s.config.View(func(c *config.Config) {
		groupedLightID = c.GroupedLightID
	})

	if groupedLightID == "" {
		return "", fmt.Errorf("no grouped light configured")
	}

	return groupedLightID, nil
}

// checkAccess verifies that the caller is the service owner
// This prevents other users or processes from controlling your lights
func (s *Service) checkAccess(sender dbus.Sender) error {
	if s.callerUID == nil {
		log.Printf("[WARN] Access denied: no caller UID resolver configured")
		return fmt.Errorf("access denied: unable to verify caller identity")
	}

	callerUID, err := s.callerUID(sender)
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
			Name: "GetStartupScene",
			Args: []introspect.Arg{
				{Name: "sceneName", Type: "s", Direction: "out"},
			},
		},
		{
			Name: "SetStartupScene",
			Args: []introspect.Arg{
				{Name: "sceneName", Type: "s", Direction: "in"},
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
	var configured bool
	s.config.View(func(c *config.Config) {
		configured = c.IsConfigured()
	})
	if !configured {
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
	// Access control: only service owner can control lights
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] ActivateScene access denied")
		return "", dbus.MakeFailedError(err)
	}

	if err := common.ValidateDBusString("displayName", displayName, 255); err != nil {
		log.Printf("[WARN] ActivateScene invalid input: %v", err)
		return "", dbus.MakeFailedError(err)
	}

	scene, err := s.activateSceneByDisplayName(displayName)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}

	// Auto-update GroupedLightID to match the scene's room so that
	// brightness/power controls target the same lights as the scene.
	if scene.GroupedLightID != "" {
		saveErr := s.config.Update(func(c *config.Config) {
			c.GroupedLightID = scene.GroupedLightID
		}, nil)
		if saveErr != nil {
			log.Printf("[WARN] Failed to save config after scene activation: %v", saveErr)
		}
		log.Printf("Auto-updated GroupedLightID to %s (room: %s)", scene.GroupedLightID, scene.RoomName)
	}

	return "Scene activated: " + displayName, nil
}

// ActivateStartupScene is invoked directly from main() at boot, not over
// DBus, since the process owner is inherently trusted. It leaves
// GroupedLightID untouched so it doesn't silently override a room the user
// separately picked for brightness/power control.
func (s *Service) ActivateStartupScene() error {
	var displayName string
	s.config.View(func(c *config.Config) {
		displayName = c.StartupScene
	})

	if displayName == "" {
		return nil
	}

	_, err := s.activateSceneByDisplayName(displayName)
	return err
}

// activateSceneByDisplayName matches displayName against "Room - SceneName"
// or bare "SceneName" as returned by GetScenes, and activates it on the bridge.
func (s *Service) activateSceneByDisplayName(displayName string) (hue.Scene, error) {
	if s.hueClient == nil {
		return hue.Scene{}, fmt.Errorf("hue client not initialized")
	}

	// Get all scenes and find matching one
	scenes, err := s.hueClient.GetScenes()
	if err != nil {
		log.Printf("Failed to get scenes: %v", err)
		return hue.Scene{}, err
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
				return hue.Scene{}, err
			}
			log.Printf("Activated scene: %s (ID: %s)", displayName, scene.ID)
			return scene, nil
		}
	}

	log.Printf("Scene not found: %s", displayName)
	return hue.Scene{}, fmt.Errorf("scene not found: %s", displayName)
}

// GetScenes returns list of available scenes with room names
func (s *Service) GetScenes(sender dbus.Sender) ([]string, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetScenes access denied")
		return nil, dbus.MakeFailedError(err)
	}

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
func (s *Service) GetGroupedLights(sender dbus.Sender) ([]struct{ ID, Name, Type string }, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetGroupedLights access denied")
		return nil, dbus.MakeFailedError(err)
	}

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
func (s *Service) GetState(sender dbus.Sender) (bool, int32, bool, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetState access denied")
		return false, 0, false, dbus.MakeFailedError(err)
	}

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
	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetGroupedLight access denied")
		return false, dbus.MakeFailedError(err)
	}

	if err := common.ValidateDBusString("groupedLightID", groupedLightID, 255); err != nil {
		log.Printf("[WARN] SetGroupedLight invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	if groupedLightID == "" {
		return false, dbus.MakeFailedError(fmt.Errorf("grouped light ID cannot be empty"))
	}

	err := s.config.Update(func(c *config.Config) {
		c.GroupedLightID = groupedLightID
	}, nil)

	if err != nil {
		log.Printf("[ERROR] Failed to save config: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("Grouped light ID set to: %s", groupedLightID)
	return true, nil
}

// GetConnectionStatus returns the current bridge connection status
func (s *Service) GetConnectionStatus(sender dbus.Sender) (map[string]interface{}, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetConnectionStatus access denied")
		return nil, dbus.MakeFailedError(err)
	}

	if s.hueClient == nil {
		var bridgeIP string
		s.config.View(func(c *config.Config) {
			bridgeIP = c.Bridge
		})
		return map[string]interface{}{
			"connected":   false,
			"lastError":   "Hue client not initialized",
			"bridgeIP":    bridgeIP,
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
func (s *Service) RetryConnection(sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can trigger bridge I/O
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] RetryConnection access denied")
		return false, dbus.MakeFailedError(err)
	}

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
	var settings config.SyncConfig
	s.config.View(func(c *config.Config) {
		settings = c.Sync
	})

	return map[string]interface{}{
		"fps":            settings.FPS,
		"subsampleWidth": settings.SubsampleWidth,
		"monitor":        settings.Monitor,
		"enabled":        settings.Enabled,
	}, nil
}

// SetSyncSettings updates Screen Sync configuration.
// FPS applies immediately, including to a running sync loop; subsampleWidth
// takes effect on the next sync start. Monitor is persisted but not yet
// honored: NewEngine always captures all monitors.
func (s *Service) SetSyncSettings(fps int32, subsampleWidth int32, monitor string, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetSyncSettings access denied")
		return false, dbus.MakeFailedError(err)
	}

	if err := common.ValidateDBusString("monitor", monitor, 255); err != nil {
		log.Printf("[WARN] SetSyncSettings invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	// Validate FPS
	if fps < capture.MinFPS || fps > capture.MaxFPS {
		return false, dbus.MakeFailedError(fmt.Errorf("FPS must be between %d and %d (got %d)", capture.MinFPS, capture.MaxFPS, fps))
	}

	// Validate subsample width
	if subsampleWidth < color.MinSubsampleWidth || subsampleWidth > color.MaxSubsampleWidth {
		return false, dbus.MakeFailedError(fmt.Errorf("subsample width must be between %d and %d (got %d)", color.MinSubsampleWidth, color.MaxSubsampleWidth, subsampleWidth))
	}

	var prev config.SyncConfig
	err := s.config.Update(func(c *config.Config) {
		prev = c.Sync
		c.Sync.FPS = int(fps)
		c.Sync.SubsampleWidth = int(subsampleWidth)
		c.Sync.Monitor = monitor
	}, func(c *config.Config) {
		c.Sync = prev
	})
	if err != nil {
		// Save writes the file before it chmods it, so a late failure can
		// leave the new values on disk under the reverted in-memory ones.
		log.Printf("[ERROR] Failed to save sync settings, reverted in memory (the file on disk may already hold the new values): %v", err)
		return false, dbus.MakeFailedError(err)
	}

	if s.syncEngine != nil {
		if err := s.syncEngine.SetFPS(int(fps)); err != nil {
			log.Printf("[ERROR] Failed to apply FPS to sync engine: %v", err)
			return false, dbus.MakeFailedError(err)
		}
	}

	log.Printf("[INFO] Sync settings updated: FPS=%d, SubsampleWidth=%d, Monitor=%s", fps, subsampleWidth, monitor)
	return true, nil
}

// GetBridgeSettings returns bridge connection information
func (s *Service) GetBridgeSettings(sender dbus.Sender) (map[string]interface{}, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetBridgeSettings access denied")
		return nil, dbus.MakeFailedError(err)
	}

	var bridgeIP string
	s.config.View(func(c *config.Config) {
		bridgeIP = c.Bridge
	})

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
func (s *Service) TestBridgeConnection(sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can trigger bridge I/O
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] TestBridgeConnection access denied")
		return false, dbus.MakeFailedError(err)
	}

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
func (s *Service) GetSelectedRoom(sender dbus.Sender) (string, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetSelectedRoom access denied")
		return "", dbus.MakeFailedError(err)
	}

	var roomID string
	s.config.View(func(c *config.Config) {
		roomID = c.GroupedLightID
	})
	return roomID, nil
}

// SetSelectedRoom updates the room/zone selection
func (s *Service) SetSelectedRoom(roomID string, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetSelectedRoom access denied")
		return false, dbus.MakeFailedError(err)
	}

	if err := common.ValidateDBusString("roomID", roomID, 255); err != nil {
		log.Printf("[WARN] SetSelectedRoom invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	if roomID == "" {
		return false, dbus.MakeFailedError(fmt.Errorf("room ID cannot be empty"))
	}

	err := s.config.Update(func(c *config.Config) {
		c.GroupedLightID = roomID
	}, nil)

	if err != nil {
		log.Printf("[ERROR] Failed to save room selection: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("[INFO] Selected room set to: %s", roomID)
	return true, nil
}

// GetStartupScene returns the configured startup scene display name.
func (s *Service) GetStartupScene(sender dbus.Sender) (string, *dbus.Error) {
	// Access control: only service owner can read bridge-derived data
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] GetStartupScene access denied")
		return "", dbus.MakeFailedError(err)
	}

	var displayName string
	s.config.View(func(c *config.Config) {
		displayName = c.StartupScene
	})
	return displayName, nil
}

// SetStartupScene clears the startup scene when displayName is empty.
func (s *Service) SetStartupScene(displayName string, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetStartupScene access denied")
		return false, dbus.MakeFailedError(err)
	}

	if err := common.ValidateDBusString("displayName", displayName, 255); err != nil {
		log.Printf("[WARN] SetStartupScene invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	err := s.config.Update(func(c *config.Config) {
		c.StartupScene = displayName
	}, nil)

	if err != nil {
		log.Printf("[ERROR] Failed to save startup scene: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("[INFO] Startup scene set to: %q", displayName)
	return true, nil
}

// SetGamingMode enables or disables gaming mode auto-sync
func (s *Service) SetGamingMode(sender dbus.Sender, enabled bool) (bool, *dbus.Error) {
	// Check access control
	if err := s.checkAccess(sender); err != nil {
		return false, dbus.MakeFailedError(err)
	}

	err := s.config.Update(func(c *config.Config) {
		c.GamingMode.Enabled = enabled
	}, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to save gaming mode config: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	// IMPORTANT: Actually start/stop the detector!
	if enabled {
		// Stop existing detector if running
		s.StopGamingMode()

		// Start new detector
		s.InitGamingMode()
		log.Println("[INFO] Gaming mode enabled - detector started")
	} else {
		s.disableGamingMode()
		log.Println("[INFO] Gaming mode disabled - detector stopped")
	}

	return true, nil
}

// IsGamingModeEnabled returns whether gaming mode is enabled in config
func (s *Service) IsGamingModeEnabled() (bool, *dbus.Error) {
	var enabled bool
	s.config.View(func(c *config.Config) {
		enabled = c.GamingMode.Enabled
	})
	return enabled, nil
}

// IsGamingModeActive returns whether gaming mode is currently detecting gaming activity
func (s *Service) IsGamingModeActive() (bool, *dbus.Error) {
	s.mu.RLock()
	detector := s.gamingDetector
	s.mu.RUnlock()

	// Detection execs subprocesses, and every gaming state change waits on mu.
	return detector.IsGaming(), nil
}

// InitGamingMode initializes the gaming detector if enabled in config
// Should be called from main() after service is created
func (s *Service) InitGamingMode() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var gm config.GamingModeConfig
	s.config.View(func(c *config.Config) {
		gm = c.GamingMode
	})

	if !gm.Enabled {
		log.Println("[INFO] Gaming mode disabled in config")
		return
	}

	if s.syncEngine == nil {
		log.Println("[WARN] Gaming mode requires Entertainment API configuration")
		return
	}

	// Create gaming detector config
	gamingCfg := gaming.Config{
		PollInterval:      time.Duration(gm.PollInterval) * time.Second,
		DebounceDelay:     time.Duration(gm.DebounceDelay) * time.Second,
		UseSystemdInhibit: gm.UseSystemdInhibit,
		UsePowerProfile:   gm.UsePowerProfile,
		UseSteamAppId:     gm.UseSteamAppId,
		UseGameMode:       gm.UseGameMode,
		UseFullscreen:     gm.UseFullscreen,
		InitiallyGaming:   s.gamingModeActive,
	}

	// Create detector with callback. The callback names its detector so a
	// late one from a detector already shut down can be told apart: Close
	// does not wait for a check already in flight.
	var detector *gaming.Detector
	detector, err := gaming.NewDetector(gamingCfg, func(isGaming bool) {
		s.onGamingStateChanged(detector, isGaming)
	})

	if err != nil {
		log.Printf("[ERROR] Failed to create gaming detector: %v", err)
		return
	}

	if detector == nil {
		log.Println("[WARN] No gaming detection methods available")
		return
	}

	s.gamingDetector = detector
	s.gamingDetector.Start()
	log.Println("[INFO] Gaming mode detector started")

	// Seeded mid-game, the detector reports nothing while that game goes on,
	// so the supervisor the old detector's game had is recreated here rather
	// than waiting on a callback that will not come.
	if s.gamingModeActive && s.stopGamingSupervisor == nil {
		ctx, cancel := context.WithCancelCause(context.Background())
		s.stopGamingSupervisor = cancel
		go s.superviseGamingSync(ctx, s.syncEngine)
	}
}

// StopGamingMode stops the gaming detector
func (s *Service) StopGamingMode() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopGamingModeLocked()
}

// stopGamingModeLocked is StopGamingMode for a caller already holding s.mu.
func (s *Service) stopGamingModeLocked() {
	if s.gamingDetector != nil {
		s.gamingDetector.Close()
		s.gamingDetector = nil
		log.Println("[INFO] Gaming mode detector stopped")
	}

	// The session and gamingModeActive are left as they are: gaming mode
	// being reconfigured is not the game ending. A detector that replaces
	// this one is seeded with gamingModeActive, so it can still report that
	// game ending, and a late game-ended callback from this one still stops
	// it. Disabling gaming mode is what hands the session back.
	if s.stopGamingSupervisor != nil {
		s.stopGamingSupervisor(errDetectorStopped)
		s.stopGamingSupervisor = nil
	}
}

// disableGamingMode stops the detector and hands the session back. Any running
// session is left running, but gaming mode no longer owns it: with no detector,
// nothing would report the game ending, and a stale gamingModeActive would stop
// the next game after gaming mode is re-enabled from ever starting sync.
//
// One critical section for both: between them a concurrent enable would see the
// game still on, seed a new detector with it and start a supervisor, which this
// would then orphan by clearing the flag underneath.
func (s *Service) disableGamingMode() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopGamingModeLocked()
	s.gamingModeActive = false
}

// Why a gaming supervisor was cancelled. Only the game ending means a session
// the supervisor has just started should be undone.
var (
	errGamingEnded     = errors.New("gaming ended")
	errDetectorStopped = errors.New("gaming detector stopped")
)

// Recovery limits for a sync session gaming mode started. Capture can now end
// a session on its own, and the gaming detector is edge triggered, so without
// this a stream that dies mid-game stays dead until the game does.
//
// Bounded and backed off because the cause may be a screen share the user
// revoked, which the capture layer cannot tell from a transient failure: every
// attempt then re-prompts the portal, so there must be few of them. The budget
// is restored once a session has held up, so an evening of hotplugs does not
// accumulate into a refusal to retry.
const (
	gamingRestartAttempts = 3
	gamingRestartBackoff  = 5 * time.Second
	gamingRestartHealthy  = time.Minute
	gamingSupervisorPoll  = 2 * time.Second
)

// gamingRestartDelay is how long to wait after the given attempt, counting from
// one, before making the next. The first attempt goes out on the next poll; the
// waits then double so the retries spread out, because each one may re-prompt
// the portal when the cause is a revoked screen share.
func gamingRestartDelay(attempt int) time.Duration {
	return gamingRestartBackoff * time.Duration(1<<(attempt-1))
}

// restartDecision is what the supervisor should do on one tick.
type restartDecision int

const (
	restartWait    restartDecision = iota // nothing to do
	restartNow                            // bring the session back
	restartExhaust                        // gave up, and has not said so yet
)

// decideRestart separates the three states that look alike from outside: a
// session that is running, one the user or the detector stopped deliberately,
// and one capture ended by itself. Only the last is ours to undo.
func decideRestart(running bool, failure error, attempts int, now, nextAttemptAt time.Time) restartDecision {
	if running || failure == nil {
		return restartWait
	}
	if attempts >= gamingRestartAttempts {
		return restartExhaust
	}
	if now.Before(nextAttemptAt) {
		return restartWait
	}
	return restartNow
}

// gamingEnded reports whether ctx was cancelled because the game ended, as
// opposed to the detector being reconfigured, which leaves the session alone.
func gamingEnded(ctx context.Context) bool {
	return ctx != nil && errors.Is(context.Cause(ctx), errGamingEnded)
}

// startSyncForGaming starts sync on behalf of gaming mode, and undoes it if the
// game ended while Start was running. The detector runs its callbacks
// concurrently and Start can block for seconds on the portal and the bridge, so
// the game-ended handler can finish before this Start does. Its Stop then found
// nothing to stop, and the session it would have ended outlives the game. The
// undo is by session, not a plain Stop: by the time Start returns, another
// start can be queued behind it, and that session is not this call's to end.
//
// The session is started on its own context, not ctx: ctx only decides whether
// to keep it, and a session tied to it would be stranded, still reported as
// running, the moment the supervisor was cancelled.
func startSyncForGaming(ctx context.Context, engine *syncengine.Engine) error {
	gen, err := engine.StartSession(context.Background())
	if err != nil {
		return err
	}
	if !gamingEnded(ctx) {
		return nil
	}
	if err := engine.StopSession(gen); err != nil && !errors.Is(err, syncengine.ErrNotRunning) {
		log.Printf("warn: failed to stop sync started after gaming ended: %v", err)
	}
	return errGamingEnded
}

// superviseGamingSync brings back a sync session that capture ended while a
// game is still running.
func (s *Service) superviseGamingSync(ctx context.Context, engine *syncengine.Engine) {
	ticker := time.NewTicker(gamingSupervisorPoll)
	defer ticker.Stop()

	var (
		attempts      int
		healthySince  time.Time
		nextAttemptAt time.Time
		saidGaveUp    bool
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		now := time.Now()
		running := engine.IsRunning()

		if running {
			if healthySince.IsZero() {
				healthySince = now
			}
			if now.Sub(healthySince) >= gamingRestartHealthy {
				attempts = 0
				saidGaveUp = false
			}
			continue
		}
		healthySince = time.Time{}

		switch decideRestart(running, engine.LastFailure(), attempts, now, nextAttemptAt) {
		case restartWait:
			continue
		case restartExhaust:
			if !saidGaveUp {
				log.Printf("[ERROR] Screen sync failed %d times during gaming, not retrying again: %v",
					attempts, engine.LastFailure())
				saidGaveUp = true
			}
			continue
		}

		attempts++
		nextAttemptAt = now.Add(gamingRestartDelay(attempts))
		log.Printf("[INFO] Screen sync stopped during gaming (%v), restarting (attempt %d of %d)",
			engine.LastFailure(), attempts, gamingRestartAttempts)

		err := startSyncForGaming(ctx, engine)
		if errors.Is(err, errGamingEnded) {
			return
		}
		// Someone started it between the check above and this call.
		if errors.Is(err, syncengine.ErrAlreadyRunning) {
			continue
		}
		if err != nil {
			log.Printf("[ERROR] Failed to restart sync during gaming: %v", err)
		}
	}
}

// onGamingStateChanged is called when gaming state changes
func (s *Service) onGamingStateChanged(src *gaming.Detector, isGaming bool) {
	// Check state and determine action while holding lock. The supervisor is
	// created and cancelled here too, in the same critical section as
	// gamingModeActive, so a game-ended callback that overtakes a slow
	// game-started one always finds the supervisor it has to cancel.
	s.mu.Lock()
	// A game starting is only acted on from the current detector. One already
	// shut down would otherwise start sync, and a supervisor nothing cancels,
	// with gaming mode switched off. A game ending is still honoured from any
	// detector: it is a real observation, and a late one from the replaced
	// detector can arrive before the new one has seen the same end.
	if isGaming && src != s.gamingDetector {
		s.mu.Unlock()
		return
	}
	engine := s.syncEngine
	shouldStart := isGaming && engine != nil && !s.gamingModeActive
	shouldStop := !isGaming && s.gamingModeActive && engine != nil
	s.gamingModeActive = isGaming

	var superviseCtx context.Context
	if isGaming && engine != nil && s.stopGamingSupervisor == nil {
		ctx, cancel := context.WithCancelCause(context.Background())
		s.stopGamingSupervisor = cancel
		superviseCtx = ctx
	}

	var cancelSupervisor context.CancelCauseFunc
	if !isGaming {
		cancelSupervisor = s.stopGamingSupervisor
		s.stopGamingSupervisor = nil
	}
	s.mu.Unlock()

	// Before the Stop below, or the supervisor would read the stopped session
	// as one to bring back.
	if cancelSupervisor != nil {
		cancelSupervisor(errGamingEnded)
	}

	// Perform sync operations WITHOUT holding lock to avoid deadlock
	if shouldStart {
		log.Println("[INFO] Gaming detected - starting screen sync")
		// A failed Start records its error, so the supervisor retries it.
		err := startSyncForGaming(superviseCtx, engine)
		switch {
		case errors.Is(err, errGamingEnded):
			log.Println("[INFO] Gaming ended while sync was starting - stopped it again")
		case errors.Is(err, syncengine.ErrAlreadyRunning):
			log.Println("[INFO] Screen sync already running - gaming mode taking it over")
		case err != nil:
			log.Printf("[ERROR] Failed to start sync for gaming mode: %v", err)
		default:
			log.Println("[INFO] Screen sync enabled for immersive gaming")
		}
	}

	// Started after the initial Start so the two never race to bring the
	// session up. If the game has already ended, ctx is cancelled and it
	// exits on its first check.
	if superviseCtx != nil {
		go s.superviseGamingSync(superviseCtx, engine)
	}

	if shouldStop {
		log.Println("[INFO] Gaming stopped - stopping screen sync")
		// Not an error: capture can end the session on its own, so by the
		// time gaming stops there may be nothing left to stop.
		if err := engine.Stop(); err != nil && !errors.Is(err, syncengine.ErrNotRunning) {
			log.Printf("warn: failed to stop sync engine: %v", err)
		}
		log.Println("[INFO] Screen sync disabled")
	}
}

// GetTrayIcons returns the configured tray icon names
func (s *Service) GetTrayIcons() (string, string, string, *dbus.Error) {
	var icons config.IconConfig
	s.config.View(func(c *config.Config) {
		icons = c.UI.Icons
	})
	return icons.Gaming, icons.Syncing, icons.Idle, nil
}

// SetTrayIcons updates the tray icon configuration
func (s *Service) SetTrayIcons(gaming string, syncing string, idle string, sender dbus.Sender) (bool, *dbus.Error) {
	// Access control: only service owner can modify settings
	if err := s.checkAccess(sender); err != nil {
		log.Printf("[WARN] SetTrayIcons access denied")
		return false, dbus.MakeFailedError(err)
	}

	// SEC-007: Validate DBus string inputs
	if err := common.ValidateDBusString("gaming", gaming, 255); err != nil {
		log.Printf("[WARN] SetTrayIcons invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}
	if err := common.ValidateDBusString("syncing", syncing, 255); err != nil {
		log.Printf("[WARN] SetTrayIcons invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}
	if err := common.ValidateDBusString("idle", idle, 255); err != nil {
		log.Printf("[WARN] SetTrayIcons invalid input: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	err := s.config.Update(func(c *config.Config) {
		c.UI.Icons.Gaming = gaming
		c.UI.Icons.Syncing = syncing
		c.UI.Icons.Idle = idle
	}, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to save tray icon settings: %v", err)
		return false, dbus.MakeFailedError(err)
	}

	log.Printf("[INFO] Tray icons updated: Gaming=%s, Syncing=%s, Idle=%s", gaming, syncing, idle)
	return true, nil
}
