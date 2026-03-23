package dbus

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
)

const (
	dbusName      = "org.kde.plasma.hue"
	dbusPath      = "/org/kde/plasma/hue"
	dbusInterface = "org.kde.plasma.hue"
)

// Service provides DBus interface for the plasmoid
type Service struct {
	conn      *dbus.Conn
	config    *config.Config
	hueClient *hue.Client
}

// NewService creates a new DBus service
func NewService(cfg *config.Config, client *hue.Client) (*Service, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	return &Service{
		conn:      conn,
		config:    cfg,
		hueClient: client,
	}, nil
}

// Start begins the DBus service
func (s *Service) Start() error {
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
	return nil
}

// Stop stops the DBus service
func (s *Service) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
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
func (s *Service) SetPower(on bool) (bool, *dbus.Error) {
	if s.hueClient == nil {
		return false, dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	// TODO: Get the actual grouped light ID from config
	// For now, this is a placeholder
	log.Printf("SetPower called: %v (not implemented yet - need grouped light ID)", on)
	return false, dbus.MakeFailedError(fmt.Errorf("not implemented: need to configure grouped light ID"))
}

// SetBrightness sets the brightness (0-100)
func (s *Service) SetBrightness(brightness int32) (bool, *dbus.Error) {
	if s.hueClient == nil {
		return false, dbus.MakeFailedError(fmt.Errorf("hue client not initialized"))
	}

	if brightness < 0 || brightness > 100 {
		return false, dbus.MakeFailedError(fmt.Errorf("brightness must be 0-100"))
	}

	// TODO: Get the actual grouped light ID from config
	log.Printf("SetBrightness called: %d (not implemented yet - need grouped light ID)", brightness)
	return false, dbus.MakeFailedError(fmt.Errorf("not implemented: need to configure grouped light ID"))
}

// ActivateScene activates a scene by name (with optional room prefix)
func (s *Service) ActivateScene(displayName string) (string, *dbus.Error) {
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
			log.Printf("✓ Activated scene: %s (ID: %s)", displayName, scene.ID)
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

// StartSync starts screen synchronization
func (s *Service) StartSync() (bool, *dbus.Error) {
	// TODO: Implement sync engine
	log.Println("StartSync called (not implemented yet)")
	return false, dbus.MakeFailedError(fmt.Errorf("sync not implemented yet"))
}

// StopSync stops screen synchronization
func (s *Service) StopSync() (bool, *dbus.Error) {
	// TODO: Implement sync engine
	log.Println("StopSync called (not implemented yet)")
	return false, dbus.MakeFailedError(fmt.Errorf("sync not implemented yet"))
}

// IsSyncing returns whether sync is active
func (s *Service) IsSyncing() (bool, *dbus.Error) {
	// TODO: Implement sync engine state
	return false, nil
}
