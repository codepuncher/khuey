# KDE Hue Control - Features

Complete feature documentation for KDE Hue Control.

## Table of Contents

- [Basic Features](#basic-features)
- [Screen Sync](#screen-sync)
- [Gaming Mode](#gaming-mode)
- [Restore Token](#restore-token)
- [Settings Dialog](#settings-dialog)
- [Advanced Usage](#advanced-usage)

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
  restoreToken: "..."      # Saved permission token
  metricsInterval: 60      # Seconds between metrics lines while syncing (0 = off)
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
  useSystemdInhibit: true    # PRIMARY: systemd-inhibit lock
  usePowerProfile: true      # SECONDARY: needs useSteamAppId too
  useSteamAppId: true        # SECONDARY: needs usePowerProfile too

  # Legacy detection (fallback)
  useGameMode: false         # Feral GameMode (if installed)
```

### Detection Methods

Every `pollInterval` seconds the backend runs the enabled checks and treats the
system as gaming when this holds:

```
systemd-inhibit || (power profile && Steam AppId) || GameMode
```

A check turned off in the config counts as false. Turning off either
`usePowerProfile` or `useSteamAppId` turns off the second term.

#### 1. systemd-inhibit (Primary)
- Runs `systemd-inhibit --list` and looks for the lock CachyOS's `game-performance` wrapper takes, or any lock that mentions "game" or "gaming" in `block` mode
- Start the game through the wrapper: `game-performance <command>`, or `game-performance %command%` as a Steam launch option
- A game started without a wrapper takes no lock and is not seen by this check
- `game-performance` takes no lock when `GAME_PERFORMANCE_SCREENSAVER_ON` is set; it still switches the power profile
- `game-performance` runs the game unchanged, with no lock and no profile switch, when `powerprofilesctl list` has no `performance` profile

#### 2. Power Profile + Steam AppId (Secondary)
- Power profile: `powerprofilesctl get` returns `performance`. `game-performance` sets it for as long as the game runs
- Steam AppId: `pgrep -a reaper` shows a command line containing `AppId=`. Steam starts each game under `reaper SteamLaunch AppId=<id>`
- Both have to be true together. A performance profile set by hand, or a Steam game without it, is not enough

#### 3. Feral GameMode (Fallback)
- Off by default; enable with `useGameMode`
- Calls `QueryStatus` on gamemoded's session bus interface (`com.feralinteractive.GameMode`) and is true while any client holds GameMode, e.g. a game started with `gamemoderun`
- Does not start gamemoded; when it isn't running, this check is false

### How It Works

```
Game starts → Detection(s) trigger → Debounce delay → Screen sync starts
Game exits  → Detection(s) clear   → Debounce delay → Screen sync stops
```

The new state has to hold for `debounceDelay` seconds, measured from the first
poll that sees it and checked on each later poll. With the defaults a change
takes effect 6 seconds after it is first seen. The delay keeps a brief detection
change from starting or stopping sync.

### Testing Detection

```bash
# Current detection result, without the debounce
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeActive

# Watch state changes
journalctl --user -u hue-backend -f | grep -i gaming
```

**Example log output:**
```
[INFO] Gaming state change detected: false → true (waiting for debounce)
[INFO] Gaming state changed: true (debounced after 6.000392601s)
[INFO] Gaming detected - starting screen sync
[INFO] Screen sync enabled for immersive gaming
```

### Manual Override

Gaming Mode respects manual control:
- If you stop sync by hand while a game is detected, it stays stopped until that game ends; the next game starts it again
- Manual start sync works even with Gaming Mode disabled
- If sync is already running when a game is detected, Gaming Mode takes it over and stops it when the game ends

### Troubleshooting

**Gaming not detected?**
1. Check `enabled` and the detection methods in the config
2. Gaming Mode needs the Entertainment API configured; without it the log shows `Gaming mode requires Entertainment API configuration`
3. Check the game holds a lock: `systemd-inhibit --list | grep -i game`. If not, start it through `game-performance`, which needs `powerprofilesctl list` to show a `performance` profile
4. For Steam games without the lock, check `powerprofilesctl get` shows `performance` while the game runs
5. Check logs: `journalctl --user -u hue-backend -f | grep -i gaming`

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

GUI for the settings the tray app can change. Everything else is set in the config file.

### Access

Right-click tray icon → **Settings...**

### Tabs

#### Screen Sync
- Frame Rate: FPS slider and spin box (10-60)
- Processing Quality: subsample width slider and spin box (16-256)
- Capture screen: a **Change capture screen** button. The screen-share portal takes no option naming an output, so the screen is the one picked in its dialog; the button drops the saved grant and the dialog asks again on the next sync start
- Gaming Mode checkbox (the other `gamingMode` settings are config-file only)

#### Light Control
- Room/Zone: the rooms and zones from the bridge, with a **Refresh** button. Power and brightness controls act on the selected one. Activating a scene from the tray selects that scene's room
- Login Behavior: a scene to activate each time the backend starts, or "(Disabled)"

#### Connection
- Bridge IP, read-only; the tab points to the config file for changing the IP or API key
- Connection status and the last error
- **Test Connection** and **Reconnect** buttons, which both check the bridge is reachable

#### Appearance
- Tray icon pickers for three states: Gaming + Sync, Sync Active and Idle
- **Reset to Defaults**: `applications-games`, `media-record` and `preferences-desktop-display-color`

### Saving Changes

**Apply** or **OK** saves the Screen Sync, Light Control and Appearance tabs through the backend's DBus setters, one at a time, and stops at the first one that fails. Each setting takes effect at a different point:
- FPS: immediately, including a running sync
- Subsample width: the next time sync starts
- Gaming Mode: immediately
- Room/Zone: the next power or brightness change
- Login scene: the next backend start
- Tray icons: when the dialog closes

### Validation

The spin boxes only accept FPS 10-60 and subsample width 16-256, and the backend rejects values outside the same ranges.

---

## Advanced Usage

### DBus Interface

The backend's DBus interface, for scripting and automation.

**Service**: `org.kde.plasma.hue`
**Path**: `/org/kde/plasma/hue`
**Interface**: `org.kde.plasma.hue`

Only callers running as the backend's user can use any method except `GetStatus`, `IsSyncing`, `GetSyncSettings`, `IsGamingModeEnabled`, `IsGamingModeActive` and `GetTrayIcons`, which anyone on the session bus can call. A failed call returns a DBus error.

#### Methods

```
# Status
GetStatus() → string                          # "Ready" or "Not configured"
GetState() → (bool power, int brightness, bool success)
GetConnectionStatus() → dict                  # connected, lastError, bridgeIP, lastAttempt
RetryConnection() → bool
TestBridgeConnection() → bool

# Scenes
GetScenes() → array of string                 # "Room Name - Scene Name", or the bare name without a room; sorted by room, then scene
ActivateScene(string sceneName) → string      # a name from GetScenes or a bare scene name; selects the scene's room

# Lights (the selected room or zone)
SetPower(bool on) → bool
SetBrightness(int brightness) → bool          # 0-100
GetGroupedLights() → array of (string id, string name, string type)
GetSelectedRoom() → string
SetSelectedRoom(string roomID) → bool
SetGroupedLight(string groupedLightID) → bool

# Startup scene
GetStartupScene() → string
SetStartupScene(string sceneName) → bool      # "" clears it

# Screen sync
StartSync() → bool
StopSync() → bool
IsSyncing() → bool
GetSyncSettings() → dict                      # fps, subsampleWidth, enabled
SetSyncSettings(int fps, int subsampleWidth) → bool
ResetCaptureSource() → bool                   # portal asks which screen on the next sync start

# Gaming mode
SetGamingMode(bool enabled) → bool
IsGamingModeEnabled() → bool
IsGamingModeActive() → bool                   # current detection result, without the debounce; false while gaming mode is off

# Bridge and tray
GetBridgeSettings() → dict                    # bridgeIP, connected, lastError, configFile
GetTrayIcons() → (string gaming, string syncing, string idle)
SetTrayIcons(string gaming, string syncing, string idle) → bool
```

`StartSync` waits for the screen-share dialog when there is no valid restore token, and `dbus-send` gives up after 25 seconds while the backend keeps waiting.
