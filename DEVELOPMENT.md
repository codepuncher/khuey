# Development Guide

## Current Implementation Status

### ✅ Completed Components

1. **Project Structure**
   - Go backend with clean package organization
   - KDE Plasma widget with QML UI
   - CMake build system
   - Testing utilities

2. **Configuration Management** (`backend/internal/config/`)
   - YAML-based config (~/.openhue/config.yaml)
   - Compatible with openhue-cli
   - Support for sync settings, channels, Entertainment API

3. **Hue API Client** (`backend/internal/hue/`)
   - Wrapper around openhue-go
   - Scene listing and activation
   - Light control (power, brightness)
   - Bridge connectivity check

4. **DBus Service** (`backend/internal/dbus/`)
   - Session bus interface at org.kde.plasma.hue
   - Methods: GetStatus, SetPower, SetBrightness, ActivateScene, GetScenes
   - Sync controls (StartSync, StopSync, IsSyncing)

5. **Plasmoid UI** (`plasmoid/contents/ui/`)
   - Compact representation (system tray icon)
   - Full representation (popup interface)
   - DBus integration for real-time updates
   - Scene selector with dynamic loading

### ⏳ TODO Components

1. **Wayland Screen Capture** (`backend/internal/capture/`)
   - Pipewire integration via DBus portal
   - Monitor selection
   - Frame capture at configurable FPS

2. **Image Processing** (`backend/internal/sync/`)
   - Image subsampling for performance
   - UV-based zone extraction
   - Mean color calculation per zone
   - Gamma correction

3. **Entertainment API** (`backend/internal/sync/`)
   - DTLS 1.2 client (github.com/pion/dtls)
   - HueStream v2 protocol
   - RGB color streaming (16-bit channels)
   - Channel management

4. **Settings Dialog** (`plasmoid/contents/ui/`)
   - Bridge configuration
   - Entertainment area selection
   - Zone mapping configuration
   - Performance tuning

## Building & Testing

### Backend Development

```bash
cd backend

# Run tests
go test ./...

# Test specific component
go test ./internal/config -v

# Build
go build -o hue-sync ./cmd/hue-sync

# Run with debug logging
./hue-sync

# Test DBus interface
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetScenes
```

### Plasmoid Development

```bash
# Install for testing
kpackagetool6 --upgrade plasmoid --type Plasma/Applet

# View QML errors
journalctl -f | grep plasmashell

# Restart Plasma to reload
kquitapp6 plasmashell && plasmashell &

# Check if installed
kpackagetool6 --list | grep hue
```

### Integration Testing

```bash
# 1. Start backend
./backend/hue-sync &
BACKEND_PID=$!

# 2. Test DBus
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus

# 3. Test scene activation
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.ActivateScene \
  string:"Relax"

# 4. Stop backend
kill $BACKEND_PID
```

## Code Style

### Go
- Follow standard Go conventions
- Use `go fmt` for formatting
- Document exported functions
- Keep packages focused and cohesive

### QML
- Use Plasma Components where possible
- Follow KDE HIG (Human Interface Guidelines)
- Use property bindings over imperative code
- Keep business logic in backend (DBus)

## Adding New Features

### Adding a DBus Method

1. Add method signature to `backend/internal/dbus/service.go`:
```go
func (s *Service) MyNewMethod(param string) (bool, *dbus.Error) {
    // Implementation
    return true, nil
}
```

2. Add to introspection:
```go
{
    Name: "MyNewMethod",
    Args: []introspect.Arg{
        {Name: "param", Type: "s", Direction: "in"},
        {Name: "success", Type: "b", Direction: "out"},
    },
}
```

3. Call from QML:
```qml
dbusInterface.call("MyNewMethod", ["value"], function(result) {
    console.log("Result:", result)
})
```

### Adding a Config Option

1. Add field to `internal/config/config.go`:
```go
type Config struct {
    MyNewOption string `mapstructure:"myNewOption"`
}
```

2. Update DefaultConfig():
```go
func DefaultConfig() *Config {
    return &Config{
        MyNewOption: "default",
    }
}
```

3. Use in code:
```go
cfg, _ := config.Load()
value := cfg.MyNewOption
```

## Architecture Decisions

### Why Go for Backend?
- Fast compilation
- Great concurrency support (goroutines for sync engine)
- Official openhue-go library
- Good DBus support
- Single binary deployment

### Why DBus for IPC?
- Native to Linux desktop
- Well-supported in Qt/QML
- Session bus for user services
- Introspection support
- Standard for Plasma widgets

### Why Wayland-only?
- User's primary environment
- Modern display protocol
- Can add X11 later if needed
- Pipewire integration for capture

## Performance Considerations

### Screen Sync
- Target 30 FPS default (good balance)
- Subsample to 64px width (fast)
- Use mean color (faster than k-means)
- Async light updates (non-blocking)

### Memory
- Reuse image buffers
- Limit channel history
- Stream don't store

### Network
- Batch light updates when possible
- Use Entertainment API (UDP) for sync
- Standard API (HTTP) for scenes/control

## Future Enhancements

1. **Entertainment Area Setup**
   - Guide user through Hue app setup
   - Validate Entertainment configuration
   - Test streaming before enabling

2. **Zone Mapping UI**
   - Visual screen preview
   - Drag-and-drop zone placement
   - Preview light positions

3. **Presets & Profiles**
   - Save zone configurations
   - Per-game/application profiles
   - Quick switch between setups

4. **Advanced Features**
   - Audio sync (music reactive)
   - Keyboard shortcuts
   - Notification integration
   - Multi-monitor support

## Resources

- [KDE Plasma QML API](https://api.kde.org/frameworks/)
- [openhue-go docs](https://pkg.go.dev/github.com/openhue/openhue-go)
- [DBus Specification](https://dbus.freedesktop.org/doc/dbus-specification.html)
- [Hue Entertainment API](https://developers.meethue.com/develop/hue-entertainment/)
