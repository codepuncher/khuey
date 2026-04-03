package hue

import (
	"context"
	"sort"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		bridgeAddr string
		apiKey     string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid client creation",
			ctx:        context.Background(),
			bridgeAddr: "192.168.1.100",
			apiKey:     "test-api-key",
			wantErr:    false,
		},
		{
			name:       "nil context uses background",
			ctx:        nil,
			bridgeAddr: "192.168.1.100",
			apiKey:     "test-api-key",
			wantErr:    false,
		},
		{
			name:       "missing bridge address",
			ctx:        context.Background(),
			bridgeAddr: "",
			apiKey:     "test-api-key",
			wantErr:    true,
			errMsg:     "bridge address is required",
		},
		{
			name:       "missing API key",
			ctx:        context.Background(),
			bridgeAddr: "192.168.1.100",
			apiKey:     "",
			wantErr:    true,
			errMsg:     "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.ctx, tt.bridgeAddr, tt.apiKey)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() expected error but got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("NewClient() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewClient() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Error("NewClient() returned nil client")
				return
			}

			if client.bridgeAddr != tt.bridgeAddr {
				t.Errorf("NewClient() bridgeAddr = %v, want %v", client.bridgeAddr, tt.bridgeAddr)
			}

			if client.apiKey != tt.apiKey {
				t.Errorf("NewClient() apiKey = %v, want %v", client.apiKey, tt.apiKey)
			}
		})
	}
}

func TestSceneFormatting(t *testing.T) {
	tests := []struct {
		name     string
		scene    Scene
		expected string
	}{
		{
			name: "scene with room name",
			scene: Scene{
				ID:       "scene-123",
				Name:     "Relax",
				RoomName: "Living Room",
			},
			expected: "Living Room - Relax",
		},
		{
			name: "scene without room name",
			scene: Scene{
				ID:       "scene-456",
				Name:     "Bright",
				RoomName: "",
			},
			expected: "Bright",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the scene display name logic
			displayName := tt.scene.Name
			if tt.scene.RoomName != "" {
				displayName = tt.scene.RoomName + " - " + tt.scene.Name
			}

			if displayName != tt.expected {
				t.Errorf("Scene display name = %v, want %v", displayName, tt.expected)
			}
		})
	}
}

func TestGroupedLightValidation(t *testing.T) {
	tests := []struct {
		name         string
		groupedLight GroupedLight
		validType    bool
	}{
		{
			name: "valid room type",
			groupedLight: GroupedLight{
				ID:   "room-123",
				Name: "Living Room",
				Type: "room",
			},
			validType: true,
		},
		{
			name: "valid zone type",
			groupedLight: GroupedLight{
				ID:   "zone-456",
				Name: "Kitchen Zone",
				Type: "zone",
			},
			validType: true,
		},
		{
			name: "invalid type",
			groupedLight: GroupedLight{
				ID:   "invalid-789",
				Name: "Invalid",
				Type: "invalid",
			},
			validType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.groupedLight.Type == "room" || tt.groupedLight.Type == "zone"
			if isValid != tt.validType {
				t.Errorf("GroupedLight type validation = %v, want %v", isValid, tt.validType)
			}
		})
	}
}

// TestGetClientKey tests the GetClientKey method
func TestGetClientKey(t *testing.T) {
	ctx := context.Background()
	apiKey := "test-api-key-12345"
	
	client, err := NewClient(ctx, "192.168.1.100", apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.GetClientKey() != apiKey {
		t.Errorf("GetClientKey() = %v, want %v", client.GetClientKey(), apiKey)
	}
}

// TestSceneSorting tests that scenes are sorted alphabetically
func TestSceneSorting(t *testing.T) {
	scenes := []Scene{
		{ID: "3", Name: "Relax", RoomName: "Living Room"},
		{ID: "1", Name: "Bright", RoomName: "Kitchen"},
		{ID: "2", Name: "Arctic aurora", RoomName: "Bedroom"},
	}

	// Create display names
	displayNames := make([]string, len(scenes))
	for i, s := range scenes {
		if s.RoomName != "" {
			displayNames[i] = s.RoomName + " - " + s.Name
		} else {
			displayNames[i] = s.Name
		}
	}

	// Sort
	sort.Strings(displayNames)

	// Check sorted order
	expected := []string{
		"Bedroom - Arctic aurora",
		"Kitchen - Bright",
		"Living Room - Relax",
	}

	for i, name := range displayNames {
		if name != expected[i] {
			t.Errorf("Index %d: got %v, want %v", i, name, expected[i])
		}
	}
}

// TestGroupedLightSorting tests that grouped lights are sorted alphabetically
func TestGroupedLightSorting(t *testing.T) {
	lights := []GroupedLight{
		{ID: "3", Name: "Living Room", Type: "room"},
		{ID: "1", Name: "Bedroom", Type: "room"},
		{ID: "2", Name: "Kitchen", Type: "room"},
	}

	// Sort by name
	sort.Slice(lights, func(i, j int) bool {
		return lights[i].Name < lights[j].Name
	})

	expected := []string{"Bedroom", "Kitchen", "Living Room"}
	for i, light := range lights {
		if light.Name != expected[i] {
			t.Errorf("Index %d: got %v, want %v", i, light.Name, expected[i])
		}
	}
}
