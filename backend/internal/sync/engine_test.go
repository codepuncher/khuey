package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/color"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/entertainment"
)

// testEngineConfig returns a config with the minimum fields NewEngine needs
// to construct successfully (no network calls happen until Start/Connect).
func testEngineConfig(fps int) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-api-key"
	cfg.ClientKey = "0123456789ABCDEF0123456789ABCDEF"
	cfg.EntertainmentConfigurationID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	cfg.Channels = []config.ChannelConfig{
		{ID: 0, Active: true, DeviceName: "Test Light", UVA: config.UV{X: 0.0, Y: 0.0}, UVB: config.UV{X: 1.0, Y: 1.0}},
	}
	cfg.Sync.FPS = fps
	return cfg
}

// newTestEngine builds an Engine, skipping when no session bus is reachable.
// NewEngine constructs a ScreenCapture, which connects to the session bus, and
// a headless CI runner may not have one.
func newTestEngine(t *testing.T, cfg *config.Config) *Engine {
	t.Helper()

	engine, err := NewEngine(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "session bus") {
			t.Skipf("no DBus session bus available: %v", err)
		}
		t.Fatalf("NewEngine() error = %v", err)
	}
	t.Cleanup(engine.capturer.Stop)
	return engine
}

// TestNewEngine_HonorsConfiguredFPS is a regression test for F3: NewEngine
// used to hardcode fps to 30 in both the engine field and the capture.Config
// passed to NewScreenCapture, silently ignoring cfg.Sync.FPS.
func TestNewEngine_HonorsConfiguredFPS(t *testing.T) {
	const configuredFPS = 45 // deliberately not the old hardcoded default of 30

	engine := newTestEngine(t, testEngineConfig(configuredFPS))

	if got := int(engine.fps.Load()); got != configuredFPS {
		t.Errorf("engine.fps = %d, want %d (configured FPS)", got, configuredFPS)
	}

	// The capture config was hardcoded separately from the engine field, so
	// asserting only the latter leaves half the bug uncovered.
	wantInterval := time.Second / time.Duration(configuredFPS)
	if got := engine.capturer.GetFrameInterval(); got != wantInterval {
		t.Errorf("capturer.GetFrameInterval() = %v, want %v", got, wantInterval)
	}
}

// TestEngine_SetFPS_ValidatesBounds covers the FPS bounds SetFPS enforces,
// which now match capture.MinFPS/capture.MaxFPS rather than a separately
// hardcoded 1-60 range.
func TestEngine_SetFPS_ValidatesBounds(t *testing.T) {
	tests := []struct {
		name    string
		fps     int
		wantErr bool
	}{
		{name: "below minimum", fps: capture.MinFPS - 1, wantErr: true},
		{name: "at minimum", fps: capture.MinFPS, wantErr: false},
		{name: "at maximum", fps: capture.MaxFPS, wantErr: false},
		{name: "above maximum", fps: capture.MaxFPS + 1, wantErr: true},
	}

	engine := newTestEngine(t, testEngineConfig(config.DefaultFPS))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.SetFPS(tt.fps)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetFPS(%d) error = %v, wantErr %v", tt.fps, err, tt.wantErr)
			}
			if got := int(engine.fps.Load()); !tt.wantErr && got != tt.fps {
				t.Errorf("engine.fps = %d, want %d after successful SetFPS", got, tt.fps)
			}
		})
	}
}

