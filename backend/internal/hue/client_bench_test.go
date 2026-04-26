// Benchmark tests for Hue client operations
// Run with: go test -bench=. -benchmem
package hue

import (
	"context"
	"testing"
)

// BenchmarkNewClient benchmarks client creation
func BenchmarkNewClient(b *testing.B) {
	ctx := context.Background()
	bridgeAddr := "192.168.1.100"
	apiKey := "test-api-key-1234567890abcdefghijklmnopqrstuvwxyz"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewClient(ctx, bridgeAddr, apiKey)
		if err != nil {
			b.Fatalf("NewClient() failed: %v", err)
		}
	}
}

// BenchmarkConnectionStatus benchmarks getting connection status
func BenchmarkConnectionStatus(b *testing.B) {
	ctx := context.Background()
	client, err := NewClient(ctx, "192.168.1.100", "test-key")
	if err != nil {
		b.Fatalf("NewClient() failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetConnectionStatus()
	}
}

// BenchmarkSceneComparison benchmarks scene comparison for sorting
func BenchmarkSceneComparison(b *testing.B) {
	scenes := []Scene{
		{ID: "1", Name: "Bright", Room: "room1", RoomName: "Living Room"},
		{ID: "2", Name: "Relax", Room: "room1", RoomName: "Living Room"},
		{ID: "3", Name: "Concentrate", Room: "room2", RoomName: "Office"},
		{ID: "4", Name: "Energize", Room: "room2", RoomName: "Office"},
		{ID: "5", Name: "Dimmed", Room: "room3", RoomName: "Bedroom"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate sorting comparison
		for j := 0; j < len(scenes)-1; j++ {
			s1 := scenes[j].RoomName + " - " + scenes[j].Name
			s2 := scenes[j+1].RoomName + " - " + scenes[j+1].Name
			_ = s1 < s2
		}
	}
}
