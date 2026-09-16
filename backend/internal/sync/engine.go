// Package sync provides screen synchronization with Hue Entertainment API.
// It captures screen content, extracts colors from zones, and streams them to Hue lights in real-time.
package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/color"
	"github.com/codepuncher/khuey/internal/common"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/entertainment"
)

// metricsData holds every counter a sync session accumulates. It is kept apart
// from the mutex so reset can clear the whole set in a single assignment, which
// a counter added later cannot escape.
type metricsData struct {
	// Timing metrics
	frameCount       uint64
	startTime        time.Time
	lastLogTime      time.Time
	totalFrameTime   time.Duration
	totalCaptureTime time.Duration
	totalExtractTime time.Duration
	totalStreamTime  time.Duration

	// Frame drop tracking
	framesDropped uint64

	// Histogram buckets for frame times (in milliseconds)
	frameTimes [100]int // 0-99ms buckets
}

// PerformanceMetrics tracks sync loop performance
type PerformanceMetrics struct {
	mu sync.RWMutex
	metricsData
}

// reset clears the counters so a sync session reports only its own frames.
func (m *PerformanceMetrics) reset(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsData = metricsData{startTime: now, lastLogTime: now}
}

// dropLogInterval throttles the frame-skip warning; the periodic metrics line
// carries the totals.
const dropLogInterval = 5 * time.Second

// captureErrLogInterval throttles the capture-error warning, which otherwise
// repeats every tick for as long as capture is down.
const captureErrLogInterval = 5 * time.Second

// streamActivationDelay is how long the bridge is given to open UDP 2100 after
// it accepts an activation.
const streamActivationDelay = 100 * time.Millisecond

// streamRetryInterval and streamConnectTimeout bound connectStream's retries.
const (
	streamRetryInterval  = 500 * time.Millisecond
	streamConnectTimeout = 10 * time.Second
)

// Sentinel errors
var (
	ErrAlreadyRunning = fmt.Errorf("sync already running")
	ErrNotRunning     = fmt.Errorf("sync not running")
)

// Engine manages screen synchronization
type Engine struct {
	config   *config.Config
	capturer *capture.ScreenCapture
	client   *entertainment.Client

	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc

	// Bumped per session so a sync loop winding down can only stop the
	// session it belongs to, never one started while it was stopping.
	generation uint64

	// The next session reuses the capturer and client, so a stop waits for
	// the loop to leave them before tearing them down. Added to and waited on
	// under mu only, which keeps an Add from racing a Wait.
	syncLoopWg sync.WaitGroup

	// Why the last session ended or failed to start, or nil if it was stopped
	// deliberately. A caller that restarts sync needs the two apart: undoing
	// the user's own Stop would be a bug, not a recovery.
	lastFailure error

	// Atomic so a settings change never blocks on e.mu, which Start holds
	// across the portal dialog and the DTLS connect.
	fps        atomic.Int64
	zones      []color.Zone
	httpClient *http.Client

	// Performance metrics
	metrics PerformanceMetrics
}

