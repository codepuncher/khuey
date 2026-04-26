// Benchmark tests for config package
// Run with: go test -bench=. -benchmem
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkLoad benchmarks loading configuration from disk
func BenchmarkLoad(b *testing.B) {
	// Create temporary directory and config file
	tempDir := b.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Write a realistic config file
	configYAML := `Bridge: 192.168.1.100
Key: test-api-key-1234567890abcdefghijklmnopqrstuvwxyz
clientkey: test-client-key-abcdefghijklmnopqrstuvwxyz1234567890
entertainmentConfigurationId: abc123-def456-ghi789
sync:
  fps: 30
  subsampleWidth: 64
  enabled: false
  monitor: ""
  restoreToken: ""
gamingMode:
  enabled: true
  pollInterval: 2
  debounceDelay: 5
  useSystemdInhibit: true
  usePowerProfile: true
  useSteamAppId: true
  useGameMode: false
  useFullscreen: false
ui:
  icons:
    gaming: "applications-games"
    syncing: "media-record"
    idle: "preferences-desktop-display-color"
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA:
      x: 0.0
      y: 0.0
    uvB:
      x: 0.5
      y: 1.0
  - id: 1
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA:
      x: 0.5
      y: 0.0
    uvB:
      x: 1.0
      y: 1.0
log_level: info
`
	err := os.WriteFile(configFile, []byte(configYAML), 0600)
	if err != nil {
		b.Fatalf("Failed to write test config: %v", err)
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

	// Reset timer before actual benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load()
		if err != nil {
			b.Fatalf("Load() failed: %v", err)
		}
	}
}

// BenchmarkDefaultConfig benchmarks creating default configuration
func BenchmarkDefaultConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig()
	}
}

// BenchmarkValidate benchmarks config validation
func BenchmarkValidate(b *testing.B) {
	cfg := &Config{
		Bridge: "192.168.1.100",
		Key:    "test-api-key",
		Sync: SyncConfig{
			FPS:            30,
			SubsampleWidth: 64,
		},
		Channels: []ChannelConfig{
			{
				ID:          0,
				Active:      true,
				DeviceName:  "Left Light",
				GammaFactor: 2.2,
				UVA:         UV{X: 0.0, Y: 0.0},
				UVB:         UV{X: 0.5, Y: 1.0},
			},
			{
				ID:          1,
				Active:      true,
				DeviceName:  "Right Light",
				GammaFactor: 2.2,
				UVA:         UV{X: 0.5, Y: 0.0},
				UVB:         UV{X: 1.0, Y: 1.0},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cfg.Validate()
		if err != nil {
			b.Fatalf("Validate() failed: %v", err)
		}
	}
}

// BenchmarkSave benchmarks saving configuration to disk
func BenchmarkSave(b *testing.B) {
	// Create temporary directory for test config
	tempDir := b.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

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

	cfg := DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-api-key"
	cfg.ClientKey = "test-client-key"
	cfg.EntertainmentConfigurationID = "test-ent-id"

	// Reset timer before actual benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cfg.Save()
		if err != nil {
			b.Fatalf("Save() failed: %v", err)
		}
	}
}

// BenchmarkValidateChannels benchmarks channel validation with many channels
func BenchmarkValidateChannels(b *testing.B) {
	cfg := &Config{
		Bridge: "192.168.1.100",
		Key:    "test-api-key",
		Sync: SyncConfig{
			FPS:            30,
			SubsampleWidth: 64,
		},
		Channels: make([]ChannelConfig, 10),
	}

	// Create 10 channels
	for i := 0; i < 10; i++ {
		cfg.Channels[i] = ChannelConfig{
			ID:          uint8(i),
			Active:      true,
			DeviceName:  "Light " + string(rune('0'+i)),
			GammaFactor: 2.2,
			UVA:         UV{X: 0.0, Y: 0.0},
			UVB:         UV{X: 1.0, Y: 1.0},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cfg.Validate()
		if err != nil {
			b.Fatalf("Validate() failed: %v", err)
		}
	}
}
