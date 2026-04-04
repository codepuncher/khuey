// Package sync provides screen synchronization with Hue Entertainment API.
// It captures screen content, extracts colors from zones, and streams them to Hue lights in real-time.
package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/color"
	"github.com/codepuncher/khuey/internal/common"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/entertainment"
)

// Engine manages screen synchronization
type Engine struct {
	config   *config.Config
	capturer *capture.ScreenCapture
	client   *entertainment.Client

	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc

	fps        int
	zones      []color.Zone
	httpClient *http.Client // PERF-004: Reusable HTTP client
}

// NewEngine creates a new sync engine
func NewEngine(cfg *config.Config) (*Engine, error) {
	// Create screen capture - native PipeWire capture with CGo
	capturer, err := capture.NewScreenCapture(capture.Config{
		FPS:              30,    // Default 30 FPS
		Monitor:          -1,    // All monitors
		UseMockFrames:    false, // Disable mock frames
		UseNativeCapture: true,  // Use native CGo PipeWire capture
		UseScreenshot:    false, // Disable screenshot fallback
		CaptureWidth:     0,     // Full resolution (native capture is fast)
		CaptureHeight:    0,     // Full resolution (native capture is fast)
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
		ChannelCount:    len(cfg.Channels),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create entertainment client: %w", err)
	}

	// Create zones from config channels
	zones := createZonesFromConfig(cfg)

	return &Engine{
		config:     cfg,
		capturer:   capturer,
		client:     client,
		running:    false,
		fps:        30, // Default 30 FPS
		zones:      zones,
		httpClient: common.NewHueHTTPClient(), // PERF-004: Reuse HTTP client
	}, nil
}

// createZonesFromConfig creates zones based on channel UV coordinates
// Falls back to auto-split if UV coordinates are not configured
func createZonesFromConfig(cfg *config.Config) []color.Zone {
	// PERF-007: Pre-allocate with maximum possible capacity (all channels)
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
				log.Printf("⚠️  Warning: Invalid UV coordinates for channel %d (%s): %v. Using auto-split.",
					ch.ID, ch.DeviceName, err)
				zone = createDefaultZone(autoSplitIndex, activeChannels)
			} else {
				// Use configured UV coordinates
				zone = color.Zone{
					ID:   int(ch.ID),
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
// Uses the same logic as the previous hardcoded implementation
func createDefaultZone(index, total int) color.Zone {
	zone := color.Zone{ID: index}

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
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("sync already running")
	}

	// Use provided context or default to Background
	if ctx == nil {
		ctx = context.Background()
	}

	// Activate Entertainment Area first
	if err := e.activateEntertainmentArea(); err != nil {
		log.Printf("⚠️  Warning: Failed to activate Entertainment Area: %v", err)
		log.Println("   Attempting connection anyway...")
	}

	// PERF-002: Reduced sleep time from 500ms to 100ms
	// Bridge activation is typically fast, shorter wait improves startup performance
	time.Sleep(100 * time.Millisecond)

	// Start screen capture
	if err := e.capturer.Start(); err != nil {
		return fmt.Errorf("failed to start screen capture: %w", err)
	}

	// Connect to Entertainment API
	if err := e.client.Connect(); err != nil {
		e.capturer.Stop() // Clean up capture on connection failure
		return fmt.Errorf("failed to connect to Entertainment API: %w", err)
	}

	// Create child context for cancellation
	syncCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.running = true

	// Start sync loop in goroutine
	go e.syncLoop(syncCtx)

	log.Printf("Screen sync started at %d FPS", e.fps)
	return nil
}

// Stop halts screen synchronization
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("sync not running")
	}

	// Cancel context to stop loop
	if e.cancel != nil {
		e.cancel()
	}

	// Stop screen capture
	e.capturer.Stop()

	// Close Entertainment API connection
	if err := e.client.Close(); err != nil {
		log.Printf("⚠️  Error closing Entertainment API: %v", err)
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

// SetFPS updates the target frame rate
func (e *Engine) SetFPS(fps int) error {
	if fps < 1 || fps > 60 {
		return fmt.Errorf("fps must be 1-60")
	}
	e.mu.Lock()
	e.fps = fps
	e.mu.Unlock()
	log.Printf("FPS set to %d", fps)
	return nil
}

// syncLoop is the main synchronization loop
func (e *Engine) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second / time.Duration(e.fps))
	defer ticker.Stop()

	// Create extractor with configured subsample width and gamma 2.2
	extractor, err := color.NewExtractor(e.config.Sync.SubsampleWidth, 2.2)
	if err != nil {
		log.Printf("❌ Failed to create extractor: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Non-blocking ticker drain to handle frame drops gracefully
			// If processing takes longer than frame interval, skip accumulated ticks
			drained := 0
		drainLoop:
			for {
				select {
				case <-ticker.C:
					drained++
				default:
					break drainLoop
				}
			}
			if drained > 0 {
				log.Printf("⚠️  Frame skip: dropped %d frames (processing too slow for %d FPS)", drained, e.fps)
			}

			// Capture frame (currently mock gradient)
			frame, err := e.capturer.CaptureFrame()
			if err != nil {
				log.Printf("⚠️  Capture error: %v", err)
				continue
			}
			// RES-007: Return frame buffer to pool after processing
			defer capture.PutImageBuffer(frame)

			// Extract colors from zones
			zoneColors, err := extractor.ExtractColors(frame, e.zones)
			if err != nil {
				log.Printf("⚠️  Color extraction error: %v", err)
				continue
			}

			// Convert to ChannelColor format with proper channel IDs
			// PERF-007: Pre-allocate with exact capacity (hot path optimization)
			channelColors := make([]entertainment.ChannelColor, len(zoneColors))
			for i, zc := range zoneColors {
				// Get channel ID from config
				channelID := 0
				if i < len(e.config.Channels) {
					channelID = int(e.config.Channels[i].ID)
				}

				// Convert 8-bit RGB to 16-bit (0-255 → 0-65535)
				channelColors[i] = entertainment.ChannelColor{
					ChannelID: channelID,
					R:         uint16(zc.R) * entertainment.Color8To16Multiplier,
					G:         uint16(zc.G) * entertainment.Color8To16Multiplier,
					B:         uint16(zc.B) * entertainment.Color8To16Multiplier,
				}
			}

			// Stream to Entertainment API
			if err := e.client.StreamColors(channelColors); err != nil {
				log.Printf("⚠️  Streaming error: %v", err)
				// Don't stop on errors, just log and continue
			}

			// Update ticker if FPS changed
			e.mu.RLock()
			currentFPS := e.fps
			e.mu.RUnlock()

			newInterval := time.Second / time.Duration(currentFPS)
			if ticker.C != nil {
				ticker.Reset(newInterval)
			}
		}
	}
}

// activateEntertainmentArea activates the Entertainment Area on the bridge
func (e *Engine) activateEntertainmentArea() error {
	url := fmt.Sprintf("https://%s/clip/v2/resource/entertainment_configuration/%s",
		e.config.Bridge, e.config.EntertainmentConfigurationID)

	// Create request body
	body := []byte(`{"action":"start"}`)

	// PERF-004: Use reusable HTTP client instead of creating new one
	// Create PUT request with body
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("hue-application-key", e.config.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to activate: %w", err)
	}
	defer func() {
		// QUAL-003: Check error on defer Close()
		if err := resp.Body.Close(); err != nil {
			log.Printf("⚠️  Failed to close response body: %v", err)
		}
	}()

	// Check response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for errors
	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("bridge returned errors: %v", errors)
	}

	log.Printf("Entertainment Area activated")
	return nil
}
