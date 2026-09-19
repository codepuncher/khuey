package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Sync.FPS != 30 {
		t.Errorf("Expected default FPS of 30, got %d", cfg.Sync.FPS)
	}

	if cfg.Sync.SubsampleWidth != 64 {
		t.Errorf("Expected default subsample width of 64, got %d", cfg.Sync.SubsampleWidth)
	}

	if cfg.Sync.Enabled {
		t.Error("Expected sync to be disabled by default")
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.LogLevel)
	}

	if cfg.StartupScene != "" {
		t.Errorf("Expected startup scene to be disabled by default, got %q", cfg.StartupScene)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()

	// Test IsConfigured
	if cfg.IsConfigured() {
		t.Error("Expected config to be unconfigured without bridge/key")
	}

	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"

	if !cfg.IsConfigured() {
		t.Error("Expected config to be configured with bridge and key")
	}

	// Test HasEntertainmentConfig
	if cfg.HasEntertainmentConfig() {
		t.Error("Expected entertainment config to be incomplete")
	}

	cfg.ClientKey = "test-client-key"
	cfg.EntertainmentConfigurationID = "test-id"

	if !cfg.HasEntertainmentConfig() {
		t.Error("Expected entertainment config to be complete")
	}
}

func TestUVCoordinates(t *testing.T) {
	uv := UV{X: 0.5, Y: 0.75}

	if uv.X != 0.5 {
		t.Errorf("Expected X=0.5, got %f", uv.X)
	}

	if uv.Y != 0.75 {
		t.Errorf("Expected Y=0.75, got %f", uv.Y)
	}
}

func TestChannelConfig(t *testing.T) {
	channel := ChannelConfig{
		ID:          1,
		Active:      true,
		DeviceName:  "Hue Play 1",
		GammaFactor: 0.0,
		UVA:         UV{X: 0.0, Y: 0.0},
		UVB:         UV{X: 0.5, Y: 1.0},
	}

	if channel.ID != 1 {
		t.Errorf("Expected channel ID 1, got %d", channel.ID)
	}

	if !channel.Active {
		t.Error("Expected channel to be active")
	}

	if channel.DeviceName != "Hue Play 1" {
		t.Errorf("Expected device name 'Hue Play 1', got '%s'", channel.DeviceName)
	}
}

// Validate checks the gaming mode timings whatever gamingMode.enabled says, so
// a Config literal built without them fails on the zero PollInterval.
var validGamingMode = GamingModeConfig{
	PollInterval:  DefaultPollInterval,
	DebounceDelay: DefaultDebounceDelay,
}

