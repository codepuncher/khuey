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
│  │  • HueBackend proxy │              ┌─────────────────────────┐    │
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

Most packages are a single file; `capture/`, `gaming/` and `testutil/` are the
exceptions. The layout below is every non-test file, with `_test.go` files left
out.

```
backend/
├── cmd/
│   ├── hue-sync/               # The daemon
│   ├── profile-sync/           # CPU/memory profiling of a sync session
│   ├── get-entertainment-info/ # Lists the bridge's entertainment areas
│   ├── register-entertainment/ # Link-button pairing, prints Key and clientkey
│   ├── test-capture/           # Screen capture
│   ├── test-color/             # Color extraction
│   ├── test-entertainment/     # Entertainment API streaming
│   ├── test-gaming-detection/  # Gaming detector
│   └── test-zones-visual/      # Zone mapping visualization
│
└── internal/
    ├── config/
    │   └── config.go            # Config struct, load, save, validation
    │
    ├── hue/
    │   └── client.go            # REST wrapper: scenes, power, brightness, rooms
    │
    ├── dbus/
    │   └── service.go           # Methods, introspection and access control
    │
    ├── sync/
    │   └── engine.go            # Sync loop, metrics, area activation
    │
    ├── capture/
    │   ├── capture.go           # ScreenCapture, frame reader loop, buffer pool
    │   ├── portal.go            # XDG Desktop Portal
    │   ├── pipewire_native.go   # CGo bindings, format conversion, stream health
    │   ├── pipewire_native.c    # libpipewire-0.3 stream and frame handler
    │   └── pipewire_native.h    # The C interface Go calls
    │
    ├── entertainment/
    │   └── client.go            # DTLS client and HueStream v2 packets
    │
    ├── color/
    │   └── extractor.go         # UV zones, stride sampling, gamma
    │
    ├── gaming/
    │   ├── detector.go          # Aggregation, polling, debounce
    │   ├── systemd.go           # systemd-inhibit check
    │   ├── powerprofile.go      # powerprofilesctl check
    │   ├── steam.go             # Steam AppId check
    │   └── gamemode.go          # Feral GameMode check
    │
    ├── common/
    │   └── utils.go             # HTTP client, DBus string validation, log sanitising
    │
    └── testutil/
        ├── mock_bridge.go        # Mock Hue bridge
        └── mock_entertainment.go # Mock Entertainment API endpoint
```

Dependencies are not vendored; the build resolves them from the module cache.

### Tray Application Structure

```
trayapp/
├── main.cpp               # Entry point, tray icon, menu
│   ├── KStatusNotifierItem setup
│   ├── HueBackend proxy
│   ├── Menu population
│   └── Signal/slot connections
│
├── settingsdialog.h       # Settings GUI
├── settingsdialog.cpp     # Holds its own HueBackend proxy
│
├── huebackend.h           # Hand-written DBus proxy, header only
│
└── CMakeLists.txt         # Qt6 Core/Widgets/DBus, KF6 StatusNotifierItem,
                           # Notifications and IconThemes; C++17; AUTOMOC
```

There is no generated DBus interface: `huebackend.h` is written by hand because
`QDBusInterface` introspects the service in its constructor, and that call
blocks until the DBus timeout when the backend is hung.

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
    GroupedLightID               string            // Room/zone for control
    ClientKey                    string            // Entertainment API client key
    EntertainmentConfigurationID string            // Entertainment area ID
    Channels                     []ChannelConfig   // Light channel mapping
    StartupScene                 string            // Scene activated at backend start
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
- One `sync.Mutex` taken by `View`, `Update` and `Save`
- Non-global Viper instance (prevents global state contamination)
- `IsConfigured`, `HasEntertainmentConfig` and `Validate` take no lock: they
  read fields directly and are meant to be called from inside `View` or
  `Update`, which already hold it

