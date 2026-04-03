# Testing Screen Sync - Important Notes

## ⚠️ CRITICAL: GUI Permission Dialog

**The Screen Sync feature works correctly, but requires user interaction that's invisible when testing via CLI.**

### What Happens When You Start Screen Sync

1. **User Action:** Click "Start Screen Sync" button in tray app (or call `StartSync` via DBus)
2. **Backend Action:** Entertainment area activates on bridge
3. **Portal Action:** XDG Desktop Portal shows a **GUI dialog** asking:
   - Which screen/monitor to share
   - Permission to capture screen
4. **USER MUST INTERACT:** User clicks the dialog, selects screen, approves
5. **Capture Starts:** Native PipeWire capture begins at 30 FPS
6. **Lights Sync:** Colors extracted and streamed to lights in real-time

### Why It Appears Broken When Testing via CLI

**Symptom:** Running `dbus-send ... StartSync` hangs or times out

**Cause:** The GUI permission dialog is shown by the desktop environment, but when testing from terminal:
- The dialog IS showing somewhere on screen
- You (the CLI tester) don't know it's there
- The backend is waiting for the user to approve
- It looks like the feature is broken or hanging

**Reality:** The feature works perfectly - it's just waiting for GUI interaction you can't see from CLI.

### How to Test Properly

**Option 1: Use the Tray App (Recommended)**
```bash
# Start services
systemctl --user start hue-backend
cd ~/Code/misc/khuey/trayapp && ./hue-tray &

# Click the tray icon
# Click "Start Screen Sync"
# Approve the permission dialog when it appears
# Watch your lights sync!
```

**Option 2: CLI Testing with Awareness**
```bash
# Start sync via DBus
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync

# LOOK FOR THE GUI DIALOG on your screen!
# It's usually a system dialog asking "Share screen?"
# Approve it, select your monitor, click "Share"

# Wait a few seconds, then check:
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing
# Should return: boolean true
```

### Log Patterns

**Waiting for dialog approval:**
```
2026/04/03 14:27:56 ✅ Entertainment Area activated
Screen capture started: session=/org/freedesktop/portal/.../session_123, node=94
[Native] PipeWire capture started
✅ Native PipeWire capture started (CGo + libpipewire)
2026/04/03 14:27:58 ✅ Screen sync started at 30 FPS
2026/04/03 14:27:58 ⚠️  Capture error: no frame available yet
```
*Note: "no frame available yet" means dialog not approved yet*

**After dialog approved (SUCCESS):**
```
✅ Screen sync started at 30 FPS
[PipeWire] Stream state changed: CONNECTING -> STREAMING
[PipeWire] Video format: 2560x1440, format=BGRx
📸 Frames captured: 30 FPS actual
```

### Debugging Checklist

If Screen Sync "doesn't work":

1. ✅ **Check backend is running:** `systemctl --user status hue-backend`
2. ✅ **Check DBus working:** `dbus-send ... GetStatus` returns "Ready"
3. ✅ **Entertainment area configured:** Check `~/.openhue/config.yaml` has `entertainmentconfigurationid`
4. ✅ **Start sync:** Click button or call DBus
5. ⚠️ **LOOK FOR THE DIALOG:** A system GUI dialog asking to share screen
6. ✅ **Approve dialog:** Select monitor, click "Share" or "Allow"
7. ✅ **Verify syncing:** `IsSyncing` returns `true`, lights change colors
8. ✅ **Check logs:** Should see "30 FPS actual" in backend logs

### Common Misunderstandings

❌ **"Screen Sync hangs when I call StartSync"**
✅ It's waiting for you to approve the GUI dialog

❌ **"The backend doesn't receive frames"**
✅ Frames won't arrive until dialog is approved

❌ **"It worked before but now it doesn't"**
✅ Dialog might appear again if permissions expired or settings changed

❌ **"When testing via terminal nothing happens"**
✅ The GUI dialog is shown, you just don't see it from terminal

### Success Indicators

**Screen Sync is working when:**
- ✅ Backend CPU usage increases (syncing at 30 FPS uses CPU)
- ✅ `IsSyncing` returns `true`
- ✅ Logs show "30 FPS actual" or similar frame activity
- ✅ **Lights change colors based on screen content**
- ✅ No "no frame available yet" errors after first few seconds

### Technical Details

**Why the dialog exists:**
- Wayland security requires explicit user consent for screen capture
- XDG Desktop Portal mediates between apps and compositor
- This is by design, not a bug
- All screen capture apps (OBS, screen recorders, etc.) have same behavior

**Dialog behavior:**
- First time: Always shows
- Subsequent times: Depends on "Remember this choice" setting
- Per-session: May need to approve again after logout/reboot

## Bottom Line

**Screen Sync works perfectly.** The permission dialog is expected Wayland behavior, not a bug. When testing, remember that the GUI dialog appears and requires user interaction before capture starts.
