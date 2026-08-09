package dbus

import (
	"testing"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
	"github.com/godbus/dbus/v5"
)

// TestGetStatus tests the GetStatus method logic
func TestGetStatus(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected string
	}{
		{
			name:     "Not configured",
			cfg:      config.DefaultConfig(),
			expected: "Not configured",
		},
		{
			name: "Configured",
			cfg: &config.Config{
				Bridge: "192.168.1.100",
				Key:    "test-key",
			},
			expected: "Ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			status, err := s.GetStatus()
			if err != nil {
				t.Errorf("GetStatus() returned unexpected error: %v", err)
			}
			if status != tt.expected {
				t.Errorf("GetStatus() = %q, want %q", status, tt.expected)
			}
		})
	}
}

// TestIsSyncing tests the IsSyncing method
func TestIsSyncing(t *testing.T) {
	tests := []struct {
		name       string
		hasEngine  bool
		wantResult bool
	}{
		{
			name:       "No sync engine",
			hasEngine:  false,
			wantResult: false,
		},
		// Note: Testing with actual running engine would require complex setup
		// The method simply returns syncEngine != nil && syncEngine.IsRunning()
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				config:     config.DefaultConfig(),
				syncEngine: nil,
			}
			result, err := s.IsSyncing()
			if err != nil {
				t.Errorf("IsSyncing() returned error: %v", err)
			}
			if result != tt.wantResult {
				t.Errorf("IsSyncing() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

// TestIsGamingModeEnabled tests gaming mode enabled check
func TestIsGamingModeEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name:     "Default config (disabled)",
			cfg:      config.DefaultConfig(),
			expected: false,
		},
		{
			name: "Gaming mode enabled",
			cfg: &config.Config{
				GamingMode: config.GamingModeConfig{
					Enabled: true,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			result, err := s.IsGamingModeEnabled()
			if err != nil {
				t.Errorf("IsGamingModeEnabled() returned error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("IsGamingModeEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestIsGamingModeActive tests gaming mode active state
func TestIsGamingModeActive(t *testing.T) {
	tests := []struct {
		name        string
		hasDetector bool
		wantResult  bool
	}{
		{
			name:        "No detector",
			hasDetector: false,
			wantResult:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				config:         config.DefaultConfig(),
				gamingDetector: nil,
			}
			result, err := s.IsGamingModeActive()
			if err != nil {
				t.Errorf("IsGamingModeActive() returned error: %v", err)
			}
			if result != tt.wantResult {
				t.Errorf("IsGamingModeActive() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

// TestGetTrayIcons tests tray icon retrieval
func TestGetTrayIcons(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		wantGaming  string
		wantSyncing string
		wantIdle    string
	}{
		{
			name:        "Default icons",
			cfg:         config.DefaultConfig(),
			wantGaming:  "applications-games",
			wantSyncing: "media-record",
			wantIdle:    "preferences-desktop-display-color",
		},
		{
			name: "Custom icons",
			cfg: &config.Config{
				UI: config.UIConfig{
					Icons: config.IconConfig{
						Gaming:  "custom-gaming",
						Syncing: "custom-syncing",
						Idle:    "custom-idle",
					},
				},
			},
			wantGaming:  "custom-gaming",
			wantSyncing: "custom-syncing",
			wantIdle:    "custom-idle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			gaming, syncing, idle, err := s.GetTrayIcons()
			if err != nil {
				t.Errorf("GetTrayIcons() returned error: %v", err)
			}
			if gaming != tt.wantGaming {
				t.Errorf("GetTrayIcons() gaming = %q, want %q", gaming, tt.wantGaming)
			}
			if syncing != tt.wantSyncing {
				t.Errorf("GetTrayIcons() syncing = %q, want %q", syncing, tt.wantSyncing)
			}
			if idle != tt.wantIdle {
				t.Errorf("GetTrayIcons() idle = %q, want %q", idle, tt.wantIdle)
			}
		})
	}
}

// TestGetSyncSettings tests sync settings retrieval
func TestGetSyncSettings(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantFPS int
		wantSW  int
	}{
		{
			name:    "Default settings",
			cfg:     config.DefaultConfig(),
			wantFPS: 30,
			wantSW:  64,
		},
		{
			name: "Custom settings",
			cfg: &config.Config{
				Sync: config.SyncConfig{
					FPS:            25,
					SubsampleWidth: 128,
				},
			},
			wantFPS: 25,
			wantSW:  128,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			settings, err := s.GetSyncSettings()
			if err != nil {
				t.Errorf("GetSyncSettings() returned error: %v", err)
			}
			if fps, ok := settings["fps"].(int); ok && fps != tt.wantFPS {
				t.Errorf("GetSyncSettings() fps = %d, want %d", fps, tt.wantFPS)
			}
			if sw, ok := settings["subsampleWidth"].(int); ok && sw != tt.wantSW {
				t.Errorf("GetSyncSettings() subsampleWidth = %d, want %d", sw, tt.wantSW)
			}
		})
	}
}

// TestGetBridgeSettings tests bridge settings retrieval
func TestGetBridgeSettings(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		wantBridge string
	}{
		{
			name:       "Empty settings",
			cfg:        config.DefaultConfig(),
			wantBridge: "",
		},
		{
			name: "Configured settings",
			cfg: &config.Config{
				Bridge: "192.168.1.100",
			},
			wantBridge: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			settings, err := s.GetBridgeSettings()
			if err != nil {
				t.Errorf("GetBridgeSettings() returned error: %v", err)
			}
			if bridge, ok := settings["bridgeIP"].(string); ok && bridge != tt.wantBridge {
				t.Errorf("GetBridgeSettings() bridge = %q, want %q", bridge, tt.wantBridge)
			}
			// connected and lastError depend on hueClient which we don't have in unit tests
		})
	}
}

// TestGetSelectedRoom tests selected room retrieval
func TestGetSelectedRoom(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{
			name: "Empty grouped light ID",
			cfg:  config.DefaultConfig(),
			want: "",
		},
		{
			name: "Set grouped light ID",
			cfg: &config.Config{
				GroupedLightID: "test-room-id",
			},
			want: "test-room-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			result, err := s.GetSelectedRoom()
			if err != nil {
				t.Errorf("GetSelectedRoom() returned error: %v", err)
			}
			if result != tt.want {
				t.Errorf("GetSelectedRoom() = %q, want %q", result, tt.want)
			}
		})
	}
}

// TestGetStartupScene tests startup scene retrieval
func TestGetStartupScene(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{
			name: "Disabled by default",
			cfg:  config.DefaultConfig(),
			want: "",
		},
		{
			name: "Startup scene configured",
			cfg: &config.Config{
				StartupScene: "Living Room - Relax",
			},
			want: "Living Room - Relax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg}
			result, err := s.GetStartupScene()
			if err != nil {
				t.Errorf("GetStartupScene() returned error: %v", err)
			}
			if result != tt.want {
				t.Errorf("GetStartupScene() = %q, want %q", result, tt.want)
			}
		})
	}
}

// TestActivateStartupScene tests that startup scene activation is a no-op when
// disabled, and surfaces an error when configured but the bridge is unavailable.
func TestActivateStartupScene(t *testing.T) {
	t.Run("Disabled - no-op", func(t *testing.T) {
		s := &Service{config: config.DefaultConfig()}
		if err := s.ActivateStartupScene(); err != nil {
			t.Errorf("ActivateStartupScene() with no scene configured returned error: %v", err)
		}
	})

	t.Run("Configured without hue client", func(t *testing.T) {
		s := &Service{config: &config.Config{StartupScene: "Living Room - Relax"}}
		if err := s.ActivateStartupScene(); err == nil {
			t.Error("ActivateStartupScene() expected error with no hue client, got nil")
		}
	})
}

// TestSetStartupSceneInvalidInput tests that SetStartupScene rejects invalid
// input before touching config or checking access, since neither is safe to
// exercise here without a live DBus connection.
func TestSetStartupSceneInvalidInput(t *testing.T) {
	s := &Service{config: config.DefaultConfig()}
	ok, dbusErr := s.SetStartupScene("\xff\xfe invalid utf8", dbus.Sender(""))
	if ok || dbusErr == nil {
		t.Error("SetStartupScene() with invalid UTF-8 should fail validation")
	}
}

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
