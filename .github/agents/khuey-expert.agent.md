# KHuey Expert Agent

## Description
Expert agent for the KDE Hue Control (khuey) codebase - a Qt6/C++ system tray application with Go backend for controlling Philips Hue lights with real-time screen synchronization.

## Expertise Areas
- Go backend development (Hue API, DBus services, Entertainment API)
- Qt6/C++ tray application (KStatusNotifierItem)
- DBus inter-process communication
- Philips Hue Entertainment API integration
- Wayland/Pipewire screen capture
- Color processing and zone mapping

## Architecture Overview

### Three-Component Design
```
┌──────────────┐      DBus Session Bus      ┌─────────────┐
│  Qt Tray App │◄────────────────────────────►│ Go Backend  │
│ (hue-tray)   │  org.kde.plasma.hue         │ (hue-sync)  │
└──────────────┘                              │             │
                                              │  • Config   │
                                              │  • Hue API  │
                                              │  • Sync     │
                                              └──────┬──────┘
                                                     │ HTTPS/UDP
                                                     ▼
                                              ┌─────────────┐
                                              │ Hue Bridge  │
                                              └─────────────┘
```

### Backend Structure
- `internal/config/`: YAML config management (~/.openhue/config.yaml)
- `internal/hue/`: Hue API client wrapper (openhue-go)
- `internal/dbus/`: DBus service implementation
- `internal/capture/`: Wayland/Pipewire screen capture
- `internal/sync/`: Entertainment API streaming and color sync
- `internal/entertainment/`: DTLS Entertainment API client
- `internal/color/`: Color extraction and zone mapping

### DBus Interface
- Service: `org.kde.plasma.hue`
- Path: `/org/kde/plasma/hue`
- Interface: `org.kde.plasma.hue`

Key Methods:
- `GetStatus() → string`
- `GetScenes() → []Scene`
- `ActivateScene(string sceneId) → (string, *dbus.Error)`
- `SetPower(bool on) → (bool, *dbus.Error)`
- `SetBrightness(int value) → (bool, *dbus.Error)`
- `StartSync() / StopSync() / IsSyncing() → bool`

## Build Commands

### Backend (Go)
```bash
cd backend
go build -o hue-sync ./cmd/hue-sync
```

### Tray Application (Qt/C++)
```bash
cd trayapp
cmake . && make
```

### Manual Installation
```bash
# Build components
cd backend && go build -o hue-sync ./cmd/hue-sync && cd ..
cd trayapp && cmake . && make && cd ..

# Install systemd service
mkdir -p ~/.config/systemd/user/
cp systemd/hue-backend.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now hue-backend

# Install autostart
mkdir -p ~/.config/autostart/
cp systemd/hue-tray.desktop ~/.config/autostart/
```

## Test Commands

### Go Tests
```bash
cd backend

# Run all tests
go test ./...

# Test specific package
go test ./internal/config -v
go test ./internal/hue -v

# Run specific test function
go test ./internal/config -v -run TestDefaultConfig
```

### Integration Testing
```bash
# Full integration test (requires Hue bridge)
./scripts/test-integration.sh

# Manual DBus testing
./backend/hue-sync &
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus
```

### Test Utilities
Backend includes test binaries in `backend/cmd/`:
- `test-capture`: Test Wayland screen capture
- `test-color`: Test color extraction
- `test-entertainment`: Test Entertainment API streaming
- `test-zones-visual`: Visualize zone mapping

```bash
cd backend
go run ./cmd/test-capture
go run ./cmd/test-zones-visual
```

## Configuration

Config file: `~/.openhue/config.yaml`

```yaml
Bridge: "192.168.1.X"
Key: "YOUR-API-KEY"
clientkey: "CLIENT-KEY"  # For Entertainment API
sync:
  enabled: false
  fps: 30
  subsampleWidth: 64
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
```

### UV Coordinates
Screen zones use UV coordinates (0.0-1.0) for monitor-agnostic mapping. Each channel has `uvA` (top-left) and `uvB` (bottom-right) corners:
- Left half: `uvA: {x: 0.0, y: 0.0}`, `uvB: {x: 0.5, y: 1.0}`
- Right half: `uvA: {x: 0.5, y: 0.0}`, `uvB: {x: 1.0, y: 1.0}`
- Top half: `uvA: {x: 0.0, y: 0.0}`, `uvB: {x: 1.0, y: 0.5}`

## Key Conventions

### Scene Format
Scenes returned by `GetScenes()` include room names:
- Format: `"Room Name - Scene Name"` (e.g., "Living Room - Relax")
- Sorted alphabetically for consistent UI display

### Auto-start Integration
Two systemd components:
1. **Backend service**: `~/.config/systemd/user/plasma-hue-backend.service`
   - Managed via `systemctl --user enable/start plasma-hue-backend`
2. **Tray autostart**: `~/.config/autostart/hue-tray.desktop`
   - Launches on KDE login

### Entertainment API
Uses `github.com/pion/dtls/v2` for secure streaming:
- Protocol: HueStream v2
- Transport: DTLS 1.2 over UDP
- Format: 16-bit RGB channels per light

