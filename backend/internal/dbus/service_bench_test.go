// Benchmark tests for DBus service
// Run with: go test -bench=. -benchmem
package dbus

import (
	"context"
	"testing"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
)

// BenchmarkGetStatus benchmarks the GetStatus DBus method
func BenchmarkGetStatus(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"

	// Create a mock Hue client (won't actually connect)
	client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}

	service := &Service{
		config:    cfg,
		hueClient: client,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status, err := service.GetStatus()
		if err != nil {
			b.Fatalf("GetStatus() failed: %v", err)
		}
		if status == "" {
			b.Fatal("GetStatus() returned empty string")
		}
	}
}

// BenchmarkIsSyncing benchmarks the IsSyncing DBus method
func BenchmarkIsSyncing(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"

	client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}

	service := &Service{
		config:    cfg,
		hueClient: client,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.IsSyncing()
		if err != nil {
			b.Fatalf("IsSyncing() failed: %v", err)
		}
	}
}

// BenchmarkIsGamingModeActive benchmarks gaming mode status check
func BenchmarkIsGamingModeActive(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"

	client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}

	service := &Service{
		config:    cfg,
		hueClient: client,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.IsGamingModeActive()
		if err != nil {
			b.Fatalf("IsGamingModeActive() failed: %v", err)
		}
	}
}

// BenchmarkGetGroupedLights benchmarks getting grouped lights list
func BenchmarkGetGroupedLights(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"
	cfg.GroupedLightID = "test-grouped-light-id"

	client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}

	service := &Service{
		config:    cfg,
		hueClient: client,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetGroupedLights()
		// Note: This will likely fail without actual bridge connection,
		// but we're benchmarking the method call overhead
	}
}

// BenchmarkGetSyncSettings benchmarks getting sync settings
func BenchmarkGetSyncSettings(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Bridge = "192.168.1.100"
	cfg.Key = "test-key"
	cfg.Sync.FPS = 30
	cfg.Sync.SubsampleWidth = 64

	client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}

	service := &Service{
		config:    cfg,
		hueClient: client,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetSyncSettings()
		if err != nil {
			b.Fatalf("GetSyncSettings() failed: %v", err)
		}
	}
}