// NewEngine creates a new sync engine
func NewEngine(cfg *config.Config) (*Engine, error) {
	// Create placeholder engine to use in token callback
	engine := &Engine{
		config:     cfg,
		running:    false,
		httpClient: common.NewHueHTTPClient(), // PERF-004: Reuse HTTP client
	}
	engine.fps.Store(int64(cfg.Sync.FPS))

	zones := createZonesFromConfig(cfg)
	if len(zones) == 0 {
		return nil, fmt.Errorf("no active channels configured")
	}

	// Create screen capture - native PipeWire capture with CGo
	capturer, err := capture.NewScreenCapture(capture.Config{
		FPS:              cfg.Sync.FPS,
		Monitor:          -1,                    // All monitors
		UseMockFrames:    false,                 // Disable mock frames
		UseNativeCapture: true,                  // Use native CGo PipeWire capture
		UseScreenshot:    false,                 // Disable screenshot fallback
		CaptureWidth:     0,                     // Full resolution (native capture is fast)
		CaptureHeight:    0,                     // Full resolution (native capture is fast)
		RestoreToken:     cfg.Sync.RestoreToken, // Pass saved token
		// Token callback: capture will call this when portal returns new restore token
		// Callback runs in goroutine (see capture.Start) to avoid blocking capture startup
		OnTokenUpdate: func(newToken string) {
			engine.updateRestoreToken(newToken)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create screen capture: %w", err)
	}

	// Create Entertainment API client
	client, err := entertainment.NewClient(entertainment.Config{
		BridgeIP:        cfg.Bridge,
		Username:        cfg.Key,
		ClientKey:       cfg.ClientKey,
		EntertainmentID: cfg.EntertainmentConfigurationID,
		ChannelCount:    len(zones),
	})
	if err != nil {
		capturer.Stop()
		return nil, fmt.Errorf("failed to create entertainment client: %w", err)
	}

	// Update engine with created components
	engine.capturer = capturer
	engine.client = client
	engine.zones = zones

	return engine, nil
}

// createZonesFromConfig creates zones based on channel UV coordinates
// Falls back to auto-split if UV coordinates are not configured
func createZonesFromConfig(cfg *config.Config) []color.Zone {
	zones := make([]color.Zone, 0, len(cfg.Channels))
	activeChannels := 0

	// First pass: count active channels
	for _, ch := range cfg.Channels {
		if ch.Active {
			activeChannels++
		}
	}

	// Track position for auto-split fallback
	autoSplitIndex := 0

	for _, ch := range cfg.Channels {
		if !ch.Active {
			continue
		}

		var zone color.Zone

		// Check if UV coordinates are configured (non-zero values)
		hasUVConfig := (ch.UVA.X != 0 || ch.UVA.Y != 0 || ch.UVB.X != 0 || ch.UVB.Y != 0)

		if hasUVConfig {
			// Validate UV coordinates
			if err := validateUVCoordinates(&ch.UVA, &ch.UVB); err != nil {
				log.Printf("[WARN] Invalid UV coordinates for channel %d (%s): %v. Using auto-split.",
					ch.ID, ch.DeviceName, err)
				zone = createDefaultZone(autoSplitIndex, activeChannels)
			} else {
				// Use configured UV coordinates
				zone = color.Zone{
					U1:   float64(ch.UVA.X),
					V1:   float64(ch.UVA.Y),
					U2:   float64(ch.UVB.X),
					V2:   float64(ch.UVB.Y),
					Name: ch.DeviceName,
				}
				log.Printf("Zone %d (%s): UV [%.2f,%.2f] to [%.2f,%.2f]",
					ch.ID, ch.DeviceName, ch.UVA.X, ch.UVA.Y, ch.UVB.X, ch.UVB.Y)
			}
		} else {
			// No UV config - use auto-split
			zone = createDefaultZone(autoSplitIndex, activeChannels)
			log.Printf("Zone %d (%s): Auto-split [%.2f,%.2f] to [%.2f,%.2f]",
				ch.ID, ch.DeviceName, zone.U1, zone.V1, zone.U2, zone.V2)
		}

		// A zone's ID is the Hue channel its colour is streamed to. Zones
		// cover active channels only, so a zone's position is not its channel.
		zone.ID = int(ch.ID)

		zones = append(zones, zone)
		autoSplitIndex++
	}

	return zones
}

// validateUVCoordinates ensures UV coordinates are valid
func validateUVCoordinates(uvA, uvB *config.UV) error {
	// Check bounds [0.0, 1.0]
	if uvA.X < 0 || uvA.X > 1 || uvA.Y < 0 || uvA.Y > 1 {
		return fmt.Errorf("uvA out of bounds: (%.2f, %.2f)", uvA.X, uvA.Y)
	}
	if uvB.X < 0 || uvB.X > 1 || uvB.Y < 0 || uvB.Y > 1 {
		return fmt.Errorf("uvB out of bounds: (%.2f, %.2f)", uvB.X, uvB.Y)
	}

	// Ensure uvB > uvA (non-zero area)
	if uvB.X <= uvA.X || uvB.Y <= uvA.Y {
		return fmt.Errorf("uvB (%.2f,%.2f) must be greater than uvA (%.2f,%.2f)",
			uvB.X, uvB.Y, uvA.X, uvA.Y)
	}

	return nil
}

// createDefaultZone creates auto-split zone for channel at given index
func createDefaultZone(index, total int) color.Zone {
	var zone color.Zone

	switch total {
	case 1:
		// Single light - whole screen
		zone.U1, zone.V1, zone.U2, zone.V2 = 0.0, 0.0, 1.0, 1.0
	case 2:
		// Two lights - left/right split
		if index == 0 {
			zone.U1, zone.V1, zone.U2, zone.V2 = 0.0, 0.0, 0.5, 1.0
		} else {
			zone.U1, zone.V1, zone.U2, zone.V2 = 0.5, 0.0, 1.0, 1.0
		}
	case 3:
		// Three lights - left/center/right
		switch index {
		case 0:
			zone.U1, zone.V1, zone.U2, zone.V2 = 0.0, 0.0, 0.33, 1.0
		case 1:
			zone.U1, zone.V1, zone.U2, zone.V2 = 0.33, 0.0, 0.67, 1.0
		case 2:
			zone.U1, zone.V1, zone.U2, zone.V2 = 0.67, 0.0, 1.0, 1.0
		}
	default:
		// More lights - divide evenly
		step := 1.0 / float64(total)
		zone.U1 = float64(index) * step
		zone.V1 = 0.0
		zone.U2 = float64(index+1) * step
		zone.V2 = 1.0
	}

	return zone
}

// Start begins screen synchronization with the provided context
func (e *Engine) Start(ctx context.Context) error {
	_, err := e.StartSession(ctx)
	return err
}

// StartSession starts sync like Start and also returns an identifier for the
// session it started, which StopSession accepts. A caller that may need to
// undo its own start uses the pair: Start can block for as long as the portal
// dialog stays open, and by the time it returns another session can already be
// queued behind it, so stopping whatever is running would stop the wrong one.
func (e *Engine) StartSession(ctx context.Context) (uint64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return 0, ErrAlreadyRunning
	}

	// Use provided context or default to Background
	if ctx == nil {
		ctx = context.Background()
	}

	// Start screen capture
	if err := e.capturer.Start(); err != nil {
		err = fmt.Errorf("failed to start screen capture: %w", err)
		// Declining the dialog is a choice, recorded as a deliberate stop
		// would be, so nothing reopens it.
		e.lastFailure = err
		if capture.IsPermissionDenied(err) {
			e.lastFailure = nil
		}
		return 0, err
	}

	if err := e.connectStream(ctx); err != nil {
		e.capturer.Stop() // Clean up capture on connection failure
		e.lastFailure = fmt.Errorf("failed to connect to Entertainment API: %w", err)
		return 0, e.lastFailure
	}

	gen := e.launchLoopLocked(ctx)
	log.Printf("Screen sync started at %d FPS", e.fps.Load())
	return gen, nil
}

// connectStream activates the Entertainment Area and opens the stream to it.
//
// An activation that reaches the bridge while it is still tearing down the
// previous stream is answered 200 and then discarded: the configuration goes
// inactive and UDP 2100 closes, which is what a start issued straight after a
// stop hits. Nothing reopens the port but another activation, so a refused
// handshake is retried against a fresh one rather than against the dial alone.
func (e *Engine) connectStream(ctx context.Context) error {
	deadline := time.Now().Add(streamConnectTimeout)
	for {
		activateErr := e.activateEntertainmentArea()
		if activateErr != nil {
			log.Printf("[WARN] Failed to activate Entertainment Area: %v", activateErr)
			log.Println("   Attempting connection anyway...")
		}

		// The bridge opens the port shortly after accepting the activation.
		time.Sleep(streamActivationDelay)

		err := e.client.Connect()
		if err == nil {
			return nil
		}
		// Only an activation the bridge accepted reopens the port, so
		// retrying one it never answered just buries why it failed.
		if activateErr != nil {
			return fmt.Errorf("%w (entertainment area not activated: %v)", err, activateErr)
		}
		if !errors.Is(err, syscall.ECONNREFUSED) || time.Now().After(deadline) {
			return err
		}

		log.Printf("[WARN] Bridge refused the stream, reactivating: %v", err)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(streamRetryInterval):
		}
	}
}

