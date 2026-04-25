# KDE Hue Control - Architecture

This document provides a comprehensive overview of the KDE Hue Control architecture, component interactions, and design decisions.

## Table of Contents

1. [System Overview](#system-overview)
2. [Component Architecture](#component-architecture)
3. [Data Flow](#data-flow)
4. [Communication Patterns](#communication-patterns)
5. [Screen Sync Pipeline](#screen-sync-pipeline)
6. [Performance Characteristics](#performance-characteristics)
7. [Design Decisions](#design-decisions)

---

## System Overview

KDE Hue Control is a **three-component system** that bridges KDE Plasma with Philips Hue lighting:

```
┌─────────────────┐     DBus Session Bus      ┌──────────────────┐
│   Qt6 Tray App  │◄──────────────────────────►│   Go Backend     │
│   (hue-tray)    │  org.kde.plasma.hue        │   (hue-sync)     │
│                 │                             │                  │
│  • UI/Controls  │                             │  • Config        │
│  • User Input   │                             │  • Hue API       │
│  • Notifications│                             │  • Screen Sync   │
│  • Tray Icon    │                             │  • Gaming Mode   │
└─────────────────┘                             └────────┬─────────┘
                                                         │
                                                         │ HTTPS/UDP
                                                         │
                                                         ▼
                                                ┌─────────────────┐
                                                │   Hue Bridge    │
                                                │                 │
                                                │  • REST API     │
                                                │  • Entertainment│
                                                │    API (DTLS)   │
                                                └─────────────────┘
```

### Components

1. **Tray Application** (`trayapp/`): Qt6/C++ GUI for user interaction
2. **Backend Service** (`backend/`): Go daemon handling all business logic
3. **Hue Bridge**: Philips Hue hardware gateway (external)

---

## Component Architecture

### Backend Service (`backend/`)

The Go backend is structured as a modular service with clean separation of concerns:

```
backend/
├── cmd/
│   ├── hue-sync/          # Main service binary
│   ├── profile-sync/      # Performance profiling tool
│   └── test-*/            # Testing utilities
├── internal/
│   ├── config/            # YAML configuration management
│   ├── hue/               # Hue API client wrapper
│   ├── dbus/              # DBus service implementation
│   ├── sync/              # Screen sync engine
│   ├── capture/           # Screen capture (Wayland/PipeWire)
│   ├── entertainment/     # DTLS Entertainment API client
│   ├── color/             # Color extraction and processing
│   └── gaming/            # Gaming mode detection
└── vendor/                # Vendored dependencies
```

#### Key Packages

**`internal/dbus/`** - DBus Service
- Exposes `org.kde.plasma.hue` interface
- Methods: GetStatus, GetScenes, ActivateScene, StartSync, StopSync, etc.
- UID-based access control for security
- Input validation on all external data

**`internal/sync/`** - Sync Engine
- Orchestrates screen capture → color extraction → streaming
- Performance monitoring with detailed metrics
- Circuit breaker for error recovery (stops after 30 consecutive errors)
- Runs at configurable FPS (default 30)

**`internal/capture/`** - Screen Capture
- Native PipeWire integration via CGo
- XDG Desktop Portal for permission dialogs
- 2-minute timeout on permission dialog
- Reusable RGBA buffer (eliminates 12.4GB allocations)

**`internal/color/`** - Color Processing
- UV-based zone mapping (monitor-agnostic)
- Stride-based sampling (41% faster than resampling)
- Gamma correction per channel
- Mean color calculation per zone

**`internal/entertainment/`** - Entertainment API
- DTLS 1.2 over UDP (github.com/pion/dtls)
- HueStream v2 protocol
- 16-bit RGB channels per light
- Automatic reconnection on errors

**`internal/gaming/`** - Gaming Detection
- Monitors game processes via CachyOS DB
- Automatic sync enable/disable
- Buffered channels to prevent blocking
- Configurable polling interval

### Tray Application (`trayapp/`)

Qt6/C++ application using KDE Frameworks:

```
trayapp/
├── main.cpp               # Entry point, tray icon, menu
├── CMakeLists.txt         # Build configuration
└── (Future: settings dialog, zone editor)
```

**Key Technologies:**
- **KStatusNotifierItem**: KDE6 system tray integration (not QSystemTrayIcon)
- **QDBus**: Async DBus communication with backend
- **KNotification**: Desktop notifications
- **Qt6 Widgets**: UI framework

---

## Data Flow

### Scene Activation Flow

```
1. User clicks scene in tray menu
   ↓
2. Tray app: QDBusInterface.call("ActivateScene", sceneId)
   ↓
3. Backend: DBus service receives call
   ↓
4. Backend: hue.Client.ActivateScene(sceneId)
   ↓
5. Backend: HTTPS POST to bridge
   ↓
6. Bridge: Applies scene to lights
   ↓
7. Backend: Returns success/error via DBus
   ↓
8. Tray app: Shows notification (KNotification)
```

### Screen Sync Flow

```
1. User clicks "Start Screen Sync"
   ↓
2. Tray app: QDBusInterface.call("StartSync")
   ↓
3. Backend: Activates Entertainment Area
   ↓
4. Backend: Initiates PipeWire capture via XDG Portal
   ↓
5. System: Shows permission dialog (user must approve)
   ↓
6. User: Approves screen sharing
   ↓
7. PipeWire: Streams video frames to backend
   ↓
8. Backend: [SYNC LOOP - runs at 30 FPS]
   │
   ├─ Capture frame from PipeWire (native CGo)
   │  ↓
   ├─ Extract colors for each UV zone (stride sampling)
   │  ↓
   ├─ Apply gamma correction per channel
   │  ↓
   ├─ Stream to Entertainment API via DTLS
   │  ↓
   └─ [Repeat every 33ms for 30 FPS]
```

---

## Communication Patterns

### DBus Interface Specification

**Service Name:** `org.kde.plasma.hue`
**Object Path:** `/org/kde/plasma/hue`
**Interface:** `org.kde.plasma.hue`

**Methods:**

```
GetStatus() → string
  Returns: "Ready", "Not Configured", "Bridge Unreachable"

GetScenes() → []Scene
  Returns: Array of {Id: string, Name: string}
  Format: "Room Name - Scene Name" (sorted alphabetically)

ActivateScene(sceneId: string) → (success: string, error: *dbus.Error)
  Activates a scene by ID

SetPower(on: bool) → (success: bool, error: *dbus.Error)
  Controls power for grouped lights

SetBrightness(value: int) → (success: bool, error: *dbus.Error)
  Sets brightness (0-100) for grouped lights

StartSync() → (success: string, error: *dbus.Error)
  Starts screen synchronization

StopSync() → (success: string, error: *dbus.Error)
  Stops screen synchronization

IsSyncing() → bool
  Returns current sync status
```

**Async Pattern:**
- Tray app uses async DBus calls (non-blocking UI)
- Backend processes calls in goroutines
- Errors returned as `*dbus.Error` for proper propagation

---

## Screen Sync Pipeline

### Frame Processing Pipeline

Each frame goes through 4 stages:

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐     ┌────────────┐
│   Capture    │────→│   Extract   │────→│   Process    │────→│   Stream   │
│  (PipeWire)  │     │  (UV Zones) │     │  (Gamma)     │     │   (DTLS)   │
└──────────────┘     └─────────────┘     └──────────────┘     └────────────┘
   ~2ms               ~11ms               <1ms                ~1ms
```

### Detailed Stages

**1. Capture (2ms)**
- Native PipeWire capture via CGo (`libpipewire-0.3`)
- Direct memory access to video buffer (DMA-BUF)
- Reuses single RGBA buffer (no allocations)
- Returns `image.Image` interface

**2. Extract (11ms)**
- For each configured channel:
  - Calculate pixel zone from UV coordinates
  - Stride-based sampling (skip pixels for speed)
  - Calculate mean RGB value
- Optimized: No global resampling (eliminated 65% CPU overhead)

**3. Process (<1ms)**
- Apply gamma correction per channel (default 2.2)
- Convert RGB to Entertainment API format (16-bit)
- Pack into DTLS packet

**4. Stream (1ms)**
- Send via DTLS to Entertainment API
- UDP transport (low latency)
- Automatic retry on errors

### Performance Characteristics

**Target:** 30 FPS (33ms per frame)
**Achieved:** 30.0 FPS (11ms processing time)

**Latency Distribution:**
- p50 (median): 11ms
- p95: 15ms
- p99: 19ms
- Max: ~25ms (well under 33ms budget)

**Memory Usage:**
- Allocations: 80 MB/sec (down from 827 MB/sec)
- Buffer reuse eliminates 12.4GB allocations per 15 seconds

---

## Performance Characteristics

### Optimizations Implemented

1. **Stride-Based Sampling** (PR #42)
   - Replaced Lanczos resampling with direct pixel sampling
   - Skip pixels based on stride calculation
   - Result: 41% faster (19ms → 11ms frame time)

2. **Buffer Reuse** (PR #43)
   - Single RGBA buffer reused across all frames
   - Eliminates 28MB allocation per frame
   - Result: 91% fewer allocations

3. **Circuit Breaker** (PR #46)
   - Stop after 30 consecutive capture errors
   - Prevents infinite error loops
   - Logs clear message on circuit trip

4. **Portal Timeout** (PR #47)
   - 2-minute timeout on permission dialog
   - Prevents indefinite hangs
   - Returns clear error message

### Benchmarks

**Before Optimization:**
```
FPS: 27.5 (dropped frames)
Frame time: 19ms
Memory: 827 MB/sec allocations
CPU: 72% in imaging.resizeHorizontal
```

**After Optimization:**
```
FPS: 30.0 (perfect)
Frame time: 11ms (-41%)
Memory: 80 MB/sec allocations (-90%)
CPU: 5% in imaging operations (-67%)
```

---

## Design Decisions

### Why Three Components?

**Why not a monolithic application?**

1. **Separation of Concerns:**
   - Backend: Complex logic, system integration, network
   - Tray: Simple UI, user interaction
   - Clean interface via DBus

2. **Crash Isolation:**
   - Tray crash doesn't affect sync
   - Backend crash doesn't freeze UI

3. **Testability:**
   - Backend fully testable via DBus commands
   - No GUI required for integration tests

4. **Reusability:**
   - Backend usable from CLI, scripts, other apps
   - Tray replaceable with plasmoid or different UI

### Why DBus?

**Alternatives considered:** Websockets, Unix sockets, HTTP

**Chosen DBus because:**
- Native to Linux desktop environments
- Built-in access control (UID-based)
- Introspection support (auto-documentation)
- Standard for KDE/GNOME system services
- Tool support (`dbus-send`, `dbus-monitor`, `d-feet`)

### Why Native PipeWire?

**Alternatives considered:** GStreamer, FFmpeg, libav

**Chosen native PipeWire because:**
- Direct memory access (DMA-BUF) - zero-copy
- Lower latency than GStreamer pipeline
- No external dependencies beyond libpipewire
- Native Wayland integration
- Smaller attack surface

### Why UV Coordinates?

**Alternatives considered:** Pixel coordinates, percentage-based

**Chosen UV coordinates because:**
- Resolution-agnostic (works on any monitor)
- Standard in graphics (OpenGL, Vulkan, game engines)
- Easy multi-monitor support (extend U axis)
- Intuitive: (0,0) = top-left, (1,1) = bottom-right
- Survives monitor changes/rotation

### Why Go for Backend?

**Alternatives considered:** C++, Rust, Python

**Chosen Go because:**
- Excellent concurrency primitives (goroutines, channels)
- Fast compilation for rapid iteration
- Strong standard library (HTTP, TLS, context)
- Good performance (compiled, GC)
- Easy cross-platform builds
- Great DBus library (godbus)

### Why Qt/C++ for Tray?

**Alternatives considered:** GTK, Electron, pure C++

**Chosen Qt/KDE because:**
- Native KDE Plasma integration (KStatusNotifierItem)
- Mature QDBus for async communication
- KNotification for desktop notifications
- Consistent with KDE ecosystem
- High performance (native, not Electron)
- C++ familiar to KDE contributors

---

## Security Considerations

### DBus Access Control

- UID-based filtering: Only same-user processes
- Input validation: String length, UTF-8 encoding
- Rate limiting: 10 requests/sec to bridge

### Entertainment API

- DTLS 1.2 encryption
- Client key required (obtained during bridge pairing)
- Bridge validates client certificate

### Screen Capture

- User must explicitly approve via portal dialog
- Permission per-session (doesn't persist by default)
- Wayland security model (no X11-style global capture)

### Config File Security

- Stored at `~/.openhue/config.yaml`
- Permissions: 0600 (owner read/write only)
- Contains sensitive API keys
- Never logged or transmitted except to bridge

---

## Future Architecture Considerations

### Potential Multi-Monitor Support

```
channels:
  - id: 0
    deviceName: "Left Light"
    monitor: 0  # NEW: Monitor index
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}
  - id: 1
    deviceName: "Right Light"
    monitor: 1  # Different monitor
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}
```

### Potential Settings Dialog Architecture

```
┌───────────────────────────┐
│   Settings Dialog (Qt)    │
│  • Zone Editor (visual)   │
│  • Bridge Config          │
│  • Performance Tuning     │
└─────────────┬─────────────┘
              │ QDBus
              ▼
┌───────────────────────────┐
│  Backend DBus Interface   │
│  • GetConfig()            │
│  • SetConfig(yaml)        │
│  • ValidateConfig()       │
└───────────────────────────┘
```

### Potential Gaming Integration

Already implemented! See `internal/gaming/` for CachyOS-based game detection.

---

## Debugging Guide

### DBus Communication

```bash
# Monitor all DBus traffic
dbus-monitor --session "interface='org.kde.plasma.hue'"

# Introspect interface
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.freedesktop.DBus.Introspectable.Introspect

# Call method manually
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus
```

### Backend Logs

```bash
# View live logs
journalctl --user -u hue-backend -f

# View last 50 lines
journalctl --user -u hue-backend -n 50

# Search for errors
journalctl --user -u hue-backend | grep -i error
```

### Performance Profiling

```bash
# Profile screen sync for 15 seconds
cd backend
go run ./cmd/profile-sync -duration 15s

# Generate profiles
go tool pprof -http=:8080 cpu.prof
go tool pprof -http=:8081 mem.prof
```

---

## Contributing

When modifying the architecture:

1. **Maintain DBus stability**: Never break existing methods
2. **Update this doc**: Keep architecture current
3. **Test integration**: Use `scripts/test-integration.sh`
4. **Consider performance**: Profile before/after changes
5. **Document decisions**: Explain "why" in comments and docs

---

For more details on specific components:
- Development: See `DEVELOPMENT.md`
- Testing: See `TESTING.md`
- Contributing: See `CONTRIBUTING.md`
- Usage: See `USAGE.md`
