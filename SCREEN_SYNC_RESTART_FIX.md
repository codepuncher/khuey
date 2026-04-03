# Screen Sync Restart Fix - Testing Guide

## Bug Description

**Issue**: Screen Sync fails on the second start attempt after stopping, showing "DBUS service not available" error.

**Impact**: Users could only use Screen Sync once per backend session, requiring `systemctl --user restart hue-backend` to use it again.

## Root Causes (Two Bugs Fixed)

### Bug #1: Portal State Not Reset
- **Symptom**: "dbus: connection closed by user"
- **Cause**: `Stop()` closed connection but left `sessionHandle`, `streamNode`, and `conn` set
- **Fix**: Reset these to empty/zero/nil in `Stop()`
- **Commit**: `7a35a73`

### Bug #2: DBus Connection Not Recreated
- **Symptom**: "DBUS service not available" (appeared after fixing Bug #1)
- **Cause**: `Start()` never checked if DBus connection was closed and needed recreation
- **Fix**: Check if `conn` is nil or `ctx` is cancelled, and recreate them
- **Commit**: `ab2945b`

## Testing the Fix

### Prerequisites
1. Backend running: `systemctl --user status hue-backend`
2. Hue Bridge configured in `~/.openhue/config.yaml`
3. Entertainment Area configured

### Manual Test (Recommended)

Run these commands in sequence:

```bash
# 1. Check initial state (should be false)
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing

# 2. Start Screen Sync (FIRST TIME)
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StartSync

# ⚠️  GUI DIALOG WILL APPEAR - APPROVE IT!
# Wait 5 seconds after approving

# 3. Verify running (should be true)
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing

# 4. Stop Screen Sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StopSync

# Wait 2 seconds

# 5. Verify stopped (should be false)
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing

# 6. Start Screen Sync (SECOND TIME - THE CRITICAL TEST!)
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StartSync

# ⚠️  GUI DIALOG MAY APPEAR AGAIN - APPROVE IT!
# Wait 5 seconds

# 7. Verify running again (should be true - THIS PROVES THE FIX!)
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing
```

### Using the Tray App

1. Click "Start Screen Sync" in tray menu
2. Approve GUI dialog
3. Verify lights sync to screen
4. Click "Stop Screen Sync"
5. Click "Start Screen Sync" again (CRITICAL TEST)
6. Approve GUI dialog if shown
7. Verify lights sync to screen again

**Success**: If step 6-7 work without errors, the bug is fixed!

### Expected Results

#### Before Fix ❌
```
1. Start → ✅ Works
2. Stop → ✅ Works
3. Start → ❌ Error: "DBUS service not available"
4. Need systemctl --user restart hue-backend
```

#### After Fix ✅
```
1. Start → ✅ Works
2. Stop → ✅ Works
3. Start → ✅ Works (may show dialog again)
4. Stop → ✅ Works
5. Start → ✅ Works
... (indefinitely)
```

## Watching Backend Logs

In a separate terminal, monitor the backend:

```bash
journalctl --user -u hue-backend -f
```

Look for:
- ✅ "Entertainment Area activated"
- ✅ "Screen capture started: session=..."
- ✅ "Native PipeWire capture started"
- ✅ "Screen sync started at 30 FPS"

No errors like:
- ❌ "DBUS service not available"
- ❌ "dbus: connection closed by user"
- ❌ "failed to connect to session bus"

## Important Notes

### GUI Permission Dialog

The XDG Desktop Portal will show a GUI dialog asking:
- **"Which screen do you want to share?"**

This is **normal Wayland behavior** - you must approve it to allow screen capture. This dialog may appear:
- Always on first start
- Sometimes on subsequent starts (depends on system settings)

**This is not a bug** - it's a security feature. Just approve it.

### Multiple Cycles

Test at least **3 full start/stop cycles** to verify the fix:

```
Start → Stop → Start → Stop → Start
```

All should work without backend restart.

## Automated Test Script

You can also use the automated test script:

```bash
./test-screen-sync-restart.sh
```

Note: This requires you to manually approve GUI dialogs when they appear.

## Success Criteria

- ✅ Start/Stop/Start cycles work indefinitely
- ✅ No backend restart needed between cycles
- ✅ No "DBUS service not available" errors
- ✅ Lights sync to screen correctly on each start
- ✅ Backend logs show clean session creation
- ✅ All tests pass: `go test ./...`

## Files Changed

1. `backend/internal/capture/capture.go`:
   - Modified `Stop()`: Reset portal state variables
   - Modified `Start()`: Recreate DBus connection and context if needed

## Related

- PR: #14
- Issue: Screen Sync restart failure
- Branch: `fix/screen-sync-restart-issue`
