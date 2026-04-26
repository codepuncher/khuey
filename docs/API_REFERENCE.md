# KDE Hue Control - DBus API Reference

Complete reference for the `org.kde.plasma.hue` DBus API.

## Table of Contents

1. [Overview](#overview)
2. [Connection Details](#connection-details)
3. [Access Control](#access-control)
4. [Methods](#methods)
   - [Status & Connection](#status--connection)
   - [Scene Control](#scene-control)
   - [Light Control](#light-control)
   - [Screen Sync](#screen-sync)
   - [Gaming Mode](#gaming-mode)
   - [Configuration](#configuration)
5. [Error Handling](#error-handling)
6. [Usage Examples](#usage-examples)
7. [Type Reference](#type-reference)

---

## Overview

KDE Hue Control exposes a DBus service that allows programmatic control of Philips Hue lights. The tray application communicates with the backend exclusively through this API.

**Key Features:**
- Scene activation and management
- Power and brightness control
- Screen synchronization (Entertainment API)
- Gaming mode automation
- Configuration management
- Connection monitoring

---

## Connection Details

**Service Name:** `org.kde.plasma.hue`
**Object Path:** `/org/kde/plasma/hue`
**Interface:** `org.kde.plasma.hue`
**Bus:** Session bus (not system bus)

### Verifying Service Availability

```bash
# Check if service is running
dbus-send --session --print-reply \
  --dest=org.freedesktop.DBus \
  /org/freedesktop/DBus \
  org.freedesktop.DBus.ListNames | grep hue

# Expected output: string "org.kde.plasma.hue"
```

### Introspection

```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.freedesktop.DBus.Introspectable.Introspect
```

---

## Access Control

**Security Model:** UID-based access control

- Only processes running as the **same user** that started the backend can call methods
- Prevents other users or malicious processes from controlling your lights
- System enforces access control automatically via DBus

**Protected Methods:**
- All methods that modify state (SetPower, ActivateScene, StartSync, etc.)
- Methods that change configuration (SetGroupedLight, SetSyncSettings, etc.)

**Read-Only Methods (No Access Control):**
- GetStatus, GetScenes, IsSyncing, IsGamingModeEnabled

**Access Denied Response:**
```
Error: access denied: only the service owner can perform this operation
```

---

## Methods

### Status & Connection

#### GetStatus

Returns the current backend status.

**Signature:** `GetStatus() → string`

**Returns:**
- `"Ready"` - Backend is configured and ready
- `"Not configured"` - Missing bridge IP or API key

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus

# Output: string "Ready"
```

**Example (Qt/C++):**
```cpp
QDBusInterface iface("org.kde.plasma.hue",
                     "/org/kde/plasma/hue",
                     "org.kde.plasma.hue",
                     QDBusConnection::sessionBus());

QDBusReply<QString> reply = iface.call("GetStatus");
if (reply.isValid()) {
    QString status = reply.value();
    qDebug() << "Status:" << status;
}
```

**Example (Go):**
```go
conn, _ := dbus.ConnectSessionBus()
obj := conn.Object("org.kde.plasma.hue", "/org/kde/plasma/hue")

var status string
err := obj.Call("org.kde.plasma.hue.GetStatus", 0).Store(&status)
if err == nil {
    fmt.Println("Status:", status)
}
```

---

#### GetConnectionStatus

Returns detailed bridge connection information.

**Signature:** `GetConnectionStatus() → map[string]interface{}`

**Returns (map keys):**
- `connected` (bool) - True if bridge is reachable
- `lastError` (string) - Last error message (empty if no error)
- `bridgeIP` (string) - Configured bridge IP address
- `lastAttempt` (string) - Timestamp of last connection attempt (RFC 3339)

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetConnectionStatus

# Output:
#   dict entry(
#     string "connected"
#     variant boolean true
#   )
#   dict entry(
#     string "bridgeIP"
#     variant string "192.168.1.100"
#   )
#   ...
```

**Example (Qt/C++):**
```cpp
QDBusReply<QVariantMap> reply = iface.call("GetConnectionStatus");
if (reply.isValid()) {
    QVariantMap status = reply.value();
    bool connected = status["connected"].toBool();
    QString bridgeIP = status["bridgeIP"].toString();
    QString lastError = status["lastError"].toString();

    qDebug() << "Connected:" << connected;
    qDebug() << "Bridge:" << bridgeIP;
    if (!lastError.isEmpty()) {
        qWarning() << "Last error:" << lastError;
    }
}
```

---

#### RetryConnection

Attempts to reconnect to the Hue bridge.

**Signature:** `RetryConnection() → bool`

**Returns:**
- `true` - Connection successful
- `false` - Connection failed (error in DBus error field)

**Errors:**
- `"bridge still unreachable"` - Bridge is not responding
- `"hue client not initialized"` - Backend not configured

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.RetryConnection
```

**Example (Qt/C++):**
```cpp
QDBusReply<bool> reply = iface.call("RetryConnection");
if (reply.isValid() && reply.value()) {
    qDebug() << "Connection restored";
} else {
    qWarning() << "Connection failed:" << reply.error().message();
}
```

---

#### TestBridgeConnection

Tests connectivity to the bridge without retrying.

**Signature:** `TestBridgeConnection() → bool`

**Returns:**
- `true` - Bridge is reachable
- `false` - Bridge is not reachable

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.TestBridgeConnection
```

---

#### GetBridgeSettings

Returns bridge configuration information.

**Signature:** `GetBridgeSettings() → map[string]interface{}`

**Returns (map keys):**
- `bridgeIP` (string) - Configured bridge IP address
- `connected` (bool) - Current connection status
- `lastError` (string) - Last error message

**Example (Qt/C++):**
```cpp
QDBusReply<QVariantMap> reply = iface.call("GetBridgeSettings");
if (reply.isValid()) {
    QVariantMap settings = reply.value();
    qDebug() << "Bridge IP:" << settings["bridgeIP"].toString();
    qDebug() << "Connected:" << settings["connected"].toBool();
}
```

---

### Scene Control

#### GetScenes

Returns list of available scenes from all rooms.

**Signature:** `GetScenes() → []string`

**Returns:**
- Array of scene names in format: `"Room Name - Scene Name"`
- If no room, format is just: `"Scene Name"`
- **Sorted alphabetically** for consistent UI display

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetScenes

# Output:
#   array [
#     string "Living Room - Bright"
#     string "Living Room - Concentrate"
#     string "Living Room - Relax"
#   ]
```

**Example (Qt/C++):**
```cpp
QDBusReply<QStringList> reply = iface.call("GetScenes");
if (reply.isValid()) {
    QStringList scenes = reply.value();
    for (const QString &scene : scenes) {
        qDebug() << "Scene:" << scene;
        // Add to menu or list widget
    }
}
```

**Example (Go):**
```go
var scenes []string
err := obj.Call("org.kde.plasma.hue.GetScenes", 0).Store(&scenes)
if err == nil {
    for _, scene := range scenes {
        fmt.Println("Scene:", scene)
    }
}
```

---

#### ActivateScene

Activates a scene by its display name.

**Signature:** `ActivateScene(displayName: string) → string`

**Parameters:**
- `displayName` (string) - Scene name as returned by GetScenes
  - Accepts full format: `"Room Name - Scene Name"`
  - Also accepts short format: `"Scene Name"`

**Returns:**
- Success message: `"Scene activated: Scene Name"`

**Errors:**
- `"scene not found: XYZ"` - Scene doesn't exist
- `"hue client not initialized"` - Backend not configured
- `"access denied"` - Caller is not service owner
- `"displayName exceeds maximum length"` - Input validation failed

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.ActivateScene \
  string:"Living Room - Relax"

# Output: string "Scene activated: Living Room - Relax"
```

**Example (Qt/C++):**
```cpp
QString sceneName = "Living Room - Relax";
QDBusReply<QString> reply = iface.call("ActivateScene", sceneName);

if (reply.isValid()) {
    QString result = reply.value();
    // Show success notification
    KNotification::event("SceneActivated",
                        "Hue Control",
                        result,
                        "preferences-desktop-display-color");
} else {
    // Show error
    qWarning() << "Failed to activate scene:" << reply.error().message();
}
```

**Example (Go):**
```go
var result string
err := obj.Call("org.kde.plasma.hue.ActivateScene", 0, "Living Room - Relax").Store(&result)
if err == nil {
    fmt.Println(result)
} else {
    fmt.Println("Error:", err)
}
```

---

### Light Control

#### GetGroupedLights

Returns available rooms and zones with grouped lights.

**Signature:** `GetGroupedLights() → []struct{ID, Name, Type string}`

**Returns:**
- Array of grouped light info:
  - `ID` (string) - Grouped light UUID
  - `Name` (string) - Room or zone name
  - `Type` (string) - "room" or "zone"

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetGroupedLights

# Output:
#   array [
#     struct {
#       string "abc-123-def"
#       string "Living Room"
#       string "room"
#     }
#     ...
#   ]
```

**Example (Qt/C++):**
```cpp
// Define struct for Qt DBus
struct GroupedLight {
    QString id;
    QString name;
    QString type;
};
Q_DECLARE_METATYPE(GroupedLight)

// Register metatype
qDBusRegisterMetaType<GroupedLight>();
qDBusRegisterMetaType<QList<GroupedLight>>();

QDBusReply<QList<GroupedLight>> reply = iface.call("GetGroupedLights");
if (reply.isValid()) {
    for (const GroupedLight &light : reply.value()) {
        qDebug() << light.name << "(" << light.type << ")";
    }
}
```

---

#### SetGroupedLight

Sets which room/zone to control with power and brightness methods.

**Signature:** `SetGroupedLight(groupedLightID: string) → bool`

**Parameters:**
- `groupedLightID` (string) - Grouped light UUID from GetGroupedLights

**Returns:**
- `true` - Successfully saved to config
- `false` - Failed to save (error in DBus error field)

**Errors:**
- `"grouped light ID cannot be empty"` - Invalid parameter
- `"access denied"` - Caller is not service owner
- `"failed to save config"` - File system error

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetGroupedLight \
  string:"abc-123-def-456"
```

**Example (Qt/C++):**
```cpp
QString groupedLightID = "abc-123-def-456";
QDBusReply<bool> reply = iface.call("SetGroupedLight", groupedLightID);

if (reply.isValid() && reply.value()) {
    qDebug() << "Grouped light set successfully";
} else {
    qWarning() << "Failed:" << reply.error().message();
}
```

---

#### GetSelectedRoom

Returns the currently selected room/zone ID.

**Signature:** `GetSelectedRoom() → string`

**Returns:**
- Grouped light ID (string) - Empty if not configured

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetSelectedRoom
```

---

#### SetSelectedRoom

Updates the room/zone selection (alias for SetGroupedLight).

**Signature:** `SetSelectedRoom(roomID: string) → bool`

**Parameters:**
- `roomID` (string) - Grouped light UUID

**Returns:**
- `true` - Successfully saved
- `false` - Failed (error in DBus error field)

---

#### SetPower

Turns lights on or off for the selected room/zone.

**Signature:** `SetPower(on: bool) → bool`

**Parameters:**
- `on` (bool) - True to turn on, false to turn off

**Returns:**
- `true` - Successfully changed power state
- `false` - Failed (error in DBus error field)

**Errors:**
- `"no grouped light configured"` - Must call SetGroupedLight first
- `"hue client not initialized"` - Backend not configured
- `"access denied"` - Caller is not service owner

**Example (dbus-send):**
```bash
# Turn on
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetPower \
  boolean:true

# Turn off
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetPower \
  boolean:false
```

**Example (Qt/C++):**
```cpp
bool turnOn = true;
QDBusReply<bool> reply = iface.call("SetPower", turnOn);

if (reply.isValid() && reply.value()) {
    qDebug() << "Power changed successfully";
} else {
    qWarning() << "Failed to change power:" << reply.error().message();
}
```

**Example (Go):**
```go
var success bool
err := obj.Call("org.kde.plasma.hue.SetPower", 0, true).Store(&success)
if err == nil && success {
    fmt.Println("Lights turned on")
}
```

---

#### SetBrightness

Sets brightness for the selected room/zone.

**Signature:** `SetBrightness(brightness: int32) → bool`

**Parameters:**
- `brightness` (int32) - Brightness level (0-100)
  - 0 = minimum brightness (not off)
  - 100 = maximum brightness

**Returns:**
- `true` - Successfully changed brightness
- `false` - Failed (error in DBus error field)

**Errors:**
- `"brightness must be 0-100"` - Invalid range
- `"no grouped light configured"` - Must call SetGroupedLight first
- `"access denied"` - Caller is not service owner

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetBrightness \
  int32:75
```

**Example (Qt/C++):**
```cpp
int brightness = 75;
QDBusReply<bool> reply = iface.call("SetBrightness", brightness);

if (reply.isValid() && reply.value()) {
    qDebug() << "Brightness set to" << brightness;
} else {
    qWarning() << "Failed:" << reply.error().message();
}
```

---

#### GetState

Returns current power and brightness state of the selected room/zone.

**Signature:** `GetState() → (power: bool, brightness: int32, success: bool)`

**Returns:**
- `power` (bool) - True if lights are on
- `brightness` (int32) - Current brightness (0-100)
- `success` (bool) - True if query succeeded

**Errors:**
- `"no grouped light configured"` - Must call SetGroupedLight first
- `"hue client not initialized"` - Backend not configured

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetState

# Output:
#   boolean true
#   int32 75
#   boolean true
```

**Example (Qt/C++):**
```cpp
QDBusMessage reply = iface.call("GetState");
if (reply.type() == QDBusMessage::ReplyMessage) {
    QList<QVariant> args = reply.arguments();
    bool power = args[0].toBool();
    int brightness = args[1].toInt();
    bool success = args[2].toBool();

    if (success) {
        qDebug() << "Power:" << power;
        qDebug() << "Brightness:" << brightness;
    }
}
```

---

### Screen Sync

#### StartSync

Starts screen synchronization (Entertainment API streaming).

**Signature:** `StartSync() → bool`

**Returns:**
- `true` - Sync started successfully
- `false` - Failed to start (error in DBus error field)

**Errors:**
- `"sync engine not available"` - Entertainment API not configured
- `"access denied"` - Caller is not service owner
- `"PortalError:TYPE:HINT"` - Screen capture permission error

**Portal Errors:**
- `"PortalError:UserCancelled:..."` - User cancelled permission dialog
- `"PortalError:Timeout:..."` - Permission dialog timed out (2 minutes)
- `"PortalError:InvalidSession:..."` - Previous session is invalid

**Important Notes:**
- **User must approve screen sharing** via XDG Desktop Portal dialog
- This is normal Wayland security behavior
- Dialog may appear on first run or after logout
- Subsequent runs may not show dialog if "Remember" was checked

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StartSync
```

**Example (Qt/C++):**
```cpp
QDBusReply<bool> reply = iface.call("StartSync");

if (reply.isValid() && reply.value()) {
    qDebug() << "Screen sync started";
    // Update UI to show sync is active
} else {
    QString error = reply.error().message();

    // Check for portal errors
    if (error.startsWith("PortalError:")) {
        QStringList parts = error.split(":");
        QString errorType = parts[1];
        QString hint = parts[2];

        if (errorType == "UserCancelled") {
            qDebug() << "User cancelled screen sharing";
        } else if (errorType == "Timeout") {
            qDebug() << "Permission dialog timed out";
        }
    } else {
        qWarning() << "Failed to start sync:" << error;
    }
}
```

**Example (Go):**
```go
var success bool
err := obj.Call("org.kde.plasma.hue.StartSync", 0).Store(&success)
if err != nil {
    if strings.Contains(err.Error(), "PortalError:") {
        parts := strings.Split(err.Error(), ":")
        if len(parts) >= 2 {
            errorType := parts[1]
            fmt.Printf("Portal error: %s\n", errorType)
        }
    } else {
        fmt.Println("Error:", err)
    }
}
```

---

#### StopSync

Stops screen synchronization.

**Signature:** `StopSync() → bool`

**Returns:**
- `true` - Sync stopped successfully
- `false` - Failed to stop (error in DBus error field)

**Errors:**
- `"sync engine not available"` - Entertainment API not configured
- `"access denied"` - Caller is not service owner

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StopSync
```

**Example (Qt/C++):**
```cpp
QDBusReply<bool> reply = iface.call("StopSync");

if (reply.isValid() && reply.value()) {
    qDebug() << "Screen sync stopped";
} else {
    qWarning() << "Failed to stop sync:" << reply.error().message();
}
```

---

#### IsSyncing

Returns whether screen sync is currently active.

**Signature:** `IsSyncing() → bool`

**Returns:**
- `true` - Sync is running
- `false` - Sync is not running or not available

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing
```

**Example (Qt/C++):**
```cpp
QDBusReply<bool> reply = iface.call("IsSyncing");
if (reply.isValid()) {
    bool syncing = reply.value();
    // Update UI icon/state
    if (syncing) {
        trayIcon->setIcon("media-record");
    } else {
        trayIcon->setIcon("preferences-desktop-display-color");
    }
}
```

---

#### GetSyncSettings

Returns current screen sync configuration.

**Signature:** `GetSyncSettings() → map[string]interface{}`

**Returns (map keys):**
- `fps` (int) - Target frames per second (1-60)
- `subsampleWidth` (int) - Resize width for processing (16-256)
- `monitor` (string) - Monitor name to capture (empty = default)
- `enabled` (bool) - Whether sync is enabled in config

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetSyncSettings

# Output:
#   dict entry(
#     string "fps"
#     variant int32 30
#   )
#   dict entry(
#     string "subsampleWidth"
#     variant int32 64
#   )
#   ...
```

**Example (Qt/C++):**
```cpp
QDBusReply<QVariantMap> reply = iface.call("GetSyncSettings");
if (reply.isValid()) {
    QVariantMap settings = reply.value();
    int fps = settings["fps"].toInt();
    int subsampleWidth = settings["subsampleWidth"].toInt();
    QString monitor = settings["monitor"].toString();

    qDebug() << "FPS:" << fps;
    qDebug() << "Subsample Width:" << subsampleWidth;
}
```

---

#### SetSyncSettings

Updates screen sync configuration.

**Signature:** `SetSyncSettings(fps: int32, subsampleWidth: int32, monitor: string) → bool`

**Parameters:**
- `fps` (int32) - Target frames per second (1-60)
- `subsampleWidth` (int32) - Resize width for processing (16-256)
- `monitor` (string) - Monitor name (empty = default monitor)

**Returns:**
- `true` - Settings saved successfully
- `false` - Failed (error in DBus error field)

**Errors:**
- `"FPS must be between 1 and 60"` - Invalid FPS
- `"subsample width must be between 16 and 256"` - Invalid width
- `"access denied"` - Caller is not service owner
- `"failed to save sync settings"` - File system error

**Note:** Changes require restarting sync to take effect.

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetSyncSettings \
  int32:30 \
  int32:64 \
  string:""
```

**Example (Qt/C++):**
```cpp
int fps = 30;
int subsampleWidth = 64;
QString monitor = "";

QDBusReply<bool> reply = iface.call("SetSyncSettings", fps, subsampleWidth, monitor);

if (reply.isValid() && reply.value()) {
    qDebug() << "Sync settings updated";
    // Prompt user to restart sync if running
} else {
    qWarning() << "Failed:" << reply.error().message();
}
```

---

### Gaming Mode

Gaming mode automatically starts screen sync when a game is detected.

#### SetGamingMode

Enables or disables gaming mode.

**Signature:** `SetGamingMode(enabled: bool) → bool`

**Parameters:**
- `enabled` (bool) - True to enable, false to disable

**Returns:**
- `true` - Gaming mode state changed successfully
- `false` - Failed (error in DBus error field)

**Errors:**
- `"access denied"` - Caller is not service owner
- `"failed to save gaming mode config"` - File system error

**Side Effects:**
- When enabled: Starts gaming detector process
- When disabled: Stops gaming detector process

**Example (dbus-send):**
```bash
# Enable gaming mode
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetGamingMode \
  boolean:true
```

**Example (Qt/C++):**
```cpp
bool enable = true;
QDBusReply<bool> reply = iface.call("SetGamingMode", enable);

if (reply.isValid() && reply.value()) {
    qDebug() << "Gaming mode" << (enable ? "enabled" : "disabled");
} else {
    qWarning() << "Failed:" << reply.error().message();
}
```

---

#### IsGamingModeEnabled

Returns whether gaming mode is enabled in configuration.

**Signature:** `IsGamingModeEnabled() → bool`

**Returns:**
- `true` - Gaming mode is enabled
- `false` - Gaming mode is disabled

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsGamingModeEnabled
```

**Example (Qt/C++):**
```cpp
QDBusReply<bool> reply = iface.call("IsGamingModeEnabled");
if (reply.isValid()) {
    bool enabled = reply.value();
    // Update checkbox or toggle state
}
```

---

#### IsGamingModeActive

Returns whether gaming activity is currently detected.

**Signature:** `IsGamingModeActive() → bool`

**Returns:**
- `true` - Game is currently detected
- `false` - No game detected or gaming mode disabled

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsGamingModeActive
```

**Example (Qt/C++):**
```cpp
QDBusReply<bool> reply = iface.call("IsGamingModeActive");
if (reply.isValid()) {
    bool active = reply.value();
    // Update icon if gaming + syncing
    if (active) {
        trayIcon->setIcon("applications-games");
    }
}
```

---

### Configuration

#### GetTrayIcons

Returns tray icon theme names for different states.

**Signature:** `GetTrayIcons() → (gaming: string, syncing: string, idle: string)`

**Returns:**
- `gaming` (string) - Icon when gaming + syncing
- `syncing` (string) - Icon when syncing (no game)
- `idle` (string) - Icon when idle

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetTrayIcons

# Output:
#   string "applications-games"
#   string "media-record"
#   string "preferences-desktop-display-color"
```

**Example (Qt/C++):**
```cpp
QDBusMessage reply = iface.call("GetTrayIcons");
if (reply.type() == QDBusMessage::ReplyMessage) {
    QList<QVariant> args = reply.arguments();
    QString gamingIcon = args[0].toString();
    QString syncingIcon = args[1].toString();
    QString idleIcon = args[2].toString();
}
```

---

#### SetTrayIcons

Updates tray icon theme names.

**Signature:** `SetTrayIcons(gaming: string, syncing: string, idle: string) → bool`

**Parameters:**
- `gaming` (string) - Icon name for gaming + syncing state
- `syncing` (string) - Icon name for syncing state
- `idle` (string) - Icon name for idle state

**Returns:**
- `true` - Icons saved successfully
- `false` - Failed (error in DBus error field)

**Errors:**
- `"access denied"` - Caller is not service owner
- `"failed to save config"` - File system error

**Example (dbus-send):**
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetTrayIcons \
  string:"applications-games" \
  string:"media-record" \
  string:"preferences-desktop-display-color"
```

---

## Error Handling

### Error Types

DBus errors are returned as `*dbus.Error` with the following formats:

**Configuration Errors:**
- `"bridge address is required"`
- `"API key is required"`
- `"no grouped light configured"`
- `"sync engine not available - check Entertainment API configuration"`

**Validation Errors:**
- `"brightness must be 0-100"`
- `"FPS must be between 1 and 60 (got X)"`
- `"subsample width must be between 16 and 256 (got X)"`
- `"displayName exceeds maximum length"`

**Access Control Errors:**
- `"access denied: only the service owner can perform this operation"`
- `"access denied: unable to verify caller identity"`

**Connection Errors:**
- `"bridge still unreachable"`
- `"bridge is not reachable"`
- `"hue client not initialized"`

**Portal Errors (Screen Capture):**
- `"PortalError:UserCancelled:user cancelled screen sharing permission"`
- `"PortalError:Timeout:permission dialog timed out after 2 minutes"`
- `"PortalError:InvalidSession:previous session is no longer valid"`

**Scene Errors:**
- `"scene not found: XYZ"`

**File System Errors:**
- `"failed to save config: <reason>"`

### Error Handling Patterns

**Qt/C++ Pattern:**
```cpp
QDBusReply<bool> reply = iface.call("MethodName", arg1, arg2);

if (!reply.isValid()) {
    QDBusError error = reply.error();
    qWarning() << "DBus error:" << error.message();

    // Show user-friendly message
    KNotification::event("Error",
                        "Hue Control",
                        "Failed to perform action: " + error.message(),
                        "dialog-error");
    return;
}

bool success = reply.value();
if (!success) {
    qWarning() << "Operation returned false";
}
```

**Go Pattern:**
```go
var result bool
err := obj.Call("org.kde.plasma.hue.MethodName", 0, arg1, arg2).Store(&result)

if err != nil {
    // Check if it's a DBus error
    if dbusErr, ok := err.(dbus.Error); ok {
        fmt.Printf("DBus error: %s\n", dbusErr.Error())
    } else {
        fmt.Printf("Call error: %v\n", err)
    }
    return
}

if !result {
    fmt.Println("Operation failed (returned false)")
}
```

**Bash Pattern:**
```bash
if dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.MethodName \
  string:"arg1" 2>&1 | grep -q "Error"; then
    echo "DBus call failed"
    exit 1
fi
```

---

## Usage Examples

### Complete Scene Control Example (Qt/C++)

```cpp
#include <QDBusConnection>
#include <QDBusInterface>
#include <QDBusReply>
#include <KNotification>
#include <QDebug>

class HueController {
public:
    HueController() {
        // Connect to DBus service
        iface = new QDBusInterface("org.kde.plasma.hue",
                                   "/org/kde/plasma/hue",
                                   "org.kde.plasma.hue",
                                   QDBusConnection::sessionBus(),
                                   this);

        if (!iface->isValid()) {
            qCritical() << "Failed to connect to Hue backend";
        }
    }

    void activateScene(const QString &sceneName) {
        QDBusReply<QString> reply = iface->call("ActivateScene", sceneName);

        if (reply.isValid()) {
            QString result = reply.value();
            KNotification::event("SceneActivated",
                                "Hue Control",
                                result,
                                "preferences-desktop-display-color");
        } else {
            KNotification::event("Error",
                                "Hue Control",
                                "Failed to activate scene: " + reply.error().message(),
                                "dialog-error");
        }
    }

    QStringList getScenes() {
        QDBusReply<QStringList> reply = iface->call("GetScenes");
        if (reply.isValid()) {
            return reply.value();
        }
        return QStringList();
    }

private:
    QDBusInterface *iface;
};

// Usage
HueController controller;
QStringList scenes = controller.getScenes();
for (const QString &scene : scenes) {
    qDebug() << "Available scene:" << scene;
}
controller.activateScene("Living Room - Relax");
```

### Complete Screen Sync Example (Qt/C++)

```cpp
class ScreenSyncController {
public:
    ScreenSyncController() {
        iface = new QDBusInterface("org.kde.plasma.hue",
                                   "/org/kde/plasma/hue",
                                   "org.kde.plasma.hue",
                                   QDBusConnection::sessionBus());

        // Setup timer to check sync status
        statusTimer = new QTimer(this);
        connect(statusTimer, &QTimer::timeout, this, &ScreenSyncController::updateStatus);
        statusTimer->start(1000); // Check every second
    }

    void toggleSync() {
        QDBusReply<bool> reply = iface->call("IsSyncing");
        if (!reply.isValid()) {
            qWarning() << "Failed to get sync status";
            return;
        }

        bool currentlySyncing = reply.value();

        if (currentlySyncing) {
            stopSync();
        } else {
            startSync();
        }
    }

    void startSync() {
        QDBusReply<bool> reply = iface->call("StartSync");

        if (reply.isValid() && reply.value()) {
            qDebug() << "Screen sync started";
            emit syncStatusChanged(true);
        } else {
            QString error = reply.error().message();

            // Handle portal errors specially
            if (error.startsWith("PortalError:")) {
                QStringList parts = error.split(":");
                if (parts.size() >= 3) {
                    QString errorType = parts[1];
                    QString hint = parts[2];

                    if (errorType == "UserCancelled") {
                        KNotification::event("Info",
                                            "Hue Control",
                                            "Screen sharing was cancelled",
                                            "dialog-information");
                    } else if (errorType == "Timeout") {
                        KNotification::event("Warning",
                                            "Hue Control",
                                            "Screen sharing permission timed out",
                                            "dialog-warning");
                    }
                }
            } else {
                KNotification::event("Error",
                                    "Hue Control",
                                    "Failed to start screen sync: " + error,
                                    "dialog-error");
            }
        }
    }

    void stopSync() {
        QDBusReply<bool> reply = iface->call("StopSync");

        if (reply.isValid() && reply.value()) {
            qDebug() << "Screen sync stopped";
            emit syncStatusChanged(false);
        } else {
            qWarning() << "Failed to stop sync:" << reply.error().message();
        }
    }

private slots:
    void updateStatus() {
        QDBusReply<bool> reply = iface->call("IsSyncing");
        if (reply.isValid()) {
            bool syncing = reply.value();

            // Update tray icon based on status
            if (syncing) {
                trayIcon->setIcon(QIcon::fromTheme("media-record"));
                trayIcon->setToolTip("Hue Control (Syncing)");
            } else {
                trayIcon->setIcon(QIcon::fromTheme("preferences-desktop-display-color"));
                trayIcon->setToolTip("Hue Control");
            }
        }
    }

signals:
    void syncStatusChanged(bool syncing);

private:
    QDBusInterface *iface;
    QTimer *statusTimer;
    KStatusNotifierItem *trayIcon;
};
```

### Monitoring Connection Status (Go)

```go
package main

import (
    "fmt"
    "time"
    "github.com/godbus/dbus/v5"
)

func monitorConnection() {
    conn, err := dbus.ConnectSessionBus()
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    obj := conn.Object("org.kde.plasma.hue", "/org/kde/plasma/hue")

    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        var status map[string]interface{}
        err := obj.Call("org.kde.plasma.hue.GetConnectionStatus", 0).Store(&status)

        if err != nil {
            fmt.Printf("Error: %v\n", err)
            continue
        }

        connected := status["connected"].(bool)
        bridgeIP := status["bridgeIP"].(string)
        lastError := status["lastError"].(string)

        if connected {
            fmt.Printf("✓ Connected to bridge at %s\n", bridgeIP)
        } else {
            fmt.Printf("✗ Not connected to %s: %s\n", bridgeIP, lastError)

            // Try to reconnect
            var success bool
            err = obj.Call("org.kde.plasma.hue.RetryConnection", 0).Store(&success)
            if err == nil && success {
                fmt.Println("✓ Reconnected successfully")
            }
        }
    }
}
```

### CLI Scene Activation Script (Bash)

```bash
#!/bin/bash
# activate-scene.sh - Activate a Hue scene from command line

DBUS_DEST="org.kde.plasma.hue"
DBUS_PATH="/org/kde/plasma/hue"
DBUS_IFACE="org.kde.plasma.hue"

# Check if backend is running
if ! dbus-send --session --print-reply --dest=org.freedesktop.DBus \
     /org/freedesktop/DBus org.freedesktop.DBus.ListNames \
     | grep -q "$DBUS_DEST"; then
    echo "Error: Hue backend is not running"
    exit 1
fi

# Get list of scenes
echo "Available scenes:"
scenes=$(dbus-send --session --print-reply \
    --dest="$DBUS_DEST" \
    "$DBUS_PATH" \
    "${DBUS_IFACE}.GetScenes" \
    | grep -oP '(?<=string ").*(?=")')

i=1
while IFS= read -r scene; do
    echo "$i) $scene"
    ((i++))
done <<< "$scenes"

# Prompt for scene selection
read -p "Select scene number: " selection

# Get selected scene name
scene_name=$(echo "$scenes" | sed -n "${selection}p")

if [ -z "$scene_name" ]; then
    echo "Invalid selection"
    exit 1
fi

# Activate scene
echo "Activating: $scene_name"
result=$(dbus-send --session --print-reply \
    --dest="$DBUS_DEST" \
    "$DBUS_PATH" \
    "${DBUS_IFACE}.ActivateScene" \
    string:"$scene_name" \
    2>&1)

if echo "$result" | grep -q "Error"; then
    echo "Failed to activate scene"
    echo "$result"
    exit 1
else
    echo "Success!"
fi
```

---

## Type Reference

### Primitive Types

**DBus Type Signature → Go Type → Qt Type:**
- `s` (string) → `string` → `QString`
- `b` (boolean) → `bool` → `bool`
- `i` (int32) → `int32` → `int` or `qint32`
- `u` (uint32) → `uint32` → `quint32`
- `as` (array of strings) → `[]string` → `QStringList`
- `a{sv}` (dict) → `map[string]interface{}` → `QVariantMap`

### Struct Types

**GroupedLight:**
```go
// Go
struct {
    ID   string
    Name string
    Type string
}
```

```cpp
// Qt/C++
struct GroupedLight {
    QString id;
    QString name;
    QString type;
};
Q_DECLARE_METATYPE(GroupedLight)
```

**DBus Signature:** `(sss)` - Struct with 3 strings

---

## Best Practices

### 1. Always Check Errors

```cpp
// ✅ Good
QDBusReply<bool> reply = iface.call("Method");
if (!reply.isValid()) {
    qWarning() << "Error:" << reply.error().message();
    return;
}

// ❌ Bad
QDBusReply<bool> reply = iface.call("Method");
bool result = reply.value(); // Crashes if error occurred
```

### 2. Use Async Calls for UI Applications

```cpp
// ✅ Good - Non-blocking UI
QDBusPendingCall async = iface.asyncCall("Method");
QDBusPendingCallWatcher *watcher = new QDBusPendingCallWatcher(async, this);
connect(watcher, &QDBusPendingCallWatcher::finished,
        this, &MyClass::handleReply);

// ❌ Bad - Blocks UI thread
QDBusReply<bool> reply = iface.call("Method"); // UI freezes during call
```

### 3. Cache DBusInterface Objects

```cpp
// ✅ Good - Create once, reuse
class MyClass {
    QDBusInterface *iface;
public:
    MyClass() {
        iface = new QDBusInterface("org.kde.plasma.hue", ...);
    }
};

// ❌ Bad - Creates new connection every call
void myFunction() {
    QDBusInterface iface("org.kde.plasma.hue", ...);
    iface.call("Method");
} // Connection destroyed
```

### 4. Handle Portal Errors Gracefully

```cpp
// ✅ Good - Parse and explain portal errors
QString error = reply.error().message();
if (error.startsWith("PortalError:")) {
    // Parse error type and show user-friendly message
    showPortalErrorDialog(error);
} else {
    showGenericError(error);
}

// ❌ Bad - Show raw error to user
QMessageBox::critical(this, "Error", reply.error().message());
```

### 5. Validate Before Calling

```cpp
// ✅ Good - Check status first
QDBusReply<QString> status = iface.call("GetStatus");
if (status.value() != "Ready") {
    qWarning() << "Backend not ready";
    return;
}
iface.call("StartSync");

// ❌ Bad - Call without checking
iface.call("StartSync"); // May fail if not configured
```

---

## Debugging

### Check Service Status

```bash
# Is service registered?
dbus-send --session --dest=org.freedesktop.DBus \
  --print-reply /org/freedesktop/DBus \
  org.freedesktop.DBus.ListNames \
  | grep hue

# Get introspection
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.freedesktop.DBus.Introspectable.Introspect
```

### Monitor DBus Traffic

```bash
# Monitor all Hue-related DBus calls
dbus-monitor --session "interface='org.kde.plasma.hue'"

# Monitor specific method
dbus-monitor --session "interface='org.kde.plasma.hue',member='ActivateScene'"
```

### Test Methods from CLI

```bash
# Get status
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus

# Call method with arguments
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.SetBrightness \
  int32:75
```

### Use D-Feet GUI Tool

```bash
# Install d-feet (graphical DBus browser)
sudo apt install d-feet

# Launch
d-feet
```

Navigate to Session Bus → org.kde.plasma.hue to explore methods and call them interactively.

---

## See Also

- **Architecture:** See `ARCHITECTURE_DEEP_DIVE.md` for system design
- **Configuration:** See `CONFIGURATION.md` for config.yaml reference
- **Development:** See `DEVELOPMENT.md` for building and testing
- **Troubleshooting:** See `TROUBLESHOOTING.md` for common issues

---

**API Version:** 1.0
**Last Updated:** April 2026
**Backend Version:** Compatible with khuey backend v1.x