**Key Functions:**
- `Load()` - Read `config.yaml` from the directory `getConfigPath` returns (see [Config File Structure](#config-file-structure))
- `Save()` - Write config with secure permissions (0600)
- `Validate()` - Validate all fields with helpful error messages
- `IsConfigured()` - Check if basic setup is complete
- `HasEntertainmentConfig()` - Check if Entertainment API is ready

**Load Behavior:**
- Creates the config directory and attempts a read every time; a missing file
  returns the defaults rather than an error (first run)
- Validation is eager, at the end of `Load`, so a bad file stops the daemon at
  startup instead of surfacing on first access
- The package holds no cache: `main` calls `Load` once and every component
  shares that one `*Config`

---

### internal/hue - Hue API Client

**Purpose:** Wrapper around openhue-go library for REST API communication.

**Key Types:**
```go
type Client struct {
    client     *openhue.ClientWithResponses
    bridgeAddr string
    apiKey     string
    ctx        context.Context
    connStatus ConnectionStatus
    connMutex  sync.RWMutex  // Protects connStatus
    limiter    *rate.Limiter // rate.NewLimiter(10, 20): 10 req/sec, burst 20
}

type ConnectionStatus struct {
    Connected   bool      // Is bridge reachable?
    LastError   string    // Last error message
    LastAttempt time.Time // Last connection attempt
    BridgeAddr  string    // Bridge IP
}

type Scene struct {
    ID             string // Scene UUID
    Name           string // Scene name
    Room           string // Room UUID
    RoomName       string // Room name (for display)
    GroupedLightID string // The room's grouped light, for power and brightness
}
```

**Connection Management:**
- `NewClient` builds the client and records a first reachability result; there
  is no separate connect step
- Every call updates `connStatus`, which `GetConnectionStatus` exposes over DBus
- Rate limiting to prevent bridge overload (10 req/sec, burst 20)

**Key Functions:**
- `GetScenes()` - Fetch all scenes with room names
- `ActivateScene(id)` - Activate a scene
- `SetLightPower(id, on)` - Control power
- `SetLightBrightness(id, brightness)` - Control brightness
- `GetGroupedLights()` - List rooms/zones
- `IsReachable()` - Test bridge connectivity

**Error Handling:**
- Network errors are captured into `connStatus` and returned to the caller
- No retry: a failed REST call fails. `RetryConnection` and
  `TestBridgeConnection` are the DBus methods that re-probe on demand
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

    // Cancels the supervisor watching the session gaming mode started. Only
    // set while gamingModeActive, cleared whenever that goes false, all under
    // mu, so a supervisor never outlives its game.
    stopGamingSupervisor context.CancelCauseFunc

    mu sync.RWMutex // Guards the gaming state; config has its own lock

    setGamingModeMu sync.Mutex // Serialises SetGamingMode's write with its detector transition
    ownerUID        uint32     // Service owner UID
    callerUID       func(dbus.Sender) (uint32, error) // Resolves sender to UID; nil fails closed
}
```

**Access Control:**
- UID-based filtering (only same-user processes)
- `checkAccess(sender)` verifies caller UID
- Reads that return bridge or bridge-derived data require owner; local config/state reads that expose no bridge resource identifiers or bridge state (e.g. GetStatus, GetSyncSettings) do not - GetStatus additionally reveals only whether bridge credentials are configured
- Write methods require owner UID match

**Method Categories** (27 methods; see `API_REFERENCE.md` for signatures):
1. **Status:** GetStatus, GetConnectionStatus, RetryConnection, TestBridgeConnection
2. **Scenes:** GetScenes, ActivateScene, GetStartupScene, SetStartupScene
3. **Lights:** SetPower, SetBrightness, GetState, GetGroupedLights
4. **Sync:** StartSync, StopSync, IsSyncing, GetSyncSettings, SetSyncSettings, ResetCaptureSource
5. **Gaming:** SetGamingMode, IsGamingModeEnabled, IsGamingModeActive
6. **Config:** SetGroupedLight, GetBridgeSettings, GetSelectedRoom, SetSelectedRoom
7. **Tray:** GetTrayIcons, SetTrayIcons

**Introspection:**
- `introspectionMethods()` in the same file returns a `[]introspect.Method`
  literal; godbus renders the XML at call time. There is no XML in the tree
- Compatible with d-feet and dbus-send
- A new method has to be added there as well as implemented, or it works over
  dbus-send and is invisible to introspection

---

### internal/sync - Screen Sync Engine

**Purpose:** Orchestrate screen capture, color extraction, and Entertainment API streaming.

**Key Types:**
```go
type Engine struct {
    config   *config.Config
    capturer *capture.ScreenCapture
    client   *entertainment.Client

    mu      sync.RWMutex
    running bool
    cancel  context.CancelFunc

    // Bumped per session so a loop winding down can only stop the session it
    // belongs to, never one started while it was stopping.
    generation uint64

    // The next session reuses the capturer and client, so a stop waits for
    // the loop to leave them before tearing them down.
    syncLoopWg sync.WaitGroup

    // Why the last session ended, or nil if it was stopped deliberately. A
    // caller that restarts sync needs the two apart.
    lastFailure error

    // Atomic so a settings change never blocks on e.mu, which Start holds
    // across the portal dialog and the DTLS connect.
    fps        atomic.Int64
    zones      []color.Zone
    httpClient *http.Client

    metrics PerformanceMetrics
}

// Counters, not a snapshot: percentiles are derived from the histogram when a
// metrics line is logged, and nothing exposes them as fields.
type PerformanceMetrics struct {
    mu sync.RWMutex
    metricsData
}

type metricsData struct {
    frameCount       uint64
    startTime        time.Time
    lastLogTime      time.Time
    totalFrameTime   time.Duration
    totalCaptureTime time.Duration
    totalExtractTime time.Duration
    totalStreamTime  time.Duration
    framesDropped    uint64
    frameTimes       [100]int // 0-99ms buckets
}
```

**Sync Loop** (`streamFrames`, abridged):
```go
func (e *Engine) streamFrames(ctx context.Context) error {
    ticker := time.NewTicker(time.Second / time.Duration(e.fps.Load()))
    defer ticker.Stop()

    // Copied once per session rather than read per frame, where View would
    // wait out every Save's disk write.
    var subsampleWidth, metricsInterval int
    e.config.View(func(c *config.Config) {
        subsampleWidth = c.Sync.SubsampleWidth
        metricsInterval = c.Sync.MetricsInterval
    })

    // One extractor per session. The gamma is hardcoded here; see
    // channels[].gammaFactor under Validation Rules.
    extractor, err := color.NewExtractor(subsampleWidth, 2.2)
    if err != nil {
        return err
    }

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            // An FPS change resets the ticker before the error paths below,
            // so a settings change lands even while capture is failing.

            frame, err := e.capturer.CaptureFrame()
            if err != nil {
                // Capture is over: every later tick would stream the same
                // frozen frame while IsRunning reported a live session.
                if errors.Is(err, capture.ErrCaptureStopped) {
                    return err
                }
                // Logged at most once every 5s, then keep going.
                continue
            }

            zoneColors, err := extractor.ExtractColors(frame, e.zones)
            capture.PutImageBuffer(frame) // Straight back to the pool
            if err != nil {
                continue
            }

            // 8-bit means times 257, not shifted: 255 has to reach 65535.
            channelColors := ...
            if err := e.client.StreamColors(channelColors); err != nil {
                // Logged and dropped. There is no reconnect.
            }

            e.updateMetrics(frameTime, captureTime, extractTime, streamTime)
        }
    }
}
```

**Giving Up:** the engine has no breaker of its own. Capture owns that decision
(see [internal/capture](#internalcapture---screen-capture)) and reports it as
`capture.ErrCaptureStopped`, which is the one error that ends the session here.
Every other error is logged and the loop continues.

**Performance Monitoring:**
- Frame time tracking, split into capture, extract and stream
- Latency percentiles (p50, p95, p99), computed from a 100-bucket millisecond
  histogram when a line is logged, not stored
- Dropped frames counted from elapsed wall-clock time against the target
  interval, because `time.Ticker` buffers only one pending tick
- Logged every `sync.metricsInterval` seconds (default 60, 0 turns it off) and
  once more when a session stops

---

### internal/capture - Screen Capture

**Purpose:** Capture screen content using native PipeWire integration.

**Key Types:**
```go
// The portal session is not a type of its own; its state lives here.
type ScreenCapture struct {
    conn          *dbus.Conn // Session bus, for the portal
    sessionHandle string     // Portal session object path
    streamNode    uint32     // PipeWire node ID
    fps           int
    fpsMutex      sync.RWMutex
    ctx           context.Context
    cancel        context.CancelFunc

    gstCmd           *exec.Cmd
    nativeCapture    *NativePipeWireCapture
    frameBuffer      *image.RGBA  // The published frame, replaced by publishFrame
    frameMutex       sync.RWMutex // Write side drains readers before recycling
    useMockFrames    bool
    useNativeCapture bool
    frameReaderWg    sync.WaitGroup
    consecutiveErrs  int   // Attempts in the current failure streak
    captureErr       error // Why capture gave up; cleared by Start

    useScreenshot  bool
    screenshotTool string
    captureWidth   int
    captureHeight  int

    restoreToken string
    tokenMutex   sync.Mutex

    // Bumped by ClearRestoreToken so a start already waiting on the portal
    // dialog cannot save its grant over a reset the user was told had worked.
    tokenGeneration uint64

    onTokenUpdate func(newToken string)
}
```

The sync engine always asks for native PipeWire at full resolution. `Config`
also selects a mock gradient, a screenshot tool (spectacle, grim or import) and
a GStreamer path, which only the `cmd/test-*` tools reach.

**Native PipeWire Integration (CGo):**
```c
// pipewire_native.h - the whole C interface
struct user_data *pw_stream_connect_to_node(uint32_t node_id);
int pw_start_loop(struct user_data *ud);
void pw_stop_loop(struct user_data *ud);
void pw_cleanup(struct user_data *ud);
int pw_get_frame(struct user_data *ud, uint8_t **data, int *width, int *height,
                 int *stride, uint32_t *format, struct frame_status *status);
