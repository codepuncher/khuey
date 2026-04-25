# Quick Reference - Gaming Mode Commands

## Testing Commands

### Test Detection (Real-Time Monitor)
```bash
cd backend && ./test-gaming-detection
```
Shows live status of all detection methods. Launch a game to see it detect!

### Run Unit Tests
```bash
cd backend && go test ./...
```

### Run Gaming Package Tests
```bash
cd backend && go test ./internal/gaming -v
```

## Build Commands

### Build Backend
```bash
cd backend && go build -o hue-sync ./cmd/hue-sync
```

### Build Test Utility
```bash
cd backend && go build -o test-gaming-detection ./cmd/test-gaming-detection
```

## Service Management

### Restart Backend
```bash
systemctl --user restart hue-backend
```

### Check Backend Status
```bash
systemctl --user status hue-backend
```

### View Backend Logs (Real-Time)
```bash
journalctl --user -u hue-backend -f
```

### View Last 50 Log Lines
```bash
journalctl --user -u hue-backend -n 50
```

## DBus Commands

### Enable Gaming Mode
```bash
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true
```

### Disable Gaming Mode
```bash
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:false
```

### Check if Gaming Mode is Enabled (Config)
```bash
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled
```

### Check if Gaming is Currently Active (Detection)
```bash
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeActive
```

### Get Backend Status
```bash
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetStatus
```

## Manual Detection Testing

### Check systemd-inhibit
```bash
systemd-inhibit --list | grep -i game
```
Should show CachyOS game-performance when game is running.

### Check Power Profile
```bash
powerprofilesctl get
```
Should return "performance" when game is running (on CachyOS).

### Check Steam AppId
```bash
pgrep -a reaper | grep AppId
```
Shows Steam game process with AppId (e.g., AppId=489830 for Skyrim SE).

## Configuration

### View Gaming Mode Config
```bash
grep -A 8 "gamingMode:" ~/.openhue/config.yaml
```

### Edit Config
```bash
nano ~/.openhue/config.yaml
```

### Example Gaming Mode Config
```yaml
gamingMode:
  enabled: true                # Enable feature
  pollInterval: 2              # Check every 2 seconds
  debounceDelay: 5             # Wait 5s before triggering
  useSystemdInhibit: true      # CachyOS detection (primary)
  usePowerProfile: true        # Power profile validation
  useSteamAppId: true          # Steam AppId detection
  useGameMode: false           # Legacy fallback
  useFullscreen: false         # Legacy fallback
```

## Debugging

### Full Debug Sequence
```bash
# 1. Enable gaming mode
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true

# 2. Watch logs in real-time
journalctl --user -u hue-backend -f &

# 3. Run detection test in another terminal
cd backend && ./test-gaming-detection &

# 4. Launch game and observe detection

# 5. Check active status
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeActive
```

### Check Detection Methods Manually
```bash
# While game is running:
echo "systemd-inhibit:"
systemd-inhibit --list | grep -i game

echo "Power profile:"
powerprofilesctl get

echo "Steam AppId:"
pgrep -a reaper | grep AppId
```

## Logs to Look For

### Successful Startup
```
✅ systemd-inhibit detector initialized (CachyOS)
✅ Power profile detector initialized (CachyOS)
✅ Steam AppId detector initialized
✅ Gaming mode detector started
```

### Gaming Detected
```
🎮 Gaming detected - starting screen sync
✅ Screen sync enabled for immersive gaming
```

### Gaming Stopped
```
🎮 Gaming stopped - stopping screen sync
✅ Screen sync disabled
```

## Quick Troubleshooting

### Gaming mode not starting?
```bash
# Check if enabled
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled

# Check backend logs
journalctl --user -u hue-backend -n 50 | grep -i gaming
```

### Not detecting game?
```bash
# Test detection methods manually
systemd-inhibit --list | grep -i game
powerprofilesctl get
pgrep -a reaper | grep AppId

# Run test utility
cd backend && ./test-gaming-detection
```

### Detector not stopping?
```bash
# Force disable
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:false

# Restart backend
systemctl --user restart hue-backend
```

## Installation/Deployment

### Install New Backend
```bash
cd backend
go build -o hue-sync ./cmd/hue-sync
sudo cp hue-sync /usr/local/bin/
systemctl --user restart hue-backend
```

### Install Test Utility
```bash
cd backend
go build -o test-gaming-detection ./cmd/test-gaming-detection
sudo cp test-gaming-detection /usr/local/bin/
```

## Expected Test Output

### When Skyrim SE is Running
```
[17:59:36]
  systemd-inhibit: true ✅
  Power profile:   true ✅
  Steam AppId:     true (AppId: 489830) ✅
  🎮 Gaming Active: true ✅
```

### When No Game is Running
```
[17:59:38]
  systemd-inhibit: false
  Power profile:   false
  Steam AppId:     false
  🎮 Gaming Active: false
```

## Files to Review

- `backend/internal/gaming/systemd.go` - systemd-inhibit detector
- `backend/internal/gaming/powerprofile.go` - Power profile detector
- `backend/internal/gaming/steam.go` - Steam AppId detector
- `backend/internal/gaming/detector.go` - Main detection logic
- `backend/internal/config/config.go` - Config structure
- `backend/internal/dbus/service.go` - DBus service methods
- `USAGE.md` - User documentation
- `GAMING_MODE_IMPLEMENTATION.md` - Full implementation details
- `TEST_CHECKLIST.md` - Testing checklist

## Quick Start for Testing

```bash
# 1. Build everything
cd backend
go build -o hue-sync ./cmd/hue-sync
go build -o test-gaming-detection ./cmd/test-gaming-detection

# 2. Restart backend with new code
systemctl --user restart hue-backend

# 3. Enable gaming mode
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true

# 4. Launch Skyrim SE (or any game)
# Wait 2-7 seconds...

# 5. Check if screen sync auto-started
journalctl --user -u hue-backend -n 20 | grep -i gaming
```

That's it! Gaming Mode should now automatically start/stop screen sync. 🎮✨
