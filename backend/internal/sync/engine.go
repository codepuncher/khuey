package sync

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/codepuncher/khuey/internal/capture"
	"github.com/codepuncher/khuey/internal/color"
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

	fps   int
	zones []color.Zone
}

// NewEngine creates a new sync engine
func NewEngine(cfg *config.Config) (*Engine, error) {
	// Create screen capture - optimized screenshot method with downsampling
	// Capturing at 640x360 is ~6x less pixels than 1920x1080, much faster
	capturer, err := capture.NewScreenCapture(capture.Config{
		FPS:            30,    // Default 30 FPS
		Monitor:        -1,    // All monitors
		UseMockFrames:  false, // Disable mock frames
		UseScreenshot:  true,  // Enable real screenshot capture
		ScreenshotTool: "",    // Auto-detect (spectacle, grim, or import)
		CaptureWidth:   640,   // Downsample to 640px width (faster)
		CaptureHeight:  360,   // Downsample to 360px height (faster)
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

	// Define zones based on channels (simple left/center/right for 3 lights)
	zones := make([]color.Zone, len(cfg.Channels))
	switch len(cfg.Channels) {
	case 1:
		// Single light - whole screen
		zones[0] = color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0}
	case 2:
		// Two lights - left/right split
		zones[0] = color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0}
		zones[1] = color.Zone{ID: 1, U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0}
	case 3:
		// Three lights - left/center/right
		zones[0] = color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0}
		zones[1] = color.Zone{ID: 1, U1: 0.33, V1: 0.0, U2: 0.67, V2: 1.0}
		zones[2] = color.Zone{ID: 2, U1: 0.67, V1: 0.0, U2: 1.0, V2: 1.0}
	default:
		// More lights - divide evenly
		step := 1.0 / float64(len(cfg.Channels))
		for i := range zones {
			zones[i] = color.Zone{
				ID: i,
				U1: float64(i) * step,
				V1: 0.0,
				U2: float64(i+1) * step,
				V2: 1.0,
			}
		}
	}

	return &Engine{
		config:   cfg,
		capturer: capturer,
		client:   client,
		running:  false,
		fps:      30, // Default 30 FPS
		zones:    zones,
	}, nil
}

// Start begins screen synchronization
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("sync already running")
	}

	// Activate Entertainment Area first
	if err := e.activateEntertainmentArea(); err != nil {
		log.Printf("⚠️  Warning: Failed to activate Entertainment Area: %v", err)
		log.Println("   Attempting connection anyway...")
	}

	// Give bridge a moment to activate
	time.Sleep(500 * time.Millisecond)

	// Start screen capture
	if err := e.capturer.Start(); err != nil {
		return fmt.Errorf("failed to start screen capture: %w", err)
	}

	// Connect to Entertainment API
	if err := e.client.Connect(); err != nil {
		e.capturer.Stop() // Clean up capture on connection failure
		return fmt.Errorf("failed to connect to Entertainment API: %w", err)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.running = true

	// Start sync loop
	go e.syncLoop(ctx)

	log.Printf("✅ Screen sync started at %d FPS", e.fps)
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
	log.Println("✅ Screen sync stopped")
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

	// Create extractor with subsample width 64px and gamma 2.2
	extractor, err := color.NewExtractor(64, 2.2)
	if err != nil {
		log.Printf("❌ Failed to create extractor: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Capture frame (currently mock gradient)
			frame, err := e.capturer.CaptureFrame()
			if err != nil {
				log.Printf("⚠️  Capture error: %v", err)
				continue
			}

			// Extract colors from zones
			zoneColors, err := extractor.ExtractColors(frame, e.zones)
			if err != nil {
				log.Printf("⚠️  Color extraction error: %v", err)
				continue
			}

			// Convert to ChannelColor format with proper channel IDs
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
					R:         uint16(zc.R) * 257, // 257 = 65535 / 255
					G:         uint16(zc.G) * 257,
					B:         uint16(zc.B) * 257,
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

	// Create HTTP client with TLS skip (Hue bridge uses self-signed cert)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 5 * time.Second,
	}

	// Create PUT request with body
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("hue-application-key", e.config.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to activate: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for errors
	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("bridge returned errors: %v", errors)
	}

	log.Println("✅ Entertainment Area activated")
	return nil
}