// launchLoopLocked marks a new session running and starts its sync loop.
// Called with e.mu held.
func (e *Engine) launchLoopLocked(ctx context.Context) uint64 {
	syncCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.running = true
	e.generation++
	e.lastFailure = nil
	e.metrics.reset(time.Now())

	e.syncLoopWg.Add(1)
	go e.syncLoop(syncCtx, e.generation)
	return e.generation
}

// Stop halts screen synchronization
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Cleared even when nothing is running: after a failed start or a session
	// capture ended, a Stop is the caller saying to leave it stopped, and a
	// failure left in place would read as one still waiting to be retried.
	e.lastFailure = nil
	return e.stopLocked()
}

// StopSession stops sync only while the session StartSession returned gen for
// is still the one running, and is otherwise a no-op returning ErrNotRunning.
// A deliberate stop like Stop, so it clears the recorded failure too.
func (e *Engine) StopSession(gen uint64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.generation != gen || !e.running {
		return ErrNotRunning
	}
	e.lastFailure = nil
	return e.stopLocked()
}

// LastFailure reports why the last session ended or failed to start, or nil if
// sync is running or was stopped deliberately.
func (e *Engine) LastFailure() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastFailure
}

// endSession records why this loop is giving up and stops the session it
// belongs to. Gated on the generation because a loop that has decided to give
// up can be waiting on e.mu long enough for the user to stop and start again,
// and it must not mark or tear down the session that replaced it.
func (e *Engine) endSession(gen uint64, cause error) {
	e.mu.Lock()
	// Not running means a Stop got here first, between the loop deciding to
	// give up and taking the lock. That Stop cleared the failure on purpose,
	// and writing it back would have the session restarted against it.
	if e.generation != gen || !e.running {
		e.mu.Unlock()
		return
	}
	e.lastFailure = cause
	err := e.stopLocked()
	e.mu.Unlock()

	if err != nil && !errors.Is(err, ErrNotRunning) {
		log.Printf("[ERROR] Failed to stop sync after capture failure: %v", err)
	}
}

