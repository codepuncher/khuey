package gaming

import (
	"log"
	"sync"
	"time"
)

// StateChangeCallback is called when gaming state changes
type StateChangeCallback func(isGaming bool)

// Detector monitors gaming activity and triggers callbacks on state changes
type Detector struct {
	// CachyOS-optimized detectors
	systemdDetector      *SystemdDetector
	powerProfileDetector *PowerProfileDetector
	steamDetector        *SteamDetector

	// Legacy detector (fallback)
	gameMode *GameModeDetector

	callback  StateChangeCallback
	isRunning bool
	stopChan  chan struct{}
	mu        sync.RWMutex

	// Configuration
	pollInterval      time.Duration
	debounceDelay     time.Duration
	useSystemdInhibit bool
	usePowerProfile   bool
	useSteamAppId     bool
	useGameMode       bool

	// State tracking
	currentState      bool
	pendingState      bool      // State waiting for debounce
	stateChangedAt    time.Time // When state first changed
	debounceTriggered bool      // Whether callback was already triggered for this state
}

// Config holds detector configuration
type Config struct {
	PollInterval  time.Duration // How often to check (default: 2s)
	DebounceDelay time.Duration // Wait before triggering (default: 5s)

	// CachyOS-optimized detection (recommended)
	UseSystemdInhibit bool // systemd-inhibit check (default: true)
	UsePowerProfile   bool // Power profile validation (default: true)
	UseSteamAppId     bool // Steam AppId detection (default: true)

	// Legacy detection (fallback)
	UseGameMode bool // Enable GameMode detection (default: false)

	// The state to start from. A detector replacing one that had already seen
	// a game start needs to begin believing it, or it could never report that
	// game ending: it only calls back on a change.
	InitiallyGaming bool
}

// DefaultConfig returns default detector configuration
func DefaultConfig() Config {
	return Config{
		PollInterval:      2 * time.Second,
		DebounceDelay:     5 * time.Second,
		UseSystemdInhibit: true,  // CachyOS primary detection
		UsePowerProfile:   true,  // CachyOS secondary validation
		UseSteamAppId:     true,  // Steam-specific detection
		UseGameMode:       false, // Legacy (not installed by default)
	}
}

// NewDetector creates a new gaming detector
func NewDetector(cfg Config, callback StateChangeCallback) (*Detector, error) {
	d := &Detector{
		callback:          callback,
		pollInterval:      cfg.PollInterval,
		debounceDelay:     cfg.DebounceDelay,
		useSystemdInhibit: cfg.UseSystemdInhibit,
		usePowerProfile:   cfg.UsePowerProfile,
		useSteamAppId:     cfg.UseSteamAppId,
		useGameMode:       cfg.UseGameMode,
		currentState:      cfg.InitiallyGaming,
		pendingState:      cfg.InitiallyGaming,
		stateChangedAt:    time.Now(),
		debounceTriggered: false,
	}

	// Initialize CachyOS-optimized detectors
	if cfg.UseSystemdInhibit {
		d.systemdDetector = NewSystemdDetector()
		log.Println("[INFO] systemd-inhibit detector initialized (CachyOS)")
	}

	if cfg.UsePowerProfile {
		d.powerProfileDetector = NewPowerProfileDetector()
		log.Println("[INFO] Power profile detector initialized (CachyOS)")
	}

	if cfg.UseSteamAppId {
		d.steamDetector = NewSteamDetector()
		log.Println("[INFO] Steam AppId detector initialized")
	}

	// Initialize the legacy detector as fallback
	if cfg.UseGameMode {
		gameMode, err := NewGameModeDetector()
		if err != nil {
			log.Printf("[WARN] GameMode detection unavailable: %v", err)
			log.Println("   Will use other detection methods")
		} else {
			d.gameMode = gameMode
			log.Println("[INFO] GameMode detector initialized (legacy)")
		}
	}

	// Check if at least one detector is available
	if d.systemdDetector == nil && d.powerProfileDetector == nil && d.steamDetector == nil &&
		d.gameMode == nil {
		log.Println("[WARN] No gaming detection methods available, gaming mode disabled")
		return nil, nil
	}

	return d, nil
}

// Start begins monitoring for gaming activity
func (d *Detector) Start() {
	if d == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isRunning {
		return
	}
	// Per run, since Stop closes it.
	d.stopChan = make(chan struct{})
	d.isRunning = true

	log.Println("[INFO] Gaming mode detector started")
	go d.monitorLoop(d.stopChan)
}

// Stop stops monitoring
func (d *Detector) Stop() {
	if d == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.isRunning {
		return
	}
	d.isRunning = false
	// Closed under the lock: once it is released, a Start can replace the
	// channel, and closing after that would stop the new run instead.
	close(d.stopChan)
	log.Println("[INFO] Gaming mode detector stopped")
}

// monitorLoop polls for gaming activity and triggers callbacks with debouncing.
// It takes its run's stop channel rather than reading d.stopChan, which the
// next Start replaces.
func (d *Detector) monitorLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			d.checkGamingState()
		}
	}
}

// checkGamingState checks current gaming state and triggers callback if changed
func (d *Detector) checkGamingState() {
	isGaming := d.detectGaming()

	d.mu.Lock()
	defer d.mu.Unlock()

	// State changed from what we're currently reporting?
	if isGaming != d.currentState {
		// First time seeing this new state - start debounce timer
		if isGaming != d.pendingState {
			d.pendingState = isGaming
			d.stateChangedAt = time.Now()
			d.debounceTriggered = false
			log.Printf("[INFO] Gaming state change detected: %v → %v (waiting for debounce)", d.currentState, isGaming)
			return
		}

		// State is same as pending state - check if debounce period elapsed
		if !d.debounceTriggered && time.Since(d.stateChangedAt) >= d.debounceDelay {
			// Debounce period elapsed - trigger callback and update state
			d.currentState = isGaming
			d.debounceTriggered = true
			log.Printf("[INFO] Gaming state changed: %v (debounced after %v)", isGaming, time.Since(d.stateChangedAt))
			if d.callback != nil {
				go d.callback(isGaming)
			}
		}
	} else {
		// State matches current state - reset pending state
		if d.pendingState != d.currentState {
			d.pendingState = d.currentState
			d.debounceTriggered = false
		}
	}
}

// detectGaming checks if gaming is active using available detectors
// Priority order (highest to lowest):
// 1. systemd-inhibit (CachyOS game-performance - most reliable)
// 2. Power profile + Steam AppId (combined validation - high confidence)
// 3. GameMode DBus (legacy fallback - if installed)
func (d *Detector) detectGaming() bool {
	// Priority 1: systemd-inhibit (CachyOS game-performance)
	// This is the most reliable method on CachyOS
	if d.systemdDetector != nil && d.systemdDetector.IsActive() {
		return true
	}

	// Priority 2: Power profile + Steam AppId (combined validation)
	// Requires both to be true to avoid false positives
	// (user might manually set performance profile)
	if d.powerProfileDetector != nil && d.steamDetector != nil {
		if d.powerProfileDetector.IsPerformanceMode() && d.steamDetector.IsActive() {
			return true
		}
	}

	// Priority 3: Legacy GameMode detection (fallback)
	if d.gameMode != nil && d.gameMode.IsActive() {
		return true
	}

	return false
}

// IsGaming returns the current gaming state (without debouncing)
func (d *Detector) IsGaming() bool {
	if d == nil {
		return false
	}
	return d.detectGaming()
}

// Close releases all resources
func (d *Detector) Close() {
	if d == nil {
		return
	}

	d.Stop()

	if d.gameMode != nil {
		d.gameMode.Close()
	}
}
