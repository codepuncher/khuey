// Profile screen sync performance
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/sync"
)

func main() {
	duration := flag.Int("duration", 15, "Duration to run profiling in seconds")
	cpuProfile := flag.String("cpuprofile", "cpu.prof", "Write CPU profile to file")
	memProfile := flag.String("memprofile", "mem.prof", "Write memory profile to file")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("[INFO] Screen Sync Performance Profiler")
	log.Printf("   Duration: %d seconds", *duration)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg.Sync.FPS = 30
	log.Printf("   FPS Target: %d", cfg.Sync.FPS)
	log.Printf("   Channels: %d", len(cfg.Channels))

	// Start CPU profiling
	f, err := os.Create(*cpuProfile)
	if err != nil {
		log.Fatalf("Failed to create CPU profile: %v", err)
	}
	defer f.Close() //nolint:errcheck
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatalf("Failed to start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()
	log.Printf("   CPU profiling: %s", *cpuProfile)

	// Create sync engine
	engine, err := sync.NewEngine(cfg)
	if err != nil {
		log.Fatalf("Failed to create sync engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*duration)*time.Second)
	defer cancel()

	// Start sync
	log.Println("\n[INFO] Starting screen sync...")
	log.Println("   NOTE: Will show permission dialog - approve to start profiling")
	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start sync: %v", err)
	}

	// Wait for duration
	<-ctx.Done()
	log.Printf("\n[INFO] Duration complete (%ds)", *duration)

	// Stop sync
	engine.Stop() //nolint:errcheck

	// Write memory profile
	mf, err := os.Create(*memProfile)
	if err != nil {
		log.Fatalf("Failed to create memory profile: %v", err)
	}
	defer mf.Close() //nolint:errcheck
	if err := pprof.WriteHeapProfile(mf); err != nil {
		log.Fatalf("Failed to write memory profile: %v", err)
	}
	log.Printf("   Memory profile: %s", *memProfile)

	log.Println("\n[INFO] Profiling complete!")
	log.Println("\nAnalyze results:")
	log.Printf("   go tool pprof -http=:8080 %s", *cpuProfile)
	log.Printf("   go tool pprof -http=:8081 %s", *memProfile)
}