func (e *Engine) stopLocked() error {
	if !e.running {
		return ErrNotRunning
	}

	// Cancel context to stop loop
	if e.cancel != nil {
		e.cancel()
	}
	e.syncLoopWg.Wait()
	e.logFinalMetrics()

	// Stop screen capture
	e.capturer.Stop()

	// Close Entertainment API connection
	if err := e.client.Close(); err != nil {
		log.Printf("[ERROR] closing Entertainment API: %v", err)
	}

	e.running = false
	log.Println("Screen sync stopped")
	return nil
}

// IsRunning returns whether sync is active
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// SetFPS updates the target frame rate. Takes effect on the next tick of a
// running sync loop, or immediately for a subsequent Start.
//
// The capturer holds its own copy driving the frame reader loop, so it is
// updated too: leaving it stale would cap how often a new frame exists and
// make the sync loop re-read the same one at the higher rate.
func (e *Engine) SetFPS(fps int) error {
	if fps < capture.MinFPS || fps > capture.MaxFPS {
		return fmt.Errorf("fps must be between %d and %d", capture.MinFPS, capture.MaxFPS)
	}
	if e.capturer != nil {
		if err := e.capturer.SetFPS(fps); err != nil {
			return err
		}
	}
	e.fps.Store(int64(fps))
	log.Printf("FPS set to %d", fps)
	return nil
}

