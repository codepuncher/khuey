package config

import (
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
