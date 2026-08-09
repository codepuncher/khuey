package dbus

import (
	"strings"
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

	// GetBridgeSettings is owner-guarded (touches bridge-derived data); inject
	// an allowing resolver so this test exercises the config-reading logic.
	allowingCallerUID := func(dbus.Sender) (uint32, error) {
		return 1000, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg, ownerUID: 1000, callerUID: allowingCallerUID}
			settings, err := s.GetBridgeSettings(dbus.Sender("owner"))
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

	// GetSelectedRoom is owner-guarded (returns a bridge grouped-light UUID);
	// inject an allowing resolver so this test exercises the config-reading
	// logic.
	allowingCallerUID := func(dbus.Sender) (uint32, error) {
		return 1000, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg, ownerUID: 1000, callerUID: allowingCallerUID}
			result, err := s.GetSelectedRoom(dbus.Sender("owner"))
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

	// GetStartupScene is owner-guarded (returns a bridge-derived scene display
	// name); inject an allowing resolver so this test exercises the
	// config-reading logic.
	allowingCallerUID := func(dbus.Sender) (uint32, error) {
		return 1000, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{config: tt.cfg, ownerUID: 1000, callerUID: allowingCallerUID}
			result, err := s.GetStartupScene(dbus.Sender("owner"))
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

// TestSetStartupSceneInvalidInput pins that SetStartupScene, with checkAccess
// satisfied by an allowing resolver, still rejects invalid UTF-8 input before
// touching config - i.e. that the invalid-input path is reachable and
// distinguishable from an access-denied failure.
func TestSetStartupSceneInvalidInput(t *testing.T) {
	const ownerUID = 1000
	allowingCallerUID := func(dbus.Sender) (uint32, error) {
		return ownerUID, nil
	}
	s := &Service{config: config.DefaultConfig(), ownerUID: ownerUID, callerUID: allowingCallerUID}
	ok, dbusErr := s.SetStartupScene("\xff\xfe invalid utf8", dbus.Sender("owner"))
	if ok || dbusErr == nil {
		t.Error("SetStartupScene() with invalid UTF-8 should fail validation")
	}
	if dbusErr != nil && strings.Contains(dbusErr.Error(), "access denied") {
		t.Errorf("SetStartupScene() failed on access check, want invalid-input failure: %v", dbusErr)
	}
	// Assert the specific validation message: Save() fails anyway on the
	// unconfigured default config, so a weaker assertion passes even with the
	// ValidateDBusString call removed.
	if dbusErr != nil && !strings.Contains(dbusErr.Error(), "invalid UTF-8") {
		t.Errorf("SetStartupScene() error = %v, want invalid UTF-8 validation failure", dbusErr)
	}
}

// TestSetSelectedRoomInvalidInput pins that SetSelectedRoom, with checkAccess
// satisfied by an allowing resolver, still rejects invalid UTF-8 input before
// touching config - i.e. that the invalid-input path is reachable and
// distinguishable from an access-denied failure.
func TestSetSelectedRoomInvalidInput(t *testing.T) {
	const ownerUID = 1000
	allowingCallerUID := func(dbus.Sender) (uint32, error) {
		return ownerUID, nil
	}
	s := &Service{config: config.DefaultConfig(), ownerUID: ownerUID, callerUID: allowingCallerUID}
	ok, dbusErr := s.SetSelectedRoom("\xff\xfe invalid utf8", dbus.Sender("owner"))
	if ok || dbusErr == nil {
		t.Error("SetSelectedRoom() with invalid UTF-8 should fail validation")
	}
	if dbusErr != nil && strings.Contains(dbusErr.Error(), "access denied") {
		t.Errorf("SetSelectedRoom() failed on access check, want invalid-input failure: %v", dbusErr)
	}
	// Config.Save() also fails on DefaultConfig() (no bridge address set), so
	// pin the specific validation error text - not just "any non-access error" -
	// to distinguish a real validation rejection from that unrelated save failure.
	if dbusErr != nil && !strings.Contains(dbusErr.Error(), "invalid UTF-8") {
		t.Errorf("SetSelectedRoom() error = %v, want invalid UTF-8 validation failure", dbusErr)
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

// sentinelConfig returns a config seeded with known values for every field
// that a mutating DBus method might write, so a denied call's "config
// unchanged" assertion has something specific to check against.
func sentinelConfig() *config.Config {
	return &config.Config{
		GroupedLightID: "sentinel-light-id",
		StartupScene:   "sentinel-scene",
		Sync: config.SyncConfig{
			FPS:            15,
			SubsampleWidth: 32,
			Monitor:        "sentinel-monitor",
		},
		GamingMode: config.GamingModeConfig{Enabled: false},
		UI: config.UIConfig{
			Icons: config.IconConfig{
				Gaming:  "sentinel-gaming",
				Syncing: "sentinel-syncing",
				Idle:    "sentinel-idle",
			},
		},
	}
}

// guardedMethodCase describes one checkAccess-gated DBus method, shared by
// the deny-direction (TestMutatorsRequireOwnerAccess) and allow-direction
// (TestGuardedMethodsAllowOwnerCaller) tests below.
type guardedMethodCase struct {
	name string
	call func(s *Service, sender dbus.Sender) *dbus.Error
	// checkUnchanged asserts the field(s) this method mutates on success
	// were left alone by a denied call. Nil for methods that don't touch
	// config (e.g. pure bridge reads).
	checkUnchanged func(t *testing.T, cfg *config.Config)
}

// guardedMethodCases enumerates every DBus method gated by checkAccess:
// the original mutators plus the seven bridge-data readers (GetScenes,
// GetGroupedLights, GetState, GetConnectionStatus, GetBridgeSettings,
// GetSelectedRoom, GetStartupScene) guarded under the "touches the bridge or
// bridge-derived data requires owner" rule.
var guardedMethodCases = []guardedMethodCase{
	{
		name: "SetPower",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetPower(true, sender)
			return err
		},
	},
	{
		name: "SetBrightness",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetBrightness(50, sender)
			return err
		},
	},
	{
		name: "ActivateScene",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.ActivateScene("Living Room - Relax", sender)
			return err
		},
	},
	{
		name: "SetGroupedLight",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetGroupedLight("new-light-id", sender)
			return err
		},
		checkUnchanged: func(t *testing.T, cfg *config.Config) {
			if cfg.GroupedLightID != "sentinel-light-id" {
				t.Errorf("GroupedLightID = %q, want unchanged %q", cfg.GroupedLightID, "sentinel-light-id")
			}
		},
	},
	{
		name: "StartSync",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.StartSync(sender)
			return err
		},
	},
	{
		name: "StopSync",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.StopSync(sender)
			return err
		},
	},
	{
		name: "SetSyncSettings",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetSyncSettings(30, 64, "eDP-1", sender)
			return err
		},
		checkUnchanged: func(t *testing.T, cfg *config.Config) {
			if cfg.Sync.FPS != 15 || cfg.Sync.SubsampleWidth != 32 || cfg.Sync.Monitor != "sentinel-monitor" {
				t.Errorf("Sync settings mutated: %+v", cfg.Sync)
			}
		},
	},
	{
		name: "SetSelectedRoom",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetSelectedRoom("new-room-id", sender)
			return err
		},
		checkUnchanged: func(t *testing.T, cfg *config.Config) {
			if cfg.GroupedLightID != "sentinel-light-id" {
				t.Errorf("GroupedLightID = %q, want unchanged %q", cfg.GroupedLightID, "sentinel-light-id")
			}
		},
	},
	{
		name: "SetStartupScene",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetStartupScene("Kitchen - Bright", sender)
			return err
		},
		checkUnchanged: func(t *testing.T, cfg *config.Config) {
			if cfg.StartupScene != "sentinel-scene" {
				t.Errorf("StartupScene = %q, want unchanged %q", cfg.StartupScene, "sentinel-scene")
			}
		},
	},
	{
		name: "SetGamingMode",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetGamingMode(sender, true)
			return err
		},
		checkUnchanged: func(t *testing.T, cfg *config.Config) {
			if cfg.GamingMode.Enabled {
				t.Errorf("GamingMode.Enabled = %v, want unchanged false", cfg.GamingMode.Enabled)
			}
		},
	},
	{
		name: "SetTrayIcons",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.SetTrayIcons("new-gaming", "new-syncing", "new-idle", sender)
			return err
		},
		checkUnchanged: func(t *testing.T, cfg *config.Config) {
			if cfg.UI.Icons.Gaming != "sentinel-gaming" || cfg.UI.Icons.Syncing != "sentinel-syncing" || cfg.UI.Icons.Idle != "sentinel-idle" {
				t.Errorf("UI.Icons mutated: %+v", cfg.UI.Icons)
			}
		},
	},
	{
		name: "RetryConnection",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.RetryConnection(sender)
			return err
		},
	},
	{
		name: "TestBridgeConnection",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.TestBridgeConnection(sender)
			return err
		},
	},
	{
		name: "GetScenes",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.GetScenes(sender)
			return err
		},
	},
	{
		name: "GetGroupedLights",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.GetGroupedLights(sender)
			return err
		},
	},
	{
		name: "GetState",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, _, _, err := s.GetState(sender)
			return err
		},
	},
	{
		name: "GetConnectionStatus",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.GetConnectionStatus(sender)
			return err
		},
	},
	{
		name: "GetBridgeSettings",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.GetBridgeSettings(sender)
			return err
		},
	},
	{
		name: "GetSelectedRoom",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.GetSelectedRoom(sender)
			return err
		},
	},
	{
		name: "GetStartupScene",
		call: func(s *Service, sender dbus.Sender) *dbus.Error {
			_, err := s.GetStartupScene(sender)
			return err
		},
	},
}