// syncLoop runs a session's frames and ends the session if they give up.
func (e *Engine) syncLoop(ctx context.Context, gen uint64) {
	err := e.streamFrames(ctx)
	// Before endSession, whose stop waits for this very loop.
	e.syncLoopWg.Done()
	if err != nil {
		e.endSession(gen, err)
	}
}

// streamFrames captures, extracts and streams one frame per tick until ctx is
// cancelled, which returns nil, or capture stops for good, which returns why.
func (e *Engine) streamFrames(ctx context.Context) error {
	lastFPS := int(e.fps.Load())

	interval := time.Second / time.Duration(lastFPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	drops := &dropCounter{interval: interval, lastTick: time.Now()}
	var lastDropLog, lastCaptureErrLog time.Time

	// Copied once per session rather than read per frame, where View would
	// wait out every Save's disk write.
	var subsampleWidth int
	e.config.View(func(c *config.Config) {
		subsampleWidth = c.Sync.SubsampleWidth
	})

	extractor, err := color.NewExtractor(subsampleWidth, 2.2)
	if err != nil {
		log.Printf("[ERROR] Failed to create extractor, ending sync: %v", err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			frameStart := time.Now()

			// time.Ticker only buffers one pending tick and silently drops the
			// rest when the receiver falls behind, so count drops from elapsed
			// wall-clock time against the target interval rather than draining
			// the channel (which can never observe more than one extra tick).
			if dropped := drops.observe(frameStart); dropped > 0 {
				e.metrics.mu.Lock()
				e.metrics.framesDropped += uint64(dropped)
				e.metrics.mu.Unlock()

				// Throttled: a machine too slow to keep up drops on every
				// frame, so an unthrottled line here costs it the frame rate
				// in log writes when it can least afford them.
				// logPerformanceMetrics carries the totals every 5s.
				if time.Since(lastDropLog) >= dropLogInterval {
					log.Printf("[WARN] Frame skip: dropped %d frames (processing too slow for %d FPS)", dropped, lastFPS)
					lastDropLog = time.Now()
				}
			}

			// Applied before the error paths below, which continue: a settings
			// change made while capture is failing must still take effect.
			if currentFPS := int(e.fps.Load()); currentFPS != lastFPS {
				interval = time.Second / time.Duration(currentFPS)
				ticker.Reset(interval)
				lastFPS = currentFPS
				drops.setInterval(interval, frameStart)
			}

			// Capture phase
			captureStart := time.Now()
			frame, err := e.capturer.CaptureFrame()
			captureTime := time.Since(captureStart)

			if err != nil {
				// Capture is over, so every later tick would stream the same
				// frozen frame while IsRunning kept reporting a live session.
				if errors.Is(err, capture.ErrCaptureStopped) {
					log.Printf("[ERROR] Screen capture stopped, ending sync: %v", err)
					return err
				}
				// Throttled: this fires on every tick while capture is down,
				// which at 60 FPS buries the journal before the reader loop's
				// own deadline is even reached.
				if time.Since(lastCaptureErrLog) >= captureErrLogInterval {
					log.Printf("[WARN] Capture error: %v", err)
					lastCaptureErrLog = time.Now()
				}
				continue
			}

			// Extract phase
			extractStart := time.Now()
			zoneColors, err := extractor.ExtractColors(frame, e.zones)
			extractTime := time.Since(extractStart)

			// Return frame buffer to pool immediately
			capture.PutImageBuffer(frame)

			if err != nil {
				log.Printf("[WARN] Color extraction error: %v", err)
				continue
			}

			// Convert colors for streaming
			channelColors := make([]entertainment.ChannelColor, len(zoneColors))
			for i, zc := range zoneColors {
				channelColors[i] = entertainment.ChannelColor{
					ChannelID: zc.ZoneID,
					R:         uint16(zc.R) * entertainment.Color8To16Multiplier,
					G:         uint16(zc.G) * entertainment.Color8To16Multiplier,
					B:         uint16(zc.B) * entertainment.Color8To16Multiplier,
				}
			}

			// Stream phase
			streamStart := time.Now()
			if err := e.client.StreamColors(channelColors); err != nil {
				log.Printf("[WARN] Streaming error: %v", err)
			}
			streamTime := time.Since(streamStart)

			// Update metrics
			frameTime := time.Since(frameStart)
			e.updateMetrics(frameTime, captureTime, extractTime, streamTime)

			// Log metrics every 5 seconds
			e.metrics.mu.RLock()
			timeSinceLog := time.Since(e.metrics.lastLogTime)
			e.metrics.mu.RUnlock()

			if timeSinceLog >= 5*time.Second {
				e.logPerformanceMetrics()
			}

		}
	}
}

// stallGap is where a gap stops looking like slow processing and starts looking
// like the clock jumping. A suspend/resume would otherwise count every interval
// it slept through and pin the drop rate near 100% for the rest of the session,
// since framesDropped never resets. Absolute rather than a multiple of the
// interval: a multiple scales with the target rate, so the same real stall
// would be counted at 10 FPS and discarded at 60.
const stallGap = 2 * time.Second

// dropCounter tracks the tick bookkeeping behind the frame-drop metric.
type dropCounter struct {
	interval time.Duration
	lastTick time.Time
	// Time that has elapsed but not yet added up to a whole interval. Without
	// carrying it, a frame that consistently takes 1.9 intervals truncates to
	// 1 and reports no drops at all, which is the mild overload the warning
	// exists for.
	carry time.Duration
}

// observe records a processed frame at now and returns the ticks missed since
// the previous one.
func (d *dropCounter) observe(now time.Time) int {
	elapsed := now.Sub(d.lastTick)
	d.lastTick = now

	if d.interval <= 0 {
		return 0
	}
	if elapsed >= stallGap {
		d.carry = 0
		return 0
	}

	total := d.carry + elapsed
	ticks := int(total / d.interval)
	d.carry = total % d.interval
	if ticks <= 1 {
		return 0
	}
	return ticks - 1
}

// setInterval adopts a new target interval. The span since the last frame was
// measured against the old one, so it is rebased rather than carried over:
// dividing it by a shorter interval would report drops that never happened.
func (d *dropCounter) setInterval(interval time.Duration, now time.Time) {
	d.interval = interval
	d.lastTick = now
	d.carry = 0
}

// activateEntertainmentArea activates the Entertainment Area on the bridge
func (e *Engine) activateEntertainmentArea() error {
	var url, appKey string
	e.config.View(func(c *config.Config) {
		url = fmt.Sprintf("https://%s/clip/v2/resource/entertainment_configuration/%s",
			c.Bridge, c.EntertainmentConfigurationID)
		appKey = c.Key
	})

	// Create request body
	body := []byte(`{"action":"start"}`)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("hue-application-key", appKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to activate: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Failed to close response body: %v", err)
		}
	}()

	// Check response
	var result struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("bridge returned errors: %s", result.Errors)
	}

	log.Printf("Entertainment Area activated")
	return nil
}

