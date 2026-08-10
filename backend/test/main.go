package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/hue"
)

func main() {
	testConfig := flag.Bool("config", false, "Test configuration loading")
	testHue := flag.Bool("hue", false, "Test Hue client connection")
	bridgeAddr := flag.String("bridge", "", "Bridge address")
	apiKey := flag.String("key", "", "API key")

	flag.Parse()

	if !*testConfig && !*testHue {
		fmt.Println("Backend Component Tester")
		fmt.Println("\nUsage:")
		fmt.Println("  -config          Test configuration")
		fmt.Println("  -hue             Test Hue client")
		fmt.Println("  -bridge <addr>   Bridge address")
		fmt.Println("  -key <key>       API key")
		fmt.Println("\nExamples:")
		fmt.Println("  go run test/main.go -config")
		fmt.Println("  go run test/main.go -hue -bridge 192.168.1.100 -key YOUR_KEY")
		os.Exit(0)
	}

	if *testConfig {
		fmt.Println("=== Testing Configuration ===")
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed: %v", err)
		}

		fmt.Printf("Config loaded\n")
		fmt.Printf("  Bridge: %s\n", cfg.Bridge)
		fmt.Printf("  Configured: %v\n", cfg.IsConfigured())
		fmt.Printf("  Sync FPS: %d\n", cfg.Sync.FPS)
	}

	if *testHue {
		fmt.Println("=== Testing Hue Client ===")
		addr, key := *bridgeAddr, *apiKey

		if addr == "" || key == "" {
			cfg, _ := config.Load()
			if cfg != nil && cfg.IsConfigured() {
				addr, key = cfg.Bridge, cfg.Key
			}
		}

		if addr == "" {
			log.Fatal("Bridge address required")
		}
		if key == "" {
			log.Fatal("API key required")
		}

		client, err := hue.NewClient(context.Background(), addr, key)
		if err != nil {
			log.Fatalf("Failed: %v", err)
		}

		fmt.Print("Ping... ")
		if err := client.Ping(); err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		fmt.Println()

		fmt.Print("Scenes... ")
		scenes, err := client.GetScenes()
		if err != nil {
			fmt.Printf("%v\n", err)
		} else {
			fmt.Printf("%d scene(s)\n", len(scenes))
			for i, s := range scenes {
				if i < 3 {
					fmt.Printf("  - %s\n", s.Name)
				}
			}
			if len(scenes) > 3 {
				fmt.Printf("  ... and %d more\n", len(scenes)-3)
			}
		}
	}
}