// TestMutatorsRequireOwnerAccess enumerates every checkAccess-gated DBus
// method and verifies each one rejects a caller whose UID doesn't match
// ownerUID. It is a regression test for the SetTrayIcons access-control gap
// (F4): a bare "err != nil" check would pass even without a checkAccess
// call, since config.Save() can already fail on its own (e.g. "bridge
// address is required"), so the assertions here pin both the exact "access
// denied" error and that config was left untouched.
//
// Directly-constructed Services (as used throughout this file) have a nil
// callerUID resolver, which would make checkAccess panic via getCallerUID's
// nil conn dereference; a fake resolver is injected here instead.
//
// This test alone only pins the deny direction: a checkAccess sabotaged to
// reject every caller (including the legitimate owner) would still pass it.
// TestGuardedMethodsAllowOwnerCaller below pins the allow direction.
func TestMutatorsRequireOwnerAccess(t *testing.T) {
	// config.Save() writes to the real ~/.openhue/config.yaml; point it at a
	// throwaway HOME so a future sentinelConfig() gaining Bridge/Key would
	// fail this test cleanly instead of clobbering the developer's config.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	const ownerUID = 1000
	deniedCallerUID := func(dbus.Sender) (uint32, error) {
		return ownerUID + 1, nil
	}

	for _, tt := range guardedMethodCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := sentinelConfig()
			s := &Service{
				config:    cfg,
				ownerUID:  ownerUID,
				callerUID: deniedCallerUID,
			}

			dbusErr := tt.call(s, dbus.Sender("attacker"))
			if dbusErr == nil {
				t.Fatalf("%s() with mismatched UID returned no error, want access denied", tt.name)
			}
			if !strings.Contains(dbusErr.Error(), "access denied") {
				t.Errorf("%s() error = %q, want it to contain %q", tt.name, dbusErr.Error(), "access denied")
			}

			if tt.checkUnchanged != nil {
				tt.checkUnchanged(t, cfg)
			}
		})
	}
}

