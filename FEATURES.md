# KDE Hue Control - Features

Complete feature documentation for KDE Hue Control.

## Table of Contents

- [Basic Features](#basic-features)
- [Screen Sync](#screen-sync)
- [Gaming Mode](#gaming-mode)
- [Restore Token](#restore-token)
- [Settings Dialog](#settings-dialog)

---

## Basic Features

### Scene Control
- View and activate any Hue scene from your system tray
- Scenes are organized by room: "Living Room - Relax", "Bedroom - Concentrate"
- One-click activation with desktop notifications for feedback

### Power Control
- Toggle lights on/off for configured room or grouped light
- Quick access from tray menu
- Works with both rooms and Entertainment Areas

### Brightness Control
- Adjust brightness from 1-100%
- Instant feedback via desktop notifications
- Applies to all lights in configured room/group

### Desktop Integration
- Native KDE StatusNotifierItem (system tray) integration
- Desktop notifications for all operations (success/error)
- Auto-start on login via systemd service + autostart desktop file
- DBus interface for external control

---

## Screen Sync

Real-time screen-to-lights synchronization using the Philips Hue Entertainment API.

### How It Works

1. **Screen Capture**: Native PipeWire capture via CGo (Wayland-optimized)
2. **Zone Mapping**: Different screen areas mapped to different lights
3. **Color Extraction**: Average color calculated per zone
4. **DTLS Streaming**: Secure real-time streaming to Hue bridge (Entertainment API v2)

### Configuration

Enable in `~/.openhue/config.yaml`:

```yaml
sync:
  enabled: true
  fps: 30                  # Frame rate (10-60)
  subsampleWidth: 64       # Processing width (lower = faster)
  monitor: ""              # Monitor name (empty = default)
  restoreToken: "..."      # Saved permission token
```

### Zone Mapping

Map screen regions to lights using UV coordinates (0.0-1.0):

```yaml
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}   # Top-left
    uvB: {x: 0.5, y: 1.0}   # Bottom-right (left half of screen)

  - id: 1
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA: {x: 0.5, y: 0.0}   # Top-left (right half start)
    uvB: {x: 1.0, y: 1.0}   # Bottom-right (full right)
```

**Common Mappings:**
- Left/Right split: `uvA: {x: 0.0, y: 0.0}` to `uvB: {x: 0.5, y: 1.0}` (left), `uvA: {x: 0.5, y: 0.0}` to `uvB: {x: 1.0, y: 1.0}` (right)
- Top/Bottom split: Use `uvA.y` and `uvB.y` to define vertical regions
- Three-way split: Divide X axis into thirds (0.0-0.33, 0.33-0.67, 0.67-1.0)

### Performance Tuning

- **FPS**: 20-30 recommended (balance between responsiveness and CPU usage)
- **Subsample Width**: 64 default (lower = faster, less color precision)
- **Monitor**: Leave empty for default, or specify monitor name for multi-monitor setups

### Manual Control

```bash
# Start sync via DBus
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync

# Stop sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StopSync

# Check if syncing
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing
```

---

## Gaming Mode

**Auto-starts screen sync when gaming detected** - perfect for immersive gaming experiences.

### Overview

Gaming Mode automatically:
1. Detects when you start playing a game
2. Waits for debounce period (default 5 seconds)
3. Starts screen sync automatically
4. Stops sync when you exit the game

**No manual intervention required!**

### Configuration

Enable in `~/.openhue/config.yaml`:

```yaml
gamingMode:
  enabled: true              # Feature toggle (disabled by default)
  pollInterval: 2            # Check every N seconds
  debounceDelay: 5           # Wait N seconds before triggering

  # CachyOS-optimized detection (recommended)
  useSystemdInhibit: true    # PRIMARY: systemd-inhibit check
  usePowerProfile: true      # SECONDARY: power profile validation
  useSteamAppId: true        # Steam-specific detection

  # Legacy detection (fallback)
  useGameMode: false         # Feral GameMode (if installed)
```

### Detection Methods

#### 1. systemd-inhibit (CachyOS Primary)
- Detects games via system idle inhibitor locks
- Most reliable on CachyOS and modern systemd distros
- Checks `/proc/<pid>/comm` for game processes

**Games detected:** Steam games, native Linux games, Wine/Proton games

#### 2. Power Profile (CachyOS Secondary)
- Validates gaming state via `power-profiles-daemon`
- Checks if profile is set to "performance"
- Works with CachyOS's automatic profile switching

#### 3. Steam AppId Detection
- Detects Steam games via `STEAM_GAME` environment variable
- Reads `/proc/<pid>/environ` for Steam processes
- Catches games launched directly from Steam

#### 4. Feral GameMode (Legacy)
- Checks if `gamemoded` is running
- Requires Feral GameMode installed
- Fallback for non-CachyOS systems

### How It Works

```
Game starts → Detection(s) trigger → Debounce delay (5s) → Screen sync starts
Game exits  → Detection(s) clear   → Debounce delay (5s) → Screen sync stops
```

The debounce delay prevents flickering from brief detection changes.

### Testing Detection

```bash
# Check current gaming state
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGaming

# Check detector status
journalctl --user -u hue-backend -f | grep "Gaming"
```

**Example log output:**
```
🎮 Gaming state change detected: false → true (waiting for debounce)
🎮 Gaming state changed: true (debounced after 5.002s)
🎮 Gaming detected - starting screen sync
✅ Screen sync enabled for immersive gaming
```

### Manual Override

Gaming Mode respects manual control:
- If you manually stop sync, it won't auto-start again until you restart the backend
- Manual start sync works even with Gaming Mode disabled

### Troubleshoments

**Gaming not detected?**
1. Check detection methods are enabled in config
2. Enable more detectors (especially systemd-inhibit + Steam AppId)
3. Check logs: `journalctl --user -u hue-backend -f | grep Gaming`
4. Verify game is actually running: `ps aux | grep <game-name>`

**False positives?**
1. Increase `debounceDelay` to 10+ seconds
2. Rely on systemd-inhibit alone by disabling `usePowerProfile`, `useSteamAppId` and `useGameMode`

---

## Restore Token

**Eliminates the annoying screen share permission dialog** every time you start screen sync.

### The Problem

By default, Wayland's XDG Desktop Portal shows a permission dialog every time you start screen sync:
- Interrupts gaming/movie watching
- Requires manual approval each time
- Breaks Gaming Mode automation

### The Solution

Restore Token feature:
1. First sync: Dialog appears → You approve → Token saved to config
2. Future syncs: Token reused → No dialog!

### How It Works

1. **First Sync**:
   - Portal shows screen share dialog
   - User selects screen and approves
   - Portal returns `restore_token` in response
   - Backend saves token to `~/.openhue/config.yaml`

2. **Subsequent Syncs**:
   - Backend passes saved token to portal with `persist_mode: 2`
   - Portal validates token
   - Screen capture starts immediately (no dialog)

### Configuration

Token is stored automatically in config:

```yaml
sync:
  restoreToken: "c5082945-ef91-4c83-938a-f6bede8807eb"  # Auto-saved
```

**Do not manually edit this field** - it's managed automatically.

### Verification

After first successful sync, check config:

```bash
grep restoreToken ~/.openhue/config.yaml
```

Should show:
```yaml
restoreToken: <long-uuid-string>
```

Check logs for confirmation:

```bash
journalctl --user -u hue-backend | grep "Screen share permission saved"
```

### Token Lifecycle

- **Saved**: After first successful sync
- **Reused**: Every subsequent sync
- **Invalidated**: If portal session ends or system reboots (varies by compositor)
- **Refreshed**: Backend auto-updates if portal provides new token

### Reset Token

To force new permission dialog (e.g., to change screen):

```bash
# Remove token from config
sed -i '/restoreToken:/d' ~/.openhue/config.yaml

# Restart backend
systemctl --user restart hue-backend

# Next sync will show dialog again
```

### Security

Restore tokens are:
- **Session-specific**: Tied to your user session
- **Screen-specific**: Only grants access to the approved screen
- **Revocable**: Portal can invalidate at any time
- **Stored locally**: In your config file (0600 permissions)

---

## Settings Dialog

GUI configuration interface for all features.

### Access

Right-click tray icon → **Settings**

### Features

#### Bridge Configuration Tab
- Bridge IP address input
- API key management
- Connection test button
- Entertainment API status

#### Room Selection Tab
- List of available rooms
- Entertainment Area selection
- Grouped light selection
- Apply button to save

#### Screen Sync Tab
- Enable/disable screen sync
- FPS slider (10-60)
- Subsample width input
- Monitor selection
- Zone mapping configuration
- Test sync button

#### Gaming Mode Tab
- Enable/disable Gaming Mode
- Poll interval configuration
- Debounce delay setting
- Detection method toggles
- Test detection button

#### Icon Theme Tab
- Gaming icon selector
- Syncing icon selector
- Idle icon selector
- Preview of current icons

### Saving Changes

Click **Apply** or **OK** to save changes. Backend automatically reloads configuration.

### Validation

Dialog validates inputs:
- IP addresses must be valid IPv4 format
- FPS must be 10-60
- Subsample width must be 16-512
- UV coordinates must be 0.0-1.0

---

## Advanced Usage

### DBus Interface

Full DBus interface for scripting and automation.

**Service**: `org.kde.plasma.hue`
**Path**: `/org/kde/plasma/hue`
**Interface**: `org.kde.plasma.hue`

#### Methods

```bash
# Get status
GetStatus() → string

# Scene control
GetScenes() → array of Scene
ActivateScene(string sceneId) → (string result, error)

# Power control
SetPower(bool on) → (bool success, error)

# Brightness control (1-100)
SetBrightness(int value) → (bool success, error)

# Screen sync
StartSync() → error
StopSync() → error
IsSyncing() → bool

# Gaming mode
IsGaming() → bool
GetGamingDetectionMethods() → array of string

# Configuration
GetConfig() → Config struct
UpdateConfig(Config config) → error
ReloadConfig() → error
