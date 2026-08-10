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

	// Legacy detectors (fallback)
	gameMode *GameModeDetector
	kwin     *KWinDetector

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
	useFullscreen     bool

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
	UseGameMode   bool // Enable GameMode detection (default: false)
	UseFullscreen bool // Enable fullscreen detection (default: false)
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
		UseFullscreen:     false, // Legacy (unreliable on Wayland)
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
		useFullscreen:     cfg.UseFullscreen,
		stopChan:          make(chan struct{}, 1), // Buffered to prevent blocking on close
		currentState:      false,
		pendingState:      false,
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

	// Initialize legacy detectors as fallback
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

	if cfg.UseFullscreen {
		kwin, err := NewKWinDetector()
		if err != nil {
			log.Printf("[WARN] KWin fullscreen detection unavailable: %v", err)
			log.Println("   Will use other detection methods")
		} else {
			d.kwin = kwin
			log.Println("[INFO] KWin fullscreen detector initialized (legacy)")
		}
	}

	// Check if at least one detector is available
	if d.systemdDetector == nil && d.powerProfileDetector == nil && d.steamDetector == nil &&
		d.gameMode == nil && d.kwin == nil {
		log.Println("[ERROR] No gaming detectors available - feature disabled")
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
	if d.isRunning {
		d.mu.Unlock()
		return
	}
	// Recreate stop channel for restart capability
	// (channel is closed by Stop(), must be recreated)
	d.stopChan = make(chan struct{}, 1) // Buffered to prevent blocking
	d.isRunning = true

	log.Println("[INFO] Gaming mode detector started")
	// Spawn goroutine while holding lock to prevent race
	go d.monitorLoop()
	d.mu.Unlock()
}

// Stop stops monitoring
func (d *Detector) Stop() {
	if d == nil {
		return
	}

	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = false
	d.mu.Unlock()

	close(d.stopChan)
	log.Println("[INFO] Gaming mode detector stopped")
}

// monitorLoop polls for gaming activity and triggers callbacks with debouncing
func (d *Detector) monitorLoop() {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
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
// 4. KWin fullscreen (legacy fallback - unreliable on Wayland)
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

	// Priority 4: Legacy fullscreen detection (lowest priority)
	if d.kwin != nil && d.kwin.IsFullscreenActive() {
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
	if d.kwin != nil {
		d.kwin.Close()
	}
}
