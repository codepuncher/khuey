// Integration tests for KDE Hue Control
// Run with: go test -tags=integration -v
//go:build integration

package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
	"github.com/codepuncher/khuey/internal/testutil"
)

// setupTestClient creates a test client with TLS skip verify
func setupTestClient(t *testing.T, bridgeAddr, apiKey string) *hue.Client {
	// Set TLS skip verify for test
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	ctx := context.Background()
	client, err := hue.NewClient(ctx, bridgeAddr, apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	return client
}

// TestHueClient_GetScenes tests scene retrieval
func TestHueClient_GetScenes(t *testing.T) {
	// Setup mock bridge with TLS
	mock := testutil.NewMockBridgeTLS()
	defer mock.Close()
	mock.SetupDefaultScenario()

	bridgeAddr := mock.URL()[8:] // Remove https://
	client := setupTestClient(t, bridgeAddr, "test-api-key")

	scenes, err := client.GetScenes()
	if err != nil {
		t.Fatalf("GetScenes() error = %v", err)
	}

	if len(scenes) != 3 {
		t.Errorf("GetScenes() returned %d scenes, want 3", len(scenes))
	}

	// Verify scene data
	sceneNames := make(map[string]bool)
	for _, scene := range scenes {
		sceneNames[scene.Name] = true
	}

	expected := []string{"Relax", "Bright", "Concentrate"}
	for _, name := range expected {
		if !sceneNames[name] {
			t.Errorf("Expected scene %q not found", name)
		}
	}
}

// TestHueClient_ActivateScene tests scene activation
func TestHueClient_ActivateScene(t *testing.T) {
	mock := testutil.NewMockBridgeTLS()
	defer mock.Close()
	mock.SetupDefaultScenario()

	bridgeAddr := mock.URL()[8:]
	client := setupTestClient(t, bridgeAddr, "test-api-key")

	// Get scenes first
	scenes, err := client.GetScenes()
	if err != nil || len(scenes) == 0 {
		t.Fatal("No scenes available for testing")
	}

	// Activate first scene
	sceneID := scenes[0].ID
	err = client.ActivateScene(sceneID)
	if err != nil {
		t.Errorf("ActivateScene() error = %v", err)
	}

	// Verify request was made
	requestCount := mock.GetRequestCount()
	if requestCount < 2 { // At least GET scenes + PUT activate
		t.Errorf("Expected at least 2 requests, got %d", requestCount)
	}
}

// TestHueClient_ErrorHandling tests error scenarios
func TestHueClient_ErrorHandling(t *testing.T) {
	t.Run("InvalidSceneID", func(t *testing.T) {
		mock := testutil.NewMockBridgeTLS()
		defer mock.Close()
		mock.SetupDefaultScenario()

		bridgeAddr := mock.URL()[8:]
		client := setupTestClient(t, bridgeAddr, "test-api-key")

		err := client.ActivateScene("nonexistent-scene-id")
		// Should not panic, may or may not error depending on mock implementation
		if err != nil {
			t.Logf("ActivateScene with invalid ID returned error: %v (acceptable)", err)
		}
	})

	t.Run("BridgeUnreachable", func(t *testing.T) {
		// Create client with invalid bridge
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		badClient, err := hue.NewClient(ctxTimeout, "192.0.2.1:8080", "test-key")
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// This should timeout or error
		_, err = badClient.GetScenes()
		if err == nil {
			t.Error("Expected error for unreachable bridge, got nil")
		} else {
			t.Logf("Got expected error: %v", err)
		}
	})
}

// TestConfig_Integration tests config loading and validation
func TestConfig_DefaultAndConfiguredStates(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		cfg := config.DefaultConfig()

		if cfg.IsConfigured() {
			t.Error("Default config should not be configured")
		}

		if cfg.HasEntertainmentConfig() {
			t.Error("Default config should not have entertainment config")
		}

		// Verify default values
		if cfg.Sync.FPS != 30 {
			t.Errorf("Default FPS = %d, want 30", cfg.Sync.FPS)
		}

		if cfg.Sync.SubsampleWidth != 64 {
			t.Errorf("Default SubsampleWidth = %d, want 64", cfg.Sync.SubsampleWidth)
		}
	})

	t.Run("ConfiguredState", func(t *testing.T) {
		cfg := &config.Config{
			Bridge: "192.168.1.100",
			Key:    "test-api-key",
		}

		if !cfg.IsConfigured() {
			t.Error("Config with bridge and key should be configured")
		}

		if cfg.HasEntertainmentConfig() {
			t.Error("Config without entertainment fields should not have entertainment config")
		}

		cfg.ClientKey = "test-client-key"
		cfg.EntertainmentConfigurationID = "test-ent-id"

		if !cfg.HasEntertainmentConfig() {
			t.Error("Config with all fields should have entertainment config")
		}
	})
}

