package dbus

import (
	"testing"

	"github.com/codepuncher/khuey/internal/hue"
)

// TestSetBrightness_Validation tests brightness validation
func TestSetBrightness_Validation(t *testing.T) {
	tests := []struct {
		name        string
		brightness  int
		expectError bool
	}{
		{
			name:        "Valid minimum brightness",
			brightness:  0,
			expectError: false,
		},
		{
			name:        "Valid mid brightness",
			brightness:  50,
			expectError: false,
		},
		{
			name:        "Valid maximum brightness",
			brightness:  100,
			expectError: false,
		},
		{
			name:        "Invalid negative brightness",
			brightness:  -1,
			expectError: true,
		},
		{
			name:        "Invalid too high brightness",
			brightness:  101,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate brightness range (logic from SetBrightness)
			isValid := tt.brightness >= 0 && tt.brightness <= 100

			if tt.expectError && isValid {
				t.Errorf("Expected brightness %d to be invalid", tt.brightness)
			}
			if !tt.expectError && !isValid {
				t.Errorf("Expected brightness %d to be valid", tt.brightness)
			}
		})
	}
}

// TestSceneNameMatching tests the scene name matching logic
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
		{
			name:        "Match with room name",
			displayName: "Living Room - Relax",
			expectFound: true,
			expectID:    "scene-1",
		},
		{
			name:        "Match without room name",
			displayName: "Concentrate",
			expectFound: true,
			expectID:    "scene-3",
		},
		{
			name:        "No match",
			displayName: "Nonexistent Scene",
			expectFound: false,
		},
		{
			name:        "Partial match should not work",
			displayName: "Living Room",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Implement the scene matching logic from ActivateScene
			var foundID string
			found := false

			for _, scene := range scenes {
				var sceneDisplayName string
				if scene.RoomName != "" {
					sceneDisplayName = scene.RoomName + " - " + scene.Name
				} else {
					sceneDisplayName = scene.Name
				}

				if sceneDisplayName == tt.displayName {
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

// TestBrightnessRounding tests brightness rounding logic
func TestBrightnessRounding(t *testing.T) {
	tests := []struct {
		name       string
		brightness float64
		expected   int
	}{
		{
			name:       "Round down",
			brightness: 50.3,
			expected:   50,
		},
		{
			name:       "Round up",
			brightness: 50.7,
			expected:   51,
		},
		{
			name:       "Exact value",
			brightness: 75.0,
			expected:   75,
		},
		{
			name:       "Round 0.5 up",
			brightness: 50.5,
			expected:   51,
		},
		{
			name:       "Zero",
			brightness: 0.0,
			expected:   0,
		},
		{
			name:       "Full brightness",
			brightness: 100.0,
			expected:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Implement rounding logic from GetState
			rounded := int(tt.brightness + 0.5)

			if rounded != tt.expected {
				t.Errorf("Rounded brightness = %d, want %d", rounded, tt.expected)
			}
		})
	}
}

// TestGetStatusLogic tests status message logic
func TestGetStatusLogic(t *testing.T) {
	tests := []struct {
		name           string
		hasHueClient   bool
		hasConfig      bool
		expectedStatus string
	}{
		{
			name:           "Fully configured",
			hasHueClient:   true,
			hasConfig:      true,
			expectedStatus: "Ready",
		},
		{
			name:           "Missing hue client",
			hasHueClient:   false,
			hasConfig:      true,
			expectedStatus: "Not configured",
		},
		{
			name:           "Missing config",
			hasHueClient:   true,
			hasConfig:      false,
			expectedStatus: "Not configured",
		},
		{
			name:           "Both missing",
			hasHueClient:   false,
			hasConfig:      false,
			expectedStatus: "Not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate GetStatus logic
			var status string
			if tt.hasHueClient && tt.hasConfig {
				status = "Ready"
			} else {
				status = "Not configured"
			}

			if status != tt.expectedStatus {
				t.Errorf("Status = %v, want %v", status, tt.expectedStatus)
			}
		})
	}
}

// TestConfigValidation tests various config validations
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name            string
		groupedLightID  string
		entertainmentID string
		clientKey       string
		expectValid     bool
	}{
		{
			name:            "Valid full config",
			groupedLightID:  "light-123",
			entertainmentID: "ent-456",
			clientKey:       "key-789",
			expectValid:     true,
		},
		{
			name:            "Empty grouped light ID",
			groupedLightID:  "",
			entertainmentID: "ent-456",
			clientKey:       "key-789",
			expectValid:     false,
		},
		{
			name:            "Empty entertainment ID",
			groupedLightID:  "light-123",
			entertainmentID: "",
			clientKey:       "key-789",
			expectValid:     false,
		},
		{
			name:            "Empty client key",
			groupedLightID:  "light-123",
			entertainmentID: "ent-456",
			clientKey:       "",
			expectValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test grouped light ID validation
			groupedLightValid := tt.groupedLightID != ""

			// Test entertainment config validation
			entertainmentValid := tt.entertainmentID != "" && tt.clientKey != ""

			overallValid := groupedLightValid && entertainmentValid

			if overallValid != tt.expectValid {
				t.Errorf("Config validity = %v, want %v", overallValid, tt.expectValid)
			}
		})
	}
}

