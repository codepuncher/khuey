# KDE Hue Control - Architecture Deep Dive

Comprehensive technical documentation of the KDE Hue Control system architecture, component interactions, threading model, and design decisions.

## Table of Contents

1. [System Overview](#system-overview)
2. [Component Architecture](#component-architecture)
3. [Package Structure](#package-structure)
4. [Data Flow](#data-flow)
5. [Threading & Concurrency](#threading--concurrency)
6. [State Machines](#state-machines)
7. [Screen Sync Pipeline](#screen-sync-pipeline)
8. [Entertainment API Flow](#entertainment-api-flow)
9. [Gaming Mode Detection](#gaming-mode-detection)
10. [Configuration System](#configuration-system)
11. [Error Handling](#error-handling)
12. [Security Architecture](#security-architecture)
13. [Performance Characteristics](#performance-characteristics)
14. [Design Decisions](#design-decisions)
15. [Future Architecture](#future-architecture)

---

## System Overview

KDE Hue Control is a **three-component distributed system** that bridges KDE Plasma with Philips Hue lighting through a clean DBus interface.

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                          USER SPACE                                   │
├──────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌─────────────────────┐        DBus Session Bus                     │
│  │   Qt6 Tray App      │◄──────────────────────────┐                 │
│  │   (hue-tray)        │  org.kde.plasma.hue       │                 │
│  │                     │                            │                 │
│  │  • KStatusNotifier  │                            ▼                 │
│  │  • QDBusInterface   │              ┌─────────────────────────┐    │
│  │  • KNotification    │              │   Go Backend            │    │
│  │  • Menu/Controls    │              │   (hue-sync)            │    │
│  └─────────────────────┘              │                         │    │
│                                        │  • DBus Service         │    │
│                                        │  • Config Manager       │    │
│                                        │  • Hue API Client       │    │
│                                        │  • Sync Engine          │    │
│                                        │  • Gaming Detector      │    │
│                                        └────────┬────────────────┘    │
│                                                 │                     │
└─────────────────────────────────────────────────┼─────────────────────┘
                                                  │
                                        HTTPS/UDP │
                                                  │
┌─────────────────────────────────────────────────┼─────────────────────┐
│                      NETWORK LAYER              │                     │
├─────────────────────────────────────────────────┼─────────────────────┤
│                                                 ▼                     │
│                                    ┌──────────────────────┐           │
│                                    │   Hue Bridge         │           │
│                                    │   (192.168.x.x)      │           │
│                                    │                      │           │
│                                    │  • REST API (HTTPS)  │           │
│                                    │  • Entertainment API │           │
│                                    │    (DTLS/UDP)        │           │
│                                    │  • mDNS Discovery    │           │
│                                    └──────────────────────┘           │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

**1. Tray Application (trayapp/)**
- **Language:** Qt6/C++
- **Role:** User interface and interaction
- **Responsibilities:**
  - Display system tray icon with context menu
  - Show scene list and activation controls
  - Display desktop notifications
  - Handle user input events
  - Call backend via DBus
- **Does NOT:** Directly communicate with Hue bridge or manage state

**2. Backend Service (backend/)**
- **Language:** Go
- **Role:** Business logic and system integration
- **Responsibilities:**
  - Maintain persistent connection to Hue bridge
  - Manage configuration (load/save/validate)
  - Expose DBus service interface
  - Control Entertainment API streaming
  - Capture and process screen content
  - Detect gaming activity
  - Handle errors and reconnections
- **Does NOT:** Directly render UI or handle user input

**3. Hue Bridge (External Hardware)**
- **Role:** Smart home hub
- **Responsibilities:**
  - Control Philips Hue lights
  - Provide REST API for scenes/groups
  - Provide Entertainment API for streaming
  - Maintain light state and configuration
- **Protocols:** HTTPS (REST), DTLS/UDP (Entertainment)

---

## Component Architecture

### Backend Service Structure

The Go backend is organized as a modular service with clean separation of concerns:

```
backend/
├── cmd/
│   ├── hue-sync/           # Main service entry point
│   ├── profile-sync/       # Performance profiling tool
│   ├── test-capture/       # Screen capture testing
│   ├── test-entertainment/ # Entertainment API testing
│   └── test-zones-visual/  # Zone mapping visualization
│
├── internal/
│   ├── config/            # Configuration management
│   │   ├── config.go        # Config struct and loading
│   │   ├── validation.go    # Config validation
│   │   └── config_test.go   # Unit tests
│   │
│   ├── hue/               # Hue API client
│   │   ├── client.go        # REST API wrapper
│   │   ├── scenes.go        # Scene management
│   │   ├── lights.go        # Light control
│   │   └── connection.go    # Connection management
│   │
│   ├── dbus/              # DBus service
│   │   ├── service.go       # Service implementation
│   │   ├── introspection.go # Introspection data
│   │   └── access_control.go # UID-based security
│   │
│   ├── sync/              # Screen sync engine
│   │   ├── engine.go        # Main sync loop
│   │   ├── performance.go   # Metrics and monitoring
│   │   └── circuit_breaker.go # Error recovery
│   │
│   ├── capture/           # Screen capture
│   │   ├── pipewire.go      # Native PipeWire integration
│   │   ├── portal.go        # XDG Desktop Portal
│   │   ├── buffer.go        # Reusable RGBA buffer
│   │   └── pipewire_native.c # CGo PipeWire bindings
│   │
│   ├── entertainment/     # Entertainment API
│   │   ├── client.go        # DTLS streaming client
│   │   ├── protocol.go      # HueStream v2 protocol
│   │   └── activation.go    # Entertainment area activation
│   │
│   ├── color/             # Color processing
│   │   ├── extractor.go     # UV-based color extraction
│   │   ├── gamma.go         # Gamma correction
│   │   └── sampling.go      # Stride-based sampling
│   │
│   ├── gaming/            # Gaming mode detection
│   │   ├── detector.go      # Game process detection
│   │   ├── cachyos.go       # CachyOS game database
│   │   └── detection_methods.go # Multiple detection strategies
│   │
│   ├── common/            # Common utilities
│   │   └── validation.go    # Input validation
│   │
│   └── testutil/          # Testing utilities
│       ├── mock_bridge.go   # Mock Hue bridge
│       └── helpers.go       # Test helpers
│
└── vendor/                 # Vendored dependencies
```

### Tray Application Structure

```
trayapp/
├── main.cpp               # Entry point
│   ├── KStatusNotifierItem setup
│   ├── QDBusInterface creation
│   ├── Menu population
│   └── Signal/slot connections
│
├── CMakeLists.txt         # Build configuration
│   ├── Qt6 dependencies
│   ├── KF6 StatusNotifierItem
│   └── DBus XML generation
│
└── (Future: Additional dialogs/windows)
```

---

## Package Structure

### internal/config - Configuration Management

**Purpose:** Load, validate, and save configuration files.

**Key Types:**
```go
type Config struct {
    Version                      int               // Config format version
    Bridge                       string            // Bridge IP address
    Key                          string            // API key
    ClientKey                    string            // Entertainment API client key
    EntertainmentConfigurationID string            // Entertainment area ID
    GroupedLightID               string            // Room/zone for control
    Channels                     []ChannelConfig   // Light channel mapping
    Sync                         SyncConfig        // Screen sync settings
    GamingMode                   GamingModeConfig  // Gaming mode settings
    UI                           UIConfig          // UI preferences
    LogLevel                     string            // Logging level
    mu                           sync.Mutex        // Protects concurrent access
    v                            *viper.Viper      // Non-global viper instance
}

type ChannelConfig struct {
    ID          uint8   // Channel ID (0-255)
    Active      bool    // Enable/disable channel
    DeviceName  string  // Friendly name
    GammaFactor float32 // Gamma correction (default 2.2)
    UVA         UV      // Top-left corner (0.0-1.0)
    UVB         UV      // Bottom-right corner (0.0-1.0)
}
```

**Thread Safety:**
- Uses `sync.Mutex` for concurrent access protection
- Non-global Viper instance (prevents global state contamination)
- All exported methods are goroutine-safe

**Key Functions:**
- `Load()` - Read `config.yaml` from the directory `getConfigPath` returns (see [Config File Structure](#config-file-structure))
- `Save()` - Write config with secure permissions (0600)
- `Validate()` - Validate all fields with helpful error messages
- `IsConfigured()` - Check if basic setup is complete
- `HasEntertainmentConfig()` - Check if Entertainment API is ready

**Performance:**
- Fast path for defaults (no disk I/O)
- Config caching (loaded once, reused)
- Lazy validation (only when accessing)

---

### internal/hue - Hue API Client

**Purpose:** Wrapper around openhue-go library for REST API communication.

**Key Types:**
```go
type Client struct {
    client       *openhue.Client // Underlying openhue client
    bridgeAddr   string           // Bridge IP
    apiKey       string           // API key
    connStatus   ConnectionStatus // Connection state
    mu           sync.RWMutex     // Protects connStatus
    rateLimiter  *rate.Limiter    // Rate limiting (10 req/sec)
}

type ConnectionStatus struct {
    Connected   bool      // Is bridge reachable?
    LastError   string    // Last error message
    BridgeAddr  string    // Bridge IP
    LastAttempt time.Time // Last connection attempt
}

type Scene struct {
    ID       string // Scene UUID
    Name     string // Scene name
    RoomName string // Room name (for display)
}
```

**Connection Management:**
- Lazy connection (connects on first use)
- Automatic error tracking
- Connection status exposed via DBus
- Rate limiting to prevent bridge overload (10 req/sec)

**Key Functions:**
- `GetScenes()` - Fetch all scenes with room names
- `ActivateScene(id)` - Activate a scene
- `SetLightPower(id, on)` - Control power
- `SetLightBrightness(id, brightness)` - Control brightness
- `GetGroupedLights()` - List rooms/zones
- `IsReachable()` - Test bridge connectivity

**Error Handling:**
- Network errors captured and stored
- Automatic retry on transient failures
- Clear error messages for users

---

### internal/dbus - DBus Service

**Purpose:** Expose backend functionality via DBus interface.

**Key Types:**
```go
type Service struct {
    conn             *dbus.Conn         // DBus connection
    config           *config.Config     // Config reference
    hueClient        *hue.Client        // Hue client
    syncEngine       *syncengine.Engine // Sync engine
    gamingDetector   *gaming.Detector   // Gaming detector
    gamingModeActive bool               // Gaming state
    mu               sync.RWMutex       // Guards the gaming state; config has its own lock
    ownerUID         uint32             // Service owner UID
    callerUID        func(dbus.Sender) (uint32, error) // Resolves sender to UID; nil fails closed
}
```

**Access Control:**
- UID-based filtering (only same-user processes)
- `checkAccess(sender)` verifies caller UID
- Reads that return bridge or bridge-derived data require owner; local config/state reads that expose no bridge resource identifiers or bridge state (e.g. GetStatus, GetSyncSettings) do not - GetStatus additionally reveals only whether bridge credentials are configured
- Write methods require owner UID match

**Method Categories:**
1. **Status:** GetStatus, GetConnectionStatus
2. **Scenes:** GetScenes, ActivateScene
3. **Lights:** SetPower, SetBrightness, GetState
4. **Sync:** StartSync, StopSync, IsSyncing
5. **Gaming:** SetGamingMode, IsGamingModeEnabled, IsGamingModeActive
6. **Config:** SetGroupedLight, SetSyncSettings, GetBridgeSettings

**Introspection:**
- Full introspection data exposed
- Compatible with d-feet and dbus-send
- Type-safe method signatures

---

### internal/sync - Screen Sync Engine

**Purpose:** Orchestrate screen capture, color extraction, and Entertainment API streaming.

**Key Types:**
```go
type Engine struct {
    cfg              *config.Config
    capture          *capture.Capturer
    entertainClient  *entertainment.Client
    running          bool
    stopChan         chan struct{}
    metrics          PerformanceMetrics
    mu               sync.RWMutex
    circuitBreaker   *CircuitBreaker // Error recovery
}

type PerformanceMetrics struct {
    FramesProcessed  uint64
    FramesDropped    uint64
    AvgFrameTime     time.Duration
    P50Latency       time.Duration
    P95Latency       time.Duration
    P99Latency       time.Duration
}
```

**Sync Loop:**
```go
// Simplified sync loop structure
func (e *Engine) syncLoop(ctx context.Context) {
    ticker := time.NewTicker(time.Second / time.Duration(e.cfg.Sync.FPS))
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            start := time.Now()

            // 1. Capture frame from PipeWire
            frame, err := e.capture.CaptureFrame()
            if err != nil {
                e.handleCaptureError(err)
                continue
            }

            // 2. Extract colors for each channel
            colors := make([]entertainment.RGBColor, len(e.cfg.Channels))
            for i, ch := range e.cfg.Channels {
                if !ch.Active {
                    continue
                }
                colors[i] = color.ExtractZone(frame, ch.UVA, ch.UVB, ch.GammaFactor)
            }

            // 3. Stream to Entertainment API
            err = e.entertainClient.Stream(colors)
            if err != nil {
                e.handleStreamError(err)
                continue
            }

            // 4. Update metrics
            e.metrics.RecordFrame(time.Since(start))

        case <-e.stopChan:
            return
        case <-ctx.Done():
            return
        }
    }
}
```

**Circuit Breaker:**
- Stops after 30 consecutive errors
- Prevents infinite error loops
- Logs clear message on circuit trip
- Manual restart required

**Performance Monitoring:**
- Frame time tracking
- Latency percentiles (P50, P95, P99)
- Dropped frame counting
- Metrics exposed via logs

---

### internal/capture - Screen Capture

**Purpose:** Capture screen content using native PipeWire integration.

**Key Types:**
```go
type Capturer struct {
    portalSession  *PortalSession
    pipewireNode   uint32
    pipewireStream *PipeWireStream
    rgbaBuffer     *RGBABuffer // Reusable buffer
    mu             sync.Mutex
}

type PortalSession struct {
    sessionPath string // DBus object path
    streamNode  uint32 // PipeWire node ID
    restoreToken string // Session restoration token
}
```

**Native PipeWire Integration (CGo):**
```c
// pipewire_native.c
// Direct libpipewire-0.3 integration for zero-copy capture
pw_stream* create_pipewire_stream(uint32_t node_id);
int capture_frame_to_buffer(pw_stream *stream, uint8_t *buffer, size_t size);
```

**XDG Desktop Portal Flow:**
1. Request screen sharing permission
2. User approves via GUI dialog
3. Portal returns PipeWire node ID
4. Connect to PipeWire stream
5. Capture frames via native CGo

**Buffer Reuse:**
- Single RGBA buffer allocated once
- Reused for all frames (eliminates 28MB/frame allocation)
- 91% reduction in memory allocations
- Prevents GC pressure

**Portal Timeout:**
- 2-minute timeout on permission dialog
- Prevents indefinite hangs
- Returns `PortalError:Timeout` if exceeded

---

### internal/entertainment - Entertainment API

**Purpose:** Stream color data to Hue bridge via DTLS-encrypted UDP.

**Key Types:**
```go
type Client struct {
    bridgeAddr   string
    clientKey    string
    areaID       string
    dtlsConn     *dtls.Conn
    isActive     bool
    mu           sync.Mutex
}
```

**Protocol:** HueStream v2
- Transport: DTLS 1.2 over UDP
- Port: 2100 (default Entertainment API port)
- Encryption: PSK-based (using clientKey)
- Format: 16-bit RGB channels per light

**Packet Format:**
```
┌──────────────────────────────────────────────┐
│ Header (9 bytes)                             │
│  - Protocol name: "HueStream"                │
│  - Version: 2.0                              │
│  - Sequence number                           │
│  - Reserved bytes                            │
├──────────────────────────────────────────────┤
│ Color Data (7 bytes per channel)             │
│  - Channel ID (1 byte)                       │
│  - Red (2 bytes, big-endian)                 │
│  - Green (2 bytes, big-endian)               │
│  - Blue (2 bytes, big-endian)                │
├──────────────────────────────────────────────┤
│ ... repeat for each active channel ...       │
└──────────────────────────────────────────────┘
```

**Activation Flow:**
1. PUT `/clip/v2/entertainment_configuration/{id}`
2. Wait for bridge response
3. Establish DTLS connection
4. Stream color packets at target FPS
5. Deactivate on stop

**Error Handling:**
- Automatic reconnection on DTLS errors
- Validates bridge response codes
- Logs detailed error information

---

### internal/color - Color Processing

**Purpose:** Extract colors from screen zones using UV coordinates.

**Key Types:**
```go
type Extractor struct {
    gammaTable []uint8 // Pre-computed gamma correction
}
```

**UV Coordinate System:**
```
Screen (any resolution):
    0.0             0.5             1.0
0.0 ┌───────────────┬───────────────┐
    │               │               │
    │   Left Zone   │  Right Zone   │
    │  UV: (0,0) to │  UV: (0.5,0)  │
0.5 │     (0.5,1)   │  to (1,1)     │
    │               │               │
    │               │               │
1.0 └───────────────┴───────────────┘

Resolution-agnostic: Works on 1080p, 1440p, 4K, etc.
```

**Stride-Based Sampling:**
```go
// Extract mean color from zone without resampling
func ExtractZone(img image.Image, uvA, uvB UV, gamma float32) RGBColor {
    bounds := img.Bounds()

    // Convert UV to pixel coordinates
    x1 := int(uvA.X * float32(bounds.Dx()))
    y1 := int(uvA.Y * float32(bounds.Dy()))
    x2 := int(uvB.X * float32(bounds.Dx()))
    y2 := int(uvB.Y * float32(bounds.Dy()))

    // Sample pixels with stride (skip pixels for speed)
    stride := calculateStride(x2 - x1)
    var r, g, b uint64
    var count int

    for y := y1; y < y2; y += stride {
        for x := x1; x < x2; x += stride {
            color := img.At(x, y)
            r32, g32, b32, _ := color.RGBA()
            r += uint64(r32 >> 8)
            g += uint64(g32 >> 8)
            b += uint64(b32 >> 8)
            count++
        }
    }

    // Calculate mean
    meanR := uint8(r / uint64(count))
    meanG := uint8(g / uint64(count))
    meanB := uint8(b / uint64(count))

    // Apply gamma correction
    return RGBColor{
        R: applyGamma(meanR, gamma),
        G: applyGamma(meanG, gamma),
        B: applyGamma(meanB, gamma),
    }
}
```

**Gamma Correction:**
- Default: 2.2 (standard sRGB)
- Pre-computed lookup table (fast)
- Per-channel configuration
- Range: 0.5 - 4.0

**Performance:**
- 41% faster than Lanczos resampling
- No intermediate buffer allocations
- Cache-friendly access patterns

---

### internal/gaming - Gaming Mode Detection

**Purpose:** Automatically start screen sync when gaming is detected.

**Key Types:**
```go
type Detector struct {
    systemdDetector      *SystemdDetector
    powerProfileDetector *PowerProfileDetector
    steamDetector        *SteamDetector
    gameMode             *GameModeDetector

    callback  StateChangeCallback
    isRunning bool
    stopChan  chan struct{}
    mu        sync.RWMutex

    pollInterval      time.Duration
    debounceDelay     time.Duration
    useSystemdInhibit bool
    usePowerProfile   bool
    useSteamAppId     bool
    useGameMode       bool

    currentState      bool      // State last reported, or InitiallyGaming
    pendingState      bool      // State waiting for the debounce
    stateChangedAt    time.Time // When pendingState was first seen
    debounceTriggered bool
}

type Config struct {
    PollInterval      time.Duration
    DebounceDelay     time.Duration
    UseSystemdInhibit bool
    UsePowerProfile   bool
    UseSteamAppId     bool
    UseGameMode       bool
    InitiallyGaming   bool // Seeds a detector that replaces one mid-game
}
```

A check turned off in `Config` leaves its detector nil. `NewDetector` returns
`nil, nil` when all four are nil.

**Detection Methods (Priority Order):**

1. **systemd-inhibit (Primary):** the lock CachyOS's `game-performance` wrapper takes
2. **Power Profile + Steam AppId (Secondary):** both have to be true
3. **Feral GameMode (Fallback):** `QueryStatus` on the session bus, off by default

[Gaming Mode Detection](#gaming-mode-detection) has the details of each.

**Detection Flow:**
```
Detector.Start()
 └─> go monitorLoop(stopChan)              one goroutine per run
      └─> every PollInterval: checkGamingState()
           ├─> detectGaming()              runs the checks one after another
           └─> debounce under Detector.mu
                └─> go callback(isGaming)  once per confirmed change
```

**Debouncing:**
- A result that differs from the reported state starts the timer
- The callback fires on the first poll at least `DebounceDelay` later, if the result still differs
- A poll that matches the reported state again drops the pending change
- With the defaults (2 s poll, 5 s delay) a change is reported 6 s after the first poll that sees it

**Callback Integration:**

`initGamingModeLocked` (`internal/dbus/service.go`) creates the detector only
when `gamingMode.enabled` is set and the sync engine exists, which needs the
Entertainment API configured. The callback calls
`onGamingStateChanged(detector, isGaming)`:

- **Game started:** ignored unless it comes from the current detector. If gaming mode doesn't already own sync, it sets `gamingModeActive`, starts sync through `startSyncForGaming` and runs `superviseGamingSync`. When sync is already running, gaming mode takes that session over.
- **Game ended:** accepted from any detector. It cancels the supervisor, then stops sync if gaming mode owned it.
- **Supervisor:** polls every 2 s and restarts a session that ended with a recorded failure, up to 3 attempts, waiting 5 s after the first and doubling. A minute of healthy running restores the budget. A manual `StopSync` clears the failure, so sync stays stopped until the game ends.

---

## Data Flow

### Scene Activation Flow

```
┌─────────────┐
│    User     │ Clicks scene in tray menu
│  (Human)    │
└──────┬──────┘
       │
       │ Qt signal/slot
       ▼
┌─────────────────────────┐
│   Tray App (Qt/C++)     │
│  • QDBusInterface.call()│
└────────────┬────────────┘
             │
             │ DBus Session Bus
             │ Method: ActivateScene(displayName)
             ▼
┌──────────────────────────────────┐
│   Backend DBus Service (Go)      │
│  • Validate input                │
│  • Check access control (UID)    │
│  • Parse scene name              │
└────────────┬─────────────────────┘
             │
             │ Function call
             ▼
┌──────────────────────────────────┐
│   Hue Client (Go)                │
│  • Look up scene by name         │
│  • Get scene UUID                │
└────────────┬─────────────────────┘
             │
             │ HTTPS POST
             │ /clip/v2/resource/scene/{id}
             ▼
┌──────────────────────────────────┐
│   Hue Bridge (Hardware)          │
│  • Validate scene ID             │
│  • Apply colors to lights        │
│  • Update light state            │
└────────────┬─────────────────────┘
             │
             │ HTTP 200 OK
             ▼
┌──────────────────────────────────┐
│   Backend DBus Service           │
│  • Return success message        │
└────────────┬─────────────────────┘
             │
             │ DBus reply
             ▼
┌─────────────────────────┐
│   Tray App              │
│  • Show notification    │
│  • "Scene activated"    │
└─────────────────────────┘
```

**Timing:**
- DBus call: ~1ms
- Bridge communication: ~50-100ms
- Total latency: ~100ms (imperceptible to user)

---

### Screen Sync Flow

```
┌─────────────┐
│    User     │ Clicks "Start Screen Sync"
└──────┬──────┘
       │
       ▼
┌─────────────────────────┐
│   Tray App              │
│  • QDBusInterface.call()│
│    StartSync()          │
└────────────┬────────────┘
             │
             │ DBus
             ▼
┌────────────────────────────────────────────────────────────┐
│   Backend: Sync Engine Start                              │
│  1. Activate Entertainment Area (HTTPS PUT to bridge)     │
│  2. Initialize PipeWire capture                           │
│  3. Request screen sharing permission (XDG Portal)        │
└────────────┬───────────────────────────────────────────────┘
             │
             │ XDG Desktop Portal
             ▼
┌────────────────────────────────────────────────────────────┐
│   System: Permission Dialog                               │
│   [Allow screen sharing? Select monitor]                  │
│   User must approve                                        │
└────────────┬───────────────────────────────────────────────┘
             │
             │ User clicks "Share"
             ▼
┌────────────────────────────────────────────────────────────┐
│   PipeWire: Start Streaming                               │
│  • Provides node ID                                        │
│  • Backend connects to stream                             │
└────────────┬───────────────────────────────────────────────┘
             │
             │ Enter sync loop (30 FPS)
             ▼
╔════════════════════════════════════════════════════════════╗
║   SYNC LOOP (runs every 33ms)                             ║
╠════════════════════════════════════════════════════════════╣
║                                                            ║
║  ┌──────────────────────────────────────────────────┐     ║
║  │ 1. Capture Frame (PipeWire native)               │     ║
║  │    • Read video buffer via CGo                    │     ║
║  │    • Reuse RGBA buffer (no allocation)           │     ║
║  │    Time: ~2ms                                     │     ║
║  └────────────┬─────────────────────────────────────┘     ║
║               │                                            ║
║               ▼                                            ║
║  ┌──────────────────────────────────────────────────┐     ║
║  │ 2. Extract Colors (color package)                │     ║
║  │    • For each active channel:                     │     ║
║  │      - Map UV to pixels                           │     ║
║  │      - Stride-based sampling                      │     ║
║  │      - Calculate mean RGB                         │     ║
║  │    Time: ~11ms                                    │     ║
║  └────────────┬─────────────────────────────────────┘     ║
║               │                                            ║
║               ▼                                            ║
║  ┌──────────────────────────────────────────────────┐     ║
║  │ 3. Apply Gamma Correction                        │     ║
║  │    • Per-channel gamma (default 2.2)             │     ║
║  │    • Lookup table (fast)                          │     ║
║  │    Time: <1ms                                     │     ║
║  └────────────┬─────────────────────────────────────┘     ║
║               │                                            ║
║               ▼                                            ║
║  ┌──────────────────────────────────────────────────┐     ║
║  │ 4. Stream to Entertainment API                   │     ║
║  │    • Build HueStream v2 packet                    │     ║
║  │    • Send via DTLS/UDP                            │     ║
║  │    • Bridge updates lights immediately           │     ║
║  │    Time: ~1ms                                     │     ║
║  └────────────┬─────────────────────────────────────┘     ║
║               │                                            ║
║               ▼                                            ║
║  ┌──────────────────────────────────────────────────┐     ║
║  │ 5. Update Metrics                                │     ║
║  │    • Record frame time                            │     ║
║  │    • Update latency percentiles                   │     ║
║  │    Time: <1ms                                     │     ║
║  └──────────────────────────────────────────────────┘     ║
║                                                            ║
║  Total: ~15ms average (well under 33ms budget)            ║
║                                                            ║
║  ┌─────────────────────────────────────────────────┐      ║
║  │ Sleep until next frame (33ms - frame_time)      │      ║
║  └─────────────────────────────────────────────────┘      ║
║                                                            ║
║  Loop until StopSync() called                             ║
╚════════════════════════════════════════════════════════════╝
```

**Performance Budget (30 FPS = 33ms per frame):**
- Capture: 2ms (6%)
- Color extraction: 11ms (33%)
- Gamma correction: <1ms (3%)
- Streaming: 1ms (3%)
- Metrics: <1ms (3%)
- **Total: ~15ms (45% of budget)**
- **Headroom: ~18ms (55%)**

---

## Threading & Concurrency

### Backend Concurrency Model

The Go backend uses goroutines and channels for concurrent operations:

**Main Goroutines:**
```
main() goroutine
├─> DBus service goroutine (handles method calls)
├─> Sync engine goroutine (30 FPS loop)
├─> Gaming detector goroutine (polls every 2 seconds)
└─> (Implicit) HTTP client goroutines (bridge communication)
```

**Synchronization Primitives:**

1. **sync.Mutex / sync.RWMutex**
   - `config.Config.mu` - Guards every read and write of a shared config, taken by `View`, `Update` and `Save`
   - `dbus.Service.mu` - Guards the gaming detector state
   - `sync.Engine.mu` - Protects engine state
   - `gaming.Detector.mu` - Protects gaming state

2. **Channels and wait groups**
   - `gaming.Detector.stopChan` - Stop signal for the current monitor loop, made by `Start` and closed by `Stop` under `Detector.mu`
   - `sync.Engine.syncLoopWg` - Lets a stop wait for the sync loop to exit before tearing down the capturer and client the next session reuses

3. **Context**
   - `context.Context` passed to long-running operations
   - Cancellation propagates to child goroutines

**DBus Method Concurrency:**
```go
// Each DBus method call runs in its own goroutine
// Service must be thread-safe

func (s *Service) GetScenes() ([]string, *dbus.Error) {
    // No mutex needed - read-only operation
    return s.hueClient.GetScenes()
}

func (s *Service) SetGroupedLight(id string, sender dbus.Sender) (bool, *dbus.Error) {
    // Update changes and saves the config under its own lock
    err := s.config.Update(func(c *config.Config) {
        c.GroupedLightID = id
    }, nil)
    return err == nil, dbusError(err)
}
```

**Sync Engine Concurrency:**
```go
// One sync loop per session, stopped by cancelling its context. A stop waits
// for the loop to exit before tearing down the capturer and client, which the
// next session reuses.

func (e *Engine) launchLoopLocked(ctx context.Context) uint64 {
    syncCtx, cancel := context.WithCancel(ctx)
    e.cancel = cancel
    e.running = true
    e.generation++
    // ...
    e.syncLoopWg.Add(1)
    go e.syncLoop(syncCtx, e.generation)
    return e.generation
}

func (e *Engine) stopLocked() error {
    if !e.running {
        return ErrNotRunning
    }
    e.cancel()
    e.syncLoopWg.Wait()
    e.logFinalMetrics()
    e.capturer.Stop()
    // ... close the Entertainment API client
    e.running = false
    return nil
}

// A loop that gives up marks itself done before ending its session, since
// endSession stops the session and so waits for this loop.
func (e *Engine) syncLoop(ctx context.Context, gen uint64) {
    err := e.streamFrames(ctx)
    e.syncLoopWg.Done()
    if err != nil {
        e.endSession(gen, err)
    }
}
```

**Gaming Detector Concurrency:**
```go
// One polling goroutine per run. Each run gets its own stop channel, closed
// under the lock, so a Start landing inside a Stop can't have its new
// channel closed by that Stop.

func (d *Detector) Start() {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.isRunning {
        return
    }
    d.stopChan = make(chan struct{})
    d.isRunning = true
    go d.monitorLoop(d.stopChan)
}

func (d *Detector) Stop() {
    d.mu.Lock()
    defer d.mu.Unlock()
    if !d.isRunning {
        return
    }
    d.isRunning = false
    close(d.stopChan)
}
```

---

## State Machines

### Sync Engine State Machine

```
┌─────────────┐
│   STOPPED   │ Initial state
└──────┬──────┘
       │
       │ StartSync()
       ▼
┌──────────────────────┐
│   ACTIVATING         │ Activating Entertainment Area
│   • PUT to bridge    │
│   • Wait for 200 OK  │
└──────┬───────────────┘
       │
       │ Success
       ▼
┌──────────────────────┐
│   INITIALIZING       │ Setting up PipeWire
│   • Portal request   │
│   • User approval    │
│   • Connect stream   │
└──────┬───────────────┘
       │
       │ Stream ready
       ▼
┌──────────────────────┐
│   RUNNING            │ Sync loop active
│   • Capture frames   │ ◄─────┐
│   • Extract colors   │       │
│   • Stream to bridge │       │ Every 33ms (30 FPS)
└──────┬───────────────┘       │
       │                       │
       │ StopSync()            │
       ▼                       │
┌──────────────────────┐       │
│   DEACTIVATING       │───────┘ Error (retry)
│   • Stop capture     │
│   • Deactivate area  │
│   • Close DTLS       │
└──────┬───────────────┘
       │
       │ Cleanup complete
       ▼
┌─────────────┐
│   STOPPED   │
└─────────────┘
```

**Error States:**
- **CAPTURE_ERROR:** Frame capture failed (circuit breaker after 30 errors)
- **STREAM_ERROR:** Entertainment API send failed (retry)
- **PORTAL_ERROR:** User cancelled or timeout (stop immediately)

---

### Gaming Mode State Machine

```
┌──────────────┐
│   DISABLED   │ gamingMode.enabled false, no sync engine, or no check available
└──────┬───────┘
       │
       │ Startup with enabled set, or SetGamingMode(true)
       ▼
┌──────────────────────┐
│   IDLE               │◄──────────────────────────┐
│   • Polls every 2 s  │                           │
└──────┬───────────────┘                           │
       │                                           │
       │ detectGaming() true                       │
       ▼                                           │
┌──────────────────────┐   false again             │
│   PENDING START      │───────────────────────────┤
│   • Waits 6 s        │                           │
└──────┬───────────────┘                           │
       │                                           │
       │ Still true after the delay                │
       ▼                                           │
┌──────────────────────┐                           │
│   GAMING             │ callback(true)            │
│   • Sync started     │◄──────────┐               │
│   • Supervisor runs  │           │ true again    │
└──────┬───────────────┘           │               │
       │                           │               │
       │ detectGaming() false      │               │
       ▼                           │               │
┌──────────────────────┐           │               │
│   PENDING STOP       │───────────┘               │
│   • Waits 6 s        │                           │
└──────┬───────────────┘                           │
       │                                           │
       │ Still false after the delay:              │
       │ callback(false), sync stopped             │
       └───────────────────────────────────────────┘
```

`SetGamingMode(false)` returns to DISABLED from any state. It stops the
detector and the supervisor and leaves a running session running, no longer
owned by gaming mode.

The tray shows the gaming icon while sync runs and `IsGamingModeActive` is
true. That method returns the current detection result without the debounce.

---

## Screen Sync Pipeline

### Pipeline Stages (Detailed)

**Stage 1: Screen Capture (2ms)**

```
┌────────────────────────────────────────────────────────────┐
│  Native PipeWire Capture (CGo)                             │
│                                                            │
│  1. pw_stream_dequeue_buffer(stream) → buffer_ptr         │
│  2. spa_buffer_find_meta_data(buffer, VIDEO_FRAME)        │
│  3. memcpy(rgba_buffer, dma_buf, width * height * 4)      │
│  4. pw_stream_queue_buffer(stream, buffer)                │
│                                                            │
│  ✓ Zero-copy access to video memory (DMA-BUF)             │
│  ✓ Reuses single RGBA buffer (no allocation)              │
│  ✓ Direct memory access (no syscalls)                     │
└────────────────────────────────────────────────────────────┘
```

**Stage 2: Color Extraction (11ms)**

```
┌────────────────────────────────────────────────────────────┐
│  Stride-Based Sampling                                     │
│                                                            │
│  For each channel (e.g., 2 channels):                      │
│    1. Map UV coordinates to pixels                        │
│       uvA: (0.0, 0.0) → (0px, 0px)                        │
│       uvB: (0.5, 1.0) → (1280px, 1440px)                  │
│                                                            │
│    2. Calculate stride (skip pixels)                      │
│       stride = max(1, zone_width / 32)                    │
│       (Sample 32x32 = 1024 pixels per zone)               │
│                                                            │
│    3. Sample with stride                                  │
│       for y in 0..1440 step stride:                       │
│         for x in 0..1280 step stride:                     │
│           sum += pixel_at(x, y)                           │
│                                                            │
│    4. Calculate mean RGB                                  │
│       meanR = sum.R / sample_count                        │
│       meanG = sum.G / sample_count                        │
│       meanB = sum.B / sample_count                        │
│                                                            │
│  ✓ No image resampling (41% faster)                       │
│  ✓ Cache-friendly linear access                           │
│  ✓ Configurable accuracy vs performance                   │
└────────────────────────────────────────────────────────────┘
```

**Stage 3: Gamma Correction (<1ms)**

```
┌────────────────────────────────────────────────────────────┐
│  Lookup Table Gamma Correction                            │
│                                                            │
│  Pre-computed table (done once at startup):               │
│    for i in 0..255:                                        │
│      gammaTable[i] = pow(i/255, 1/gamma) * 255            │
│                                                            │
│  Application (per channel):                               │
│    correctedR = gammaTable[meanR]                         │
│    correctedG = gammaTable[meanG]                         │
│    correctedB = gammaTable[meanB]                         │
│                                                            │
│  ✓ O(1) lookup (no pow() calls)                           │
│  ✓ Per-channel gamma values                               │
│  ✓ Default: 2.2 (sRGB standard)                           │
└────────────────────────────────────────────────────────────┘
```

**Stage 4: Entertainment API Streaming (1ms)**

```
┌────────────────────────────────────────────────────────────┐
│  DTLS Packet Construction & Send                          │
│                                                            │
│  Packet structure:                                         │
│    Header (9 bytes):                                       │
│      [0-8]: "HueStream"                                    │
│      [9]: Version (0x02)                                   │
│      [10-11]: Sequence number (uint16)                     │
│      [12-15]: Reserved (0x00)                              │
│                                                            │
│    Per-channel data (7 bytes):                             │
│      [0]: Channel ID (uint8)                               │
│      [1-2]: Red (uint16, big-endian, 0-65535)             │
│      [3-4]: Green (uint16, big-endian, 0-65535)           │
│      [5-6]: Blue (uint16, big-endian, 0-65535)            │
│                                                            │
│  Send:                                                     │
│    dtls.Write(packet) → UDP to bridge:2100                │
│                                                            │
│  ✓ Low latency (UDP, no ACK wait)                         │
│  ✓ Encrypted (DTLS 1.2)                                   │
│  ✓ Automatic retry on transient errors                    │
└────────────────────────────────────────────────────────────┘
```

### Performance Optimization History

**Before Optimization (PR #41 baseline):**
- Frame time: 19ms average
- FPS: 27.5 (dropping frames)
- Memory: 827 MB/sec allocations
- CPU: 72% in image resampling

**After Stride Sampling (PR #42):**
- Frame time: 11ms average (-41%)
- FPS: 30.0 (perfect)
- Memory: Still high (resampling allocations)
- CPU: 5% in image operations (-67%)

**After Buffer Reuse (PR #43):**
- Frame time: 11ms average (maintained)
- FPS: 30.0 (perfect)
- Memory: 80 MB/sec allocations (-90%)
- Eliminated: 28MB allocation per frame

**After Circuit Breaker (PR #46):**
- Added: Stop after 30 consecutive errors
- Prevents: Infinite error loops
- Logs: Clear message on circuit trip

**After Portal Timeout (PR #47):**
- Added: 2-minute timeout on permission dialog
- Prevents: Indefinite hangs
- Returns: Clear error message

**Current Performance (PRs #42-50):**
- ✅ 30.0 FPS sustained
- ✅ 11ms average frame time
- ✅ 80 MB/sec allocations
- ✅ No dropped frames
- ✅ Robust error handling

---

## Entertainment API Flow

### Full Activation Sequence

```
┌────────────────────────────────────────────────────────────────┐
│ Step 1: Get Entertainment Configuration                       │
│                                                                │
│ GET /clip/v2/resource/entertainment_configuration              │
│                                                                │
│ Response:                                                      │
│ {                                                              │
│   "data": [{                                                   │
│     "id": "abc-123-def",                                       │
│     "name": "Entertainment area 1",                            │
│     "status": "inactive",                                      │
│     "channels": [                                              │
│       {"channel_id": 0, "position": {"x": -0.5, "y": 0, "z": 0}},│
│       {"channel_id": 1, "position": {"x": 0.5, "y": 0, "z": 0}}  │
│     ]                                                          │
│   }]                                                           │
│ }                                                              │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Step 2: Activate Entertainment Area                           │
│                                                                │
│ PUT /clip/v2/resource/entertainment_configuration/{id}         │
│ {                                                              │
│   "action": "start"                                            │
│ }                                                              │
│                                                                │
│ Response:                                                      │
│ {                                                              │
│   "data": [{                                                   │
│     "rid": "abc-123-def",                                      │
│     "rtype": "entertainment_configuration"                     │
│   }],                                                          │
│   "errors": []                                                 │
│ }                                                              │
│                                                                │
│ ✓ Bridge reserves Entertainment API for this client           │
│ ✓ Normal light control now blocked (Entertainment mode)       │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Step 3: Establish DTLS Connection                             │
│                                                                │
│ DTLS handshake to bridge:2100                                  │
│   • PSK authentication using clientKey                         │
│   • TLS_PSK_WITH_AES_128_GCM_SHA256 cipher                    │
│   • No certificate validation required                         │
│                                                                │
│ DTLS connection established                                    │
│   • Bidirectional encrypted UDP channel                        │
│   • Low latency (no TCP overhead)                             │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Step 4: Stream Color Data                                     │
│                                                                │
│ Send HueStream v2 packets at 30 FPS                            │
│   • Each packet updates all channels                           │
│   • Bridge applies colors immediately                          │
│   • No response required (fire-and-forget)                     │
│                                                                │
│ Continue streaming until StopSync() called                     │
└────────────────────────────────────────────────────────────────┘
                              │
                              │ User stops sync
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Step 5: Deactivate Entertainment Area                         │
│                                                                │
│ PUT /clip/v2/resource/entertainment_configuration/{id}         │
│ {                                                              │
│   "action": "stop"                                             │
│ }                                                              │
│                                                                │
│ ✓ Bridge releases Entertainment mode                           │
│ ✓ Normal light control now available                          │
│ ✓ DTLS connection closed                                      │
└────────────────────────────────────────────────────────────────┘
```

### Entertainment API Limitations

**While Active:**
- ❌ Cannot activate scenes via REST API
- ❌ Cannot control lights via REST API
- ❌ Cannot change brightness/color via REST API
- ✅ Can read light state (GET requests still work)
- ✅ Can control via Entertainment API streaming

**Connection Requirements:**
- Must have `clientkey` (obtained during bridge pairing)
- Must have `entertainmentConfigurationId` (Entertainment area UUID)
- Must activate area before streaming
- Must deactivate when done (or timeout after 10 minutes)

**Performance Limits:**
- Maximum: 60 FPS
- Recommended: 20-30 FPS
- Channels: Up to 10 lights per area
- Latency: ~20-50ms (network + light response)

---

## Gaming Mode Detection

### Detection Architecture

`detectGaming` runs on the `monitorLoop` goroutine. It calls the enabled checks
one after another and returns at the first that holds:

```
systemdDetector.IsActive()                   true ──► gaming
powerProfileDetector.IsPerformanceMode()
  && steamDetector.IsActive()                true ──► gaming
gameMode.IsActive()                          true ──► gaming
                                             otherwise not gaming
```

Each check execs a command or makes a DBus call, so a poll takes as long as
the checks it reaches.

### Detection Methods (Technical Details)

**1. systemd-inhibit (Primary)**, `systemd.go`

`game-performance` runs the game under `systemd-inhibit --why "CachyOS
game-performance is running"`, which lists as:

```
WHO        UID  USER PID    COMM            WHAT                WHY                                 MODE
<command>  1000 user 221404 systemd-inhibit shutdown:sleep:idle CachyOS game-performance is running block
```

`IsActive` runs `systemd-inhibit --list` and returns true when:
- the whole output, lowercased, contains both `cachyos` and `game`
- the output contains `game-performance`
- any line, lowercased, contains `game` or `gaming` together with `block`

With `GAME_PERFORMANCE_SCREENSAVER_ON` set, `game-performance` skips the lock
and only sets the power profile. When `powerprofilesctl list` has no
`performance` profile, it runs the game with neither.

**2. Power Profile (Secondary)**, `powerprofile.go`

`IsPerformanceMode` runs `powerprofilesctl get` and compares the trimmed output
with `performance`. `game-performance` starts the game under
`powerprofilesctl launch -p performance`, which holds that profile until the
game exits. A profile set by hand reads the same, which is why this check only
counts together with Steam AppId.

**3. Steam AppId (Secondary)**, `steam.go`

`IsActive` runs `pgrep -a reaper` and returns true when the output contains
`AppId=`. Steam starts each game as `reaper SteamLaunch AppId=<id> -- ...`.
`pgrep` also lists the kernel's `oom_reaper` thread, whose line has no
`AppId=`.

**4. Feral GameMode (Fallback)**, `gamemode.go`

`NewGameModeDetector` connects to the session bus, where gamemoded runs per
user. `IsActive` calls `com.feralinteractive.GameMode.QueryStatus(0)` on
`/com/feralinteractive/GameMode` and returns true when the result is above 0,
which it is while any client holds GameMode. The call passes
`dbus.FlagNoAutoStart`, so polling never starts gamemoded; with the daemon not
running the call fails and the check is false. gamemoded drops a client that
exits without unregistering on its next reaper pass, 5 s apart by default, so
the check can stay true that long after a game crashes.

### Aggregation Logic

`detectGaming` has one rule, whatever the distro. A detector disabled in the
config is nil and drops out of it.

```go
// Primary: systemd-inhibit (high confidence)
// Secondary: power profile AND Steam AppId, which have to agree
// Fallback: Feral GameMode
isGaming := systemdInhibit || (powerProfile && steamAppId) || gameMode
```

### Debouncing Algorithm

`checkGamingState` keeps the debounce in the detector's own fields, under
`Detector.mu`:

```go
func (d *Detector) checkGamingState() {
    isGaming := d.detectGaming()

    d.mu.Lock()
    defer d.mu.Unlock()

    if isGaming != d.currentState {
        if isGaming != d.pendingState {
            d.pendingState = isGaming
            d.stateChangedAt = time.Now()
            d.debounceTriggered = false
            return
        }

        if !d.debounceTriggered && time.Since(d.stateChangedAt) >= d.debounceDelay {
            d.currentState = isGaming
            d.debounceTriggered = true
            if d.callback != nil {
                go d.callback(isGaming)
            }
        }
    } else {
        if d.pendingState != d.currentState {
            d.pendingState = d.currentState
            d.debounceTriggered = false
        }
    }
}
```

The callback runs on its own goroutine, so a slow `StartSync` doesn't hold up
polling. Two callbacks can then run at once, and `onGamingStateChanged` is
written for a game-ended call finishing before the game-started one.

---

## Configuration System

### Config File Structure

**Location:** `$XDG_CONFIG_HOME/openhue/config.yaml` when `XDG_CONFIG_HOME` is set, otherwise `~/.openhue/config.yaml`. See [Configuration File Location](CONFIGURATION.md#configuration-file-location).

**Format:**
```yaml
# Version for migration tracking
version: 1

# Bridge connection (required)
Bridge: "192.168.1.100"
Key: "your-api-key-here"

# Client key for Entertainment API (optional)
clientkey: "client-key-for-dtls"

# Entertainment area ID (optional)
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"

# Grouped light for power/brightness control (optional)
grouped_light_id: "room-uuid-here"

# Screen sync settings
sync:
  enabled: false
  fps: 30                    # 10-60
  subsampleWidth: 64         # 16-256
  monitor: ""                # Empty = default monitor
  restoreToken: ""           # Portal session token

# Gaming mode settings
gamingMode:
  enabled: false
  pollInterval: 2            # Seconds
  debounceDelay: 5           # Seconds
  useSystemdInhibit: true    # CachyOS primary
  usePowerProfile: true      # CachyOS secondary
  useSteamAppId: true        # Steam games
  useGameMode: false         # Feral GameMode

# Entertainment API channel mapping
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA:
      x: 0.0
      y: 0.0
    uvB:
      x: 0.5
      y: 1.0
  - id: 1
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA:
      x: 0.5
      y: 0.0
    uvB:
      x: 1.0
      y: 1.0

# UI settings
ui:
  icons:
    gaming: "applications-games"
    syncing: "media-record"
    idle: "preferences-desktop-display-color"

# Logging level
log_level: "info"
```

### Configuration Loading Flow

```
┌────────────────────────────────────────────────────────────┐
│ config.Load()                                              │
│                                                            │
│ 1. Get config path                                         │
│    • Check XDG_CONFIG_HOME                                 │
│    • Fallback: ~/.openhue                                  │
│                                                            │
│ 2. Create directory if missing                             │
│    • mkdir -p the directory from step 1                    │
│                                                            │
│ 3. Initialize defaults                                     │
│    • DefaultConfig() struct                                │
│                                                            │
│ 4. Read config file                                        │
│    • viper.ReadInConfig()                                  │
│    • If not found: return defaults (first run)            │
│                                                            │
│ 5. Unmarshal into struct                                   │
│    • viper.Unmarshal(&cfg)                                 │
│    • Merge with defaults                                   │
│                                                            │
│ 6. Validate configuration                                  │
│    • Check required fields                                 │
│    • Validate ranges                                       │
│    • Validate UV coordinates                               │
│                                                            │
│ 7. Check file permissions                                  │
│    • Warn if too permissive (> 0600)                       │
│    • Contains sensitive API keys                           │
│                                                            │
│ 8. Return config                                           │
│    • Ready for use                                         │
└────────────────────────────────────────────────────────────┘
```

### Validation Rules

**Bridge & Keys:**
- `Bridge` - Required, non-empty string
- `Key` - Required, non-empty string
- `ClientKey` - Optional (Entertainment API only)

**Sync Settings:**
- `sync.fps` - Range: 10-60
- `sync.subsampleWidth` - Range: 16-256
- `sync.monitor` - Any string; stored but not yet applied

**Gaming Mode:**
- `gamingMode.pollInterval` - Minimum: 1 second
- `gamingMode.debounceDelay` - Minimum: 1 second
- All `use*` flags - Boolean

**Channels:**
- `id` - Range: 0-255
- `active` - Boolean
- `gammaFactor` - Range: 0.5-4.0
- `uvA.x`, `uvA.y` - Range: 0.0-1.0
- `uvB.x`, `uvB.y` - Range: 0.0-1.0
- `uvA.x < uvB.x` - UVA must be top-left
- `uvA.y < uvB.y` - UVB must be bottom-right

### Thread Safety

**Concurrent Access Protection:**

The DBus service and the sync engine share one `*config.Config`. Once it is
shared, every read goes through `View` and every change through `Update`, both
of which take the config's own mutex, the same one `Save` takes:

```go
// Reading config
var fps int
s.config.View(func(c *config.Config) {
    fps = c.Sync.FPS
})

// Changing config: applied and saved under one lock. The second function
// undoes the change if the save fails; pass nil to keep it in memory.
var prev config.SyncConfig
err := s.config.Update(func(c *config.Config) {
    prev = c.Sync
    c.Sync.FPS = fps
}, func(c *config.Config) {
    c.Sync = prev
})
```

The sync loop copies what it needs at the start of each session rather than
calling `View` per frame, where it would wait out every `Save`'s disk write.

**Non-Global Viper:**
- Each Config has its own `*viper.Viper` instance
- Prevents global state contamination
- Safe for concurrent config instances

---

## Error Handling

### Error Categories

**1. User Errors (Fixable by User)**
- Missing configuration
- Invalid input ranges
- Bridge unreachable
- **Response:** Clear error message with fix instructions

**2. Permission Errors**
- Access control denied (UID mismatch)
- Portal permission denied
- **Response:** Explain security reason

**3. System Errors (Recoverable)**
- Network timeout
- Bridge temporarily unavailable
- PipeWire stream error
- **Response:** Automatic retry with backoff

**4. Fatal Errors (Not Recoverable)**
- Config file corruption
- DTLS handshake failure
- Circuit breaker trip
- **Response:** Log error, stop operation

### Error Handling Patterns

**DBus Error Wrapping:**
```go
// Backend returns dbus.Error
func (s *Service) ActivateScene(name string) (string, *dbus.Error) {
    scene, err := s.hueClient.FindScene(name)
    if err != nil {
        // Wrap error with context
        return "", dbus.MakeFailedError(fmt.Errorf("scene not found: %s", name))
    }

    err = s.hueClient.ActivateScene(scene.ID)
    if err != nil {
        return "", dbus.MakeFailedError(err)
    }

    return "Scene activated: " + name, nil
}
```

**Portal Error Handling:**
```go
// Special handling for portal errors
if portalErr, ok := err.(*capture.PortalError); ok {
    // Format: "PortalError:TYPE:HINT"
    errMsg := fmt.Sprintf("PortalError:%s:%s", portalErr.Type, portalErr.Hint)
    return false, dbus.MakeFailedError(fmt.Errorf("%s", errMsg))
}
```

**Circuit Breaker:**
```go
type CircuitBreaker struct {
    maxErrors     int  // 30
    errorCount    int
    tripped       bool
}

func (cb *CircuitBreaker) RecordError() bool {
    cb.errorCount++
    if cb.errorCount >= cb.maxErrors {
        cb.tripped = true
        log.Println("🔴 Circuit breaker tripped after 30 consecutive errors")
        return true // Stop operation
    }
    return false
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.errorCount = 0 // Reset on success
}
```

### Validation Error Messages

**Good Error Messages (Implemented):**
```
❌ Bad: "brightness must be 0-100"
✅ Good: "brightness must be 0-100 (got 150)"

❌ Bad: "invalid UV coordinate"
✅ Good: "channel 0: uvA.x must be 0.0-1.0 (got 1.5)
         → UV coordinates represent screen position as fractions
         → 0.0 = left/top edge, 1.0 = right/bottom edge"

❌ Bad: "bridge not configured"
✅ Good: "bridge IP not configured
         → Add 'Bridge: YOUR_BRIDGE_IP' to /home/user/.openhue/config.yaml
         → You can discover your bridge with: openhue discover"
```

---

## Security Architecture

### Attack Surface Analysis

**DBus Interface:**
- **Threat:** Unauthorized control of lights
- **Mitigation:** UID-based access control
- **Result:** Only same-user processes can control

**Screen Capture:**
- **Threat:** Unauthorized screen recording
- **Mitigation:** XDG Desktop Portal user approval
- **Result:** User must explicitly approve each session

**Entertainment API:**
- **Threat:** Man-in-the-middle attacks
- **Mitigation:** DTLS 1.2 encryption with PSK
- **Result:** Encrypted UDP packets, no certificate validation needed

**Config File:**
- **Threat:** API key theft
- **Mitigation:** File permissions 0600 (owner read/write only)
- **Result:** Only user can read/write config

### Access Control Implementation

**UID Verification:**
```go
func (s *Service) checkAccess(sender dbus.Sender) error {
    if s.callerUID == nil {
        log.Printf("[WARN] Access denied: no caller UID resolver configured")
        return fmt.Errorf("access denied: unable to verify caller identity")
    }

    callerUID, err := s.callerUID(sender)
    if err != nil {
        log.Printf("[WARN] Failed to get caller UID: %v", err)
        return fmt.Errorf("access denied: unable to verify caller identity")
    }

    // Compare with service owner UID
    if callerUID != s.ownerUID {
        log.Printf("[WARN] Access denied: caller UID %d != owner UID %d", callerUID, s.ownerUID)
        return fmt.Errorf("access denied: only the service owner can perform this operation")
    }

    return nil
}
```

A nil `callerUID` resolver (e.g. a directly-constructed `Service` in tests) fails closed rather than allowing or falling back to the real DBus lookup.

**Methods Requiring Access Control:**

Mutators:
- SetPower, SetBrightness
- ActivateScene
- SetGroupedLight, SetSyncSettings
- StartSync, StopSync
- SetSelectedRoom, SetStartupScene
- SetGamingMode
- SetTrayIcons

Bridge probes (trigger on-demand bridge I/O):
- RetryConnection, TestBridgeConnection

Bridge-data readers (return bridge or bridge-derived data):
- GetScenes, GetGroupedLights
- GetState
- GetConnectionStatus, GetBridgeSettings
- GetSelectedRoom, GetStartupScene

**Methods Without Access Control** (local config/state reads exposing no bridge resource identifiers or bridge state):
- GetStatus, IsSyncing
- GetSyncSettings, GetTrayIcons
- IsGamingModeEnabled, IsGamingModeActive

GetStatus is the exception worth noting: it discloses one bit beyond pure local state - whether `config.Bridge`/`config.Key` are set - via its "Ready" vs "Not configured" result. It stays unguarded as the basic daemon liveness/configured probe; it returns no bridge resource identifiers or bridge state.

---

## Performance Characteristics

### Latency Measurements

**Config Loading:**
- Cold start: ~10ms
- Warm start: ~1ms (cached)
- Validation: ~1ms

**DBus Method Calls:**
- Local (tray ↔ backend): ~0.1-1ms
- Remote (backend ↔ bridge): ~50-100ms

**Screen Sync:**
- Frame capture: ~2ms
- Color extraction: ~11ms
- Gamma correction: <1ms
- Entertainment streaming: ~1ms
- **Total per frame: ~15ms average**

**Gaming Detection:**
- Poll interval: 2 seconds
- Detection time: <100ms per poll
- Debounce delay: 5 seconds
- **Total activation latency: ~7 seconds**

### Memory Usage

**Backend Service:**
- Base: ~20 MB
- With sync active: ~40 MB
- RGBA buffer: 8.8 MB (2560×1440×4 bytes)

**Tray Application:**
- Base: ~30 MB (Qt overhead)
- With menu open: ~35 MB

**Allocations (Sync Active):**
- Before optimization: 827 MB/sec
- After optimization: 80 MB/sec (-90%)
- GC pressure: Minimal (buffer reuse)

### CPU Usage

**Idle:**
- Backend: <1% CPU
- Tray: 0% CPU

**Sync Active:**
- Backend: 8-12% CPU (single core)
- PipeWire: 2-3% CPU
- Total: ~15% CPU (one core)

**Gaming Detection:**
- Poll overhead: <0.1% CPU
- Runs every 2 seconds
- Negligible impact

---

## Design Decisions

### Why Three Components?

**Decision:** Separate tray app, backend service, and bridge communication.

**Rationale:**
1. **Separation of Concerns:**
   - Tray app: Simple UI, user interaction
   - Backend: Complex logic, system integration
   - Clear interface via DBus

2. **Crash Isolation:**
   - Tray crash doesn't affect sync
   - Backend crash doesn't freeze UI
   - Independent restart

3. **Testability:**
   - Backend fully testable via DBus commands
   - No GUI required for integration tests
   - Can test backend independently

4. **Reusability:**
   - Backend usable from CLI, scripts, other apps
   - Tray replaceable with plasmoid or different UI
   - Multiple frontends possible

**Alternatives Considered:**
- ❌ Monolithic Qt application - Harder to test, tight coupling
- ❌ Electron app - Higher resource usage, worse performance
- ❌ Pure CLI tool - Less user-friendly, no tray integration

---

### Why DBus?

**Decision:** Use DBus for IPC between tray app and backend.

**Rationale:**
1. **Native Integration:**
   - Standard for KDE/GNOME system services
   - Built-in to systemd and desktop environments
   - Excellent tool support (d-feet, dbus-monitor)

2. **Security:**
   - Built-in access control (UID-based)
   - Introspection support
   - Type safety

3. **Performance:**
   - Low latency (~1ms for local calls)
   - Efficient binary protocol
   - Async support in Qt

**Alternatives Considered:**
- ❌ Unix sockets - Manual access control, no introspection
- ❌ HTTP REST - Overkill, higher latency, port management
- ❌ Websockets - Complex, server management, overkill

---

### Why Native PipeWire?

**Decision:** Use native PipeWire integration via CGo instead of GStreamer.

**Rationale:**
1. **Performance:**
   - Zero-copy access to video memory (DMA-BUF)
   - Lower latency than GStreamer pipeline
   - Direct control over buffer reuse

2. **Simplicity:**
   - No external dependencies beyond libpipewire
   - Smaller attack surface
   - Easier to maintain

3. **Wayland Integration:**
   - Native Wayland screen capture
   - Works with XDG Desktop Portal
   - Future-proof (X11 being phased out)

**Alternatives Considered:**
- ❌ GStreamer - Higher latency, complex pipeline, larger dependencies
- ❌ FFmpeg - Overkill, not designed for live streaming
- ❌ X11 XGetImage - Doesn't work on Wayland, deprecated

---

### Why UV Coordinates?

**Decision:** Use UV coordinates (0.0-1.0) for zone mapping instead of pixels.

**Rationale:**
1. **Resolution Agnostic:**
   - Works on any monitor resolution
   - Config portable between displays
   - No recalculation needed

2. **Industry Standard:**
   - Used in OpenGL, Vulkan, game engines
   - Familiar to developers
   - Well-documented concept

3. **Multi-Monitor Ready:**
   - Extend U axis for multiple monitors
   - Easy to calculate relative positions
   - Future-proof architecture

**Alternatives Considered:**
- ❌ Pixel coordinates - Resolution-dependent, breaks on monitor change
- ❌ Percentage-based - Similar to UV but less standard
- ❌ Absolute positions - Not portable, config hell

---

### Why Go for Backend?

**Decision:** Use Go for backend instead of C++, Rust, or Python.

**Rationale:**
1. **Concurrency:**
   - Goroutines perfect for sync loop, gaming detection, DBus
   - Channels for clean communication
   - Excellent standard library (HTTP, TLS, context)

2. **Performance:**
   - Fast compilation (rapid iteration)
   - Good runtime performance
   - Low memory footprint
   - GC handles memory management

3. **Ecosystem:**
   - Excellent DBus library (godbus)
   - Mature DTLS library (pion/dtls)
   - Good Hue API library (openhue-go)

4. **Maintainability:**
   - Simple syntax
   - Good tooling (go fmt, go test, go mod)
   - Easy cross-platform builds

**Alternatives Considered:**
- ❌ C++ - Manual memory management, slower compilation, more complexity
- ❌ Rust - Steeper learning curve, longer compile times, less mature DBus
- ❌ Python - Slower runtime, GIL limitations, poor concurrency

---

### Why Qt/KDE for Tray?

**Decision:** Use Qt6/C++ with KDE Frameworks for tray app.

**Rationale:**
1. **Native KDE Integration:**
   - KStatusNotifierItem (proper Plasma tray)
   - KNotification (desktop notifications)
   - Consistent with KDE ecosystem

2. **Performance:**
   - Native binary (not Electron)
   - Low resource usage
   - Fast startup

3. **DBus Integration:**
   - QDBus for async communication
   - Type-safe D-Bus wrappers
   - Well-documented

**Alternatives Considered:**
- ❌ GTK - Not native to KDE, inconsistent UI
- ❌ Electron - Massive resource usage, slow startup
- ❌ Pure C++ - Qt provides better abstractions

---

## Future Architecture

### Potential Multi-Monitor Support

**Current:** Single monitor capture
**Future:** Multiple monitors with per-light mapping

```yaml
channels:
  - id: 0
    deviceName: "Monitor 1 - Left"
    monitor: 0  # Monitor index
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}

  - id: 1
    deviceName: "Monitor 1 - Right"
    monitor: 0
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}

  - id: 2
    deviceName: "Monitor 2 - Full"
    monitor: 1  # Second monitor
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

**Implementation:**
- Capture each monitor separately
- Map UV coordinates per-monitor
- Combine in Entertainment API packet

---

### Potential Settings Dialog

**Architecture:**
```
┌───────────────────────────┐
│   Settings Dialog (Qt)    │
│  • Visual zone editor     │
│  • Bridge configuration   │
│  • Performance tuning     │
│  • Gaming mode config     │
└─────────────┬─────────────┘
              │ QDBus
              ▼
┌──────────────────────────────────┐
│  Backend DBus Interface          │
│  • GetConfig() → YAML            │
│  • SetConfig(yaml) → bool        │
│  • ValidateConfig(yaml) → errors │
│  • GetChannels() → []Channel     │
│  • SetChannel(ch) → bool         │
└──────────────────────────────────┘
```

**Features:**
- Visual zone editor (drag rectangles on screenshot)
- Real-time preview (see colors per zone)
- Bridge discovery and pairing wizard
- Performance tuning (FPS, subsample width)
- Gaming mode method selection

---

### Potential Plasmoid

**Replace tray app with KDE Plasma widget:**

```
┌───────────────────────────┐
│   Plasma Widget (QML)     │
│  • Compact: Icon only     │
│  • Expanded: Full controls│
│  • Applet config: Settings│
└─────────────┬─────────────┘
              │ QDBus
              ▼
┌──────────────────────────────────┐
│  Backend (Unchanged)             │
│  • Same DBus API                 │
│  • No code changes required      │
└──────────────────────────────────┘
```

**Advantages:**
- Native Plasma integration
- Configurable in System Settings
- Better UX for KDE users

---

## Contributing

When modifying the architecture:

1. **Maintain DBus stability:** Never break existing methods
2. **Update this doc:** Keep architecture current
3. **Test integration:** Use `scripts/test-integration.sh`
4. **Consider performance:** Profile before/after changes
5. **Document decisions:** Explain "why" in comments and docs

---

## See Also

- **API Reference:** See `API_REFERENCE.md` for complete DBus API
- **Configuration:** See `CONFIGURATION.md` for config.yaml reference
- **Testing:** See `TESTING_GUIDE.md` for test patterns
- **Performance:** See `PERFORMANCE.md` for optimization guidance
- **Development:** See `DEVELOPMENT.md` for building and testing

---

**Last Updated:** April 2026
**Architecture Version:** 1.0
**Backend Version:** Compatible with khuey backend v1.x
