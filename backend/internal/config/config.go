package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/codepuncher/khuey/internal/color"
	"github.com/spf13/viper"
)

// Sentinel errors for common error conditions
var (
	ErrBridgeRequired = fmt.Errorf("bridge address is required")
	ErrKeyRequired    = fmt.Errorf("API key is required")
)

// Configuration constants
const (
	// ConfigVersion is incremented when breaking changes are made to config format
	ConfigVersion = 1

	// FPS limits for screen sync. Must match capture.MinFPS/capture.MaxFPS:
	// a config value that clears this check but fails capture's own bounds
	// would make NewEngine fail at startup with a config the user was told
	// was valid.
	DefaultFPS = 30
	MinFPS     = 10
	MaxFPS     = 60

	// The floor MinFPS used to carry. Only values at or above it are clamped
	// on load; below it was never valid, so it stays an error.
	legacyMinFPS = 1

	DefaultSubsampleWidth = 64
)

// Config represents the application configuration.
//
// Once a Config is shared between goroutines, read it only through View and
// change it only through Update. Both take mu, as Save does, so a write can
// neither race a read nor reach the file half-applied.
type Config struct {
	// Config version for migration tracking
	Version int `mapstructure:"version"`

	// Bridge configuration (compatible with openhue-cli)
	Bridge string `mapstructure:"Bridge"`
	Key    string `mapstructure:"Key"`

	// Grouped light (room/zone) for power and brightness control
	GroupedLightID string `mapstructure:"grouped_light_id"`

	// Entertainment API specific
	ClientKey                    string          `mapstructure:"clientkey"`
	EntertainmentConfigurationID string          `mapstructure:"entertainmentConfigurationId"`
	Channels                     []ChannelConfig `mapstructure:"channels"`

	// Display name (e.g. "Living Room - Relax") of the scene to activate on
	// startup; empty disables the feature.
	StartupScene string `mapstructure:"startupScene"`

	// Screen sync settings
	Sync SyncConfig `mapstructure:"sync"`

	// Gaming mode settings
	GamingMode GamingModeConfig `mapstructure:"gamingMode"`

	// UI settings
	UI UIConfig `mapstructure:"ui"`

	// Logging
	LogLevel string `mapstructure:"log_level"`

	mu sync.Mutex

	// Non-global viper instance for thread safety
	v *viper.Viper
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
	Monitor        string `mapstructure:"monitor"`      // Monitor to capture (empty = default)
	RestoreToken   string `mapstructure:"restoreToken"` // Portal session restore token (eliminates permission dialog)
}

// GamingModeConfig represents gaming mode auto-sync settings
type GamingModeConfig struct {
	Enabled       bool `mapstructure:"enabled"`       // Feature toggle (disabled by default)
	PollInterval  int  `mapstructure:"pollInterval"`  // How often to check in seconds (default: 2)
	DebounceDelay int  `mapstructure:"debounceDelay"` // Wait before triggering in seconds (default: 5)

	// CachyOS-optimized detection (recommended)
	UseSystemdInhibit bool `mapstructure:"useSystemdInhibit"` // systemd-inhibit check (PRIMARY)
	UsePowerProfile   bool `mapstructure:"usePowerProfile"`   // Power profile validation
	UseSteamAppId     bool `mapstructure:"useSteamAppId"`     // Steam AppId detection

	// Legacy detection (fallback)
	UseGameMode   bool `mapstructure:"useGameMode"`   // Feral GameMode (if installed)
	UseFullscreen bool `mapstructure:"useFullscreen"` // KWin fullscreen (unreliable)
}

// UIConfig represents user interface settings
type UIConfig struct {
	Icons IconConfig `mapstructure:"icons"` // Tray icon configuration
}

// IconConfig represents tray icon theme names for different states
type IconConfig struct {
	Gaming  string `mapstructure:"gaming"`  // Icon when gaming + syncing
	Syncing string `mapstructure:"syncing"` // Icon when syncing (no game)
	Idle    string `mapstructure:"idle"`    // Icon when idle
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Version:      ConfigVersion,
		StartupScene: "", // Disabled by default
		Sync: SyncConfig{
			Enabled:        false,
			FPS:            DefaultFPS,
			SubsampleWidth: DefaultSubsampleWidth,
			Monitor:        "",
			RestoreToken:   "", // Empty on first run
		},
		GamingMode: GamingModeConfig{
			Enabled:           false, // Disabled by default (opt-in)
			PollInterval:      2,     // Check every 2 seconds
			DebounceDelay:     5,     // Wait 5 seconds before triggering
			UseSystemdInhibit: true,  // CachyOS primary detection
			UsePowerProfile:   true,  // CachyOS secondary validation
			UseSteamAppId:     true,  // Steam-specific detection
			UseGameMode:       false, // Feral GameMode (not installed by default)
			UseFullscreen:     false, // KWin fullscreen (unreliable on Wayland)
		},
		UI: UIConfig{
			Icons: IconConfig{
				Gaming:  "applications-games",                // Gaming + Sync icon
				Syncing: "media-record",                      // Sync active icon
				Idle:    "preferences-desktop-display-color", // Default idle icon
			},
		},
		LogLevel: "info",
		Channels: []ChannelConfig{},
	}
}

