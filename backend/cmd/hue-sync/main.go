package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/dbus"
	"github.com/codepuncher/khuey/internal/hue"
)

// Release builds set this with -ldflags "-X main.version=...".
var version = "dev"

// versionString returns version, or for an unstamped build, "dev" plus the
// commit Go embeds when building inside a git checkout.
func versionString() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	var revision string
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified {
		revision += "-dirty"
	}
	return version + "-" + revision
}

func printUsage(w io.Writer, flags *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: %s [--help] [--version]\n\n", flags.Name())
	fmt.Fprintln(w, "Runs the khuey backend on the DBus session bus as org.kde.plasma.hue.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	flags.SetOutput(w)
	flags.PrintDefaults()
	flags.SetOutput(io.Discard)
}

// parseArgs exits for --help, --version and invalid input so none of them
// load the config or touch the bridge or DBus.
func parseArgs() {
	flags := flag.NewFlagSet("hue-sync", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	showVersion := flags.Bool("version", false, "print the version and exit")

	err := flags.Parse(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		printUsage(os.Stdout, flags)
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", flags.Name(), err)
		printUsage(os.Stderr, flags)
		os.Exit(2)
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "%s: unexpected argument %q\n", flags.Name(), flags.Arg(0))
		printUsage(os.Stderr, flags)
		os.Exit(2)
	}
	if *showVersion {
		fmt.Printf("%s %s\n", flags.Name(), versionString())
		os.Exit(0)
	}
}

func main() {
	parseArgs()

	logVersion := versionString()
	if version != "dev" {
		logVersion = "v" + logVersion
	}
	log.Printf("KDE Hue Control backend %s starting...", logVersion)

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
		log.Printf("Or create %s manually", config.File())
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

	// Initialize gaming mode if enabled in config
	dbusService.InitGamingMode()

	// Backgrounded: the bridge may still be slow to respond this early in a
	// login, and that shouldn't hold up backend startup finishing.
	go func() {
		if err := dbusService.ActivateStartupScene(); err != nil {
			log.Printf("WARNING: Failed to activate startup scene: %v", err)
		}
	}()

	log.Println("Backend initialized successfully")
	log.Println("DBus service running at: org.kde.plasma.hue")
	log.Println()
	log.Println("The tray application should auto-start and appear in your system tray.")
	log.Println("If not running, start it manually: ./trayapp/hue-tray")
	log.Println()
	log.Printf("Configuration: %s", config.File())
	log.Println("Run 'openhue setup' if not configured yet")
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
