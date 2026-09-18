# KDE Hue Control - Configuration Reference

Complete reference guide for configuring KDE Hue Control (khuey). This document covers every configuration option, UV coordinate mapping, Entertainment API setup, and gaming mode configuration.

## Table of Contents

1. [Overview](#overview)
2. [Configuration File Location](#configuration-file-location)
3. [File Format & Structure](#file-format--structure)
4. [Core Configuration](#core-configuration)
5. [Screen Sync Settings](#screen-sync-settings)
6. [Gaming Mode Settings](#gaming-mode-settings)
7. [Entertainment API Configuration](#entertainment-api-configuration)
8. [UV Coordinate System](#uv-coordinate-system)
9. [UI Settings](#ui-settings)
10. [Logging Configuration](#logging-configuration)
11. [Example Configurations](#example-configurations)
12. [Validation Rules](#validation-rules)
13. [Troubleshooting Common Config Issues](#troubleshooting-common-config-issues)
14. [Migration & Compatibility](#migration--compatibility)

---

## Overview

KDE Hue Control uses a YAML configuration file that is **shared with openhue-cli** for compatibility. The configuration controls:

- **Bridge Connection**: IP address and API credentials
- **Light Control**: Grouped lights for power/brightness operations
- **Screen Sync**: Real-time screen-to-light synchronization
- **Gaming Mode**: Automatic sync detection during gameplay
- **Entertainment API**: Multi-zone screen mapping with UV coordinates
- **UI Customization**: Tray icon themes
- **Logging**: Debug and diagnostic output

**When configuration changes take effect:**

The backend reads this file once at startup, so an edit made by hand applies on
the next backend restart (`systemctl --user restart hue-backend`).

Some settings changed in the tray settings dialog, which writes this file for
you, apply sooner:
- Immediately: tray icons, selected room, gaming mode on or off
- While sync runs: screen sync FPS
- On the next sync: subsample width
- On the next backend start: startup scene

---

## Configuration File Location

The backend reads the same file as openhue-cli:

- `$XDG_CONFIG_HOME/openhue/config.yaml` when `XDG_CONFIG_HOME` is set
- `~/.openhue/config.yaml` otherwise

When `XDG_CONFIG_HOME` is set, `~/.openhue` is not read, even if `$XDG_CONFIG_HOME/openhue/config.yaml` doesn't exist. openhue-cli does the same. The examples in this document use `~/.openhue`.

The backend runs as the `hue-backend` systemd user unit, which starts at login before Plasma copies the login shell's environment into the systemd user manager. An `XDG_CONFIG_HOME` exported in a shell profile therefore misses the backend started at login but reaches one restarted later, so the file it reads changes between a login and a restart. Set it in a `.conf` file under `~/.config/environment.d/` instead, then run `systemctl --user daemon-reload` and restart `hue-backend`.

The tray's settings dialog shows the file the running backend loaded, on the Connection tab, and the backend logs it at startup as `Configuration: <path>`. To see the running backend's `XDG_CONFIG_HOME`:

```bash
tr '\0' '\n' < /proc/$(systemctl --user show -p MainPID --value hue-backend)/environ | grep XDG_CONFIG_HOME
```

The `openhue-go` library's `LoadConf` reads only `~/.openhue/config.yaml`. The backend loads its config itself and doesn't call `LoadConf`. Don't change the lookup to match `openhue-go`: with `XDG_CONFIG_HOME` set, the backend and openhue-cli would read different files.

### Directory Structure

```
~/.openhue/
└── config.yaml          # Main configuration file, including the screen capture restore token (sync.restoreToken)
```

### File Permissions

**Required:** `0600` (owner read/write only)

The config file contains sensitive API keys and should only be readable by the owner. The backend will:
- **Warn** if permissions are too permissive (world/group readable)
- **Automatically secure** the file to `0600` when saving

```bash
# Fix permissions manually
chmod 600 ~/.openhue/config.yaml
```

### Creating the Configuration

**First-time setup:**

```bash
# Method 1: Use openhue-cli (recommended)
openhue setup

# Method 2: Create manually
mkdir -p ~/.openhue
cat > ~/.openhue/config.yaml <<EOF
Bridge: "192.168.1.X"
Key: "YOUR-API-KEY"
EOF
chmod 600 ~/.openhue/config.yaml
```

---

## File Format & Structure

KDE Hue Control uses **YAML** format for human-readable configuration.

### Top-Level Structure

```yaml
version: 1                              # Config format version
Bridge: "192.168.1.100"                 # Bridge IP address
Key: "YOUR-API-KEY"                     # API key
grouped_light_id: "room-1"              # Default room/zone for controls
clientkey: "CLIENT-KEY"                 # Entertainment API key
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"  # Entertainment Area ID
startupScene: ""                        # Scene activated when the backend starts
log_level: "info"                       # Logging verbosity

sync:                                   # Screen sync settings
  enabled: false
  fps: 30
  subsampleWidth: 64
  restoreToken: ""
  metricsInterval: 60

gamingMode:                             # Gaming mode settings
  enabled: false
  pollInterval: 2
  debounceDelay: 5
  useSystemdInhibit: true
  usePowerProfile: true
  useSteamAppId: true
  useGameMode: false

ui:                                     # UI customization
  icons:
    gaming: "applications-games"
    syncing: "media-record"
    idle: "preferences-desktop-display-color"

channels:                               # Entertainment API channels
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}
```

### YAML Syntax Notes

- **Strings**: Quote values containing special characters
- **Booleans**: `true` or `false` (lowercase)
- **Numbers**: No quotes needed
- **Comments**: Start with `#`
- **Indentation**: 2 spaces (not tabs)
- **Nested objects**: Use indentation or inline `{key: value}` syntax

---

## Core Configuration

Core settings for bridge connection and basic operation.

### Bridge Connection

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `version` | int | No | `1` | Config format version (for migration) |
| `Bridge` | string | **Yes** | none | Bridge IP address (e.g., `"192.168.1.100"`) |
| `Key` | string | **Yes** | none | API key for authentication |
| `clientkey` | string | No* | `""` | Client key for Entertainment API (required for screen sync) |
| `entertainmentConfigurationId` | string | No* | `""` | Entertainment Area ID (required for screen sync) |
| `grouped_light_id` | string | No | `""` | Room or zone ID for power/brightness controls |
| `startupScene` | string | No | `""` | Scene activated each time the backend starts (empty disables) |

**\*Required for Screen Sync feature**

#### Bridge (Required)

**Type:** String (IP address)
**Format:** `"192.168.1.X"` or hostname
**Example:** `"192.168.1.100"`

The IP address of your Philips Hue Bridge on your local network.

**How to find:**
```bash
# Discovery via openhue-cli
openhue discover

# Manual discovery via mDNS
avahi-browse -rt _hue._tcp

# Check router DHCP leases
# Look for device named "Philips Hue" or "ecb5fa..."
```

**Troubleshooting:**
- Bridge IP must be reachable from the machine running khuey
- Use static IP or DHCP reservation to prevent IP changes
- If bridge IP changes, update config and restart backend

#### Key (Required)

**Type:** String (hexadecimal)
**Format:** 40-character alphanumeric string
**Example:** `"ABCDEFabcdef1234567890-xYzAbC_DeF"`

The API key (username) for authenticating with the Hue Bridge.

**How to generate:**
```bash
# Using openhue-cli (recommended): pairs with the bridge and saves Bridge and Key.
# It mints a new user, so any clientkey already in the config stops matching;
# use ./cmd/register-entertainment below to get a Key and clientkey as a pair.
openhue setup

# Manual registration (press bridge button first!)
curl -X POST http://192.168.1.100/api \
  -d '{"devicetype":"khuey#user"}'
```

**Security:**
- Never commit API keys to version control
- Never share API keys publicly
- Treat as a password (grants full bridge access)
- Config file should be `0600` permissions

#### clientkey (Required for Entertainment API)

**Type:** String (hexadecimal)
**Format:** 32-character alphanumeric string
**Example:** `"ABCDEF1234567890ABCDEF1234567890"`

The client key enables DTLS encryption for Entertainment API streaming.

**How to generate:**
```bash
# Using khuey's registration tool. It registers a new bridge user, so copy both
# the Key and the clientkey it prints: the Key is the DTLS identity for that
# clientkey. Add -bridge <ip> when the config has no Bridge yet.
cd backend
go run ./cmd/register-entertainment

# Manual registration (press bridge button first!)
curl -X POST http://192.168.1.100/api \
  -d '{"devicetype":"khuey#user","generateclientkey":true}'
```

**When required:**
- Screen Sync feature (Entertainment API)
- Not needed for basic scene/light control

#### entertainmentConfigurationId

**Type:** String (UUID format)
**Format:** Bare UUID, no `entertainment_configuration/` prefix
**Example:** `"550e8400-e29b-41d4-a716-446655440000"`

The ID of the Entertainment Area (group of lights) to use for screen sync.

**How to find:**
```bash
# List available Entertainment Areas
cd backend
go run ./cmd/get-entertainment-info

# Entertainment Areas themselves are created in the Hue app
```

**Requirements:**
- Must be created in the official Hue app first
- Entertainment Area must contain lights you want to sync
- Lights in the Entertainment Area determine available channels

#### grouped_light_id

**Type:** String (resource ID)
**Format:** Room or zone ID from Hue Bridge API
**Example:** `"room/living-room"` or `"zone/desk-area"`

The grouped light (room or zone) to control with power/brightness buttons.

**How to find:**
```bash
# Via Settings Dialog
# Right-click tray icon → Settings → Light Control tab → Room/Zone dropdown

# Via openhue-cli
openhue get /clip/v2/resource/room
openhue get /clip/v2/resource/zone
```

**When to set:**
- Required for power on/off button to work
- Required for brightness slider to work
- Optional if you only use scene control

---

### Startup Scene

#### startupScene

**Type:** String (scene display name)
**Default:** `""` (disabled)
**Format:** `"Room - Scene"`, or a bare scene name
**Example:** `"Living Room - Relax"`

The scene to activate each time the backend starts, which includes login and
every `systemctl --user restart hue-backend`. An empty string turns the feature
off, and the lights are left as they are.

The name is matched against the scenes on the bridge, against the full
`"Room - Scene"` display name and against the bare scene name. Nothing checks
that the scene exists when it is saved, so a name matching none of them fails at
the next start: the backend logs `Scene not found: <name>` and leaves the lights
alone.

**How to set:**
```bash
# Via Settings Dialog
# Right-click tray icon → Settings → Light Control tab → Activate scene on login

# Via DBus
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue /org/kde/plasma/hue \
  org.kde.plasma.hue.SetStartupScene string:"Living Room - Relax"

# Scene names come from GetScenes, in the same format
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue /org/kde/plasma/hue \
  org.kde.plasma.hue.GetScenes
```

Activation runs in the background, so a slow bridge does not hold up startup.
`grouped_light_id` is left alone, so the room picked for power and brightness
does not follow the startup scene.

---

## Screen Sync Settings

Configuration for real-time screen-to-lights synchronization.

### Sync Configuration

| Option | Type | Default | Range/Values | Description |
|--------|------|---------|--------------|-------------|
| `sync.enabled` | bool | `false` | `true`/`false` | Enable/disable screen sync on startup |
| `sync.fps` | int | `30` | `10` - `60` | Frame rate for screen capture and streaming |
| `sync.subsampleWidth` | int | `64` | `16` - `256` | Resize width for processing (performance tuning) |
| `sync.restoreToken` | string | `""` | Portal token | XDG Portal restore token (auto-generated) |
| `sync.metricsInterval` | int | `60` | `0` - `3600` | Seconds between performance metrics log lines while syncing (`0` turns them off) |

#### sync.enabled

**Type:** Boolean
**Default:** `false`
**Values:** `true` or `false`

Whether to automatically start screen sync when the backend starts.

**Performance Impact:** Moderate (5-15% CPU when active)

**Example:**
```yaml
sync:
  enabled: false  # Start manually via tray app (recommended)
```

**Recommendations:**
- Set to `false` for manual control
- Set to `true` if you always want sync active
- Gaming mode can auto-start sync regardless of this setting

#### sync.fps

**Type:** Integer
**Default:** `30`
**Range:** `10` - `60` FPS
**Validation:** Values of `1` - `9` are raised to `10` on load, with a warning.
The file itself is left alone. Anything else outside the range fails config
load.

Frame rate for screen capture and Entertainment API streaming.

**Performance Impact:**
- **10 FPS**: ~2-3% CPU (battery-friendly, noticeable lag)
- **20 FPS**: ~4-6% CPU (smooth, power-efficient)
- **30 FPS**: ~6-10% CPU (very smooth, balanced - **recommended**)
- **60 FPS**: ~12-20% CPU (buttery smooth, high power draw)

**Latency:**
- **10 FPS**: 100ms per frame (visible delay)
- **20 FPS**: 50ms per frame (slight delay)
- **30 FPS**: 33ms per frame (imperceptible)
- **60 FPS**: 16ms per frame (instant)

**Example:**
```yaml
sync:
  fps: 30  # Balanced performance
```

**Recommendations:**
- **Gaming/Movies**: 30 FPS (smooth, responsive)
- **Background ambient**: 20 FPS (sufficient, saves power)
- **Laptops/battery**: 20 FPS or lower
- **Desktop/AC power**: 30-60 FPS
- **Avoid**: Below 15 FPS (choppy, distracting)

**Troubleshooting:**
- If lights stutter: Lower FPS or increase subsampleWidth
- If CPU usage too high: Lower FPS first, then subsampleWidth
- If colors lag behind screen: Increase FPS

#### sync.subsampleWidth

**Type:** Integer
**Default:** `64`
**Range:** `16` - `256` pixels
**Validation:** Must be within range or config load fails

Width to resize captured frames before color processing.

**How it works:**
1. Capture full-resolution frame (e.g., 2560x1440)
2. Resize to subsampleWidth while maintaining aspect ratio (e.g., 64x36)
3. Extract colors from UV zones
4. Stream to lights

**Performance Impact:**
- **16**: ~1-2% CPU (very fast, blocky colors)
- **32**: ~2-4% CPU (fast, acceptable colors)
- **64**: ~4-8% CPU (balanced - **recommended**)
- **128**: ~8-12% CPU (high quality, slower)
- **256**: ~15-25% CPU (excellent quality, expensive)

**Color Accuracy:**
- **16-32**: Good for solid colors, poor for gradients
- **64**: Good balance for most content
- **128-256**: Excellent for color-graded media, overkill for games

**Example:**
```yaml
sync:
  subsampleWidth: 64  # Default balanced setting
```

**Recommendations:**
- **Default**: 64 (good balance)
- **Performance**: 32-48 (lower CPU, acceptable quality)
- **Quality**: 96-128 (movies, color-graded content)
- **Maximum**: 256 (diminishing returns beyond 128)

**Troubleshooting:**
- If colors inaccurate: Increase subsampleWidth
- If CPU usage too high: Decrease subsampleWidth
- If lights flicker: May need to decrease (faster processing)

#### Which screen is captured

There is no option for this. The screen-share portal decides, and its
`SelectSources` call takes no key naming an output, so the screen is whichever
one you picked in the portal's dialog. That choice is remembered in
`sync.restoreToken`, which is why the dialog only appears once.

To capture a different screen, open the tray settings dialog and use **Change
capture screen** on the Screen Sync tab. It drops the saved grant, so the portal
asks again the next time sync starts. Editing `sync.restoreToken` by hand does
not work while the backend is running: the next config save writes the running
backend's copy back over the file.

Only one screen is captured at a time. The portal is asked for a single source
(`multiple: false`), and the capture package reads the first stream it
returns.

#### sync.restoreToken

**Type:** String
**Default:** `""` (empty)
**Format:** Auto-generated Portal token
**Example:** Long alphanumeric string

XDG Desktop Portal restore token that eliminates the screen share permission dialog on subsequent runs.

**How it works:**
1. First screen sync: User approves permission dialog
2. Portal generates restore token
3. Backend saves token to config
4. Subsequent runs: Token used instead of showing dialog

**Behavior:**
- **Empty**: Permission dialog shown every time (default)
- **Set**: Dialog skipped if token valid
- **Auto-generated**: Backend updates this field automatically

**Example:**
```yaml
sync:
  restoreToken: ""  # Empty on first run
  # Becomes populated after first sync:
  # restoreToken: "portal_restore_abc123..."
```

**Troubleshooting:**
- If dialog reappears: Token expired, will regenerate
- To force new dialog: **Change capture screen** in the settings dialog
- Token is session-specific, may expire after logout/reboot

**Security:**
- Token grants screen capture permission
- Stored in config file (should be 0600 permissions)
- Portal manages token expiration and revocation

#### sync.metricsInterval

**Type:** Integer
**Default:** `60`
**Range:** `0` - `3600` seconds

How often the sync engine logs its performance line while syncing. `0` turns
the line off; other values are a lower bound, since the line is emitted from the
capture loop and only when a frame has been processed.

One line per interval, for example:

```text
[INFO] Sync: 30.0/30 fps, frame avg=4.21ms p50=4 p95=7 p99=12, capture=2.10ms extract=1.05ms stream=1.02ms, 0 dropped (60.0s, 1800 frames)
```

The counters are cumulative for the sync session, not per interval, and reset
when sync starts. Dropped frames also get their own throttled `[WARN] Frame
skip` line, so setting this to `0` does not hide them.

The backend reads this file at startup only, so an edit here takes effect when
the backend restarts.

**Example:**
```yaml
sync:
  metricsInterval: 60  # Default
  # metricsInterval: 0     # Off
  # metricsInterval: 300   # Once every five minutes
```

**Recommendations:**
- Lower it while tuning `fps` or `subsampleWidth`, then put it back. Each change
  needs a backend restart
- Raise it or set `0` if the backend dominates your journal. journald bounds the
  journal on disk with `SystemMaxUse`, so a busy sync log shortens how far back
  every other message survives

---

## Gaming Mode Settings

Configuration for automatic screen sync detection during gameplay.

### Gaming Mode Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `gamingMode.enabled` | bool | `false` | Enable/disable gaming mode feature |
| `gamingMode.pollInterval` | int | `2` | How often to check for games (seconds) |
| `gamingMode.debounceDelay` | int | `5` | Wait time before triggering (seconds) |
| `gamingMode.useSystemdInhibit` | bool | `true` | Use systemd-inhibit detection (CachyOS) |
| `gamingMode.usePowerProfile` | bool | `true` | Use power profile validation |
| `gamingMode.useSteamAppId` | bool | `true` | Use Steam AppId detection |
| `gamingMode.useGameMode` | bool | `false` | Use Feral GameMode DBus (legacy) |

#### gamingMode.enabled

**Type:** Boolean
**Default:** `false` (opt-in feature)
**Values:** `true` or `false`

Master toggle for gaming mode feature. When enabled, automatically starts screen sync when games are detected.

**Performance Impact:** Minimal when idle (~0.1% CPU polling)

**Example:**
```yaml
gamingMode:
  enabled: true  # Enable automatic gaming detection
```

**How it works:**
1. Background polling checks for gaming activity
2. When game detected: Waits debounceDelay seconds
3. Auto-starts screen sync
4. When game exits: Auto-stops screen sync
5. Returns to idle state

**Benefits:**
- No manual sync toggling
- Immersive gaming lighting
- Automatic power management

**Recommendations:**
- Enable if you game frequently
- Disable if you prefer manual control
- Works great with Steam, Lutris, Heroic Launcher

#### gamingMode.pollInterval

**Type:** Integer
**Default:** `2` seconds
**Range:** `1` - `30` seconds (practical range)

How frequently to check for gaming activity.

**Performance Impact:**
- **1s**: ~0.2% CPU (responsive, higher overhead)
- **2s**: ~0.1% CPU (balanced - **recommended**)
- **5s**: ~0.05% CPU (slower response, efficient)
- **10s**: ~0.02% CPU (delayed detection, minimal impact)

**Example:**
```yaml
gamingMode:
  pollInterval: 2  # Check every 2 seconds
```

**Recommendations:**
- **Default**: 2 seconds (good balance)
- **Responsive**: 1 second (for instant detection)
- **Battery-saving**: 5-10 seconds (delayed but efficient)

**Tradeoffs:**
- **Lower values**: Faster game detection, more CPU overhead
- **Higher values**: Slower detection, less overhead

#### gamingMode.debounceDelay

**Type:** Integer
**Default:** `5` seconds
**Range:** `0` - `60` seconds (practical range)

Wait time after game detection before starting screen sync.

**Purpose:** Prevents false positives from quickly launching and closing games.

**Example:**
```yaml
gamingMode:
  debounceDelay: 5  # Wait 5 seconds before starting sync
```

**Recommendations:**
- **Default**: 5 seconds (prevents most false positives)
- **Aggressive**: 2-3 seconds (faster start, more false positives)
- **Conservative**: 10-15 seconds (slow start, no false positives)
- **Instant**: 0 seconds (not recommended, many false positives)

**Behavior:**
- Game detected → Timer starts
- If game still running after delay → Start sync
- If game exits before delay → Cancel, don't start sync

#### Detection Methods

Gaming mode supports **multiple detection methods** that work together. Enable methods appropriate for your system.

##### gamingMode.useSystemdInhibit

**Type:** Boolean
**Default:** `true`
**Platform:** CachyOS, systemd-based systems

Detects games via systemd inhibitor locks. **Primary detection method for CachyOS.**

**How it works:**
- CachyOS game-performance creates inhibitor locks when games run
- Detector checks `systemd-inhibit --list` output
- Most reliable method on CachyOS

**Example:**
```yaml
gamingMode:
  useSystemdInhibit: true  # CachyOS primary detection
```

**When to enable:**
- Running CachyOS
- Running systemd-based distro with game optimizations
- Want most reliable detection

**When to disable:**
- Not using systemd
- Using non-CachyOS without game profiles

##### gamingMode.usePowerProfile

**Type:** Boolean
**Default:** `true`
**Platform:** All Linux distributions with power-profiles-daemon

Validates gaming activity via power profile changes.

**How it works:**
- Checks if power profile is set to "performance"
- Games often trigger performance profile automatically
- Used as **secondary validation** with other methods

**Example:**
```yaml
gamingMode:
  usePowerProfile: true  # Secondary validation
```

**When to enable:**
- System has power-profiles-daemon
- Want additional validation
- Using CachyOS (combines with systemd-inhibit)

**When to disable:**
- No power-profiles-daemon installed
- Manually set performance profile for other reasons

##### gamingMode.useSteamAppId

**Type:** Boolean
**Default:** `true`
**Platform:** Systems with Steam installed

Detects running Steam games via AppId in process names.

**How it works:**
- Scans process list for patterns like `reaper.exe` (Steam game process)
- Checks for Steam runtime processes
- Highly accurate for Steam games

**Example:**
```yaml
gamingMode:
  useSteamAppId: true  # Detect Steam games
```

**When to enable:**
- Play Steam games
- Want Steam-specific detection
- Using Steam Runtime/Proton

**When to disable:**
- Don't use Steam
- Only play non-Steam games

##### gamingMode.useGameMode (Legacy)

**Type:** Boolean
**Default:** `false`
**Platform:** Systems with Feral GameMode installed

Detects games via Feral GameMode DBus interface.

**How it works:**
- Queries `com.feralinteractive.GameMode` DBus service
- Checks if GameMode is active
- Legacy method, replaced by systemd-inhibit on CachyOS

**Example:**
```yaml
gamingMode:
  useGameMode: false  # Disabled by default
```

**When to enable:**
- Have Feral GameMode installed
- Games launched with `gamemoderun`
- Not using CachyOS

**When to disable:**
- GameMode not installed
- Using CachyOS (systemd-inhibit is better)
- Games don't use GameMode

### Detection Priority

Gaming mode uses a **tiered priority system** for reliable detection:

**Tier 1 (Highest confidence):**
1. `useSystemdInhibit` (CachyOS game-performance)

**Tier 2 (High confidence):**
2. `usePowerProfile` + `useSteamAppId` (combined)

**Tier 3 (Fallback):**
3. `useGameMode` (if installed)

**Recommended configuration:**

```yaml
# CachyOS (recommended)
gamingMode:
  enabled: true
  useSystemdInhibit: true   # Primary
  usePowerProfile: true     # Secondary validation
  useSteamAppId: true       # Steam-specific
  useGameMode: false        # Not installed

# Non-CachyOS with Steam
gamingMode:
  enabled: true
  useSystemdInhibit: false  # Not available
  usePowerProfile: true     # Available
  useSteamAppId: true       # Primary for Steam
  useGameMode: true         # If installed

# Non-CachyOS without Steam
gamingMode:
  enabled: true
  useSystemdInhibit: false
  usePowerProfile: false
  useSteamAppId: false
  useGameMode: true         # Primary method
```

---

## Entertainment API Configuration

Configuration for multi-zone screen mapping with Entertainment API.

### Channel Configuration

Entertainment API uses **channels** to map screen regions to individual lights. Each channel controls one light in your Entertainment Area.

| Option | Type | Required | Range/Values | Description |
|--------|------|----------|--------------|-------------|
| `id` | uint8 | **Yes** | `0` - `255` | Channel index (matches Entertainment Area) |
| `active` | bool | **Yes** | `true`/`false` | Whether this channel is active |
| `deviceName` | string | No | Any string | Human-readable light name |
| `gammaFactor` | float32 | **Yes** | `0.5` - `4.0` | Gamma correction factor |
| `uvA` | UV | **Yes** | See UV section | Top-left corner of screen region |
| `uvB` | UV | **Yes** | See UV section | Bottom-right corner of screen region |

#### Channel Structure

```yaml
channels:
  - id: 0                    # First light in Entertainment Area
    active: true             # Enable this channel
    deviceName: "Left Light" # Optional friendly name
    gammaFactor: 2.2         # Standard gamma correction
    uvA: {x: 0.0, y: 0.0}   # Top-left corner
    uvB: {x: 0.5, y: 1.0}   # Bottom-right corner (left half)

  - id: 1                    # Second light
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA: {x: 0.5, y: 0.0}   # Top-left (starts at middle)
    uvB: {x: 1.0, y: 1.0}   # Bottom-right (right half)
```

#### id

**Type:** Unsigned 8-bit integer
**Range:** `0` - `255`
**Required:** Yes

The channel index in the Entertainment Area. Must match the light's position in the Entertainment Area configuration.

**How to determine:**
```bash
# List Entertainment Area channels
cd backend
go run ./cmd/get-entertainment-info

# Shows:
# Channel 0: Hue Play 1 (Left)
# Channel 1: Hue Play 2 (Right)
# Channel 2: Hue Go (Center)
```

**Example:**
```yaml
channels:
  - id: 0  # First light in Entertainment Area
  - id: 1  # Second light
  - id: 2  # Third light
```

**Important:**
- ID must match Entertainment Area channel order
- IDs should be sequential starting from 0
- Missing IDs are skipped (not controlled)
- Duplicate IDs are invalid (validation error)

#### active

**Type:** Boolean
**Required:** Yes
**Values:** `true` or `false`

Whether this channel is enabled for screen sync.

**Example:**
```yaml
channels:
  - id: 0
    active: true   # This light will sync
  - id: 1
    active: false  # This light will not sync (stays off)
```

**Use cases:**
- Disable specific lights without removing from config
- Test different zone configurations
- Temporarily disable lights for troubleshooting

#### deviceName

**Type:** String
**Required:** No (but recommended)
**Default:** Empty string

Human-readable name for the light. Used for logging and debugging.

**Example:**
```yaml
channels:
  - id: 0
    deviceName: "Left Hue Play"
  - id: 1
    deviceName: "Right Hue Play"
  - id: 2
    deviceName: "TV Backlight Go"
```

**Recommendations:**
- Use descriptive names matching physical location
- Include light type if helpful
- Keep names short (< 20 characters)

#### gammaFactor

**Type:** Float (32-bit)
**Required:** Yes
**Range:** `0.5` - `4.0`
**Default:** `2.2` (standard sRGB gamma)
**Validation:** Must be within range or config load fails

Gamma correction factor for color processing. Adjusts brightness curve to match display characteristics.

**Common values:**
- **1.0**: No gamma correction (linear)
- **2.0**: Slight gamma correction
- **2.2**: Standard sRGB gamma (**recommended for most displays**)
- **2.4**: BT.1886 gamma (HDR/cinema displays)
- **1.8**: Mac displays (older)

**Example:**
```yaml
channels:
  - id: 0
    gammaFactor: 2.2  # Standard sRGB (default)
```

**What it does:**
- **< 2.2**: Darker colors, brighter lights (more contrast)
- **= 2.2**: Matches standard display (**recommended**)
- **> 2.2**: Brighter colors, darker lights (less contrast)

**When to adjust:**
- If lights appear too dim: Decrease to 1.8-2.0
- If lights appear too bright: Increase to 2.4-2.8
- If colors don't match screen: Usually keep at 2.2

**Troubleshooting:**
- **Lights too dim**: Gamma too high → Decrease
- **Lights too bright**: Gamma too low → Increase
- **Washed out colors**: Try 2.2 first
- **Per-light adjustment**: Each channel can have different gamma

#### uvA and uvB

**Type:** UV coordinate object
**Required:** Yes
**Format:** `{x: float, y: float}`
**Range:** `0.0` - `1.0` for both x and y
**Validation:** See [Validation Rules](#validation-rules)

UV coordinates define the screen region (zone) that controls this light.

- **uvA**: Top-left corner of the zone
- **uvB**: Bottom-right corner of the zone

**See [UV Coordinate System](#uv-coordinate-system) for complete details.**

**Example:**
```yaml
channels:
  - id: 0  # Left half
    uvA: {x: 0.0, y: 0.0}  # Top-left corner of screen
    uvB: {x: 0.5, y: 1.0}  # Middle-bottom of screen

  - id: 1  # Right half
    uvA: {x: 0.5, y: 0.0}  # Middle-top of screen
    uvB: {x: 1.0, y: 1.0}  # Bottom-right corner of screen
```

---

## UV Coordinate System

UV coordinates are a **monitor-agnostic** way to define screen regions. Instead of pixel coordinates, UV uses normalized coordinates from `0.0` to `1.0`.

### Coordinate System

```
    U (X-axis) →
   0.0                     1.0
V  ┌─────────────────────────┐  0.0
(Y │                         │
│  │                         │
a  │       SCREEN            │
x  │                         │
i  │                         │
s) └─────────────────────────┘  1.0
```

### Axis Definition

| Axis | Direction | Range | Description |
|------|-----------|-------|-------------|
| **U (x)** | Horizontal | `0.0` - `1.0` | `0.0` = left edge, `1.0` = right edge |
| **V (y)** | Vertical | `0.0` - `1.0` | `0.0` = top edge, `1.0` = bottom edge |

### Zone Definition

A **zone** is a rectangular screen region defined by two UV coordinates:

- **uvA**: Top-left corner `{x, y}`
- **uvB**: Bottom-right corner `{x, y}`

**Requirements:**
- `uvA.x < uvB.x` (left edge is less than right edge)
- `uvA.y < uvB.y` (top edge is less than bottom edge)
- All values must be `0.0` - `1.0`

### Visual Examples

#### Two-Zone Layout (Left/Right Split)

```
┌───────────┬───────────┐
│           │           │
│   Zone 0  │  Zone 1   │
│  (Left)   │  (Right)  │
│           │           │
└───────────┴───────────┘
0.0        0.5         1.0
```

**Configuration:**
```yaml
channels:
  - id: 0  # Left light
    uvA: {x: 0.0, y: 0.0}   # Top-left corner
    uvB: {x: 0.5, y: 1.0}   # Bottom-right at middle

  - id: 1  # Right light
    uvA: {x: 0.5, y: 0.0}   # Top-middle
    uvB: {x: 1.0, y: 1.0}   # Bottom-right corner
```

#### Three-Zone Layout (Left/Center/Right)

```
┌──────┬────────┬──────┐
│      │        │      │
│ Z0   │   Z1   │  Z2  │
│(Left)│(Center)│(Right│
│      │        │      │
└──────┴────────┴──────┘
0.0   0.33    0.67    1.0
```

**Configuration:**
```yaml
channels:
  - id: 0  # Left
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.33, y: 1.0}

  - id: 1  # Center
    uvA: {x: 0.33, y: 0.0}
    uvB: {x: 0.67, y: 1.0}

  - id: 2  # Right
    uvA: {x: 0.67, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

#### Four-Zone Layout (Corners)

```
┌──────────┬──────────┐
│  Zone 0  │  Zone 1  │
│(Top-Left)│(Top-Right│
├──────────┼──────────┤
│  Zone 2  │  Zone 3  │
│(Bot-Left)│(Bot-Right│
└──────────┴──────────┘
```

**Configuration:**
```yaml
channels:
  - id: 0  # Top-left
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 0.5}

  - id: 1  # Top-right
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 0.5}

  - id: 2  # Bottom-left
    uvA: {x: 0.0, y: 0.5}
    uvB: {x: 0.5, y: 1.0}

  - id: 3  # Bottom-right
    uvA: {x: 0.5, y: 0.5}
    uvB: {x: 1.0, y: 1.0}
```

#### TV Backlight Layout (Edge Mapping)

```
     ┌─────────┐
     │  TOP    │
     └─────────┘
┌──┐           ┌──┐
│L │  SCREEN   │R │
│E │           │I │
│F │           │G │
│T │           │H │
│  │           │T │
└──┘           └──┘
     ┌─────────┐
     │ BOTTOM  │
     └─────────┘
```

**Configuration:**
```yaml
channels:
  - id: 0  # Left edge
    uvA: {x: 0.0, y: 0.2}
    uvB: {x: 0.1, y: 0.8}

  - id: 1  # Top edge
    uvA: {x: 0.2, y: 0.0}
    uvB: {x: 0.8, y: 0.1}

  - id: 2  # Right edge
    uvA: {x: 0.9, y: 0.2}
    uvB: {x: 1.0, y: 0.8}

  - id: 3  # Bottom edge
    uvA: {x: 0.2, y: 0.9}
    uvB: {x: 0.8, y: 1.0}
```

### Resolution Independence

UV coordinates work on **any resolution** because they're normalized:

| Resolution | Zone "Left Half" | Actual Pixels |
|------------|------------------|---------------|
| 1920x1080 | `uvA: {0.0, 0.0}`, `uvB: {0.5, 1.0}` | 0-960px, 0-1080px |
| 2560x1440 | `uvA: {0.0, 0.0}`, `uvB: {0.5, 1.0}` | 0-1280px, 0-1440px |
| 3840x2160 | `uvA: {0.0, 0.0}`, `uvB: {0.5, 1.0}` | 0-1920px, 0-2160px |

**Benefits:**
- Config works on any monitor size
- Switching monitors doesn't require reconfiguration
- Multi-monitor setups can share config
- Portable across different systems

### Zone Overlap and Gaps

#### Non-Overlapping Zones (Recommended)

```yaml
# Clean split at x=0.5 (no overlap, no gap)
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}  # Ends at 0.5
  - id: 1
    uvA: {x: 0.5, y: 0.0}  # Starts at 0.5
    uvB: {x: 1.0, y: 1.0}
```

#### Overlapping Zones (Advanced)

```yaml
# Overlap in the middle for smoother transitions
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.6, y: 1.0}  # Extends past 0.5
  - id: 1
    uvA: {x: 0.4, y: 0.0}  # Starts before 0.5
    uvB: {x: 1.0, y: 1.0}
# Overlap region: 0.4-0.6 (both lights see middle colors)
```

**Use cases for overlap:**
- Smoother color transitions between zones
- Ambient lighting effects
- Reduce harsh boundaries

**Tradeoffs:**
- More processing (overlapping regions processed twice)
- Colors in overlap zone affect multiple lights

#### Gaps Between Zones (Not Recommended)

```yaml
# Gap between 0.4-0.6 (middle 20% ignored)
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.4, y: 1.0}  # Ends at 0.4
  - id: 1
    uvA: {x: 0.6, y: 0.0}  # Starts at 0.6
    uvB: {x: 1.0, y: 1.0}
# Gap: 0.4-0.6 region is not captured
```

**Why gaps are bad:**
- Wastes screen real estate
- Middle content ignored (e.g., important action in center)
- Lights don't represent full screen

### Advanced UV Techniques

#### Small Hotspot Zones

Focus on specific screen areas (e.g., minimap, health bar):

```yaml
channels:
  - id: 0  # Small zone in top-right (minimap)
    uvA: {x: 0.8, y: 0.0}
    uvB: {x: 1.0, y: 0.2}
```

#### Weighted Zones

Different sizes based on importance:

```yaml
# Gaming setup: Focus on center action
channels:
  - id: 0  # Left (10%)
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.1, y: 1.0}

  - id: 1  # Center (80%)
    uvA: {x: 0.1, y: 0.0}
    uvB: {x: 0.9, y: 1.0}

  - id: 2  # Right (10%)
    uvA: {x: 0.9, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

#### Vertical Zones

Top/bottom split instead of left/right:

```yaml
channels:
  - id: 0  # Top half
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 1.0, y: 0.5}

  - id: 1  # Bottom half
    uvA: {x: 0.0, y: 0.5}
    uvB: {x: 1.0, y: 1.0}
```

### Testing and Visualization

**Visual zone tester:**
```bash
cd backend
go run ./cmd/test-zones-visual

# Shows:
# - Zone boundaries overlaid on test image
# - Extracted colors for each zone
# - UV coordinate visualization
```

**Interactive adjustment:**
1. Edit config UV values
2. Restart the backend (`systemctl --user restart hue-backend`). Zones are built
   once when the sync engine is created, so restarting sync alone keeps the old
   coordinates
3. Start sync and observe light colors
4. Iterate until satisfied

**Tips:**
- Start with simple 2-zone left/right
- Test with colorful content (movies, games)
- Adjust based on physical light placement
- Overlap zones for smoother effects

---

## UI Settings

Configuration for tray icon appearance and behavior.

### UI Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ui.icons.gaming` | string | `"applications-games"` | Icon when gaming + syncing |
| `ui.icons.syncing` | string | `"media-record"` | Icon when syncing (no game) |
| `ui.icons.idle` | string | `"preferences-desktop-display-color"` | Icon when idle |

#### Icon Themes

The tray icon changes based on application state:

**States:**
1. **Idle**: Not syncing (default icon)
2. **Syncing**: Screen sync active (recording icon)
3. **Gaming**: Gaming mode + syncing (game controller icon)

**Example:**
```yaml
ui:
  icons:
    gaming: "applications-games"                # Gaming + sync
    syncing: "media-record"                      # Syncing
    idle: "preferences-desktop-display-color"    # Idle
```

#### Icon Name Format

Icons use **Freedesktop icon naming specification**. Names are theme-independent.

**Finding available icons:**
```bash
# List all available icons
ls /usr/share/icons/breeze/apps/
ls /usr/share/icons/breeze-dark/apps/

# Search for specific icons
find /usr/share/icons -name "*game*" -type f
find /usr/share/icons -name "*media*" -type f
```

**Common icon names:**
- `applications-games` - Game controller
- `media-record` - Recording indicator
- `media-playback-start` - Play button
- `preferences-desktop-display` - Display settings
- `preferences-desktop-color` - Color settings
- `emblem-synchronized` - Sync symbol
- `video-display` - Monitor icon

#### Custom Icons

**Per-state customization:**
```yaml
ui:
  icons:
    gaming: "input-gaming"              # Different gaming icon
    syncing: "emblem-synchronized"      # Sync symbol
    idle: "video-display"               # Monitor icon
```

**Troubleshooting:**
- If icon not found: Falls back to default icon
- Icon theme must be installed
- Names are case-sensitive
- Use theme-neutral names (avoid "breeze-specific")

---

## Logging Configuration

Configuration for diagnostic and debug output.

### Log Level

| Option | Type | Default | Values | Description |
|--------|------|---------|--------|-------------|
| `log_level` | string | `"info"` | See below | Logging verbosity |

#### Log Levels

**Available levels** (least to most verbose):

| Level | Value | When to Use | Output |
|-------|-------|-------------|--------|
| `error` | `"error"` | Production (minimal) | Errors only |
| `warn` | `"warn"` | Production | Errors + warnings |
| `info` | `"info"` | **Default** | Status updates + warnings + errors |
| `debug` | `"debug"` | Development | Detailed debugging info |
| `trace` | `"trace"` | Deep debugging | Everything including frame-by-frame |

**Example:**
```yaml
log_level: "info"  # Default - recommended
```

#### Level Details

##### error

**Use:** Production systems, minimal logging
**Output:** Only errors that prevent operation

```yaml
log_level: "error"
```

**Example output:**
```
[ERROR] Failed to connect to bridge: connection refused
[ERROR] Entertainment API stream failed: invalid clientkey
```

##### warn

**Use:** Production systems, cautious logging
**Output:** Errors + warnings about potential issues

```yaml
log_level: "warn"
```

**Example output:**
```
[WARN] Config file has insecure permissions: 0644 (should be 0600)
[WARN] Frame processing took 45ms (target: 33ms)
[ERROR] Bridge connection lost
```

##### info (Default)

**Use:** Default for most users
**Output:** Status updates, successful operations, warnings, errors

```yaml
log_level: "info"  # Recommended
```

**Example output:**
```
[INFO] Bridge connected: 192.168.1.100
[INFO] Entertainment Area activated: Living Room
[INFO] Screen sync started at 30 FPS
[INFO] Gaming detected - starting screen sync
[WARN] Frame processing took 45ms
[ERROR] Failed to capture frame
```

##### debug

**Use:** Development, troubleshooting issues
**Output:** Detailed debugging information

```yaml
log_level: "debug"
```

**Example output:**
```
[DEBUG] Loading config from /home/user/.openhue/config.yaml
[DEBUG] Config validation passed
[DEBUG] DBus service registered: org.kde.plasma.hue
[INFO] Bridge connected: 192.168.1.100
[DEBUG] Entertainment Area channels: 2
[DEBUG] Channel 0: uvA={0.0, 0.0}, uvB={0.5, 1.0}
[DEBUG] PipeWire stream state: STREAMING
[DEBUG] Frame captured: 2560x1440 (BGRA)
[DEBUG] Subsampled to: 64x36
[DEBUG] Zone 0 color: RGB(45, 120, 200)
[INFO] Screen sync started at 30 FPS
```

##### trace

**Use:** Deep debugging, performance analysis
**Output:** Every operation including frame-by-frame processing

```yaml
log_level: "trace"  # Very verbose!
```

**Example output:**
```
[TRACE] Config file read: 1234 bytes
[TRACE] YAML parse: 5.2ms
[DEBUG] Config validation passed
[TRACE] DBus connection established
[TRACE] Introspection sent
[INFO] Bridge connected
[TRACE] HTTP GET /clip/v2/resource/entertainment_configuration
[TRACE] Response: 200 OK (234ms)
[TRACE] JSON parse: 2.1ms
[TRACE] PipeWire: format negotiation
[TRACE] PipeWire: buffer allocated (1920x1080x4 = 8MB)
[TRACE] Frame 1: captured (33.2ms)
[TRACE] Frame 1: subsampled (2.1ms)
[TRACE] Frame 1: zone 0 extracted (0.8ms)
[TRACE] Frame 1: zone 0 color: RGB(45, 120, 200)
[TRACE] Frame 1: streamed (1.2ms)
[TRACE] Frame 1: total time 37.3ms
```

**Warning:** `trace` level generates **massive logs** (MB per minute). Only use for short debugging sessions.

#### Log Output

**Viewing logs:**

```bash
# Systemd service logs (recommended)
journalctl --user -u hue-backend -f

# Last 50 lines
journalctl --user -u hue-backend -n 50

# Last hour
journalctl --user -u hue-backend --since "1 hour ago"

# Manual run (stdout)
./backend/hue-sync  # Logs to terminal
```

**Log format:** a `2006/01/02 15:04:05` timestamp from the standard library
logger, then a level tag, then the message. journalctl adds its own timestamp
and unit prefix on top of that. Both prefixes are omitted below.

```
[INFO] Screen sync started
[WARN] Frame skip: dropped 12 frames (processing too slow for 30 FPS)
```

#### Performance Impact

| Level | CPU Overhead | Disk I/O | Use Case |
|-------|--------------|----------|----------|
| `error` | ~0.1% | Minimal | Production |
| `warn` | ~0.2% | Low | Production |
| `info` | ~0.5% | Moderate | **Default** |
| `debug` | ~2-5% | High | Development |
| `trace` | ~10-20% | Very high | Debugging only |

**Recommendations:**
- **Production**: `info` or `warn`
- **Development**: `debug`
- **Bug reports**: `debug` (include logs with issue)
- **Performance profiling**: `info` (debug adds overhead)
- **Debugging crashes**: `trace` (short duration only)

---

## Example Configurations

Real-world configuration examples for common setups.

### Minimal Configuration

Basic setup without Entertainment API:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
grouped_light_id: "room/living-room"
log_level: "info"
```

**Features:**
- Scene control
- Power/brightness control

**Not available:**
- Screen sync (requires Entertainment API)

---

### Basic Screen Sync (Two Lights)

Simple left/right split for two lights:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/living-room"

sync:
  enabled: false
  fps: 30
  subsampleWidth: 64
  restoreToken: ""
  metricsInterval: 60

channels:
  - id: 0
    active: true
    deviceName: "Left Hue Play"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}

  - id: 1
    active: true
    deviceName: "Right Hue Play"
    gammaFactor: 2.2
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}

log_level: "info"
```

**Features:**
- Scene control
- Power/brightness control
- Screen sync (2 zones)

**Not available:**
- Gaming mode

---

### Multi-Zone Setup (Three Lights)

Three-zone split for immersive experience:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/gaming-room"

sync:
  enabled: false
  fps: 30
  subsampleWidth: 64
  restoreToken: ""
  metricsInterval: 60

channels:
  - id: 0
    active: true
    deviceName: "Left Play Bar"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.33, y: 1.0}

  - id: 1
    active: true
    deviceName: "Center Go"
    gammaFactor: 2.2
    uvA: {x: 0.33, y: 0.0}
    uvB: {x: 0.67, y: 1.0}

  - id: 2
    active: true
    deviceName: "Right Play Bar"
    gammaFactor: 2.2
    uvA: {x: 0.67, y: 0.0}
    uvB: {x: 1.0, y: 1.0}

log_level: "info"
```

**Features:**
- Scene control
- Power/brightness control
- Screen sync (3 zones)

**Not available:**
- Gaming mode

---

### Gaming Mode (CachyOS)

Optimized for CachyOS with automatic gaming detection:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/gaming-room"

sync:
  enabled: false        # Don't auto-start
  fps: 30               # Smooth gaming
  subsampleWidth: 64
  restoreToken: ""
  metricsInterval: 60

gamingMode:
  enabled: true         # Enable auto-detection
  pollInterval: 2
  debounceDelay: 5
  useSystemdInhibit: true   # CachyOS primary detection
  usePowerProfile: true     # Secondary validation
  useSteamAppId: true       # Steam games
  useGameMode: false        # Not installed

ui:
  icons:
    gaming: "applications-games"
    syncing: "media-record"
    idle: "preferences-desktop-display-color"

channels:
  - id: 0
    active: true
    deviceName: "Left Play"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}

  - id: 1
    active: true
    deviceName: "Right Play"
    gammaFactor: 2.2
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}

log_level: "info"
```

**Features:**
- Scene control
- Power/brightness control
- Screen sync
- **Automatic gaming detection**
- CachyOS-optimized detection

---

### TV Backlight (Four Lights)

Four-light setup for TV edge lighting:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/living-room"

sync:
  enabled: false
  fps: 25              # Slightly lower for movies
  subsampleWidth: 48   # Lower for better performance
  restoreToken: ""
  metricsInterval: 60

channels:
  - id: 0  # Left edge light
    active: true
    deviceName: "Left Strip"
    gammaFactor: 2.4   # Slightly higher for TV
    uvA: {x: 0.0, y: 0.25}
    uvB: {x: 0.15, y: 0.75}

  - id: 1  # Top edge light
    active: true
    deviceName: "Top Strip"
    gammaFactor: 2.4
    uvA: {x: 0.25, y: 0.0}
    uvB: {x: 0.75, y: 0.15}

  - id: 2  # Right edge light
    active: true
    deviceName: "Right Strip"
    gammaFactor: 2.4
    uvA: {x: 0.85, y: 0.25}
    uvB: {x: 1.0, y: 0.75}

  - id: 3  # Bottom edge light
    active: true
    deviceName: "Bottom Strip"
    gammaFactor: 2.4
    uvA: {x: 0.25, y: 0.85}
    uvB: {x: 0.75, y: 1.0}

log_level: "info"
```

**Features:**
- Scene control
- Power/brightness control
- Screen sync (4 edge zones)
- Optimized for movies (lower FPS)
- Edge-focused mapping

---

### Performance-Optimized (Laptop)

Battery-friendly configuration:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/bedroom"

sync:
  enabled: false
  fps: 20              # Lower FPS for battery
  subsampleWidth: 32   # Lower resolution
  restoreToken: ""
  metricsInterval: 60

gamingMode:
  enabled: false       # Disable gaming mode on laptop

channels:
  - id: 0
    active: true
    deviceName: "Desk Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 1.0, y: 1.0}  # Full screen (single light)

log_level: "warn"      # Less logging overhead
```

**Features:**
- Scene control
- Power/brightness control
- Screen sync (1 zone)
- **Battery-optimized** (low FPS, low resolution)
- Minimal logging

**Performance:**
- CPU usage: ~2-4% (vs 6-10% default)
- Suitable for battery operation
- Still provides ambient lighting

---

### High-Quality Setup (Desktop)

Maximum quality for desktop with AC power:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/office"

sync:
  enabled: false
  fps: 60              # Maximum smoothness
  subsampleWidth: 128  # High quality
  restoreToken: ""
  metricsInterval: 60

channels:
  - id: 0
    active: true
    deviceName: "Left Play"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}

  - id: 1
    active: true
    deviceName: "Right Play"
    gammaFactor: 2.2
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}

log_level: "debug"     # More diagnostic info
```

**Features:**
- Scene control
- Power/brightness control
- **Maximum quality** (60 FPS, high resolution)
- Detailed logging

**Performance:**
- CPU usage: ~15-25%
- Requires AC power
- Buttery smooth sync
- Best color accuracy

---

### Debugging Configuration

Configuration for troubleshooting issues:

```yaml
version: 1
Bridge: "192.168.1.100"
Key: "your-api-key-here"
clientkey: "your-client-key-here"
entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
grouped_light_id: "room/test"

sync:
  enabled: false
  fps: 30
  subsampleWidth: 64
  restoreToken: ""  # Empty forces the permission dialog on the next backend start
  metricsInterval: 60

gamingMode:
  enabled: true
  pollInterval: 2
  debounceDelay: 5
  useSystemdInhibit: true
  usePowerProfile: true
  useSteamAppId: true
  useGameMode: true

channels:
  - id: 0
    active: true
    deviceName: "Test Light 1"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}

  - id: 1
    active: true
    deviceName: "Test Light 2"
    gammaFactor: 2.2
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}

log_level: "debug"     # Detailed debugging
```

**Features:**
- All detection methods enabled
- Debug logging
- Fresh restore token (forces dialog)
- Simple 2-zone setup for testing

**When to use:**
- Troubleshooting sync issues
- Testing gaming detection
- Reporting bugs (attach logs)
- Validating configuration

---

## Validation Rules

Configuration is validated on load. Invalid configs prevent backend startup.

### Bridge Validation

| Rule | Error Message |
|------|---------------|
| Bridge IP missing | `bridge IP not configured` |
| Bridge IP empty | `bridge IP not configured` |

**How to fix:**
```yaml
Bridge: "192.168.1.100"  # Add your bridge IP
```

### API Key Validation

| Rule | Error Message |
|------|---------------|
| API key missing | `API key not configured` |
| API key empty | `API key not configured` |

**How to fix:**
```bash
# Pair with the bridge and save Bridge and Key to the config
openhue setup
```

### Sync Settings Validation

| Rule | Error Message |
|------|---------------|
| FPS < 1 | `sync.fps must be between 10 and 60 (got X)` |
| FPS 1 - 9 | clamped to 10 with a warning, no error |
| FPS > 60 | `sync.fps must be between 10 and 60 (got X)` |
| SubsampleWidth < 16 | `sync.subsampleWidth must be between 16 and 256 (got X)` |
| SubsampleWidth > 256 | `sync.subsampleWidth must be between 16 and 256 (got X)` |
| MetricsInterval < 0 | `sync.metricsInterval must be between 0 and 3600 seconds (got X)` |
| MetricsInterval > 3600 | `sync.metricsInterval must be between 0 and 3600 seconds (got X)` |

**How to fix:**
```yaml
sync:
  fps: 30              # Must be 10-60
  subsampleWidth: 64   # Must be 16-256
  metricsInterval: 60  # Must be 0-3600
```

### Channel Validation

#### UV Coordinate Rules

| Rule | Error Message |
|------|---------------|
| uvA.x < 0.0 | `channel N: uvA.x must be 0.0-1.0 (got X)` |
| uvA.x > 1.0 | `channel N: uvA.x must be 0.0-1.0 (got X)` |
| uvA.y < 0.0 | `channel N: uvA.y must be 0.0-1.0 (got X)` |
| uvA.y > 1.0 | `channel N: uvA.y must be 0.0-1.0 (got X)` |
| uvB.x < 0.0 | `channel N: uvB.x must be 0.0-1.0 (got X)` |
| uvB.x > 1.0 | `channel N: uvB.x must be 0.0-1.0 (got X)` |
| uvB.y < 0.0 | `channel N: uvB.y must be 0.0-1.0 (got X)` |
| uvB.y > 1.0 | `channel N: uvB.y must be 0.0-1.0 (got X)` |

**How to fix:**
```yaml
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}  # All values must be 0.0-1.0
    uvB: {x: 0.5, y: 1.0}
```

#### UV Ordering Rules

| Rule | Error Message |
|------|---------------|
| uvA.x >= uvB.x | `channel N: uvA.x (X) must be less than uvB.x (Y)` |
| uvA.y >= uvB.y | `channel N: uvA.y (X) must be less than uvB.y (Y)` |

**Explanation:**
- uvA is top-left corner
- uvB is bottom-right corner
- Left must be less than right
- Top must be less than bottom

**How to fix:**
```yaml
# Correct (uvA is top-left, uvB is bottom-right)
uvA: {x: 0.0, y: 0.0}
uvB: {x: 0.5, y: 1.0}

# Incorrect (uvA.x >= uvB.x)
uvA: {x: 0.5, y: 0.0}
uvB: {x: 0.5, y: 1.0}  # ERROR: 0.5 >= 0.5

# Incorrect (uvA.x > uvB.x)
uvA: {x: 0.6, y: 0.0}
uvB: {x: 0.4, y: 1.0}  # ERROR: 0.6 > 0.4 (backwards!)
```

#### Gamma Factor Rules

| Rule | Error Message |
|------|---------------|
| gammaFactor < 0.5 | `channel N: gammaFactor must be 0.5-4.0 (got X)` |
| gammaFactor > 4.0 | `channel N: gammaFactor must be 0.5-4.0 (got X)` |

**How to fix:**
```yaml
channels:
  - id: 0
    gammaFactor: 2.2  # Must be 0.5-4.0 (recommended: 2.2)
```

### Warnings (Non-Fatal)

These generate warnings but don't prevent startup:

| Condition | Warning Message |
|-----------|-----------------|
| Config file permissions > 0600 | `Config file has insecure permissions: 0XXX (should be 0600)` |
| Channel deviceName empty | `channel N has no deviceName set` |

**How to fix warnings:**
```bash
# Fix permissions
chmod 600 ~/.openhue/config.yaml

# Add device names
deviceName: "Left Light"  # Add to each channel
```

---

## Troubleshooting Common Config Issues

### Config File Not Found

**Symptom:** Backend starts but features don't work

**Cause:** Config file doesn't exist or wrong location

**Solution:**
```bash
# Check if config exists
ls -la ~/.openhue/config.yaml

# If missing, create it
mkdir -p ~/.openhue
openhue setup

# Or create manually
cat > ~/.openhue/config.yaml <<EOF
Bridge: "192.168.1.X"
Key: "YOUR-KEY"
EOF
```

### Invalid YAML Syntax

**Symptom:** Backend fails to start with parse error

**Cause:** YAML syntax error (indentation, quotes, colons)

**Solution:**
```bash
# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('/home/user/.openhue/config.yaml'))"

# Common issues:
# - Tabs instead of spaces (use 2 spaces)
# - Missing colons after keys
# - Incorrect indentation
# - Unquoted strings with special chars
```

**Example fixes:**
```yaml
# WRONG (tabs)
sync:
	enabled: true

# CORRECT (2 spaces)
sync:
  enabled: true

# WRONG (missing colon)
Bridge "192.168.1.100"

# CORRECT
Bridge: "192.168.1.100"

# WRONG (unquoted special chars)
deviceName: Left "Main" Light

# CORRECT
deviceName: "Left \"Main\" Light"
```

### Screen Sync Not Working

**Symptom:** Sync starts but lights don't change

**Diagnosis checklist:**

1. **clientkey missing:**
   ```yaml
   clientkey: "CLIENT-KEY"  # Required!
   ```

2. **entertainmentConfigurationId missing:**
   ```yaml
   entertainmentConfigurationId: "550e8400-e29b-41d4-a716-446655440000"
   ```

3. **No channels configured:**
   ```yaml
   channels:  # Must have at least one channel
     - id: 0
       active: true
       # ...
   ```

4. **All channels inactive:**
   ```yaml
   channels:
     - id: 0
       active: true  # At least one must be true
   ```

5. **Permission dialog not approved:**
   - Check for XDG Portal permission dialog
   - Approve screen sharing when prompted
   - Check logs: `journalctl --user -u hue-backend -f`

### Gaming Mode Not Triggering

**Symptom:** Gaming mode enabled but doesn't detect games

**Diagnosis checklist:**

1. **No detection methods enabled:**
   ```yaml
   gamingMode:
     enabled: true
     useSystemdInhibit: true  # At least one must be true
     usePowerProfile: true
     # etc.
   ```

2. **Wrong detection method for system:**
   - CachyOS: Use `useSystemdInhibit: true`
   - Steam: Use `useSteamAppId: true`
   - Other: Use `useGameMode: true` (if installed)

3. **Debounce delay too long:**
   ```yaml
   gamingMode:
     debounceDelay: 5  # Try lowering to 2-3
   ```

4. **Check backend logs:**
   ```bash
   journalctl --user -u hue-backend -f
   # Look for:
   # "Gaming detected"
   # "No gaming detection methods available"
   ```

### Lights Show Wrong Colors

**Symptom:** Lights don't match screen colors

**Causes and fixes:**

1. **Gamma factor incorrect:**
   ```yaml
   gammaFactor: 2.2  # Standard for most displays
   # Try 1.8-2.4 if colors off
   ```

2. **UV zones backwards:**
   ```yaml
   # WRONG (uvA.x > uvB.x)
   uvA: {x: 0.5, y: 0.0}
   uvB: {x: 0.0, y: 1.0}

   # CORRECT
   uvA: {x: 0.0, y: 0.0}
   uvB: {x: 0.5, y: 1.0}
   ```

3. **Zone not covering desired area:**
   ```bash
   # Test zones visually
   cd backend
   go run ./cmd/test-zones-visual
   ```

4. **Subsample width too low:**
   ```yaml
   sync:
     subsampleWidth: 64  # Try 96 or 128
   ```

### Permission Errors

**Symptom:** Config file warning about permissions

**Fix:**
```bash
# Secure config file
chmod 600 ~/.openhue/config.yaml

# Verify
ls -la ~/.openhue/config.yaml
# Should show: -rw------- (0600)
```

### Bridge Connection Fails

**Symptom:** Backend can't connect to bridge

**Diagnosis:**

1. **Bridge IP changed:**
   ```bash
   # Find new IP
   openhue discover

   # Update config
   Bridge: "192.168.1.NEW-IP"
   ```

2. **Bridge unreachable:**
   ```bash
   # Test connectivity
   ping 192.168.1.100
   curl http://192.168.1.100/api/config

   # Check if bridge on different network/VLAN
   ```

3. **API key invalid:**
   ```bash
   # Test key
   curl http://192.168.1.100/api/YOUR-KEY/lights

   # If fails, pair again. With a clientkey configured, use
   # 'cd backend && go run ./cmd/register-entertainment' instead: it prints a
   # Key and clientkey that belong together.
   openhue setup
   ```

---

## Migration & Compatibility

### Config Version Tracking

The `version` field tracks config format version for future migrations:

```yaml
version: 1  # Current version
```

**Version history:**
- **Version 1**: Current format (initial release)

**Future migrations:**
- Breaking changes will increment version
- Backend will auto-migrate old configs
- Backups created before migration

### openhue-cli Compatibility

KDE Hue Control config is **fully compatible** with openhue-cli:

**Shared fields:**
- `Bridge`
- `Key`
- `clientkey`

**khuey-specific fields** (ignored by openhue-cli):
- `version`
- `grouped_light_id`
- `entertainmentConfigurationId`
- `startupScene`
- `sync`
- `gamingMode`
- `ui`
- `channels`
- `log_level`

**Example shared config:**
```yaml
# Shared with openhue-cli
Bridge: "192.168.1.100"
Key: "shared-key"
clientkey: "shared-clientkey"

# khuey-specific (ignored by openhue-cli)
sync:
  enabled: false
channels:
  - id: 0
    # ...
```

### Backup and Restore

**Backup config:**
```bash
# Manual backup
cp ~/.openhue/config.yaml ~/.openhue/config.yaml.backup

# Timestamped backup
cp ~/.openhue/config.yaml ~/.openhue/config.yaml.$(date +%Y%m%d)

# Full directory backup
tar czf openhue-backup.tar.gz ~/.openhue/
```

**Restore config:**
```bash
# Restore from backup
cp ~/.openhue/config.yaml.backup ~/.openhue/config.yaml

# Restore from tar
tar xzf openhue-backup.tar.gz -C ~/
```

**Automatic backups:**
```bash
# Backend creates backup before migration
# Location: ~/.openhue/config.yaml.backup-vN
```

### Migrating from Old Format

**If you have an old config format:**

1. Backend will detect old version
2. Automatic migration occurs
3. Backup created: `config.yaml.backup-v0`
4. New format saved

**Manual migration (if needed):**
```bash
# Backup first
cp ~/.openhue/config.yaml config.yaml.old

# Add version field
echo "version: 1" | cat - config.yaml.old > ~/.openhue/config.yaml

# Restart backend
systemctl --user restart hue-backend
```

### Cross-Platform Compatibility

**Config works across:**
- Different Linux distributions
- Different desktop environments (KDE, GNOME, etc.)
- Different screen resolutions (UV coordinates)
- Different monitor setups

**Platform-specific considerations:**

| Feature | Requirement |
|---------|-------------|
| Screen Sync | Wayland + PipeWire |
| Gaming Mode (systemd-inhibit) | CachyOS or systemd-based |
| Gaming Mode (GameMode) | Feral GameMode installed |
| Gaming Mode (Power Profile) | power-profiles-daemon |

**Example portable config:**
```yaml
# Works on any resolution (UV coordinates)
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}

# Gaming detection adapts to platform
gamingMode:
  enabled: true
  useSystemdInhibit: true   # CachyOS
  usePowerProfile: true     # Universal
  useSteamAppId: true       # If Steam installed
  useGameMode: true         # If GameMode installed
```

---

## Additional Resources

### Related Documentation

- **[FEATURES.md](../FEATURES.md)** - Complete feature documentation
- **[USAGE.md](../USAGE.md)** - Usage guide and tutorials
- **[ARCHITECTURE.md](../ARCHITECTURE.md)** - System architecture overview
- **[TROUBLESHOOTING.md](../TROUBLESHOOTING.md)** - Troubleshooting guide
- **[DEVELOPMENT.md](../DEVELOPMENT.md)** - Development guide
- **[API_REFERENCE.md](./API_REFERENCE.md)** - DBus API reference

### Configuration Tools

**openhue-cli:**
```bash
# Install
cargo install openhue-cli

# Setup (generates config)
openhue setup

# Test config
openhue get /clip/v2/resource
```

**Test utilities:**
```bash
cd backend

# Test Entertainment Area setup
go run ./cmd/get-entertainment-info

# Visual zone tester
go run ./cmd/test-zones-visual

# Test screen capture
go run ./cmd/test-capture
```

### Getting Help

**Check logs:**
```bash
# Backend logs
journalctl --user -u hue-backend -f

# Set debug logging
log_level: "debug"
```

**Validate config:**
```bash
# YAML syntax
python3 -c "import yaml; yaml.safe_load(open('~/.openhue/config.yaml'))"

# Backend validation
./backend/hue-sync  # Will show validation errors
```

**Community support:**
- GitHub Issues: Report bugs, request features
- Discussions: Ask questions, share configs
- Pull Requests: Contribute improvements

---

## Summary

This configuration reference covers:
- Every configuration option with type, range, defaults
- UV coordinate system with visual examples
- Entertainment API channel configuration
- Gaming mode detection methods
- 8 real-world example configurations
- Complete validation rules
- Troubleshooting common issues
- Migration and compatibility notes

**Key takeaways:**
1. Config file: `~/.openhue/config.yaml` (0600 permissions)
2. UV coordinates: Monitor-agnostic zone mapping (0.0-1.0)
3. Gaming mode: Multiple detection methods (systemd-inhibit, Steam, GameMode)
4. Validation: Strict rules prevent invalid configs
5. Compatibility: Works with openhue-cli

**Next steps:**
- Review [USAGE.md](../USAGE.md) for practical usage examples
- Check [TROUBLESHOOTING.md](../TROUBLESHOOTING.md) if issues arise
- See [FEATURES.md](../FEATURES.md) for feature documentation

---

**Document version:** 1.0
**Last updated:** 2024
**Config format version:** 1
