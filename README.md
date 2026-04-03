# KDE Hue Control - Installation & Quick Start

A KDE system tray application for controlling Philips Hue lights with real-time screen synchronization.

## Quick Installation

```bash
# 1. Clone and setup
git clone <your-repo-url>
cd khuey

# 2. Configure your Hue bridge (if not done already)
openhue setup

# 3. Install everything
./scripts/install.sh
```

That's it! The tray icon will appear in your system tray and auto-start on login.

## Manual Installation

If you prefer manual steps:

```bash
# Build backend
cd backend
go build -o hue-sync ./cmd/hue-sync

# Build tray app
cd ../trayapp
cmake . && make

# Install systemd service (optional, for auto-start)
cp ../systemd/hue-backend.service ~/.config/systemd/user/
systemctl --user enable --now hue-backend

# Install autostart file (optional)
cp ../systemd/hue-tray.desktop ~/.config/autostart/

# Start manually (if not using systemd)
../backend/hue-sync &
./hue-tray &
```

## Features

### ✅ Currently Working
- **Scene Control**: Activate any of your Hue scenes with room names
- **Alphabetical Sorting**: Scenes organized by room and name
- **Desktop Notifications**: Success/error feedback for all operations
- **Auto-start**: Backend and tray app start automatically on login
- **System Tray Integration**: Native KDE StatusNotifierItem integration

### 🚧 In Development
- **Screen Sync**: Synchronized lighting with screen colors (Entertainment API)
- **Power/Brightness Controls**: Toggle and dim lights (needs grouped light config)
- **Multi-zone Mapping**: Different screen areas control different lights
- **Settings Dialog**: GUI for configuration

## Usage

### Basic Controls
1. Click the Hue icon in your system tray
2. Select and activate scenes from the list
3. Scenes are shown as "Room Name - Scene Name" in alphabetical order

### Screen Sync (Coming Soon)
Screen-to-lights synchronization using Entertainment API for gaming/movies.

## Configuration

Config file: `~/.openhue/config.yaml`

### Basic Configuration

```yaml
Bridge: "192.168.1.X"          # Your bridge IP
Key: "YOUR-API-KEY"             # API key from setup
clientkey: "CLIENT-KEY"         # For Entertainment API (optional)
sync:
  enabled: false
  fps: 30                       # Sync frame rate (10-60)
  subsampleWidth: 64            # Performance tuning
```

### Zone-Based Screen Mapping

Each light can be mapped to a specific screen region for the Ambilight effect. Configure zones using UV coordinates (0.0-1.0):

- **U (x-axis):** 0.0 = left edge, 1.0 = right edge
- **V (y-axis):** 0.0 = top edge, 1.0 = bottom edge

#### Example: Three Lights (Left/Center/Right)

```yaml
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}    # Top-left corner
    uvB: {x: 0.33, y: 1.0}   # Bottom-right corner
  - id: 1
    active: true
    deviceName: "Center Light"
    gammaFactor: 2.2
    uvA: {x: 0.33, y: 0.0}
    uvB: {x: 0.67, y: 1.0}
  - id: 2
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA: {x: 0.67, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

#### Example: Two Lights (Left/Right Split)

```yaml
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}    # Left half of screen
  - id: 1
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}    # Right half of screen
```

#### Example: TV Backlighting (4 Lights)

```yaml
channels:
  - id: 0  # Left edge
    uvA: {x: 0.0, y: 0.25}
    uvB: {x: 0.1, y: 0.75}
  - id: 1  # Top edge
    uvA: {x: 0.25, y: 0.0}
    uvB: {x: 0.75, y: 0.1}
  - id: 2  # Right edge
    uvA: {x: 0.9, y: 0.25}
    uvB: {x: 1.0, y: 0.75}
  - id: 3  # Bottom edge
    uvA: {x: 0.25, y: 0.9}
    uvB: {x: 0.75, y: 1.0}
```

**Note:** If UV coordinates are not specified, lights will auto-split the screen evenly (backward compatible).

## Troubleshooting

### "Backend not running"
```bash
# Check if backend is running
systemctl --user status hue-backend

# Restart backend
systemctl --user restart hue-backend

# Check logs
journalctl --user -u hue-backend -f
```

### "Not configured" status
Run `openhue setup` to configure your bridge, or check `~/.openhue/config.yaml` exists.

### Tray icon doesn't appear
```bash
# Check if tray app is running
ps aux | grep hue-tray

# Restart it
systemctl --user restart hue-tray
# Or manually: /path/to/hue-tray &
```

### Scenes work but power/brightness doesn't
Power and brightness controls are disabled. They need grouped light configuration - coming in a future update!

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Development environment setup
- Git hooks with Lefthook
- Coding standards and conventions
- Testing procedures
- Pull request workflow

### Quick Start for Contributors

```bash
# Install Lefthook for Git hooks
go install github.com/evilmartians/lefthook/v2@latest
lefthook install

# This sets up automatic code formatting, testing, and validation
```

## Development Status

**Progress: 10/15 components complete (67%)**

✅ Project structure
✅ Configuration system
✅ Hue API client
✅ DBus service
✅ Qt tray application (KStatusNotifierItem)
✅ Scene control with room names
✅ Alphabetical sorting
✅ Desktop notifications
✅ Auto-start integration (systemd + KDE)
✅ Installation scripts
⏳ Power/brightness controls (needs room configuration)
⏳ Screen capture (Wayland/Pipewire)
⏳ Entertainment API streaming
⏳ Settings dialog

## Architecture

```
┌─────────────┐         DBus          ┌──────────────┐
│  Qt Tray    │◄─────────────────────►│  Go Backend  │
│  App (KDE)  │                       │              │
└─────────────┘                       │  - Config    │
                                      │  - Hue API   │
                                      │  - DBus IPC  │
                                      └──────┬───────┘
                                             │
                                             │ HTTPS
                                             ▼
                                      ┌──────────────┐
                                      │  Hue Bridge  │
                                      └──────────────┘
```

## Technical Details

- **Frontend**: Qt/C++ with KStatusNotifierItem
- **Backend**: Go with official openhue-go library
- **IPC**: DBus session bus
- **Config**: YAML (compatible with openhue-cli)
- **Auto-start**: systemd user service + KDE autostart

## Inspiration & References

- [Huenicorn](https://gitlab.com/openjowelsofts/huenicorn) - Entertainment API patterns
- [openhue-cli](https://github.com/openhue/openhue-cli) - Hue API integration

## License

GNU General Public License v3.0 - See LICENSE
