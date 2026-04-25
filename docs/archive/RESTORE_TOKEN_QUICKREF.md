# Restore Token Quick Reference

## What Is It?

The restore token feature eliminates the screen share permission dialog on subsequent syncs by persisting your permission choice.

## How It Works

### First Sync (One-Time Setup)
```
User: Starts screen sync
System: Shows "Select screen to share" dialog
User: Selects monitor → Clicks "Share"
Backend: Saves permission token
Log: "✅ Screen share permission saved (no dialog next time)"
```

### All Subsequent Syncs
```
User: Starts screen sync
Backend: Uses saved token
System: NO DIALOG! Instant start ✨
```

## Configuration

**File**: `~/.openhue/config.yaml`

**After first sync**:
```yaml
sync:
  enabled: false
  fps: 30
  subsampleWidth: 64
  monitor: ""
  restoreToken: "portal_generated_token_abc123"  # Automatically saved
```

## Testing Commands

### Test First Sync (With Dialog)
```bash
# Start backend
systemctl --user restart hue-backend

# Start sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync

# Approve dialog when it appears
# Check logs for success message
journalctl --user -u hue-backend -n 20 | grep "permission saved"

# Verify token was saved
cat ~/.openhue/config.yaml | grep restoreToken
```

### Test Second Sync (No Dialog)
```bash
# Stop sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StopSync

# Start sync again
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync

# Should start immediately without dialog!
```

### Test Invalid Token Fallback
```bash
# Edit config and set invalid token
vim ~/.openhue/config.yaml
# Change: restoreToken: "invalid_test_token"

# Restart backend
systemctl --user restart hue-backend

# Start sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync

# Dialog appears (fallback)
# Approve → New valid token saved
```

## Log Messages

### Success
```
✅ Screen share permission saved (no dialog next time)
```

### Error
```
⚠️  Failed to save restore token: <error message>
```
(Sync still works, just shows dialog next time)

### Debug
```
Screen capture started: session=/org/..., node=94
```

## Troubleshooting

### Dialog Still Appears Every Time

**Check token in config**:
```bash
cat ~/.openhue/config.yaml | grep restoreToken
```

If empty or missing:
- Check backend logs for save errors
- Verify config file is writable (should be 0600)
- Check disk space

### Token Not Being Saved

**Check permissions**:
```bash
ls -la ~/.openhue/config.yaml
# Should show: -rw------- (0600)
```

**Check backend logs**:
```bash
journalctl --user -u hue-backend -f
# Look for "Failed to save restore token" messages
```

### Portal Version Too Old

**Check portal version**:
```bash
# Restore token requires XDG Desktop Portal v4+ (2021+)
# Most modern systems have this
```

If version is too old:
- Feature gracefully degrades
- Dialog will appear every time (normal behavior)
- No errors or failures

## Revoking Permission

If you want to revoke the saved permission:

### Method 1: Delete Token
```bash
# Edit config
vim ~/.openhue/config.yaml

# Remove or empty the restoreToken line:
restoreToken: ""

# Or delete the line entirely
```

### Method 2: System Settings
```bash
# KDE: System Settings → Applications → Screen Sharing
# Look for "khuey" and revoke permission
```

## Gaming Mode Integration

With restore token, Gaming Mode auto-sync works seamlessly:

**Before**: Game detected → Dialog appears → User must click → Interruption!
**After**: Game detected → Sync starts instantly → No interruption! ✨

**Enable in config**:
```yaml
gamingMode:
  enabled: true
  # ... other settings
```

## Technical Details

- **Portal API**: XDG Desktop Portal ScreenCast v4+
- **Persistence**: `persist_mode: 2` (until explicitly revoked)
- **Security**: Single-use token rotation per session
- **Storage**: User-only readable file (0600 permissions)
- **Compatibility**: Graceful fallback on older systems

## Related Documentation

- **Full Implementation**: `RESTORE_TOKEN_IMPLEMENTATION.md`
- **Gaming Mode**: `GAMING_MODE_IMPLEMENTATION.md`
- **Testing Guide**: `TESTING.md`
- **Usage Guide**: `USAGE.md`

## Quick Summary

| Scenario | Behavior |
|----------|----------|
| First sync | Dialog → Approve → Token saved |
| Subsequent syncs | NO DIALOG! Instant start ✨ |
| Invalid token | Dialog → Approve → New token saved |
| Gaming Mode | Auto-sync without interruption 🎮 |
| Revoked permission | Dialog → Approve → Token saved again |

---

**Result**: One-time permission setup, then seamless forever!
