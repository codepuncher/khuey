package gaming

import (
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PollInterval != 2*time.Second {
		t.Errorf("Expected poll interval 2s, got %v", cfg.PollInterval)
	}
	if cfg.DebounceDelay != 5*time.Second {
		t.Errorf("Expected debounce delay 5s, got %v", cfg.DebounceDelay)
	}

	// CachyOS-optimized detection should be enabled by default
	if !cfg.UseSystemdInhibit {
		t.Error("Expected systemd-inhibit detection enabled by default")
	}
	if !cfg.UsePowerProfile {
		t.Error("Expected power profile detection enabled by default")
	}
	if !cfg.UseSteamAppId {
		t.Error("Expected Steam AppId detection enabled by default")
	}

	// Legacy detection should be disabled by default
	if cfg.UseGameMode {
		t.Error("Expected GameMode detection disabled by default")
	}
}

func TestDetectorCreation(t *testing.T) {
	cfg := DefaultConfig()
	callback := func(isGaming bool) {
		// Callback for testing
	}

	detector, err := NewDetector(cfg, callback)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	// Detector might be nil if no detection methods available (e.g., in CI)
	if detector != nil {
		defer detector.Close()

		// Verify initial state
		if detector.pollInterval != cfg.PollInterval {
			t.Errorf("Expected poll interval %v, got %v", cfg.PollInterval, detector.pollInterval)
		}
		if detector.debounceDelay != cfg.DebounceDelay {
			t.Errorf("Expected debounce delay %v, got %v", cfg.DebounceDelay, detector.debounceDelay)
		}
	}
}

func TestDetectorStartStop(t *testing.T) {
	cfg := Config{
		PollInterval:      100 * time.Millisecond,
		DebounceDelay:     500 * time.Millisecond,
		UseSystemdInhibit: true,
	}

	detector, err := NewDetector(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	if detector == nil {
		t.Skip("No detection methods available")
	}
	defer detector.Close()

	// Start detector
	detector.Start()
	if !detector.isRunning {
		t.Error("Expected detector to be running after Start()")
	}

	// Try starting again (should be idempotent)
	detector.Start()

	// Stop detector
	detector.Stop()
	time.Sleep(150 * time.Millisecond) // Allow goroutine to exit
	if detector.isRunning {
		t.Error("Expected detector to be stopped after Stop()")
	}

	// Try stopping again (should be idempotent)
	detector.Stop()
}

func TestDetectorCallback(t *testing.T) {
	cfg := Config{
		PollInterval:      50 * time.Millisecond,
		DebounceDelay:     100 * time.Millisecond,
		UseSystemdInhibit: true,
	}

	detector, err := NewDetector(cfg, func(isGaming bool) {
		// Callback will be invoked on state changes
	})

	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	if detector == nil {
		t.Skip("No detection methods available")
	}
	defer detector.Close()

	// Note: This test doesn't actually verify callback is called
	// because we'd need to mock what the detectors shell out to
	// The test just verifies the structure is set up correctly
	if detector.callback == nil {
		t.Error("Expected callback to be set")
	}
}

func TestDetectorWithDisabledMethods(t *testing.T) {
	// Test with all methods disabled
	cfg := Config{
		PollInterval:      100 * time.Millisecond,
		DebounceDelay:     500 * time.Millisecond,
		UseSystemdInhibit: false,
		UsePowerProfile:   false,
		UseSteamAppId:     false,
		UseGameMode:       false,
	}

	detector, err := NewDetector(cfg, nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should return nil when no detectors available
	if detector != nil {
		t.Error("Expected nil detector when all methods disabled")
	}
}

func TestDetectorIsGaming(t *testing.T) {
	cfg := DefaultConfig()
	detector, err := NewDetector(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	if detector == nil {
		t.Skip("No detection methods available")
	}
	defer detector.Close()

	// This will likely return false in test environment
	// but verifies the method doesn't panic
	isGaming := detector.IsGaming()
	_ = isGaming // Use the value to avoid unused warning
}

func TestNilDetectorMethods(t *testing.T) {
	// Verify nil detector methods don't panic
	var detector *Detector

	detector.Start() // Should not panic
	detector.Stop()  // Should not panic
	detector.Close() // Should not panic

	isGaming := detector.IsGaming()
	if isGaming {
		t.Error("Expected nil detector to report not gaming")
	}
}

// TestGameModeDetector tests GameMode detection
func TestGameModeDetector(t *testing.T) {
	detector, err := NewGameModeDetector()
	if err != nil {
		t.Skip("GameMode DBus not available (expected in test environment)")
	}
	defer detector.Close()

	// IsActive should not panic even if GameMode is unavailable
	// In test environment, likely returns false
	isActive := detector.IsActive()
	_ = isActive // Just verify it doesn't panic
}

// TestGameModeDetectorNilConn tests GameMode detector with nil connection
func TestGameModeDetectorNilConn(t *testing.T) {
	detector := &GameModeDetector{conn: nil}
	isActive := detector.IsActive()
	if isActive {
		t.Error("Expected inactive with nil connection")
	}
}

// TestSteamDetector tests Steam game detection
func TestSteamDetector(t *testing.T) {
	detector := NewSteamDetector()

	// IsActive should not panic
	// Will return false unless Steam game is actually running
	isActive := detector.IsActive()
	_ = isActive

	// GetAppId should not panic
	appId := detector.GetAppId()
	_ = appId
}

// TestSystemdDetector tests systemd-inhibit detection
func TestSystemdDetector(t *testing.T) {
	detector := NewSystemdDetector()

	// IsActive should not panic
	// Will return false unless gaming inhibitor is active
	isActive := detector.IsActive()
	_ = isActive // Just verify it doesn't panic
}

// TestPowerProfileDetector tests power profile detection
func TestPowerProfileDetector(t *testing.T) {
	detector := NewPowerProfileDetector()

	// IsPerformanceMode should not panic
	// Will return false unless performance mode is active
	isPerfMode := detector.IsPerformanceMode()
	_ = isPerfMode // Just verify it doesn't panic
}

// TestDetectorStartsFromInitialState covers a detector replacing one that had
// already seen a game start. It only calls back on a change, so one starting
// from false could never report that game ending.
func TestDetectorStartsFromInitialState(t *testing.T) {
	for _, initial := range []bool{false, true} {
		cfg := DefaultConfig()
		cfg.InitiallyGaming = initial

		detector, err := NewDetector(cfg, func(bool) {})
		if err != nil {
			t.Fatalf("NewDetector: %v", err)
		}
		if detector == nil {
			t.Skip("no gaming detection methods available")
		}

		if detector.currentState != initial || detector.pendingState != initial {
			t.Errorf("InitiallyGaming=%v: detector started at current=%v pending=%v",
				initial, detector.currentState, detector.pendingState)
		}
		detector.Close()
	}
}

// TestDetectorStartStopConcurrently covers a Start landing inside a Stop. A
// Stop that closed the channel after releasing the lock could close the one
// that Start had just made, stopping the new run and double-closing it when
// that run's own Stop came. Only fully meaningful under -race.
func TestDetectorStartStopConcurrently(t *testing.T) {
	d := &Detector{pollInterval: time.Millisecond}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				d.Start()
				d.Stop()
			}
		}()
	}
	wg.Wait()

	if d.isRunning {
		t.Error("detector still running after every Start was followed by a Stop")
	}
}