// TestValidate tests the Validate() method
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 64,
				},
			},
			wantErr: false,
		},
		{
			name: "Missing bridge",
			cfg: &Config{
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 64,
				},
			},
			wantErr: true,
			errMsg:  "bridge IP not configured",
		},
		{
			name: "Missing key",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 64,
				},
			},
			wantErr: true,
			errMsg:  "API key not configured",
		},
		{
			name: "FPS too low",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            0,
					SubsampleWidth: 64,
				},
			},
			wantErr: true,
			errMsg:  "sync.fps must be between",
		},
		{
			name: "FPS too high",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            120,
					SubsampleWidth: 64,
				},
			},
			wantErr: true,
			errMsg:  "sync.fps must be between",
		},
		{
			// FPS values below capture.MinFPS (10) must be rejected here too:
			// SetSyncSettings validates against capture's bounds, so a value
			// that passes Validate but fails there would let a hand-edited
			// config load fine yet be unchangeable via the tray settings UI.
			name: "FPS below capture minimum",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            5,
					SubsampleWidth: 64,
				},
			},
			wantErr: true,
			errMsg:  "sync.fps must be between",
		},
		{
			name: "FPS at reconciled minimum",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            MinFPS,
					SubsampleWidth: 64,
				},
			},
			wantErr: false,
		},
		{
			name: "SubsampleWidth too low",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 10,
				},
			},
			wantErr: true,
			errMsg:  "sync.subsampleWidth must be between",
		},
		{
			name: "SubsampleWidth too high",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 500,
				},
			},
			wantErr: true,
			errMsg:  "sync.subsampleWidth must be between",
		},
		{
			name: "Invalid UV coordinates - uvA.X negative",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 64,
				},
				Channels: []ChannelConfig{
					{
						ID:     0,
						Active: true,
						UVA:    UV{X: -0.1, Y: 0.0},
						UVB:    UV{X: 0.5, Y: 1.0},
					},
				},
			},
			wantErr: true,
			errMsg:  "uvA.x must be 0.0-1.0",
		},
		{
			name: "Invalid UV coordinates - uvB.X > 1",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 64,
				},
				Channels: []ChannelConfig{
					{
						ID:     0,
						Active: true,
						UVA:    UV{X: 0.0, Y: 0.0},
						UVB:    UV{X: 1.5, Y: 1.0},
					},
				},
			},
			wantErr: true,
			errMsg:  "uvB.x must be 0.0-1.0",
		},
		{
			name: "Invalid UV coordinates - uvA.X >= uvB.X",
			cfg: &Config{
				Bridge:     "192.168.1.100",
				Key:        "test-key",
				GamingMode: validGamingMode,
				Sync: SyncConfig{
					FPS:            30,
					SubsampleWidth: 64,
				},
				Channels: []ChannelConfig{
					{
						ID:     0,
						Active: true,
						UVA:    UV{X: 0.5, Y: 0.0},
						UVB:    UV{X: 0.5, Y: 1.0},
					},
				},
			},
			wantErr: true,
			errMsg:  "uvA.x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestSave tests the Save() method
func TestSave(t *testing.T) {
	// Create temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Override getConfigFile for testing
	originalGetConfigFile := getConfigFile
	getConfigFile = func() string {
		return configFile
	}
	defer func() {
		getConfigFile = originalGetConfigFile
	}()

	cfg := DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-api-key"
	cfg.ClientKey = "test-client-key"
	cfg.EntertainmentConfigurationID = "test-ent-id"
	cfg.StartupScene = "Living Room - Relax"

	// Save config
	err := cfg.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Verify file permissions (should be 0600)
	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Config file has wrong permissions: got %o, want 0600", info.Mode().Perm())
	}

	// Verify file content by loading it back
	// Override getConfigFile again for Load()
	loadedCfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedCfg.Bridge != cfg.Bridge {
		t.Errorf("Loaded Bridge = %q, want %q", loadedCfg.Bridge, cfg.Bridge)
	}
	if loadedCfg.Key != cfg.Key {
		t.Errorf("Loaded Key = %q, want %q", loadedCfg.Key, cfg.Key)
	}
	if loadedCfg.ClientKey != cfg.ClientKey {
		t.Errorf("Loaded ClientKey = %q, want %q", loadedCfg.ClientKey, cfg.ClientKey)
	}
	if loadedCfg.StartupScene != cfg.StartupScene {
		t.Errorf("Loaded StartupScene = %q, want %q", loadedCfg.StartupScene, cfg.StartupScene)
	}
}

// TestSaveWithoutBridge tests Save() validation
func TestSaveWithoutBridge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Key = "test-key"

	err := cfg.Save()
	if err == nil {
		t.Error("Save() should fail without bridge address")
	}
	if err != nil && !containsString(err.Error(), "bridge address is required") {
		t.Errorf("Save() error = %v, want error containing 'bridge address is required'", err)
	}
}

// TestSaveWithoutKey tests Save() validation
func TestSaveWithoutKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Bridge = "192.168.1.100"

	err := cfg.Save()
	if err == nil {
		t.Error("Save() should fail without API key")
	}
	if err != nil && !containsString(err.Error(), "API key is required") {
		t.Errorf("Save() error = %v, want error containing 'API key is required'", err)
	}
}

// TestLoadNonExistentFile tests Load() when config file doesn't exist
func TestLoadNonExistentFile(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "nonexistent.yaml")

	// Override getConfigFile and getConfigPath for testing
	originalGetConfigFile := getConfigFile
	originalGetConfigPath := getConfigPath
	getConfigFile = func() string {
		return configFile
	}
	getConfigPath = func() string {
		return tempDir
	}
	defer func() {
		getConfigFile = originalGetConfigFile
		getConfigPath = originalGetConfigPath
	}()

	// Load should succeed with default config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Should return default values
	if cfg.Sync.FPS != 30 {
		t.Errorf("Expected default FPS 30, got %d", cfg.Sync.FPS)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %q", cfg.LogLevel)
	}
}