// updateRestoreToken saves a new restore token to config
// Called automatically when a new token is received from the portal
func (e *Engine) updateRestoreToken(newToken string) {
	err := e.config.Update(func(c *config.Config) {
		c.Sync.RestoreToken = newToken
	}, nil)
	if err != nil {
		log.Printf("[WARN] Failed to save restore token: %v", err)
	} else {
		log.Printf("[INFO] Screen share permission saved (no dialog next time)")
	}
}

// updateMetrics updates performance metrics with frame timing data
func (e *Engine) updateMetrics(frameTime, captureTime, extractTime, streamTime time.Duration) {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	e.metrics.frameCount++
	e.metrics.totalFrameTime += frameTime
	e.metrics.totalCaptureTime += captureTime
	e.metrics.totalExtractTime += extractTime
	e.metrics.totalStreamTime += streamTime

	// Update histogram (clamp to 0-99ms)
	bucketMs := int(frameTime.Milliseconds())
	if bucketMs >= len(e.metrics.frameTimes) {
		bucketMs = len(e.metrics.frameTimes) - 1
	}
	e.metrics.frameTimes[bucketMs]++
}

// logPerformanceMetrics logs current performance metrics
func (e *Engine) logPerformanceMetrics() {
	targetFPS := e.fps.Load()

	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	if e.metrics.frameCount == 0 {
		return // No frames yet
	}

	elapsed := time.Since(e.metrics.startTime)
	actualFPS := float64(e.metrics.frameCount) / elapsed.Seconds()
	avgFrameTime := e.metrics.totalFrameTime / time.Duration(e.metrics.frameCount)
	avgCaptureTime := e.metrics.totalCaptureTime / time.Duration(e.metrics.frameCount)
	avgExtractTime := e.metrics.totalExtractTime / time.Duration(e.metrics.frameCount)
	avgStreamTime := e.metrics.totalStreamTime / time.Duration(e.metrics.frameCount)

	// Calculate percentiles from histogram
	p50 := e.metrics.calculatePercentile(50)
	p95 := e.metrics.calculatePercentile(95)
	p99 := e.metrics.calculatePercentile(99)

	dropRate := float64(e.metrics.framesDropped) / float64(e.metrics.frameCount+e.metrics.framesDropped) * 100

	log.Printf("[INFO] Performance Metrics (%.1fs elapsed, %d frames):", elapsed.Seconds(), e.metrics.frameCount)
	log.Printf("   FPS: %.1f actual / %d target", actualFPS, targetFPS)
	log.Printf("   Frame Time: avg=%.2fms p50=%.0fms p95=%.0fms p99=%.0fms",
		avgFrameTime.Seconds()*1000, p50, p95, p99)
	log.Printf("   Pipeline: capture=%.2fms extract=%.2fms stream=%.2fms",
		avgCaptureTime.Seconds()*1000, avgExtractTime.Seconds()*1000, avgStreamTime.Seconds()*1000)
	if e.metrics.framesDropped > 0 {
		log.Printf("   [WARN] Dropped: %d frames (%.1f%% drop rate)", e.metrics.framesDropped, dropRate)
	}

	e.metrics.lastLogTime = time.Now()
}

