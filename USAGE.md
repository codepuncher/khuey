# KDE Plasma Hue Widget - Usage Guide

## Quick Start

### 1. Start the Backend Service

The backend service should auto-start via systemd:

```bash
systemctl --user status hue-backend.service
```

If not running, start it:

```bash
systemctl --user start hue-backend.service
```

### 2. Launch the Tray App

```bash
~/.local/bin/hue-tray
```

Or if ~/.local/bin is in your PATH:

```bash
hue-tray
```

The Hue icon will appear in your system tray.

### 3. Control Your Lights

Click the Hue icon to open the control panel:

- **Power Switch**: Toggle all lights on/off
- **Brightness Slider**: Adjust brightness (0-100%)
- **Start Screen Sync**: Enable real-time color sync

### 4. Configure Settings (NEW!)

Right-click the tray icon and select **"⚙️ Settings..."** to access the Settings Dialog.

The Settings Dialog provides a GUI to configure:

#### Screen Sync Settings
- **FPS**: Frame rate for screen sync (10-60 FPS, default: 30)
  - Higher = smoother, but more CPU usage
  - Recommended: 20-30 for balanced performance
- **Subsample Width**: Processing quality (16-256px, default: 64)
  - Lower = better performance, less color precision
  - Higher = better color accuracy, more CPU usage
- **Monitor**: Select which monitor to capture (default: primary monitor)
- **Gaming Mode**: Automatically enable screen sync when gaming (disabled by default)
  - Detects games using GameMode and fullscreen window detection
  - Auto-starts sync when you launch a game
  - Auto-stops sync when you exit the game
  - Great for immersive gaming without manual toggling

Changes require restarting Screen Sync to take effect.
- **Subsample Width**: Processing quality (16-256 pixels, default: 64)
  - Lower = better performance, less color precision
  - Higher = more accurate colors, more CPU usage
- **Monitor**: Select which monitor to sync (default: primary)

**Note:** Screen Sync must be restarted for changes to take effect.

#### Light Control Settings
- **Room/Zone Selection**: Choose which room or zone to control with power and brightness buttons
- **Refresh**: Reload available rooms/zones from the bridge
- Shows format: "Room Name (room)" or "Zone Name (zone)"

#### Connection Settings
- **Bridge IP**: View your current bridge IP address
- **Status**: Connection status (✓ Connected / ✗ Disconnected)
- **Test Connection**: Verify bridge is reachable
- **Reconnect**: Attempt to reconnect to the bridge

**Tip:** To change bridge IP or API key, edit `~/.openhue/config.yaml` manually.

#### Dialog Controls
- **OK**: Save settings and close dialog
- **Apply**: Save settings without closing
- **Cancel**: Discard changes and close

All settings are automatically saved to `~/.openhue/config.yaml` when you click Apply or OK.

## Screen Sync Feature

### How It Works

1. Click **"Start Screen Sync"** button
2. Backend automatically:
   - Activates your Entertainment Area
   - Connects to Philips Hue Bridge via DTLS
   - Captures screen colors (currently mock gradient)
   - Streams colors to lights at 30 FPS
3. Your lights sync with screen colors in real-time!

### Current Implementation

The screen sync currently uses a **mock gradient** for testing:
- **Left zone** → BLUE
- **Center zone** → GRAY
- **Right zone** → RED

This perfectly demonstrates the Entertainment API pipeline. Real desktop screen capture is documented in `backend/SCREEN_CAPTURE.md`.

### Performance

- FPS: 30 FPS (configurable 10-60)
- Resolution: Configurable subsample width (16-256px)
- Default CPU usage: ~5% on typical systems

## Gaming Mode (NEW!)

### Overview

Gaming Mode automatically detects when you're playing games and enables screen sync for an immersive lighting experience. No more manual toggling!

### How to Enable

1. Right-click the tray icon → **Settings**
2. Go to the **Screen Sync** tab
3. Check ☑ **"Automatically enable screen sync when gaming"**
4. Click **OK** or **Apply**

### How It Works

Gaming Mode uses two detection methods:

1. **GameMode Detection** (Primary)
   - Detects games launched with GameMode (Steam, Lutris, etc.)
   - Most modern games automatically use GameMode on Linux
   - Very reliable and game-specific

2. **Fullscreen Detection** (Secondary)
   - Detects any fullscreen window via KWin
   - Catches games that don't use GameMode
   - Provides fallback coverage

**Debouncing**: Waits 5 seconds after detection before triggering to avoid false positives from alt-tabbing.

### Status Indicator

When gaming mode is active and syncing, you'll see:
- **Control Panel**: "✅ Syncing (Gaming Mode 🎮)"
- **Backend Logs**: "🎮 Gaming detected - starting screen sync"

### Configuration Options

Advanced users can customize gaming mode in `~/.openhue/config.yaml`:

```yaml
gamingMode:
  enabled: false          # Feature toggle
  pollInterval: 2         # How often to check (seconds)
  debounceDelay: 5        # Wait before triggering (seconds)
  useGameMode: true       # Check GameMode DBus
  useFullscreen: true     # Check KWin fullscreen
```

### Troubleshooting

**Gaming mode not detecting my game:**
- Check if the game uses GameMode: `gamemoded -s`
- Verify the game runs in fullscreen (not windowed/borderless)
- Check backend logs: `journalctl --user -u hue-backend -f`
- Try launching game with GameMode: `gamemoderun ./game`

**False positives (video players, etc.):**
- Debouncing helps reduce flicker
- Future: Window class filtering will be added

**Gaming mode not working at all:**
- Ensure Entertainment API is configured (required for screen sync)
- Check if GameMode is installed: `which gamemoded`
- Verify KWin is running: `qdbus org.kde.KWin`

### Performance

- Streaming: 30 FPS
- Latency: ~33ms per update
- Zero errors with proper configuration

## Configuration

Configuration file: `~/.openhue/config.yaml`

```yaml
bridge: "192.168.0.9"
key: "YOUR_USERNAME"
clientkey: "YOUR_HEX_CLIENTKEY"
entertainment_configuration_id: "YOUR_ENTERTAINMENT_AREA_ID"
channels:
  - id: 0
    active: true
  - id: 1
    active: true
  - id: 2
    active: true
```

### Getting Credentials

Run the registration tool:

```bash
cd backend
./register-entertainment
```

Follow the prompts to press the link button on your bridge.

### Finding Your Entertainment Area

```bash
cd backend
./get-entertainment-info
```

This shows all Entertainment Areas and their channel IDs.

## Troubleshooting

### Backend Not Starting

```bash
journalctl --user -u hue-backend.service -n 50
```

### Sync Not Working

1. Check if Entertainment Area is active
2. Verify no streaming errors in logs
3. Ensure lights are in the Entertainment Area

### Button Text Wrong

If you see "Start Sync" instead of "Start Screen Sync", rebuild the tray app:

```bash
cd trayapp
make
cp hue-tray ~/.local/bin/
```

## Auto-Start

The backend auto-starts via systemd. To make the tray app auto-start:

1. Create `~/.config/autostart/hue-tray.desktop`:

```desktop
[Desktop Entry]
Type=Application
Name=Hue Tray
Exec=/home/YOUR_USERNAME/.local/bin/hue-tray
X-GNOME-Autostart-enabled=true
```

2. Replace `YOUR_USERNAME` with your actual username

## Logs

View backend logs:

```bash
journalctl --user -u hue-backend.service -f
```

## More Information

- Real screen capture options: `backend/SCREEN_CAPTURE.md`
- Development guide: `DEVELOPMENT.md`
- Testing guide: `TESTING.md`
