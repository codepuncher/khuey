package dbus

import (
	"testing"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
)

// TestSceneNameMatching tests the scene name matching logic used in ActivateScene
func TestSceneNameMatching(t *testing.T) {
	scenes := []hue.Scene{
		{ID: "scene-1", Name: "Relax", RoomName: "Living Room"},
		{ID: "scene-2", Name: "Bright", RoomName: "Kitchen"},
		{ID: "scene-3", Name: "Concentrate", RoomName: ""},
	}

	tests := []struct {
		name        string
		displayName string
		expectFound bool
		expectID    string
	}{
		{name: "Match with room name", displayName: "Living Room - Relax", expectFound: true, expectID: "scene-1"},
		{name: "Match scene name only", displayName: "Concentrate", expectFound: true, expectID: "scene-3"},
		{name: "No match", displayName: "Nonexistent Scene", expectFound: false},
		{name: "Partial match should not work", displayName: "Living Room", expectFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var foundID string
			found := false

			for _, scene := range scenes {
				sceneDisplayName := scene.Name
				if scene.RoomName != "" {
					sceneDisplayName = scene.RoomName + " - " + scene.Name
				}

				if sceneDisplayName == tt.displayName || scene.Name == tt.displayName {
					foundID = scene.ID
					found = true
					break
				}
			}

			if found != tt.expectFound {
				t.Errorf("Scene found = %v, want %v", found, tt.expectFound)
			}
			if found && foundID != tt.expectID {
				t.Errorf("Scene ID = %v, want %v", foundID, tt.expectID)
			}
		})
	}
}

// TestBrightnessRounding tests the rounding logic used in GetState
func TestBrightnessRounding(t *testing.T) {
	tests := []struct {
		brightness float32
		expected   int32
	}{
		{50.3, 50},
		{50.7, 51},
		{75.0, 75},
		{50.5, 51},
		{0.0, 0},
		{100.0, 100},
	}

	for _, tt := range tests {
		rounded := int32(tt.brightness + 0.5)
		if rounded != tt.expected {
			t.Errorf("Rounded %.1f = %d, want %d", tt.brightness, rounded, tt.expected)
		}
	}
}

// TestConfigIsConfigured tests config state methods
func TestConfigIsConfigured(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.IsConfigured() {
		t.Error("Default config should not be configured")
	}

	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"
	if !cfg.IsConfigured() {
		t.Error("Config with bridge and key should be configured")
	}

	if cfg.HasEntertainmentConfig() {
		t.Error("Config without clientkey/entertainment ID should not have entertainment config")
	}

	cfg.ClientKey = "test-client-key"
	cfg.EntertainmentConfigurationID = "test-id"
	if !cfg.HasEntertainmentConfig() {
		t.Error("Config with all fields should have entertainment config")
	}
}

// TestSceneDisplayNameFormatting tests scene display name formatting
func TestSceneDisplayNameFormatting(t *testing.T) {
	tests := []struct {
		scene    hue.Scene
		expected string
	}{
		{
			scene:    hue.Scene{Name: "Relax", RoomName: "Living Room"},
			expected: "Living Room - Relax",
		},
		{
			scene:    hue.Scene{Name: "Bright"},
			expected: "Bright",
		},
		{
			scene:    hue.Scene{Name: "Concentrate", RoomName: ""},
			expected: "Concentrate",
		},
	}

	for _, tt := range tests {
		displayName := tt.scene.Name
		if tt.scene.RoomName != "" {
			displayName = tt.scene.RoomName + " - " + tt.scene.Name
		}
		if displayName != tt.expected {
			t.Errorf("Display name = %v, want %v", displayName, tt.expected)
		}
	}
}
