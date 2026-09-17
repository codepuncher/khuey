# KDE Hue Control - Troubleshooting Guide

This guide covers common issues, their causes, and solutions for KDE Hue Control.

## Table of Contents

1. [Installation Issues](#installation-issues)
2. [Backend Issues](#backend-issues)
3. [Tray App Issues](#tray-app-issues)
4. [DBus Communication](#dbus-communication)
5. [Screen Sync Issues](#screen-sync-issues)
6. [Bridge Connection](#bridge-connection)
7. [Gaming Mode Issues](#gaming-mode-issues)
8. [Performance Issues](#performance-issues)

---

## Installation Issues

### Dependencies Missing

**Symptom:** Install script fails with missing command errors

**Cause:** Required development tools not installed

**Solution:**
```bash
# Install all required dependencies
sudo pacman -S go cmake base-devel pkgconf qt6-base kstatusnotifieritem

# Verify installation
go version
cmake --version
pkg-config --modversion Qt6Core
```

### Build Fails

**Symptom:** `go build` or `make` fails during installation

**Backend Build Issues:**
```bash
# Common causes:
# 1. Go version too old (need 1.21+)
go version

# 2. Missing Go modules
cd backend
go mod download
go mod verify

# 3. CGo compilation issues
# Check libpipewire is installed
pkg-config --modversion libpipewire-0.3

# Install if missing
sudo pacman -S pipewire
```

**Tray App Build Issues:**
```bash
# Common causes:
# 1. Qt6 not found
pkg-config --modversion Qt6Core Qt6Widgets Qt6DBus

# 2. KF6StatusNotifierItem missing (it has no pkg-config file)
pacman -Q kstatusnotifieritem

# Install if missing
sudo pacman -S kstatusnotifieritem

# 3. Stale CMake cache
cd trayapp
rm -rf CMakeCache.txt CMakeFiles/
cmake .
make
```

### Service Won't Start

**Symptom:** `systemctl --user start hue-backend` fails

**Diagnosis:**
```bash
# Check service status
systemctl --user status hue-backend

# View detailed logs
journalctl --user -u hue-backend -n 50
```

**Common causes:**

1. **Config file missing:**
```bash
ls ~/.openhue/config.yaml
# If missing, create it or run: openhue setup
```

2. **Permissions incorrect:**
```bash
chmod 600 ~/.openhue/config.yaml
```

3. **Binary path wrong:**
```bash
# Check service file
cat ~/.config/systemd/user/hue-backend.service | grep ExecStart
# Should point to actual hue-sync binary location
```

---

## Backend Issues

### Backend Not Running

**Quick Check:**
```bash
# Is it running?
systemctl --user is-active hue-backend
# Returns: active or inactive

# If inactive, check why
systemctl --user status hue-backend

# Try starting
systemctl --user start hue-backend

# View logs
journalctl --user -u hue-backend -f
```

### Backend Crashes on Startup

**Symptom:** Service starts then immediately stops

**Diagnosis:**
```bash
# View crash logs
journalctl --user -u hue-backend -n 100

# Try running manually to see errors
./backend/hue-sync
```

**Common causes:**

1. **Config file invalid:**
```bash
# Check config syntax
cat ~/.openhue/config.yaml
# Must be valid YAML

# Validate required fields
grep "^Bridge:" ~/.openhue/config.yaml
grep "^Key:" ~/.openhue/config.yaml
```

2. **Port already in use:**
```bash
# DBus service name conflict (rare)
dbus-send --session --dest=org.freedesktop.DBus \
  --print-reply /org/freedesktop/DBus \
  org.freedesktop.DBus.ListNames | grep hue

# If multiple found, kill old instances
pkill -f hue-sync
systemctl --user restart hue-backend
```

### Backend High CPU Usage

**Symptom:** `hue-sync` using >20% CPU when idle

**Diagnosis:**
```bash
# Check if sync is running
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing

# If true, stop it
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StopSync
```

**Causes:**
- Screen sync running (expected: ~15-25% CPU at 30 FPS)
- Gaming mode polling (expected: <1% CPU)
- Infinite error loop (bug - check logs)

**If high CPU when not syncing:**
```bash
# Check logs for repeated errors
journalctl --user -u hue-backend | tail -100

# Restart service
systemctl --user restart hue-backend

# If issue persists, file a bug report
```

---

## Tray App Issues

### Tray Icon Not Appearing

**Quick Check:**
```bash
# Is it running?
ps aux | grep hue-tray

# If not, start it
cd /path/to/khuey/trayapp
./hue-tray &

# Check for errors in terminal output
```

**Common causes:**

1. **KDE Plasma not running:**
```bash
# Check if Plasma is active
ps aux | grep plasmashell

# Restart Plasma (Ctrl+Alt+Esc or)
kquitapp6 plasmashell && kstart plasmashell
```

2. **StatusNotifier protocol issue:**
```bash
# Check if StatusNotifier is available
qdbus org.kde.StatusNotifierWatcher /StatusNotifierWatcher

# If error, restart Plasma
```

3. **Multiple instances:**
```bash
# Kill all instances
pkill -f hue-tray

# Start single instance
./trayapp/hue-tray &
```

### Tray Icon Shows But Menu Empty

**Symptom:** Right-click shows empty menu or "Not Connected"

**Diagnosis:**
```bash
# Check backend is running
systemctl --user status hue-backend

# Check DBus communication
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus
```

**If "Not Connected":**
- Backend running but can't reach bridge
- Check [Bridge Connection](#bridge-connection) section

**If DBus error:**
- Backend not running: `systemctl --user start hue-backend`
- DBus service not registered: Check backend logs

### Tray App Crashes

**Symptom:** Icon disappears, no process found

**Diagnosis:**
```bash
# Run manually to see crash output
cd /path/to/khuey/trayapp
./hue-tray

# Look for:
# - Segmentation fault
# - Qt warnings
# - DBus errors
```

**Common causes:**

1. **Qt library mismatch:**
```bash
# Check Qt version
qmake --version
# Should be Qt6

# Rebuild with correct Qt
cd trayapp
rm -rf CMakeCache.txt CMakeFiles/
cmake .
make
```

2. **DBus communication issue:**
- Backend crashed → tray loses connection
- Solution: Improve error handling or restart both

---

## DBus Communication

### "Service Not Available" Errors

**Symptom:** Tray shows errors, DBus commands fail

**Diagnosis:**
```bash
# Check if service is registered
dbus-send --session --dest=org.freedesktop.DBus \
  --print-reply /org/freedesktop/DBus \
  org.freedesktop.DBus.ListNames | grep hue

# Should show: string "org.kde.plasma.hue"
```

**If service not registered:**
```bash
# Backend not running
systemctl --user start hue-backend

# Wait 2 seconds for registration
sleep 2

# Try again
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetStatus
```

### DBus Method Calls Timeout

**Symptom:** Tray freezes, commands take >25 seconds

**Causes:**
1. Backend busy (syncing at high FPS)
2. Bridge unreachable (network timeout)
3. Backend deadlocked (bug)

**Solutions:**
```bash
# Check backend logs
journalctl --user -u hue-backend -n 50

# If syncing, stop it
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StopSync

# If deadlocked, restart
systemctl --user restart hue-backend
```

### Monitor DBus Traffic

**Debug DBus issues:**
```bash
# Watch all hue-related DBus traffic
dbus-monitor --session "interface='org.kde.plasma.hue'"

# In another terminal, trigger action
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.GetScenes

# Observe traffic in monitor
```

---

## Screen Sync Issues

### 🚨 Screen Sync "Hangs" or "Doesn't Work"

**THIS IS THE #1 MISUNDERSTOOD ISSUE**

**Symptom:** StartSync called, but lights don't change. Logs show "no frame available".

**Cause:** **You haven't approved the permission dialog yet.**

**What Actually Happens:**

1. You call `StartSync` (via tray or DBus)
2. Backend activates Entertainment Area ✅
3. Backend initializes PipeWire capture ✅
4. **XDG Portal shows GUI dialog** ← You must interact with this!
5. Dialog asks: "Which screen do you want to share?"
6. **User must click the dialog, select monitor, click "Share"**
7. ONLY THEN does PipeWire start streaming frames
8. Lights sync to screen ✅

**The Dialog is Invisible When Testing via CLI!**

If you're running `dbus-send` commands from terminal, the GUI dialog appears on screen but you can't see it in the terminal output. To an automated agent, it looks like the feature "hung" - but really it's just waiting for human interaction.

**Solution:**
```bash
# When testing via CLI, ALWAYS tell the user first:
echo "⚠️  A GUI dialog will appear - approve screen sharing!"

# Then call StartSync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StartSync

# User approves dialog (you can't see this step)
# ... human time passes ...

# Check if it worked (should be true after approval)
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing
```

**Better: Use Tray App**
```bash
# Start tray app
./trayapp/hue-tray &

# User clicks "Start Screen Sync" in menu
# User sees dialog → approves
# Feature works immediately
```

**Verify It's Working:**
```bash
# After dialog approved, check logs for frames
journalctl --user -u hue-backend --since "30 seconds ago" | grep -i frame

# Should see:
# [PipeWire] Stream state changed: CONNECTING -> STREAMING
# [PipeWire] Video format: 2560x1440
# (no more "no frame available" errors)

# Lights should be changing colors!
```

### Screen Sync Stops After Permission Dialog Timeout

**Symptom:** Sync starts, then stops after 2 minutes. Logs show "portal timeout".

**Cause:** User didn't approve permission dialog within 2 minutes.

**Solution:**
- This is expected behavior (prevents indefinite hangs)
- Click "Start Screen Sync" again
- Approve the dialog promptly this time

**To change timeout:**
```go
// Edit backend/internal/capture/portal.go
// Change timeout in waitForResponse:
timeout := 5 * time.Minute  // Increase to 5 minutes
```

### Screen Sync Stops With "Circuit Breaker"

**Symptom:** Sync runs for a while, then stops. Logs show "circuit breaker tripped".

**Cause:** 30 consecutive frame capture errors (circuit breaker activated).

**Common reasons:**
1. Screen sharing permission revoked mid-session
2. PipeWire crashed
3. GPU driver issue
4. System suspended/resumed

**Solution:**
```bash
# Check PipeWire is running
systemctl --user status pipewire

# If stopped, start it
systemctl --user start pipewire

# Try screen sync again
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StartSync
```

### Screen Sync Low FPS

**Symptom:** Lights update slowly, logs show <30 FPS

**Diagnosis:**
```bash
# Check actual FPS in logs
journalctl --user -u hue-backend -f | grep "FPS"

# Look for:
# ✅ Screen sync started at 30 FPS
# [Metrics] FPS: 29.8, Frame time: 11.2ms  ← Good
# [Metrics] FPS: 15.3, Frame time: 45.8ms  ← Bad
```

**Common causes:**

1. **High subsample width:**
```yaml
# Edit ~/.openhue/config.yaml
sync:
  subsampleWidth: 64  # Try lower (32, 48)
```

2. **CPU overloaded:**
```bash
# Check CPU usage
top -p $(pgrep hue-sync)

# If >80%, reduce load:
# - Close other apps
# - Lower subsampleWidth
# - Lower FPS target
```

3. **Many zones configured:**
```yaml
# Each channel adds processing time
# Try reducing active channels or zone sizes
```

### Screen Sync Wrong Colors

**Symptom:** Lights don't match screen colors

**Common causes:**

1. **Zone mapping incorrect:**
```yaml
# Check UV coordinates in config
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}  # Top-left
    uvB: {x: 0.5, y: 1.0}  # Bottom-right
# Make sure zones cover intended screen areas
```

2. **Gamma correction off:**
```yaml
# Adjust gamma factor per channel
channels:
  - id: 0
    gammaFactor: 2.2  # Try 1.8, 2.0, 2.4
```

3. **Wrong Entertainment Area:**
```bash
# Verify Entertainment Area contains correct lights
cd backend
go run ./cmd/get-entertainment-info
```

---

## Bridge Connection

### "Bridge Unreachable" Error

**Quick Checks:**
```bash
# 1. Is bridge IP correct?
grep "^Bridge:" ~/.openhue/config.yaml
# Should show bridge IP like: Bridge: "192.168.1.100"

# 2. Can you ping it?
ping -c 3 $(grep "^Bridge:" ~/.openhue/config.yaml | cut -d'"' -f2)

# 3. Can you reach API?
curl -k https://$(grep "^Bridge:" ~/.openhue/config.yaml | cut -d'"' -f2)/api/config
```

**If ping fails:**
- Bridge powered off → Turn it on
- Bridge on different network → Check WiFi/Ethernet
- Firewall blocking → Check `iptables` or `ufw`

**If API fails:**
- Wrong IP in config → Update config.yaml
- Bridge API disabled (rare) → Check Hue app
- Certificate issue → Backend uses `-k` (insecure), should work

### "Invalid API Key" Error

**Symptom:** Backend runs, but all API calls fail with 401/403

**Cause:** API key in config is invalid or expired

**Solution:**
```bash
# Re-pair with bridge using openhue-cli
openhue setup

# This will:
# 1. Discover bridge
# 2. Prompt to press button on bridge
# 3. Generate new API key
# 4. Save to ~/.openhue/config.yaml

# Restart backend
systemctl --user restart hue-backend
```

### Entertainment API Not Working

**Symptom:** Regular controls work, but screen sync fails

**Diagnosis:**
```bash
# Check Entertainment Area setup
cd backend
go run ./cmd/get-entertainment-info

# Should show:
# Entertainment Area: "Screen Sync"
# Lights: [...]
# Status: Active
```

**If no Entertainment Area found:**
```bash
# Create one
cd backend
go run ./cmd/register-entertainment

# Or create in official Hue app:
# Settings → Entertainment Areas → Add Area
```

**If clientkey missing:**
```yaml
# Edit ~/.openhue/config.yaml
# Add clientkey field (generated during bridge pairing)
clientkey: "YOUR-CLIENT-KEY-HERE"
```

---

## Gaming Mode Issues

### Gaming Mode Not Activating

**Symptom:** Games start, but screen sync doesn't auto-enable

**Diagnosis:**
```bash
# Check gaming detector is running
journalctl --user -u hue-backend | grep -i gaming

# Should see:
# [INFO] Gaming detector started
```

**If not seeing game detection:**
```yaml
# Check config has gaming mode enabled
# Edit ~/.openhue/config.yaml
gaming:
  enabled: true
  syncEnabled: true
```

**If game not detected:**
```bash
# Check game is in CachyOS database
# (Gaming detector uses CachyOS game list)

# Manually check if process is a game:
ps aux | grep game-name

# If process name doesn't match DB, detection won't work
```

### Gaming Mode Stuck On

**Symptom:** Game closed, but sync still running

**Diagnosis:**
```bash
# Check if any game processes still running
ps aux | grep -E "steam|game|wine"

# Check gaming detector state
journalctl --user -u hue-backend | grep -i gaming | tail -20
```

**Solution:**
```bash
# Manually stop sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StopSync

# Restart backend to reset state
systemctl --user restart hue-backend
```

---

## Performance Issues

### High Memory Usage

**Symptom:** `hue-sync` using >500MB RAM

**Diagnosis:**
```bash
# Check actual memory usage
ps aux | grep hue-sync | awk '{print $6}'  # RSS in KB

# Normal usage:
# Idle: ~50-100MB
# Syncing: ~150-250MB
# >500MB: Potential memory leak
```

**Solutions:**
```bash
# 1. Restart backend (immediate fix)
systemctl --user restart hue-backend

# 2. Check for memory leak
cd backend
go test -memprofile=mem.prof ./...
go tool pprof -http=:8080 mem.prof

# 3. File bug report with profile
```

### High CPU Usage During Sync

**Symptom:** Screen sync uses >50% CPU

**Expected:** 15-25% CPU at 30 FPS is normal

**If higher:**
```bash
# Profile CPU usage
cd backend
go run ./cmd/profile-sync -duration 15s

# Generates cpu.prof
go tool pprof -http=:8080 cpu.prof

# Look for unexpected hotspots
```

**Common causes:**
- High subsample width (lower it)
- Many channels configured (reduce zones)
- Debug logging enabled (disable verbose logs)

### Frame Drops

**Symptom:** Lights stutter, FPS <30

**Diagnosis:**
```bash
# Check metrics in logs
journalctl --user -u hue-backend -f | grep Metrics

# Look for:
# FPS: 29.8  ← Good
# FPS: 24.5  ← Frame drops
```

**Solutions:**
1. Lower subsample width: `subsampleWidth: 48` → `32`
2. Reduce channel count
3. Close background apps
4. Check system isn't thermal throttling

---

## Advanced Debugging

### Enable Debug Logging

**Edit backend code to increase verbosity:**
```go
// In main.go or relevant file, add:
log.SetLevel(log.DebugLevel)
```

### Capture Packets

**Monitor DTLS Entertainment API traffic:**
```bash
# Capture UDP packets to bridge
sudo tcpdump -i any -n udp and host 192.168.1.100

# Should see constant traffic when syncing
```

### Profile Performance

```bash
# Full profiling session
cd backend
go run ./cmd/profile-sync -duration 30s

# Generate reports
go tool pprof -top cpu.prof      # Top CPU consumers
go tool pprof -top mem.prof      # Top memory allocators
go tool pprof -http=:8080 cpu.prof  # Interactive web UI
```

---

## Getting Help

If none of these solutions work:

1. **Collect diagnostic info:**
```bash
# System info
uname -a
go version
cmake --version
pkg-config --modversion Qt6Core

# Service status
systemctl --user status hue-backend
ps aux | grep hue-tray

# Recent logs
journalctl --user -u hue-backend -n 100

# Config (REDACT API KEYS!)
cat ~/.openhue/config.yaml | grep -v "Key\|clientkey"
```

2. **File an issue:**
   - GitHub: https://github.com/codepuncher/khuey/issues
   - Include diagnostic info above
   - Describe what you tried from this guide

3. **Ask in discussions:**
   - KDE forums
   - Hue developer community
   - Linux desktop communities

---

## Common Error Messages

| Error Message | Cause | Solution |
|---------------|-------|----------|
| "Config file not found" | Missing ~/.openhue/config.yaml | Run `openhue setup` |
| "Bridge unreachable" | Network/bridge issue | Check bridge IP and network |
| "Invalid API key" | Wrong/expired key | Re-run `openhue setup` |
| "Entertainment Area not found" | No area configured | Create area in Hue app or run register tool |
| "No frame available yet" | Portal dialog not approved | Approve screen sharing dialog |
| "Circuit breaker tripped" | 30 consecutive capture errors | Check PipeWire status, restart sync |
| "Portal timeout" | Dialog not approved in 2 min | Start sync again, approve promptly |
| "Service not available" | Backend not running | `systemctl --user start hue-backend` |

---

For more information:
- Architecture: `ARCHITECTURE.md`
- Development: `DEVELOPMENT.md`
- Testing: `TESTING.md`
- Contributing: `CONTRIBUTING.md`