// TestCreateDefaultZone tests the auto-split zone creation
func TestCreateDefaultZone(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		total    int
		expected color.Zone
	}{
		{
			name:     "Single channel",
			index:    0,
			total:    1,
			expected: color.Zone{U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0},
		},
		{
			name:     "Two channels - first",
			index:    0,
			total:    2,
			expected: color.Zone{U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		},
		{
			name:     "Two channels - second",
			index:    1,
			total:    2,
			expected: color.Zone{U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
		},
		{
			name:     "Three channels - first",
			index:    0,
			total:    3,
			expected: color.Zone{U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0},
		},
		{
			name:     "Three channels - middle",
			index:    1,
			total:    3,
			expected: color.Zone{U1: 0.33, V1: 0.0, U2: 0.67, V2: 1.0},
		},
		{
			name:     "Three channels - last",
			index:    2,
			total:    3,
			expected: color.Zone{U1: 0.67, V1: 0.0, U2: 1.0, V2: 1.0},
		},
		{
			name:     "Four channels - third",
			index:    2,
			total:    4,
			expected: color.Zone{U1: 0.5, V1: 0.0, U2: 0.75, V2: 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone := createDefaultZone(tt.index, tt.total)

			if !floatClose(zone.U1, tt.expected.U1, 0.01) {
				t.Errorf("Zone U1: expected %.2f, got %.2f", tt.expected.U1, zone.U1)
			}
			if !floatClose(zone.V1, tt.expected.V1, 0.01) {
				t.Errorf("Zone V1: expected %.2f, got %.2f", tt.expected.V1, zone.V1)
			}
			if !floatClose(zone.U2, tt.expected.U2, 0.01) {
				t.Errorf("Zone U2: expected %.2f, got %.2f", tt.expected.U2, zone.U2)
			}
			if !floatClose(zone.V2, tt.expected.V2, 0.01) {
				t.Errorf("Zone V2: expected %.2f, got %.2f", tt.expected.V2, zone.V2)
			}
		})
	}
}

// TestCreateDefaultZone_ManyChannels tests even division for many channels
func TestCreateDefaultZone_ManyChannels(t *testing.T) {
	for _, total := range []int{4, 5, 10} {
		zones := make([]color.Zone, total)
		for i := range zones {
			zones[i] = createDefaultZone(i, total)
		}

		// First zone starts at 0
		if !floatClose(zones[0].U1, 0.0, 0.001) {
			t.Errorf("%d channels: first zone should start at 0.0, got %.3f", total, zones[0].U1)
		}
		// Last zone ends at 1
		if !floatClose(zones[total-1].U2, 1.0, 0.001) {
			t.Errorf("%d channels: last zone should end at 1.0, got %.3f", total, zones[total-1].U2)
		}
		// No gaps between zones
		for i := 0; i < total-1; i++ {
			if !floatClose(zones[i].U2, zones[i+1].U1, 0.001) {
				t.Errorf("%d channels: gap between zone %d and %d: %.3f != %.3f", total, i, i+1, zones[i].U2, zones[i+1].U1)
			}
		}
		// All zones span full height
		for i, zone := range zones {
			if zone.V1 != 0.0 || zone.V2 != 1.0 {
				t.Errorf("Zone %d: expected full height, got V1=%.3f V2=%.3f", i, zone.V1, zone.V2)
			}
		}
	}
}

// TestValidateUVCoordinates tests UV coordinate validation
func TestValidateUVCoordinates(t *testing.T) {
	tests := []struct {
		name        string
		uvA         config.UV
		uvB         config.UV
		expectError bool
	}{
		{
			name:        "Valid coordinates",
			uvA:         config.UV{X: 0.0, Y: 0.0},
			uvB:         config.UV{X: 0.5, Y: 1.0},
			expectError: false,
		},
		{
			name:        "Full screen",
			uvA:         config.UV{X: 0.0, Y: 0.0},
			uvB:         config.UV{X: 1.0, Y: 1.0},
			expectError: false,
		},
		{
			name:        "uvA.X out of bounds (negative)",
			uvA:         config.UV{X: -0.1, Y: 0.0},
			uvB:         config.UV{X: 0.5, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvA.X out of bounds (>1)",
			uvA:         config.UV{X: 1.5, Y: 0.0},
			uvB:         config.UV{X: 2.0, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.X out of bounds (>1)",
			uvA:         config.UV{X: 0.0, Y: 0.0},
			uvB:         config.UV{X: 1.5, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.X <= uvA.X (zero width)",
			uvA:         config.UV{X: 0.5, Y: 0.0},
			uvB:         config.UV{X: 0.5, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.X < uvA.X (inverted)",
			uvA:         config.UV{X: 0.7, Y: 0.0},
			uvB:         config.UV{X: 0.3, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.Y <= uvA.Y (zero height)",
			uvA:         config.UV{X: 0.0, Y: 0.5},
			uvB:         config.UV{X: 1.0, Y: 0.5},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUVCoordinates(&tt.uvA, &tt.uvB)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error=%v, got %v", tt.expectError, err)
			}
		})
	}
}

// TestCreateZonesFromConfig tests zone creation from config
func TestCreateZonesFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		expectedZones int
		checkFirst    *color.Zone
	}{
		{
			name: "Three channels with UV coordinates",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Left", GammaFactor: 2.2, UVA: config.UV{X: 0.0, Y: 0.0}, UVB: config.UV{X: 0.33, Y: 1.0}},
					{ID: 1, Active: true, DeviceName: "Center", GammaFactor: 2.2, UVA: config.UV{X: 0.33, Y: 0.0}, UVB: config.UV{X: 0.67, Y: 1.0}},
					{ID: 2, Active: true, DeviceName: "Right", GammaFactor: 2.2, UVA: config.UV{X: 0.67, Y: 0.0}, UVB: config.UV{X: 1.0, Y: 1.0}},
				},
			},
			expectedZones: 3,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0, Name: "Left"},
		},
		{
			name: "Two channels without UV (auto-split)",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Left"},
					{ID: 1, Active: true, DeviceName: "Right"},
				},
			},
			expectedZones: 2,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		},
		{
			name: "One inactive channel filtered out",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Active", UVA: config.UV{X: 0.0, Y: 0.0}, UVB: config.UV{X: 1.0, Y: 1.0}},
					{ID: 1, Active: false, DeviceName: "Inactive"},
				},
			},
			expectedZones: 1,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0, Name: "Active"},
		},
		{
			name: "Invalid UV coordinates fall back to auto-split",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Bad UV", UVA: config.UV{X: 1.0, Y: 0.0}, UVB: config.UV{X: 0.5, Y: 1.0}},
				},
			},
			expectedZones: 1,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zones := createZonesFromConfig(tt.config)

			if len(zones) != tt.expectedZones {
				t.Errorf("Expected %d zones, got %d", tt.expectedZones, len(zones))
			}

			if tt.checkFirst != nil && len(zones) > 0 {
				zone := zones[0]
				if zone.ID != tt.checkFirst.ID {
					t.Errorf("Zone ID: expected %d, got %d", tt.checkFirst.ID, zone.ID)
				}
				if !floatClose(zone.U1, tt.checkFirst.U1, 0.01) {
					t.Errorf("Zone U1: expected %.2f, got %.2f", tt.checkFirst.U1, zone.U1)
				}
				if !floatClose(zone.V1, tt.checkFirst.V1, 0.01) {
					t.Errorf("Zone V1: expected %.2f, got %.2f", tt.checkFirst.V1, zone.V1)
				}
				if !floatClose(zone.U2, tt.checkFirst.U2, 0.01) {
					t.Errorf("Zone U2: expected %.2f, got %.2f", tt.checkFirst.U2, zone.U2)
				}
				if !floatClose(zone.V2, tt.checkFirst.V2, 0.01) {
					t.Errorf("Zone V2: expected %.2f, got %.2f", tt.checkFirst.V2, zone.V2)
				}
				if zone.Name != tt.checkFirst.Name {
					t.Errorf("Zone Name: expected %s, got %s", tt.checkFirst.Name, zone.Name)
				}
			}
		})
	}
}

