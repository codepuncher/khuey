package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// Bridge configuration (compatible with openhue-cli)
	Bridge string `mapstructure:"Bridge"`
	Key    string `mapstructure:"Key"`

	// Grouped light (room/zone) for power and brightness control
	GroupedLightID string `mapstructure:"grouped_light_id"`

	// Entertainment API specific
	ClientKey                    string          `mapstructure:"clientkey"`
	EntertainmentConfigurationID string          `mapstructure:"entertainmentConfigurationId"`
	Channels                     []ChannelConfig `mapstructure:"channels"`

	// Screen sync settings
	Sync SyncConfig `mapstructure:"sync"`

	// Logging
	LogLevel string `mapstructure:"log_level"`

	// Mutex to protect concurrent writes to config file
	mu sync.Mutex
}

// ChannelConfig represents a single light channel in the Entertainment Area
type ChannelConfig struct {
	ID          uint8   `mapstructure:"id"`
	Active      bool    `mapstructure:"active"`
	DeviceName  string  `mapstructure:"deviceName"`
	GammaFactor float32 `mapstructure:"gammaFactor"`
	UVA         UV      `mapstructure:"uvA"` // Top-left corner
	UVB         UV      `mapstructure:"uvB"` // Bottom-right corner
}

// UV represents a 2D coordinate in the 0.0-1.0 range
type UV struct {
	X float32 `mapstructure:"x"`
	Y float32 `mapstructure:"y"`
}

// SyncConfig represents screen sync settings
type SyncConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	FPS            int    `mapstructure:"fps"`
	SubsampleWidth int    `mapstructure:"subsampleWidth"`
	Monitor        string `mapstructure:"monitor"` // Monitor to capture (empty = default)
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Sync: SyncConfig{
			Enabled:        false,
			FPS:            30,
			SubsampleWidth: 64,
			Monitor:        "",
		},
		LogLevel: "info",
		Channels: []ChannelConfig{},
	}
}

// getConfigPath returns the path to the config directory
// Uses ~/.openhue/ to be compatible with openhue-cli
func getConfigPath() string {
	// Check XDG_CONFIG_HOME first
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "openhue")
	}

	// Fall back to ~/.openhue
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("Unable to determine home directory: %v", err))
	}
	return filepath.Join(home, ".openhue")
}

// getConfigFile returns the full path to the config file
func getConfigFile() string {
	return filepath.Join(getConfigPath(), "config.yaml")
}

// Load reads the configuration from ~/.openhue/config.yaml
func Load() (*Config, error) {
	configPath := getConfigPath()
	configFile := getConfigFile()

	// Ensure config directory exists
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Initialize with defaults
	cfg := DefaultConfig()

	// Set up viper
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Try to read existing config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file doesn't exist - this is OK for first run
			return cfg, nil
		}
		// Check if it's a simple "file not found" error as well
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into our struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to ~/.openhue/config.yaml
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	configFile := getConfigFile()

	// Set all values in viper
	viper.Set("Bridge", c.Bridge)
	viper.Set("Key", c.Key)
	viper.Set("grouped_light_id", c.GroupedLightID)
	viper.Set("clientkey", c.ClientKey)
	viper.Set("entertainmentConfigurationId", c.EntertainmentConfigurationID)
	viper.Set("channels", c.Channels)
	viper.Set("sync", c.Sync)
	viper.Set("log_level", c.LogLevel)

	// Validate required fields
	if c.Bridge == "" {
		return fmt.Errorf("bridge address is required")
	}
	if c.Key == "" {
		return fmt.Errorf("API key is required")
	}

	// Write to file
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// IsConfigured returns true if basic bridge configuration exists
func (c *Config) IsConfigured() bool {
	return c.Bridge != "" && c.Key != ""
}

// HasEntertainmentConfig returns true if Entertainment API is configured
func (c *Config) HasEntertainmentConfig() bool {
	return c.IsConfigured() &&
		c.ClientKey != "" &&
		c.EntertainmentConfigurationID != ""
}
