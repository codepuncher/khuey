package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/dbus"
	"github.com/codepuncher/khuey/internal/hue"
)

const version = "0.1.0"

func main() {
	log.Printf("Plasma Hue Widget Backend v%s starting...", version)

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if !cfg.IsConfigured() {
		log.Println("WARNING: No bridge configuration found")
		log.Println("Please run 'openhue setup' to configure your bridge")
		log.Println("Or create ~/.openhue/config.yaml manually")
		log.Println()
		log.Println("Starting DBus service anyway (will report 'not configured' status)...")
	} else {
		log.Printf("Loaded configuration for bridge: %s", cfg.Bridge)
		if cfg.HasEntertainmentConfig() {
			log.Printf("Entertainment API configured: %s", cfg.EntertainmentConfigurationID)
		}
	}

	// Initialize Hue client (only if configured)
	var hueClient *hue.Client
	if cfg.IsConfigured() {
		hueClient, err = hue.NewClient(ctx, cfg.Bridge, cfg.Key)
		if err != nil {
			log.Printf("WARNING: Failed to create Hue client: %v", err)
			log.Println("Continuing without Hue control...")
		} else {
			log.Println("Hue client initialized")

			// Test connection
			if err := hueClient.Ping(); err != nil {
				log.Printf("WARNING: Bridge unreachable: %v", err)
			} else {
				log.Println("Bridge connection verified")
			}
		}
	}

	// Initialize DBus service
	dbusService, err := dbus.NewService(cfg, hueClient)
	if err != nil {
		log.Fatalf("Failed to create DBus service: %v", err)
	}

	if err := dbusService.Start(); err != nil {
		log.Fatalf("Failed to start DBus service: %v", err)
	}
	defer dbusService.Stop()

	log.Println("Backend initialized successfully")
	log.Println("DBus service running at: org.kde.plasma.hue")
	log.Println()
	log.Println("You can now:")
	log.Println("  1. Install the plasmoid: kpackagetool6 --install plasmoid")
	log.Println("  2. Add it to your system tray")
	log.Println("  3. Control your lights from the widget!")
	log.Println()
	log.Println("Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println()
	log.Println("Shutting down...")

	// Cancel context to stop all ongoing operations
	cancel()

	// dbusService.Stop() is called by defer above

	fmt.Println("Goodbye!")
}
