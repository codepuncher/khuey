package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/sync"
)

func main() {
fmt.Println("Screen Sync Performance Test")
fmt.Println("============================")
fmt.Println()

// Load config
cfg, err := config.Load()
if err != nil {
log.Fatalf("Failed to load config: %v", err)
}

fmt.Printf("Configuration:\n")
fmt.Printf("  Bridge: %s\n", cfg.Bridge)
fmt.Printf("  Entertainment ID: %s\n", cfg.EntertainmentConfigurationID)
fmt.Printf("  Channels: %d\n", len(cfg.Channels))
fmt.Printf("  Target FPS: %d\n", cfg.Sync.FPS)
fmt.Println()

// Create engine
engine, err := sync.NewEngine(cfg)
if err != nil {
log.Fatalf("Failed to create engine: %v", err)
}

// Start sync
fmt.Println("Starting sync engine...")
if err := engine.Start(context.Background()); err != nil {
	log.Fatalf("Failed to start: %v", err)
}
defer engine.Stop()

fmt.Println("✅ Sync started successfully!")
fmt.Println()

// Monitor for 30 seconds
fmt.Println("Running for 30 seconds...")
fmt.Println("Monitor your lights for color changes")
fmt.Println()

ticker := time.NewTicker(5 * time.Second)
defer ticker.Stop()

timeout := time.After(30 * time.Second)

for {
select {
case <-ticker.C:
if engine.IsRunning() {
fmt.Println("✅ Sync still running...")
} else {
fmt.Println("❌ Sync stopped unexpectedly!")
return
}
case <-timeout:
fmt.Println()
fmt.Println("✅ Test completed successfully!")
fmt.Println()
fmt.Println("Results:")
fmt.Printf("  - Sync ran for 30 seconds without errors\n")
fmt.Printf("  - Target FPS: %d\n", cfg.Sync.FPS)
fmt.Printf("  - Channels: %d\n", len(cfg.Channels))
return
}
}
}
