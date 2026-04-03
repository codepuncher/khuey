# Zone Mapping Quick Reference Card

## UV Coordinate System

```
Screen coordinates: 0.0 to 1.0 (normalized)

     U (x-axis)
     0.0    0.5    1.0
      ├──────┼──────┤
 0.0  ┌──────┬──────┐
      │      │      │
V     │  A   │   B  │
      │      │      │
(y)   ├──────┼──────┤
      │      │      │
      │  C   │   D  │
 1.0  └──────┴──────┘

Zone A: uvA: {x: 0.0, y: 0.0}, uvB: {x: 0.5, y: 0.5}
Zone B: uvA: {x: 0.5, y: 0.0}, uvB: {x: 1.0, y: 0.5}
Zone C: uvA: {x: 0.0, y: 0.5}, uvB: {x: 0.5, y: 1.0}
Zone D: uvA: {x: 0.5, y: 0.5}, uvB: {x: 1.0, y: 1.0}
```

## Common Layouts

### 2 Lights (Left/Right)

```yaml
channels:
  - id: 0  # Left
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}
  - id: 1  # Right
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

### 3 Lights (Left/Center/Right) - YOUR CURRENT SETUP

```yaml
channels:
  - id: 0  # Left
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.33, y: 1.0}
  - id: 1  # Center
    active: true
    deviceName: "Center Light"
    gammaFactor: 2.2
    uvA: {x: 0.33, y: 0.0}
    uvB: {x: 0.67, y: 1.0}
  - id: 2  # Right
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA: {x: 0.67, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

### 4 Lights (Quadrants)

```yaml
channels:
  - id: 0  # Top-Left
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 0.5}
  - id: 1  # Top-Right
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 0.5}
  - id: 2  # Bottom-Left
    uvA: {x: 0.0, y: 0.5}
    uvB: {x: 0.5, y: 1.0}
  - id: 3  # Bottom-Right
    uvA: {x: 0.5, y: 0.5}
    uvB: {x: 1.0, y: 1.0}
```

### 4 Lights (TV Edges)

```yaml
channels:
  - id: 0  # Left edge
    uvA: {x: 0.0, y: 0.25}
    uvB: {x: 0.1, y: 0.75}
  - id: 1  # Top edge
    uvA: {x: 0.25, y: 0.0}
    uvB: {x: 0.75, y: 0.1}
  - id: 2  # Right edge
    uvA: {x: 0.9, y: 0.25}
    uvB: {x: 1.0, y: 0.75}
  - id: 3  # Bottom edge
    uvA: {x: 0.25, y: 0.9}
    uvB: {x: 0.75, y: 1.0}
```

## Commands

### View Current Zone Configuration

```bash
journalctl --user -u hue-backend --since "1 minute ago" | grep "📍 Zone"
```

### Test Zone Mapping

```bash
# Run automated visual test
/tmp/test-zone-colors.sh
```

### Restart Backend After Config Changes

```bash
systemctl --user restart hue-backend
```

### Manual Screen Sync Control

```bash
# Start
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync

# Check status
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing

# Stop
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StopSync
```

## Validation Rules

UV coordinates must:
- ✅ Be in range [0.0, 1.0]
- ✅ Have uvB.x > uvA.x (non-zero width)
- ✅ Have uvB.y > uvA.y (non-zero height)

Invalid coordinates automatically fall back to auto-split.

## Troubleshooting

### Lights not responding to zones

1. Check zone configuration in logs:
   ```bash
   journalctl --user -u hue-backend | grep "📍 Zone"
   ```

2. Verify config syntax:
   ```bash
   cat ~/.openhue/config.yaml | grep -A 4 "channels:"
   ```

3. Restart backend:
   ```bash
   systemctl --user restart hue-backend
   ```

### All lights show same color

This means zones are not configured or invalid. Check for "Auto-split" in logs:
```bash
journalctl --user -u hue-backend | grep "Auto-split"
```

If you see auto-split, your UV coordinates are either:
- Not configured
- Invalid (out of bounds or zero area)

### Want to go back to default behavior

Remove UV coordinates from config or set them all to zero:
```yaml
channels:
  - id: 0
    active: true
    # No uvA/uvB = auto-split
```

## Files

- **Config:** `~/.openhue/config.yaml`
- **Backend logs:** `journalctl --user -u hue-backend -f`
- **Test script:** `/tmp/test-zone-colors.sh`

## Documentation

- `README.md` - Overview and examples
- `SCREEN_SYNC_QUICKSTART.md` - Configuration guide
- `ZONE_MAPPING_FEATURE.md` - Complete feature docs
- `ZONE_MAPPING_IMPLEMENTATION.md` - Implementation details