// TestLoadValidFile tests Load() with a valid config file
func TestLoadValidFile(t *testing.T) {
	// Create temporary directory and config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Write a test config file
	configYAML := `Bridge: 192.168.1.100
Key: test-api-key
clientkey: test-client-key
entertainmentConfigurationId: test-ent-id
sync:
  fps: 25
  subsampleWidth: 128
  enabled: false
  restoretoken: ""
gamingMode:
  enabled: true
  detectionMethods:
    - systemd-inhibit
    - power-profile
log_level: debug
`
	err := os.WriteFile(configFile, []byte(configYAML), 0600)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override getConfigFile and getConfigPath for testing
	originalGetConfigFile := getConfigFile
	originalGetConfigPath := getConfigPath
	getConfigFile = func() string {
		return configFile
	}
	getConfigPath = func() string {
		return tempDir
	}
	defer func() {
		getConfigFile = originalGetConfigFile
		getConfigPath = originalGetConfigPath
	}()

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify loaded values
	if cfg.Bridge != "192.168.1.100" {
		t.Errorf("Bridge = %q, want '192.168.1.100'", cfg.Bridge)
	}
	if cfg.Key != "test-api-key" {
		t.Errorf("Key = %q, want 'test-api-key'", cfg.Key)
	}
	if cfg.ClientKey != "test-client-key" {
		t.Errorf("ClientKey = %q, want 'test-client-key'", cfg.ClientKey)
	}
	if cfg.Sync.FPS != 25 {
		t.Errorf("Sync.FPS = %d, want 25", cfg.Sync.FPS)
	}
	if cfg.Sync.SubsampleWidth != 128 {
		t.Errorf("Sync.SubsampleWidth = %d, want 128", cfg.Sync.SubsampleWidth)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want 'debug'", cfg.LogLevel)
	}
	if !cfg.GamingMode.Enabled {
		t.Error("GamingMode.Enabled should be true")
	}
}

// TestLoadInvalidYAML tests Load() with invalid YAML
func TestLoadInvalidYAML(t *testing.T) {
	// Create temporary directory and config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Write invalid YAML
	invalidYAML := `Bridge: 192.168.1.100
Key: test-api-key
sync:
  fps: invalid-number
`
	err := os.WriteFile(configFile, []byte(invalidYAML), 0600)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override getConfigFile and getConfigPath for testing
	originalGetConfigFile := getConfigFile
	originalGetConfigPath := getConfigPath
	getConfigFile = func() string {
		return configFile
	}
	getConfigPath = func() string {
		return tempDir
	}
	defer func() {
		getConfigFile = originalGetConfigFile
		getConfigPath = originalGetConfigPath
	}()

	// Load should fail
	_, err = Load()
	if err == nil {
		t.Error("Load() should fail with invalid YAML")
	}
}