```

**XDG Desktop Portal Flow:**
1. Request screen sharing permission, passing any saved restore token
2. User approves via GUI dialog, or the token skips it
3. Portal returns PipeWire node ID
4. Connect to PipeWire stream
5. Capture frames via native CGo

The stream is connected with `PW_STREAM_FLAG_MAP_BUFFERS` and each frame is
copied out of the mapped buffer. DmaBuf buffers arrive unmapped unless the
producer marks them mappable, and the handler rejects those rather than reading
them, so this path is not zero-copy.

**Buffer Reuse:**
- A `sync.Pool` of RGBA buffers (`GetImageBuffer`/`PutImageBuffer`), not one
  persistent buffer. A single buffer was published to another goroutine while
  being rewritten and was removed as a data race
- `GetImageBuffer` allocates when the pooled buffer is the wrong bounds, so a
  resolution change costs one allocation rather than corrupting a frame
- 91% reduction in memory allocations; prevents GC pressure
- Keep the Get/Put pairs balanced. `sync/engine.go` puts each frame back as
  soon as extraction is done with it

**Giving Up on Capture:**
- `captureBreakerTripped(firstErrAt, now, grace)` is a deadline, not an error
  count: this loop's period follows the configured FPS, so counting ticks would
  give 0.5s of grace at 60 FPS and 3s at 10
- `captureGrace`, 5s, measured from the first error of the current streak,
  whether or not a frame has ever arrived. A frame clears the streak, and so do
  `ErrNoNewFrame` (a still screen publishes nothing, which is not a fault) and
  `ErrAwaitingFirstFrame`
- `firstFrameGrace`, 6s, measured from the start of the loop and only checked
  while no frame has arrived yet. It is the backstop for the paths that keep
  clearing the streak: a stream that sits in awaiting-first-frame, or one
  bouncing between that and hard errors, would otherwise never accumulate 5s of
  continuous failure however long the dead air ran
- Continuous hard errors from startup therefore stop capture at 5s, not 6s
- `ErrStreamFailed` skips the deadline and stops immediately, since a failed
  stream never recovers
- Each of those records why through `failCapture`, behind `ErrCaptureStopped`,
  which is what ends the sync session. A plain `Stop` cancels the context and
  records nothing, which is why `Start` clears `captureErr`

**Portal Timeout:**
- 2-minute timeout on permission dialog
- Prevents indefinite hangs
- The DBus service formats portal failures as `PortalError:<type>:<hint>`, so a
  timeout reaches the tray as `PortalError:timeout:...`

---

### internal/entertainment - Entertainment API

**Purpose:** Stream color data to Hue bridge via DTLS-encrypted UDP.

**Key Types:**
```go
type Client struct {
    bridgeIP        string
    username        string // The API key, sent as the PSK identity hint
    clientKey       string // Hex; decoded to the PSK bytes
    entertainmentID string // Goes in every packet header

    conn         net.Conn
    sequenceID   uint8
    channelCount int

    mu        sync.RWMutex
    connected bool
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
│ Header (52 bytes)                            │
│  - Protocol name: "HueStream" (9 bytes)      │
│  - Version: 0x02 0x00 (2 bytes)              │
│  - Sequence number (1 byte)                  │
│  - Reserved (2 bytes)                        │
│  - Color space: 0x00 = RGB (1 byte)          │
│  - Reserved (1 byte)                         │
│  - Entertainment configuration ID (36 bytes, │
│    the UUID as ASCII, hyphens included)      │
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

**Activation Flow** (`connectStream` in `sync/engine.go`):
1. PUT `/clip/v2/resource/entertainment_configuration/{id}` with
   `{"action":"start"}`. Errors are read from the response body's `errors`
   array, not from the HTTP status
2. Wait 100ms for the bridge to open UDP 2100
3. Establish the DTLS connection
4. Stream color packets at target FPS

An activation the bridge rejected is logged and the connect is tried anyway,
once. A connect refused after an activation the bridge accepted is retried every
500ms for up to 10s, re-activating the area each time, since only an accepted
activation reopens the port.

Stopping sync closes the DTLS connection and stops sending. It does not PUT
`{"action":"stop"}`: nothing in the tree does. The bridge drops out of streaming
mode on its own once the packets stop.

**Error Handling:**
- Once connected there is no reconnect: a failed write is logged by the sync
  loop and that frame is dropped
- `Close` is the only teardown; the area is left to time out

---

### internal/color - Color Processing

**Purpose:** Extract colors from screen zones using UV coordinates.

**Key Types:**
```go
type Extractor struct {
    subsampleWidth  int     // Target samples across a zone (16-256, default 64)
    gammaCorrection float64 // One value for every zone
}

type Zone struct {
    ID   int     // Becomes the Entertainment API channel ID
    U1   float64 // Left   (0.0-1.0)
    V1   float64 // Top
    U2   float64 // Right
    V2   float64 // Bottom
    Name string
}

type ZoneColor struct {
    ZoneID  int
    R, G, B uint8
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

**Stride-Based Sampling** (`extractZoneColor`, abridged):
```go
func (e *Extractor) extractZoneColor(img image.Image, zone Zone) (ZoneColor, error) {
    bounds := img.Bounds()

    x1 := int(zone.U1 * float64(bounds.Dx()))
    y1 := int(zone.V1 * float64(bounds.Dy()))
    x2 := int(zone.U2 * float64(bounds.Dx()))
    y2 := int(zone.V2 * float64(bounds.Dy()))

    // ... each then clamped to the image, and x2/y2 to at least x1+1/y1+1, so
    // a zone always covers at least one pixel.

    // The same stride on both axes, so the sample count follows the zone's
    // aspect ratio rather than being subsampleWidth squared.
    stride := max(1, (x2-x1)/e.subsampleWidth)

    r, g, b := e.calculateMeanColorWithStride(img, x1, y1, x2, y2, stride)

    return ZoneColor{zone.ID, e.applyGamma(r), e.applyGamma(g), e.applyGamma(b)}, nil
}
```

`calculateMeanColorWithStride` has two paths. An `*image.RGBA` whose bounds
cover the zone, which is what capture produces, is read straight out of `Pix`
by `meanRGBAWithStride`. Anything else goes through `img.At`, which boxes a
`color.Color` per sample.

**Gamma Correction:**
- `applyGamma` computes `math.Pow(v/255, 1/gamma) * 255` per channel, per zone,
  per frame. There is no lookup table
- One gamma per extractor, and the sync engine hardcodes 2.2 when it builds one
- `channels[].gammaFactor` is validated to 0.5-4.0 and then never read; see
  Validation Rules

**Performance:**
- 41% faster than Lanczos resampling
- The frame is never resampled or copied; zones are sampled in place
- Cache-friendly access patterns: the fast path walks `Pix` by offset

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
│  • HueBackend async call│
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
│  • HueBackend async call│
│    StartSync()          │
└────────────┬────────────┘
             │
             │ DBus
             ▼
┌────────────────────────────────────────────────────────────┐
│   Backend: Sync Engine Start                              │
│  1. Initialize PipeWire capture                           │
│  2. Request screen sharing permission (XDG Portal)        │
└────────────┬───────────────────────────────────────────────┘
             │
             │ XDG Desktop Portal
             ▼
┌────────────────────────────────────────────────────────────┐
│   System: Permission Dialog                               │
│   [Allow screen sharing? Select monitor]                  │
│   A saved sync.restoreToken skips this                    │
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
             │ Capture running
             ▼
┌────────────────────────────────────────────────────────────┐
│   Backend: Open the Entertainment stream                  │
│  3. Activate Entertainment Area (HTTPS PUT to bridge)     │
│  4. Connect DTLS to bridge:2100                           │
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
║  │    • Copy into a buffer from the pool            │     ║
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
║  │    • Runs inside step 2, per zone mean            │     ║
║  │    • One gamma for every zone, fixed at 2.2      │     ║
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
║  │    • Record frame, capture, extract, stream time  │     ║
║  │    • Add the frame time to the histogram          │     ║
║  │    Time: <1ms                                     │     ║
║  └──────────────────────────────────────────────────┘     ║
║                                                            ║
║  Total: ~15ms average (well under 33ms budget)            ║
║                                                            ║
║  ┌─────────────────────────────────────────────────┐      ║
║  │ Wait for the next tick. A tick missed while the  │      ║
║  │ frame was in flight is counted as a drop, not    │      ║
║  │ run late: time.Ticker buffers only one           │      ║
║  └─────────────────────────────────────────────────┘      ║
║                                                            ║
║  Loop until StopSync() called                             ║
╚════════════════════════════════════════════════════════════╝
```

**Performance Budget (30 FPS = 33ms per frame):**
- Capture: 2ms (6%)
- Color extraction, gamma included: 11ms (33%)
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

func (s *Service) GetScenes(sender dbus.Sender) ([]string, *dbus.Error) {
    // Scenes are bridge data, so they need the owner check. No mutex: the
    // hue client has its own, and nothing here touches Service state.
    if err := s.checkAccess(sender); err != nil {
        return nil, dbus.MakeFailedError(err)
    }
    // hue.Client returns []Scene; this formats the display names.
    ...
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
│   INITIALIZING       │ Setting up PipeWire
│   • Portal request   │
│   • User approval    │
│   • Frame reader up  │
└──────┬───────────────┘
       │
       │ Capture started
       ▼
┌──────────────────────┐
│   ACTIVATING         │ Activating Entertainment Area
│   • PUT to bridge    │
│   • Check body errors│
│   • DTLS connect     │
└──────┬───────────────┘
       │
       │ Stream ready
       ▼
┌──────────────────────┐
│   RUNNING            │ Sync loop active
│   • Capture frames   │ ◄─────┐
│   • Extract colors   │       │ Every 33ms (30 FPS)
│   • Stream to bridge │───────┘
└──────┬───────────────┘
       │
       │ StopSync(), or capture gives up
       ▼
┌──────────────────────┐
│   STOPPING           │
│   • Cancel the loop  │
│   • Stop capture     │
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
- **CAPTURE_ERROR:** frame capture failed. The loop logs and continues until
  capture gives up on its own deadline and returns `ErrCaptureStopped`, which
  ends the session
- **STREAM_ERROR:** Entertainment API send failed. Logged, frame dropped, loop
  continues
- **PORTAL_ERROR:** user cancelled or timeout. `StartSync` fails; no session
  starts

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
│  In C, on the PipeWire loop thread:                       │
│  1. pw_stream_dequeue_buffer(stream)                      │
│  2. Reject: no datas, unmapped (DmaBuf), CORRUPTED chunk  │
│  3. Bound offset and size by the mapping, not the         │
│     producer's claims about it                            │
│  4. memcpy into the frame slot                            │
│  5. pw_stream_queue_buffer(stream, buffer)                │
│                                                            │
│  In Go, on the reader loop:                               │
│  6. pw_get_frame, then convert to RGBA into a buffer      │
│     taken from the pool                                    │
│  7. publishFrame swaps it in under the write lock and     │
│     returns the old one to the pool                       │
│                                                            │
│  - Mapped buffers and a memcpy, not zero-copy             │
│  - Buffers come from a pool, not one persistent buffer    │
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
│       stride = max(1, zone_width / subsampleWidth)        │
│       (subsampleWidth defaults to 64, so ~64 samples      │
│        across the zone and as many rows as fit)           │
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
│  - No image resampling (41% faster)                       │
│  - Cache-friendly linear access                           │
│  - Configurable accuracy vs performance                   │
└────────────────────────────────────────────────────────────┘
```

**Stage 3: Gamma Correction (<1ms)**

```
┌────────────────────────────────────────────────────────────┐
│  Gamma Correction                                          │
│                                                            │
│  Computed per channel, per zone, per frame:               │
│    corrected = pow(mean/255, 1/gamma) * 255               │
│                                                            │
│  - No lookup table. Three math.Pow calls per zone, on     │
│    means already reduced from the whole zone              │
│  - One gamma for every zone, hardcoded to 2.2 by the      │
│    sync engine                                             │
└────────────────────────────────────────────────────────────┘
```

**Stage 4: Entertainment API Streaming (1ms)**

```
┌────────────────────────────────────────────────────────────┐
│  DTLS Packet Construction & Send                          │
│                                                            │
│  Packet structure:                                         │
│    Header (52 bytes):                                      │
│      [0-8]: "HueStream"                                    │
│      [9-10]: Version (0x02 0x00)                           │
│      [11]: Sequence number (uint8, wraps)                  │
│      [12-13]: Reserved (0x00)                              │
│      [14]: Color space (0x00 = RGB)                        │
│      [15]: Reserved (0x00)                                 │
│      [16-51]: Entertainment configuration UUID, ASCII      │
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
│  - Low latency (UDP, no ACK wait)                         │
│  - Encrypted (DTLS 1.2, PSK, no certificates)             │
│  - A failed write drops the frame; no retry, no reconnect │
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
- Added: capture stops itself rather than failing forever
- Prevents: Infinite error loops
- Logs: Clear message when it gives up
- Since replaced by a deadline. The count this shipped with gave 0.5s of grace
  at 60 FPS and 3s at 10; it is now 5s from the first error of a streak, with a
  6s backstop from loop start for a stream that never delivers

**After Portal Timeout (PR #47):**
- Added: 2-minute timeout on permission dialog
- Prevents: Indefinite hangs
- Returns: Clear error message

**Current Performance (PRs #42-50):**
- 30.0 FPS sustained
- 11ms average frame time
- 80 MB/sec allocations
- No dropped frames
- Robust error handling

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
│ - Bridge reserves Entertainment API for this client           │
│ - Normal light control now blocked (Entertainment mode)       │
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
│ Step 5: Stop                                                   │
│                                                                │
│ - Sync loop cancelled, capture stopped                         │
│ - DTLS connection closed                                       │
│                                                                │
│ No PUT is sent. Nothing in the tree sends {"action":"stop"},   │
│ so the bridge leaves the area active until it times the        │
│ stream out on its own, and normal light control is blocked     │
│ for that window.                                               │
└────────────────────────────────────────────────────────────────┘
```

### Entertainment API Limitations

**While Active:**
- Cannot activate scenes via REST API
- Cannot control lights via REST API
- Cannot change brightness/color via REST API
- Can read light state (GET requests still work)
- Can control via Entertainment API streaming

**Connection Requirements:**
- Must have `clientkey` (obtained during bridge pairing, see
  `cmd/register-entertainment`)
- Must have `entertainmentConfigurationId` (Entertainment area UUID, see
  `cmd/get-entertainment-info`)
- Must activate the area before streaming
- The app never deactivates it. The bridge ends the session itself once the
  packets stop

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

# Scene activated each time the backend starts (optional, empty disables)
startupScene: ""

# Screen sync settings
sync:
  enabled: false
  fps: 30                    # 10-60
  subsampleWidth: 64         # 16-256
  restoreToken: ""           # Portal session token
  metricsInterval: 60        # 0-3600 seconds, 0 = off

# Gaming mode settings
gamingMode:
  enabled: false
  pollInterval: 2            # 1-30 seconds
  debounceDelay: 5           # 0-60 seconds, 0 fires on the next poll
  useSystemdInhibit: true    # CachyOS primary
  usePowerProfile: true      # CachyOS secondary
  useSteamAppId: true        # Steam games
  useGameMode: false         # Feral GameMode

# Entertainment API channel mapping
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2          # Validated, never read
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

# Logging level. Nothing reads it. The backend logs through the standard
# library's log package, which has no levels; Save writes the key back so it
# survives a round trip.
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
│ 5. Check file permissions                                  │
│    • Warn if any group or other read bit is set            │
│    • Contains sensitive API keys                           │
│                                                            │
│ 6. Unmarshal into struct                                   │
│    • viper.Unmarshal(&cfg)                                 │
│    • Merge with defaults                                   │
│                                                            │
│ 7. Clamp a legacy sync.fps of 1-9 up to 10, in memory      │
│    • That range was valid once. Failing Load instead       │
│      would stop a daemon that used to start                │
│    • Not written back: Save rewrites a file openhue-cli    │
│      also reads                                            │
│                                                            │
│ 8. Validate configuration                                  │
│    • Check required fields                                 │
│    • Validate ranges                                       │
│    • Validate UV coordinates                               │
│    • Any failure fails Load, which stops the daemon        │
│                                                            │
│ 9. Return config                                           │
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
- `sync.metricsInterval` - Range: 0-3600 seconds, 0 turns the line off

**Gaming Mode:**
- `gamingMode.pollInterval` - Range: 1-30 seconds. It becomes the monitor
  loop's ticker interval, and `time.NewTicker` panics on a non-positive one.
  The ceiling also keeps `time.Duration(n) * time.Second` inside int64
- `gamingMode.debounceDelay` - Range: 0-60 seconds, 0 triggers on the next poll
- Both are checked whatever `gamingMode.enabled` says: `SetGamingMode` starts
  the detector at runtime from the config already in memory
- All `use*` flags - Boolean

**Channels:**
- `id` - Range: 0-255 (it is a `uint8`, so nothing else fits)
- Two active channels may not share an `id`: one light driven from two zones
- `active` - Boolean
- `gammaFactor` - Range: 0.5-4.0, and then never read. The sync engine builds
  its extractor with a hardcoded 2.2, so setting this changes nothing
- `uvA.x`, `uvA.y` - Range: 0.0-1.0
- `uvB.x`, `uvB.y` - Range: 0.0-1.0
- `uvA.x < uvB.x` - UVA must be top-left
- `uvA.y < uvB.y` - UVB must be bottom-right
- An empty `deviceName` on an active channel warns and is accepted

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

**3. System Errors**
- Network timeout, bridge temporarily unavailable: no retry. The REST call
  fails, `connStatus` records why, and `RetryConnection` re-probes on demand
- Bridge refusing UDP 2100 at session start: retried every 500ms for up to 10s.
  A fixed interval, not a backoff, and only for the initial connect
- PipeWire stream failure: `ErrStreamFailed` stops capture at once, since a
  failed stream never recovers

**4. Fatal Errors (Not Recoverable)**
- Config file corruption
- DTLS handshake failure
- Capture giving up on its deadline
- **Response:** Log error, stop operation

### Error Handling Patterns

**DBus Error Wrapping:**
```go
// Backend returns dbus.Error
func (s *Service) ActivateScene(displayName string, sender dbus.Sender) (string, *dbus.Error) {
    if err := s.checkAccess(sender); err != nil {
        return "", dbus.MakeFailedError(err)
    }
    if err := common.ValidateDBusString("displayName", displayName, 255); err != nil {
        return "", dbus.MakeFailedError(err)
    }

    scene, err := s.activateSceneByDisplayName(displayName)
    if err != nil {
        return "", dbus.MakeFailedError(err)
    }

    // Point brightness and power at the same room the scene just lit.
    if scene.GroupedLightID != "" {
        s.config.Update(func(c *config.Config) {
            c.GroupedLightID = scene.GroupedLightID
        }, nil)
    }

    return "Scene activated: " + displayName, nil
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

**Giving Up on Capture:**
```go
// internal/capture/capture.go. A deadline, not a count: the reader loop's
// period follows the configured FPS, so a fixed count would give 0.5s of
// grace at 60 FPS and 3s at 10.
func captureBreakerTripped(firstErrAt, now time.Time, grace time.Duration) bool {
    if firstErrAt.IsZero() {
        return false
    }
    return now.Sub(firstErrAt) >= grace
}

// In nativeFrameReaderLoop:
//   captureGrace    = 5s from the first error of the current streak, with or
//                     without a frame so far. A frame, ErrNoNewFrame and
//                     ErrAwaitingFirstFrame each clear the streak
//   firstFrameGrace = 6s from loop start, checked only while no frame has
//                     arrived. The backstop for the paths that keep clearing
//                     the streak
//   ErrStreamFailed skips both and stops now: a failed stream never recovers
//
// Each of those calls failCapture, which stores the cause behind
// ErrCaptureStopped. A plain Stop cancels the context and records nothing,
// which is why Start clears captureErr.
```

### Validation Error Messages

Config validation carries the offending value and a hint:
```
"channel 0: uvA.x must be 0.0-1.0 (got 1.50)
   → UV coordinates represent screen position as fractions
   → 0.0 = left/top edge, 1.0 = right/bottom edge"

"bridge IP not configured
   → Add 'Bridge: YOUR_BRIDGE_IP' to /home/user/.openhue/config.yaml
   → You can discover your bridge with: openhue discover"
```

DBus argument validation does not. `SetBrightness` out of range returns the bare
`"brightness must be 0-100"`, with neither the value nor a hint.

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
- SetGroupedLight, SetSyncSettings, ResetCaptureSource
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
- RGBA buffer: 14.7 MB at 2560×1440 (width × height × 4 bytes). Three can be
  live at once while syncing: the published frame, the one `convertToRGBA` is
  filling for the next publish, and the copy the sync loop is extracting from

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
- Monolithic Qt application - Harder to test, tight coupling
- Electron app - Higher resource usage, worse performance
- Pure CLI tool - Less user-friendly, no tray integration

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
- Unix sockets - Manual access control, no introspection
- HTTP REST - Overkill, higher latency, port management
- Websockets - Complex, server management, overkill

---

### Why Native PipeWire?

**Decision:** Use native PipeWire integration via CGo instead of GStreamer.

**Rationale:**
1. **Performance:**
   - Lower latency than a GStreamer pipeline
   - Direct control over buffer reuse
   - One memcpy out of a mapped buffer per frame. DmaBuf is not used: the
     handler rejects unmapped buffers rather than importing them

2. **Simplicity:**
   - No external dependencies beyond libpipewire
   - Smaller attack surface
   - Easier to maintain

3. **Wayland Integration:**
   - Native Wayland screen capture
   - Works with XDG Desktop Portal
   - Future-proof (X11 being phased out)

**Alternatives Considered:**
- GStreamer - Higher latency, complex pipeline, larger dependencies
- FFmpeg - Overkill, not designed for live streaming
- X11 XGetImage - Doesn't work on Wayland, deprecated

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
- Pixel coordinates - Resolution-dependent, breaks on monitor change
- Percentage-based - Similar to UV but less standard
- Absolute positions - Not portable, config hell

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
- C++ - Manual memory management, slower compilation, more complexity
- Rust - Steeper learning curve, longer compile times, less mature DBus
- Python - Slower runtime, GIL limitations, poor concurrency

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
   - Qt6::DBus for async communication
   - `QDBusPendingCallWatcher` behind a hand-written proxy, because
     `QDBusInterface` introspects in its constructor and blocks on a hung
     backend
   - Well-documented

**Alternatives Considered:**
- GTK - Not native to KDE, inconsistent UI
- Electron - Massive resource usage, slow startup
- Pure C++ - Qt provides better abstractions

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

### Settings Dialog

Built. `trayapp/settingsdialog.{h,cpp}`, opened from the tray menu, holding its
own `HueBackend` proxy rather than sharing `main.cpp`'s.

```
┌───────────────────────────┐
│   Settings Dialog (Qt)    │
│  • Bridge configuration   │
│  • Room and startup scene │
│  • FPS and subsample width│
│  • Gaming mode toggle     │
│  • Tray icon names        │
└─────────────┬─────────────┘
              │ Qt6::DBus, async
              ▼
┌──────────────────────────────────┐
│  Backend DBus Interface          │
│  Reads:  GetBridgeSettings       │
│          GetGroupedLights        │
│          GetScenes               │
│          GetSelectedRoom         │
│          GetStartupScene         │
│          GetSyncSettings         │
│          GetTrayIcons            │
│          IsGamingModeEnabled     │
│  Writes: SetSyncSettings         │
│          SetGamingMode           │
│          SetSelectedRoom         │
│          SetStartupScene         │
│          SetTrayIcons            │
│  Acts:   RetryConnection         │
│          TestBridgeConnection    │
│          ResetCaptureSource      │
└──────────────────────────────────┘
```

There is no `GetConfig`/`SetConfig` pair: each setting has its own method, so
the dialog never round-trips the whole config.

**Still missing:**
- Visual zone editor (drag rectangles on a screenshot)
- Real-time preview (see colors per zone)
- Bridge discovery and pairing wizard

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
- **Development:** See `../DEVELOPMENT.md` for building and testing

---

**Last Updated:** September 2026
**Architecture Version:** 1.0
**Backend Version:** Compatible with khuey backend v1.x
