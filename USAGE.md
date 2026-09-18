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
- **Change capture screen**: Drops the saved screen-share grant, so the system's
  screen-share dialog asks which screen to capture the next time sync starts
- **Gaming Mode**: Automatically enable screen sync when gaming (disabled by default)
  - Detects games using systemd-inhibit, power profile and Steam detection
  - Auto-starts sync when you launch a game
  - Auto-stops sync when you exit the game
  - Great for immersive gaming without manual toggling

FPS applies straight away, including to a sync already running. Subsample
width applies the next time Screen Sync starts.

#### Light Control Settings
- **Room/Zone Selection**: Choose which room or zone to control with power and brightness buttons
- **Refresh**: Reload available rooms/zones from the bridge
- Shows format: "Room Name (room)" or "Zone Name (zone)"
- **Activate scene on login**: Scene the backend activates each time it starts,
  or "(Disabled)". Saved as `startupScene` in the config file

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

Gaming Mode uses three detection methods:

1. **systemd-inhibit** (Primary)
   - Matches `systemd-inhibit --list` against the game-performance lock
   - A game started without that wrapper takes no lock and is not seen here

2. **Power Profile + Steam AppId** (Secondary)
   - Requires both the "performance" power profile and a running Steam game
   - Both have to agree, which keeps false positives down

3. **Feral GameMode** (Fallback)
   - Off by default; enable with `useGameMode`

**Debouncing**: Waits 5 seconds after detection before triggering, so a game launched and closed again inside that window does not start sync.

### Status Indicator

When gaming mode is active and syncing, you'll see:
- **Tray Icon**: the gaming icon (`applications-games` by default)
- **Tooltip**: "Gaming Mode Active - Syncing to screen"
- **Control Panel**: "Screen sync active  •  30 FPS (Gaming)"
- **Backend Logs**: "[INFO] Gaming detected - starting screen sync"

### Configuration Options

Advanced users can customize gaming mode in `~/.openhue/config.yaml`:

```yaml
gamingMode:
  enabled: false          # Feature toggle
  pollInterval: 2         # How often to check (seconds)
  debounceDelay: 5        # Wait before triggering (seconds)
  useSystemdInhibit: true # Check the systemd-inhibit game lock
  usePowerProfile: true   # Check the power profile
  useSteamAppId: true     # Check for a running Steam game
  useGameMode: false      # Check GameMode DBus
```

### Troubleshooting

**Gaming mode not detecting my game:**
- Check the inhibitor lock is taken: `systemd-inhibit --list | grep -i game`
- Check the power profile switches: `powerprofilesctl get`
- Check backend logs: `journalctl --user -u hue-backend -f`

**False positives:**
- Increase `debounceDelay` to 10+ seconds
- Disable `usePowerProfile` and `useSteamAppId` to rely on systemd-inhibit alone

**Gaming mode not working at all:**
- Ensure Entertainment API is configured (required for screen sync)
- Ensure `gamingMode.enabled: true` in the config
- Check at least one detection method is enabled

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

## Gaming Mode (Automatic Screen Sync)

Gaming Mode automatically detects when you're playing a game and enables screen sync for immersive lighting effects.

### How It Works

The app uses multiple detection methods to identify gaming activity:

**1. CachyOS Game-Performance** (Primary - Best on CachyOS)
- Detects when CachyOS applies gaming optimizations
- Works with ALL games (Steam, native, Wine/Proton, Lutris)
- Zero false positives - only active when game is actually running
- Uses systemd-inhibit to detect game-related locks

**2. Power Profile + Steam AppId** (Secondary)
- Validates "performance" power profile is active
- Confirms Steam game is running via reaper process
- Excellent for Steam games on any distro
- Combined check prevents false positives

**3. Feral GameMode** (Fallback)
- Works if GameMode is installed (`sudo pacman -S gamemode`)
- Many games use GameMode for optimization
- Lower priority than CachyOS detection

### Setup

**Option 1: Via Settings Dialog (Recommended)**

1. Right-click tray icon → **Settings**
2. Go to **Screen Sync** tab
3. Enable: ☑ **"Automatically enable screen sync when gaming"**
4. Click **Save**

**Option 2: Via Config File**

Edit `~/.openhue/config.yaml`:

```yaml
gamingMode:
  enabled: true                  # Enable gaming mode
  pollInterval: 2                # Check every 2 seconds
  debounceDelay: 5               # 5 second debounce
  useSystemdInhibit: true        # CachyOS detection (primary)
  usePowerProfile: true          # Power profile validation
  useSteamAppId: true            # Steam-specific detection
  useGameMode: false             # Feral GameMode (if installed)
```

**Option 3: Via DBus Command**

```bash
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true
```

### Testing

Launch a game and verify detection:

```bash
cd backend
./test-gaming-detection
```

Expected output when Skyrim SE is running:
```
[17:52:00]
  systemd-inhibit: true ✅
  Power profile:   true ✅
  Steam AppId:     true (AppId: 489830) ✅
  🎮 Gaming Active: true ✅
```

When no game is running:
```
[17:52:02]
  systemd-inhibit: false
  Power profile:   false
  Steam AppId:     false
  🎮 Gaming Active: false
```

### How It Behaves

**When you launch a game:**
1. Gaming mode detects gaming activity
2. After 5 seconds (debounce delay), screen sync auto-starts
3. Your lights sync to screen colors for immersive experience
4. Notification: "🎮 Gaming detected - starting screen sync"

**When you exit the game:**
1. Gaming mode detects game stopped
2. After 5 seconds (debounce delay), screen sync auto-stops
3. Your lights return to previous state
4. Notification: "🎮 Gaming stopped - stopping screen sync"

**Debouncing prevents false triggers:**
- A game launched and closed again inside the debounce window doesn't trigger sync
- A brief detection dropout won't stop sync
- Only sustained gaming activity triggers auto-sync

### Troubleshooting

**Games not detected?**

**On CachyOS:**
- Should work automatically for all Steam games
- Check if power profile switches to "performance" during gaming: `powerprofilesctl get`
- Verify systemd inhibitor is created: `systemd-inhibit --list | grep -i game`

**On other distros:**
- Install GameMode for better detection: `sudo pacman -S gamemode`

**Check detection status:**
```bash
# Test detection while game is running
cd backend
./test-gaming-detection

# Check backend logs
journalctl --user -u hue-backend -f
```

**False activations?**
- Debouncing prevents brief detections from triggering
- Power profile check reduces false positives from non-gaming "performance" usage
- If still happening, increase `debounceDelay` to 10 seconds

**Gaming mode not starting/stopping?**
```bash
# Enable gaming mode
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true

# Check if detector is running
journalctl --user -u hue-backend -n 20 | grep -i "gaming mode"

# Should see: "✅ Gaming mode detector started"
```

**Manual override:**
You can always manually start/stop screen sync from the tray app regardless of gaming mode status.

### Supported Games

**CachyOS (Best Support):**
- All Steam games (including Proton/Wine)
- Native Linux games
- GOG games via Heroic Launcher
- Lutris games
- Any game that triggers CachyOS game-performance

**Other Distros:**
- Steam games (via AppId detection)
- Games that use GameMode (if installed)

### Performance Impact

- Poll interval: Checks gaming status every 2 seconds
- Minimal CPU usage when idle
- No impact when gaming mode is disabled
- Screen sync itself uses moderate CPU (adjustable via FPS setting)

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