// TestConcurrency_GetScenes tests concurrent scene retrieval
func TestConcurrency_GetScenes(t *testing.T) {
	mock := testutil.NewMockBridgeTLS()
	defer mock.Close()
	mock.SetupDefaultScenario()

	bridgeAddr := mock.URL()[8:]
	client := setupTestClient(t, bridgeAddr, "test-api-key")

	// Launch 10 concurrent GetScenes requests
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := client.GetScenes()
			errs <- err
		}()
	}

	// Collect results
	for i := 0; i < 10; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Concurrent GetScenes #%d error: %v", i, err)
		}
	}
}

// TestConcurrency_SceneActivation tests concurrent scene activation
func TestConcurrency_SceneActivation(t *testing.T) {
	mock := testutil.NewMockBridgeTLS()
	defer mock.Close()
	mock.SetupDefaultScenario()

	bridgeAddr := mock.URL()[8:]
	client := setupTestClient(t, bridgeAddr, "test-api-key")

	scenes, err := client.GetScenes()
	if err != nil || len(scenes) == 0 {
		t.Skip("No scenes available for concurrency test")
	}

	// Activate same scene concurrently
	errs := make(chan error, 5)
	sceneID := scenes[0].ID
	for i := 0; i < 5; i++ {
		go func() {
			err := client.ActivateScene(sceneID)
			errs <- err
		}()
	}

	// Collect results
	for i := 0; i < 5; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Concurrent ActivateScene #%d error: %v", i, err)
		}
	}
}

// TestMockBridge_RequestLogging tests request logging feature
func TestMockBridge_RequestLogging(t *testing.T) {
	mock := testutil.NewMockBridgeTLS()
	defer mock.Close()
	mock.SetupDefaultScenario()

	bridgeAddr := mock.URL()[8:]
	client := setupTestClient(t, bridgeAddr, "test-api-key")

	// Clear existing logs
	mock.ClearRequestLog()

	// Make requests
	client.GetScenes()

	// Verify logging (GetScenes makes 3 requests internally - scenes, rooms, zones)
	count := mock.GetRequestCount()
	if count < 1 {
		t.Errorf("Expected at least 1 logged request, got %d", count)
	}
	t.Logf("Logged %d requests from GetScenes call", count)
}

// TestMockBridge_Setup tests the default scenario setup
func TestMockBridge_Setup(t *testing.T) {
	mock := testutil.NewMockBridgeTLS()
	defer mock.Close()
	mock.SetupDefaultScenario()

	if len(mock.Scenes) != 3 {
		t.Errorf("Expected 3 scenes, got %d", len(mock.Scenes))
	}

	if len(mock.Lights) != 3 {
		t.Errorf("Expected 3 lights, got %d", len(mock.Lights))
	}

	if len(mock.Rooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(mock.Rooms))
	}

	if len(mock.Zones) != 1 {
		t.Errorf("Expected 1 zone, got %d", len(mock.Zones))
	}
}
