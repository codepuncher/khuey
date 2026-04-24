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
	gameMode  *GameModeDetector
	kwin      *KWinDetector
	callback  StateChangeCallback
	isRunning bool
	stopChan  chan struct{}
	mu        sync.RWMutex

	// Configuration
	pollInterval  time.Duration
	debounceDelay time.Duration
	useGameMode   bool
	useFullscreen bool

	// State tracking
	currentState   bool
	stateChangedAt time.Time
}

// Config holds detector configuration
type Config struct {
	PollInterval  time.Duration // How often to check (default: 2s)
	DebounceDelay time.Duration // Wait before triggering (default: 5s)
	UseGameMode   bool          // Enable GameMode detection (default: true)
	UseFullscreen bool          // Enable fullscreen detection (default: true)
}

// DefaultConfig returns default detector configuration
func DefaultConfig() Config {
	return Config{
		PollInterval:  2 * time.Second,
		DebounceDelay: 5 * time.Second,
		UseGameMode:   true,
		UseFullscreen: true,
	}
}

// NewDetector creates a new gaming detector
func NewDetector(cfg Config, callback StateChangeCallback) (*Detector, error) {
	d := &Detector{
		callback:       callback,
		pollInterval:   cfg.PollInterval,
		debounceDelay:  cfg.DebounceDelay,
		useGameMode:    cfg.UseGameMode,
		useFullscreen:  cfg.UseFullscreen,
		stopChan:       make(chan struct{}),
		currentState:   false,
		stateChangedAt: time.Now(),
	}

	// Initialize GameMode detector if enabled
	if cfg.UseGameMode {
		gameMode, err := NewGameModeDetector()
		if err != nil {
			log.Printf("⚠️  GameMode detection unavailable: %v", err)
			log.Println("   Will use fullscreen detection only")
		} else {
			d.gameMode = gameMode
			log.Println("✅ GameMode detector initialized")
		}
	}

	// Initialize KWin detector if enabled
	if cfg.UseFullscreen {
		kwin, err := NewKWinDetector()
		if err != nil {
			log.Printf("⚠️  KWin fullscreen detection unavailable: %v", err)
			log.Println("   Will use GameMode detection only")
		} else {
			d.kwin = kwin
			log.Println("✅ KWin fullscreen detector initialized")
		}
	}

	// Check if at least one detector is available
	if d.gameMode == nil && d.kwin == nil {
		log.Println("❌ No gaming detectors available - feature disabled")
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
	d.isRunning = true
	d.mu.Unlock()

	log.Println("🎮 Gaming mode detector started")
	go d.monitorLoop()
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
	log.Println("🎮 Gaming mode detector stopped")
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

	// State changed?
	if isGaming != d.currentState {
		// Update state change timestamp
		d.stateChangedAt = time.Now()
		d.currentState = isGaming

		// Check if enough time has passed for debouncing
		if time.Since(d.stateChangedAt) >= d.debounceDelay {
			// Debounce period elapsed - trigger callback
			log.Printf("🎮 Gaming state changed: %v (debounced)", isGaming)
			if d.callback != nil {
				go d.callback(isGaming)
			}
		} else {
			// Still in debounce period - log but don't trigger
			log.Printf("🎮 Gaming state change detected: %v (waiting for debounce)", isGaming)
		}
	} else {
		// State hasn't changed - check if we need to trigger debounced callback
		if d.currentState != isGaming && time.Since(d.stateChangedAt) >= d.debounceDelay {
			// Debounce period elapsed with stable state - trigger callback
			log.Printf("🎮 Gaming state stabilized: %v", isGaming)
			if d.callback != nil {
				go d.callback(isGaming)
			}
		}
	}
}

// detectGaming checks if gaming is active using available detectors
func (d *Detector) detectGaming() bool {
	// Check GameMode first (most reliable for modern games)
	if d.gameMode != nil && d.gameMode.IsActive() {
		return true
	}

	// Fall back to fullscreen detection
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