// TestLoadInvalidConfig tests Load() with config that fails validation
func TestLoadInvalidConfig(t *testing.T) {
	// Create temporary directory and config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Write config with invalid FPS (0 is below minimum of 1)
	invalidConfig := `Bridge: 192.168.1.100
Key: test-api-key
sync:
  fps: 0
  subsampleWidth: 64
`
	err := os.WriteFile(configFile, []byte(invalidConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Override getConfigFile and getConfigPath for testing
	originalGetConfigFile := getConfigFile
	originalGetConfigPath := getConfigPath
	getConfigFile = func() string {
		return configFile
	}
	getConfigPath = func() string {
		return tempDir
	}
	defer func() {
		getConfigFile = originalGetConfigFile
		getConfigPath = originalGetConfigPath
	}()

	// Load should fail validation
	_, err = Load()
	if err == nil {
		t.Error("Load() should fail with invalid config (FPS=0)")
	}
	if err != nil && !containsString(err.Error(), "validation failed") {
		t.Errorf("Load() error = %v, want error containing 'validation failed'", err)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)+1 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// loadConfigYAML writes body to a temp config file and loads it through Load,
// returning the config file path so callers can assert on what was written.
func loadConfigYAML(t *testing.T, body string) (*Config, string, error) {
	t.Helper()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(body), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	originalGetConfigFile := getConfigFile
	originalGetConfigPath := getConfigPath
	getConfigFile = func() string { return configFile }
	getConfigPath = func() string { return tempDir }
	defer func() {
		getConfigFile = originalGetConfigFile
		getConfigPath = originalGetConfigPath
	}()

	cfg, err := Load()
	return cfg, configFile, err
}

func TestLoadClampsLegacyFPS(t *testing.T) {
	tests := []struct {
		name    string
		fps     int
		want    int
		wantErr bool
	}{
		{"legacy value below the new minimum is clamped", 5, MinFPS, false},
		{"the old floor is still loadable", legacyMinFPS, MinFPS, false},
		{"a value at the new minimum is untouched", MinFPS, MinFPS, false},
		{"a normal value is untouched", 30, 30, false},
		{"zero was never valid and stays an error", 0, 0, true},
		{"a negative value stays an error", -1, 0, true},
		{"above the maximum stays an error", MaxFPS + 1, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf("Bridge: 192.168.1.100\nKey: test-api-key\nsync:\n  fps: %d\n  subsampleWidth: 128\n", tt.fps)
			cfg, _, err := loadConfigYAML(t, body)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() with fps %d: expected an error, got none", tt.fps)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() with fps %d: unexpected error: %v", tt.fps, err)
			}
			if cfg.Sync.FPS != tt.want {
				t.Errorf("Sync.FPS = %d, want %d", cfg.Sync.FPS, tt.want)
			}
		})
	}
}

func TestLoadDoesNotRewriteConfigWhenClamping(t *testing.T) {
	body := "Bridge: 192.168.1.100\nKey: test-api-key\nsync:\n  fps: 5\n  subsampleWidth: 128\n"
	cfg, configFile, err := loadConfigYAML(t, body)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Sync.FPS != MinFPS {
		t.Fatalf("Sync.FPS = %d, want %d", cfg.Sync.FPS, MinFPS)
	}

	// Load must not edit the file: it is shared with openhue-cli, Save is a
	// full rewrite that drops comments, and read-only tools call Load too.
	written, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read back config: %v", err)
	}
	if string(written) != body {
		t.Errorf("Load rewrote the config file.\ngot:\n%s\nwant:\n%s", written, body)
	}
}

// TestSavePersistsSyncFields guards the whole SyncConfig struct through a
// Save/Load round trip. A per-field Set("sync.restoreToken") used to shadow the
// struct, so FPS and subsampleWidth changes were silently dropped while the
// token itself survived; the restore-token chain and the tray's FPS control
// both depend on this working.
func TestSavePersistsSyncFields(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	body := "Bridge: 192.168.1.100\nKey: test-api-key\nsync:\n  fps: 30\n  subsampleWidth: 128\n  restoreToken: old-token\n"
	if err := os.WriteFile(configFile, []byte(body), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	originalGetConfigFile := getConfigFile
	originalGetConfigPath := getConfigPath
	getConfigFile = func() string { return configFile }
	getConfigPath = func() string { return tempDir }
	defer func() {
		getConfigFile = originalGetConfigFile
		getConfigPath = originalGetConfigPath
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Sync.RestoreToken != "old-token" {
		t.Fatalf("restoreToken did not load: %q", cfg.Sync.RestoreToken)
	}

	cfg.Sync.FPS = 45
	cfg.Sync.SubsampleWidth = 96
	cfg.Sync.RestoreToken = "new-token"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save error = %v", err)
	}
	if reloaded.Sync.FPS != 45 {
		t.Errorf("Sync.FPS = %d, want 45", reloaded.Sync.FPS)
	}
	if reloaded.Sync.SubsampleWidth != 96 {
		t.Errorf("Sync.SubsampleWidth = %d, want 96", reloaded.Sync.SubsampleWidth)
	}
	if reloaded.Sync.RestoreToken != "new-token" {
		t.Errorf("Sync.RestoreToken = %q, want \"new-token\"", reloaded.Sync.RestoreToken)
	}
}
