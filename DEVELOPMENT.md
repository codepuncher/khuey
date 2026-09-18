# Development Guide

## Implementation Status

`FEATURES.md` describes what the app does for a user. This section maps that
onto the code and records what is not built.

### Built

1. **Backend packages** (`backend/internal/`)
   - `config/` YAML load, validate and save, sharing the file with openhue-cli
   - `hue/` REST calls through openhue-go: scenes, power, brightness
   - `dbus/` the `org.kde.plasma.hue` service, UID access control, input
     validation on everything arriving from outside
   - `capture/` PipeWire screen capture via CGo, mediated by the XDG portal
   - `color/` UV zone to mean RGB with stride sampling and gamma correction
   - `sync/` the capture, extract and stream loop at the configured FPS
   - `entertainment/` DTLS 1.2 client, HueStream v2, 16-bit RGB per channel
   - `gaming/` gaming detection, which starts and stops sync on its own

2. **DBus service** (`backend/internal/dbus/service.go`)
   - 27 methods on `org.kde.plasma.hue`: scenes, power, brightness, sync,
     gaming mode, bridge connection, room selection, startup scene, tray icons
   - A list here goes stale. Read it from the running backend:

     ```bash
     dbus-send --session --print-reply --dest=org.kde.plasma.hue \
       /org/kde/plasma/hue org.freedesktop.DBus.Introspectable.Introspect
     ```

3. **Tray application** (`trayapp/`)
   - KStatusNotifierItem menu with scene, power, brightness and sync controls
     (`main.cpp`)
   - Settings dialog with four tabs, Screen Sync, Light Control, Connection and
     Appearance (`settingsdialog.cpp`)
   - Every backend call is asynchronous. Background failures raise a
     KNotification, dialog-driven ones a QMessageBox

### Not built

1. **Zone mapping UI**
   - `channels:`, the per-light UV rectangles, is hand-edited in config.yaml.
     `go run ./cmd/test-zones-visual` writes a picture of a fixed
     left/center/right split, not of the configured channels.

2. **Entertainment setup in the GUI**
   - Both steps are CLI-only, and both print values to paste into the config by
     hand: `go run ./cmd/register-entertainment` does the link-button
     registration and prints `key` and `clientkey`, and
     `go run ./cmd/get-entertainment-info` lists the areas and their IDs for
     `entertainmentConfigurationId`.

3. **Monitor selection, and capturing more than one monitor**
   - Not reachable from the app. The XDG ScreenCast portal's `SelectSources`
     takes no option naming an output, so the screen is chosen in the portal's
     own dialog. Capture asks for `types: monitor, multiple: false` and reads
     the one stream that comes back (`internal/capture/portal.go`). All the app
     can do is clear the saved grant so the dialog asks again
     (`ResetCaptureSource`, behind the "Change capture screen" button).

## Building and Testing

### Backend

```bash
cd backend

# pkg-config for libpipewire emits -fno-strict-overflow, which cgo rejects
# unless it is allowlisted. Anything that compiles internal/capture needs this,
# the tests included.
export CGO_CFLAGS_ALLOW='-fno-strict-overflow'

go build -o hue-sync ./cmd/hue-sync

go test ./...                                        # all tests
go test ./internal/config -v                         # one package
go test ./internal/config -v -run TestDefaultConfig  # one test
```

The backend normally runs as the `hue-backend` user unit. Only one process can
own the bus name, so a second copy started by hand exits with
`name already taken`.

### Tray application

```bash
cd trayapp

# In-source build, the same one scripts/restart.sh and scripts/install.sh run
cmake . && make

# The backend must be running for anything in the menu to work
./hue-tray
```

### Running what you built

`scripts/restart.sh` drives the installed `hue-backend` user unit, which
`scripts/install.sh` writes to `~/.config/systemd/user/`, along with the tray's
autostart entry in `~/.config/autostart/`. Run install.sh once on a fresh clone.
Without the unit, restart.sh builds both components, starts only the tray, and
exits reporting the backend inactive.

```bash
# From the repo root: rebuilds both components and restarts both
./scripts/restart.sh

journalctl --user -u hue-backend -f   # backend log
less /tmp/hue-tray.log                # tray log, when started by restart.sh
```

Restarting the backend activates `startupScene` if one is set, which turns the
lights on.

### Driving the backend by hand

```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus

# Scene names are the "Room - Scene" strings GetScenes returns
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.ActivateScene \
  string:"Living Room - Relax"

# Traffic between the tray and the backend
dbus-monitor --session "interface='org.kde.plasma.hue'"
```

`scripts/test-dbus.sh` calls `GetStatus`, `GetScenes` and `IsSyncing`, checks
the introspection XML and measures response time.

