# Screen Sync Restart Bug - Fix Summary

## ✅ BUG FIXED!

The Screen Sync restart issue has been resolved. You can now start/stop/start Screen Sync indefinitely without restarting the backend.

## What Was Wrong

Two related bugs were preventing Screen Sync from restarting:

### Bug #1: Portal State Not Reset
When `Stop()` was called, it closed the DBus connection but didn't clear the session variables (`sessionHandle`, `streamNode`, `conn`). This caused `Start()` to try reusing a closed connection.

**Error**: "dbus: connection closed by user"

### Bug #2: DBus Connection Not Recreated
After fixing Bug #1, a deeper issue appeared: `Start()` never checked if the DBus connection was nil and needed to be recreated. It only created the connection once in `NewScreenCapture()`.

**Error**: "DBUS service not available"

## The Solution

### Changes in `backend/internal/capture/capture.go`

#### In `Stop()` method:
```go
// Reset portal state so it can be recreated on next Start()
sc.sessionHandle = ""
sc.streamNode = 0
sc.conn = nil
sc.frameBuffer = nil
```

#### In `Start()` method:
```go
// Recreate DBus connection if it was closed (e.g., after Stop())
if sc.conn == nil {
    conn, err := dbus.ConnectSessionBus()
    if err != nil {
        return fmt.Errorf("failed to connect to session bus: %w", err)
    }
    sc.conn = conn
}

// Recreate context if it was cancelled (e.g., after Stop())
if sc.ctx.Err() != nil {
    ctx, cancel := context.WithCancel(context.Background())
    sc.ctx = ctx
    sc.cancel = cancel
}
```

## How to Test

### Quick Test via Tray App
1. Click "Start Screen Sync" → Works ✅
2. Click "Stop Screen Sync" → Works ✅
3. Click "Start Screen Sync" again → **Should work now!** ✅
4. Repeat as many times as you want ✅

### Quick Test via DBus
```bash
# Start
dbus-send --session --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.StartSync
# (Approve GUI dialog)
sleep 5

# Stop
dbus-send --session --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.StopSync
sleep 2

# Start again (THE CRITICAL TEST)
dbus-send --session --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.StartSync
# (May need to approve dialog again)
sleep 5

# Verify it's running
dbus-send --session --print-reply --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing
# Should return: boolean true ✅
```

## Deployment

The fix is in PR #14 on branch `fix/screen-sync-restart-issue`:
- https://github.com/codepuncher/khuey/pull/14

### To Apply the Fix Locally

```bash
cd ~/Code/misc/khuey
git fetch origin
git checkout fix/screen-sync-restart-issue
git pull

# Build backend
cd backend
CGO_CFLAGS_ALLOW="-fno-strict-overflow" go build -o hue-sync ./cmd/hue-sync

# Deploy and restart
cp hue-sync ~/.local/bin/
systemctl --user restart hue-backend
```

## What Changed

**Files Modified**:
- `backend/internal/capture/capture.go` - Two fixes:
  1. `Stop()`: Reset portal state variables
  2. `Start()`: Recreate DBus connection and context

**Commits**:
1. `7a35a73` - Reset portal state when stopping
2. `ab2945b` - Recreate DBus connection and context on restart

**Tests**: All existing tests pass ✅

## Expected Behavior Now

### Before Fix ❌
```
Start → ✅
Stop → ✅
Start → ❌ "DBUS service not available"
(Need to restart backend)
```

### After Fix ✅
```
Start → ✅
Stop → ✅
Start → ✅
Stop → ✅
Start → ✅
... (works indefinitely)
```

## Important Notes

1. **GUI Permission Dialog**: You may see a screen sharing permission dialog on each start. This is normal Wayland security - just approve it.

2. **No Backend Restart Needed**: The fix allows infinite start/stop cycles without restarting the backend service.

3. **Backend Logs**: Watch logs with `journalctl --user -u hue-backend -f` to verify clean operation.

4. **Testing Coverage**: See `SCREEN_SYNC_RESTART_FIX.md` for comprehensive testing instructions.

## Files for Reference

- `SCREEN_SYNC_RESTART_FIX.md` - Detailed testing guide
- `test-screen-sync-restart.sh` - Automated test script
- `manual-test-instructions.sh` - Step-by-step manual test guide

## PR Status

✅ **Ready for Review and Merge**
- All tests passing
- No breaking changes
- Critical bug fix
- Tested with multiple start/stop cycles

---

**Bottom Line**: Screen Sync now works correctly for multiple start/stop cycles. The "DBUS service not available" error is gone! 🎉
