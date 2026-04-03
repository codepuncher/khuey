package config

import (
	"fmt"
	"log"
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

	// Check file permissions - warn if too permissive (contains sensitive API keys)
	if info, err := os.Stat(configFile); err == nil {
		// Check if file is world-readable (0044) or group-readable (0044)
		if info.Mode().Perm()&0044 != 0 {
			log.Printf("⚠️  WARNING: Config file has insecure permissions: %o (should be 0600)", info.Mode().Perm())
			log.Printf("   File contains sensitive API keys and should only be readable by owner")
			log.Printf("   Fix with: chmod 600 %s", configFile)
		}
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

	configFile := getConfigFile()

	// Write to file
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Secure the config file - set permissions to 0600 (owner read/write only)
	// This is critical as the file contains sensitive API keys
	if err := os.Chmod(configFile, 0600); err != nil {
		return fmt.Errorf("failed to secure config file permissions: %w", err)
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

// Validate checks if the configuration is valid and returns helpful errors
func (c *Config) Validate() error {
	// Basic bridge configuration
	if c.Bridge == "" {
		return fmt.Errorf("bridge IP not configured\n" +
			"  → Add 'Bridge: YOUR_BRIDGE_IP' to ~/.openhue/config.yaml\n" +
			"  → You can discover your bridge with: openhue-cli discover")
	}

	if c.Key == "" {
		return fmt.Errorf("API key not configured\n" +
			"  → Add 'Key: YOUR_API_KEY' to ~/.openhue/config.yaml\n" +
			"  → Generate a key by pressing the bridge button and running: openhue-cli register")
	}

	// Validate sync settings
	if c.Sync.FPS < 1 || c.Sync.FPS > 60 {
		return fmt.Errorf("sync.fps must be between 1 and 60 (got %d)\n"+
			"  → Update 'sync.fps' in config.yaml\n"+
			"  → Recommended: 20-30 for balanced performance", c.Sync.FPS)
	}

	if c.Sync.SubsampleWidth < 16 || c.Sync.SubsampleWidth > 1920 {
		return fmt.Errorf("sync.subsampleWidth must be between 16 and 1920 (got %d)\n"+
			"  → Update 'sync.subsampleWidth' in config.yaml\n"+
			"  → Recommended: 64 for good balance", c.Sync.SubsampleWidth)
	}

	// Validate channels if configured
	for i, ch := range c.Channels {
		if err := validateChannel(i, ch); err != nil {
			return err
		}
	}

	return nil
}

// validateChannel validates a single channel configuration
func validateChannel(index int, ch ChannelConfig) error {
	// Validate UV coordinates (must be 0.0-1.0)
	if ch.UVA.X < 0 || ch.UVA.X > 1 {
		return fmt.Errorf("channel %d: uvA.x must be 0.0-1.0 (got %.2f)\n"+
			"  → UV coordinates represent screen position as fractions\n"+
			"  → 0.0 = left/top edge, 1.0 = right/bottom edge", index, ch.UVA.X)
	}
	if ch.UVA.Y < 0 || ch.UVA.Y > 1 {
		return fmt.Errorf("channel %d: uvA.y must be 0.0-1.0 (got %.2f)", index, ch.UVA.Y)
	}
	if ch.UVB.X < 0 || ch.UVB.X > 1 {
		return fmt.Errorf("channel %d: uvB.x must be 0.0-1.0 (got %.2f)", index, ch.UVB.X)
	}
	if ch.UVB.Y < 0 || ch.UVB.Y > 1 {
		return fmt.Errorf("channel %d: uvB.y must be 0.0-1.0 (got %.2f)", index, ch.UVB.Y)
	}

	// Validate UV ordering (A should be top-left, B should be bottom-right)
	if ch.UVA.X >= ch.UVB.X {
		return fmt.Errorf("channel %d: uvA.x (%.2f) must be less than uvB.x (%.2f)\n"+
			"  → uvA is top-left corner, uvB is bottom-right corner", index, ch.UVA.X, ch.UVB.X)
	}
	if ch.UVA.Y >= ch.UVB.Y {
		return fmt.Errorf("channel %d: uvA.y (%.2f) must be less than uvB.y (%.2f)\n"+
			"  → uvA is top-left corner, uvB is bottom-right corner", index, ch.UVA.Y, ch.UVB.Y)
	}

	// Validate gamma factor
	if ch.GammaFactor < 0.5 || ch.GammaFactor > 4.0 {
		return fmt.Errorf("channel %d: gammaFactor must be 0.5-4.0 (got %.2f)\n"+
			"  → Recommended: 2.2 for standard displays", index, ch.GammaFactor)
	}

	// Warn if device name is empty (non-fatal)
	if ch.DeviceName == "" && ch.Active {
		// This is just a warning, not an error
		fmt.Printf("⚠️  Warning: channel %d has no deviceName set\n", index)
	}

	return nil
}