// TestGuardedMethodsAllowOwnerCaller is the allow-direction counterpart to
// TestMutatorsRequireOwnerAccess. Verified empirically: an unconditional
// `return fmt.Errorf(...)` inserted at the top of checkAccess (denying every
// caller, including the owner) left the deny-direction test green, since it
// only ever exercises mismatched UIDs. This test injects a resolver
// returning the matching ownerUID and checks that checkAccess did not reject
// the call. Methods still fail downstream for unrelated reasons in this
// unconfigured test Service (no hueClient, no bridge address for
// config.Save()), so this deliberately does not assert success - only that
// the failure, if any, isn't "access denied".
func TestGuardedMethodsAllowOwnerCaller(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	const ownerUID = 1000
	allowingCallerUID := func(dbus.Sender) (uint32, error) {
		return ownerUID, nil
	}

	for _, tt := range guardedMethodCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := sentinelConfig()
			s := &Service{
				config:    cfg,
				ownerUID:  ownerUID,
				callerUID: allowingCallerUID,
			}

			dbusErr := tt.call(s, dbus.Sender("owner"))
			if dbusErr != nil && strings.Contains(dbusErr.Error(), "access denied") {
				t.Errorf("%s() with matching UID was denied: %v", tt.name, dbusErr.Error())
			}
		})
	}
}

// TestCheckAccessFailsClosedWithoutResolver verifies a Service with no
// callerUID resolver (e.g. a directly-constructed &Service{} as used
// throughout this file's other tests) denies access rather than allowing it
// or panicking via a nil-conn dereference in the real getCallerUID.
func TestCheckAccessFailsClosedWithoutResolver(t *testing.T) {
	s := &Service{config: config.DefaultConfig(), ownerUID: 1000}

	err := s.checkAccess(dbus.Sender("anyone"))
	if err == nil {
		t.Fatal("checkAccess() with no callerUID resolver returned nil, want access denied")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("checkAccess() error = %q, want it to contain %q", err.Error(), "access denied")
	}
}