// getConfigPath returns the path to the config directory
// Uses ~/.openhue/ to be compatible with openhue-cli
// getConfigPath returns the config directory path
// Made as a variable for testing purposes
var getConfigPath = func() string {
	// Check XDG_CONFIG_HOME first
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "openhue")
	}

	// Fall back to ~/.openhue
	home, err := os.UserHomeDir()
	if err != nil {
		// Home directory is required for config; log and use fallback
		log.Printf("[ERROR] Unable to determine home directory: %v", err)
		return filepath.Join("/tmp", ".openhue")
	}
	return filepath.Join(home, ".openhue")
}

// getConfigFile returns the full path to the config file
// Made as a variable for testing purposes
var getConfigFile = func() string {
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

	// Use a non-global viper instance for thread safety
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	// Try to read existing config
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file doesn't exist - this is OK for first run
			cfg.v = v
			return cfg, nil
		}
		// Check if it's a simple "file not found" error as well
		if os.IsNotExist(err) {
			cfg.v = v
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Check file permissions - warn if too permissive (contains sensitive API keys)
	if info, err := os.Stat(configFile); err == nil {
		if info.Mode().Perm()&0044 != 0 {
			log.Printf("[WARN] Config file has insecure permissions: %o (should be 0600)", info.Mode().Perm())
			log.Printf("  File contains sensitive API keys and should only be readable by owner")
			log.Printf("  Fix with: chmod 600 %s", configFile)
		}
	}

	// Unmarshal into our struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.v = v

	// An fps of 1-9 was accepted while the floor here was 1, so raising it to
	// match capture's would turn a previously valid config into a Load error
	// and, via log.Fatalf in main, a daemon that refuses to start. Clamp those
	// legacy values instead; anything outside them still fails Validate below.
	if cfg.Sync.FPS >= legacyMinFPS && cfg.Sync.FPS < MinFPS {
		log.Printf("[WARN] sync.fps %d is below the minimum of %d, using %d", cfg.Sync.FPS, MinFPS, MinFPS)
		log.Printf("  Update 'sync.fps' in %s to silence this", configFile)
		cfg.Sync.FPS = MinFPS
		// In memory only. Save is a full rewrite that strips comments and
		// lowercases keys in a file openhue-cli also reads, and Load runs in
		// read-only tools too, so correcting the value must not edit the
		// user's file behind their back.
	}

	// Validate the loaded configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// View calls read with the config locked against every Update and Save.
func (c *Config) View(read func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	read(c)
}

// Update applies change and saves the result under one lock, so no other
// writer's change can land between the two and be saved as part of this one.
// If the save fails, revert runs under the same lock to undo change; a nil
// revert leaves the change in memory.
func (c *Config) Update(change, revert func(*Config)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	change(c)
	err := c.saveLocked()
	if err != nil && revert != nil {
		revert(c)
	}
	return err
}

// Save writes the configuration to ~/.openhue/config.yaml
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

func (c *Config) saveLocked() error {
	if c.v == nil {
		c.v = viper.New()
		c.v.SetConfigFile(getConfigFile())
		c.v.SetConfigType("yaml")
	}

	// Set all values in viper
	c.v.Set("version", c.Version)
	c.v.Set("Bridge", c.Bridge)
	c.v.Set("Key", c.Key)
	c.v.Set("grouped_light_id", c.GroupedLightID)
	c.v.Set("startupScene", c.StartupScene)
	c.v.Set("clientkey", c.ClientKey)
	c.v.Set("entertainmentConfigurationId", c.EntertainmentConfigurationID)
	c.v.Set("channels", c.Channels)
	c.v.Set("sync", c.Sync)
	c.v.Set("gamingMode", c.GamingMode)
	c.v.Set("ui", c.UI)
	c.v.Set("log_level", c.LogLevel)

	// Do not Set("sync.<field>") individually. Viper stores a nested key as a
	// map under "sync", which replaces the whole-struct Set above, and every
	// field not named that way then silently falls back to the value already
	// in the file.

	// Validate required fields
	if c.Bridge == "" {
		return fmt.Errorf("bridge address is required")
	}
	if c.Key == "" {
		return fmt.Errorf("API key is required")
	}

	configFile := getConfigFile()

	// Write to file
	if err := c.v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Secure the config file - set permissions to 0600 (owner read/write only)
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
	if c.Sync.FPS < MinFPS || c.Sync.FPS > MaxFPS {
		return fmt.Errorf("sync.fps must be between %d and %d (got %d)\n"+
			"  → Update 'sync.fps' in config.yaml\n"+
			"  → Recommended: 20-30 for balanced performance", MinFPS, MaxFPS, c.Sync.FPS)
	}

	if c.Sync.SubsampleWidth < color.MinSubsampleWidth || c.Sync.SubsampleWidth > color.MaxSubsampleWidth {
		return fmt.Errorf("sync.subsampleWidth must be between %d and %d (got %d)\n"+
			"  → Update 'sync.subsampleWidth' in config.yaml\n"+
			"  → Recommended: %d for good balance", color.MinSubsampleWidth, color.MaxSubsampleWidth, c.Sync.SubsampleWidth, DefaultSubsampleWidth)
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
		log.Printf("[WARN] channel %d has no deviceName set", index)
	}

	return nil
}