### Screen Sync Flow
1. **Capture**: Pipewire screen capture (Wayland)
2. **Subsample**: Resize to configured width (default 64px)
3. **Zone extraction**: Extract color per UV zone
4. **Color calculation**: Mean RGB per zone
5. **Stream**: Send to Entertainment API at configured FPS

## Development Workflows

### Backend Changes
```bash
cd backend
# Edit code
go test ./internal/your-package  # Test
go build -o hue-sync ./cmd/hue-sync  # Build
systemctl --user restart plasma-hue-backend  # Restart
```

### Tray App Changes
```bash
cd trayapp
# Edit main.cpp
cmake . && make
./scripts/restart-tray.sh  # Restart tray app
```

### Adding DBus Methods
1. Add method to `backend/internal/dbus/service.go`:
```go
func (s *Service) MyMethod(param string) (bool, *dbus.Error) {
    // Implementation
    return true, nil
}
```

2. Update introspection in same file

3. Call from tray app:
```cpp
iface.call("MyMethod", "param_value");
```

### Adding Config Options
1. Add field to `backend/internal/config/config.go`:
```go
type Config struct {
    MyOption string `mapstructure:"myOption"`
}
```

2. Update `DefaultConfig()` with default value

3. Use in code:
```go
cfg, _ := config.Load()
value := cfg.MyOption
```

## Common Pitfalls

### Install Script is Broken
The `scripts/install.sh` script references a non-existent `plasmoid/` directory and will fail. Use manual installation steps instead.

### DBus Service Name
Always use `org.kde.plasma.hue` (not variations). This is the canonical service name used throughout the codebase.

### Config File Location
Must be `~/.openhue/config.yaml` for compatibility with openhue-cli. Don't use alternative paths.

### systemd Service Names
- Backend service: `plasma-hue-backend.service` (created by install script)
- Use `systemctl --user` (user services, not system-wide)
- Note: README.md references `hue-backend.service` but install script creates `plasma-hue-backend.service`

### KStatusNotifierItem vs QSystemTrayIcon
This project uses `KStatusNotifierItem` (KDE6) for proper Plasma integration. Don't replace with `QSystemTrayIcon`.

### Entertainment API Setup
Entertainment streaming requires:
1. Entertainment area created in Hue app
2. `clientkey` in config (generated during bridge setup)
3. Active Entertainment area activation via API

Don't attempt streaming without proper Entertainment area setup.

## Dependencies

### Go Backend
- `github.com/openhue/openhue-go`: Official Hue API client
- `github.com/godbus/dbus/v5`: DBus communication
- `github.com/spf13/viper`: Config management (YAML)
- `github.com/pion/dtls/v2`: DTLS for Entertainment API

### Qt/C++ Tray App
- Qt6 (Core, Widgets, DBus)
- KF6StatusNotifierItem (KDE Frameworks 6)
- CMake 3.16+

### System Requirements
- Wayland compositor (for screen capture)
- Pipewire (for screen capture)
- systemd (for service management)
- KDE Plasma 6 (for tray integration)

## Debugging

### Backend Logs
```bash
# Via systemd
journalctl --user -u plasma-hue-backend -f

# Manual run
./backend/hue-sync
```

### DBus Introspection
```bash
# List available methods
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.freedesktop.DBus.Introspectable.Introspect

# Monitor DBus traffic
dbus-monitor --session "interface='org.kde.plasma.hue'"
```

### Tray App Debugging
```bash
# Run manually to see Qt debug output
./trayapp/hue-tray

# Check if running
ps aux | grep hue-tray
```

### Hue Bridge Communication
```bash
cd backend
go run ./cmd/get-entertainment-info  # Check Entertainment setup
go run ./cmd/register-entertainment  # Register new Entertainment area
```

## Testing Requirements

### Before Committing
1. Run Go tests: `cd backend && go test ./...`
2. Build check: `go build -o hue-sync ./cmd/hue-sync`
3. Tray build check: `cd trayapp && cmake . && make`
4. Integration test (if available): `./scripts/test-integration.sh`

### Test Coverage Focus
- Config validation (especially UV coordinates)
- DBus method responses
- Error handling for bridge unreachable
- Scene parsing and formatting

## Performance Tuning

### Screen Sync
- `sync.fps`: Target frame rate (10-60, default 30)
- `sync.subsampleWidth`: Resize width for processing (default 64)
- Lower values = better performance, less precision

## Current Status

### Working Features
- Scene control with room names
- Alphabetical sorting
- Desktop notifications
- Auto-start integration
- System tray integration

### In Development
- Power/Brightness controls (needs grouped light config)
- Screen Sync (Entertainment API)
- Multi-zone mapping
- Settings dialog

## When to Invoke This Agent

Use `/agent khuey-expert` when:
- Making changes to backend or tray app
- Debugging DBus communication issues
- Working with Hue API integration
- Implementing Entertainment API features
- Understanding the architecture
- Troubleshooting installation or configuration issues
