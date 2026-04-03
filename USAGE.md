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