// TestColor8BitTo16BitConversion tests the color bit conversion via the real function
func TestColor8BitTo16BitConversion(t *testing.T) {
	tests := []struct {
		input8bit     uint8
		expected16bit uint16
	}{
		{input8bit: 0, expected16bit: 0},
		{input8bit: 255, expected16bit: 65535},
		{input8bit: 128, expected16bit: 32896},
		{input8bit: 64, expected16bit: 16448},
		{input8bit: 192, expected16bit: 49344},
	}

	for _, tt := range tests {
		result := entertainment.Convert8BitTo16Bit(tt.input8bit)
		if result != tt.expected16bit {
			t.Errorf("Convert8BitTo16Bit(%d): expected %d, got %d", tt.input8bit, tt.expected16bit, result)
		}
	}
}

// Helper functions

func floatClose(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

// TestDropCounterRebasesOnIntervalChange covers the FPS-change path of the
// drop metric. The elapsed span is measured against the interval in force at
// the time, so carrying it across a raise in FPS divides it by a shorter one
// and reports drops that never happened.
func TestDropCounterRebasesOnIntervalChange(t *testing.T) {
	slow := 100 * time.Millisecond // 10 FPS
	fast := 20 * time.Millisecond  // 50 FPS
	start := time.Now()

	d := &dropCounter{interval: slow, lastTick: start}

	// One on-time frame at 10 FPS.
	if dropped := d.observe(start.Add(slow)); dropped != 0 {
		t.Fatalf("on-time frame reported %d drops, want 0", dropped)
	}

	// The FPS change lands later than the tick that began the frame; that gap
	// is what a missing rebase would divide by the new, shorter interval.
	changed := start.Add(slow).Add(60 * time.Millisecond)
	d.setInterval(fast, changed)

	// The next frame arrives one new interval later, which is on time.
	if dropped := d.observe(changed.Add(fast)); dropped != 0 {
		t.Errorf("first frame after an FPS raise reported %d drops, want 0", dropped)
	}
}

func TestDropCounterCountsRealDrops(t *testing.T) {
	interval := 100 * time.Millisecond
	start := time.Now()
	d := &dropCounter{interval: interval, lastTick: start}

	// Five intervals of wall clock for one processed frame is four missed.
	if dropped := d.observe(start.Add(5 * interval)); dropped != 4 {
		t.Errorf("observe reported %d drops, want 4", dropped)
	}

	// The next on-time frame is measured from the previous one, not the start.
	if dropped := d.observe(start.Add(6 * interval)); dropped != 0 {
		t.Errorf("frame after a drop burst reported %d drops, want 0", dropped)
	}
}

// TestSetFPSPropagatesToCapturer guards the half of SetFPS that is easy to
// miss: the frame reader loop runs off the capturer's own rate, so leaving it
// stale caps how often a new frame exists and the sync loop just re-reads the
// previous one at the higher rate.
func TestSetFPSPropagatesToCapturer(t *testing.T) {
	engine := newTestEngine(t, testEngineConfig(config.DefaultFPS))

	const newFPS = 50
	if err := engine.SetFPS(newFPS); err != nil {
		t.Fatalf("SetFPS(%d) error = %v", newFPS, err)
	}

	want := time.Second / time.Duration(newFPS)
	if got := engine.capturer.GetFrameInterval(); got != want {
		t.Errorf("capturer.GetFrameInterval() = %v, want %v", got, want)
	}
}

// TestDropCounterCountsSustainedOverload covers the two cases an integer
// tick count alone gets wrong: a frame time just under two intervals, which
// truncates to zero drops forever, and a delivered rate far below target,
// which an interval-count stall bound would discard as a clock jump.
func TestDropCounterCountsSustainedOverload(t *testing.T) {
	tests := []struct {
		name      string
		interval  time.Duration
		frameTime time.Duration
		frames    int
		wantMin   int
		wantMax   int
	}{
		{
			// 30 FPS target, each frame taking 1.9 intervals: ~0.9 drops per
			// frame, so 100 frames is ~90 and must not be 0.
			name: "mild overload is not truncated away", interval: 33333333 * time.Nanosecond,
			frameTime: 63333333 * time.Nanosecond, frames: 100, wantMin: 85, wantMax: 95,
		},
		{
			// 60 FPS target delivering 1 FPS: 59 drops per frame.
			name: "severe overload is not mistaken for a stall", interval: time.Second / 60,
			frameTime: time.Second, frames: 10, wantMin: 580, wantMax: 600,
		},
		{
			name: "on time reports nothing", interval: time.Second / 30,
			frameTime: time.Second / 30, frames: 100, wantMin: 0, wantMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			d := &dropCounter{interval: tt.interval, lastTick: now}

			total := 0
			for i := 0; i < tt.frames; i++ {
				now = now.Add(tt.frameTime)
				total += d.observe(now)
			}

			if total < tt.wantMin || total > tt.wantMax {
				t.Errorf("total drops over %d frames = %d, want %d..%d", tt.frames, total, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestDropCounterIgnoresStalls covers suspend/resume: the clock jumps past any
// plausible backlog, and counting it pins the cumulative drop rate near 100%
// for the rest of the sync session.
func TestDropCounterIgnoresStalls(t *testing.T) {
	tests := []struct {
		name    string
		gap     time.Duration
		want    int
		wantMin int
	}{
		{name: "just under the stall gap still counts", gap: stallGap - time.Millisecond, wantMin: 1},
		{name: "at the stall gap is a stall", gap: stallGap, want: 0},
		{name: "an hour asleep is a stall", gap: time.Hour, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			d := &dropCounter{interval: time.Second / 60, lastTick: now}

			got := d.observe(now.Add(tt.gap))
			if tt.wantMin > 0 {
				if got < tt.wantMin {
					t.Errorf("observe after %v = %d drops, want at least %d", tt.gap, got, tt.wantMin)
				}
				return
			}
			if got != tt.want {
				t.Errorf("observe after %v = %d drops, want %d", tt.gap, got, tt.want)
			}
		})
	}
}

// TestDropCounterStallDoesNotLeakCarry makes sure a discarded stall cannot
// surface as phantom drops on the frames that follow it.
func TestDropCounterStallDoesNotLeakCarry(t *testing.T) {
	interval := time.Second / 60
	now := time.Now()
	d := &dropCounter{interval: interval, lastTick: now}

	now = now.Add(time.Hour)
	if got := d.observe(now); got != 0 {
		t.Fatalf("stall reported %d drops, want 0", got)
	}

	for i := 0; i < 100; i++ {
		now = now.Add(interval)
		if got := d.observe(now); got != 0 {
			t.Fatalf("on-time frame %d after a stall reported %d drops, want 0", i, got)
		}
	}
}

func metricsFieldsAreZero(m *metricsData) map[string]bool {
	v := reflect.ValueOf(m).Elem()
	state := make(map[string]bool, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		state[v.Type().Field(i).Name] = v.Field(i).IsZero()
	}
	return state
}

// TestMetricsResetClearsEveryAccumulator guards the reported averages: a second
// sync session divides its own frame count into whatever the accumulators hold,
// so any counter surviving a reset makes every Pipeline timing wrong.
func TestMetricsResetClearsEveryAccumulator(t *testing.T) {
	var e Engine
	e.updateMetrics(12*time.Millisecond, 5*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond)
	e.metrics.framesDropped = 7
	e.metrics.startTime = time.Now().Add(-time.Hour)
	e.metrics.lastLogTime = time.Now().Add(-time.Hour)

	for name, isZero := range metricsFieldsAreZero(&e.metrics.metricsData) {
		if isZero {
			t.Fatalf("%s is still zero before the reset, so the check below proves nothing about it; populate it above", name)
		}
	}

	now := time.Now()
	e.metrics.reset(now)

	for name, isZero := range metricsFieldsAreZero(&e.metrics.metricsData) {
		if name == "startTime" || name == "lastLogTime" {
			continue
		}
		if !isZero {
			t.Errorf("reset left %s non-zero", name)
		}
	}
	if !e.metrics.startTime.Equal(now) {
		t.Errorf("startTime = %v, want %v", e.metrics.startTime, now)
	}
	if !e.metrics.lastLogTime.Equal(now) {
		t.Errorf("lastLogTime = %v, want %v", e.metrics.lastLogTime, now)
	}
}

// TestEndSessionOnlyEndsItsOwnSession covers the race where a sync loop that
// has given up sits waiting on e.mu long enough for the user to stop and start
// again. Ending blind would tear down the session that replaced it, and would
// pin the blame for a failure the new session never had.
func TestEndSessionOnlyEndsItsOwnSession(t *testing.T) {
	e := newTestEngine(t, testEngineConfig(30))
	boom := errors.New("capture gave up")

	e.mu.Lock()
	e.running = true
	e.generation = 2
	e.mu.Unlock()

	e.endSession(1, boom)

	if !e.IsRunning() {
		t.Fatal("a superseded loop stopped the session that replaced it")
	}
	if got := e.LastFailure(); got != nil {
		t.Fatalf("a superseded loop recorded %v against the session that replaced it", got)
	}

	e.endSession(2, boom)

	if e.IsRunning() {
		t.Error("a loop failed to stop its own session")
	}
	if got := e.LastFailure(); !errors.Is(got, boom) {
		t.Errorf("LastFailure = %v, want %v", got, boom)
	}
}

// TestStopClearsLastFailure makes a deliberate Stop win over a pending retry.
// After a session capture ended, a Stop finds nothing running, and a failure
// left in place would still read as one waiting to be brought back.
func TestStopClearsLastFailure(t *testing.T) {
	e := newTestEngine(t, testEngineConfig(30))

	e.mu.Lock()
	e.running = true
	e.generation = 1
	e.mu.Unlock()
	e.endSession(1, errors.New("capture gave up"))

	if e.LastFailure() == nil {
		t.Fatal("endSession recorded no failure")
	}

	if err := e.Stop(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
	}
	if got := e.LastFailure(); got != nil {
		t.Errorf("LastFailure = %v after a deliberate Stop, want nil", got)
	}
}

// TestEndSessionAfterStopRecordsNothing covers a Stop that lands between the
// loop deciding to give up and taking the lock. The Stop cleared the failure on
// purpose, and writing it back would get the session restarted against it.
func TestEndSessionAfterStopRecordsNothing(t *testing.T) {
	e := newTestEngine(t, testEngineConfig(30))

	e.mu.Lock()
	e.running = true
	e.generation = 1
	e.mu.Unlock()

	if err := e.Stop(); err != nil {
		t.Fatalf("Stop = %v", err)
	}
	e.endSession(1, errors.New("capture gave up"))

	if got := e.LastFailure(); got != nil {
		t.Errorf("LastFailure = %v after a Stop that came first, want nil", got)
	}
}

// TestStopSessionOnlyStopsItsOwnSession covers undoing a start that took long
// enough for another session to be queued behind it. Stopping whatever is
// running would end that one instead.
func TestStopSessionOnlyStopsItsOwnSession(t *testing.T) {
	e := newTestEngine(t, testEngineConfig(30))

	e.mu.Lock()
	e.running = true
	e.generation = 2
	e.mu.Unlock()

	if err := e.StopSession(1); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("StopSession(1) = %v, want ErrNotRunning", err)
	}
	if !e.IsRunning() {
		t.Fatal("undoing an old start stopped the session queued behind it")
	}

	if err := e.StopSession(2); err != nil {
		t.Fatalf("StopSession(2) = %v, want nil", err)
	}
	if e.IsRunning() {
		t.Error("StopSession failed to stop its own session")
	}
}

// TestRestoreTokenSharesTheConfigLock covers the portal handing over a restore
// token while a settings change is being saved. Only meaningful under -race.
func TestRestoreTokenSharesTheConfigLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "openhue"), 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := testEngineConfig(config.DefaultFPS)
	e := &Engine{config: cfg}

	const rounds = 50
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			e.updateRestoreToken(fmt.Sprintf("token-%d", i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			err := cfg.Update(func(c *config.Config) {
				c.Sync.FPS = config.MinFPS + i%10
			}, nil)
			if err != nil {
				t.Errorf("Update: %v", err)
			}
		}
	}()
	wg.Wait()

	var token string
	cfg.View(func(c *config.Config) {
		token = c.Sync.RestoreToken
	})
	if want := fmt.Sprintf("token-%d", rounds-1); token != want {
		t.Errorf("RestoreToken = %q, want %q", token, want)
	}
}

// newLoopTestEngine builds an Engine whose sync loop can run without a session
// bus: the capturer never started, so every frame is an ordinary capture error,
// and the client never connected, so closing it is a no-op.
func newLoopTestEngine(t *testing.T, cfg *config.Config) *Engine {
	t.Helper()

	client, err := entertainment.NewClient(entertainment.Config{
		BridgeIP:        cfg.Bridge,
		Username:        cfg.Key,
		ClientKey:       cfg.ClientKey,
		EntertainmentID: cfg.EntertainmentConfigurationID,
		ChannelCount:    len(createZonesFromConfig(cfg)),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	e := &Engine{config: cfg, capturer: &capture.ScreenCapture{}, client: client}
	e.fps.Store(int64(cfg.Sync.FPS))
	return e
}

// TestStopWaitsForTheSyncLoop covers a Stop returning while the loop is still
// running. The next session reuses the capturer and client, so a loop left
// running past Stop carries on into that session.
func TestStopWaitsForTheSyncLoop(t *testing.T) {
	cfg := testEngineConfig(config.DefaultFPS)
	e := newLoopTestEngine(t, cfg)

	// The loop reads the config on entry, so holding it keeps the loop from
	// getting as far as its first tick.
	held := make(chan struct{})
	release := make(chan struct{})
	go cfg.View(func(*config.Config) {
		close(held)
		<-release
	})
	<-held

	e.mu.Lock()
	e.launchLoopLocked(context.Background())
	e.mu.Unlock()

	stopped := make(chan error, 1)
	go func() { stopped <- e.Stop() }()

	select {
	case err := <-stopped:
		close(release)
		t.Fatalf("Stop returned (%v) while the sync loop was still running", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-stopped; err != nil {
		t.Fatalf("Stop = %v", err)
	}
}

// TestFailingSyncLoopEndsItsSession covers a loop that gives up. It ends its
// session through the same stop that waits for the loop, so it has to count
// itself finished first or wait on itself forever.
func TestFailingSyncLoopEndsItsSession(t *testing.T) {
	cfg := testEngineConfig(config.DefaultFPS)
	cfg.Sync.SubsampleWidth = color.MinSubsampleWidth - 1
	e := newLoopTestEngine(t, cfg)

	e.mu.Lock()
	e.launchLoopLocked(context.Background())
	e.mu.Unlock()

	ended := make(chan error, 1)
	go func() {
		for e.IsRunning() {
			time.Sleep(time.Millisecond)
		}
		ended <- e.LastFailure()
	}()

	select {
	case err := <-ended:
		if err == nil {
			t.Error("the session ended without recording why")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a sync loop that gave up never ended its session")
	}
}
