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

- **Scene Control**: Activate any of your Hue scenes with room names
- **Screen Sync**: Real-time screen color synchronization with Hue lights (Entertainment API)
- **Settings Dialog**: GUI for bridge setup, room selection, and screen sync configuration
- **Power/Brightness Controls**: Toggle and dim lights with room/zone support
- **Multi-zone Mapping**: Different screen areas control different lights
- **Desktop Notifications**: Success/error feedback for all operations
- **Auto-start**: Backend and tray app start automatically on login
- **System Tray Integration**: Native KDE StatusNotifierItem integration

## Usage

### Basic Controls
1. Click the Hue icon in your system tray
2. Select and activate scenes from the list
3. Adjust power and brightness for configured rooms
4. Access Settings for advanced configuration

### Screen Sync
Screen-to-lights synchronization using Entertainment API for gaming/movies. Enable via Settings Dialog or config file.

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

## Troubleshooting

### Backend Service Issues

**Problem: Tray app says "DBus service not available"**
```bash
# Check if backend is running
systemctl --user status hue-backend

# View backend logs
journalctl --user -u hue-backend -f

# Restart backend
systemctl --user restart hue-backend
```

**Problem: Backend won't start**
```bash
# Check for errors in logs
journalctl --user -u hue-backend --no-pager -n 50

# Common issues:
# - Config file missing: Check ~/.openhue/config.yaml exists
# - Bridge address wrong: Update 'Bridge:' in config.yaml
# - API key invalid: Run 'openhue setup' to generate new key
```

### Bridge Connection Issues

**Problem: "Cannot connect to Hue Bridge" notification**

1. **Check network connectivity**
   ```bash
   ping YOUR_BRIDGE_IP
   ```

2. **Verify bridge IP in config**
   ```bash
   grep Bridge ~/.openhue/config.yaml
   ```

3. **Test bridge manually**
   ```bash
   curl -k https://YOUR_BRIDGE_IP/clip/v2/resource
   ```

4. **Use retry button**
   - Click notification's "Retry" button
   - Or open control panel and click "Refresh"

**Problem: Bridge IP changed (DHCP)**

Set a static IP for your bridge in your router settings, or update the config:
```bash
# Edit config file
nano ~/.openhue/config.yaml

# Update Bridge IP
Bridge: "192.168.1.X"  # Your new IP

# Restart backend
systemctl --user restart hue-backend
```

### Screen Sync Issues

**Problem: "Screen sharing permission denied"**

This is **normal** the first time! You must:
1. Click "Start Screen Sync"
2. **Approve the GUI permission dialog** that appears
3. Select which monitor to share
4. Click "Share"

The dialog is shown by your desktop environment (XDG Desktop Portal) and is required for security.

**Problem: Screen Sync button does nothing**

```bash
# Check if Entertainment API is configured
grep entertainmentConfigurationId ~/.openhue/config.yaml

# If not configured, set it up:
cd backend
go run ./cmd/register-entertainment
# Follow prompts to create Entertainment Area

# Update config with the ID and clientkey shown
```

**Problem: "Sync engine not available"**

You need to configure Entertainment API:
1. Open Hue app on phone
2. Create an Entertainment Area (Settings → Entertainment Areas)
3. Add your lights to the area
4. Run: `cd backend && go run ./cmd/register-entertainment`
5. Update `~/.openhue/config.yaml` with `entertainmentConfigurationId` and `clientkey`

### Scene Activation Issues

**Problem: Scenes don't appear in list**

```bash
# Check if scenes exist in Hue app
# Then verify backend can fetch them:
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetScenes
```

**Problem: Scene activation fails**

- Ensure lights are powered on (not physically off)
- Check bridge connection (see above)
- Verify scene still exists in Hue app

### Configuration Errors

**Problem: "uvA.x must be 0.0-1.0" or similar validation error**

Your config file has invalid UV coordinates. UV coordinates represent screen zones:
- `0.0` = left/top edge
- `1.0` = right/bottom edge
- `uvA` = top-left corner, `uvB` = bottom-right corner

Example fix:
```yaml
channels:
  - id: 0
    uvA:
      x: 0.0   # Left edge
      y: 0.0   # Top edge
    uvB:
      x: 0.5   # Middle (left half of screen)
      y: 1.0   # Bottom edge
```

**Problem: "sync.fps must be between 1 and 60"**

Update your config:
```yaml
sync:
  fps: 30  # Recommended: 20-30
  subsampleWidth: 64  # Recommended: 64
```

### Getting Help

Still stuck? Check:
1. **Logs**: `journalctl --user -u hue-backend -f`
2. **DBus introspection**: `dbus-send --session --dest=org.kde.plasma.hue --print-reply /org/kde/plasma/hue org.freedesktop.DBus.Introspectable.Introspect`
3. **GitHub Issues**: Report bugs or ask questions

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
