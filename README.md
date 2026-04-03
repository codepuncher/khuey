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

```yaml
Bridge: "192.168.1.X"          # Your bridge IP
Key: "YOUR-API-KEY"             # API key from setup
clientkey: "CLIENT-KEY"         # For Entertainment API (optional)
sync:
  enabled: false
  fps: 30                       # Sync frame rate (10-60)
  subsampleWidth: 64            # Performance tuning
```

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
