package main

import (
"fmt"
"log"
"time"

"github.com/codepuncher/khuey/internal/config"
"github.com/codepuncher/khuey/internal/sync"
)

func main() {
fmt.Println("Screen Sync Engine Test")
fmt.Println("=======================")
fmt.Println()

// Load config
fmt.Println("📄 Loading config...")
cfg, err := config.Load()
if err != nil {
log.Fatalf("Failed to load config: %v", err)
}
fmt.Printf("✅ Config loaded - Bridge: %s\n", cfg.Bridge)
fmt.Printf("   Entertainment ID: %s\n", cfg.EntertainmentConfigurationID)
fmt.Printf("   Channels: %d\n", len(cfg.Channels))
fmt.Println()

// Create sync engine
fmt.Println("🎮 Creating sync engine...")
engine := sync.NewEngine(cfg)
fmt.Println("✅ Engine created")
fmt.Println()

// Start sync
fmt.Println("🚀 Starting screen sync...")
if err := engine.Start(); err != nil {
log.Fatalf("Failed to start sync: %v", err)
}
fmt.Println("✅ Sync started!")
fmt.Println()

// Check status
if engine.IsRunning() {
fmt.Println("✅ Sync engine is running")
} else {
fmt.Println("❌ Sync engine is NOT running")
}
fmt.Println()

// Run for 10 seconds
fmt.Println("⏱️  Running sync for 10 seconds...")
fmt.Println("   Watch your Hue lights - they should change colors!")
time.Sleep(10 * time.Second)

// Stop sync
fmt.Println()
fmt.Println("🛑 Stopping sync...")
engine.Stop()
fmt.Println("✅ Sync stopped")
fmt.Println()

// Check status
if !engine.IsRunning() {
fmt.Println("✅ Sync engine is stopped")
} else {
fmt.Println("❌ Sync engine is still running")
}

fmt.Println()
fmt.Println("✅ Test completed successfully!")
}