Don't start sync this way. `StartSync` blocks until the portal's screen-share
dialog is answered and a terminal cannot show that dialog, so `dbus-send` gives
up after its 25 s reply timeout while the backend keeps waiting, up to 2
minutes. A valid `sync.restoreToken` skips the dialog.

## Code Style

`CONTRIBUTING.md` has the full rules and the hooks that enforce them.

### Go

- Standard Go conventions, `gofmt -s`
- Document exported functions
- Keep packages focused and cohesive

### C++/Qt

- Qt naming conventions
- Qt's parent-child ownership for memory management
- Signals and slots for communication
- Keep the business logic in the backend; the tray is UI only

## Adding New Features

### Adding a DBus Method

1. Add the method to `backend/internal/dbus/service.go`:

```go
func (s *Service) MyNewMethod(param string, sender dbus.Sender) (bool, *dbus.Error) {
    if err := s.checkAccess(sender); err != nil {
        return false, dbus.MakeFailedError(err)
    }
    if err := common.ValidateDBusString("param", param, 255); err != nil {
        return false, dbus.MakeFailedError(err)
    }
    return true, nil
}
```

`sender` is filled in by godbus and is not part of the wire signature.

2. Add it to the introspection node in the same file. Dispatch works without
   this, but nothing that introspects the service sees the method:

```go
{
    Name: "MyNewMethod",
    Args: []introspect.Arg{
        {Name: "param", Type: "s", Direction: "in"},
        {Name: "success", Type: "b", Direction: "out"},
    },
}
```

3. Call it from the tray app. A blocking `call()` freezes the UI for as long as
   the backend takes, so every call uses `asyncCall`, most of them through the
   `whenFinished` helper in `trayapp/huebackend.h`:

```cpp
QDBusPendingCall call = iface.asyncCall("MyNewMethod", "value");
whenFinished(this, {call}, [call]() {
    QDBusReply<bool> reply = call;
    if (!reply.isValid()) {
        return;
    }
    // use reply.value()
});
```

### Adding a Config Option

1. Add the field to `internal/config/config.go`:

```go
type Config struct {
    MyNewOption string `mapstructure:"myNewOption"`
}
```

2. Give it a default in `DefaultConfig()`.

3. Add the key to `saveLocked()`. A top-level field that is not `Set` there is
   never written: viper still holds the value it read at load and writes that
   back, so every change to the field is lost on the next save.

4. Range-check it in `Validate()` if it has a valid range.

5. After startup, read it through `Config.View` and change it through
   `Config.Update`, which take the config's own mutex. The daemon calls
   `config.Load()` once, at startup in `cmd/hue-sync/main.go`.

```go
var option string
cfg.View(func(c *config.Config) { option = c.MyNewOption })

err := cfg.Update(func(c *config.Config) { c.MyNewOption = "new" }, nil)
```

6. Document it in `docs/CONFIGURATION.md`.

## Architecture Decisions

### Why Go for Backend?

- Fast compilation
- Goroutines for the sync engine
- The openhue-go library
- Good DBus support
- Single binary deployment

### Why DBus for IPC?

- Native to the Linux desktop
- Well supported in Qt/C++
- Session bus for user services
- Introspection support
- Standard for KDE applications

### Why Wayland-only?

- The development environment
- Modern display protocol
- X11 can be added later if needed
- PipeWire integration for capture

## Performance Considerations

### Screen Sync

- 30 FPS by default
- Zones are sampled with a stride sized to take about `sync.subsampleWidth`
  samples across the zone, rather than resizing the frame first
- Mean color per zone
- The tray never blocks on the backend

### Memory

- Frame buffers are recycled through `capture.imageBufferPool`, not allocated
  per frame, so keep the `GetImageBuffer`/`PutImageBuffer` pairs balanced
- Stream, don't store

### Network

- One UDP packet per frame carries every channel
- Entertainment API (DTLS over UDP) for sync
- REST for scenes and light control

## Future Enhancements

1. **Entertainment Area Setup**
   - Run the registration from the settings dialog rather than the CLI
   - Validate the Entertainment configuration
   - Test streaming before enabling it

2. **Zone Mapping UI**
   - Visual screen preview
   - Drag-and-drop zone placement
   - Preview light positions

3. **Presets & Profiles**
   - Save zone configurations
   - Per-game and per-application profiles
   - Quick switch between setups

4. **Advanced Features**
   - Audio sync (music reactive)
   - Keyboard shortcuts

## Resources

- [KDE Frameworks API](https://api.kde.org/frameworks/)
- [openhue-go docs](https://pkg.go.dev/github.com/openhue/openhue-go)
- [DBus Specification](https://dbus.freedesktop.org/doc/dbus-specification.html)
- [Hue Entertainment API](https://developers.meethue.com/develop/hue-entertainment/)