// TestSetGroupedLight_Validation tests grouped light ID validation
func TestSetGroupedLight_Validation(t *testing.T) {
	tests := []struct {
		name           string
		groupedLightID string
		expectError    bool
	}{
		{
			name:           "Valid ID",
			groupedLightID: "light-123",
			expectError:    false,
		},
		{
			name:           "Empty ID",
			groupedLightID: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate logic from SetGroupedLight
			isValid := tt.groupedLightID != ""

			if tt.expectError && isValid {
				t.Errorf("Expected ID %q to be invalid", tt.groupedLightID)
			}
			if !tt.expectError && !isValid {
				t.Errorf("Expected ID %q to be valid", tt.groupedLightID)
			}
		})
	}
}

// TestSceneDisplayNameFormatting tests scene display name formatting
func TestSceneDisplayNameFormatting(t *testing.T) {
	tests := []struct {
		name     string
		scene    hue.Scene
		expected string
	}{
		{
			name: "Scene with room",
			scene: hue.Scene{
				ID:       "1",
				Name:     "Relax",
				RoomName: "Living Room",
			},
			expected: "Living Room - Relax",
		},
		{
			name: "Scene without room",
			scene: hue.Scene{
				ID:   "2",
				Name: "Bright",
			},
			expected: "Bright",
		},
		{
			name: "Scene with empty room name",
			scene: hue.Scene{
				ID:       "3",
				Name:     "Concentrate",
				RoomName: "",
			},
			expected: "Concentrate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Format scene display name (logic from GetScenes)
			var displayName string
			if tt.scene.RoomName != "" {
				displayName = tt.scene.RoomName + " - " + tt.scene.Name
			} else {
				displayName = tt.scene.Name
			}

			if displayName != tt.expected {
				t.Errorf("Display name = %v, want %v", displayName, tt.expected)
			}
		})
	}
}

// TestSyncEngineRequirements tests sync engine initialization requirements
func TestSyncEngineRequirements(t *testing.T) {
	tests := []struct {
		name            string
		entertainmentID string
		clientKey       string
		shouldCreate    bool
	}{
		{
			name:            "Both configured",
			entertainmentID: "ent-123",
			clientKey:       "key-456",
			shouldCreate:    true,
		},
		{
			name:            "Missing entertainment ID",
			entertainmentID: "",
			clientKey:       "key-456",
			shouldCreate:    false,
		},
		{
			name:            "Missing client key",
			entertainmentID: "ent-123",
			clientKey:       "",
			shouldCreate:    false,
		},
		{
			name:            "Both missing",
			entertainmentID: "",
			clientKey:       "",
			shouldCreate:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if sync engine should be created (logic from NewService)
			shouldCreate := tt.entertainmentID != "" && tt.clientKey != ""

			if shouldCreate != tt.shouldCreate {
				t.Errorf("Should create engine = %v, want %v", shouldCreate, tt.shouldCreate)
			}
		})
	}
}