// logFinalMetrics logs final performance summary on shutdown
func (e *Engine) logFinalMetrics() {
	e.metrics.mu.Lock()
	defer e.metrics.mu.Unlock()

	if e.metrics.frameCount == 0 {
		return
	}

	elapsed := time.Since(e.metrics.startTime)
	actualFPS := float64(e.metrics.frameCount) / elapsed.Seconds()
	totalFrames := e.metrics.frameCount + e.metrics.framesDropped
	dropRate := float64(e.metrics.framesDropped) / float64(totalFrames) * 100

	log.Printf("[INFO] Screen Sync Final Stats:")
	log.Printf("   Total Runtime: %.1fs", elapsed.Seconds())
	log.Printf("   Frames Processed: %d (%.1f FPS average)", e.metrics.frameCount, actualFPS)
	if e.metrics.framesDropped > 0 {
		log.Printf("   Frames Dropped: %d (%.1f%% of %d total)", e.metrics.framesDropped, dropRate, totalFrames)
	} else {
		log.Printf("   [INFO] Zero frame drops!")
	}
}

// calculatePercentile calculates percentile from histogram
// Returns percentile value in milliseconds
func (e *PerformanceMetrics) calculatePercentile(percentile float64) float64 {
	totalSamples := uint64(0)
	for _, count := range e.frameTimes {
		totalSamples += uint64(count)
	}

	if totalSamples == 0 {
		return 0
	}

	targetCount := uint64(float64(totalSamples) * percentile / 100.0)
	currentCount := uint64(0)

	for ms, count := range e.frameTimes {
		currentCount += uint64(count)
		if currentCount >= targetCount {
			return float64(ms)
		}
	}

	return float64(len(e.frameTimes) - 1)
}
