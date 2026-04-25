# Screen Sync Quick Start Guide

## Overview
Screen Sync synchronizes your Philips Hue lights to your screen content in real-time using the Entertainment API.

## Prerequisites
1. ✅ Hue Bridge on local network
2. ✅ Entertainment Area created in Hue app
3. ✅ Backend running (`hue-sync`)
4. ✅ Config file at `~/.openhue/config.yaml`

## Quick Test

### Start Sync
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StartSync
```

### Check Status
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.IsSyncing
```

### Stop Sync
```bash
dbus-send --session --print-reply \
  --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue \
  org.kde.plasma.hue.StopSync
```

## Configuration

Edit `~/.openhue/config.yaml`:

### Basic Sync Settings

```yaml
sync:
  enabled: false        # Initial state
  fps: 30              # Target frame rate (10-60)
  subsamplewidth: 64   # Downsample width for processing
  monitor: ""          # Monitor to capture (empty = all)
```

### Zone-Based Screen Mapping

Map each light to specific screen regions using UV coordinates (0.0-1.0):

#### Three Lights (Left/Center/Right)
```yaml
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}    # Top-left corner
    uvB: {x: 0.33, y: 1.0}   # Bottom-right corner (left third)
  - id: 1
    active: true
    deviceName: "Center Light"
    gammaFactor: 2.2
    uvA: {x: 0.33, y: 0.0}
    uvB: {x: 0.67, y: 1.0}   # Center third
  - id: 2
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA: {x: 0.67, y: 0.0}
    uvB: {x: 1.0, y: 1.0}    # Right third
```

#### Two Lights (Left/Right)
```yaml
channels:
  - id: 0
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.5, y: 1.0}    # Left half
  - id: 1
    uvA: {x: 0.5, y: 0.0}
    uvB: {x: 1.0, y: 1.0}    # Right half
```

#### TV Ambilight (4 Lights - Edges Only)
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

**Note:** If UV coordinates are not configured, the system automatically splits the screen evenly based on the number of active lights.

## How It Works

1. **Screen Capture** - Captures screen at 30 FPS using native PipeWire
2. **Color Extraction** - Extracts average color from each zone
3. **Zone Mapping** - Maps screen regions to lights (UV coordinates)
4. **Streaming** - Sends colors via Entertainment API (DTLS)

### Zone Layout (3 lights)
```
┌─────────────┬─────────────┬─────────────┐
│   Light 0   │   Light 1   │   Light 2   │
│   (Left)    │  (Center)   │   (Right)   │
│   0.0-0.33  │  0.33-0.67  │  0.67-1.0   │
└─────────────┴─────────────┴─────────────┘
```

## Troubleshooting

### "Sync engine not available"
- Check `entertainmentconfigurationid` in config.yaml
- Verify Entertainment Area exists in Hue app

### "Connection refused"
- Entertainment Area must be activated (auto-activated by StartSync)
- Ensure bridge is reachable on port 2100 (UDP)

### Lights not changing
- Check channel count matches Entertainment Area
- Verify `active: true` for all channels in config
- Test with high-contrast content (red/green/blue)

### Low performance
- Reduce `fps` in config (try 15 or 20)
- Increase `subsamplewidth` (try 32 instead of 64)

## Testing

### Visual Test
1. Start sync
2. Open colorful image or website
3. Move windows around
4. Lights should match screen colors immediately

### Performance Test
```bash
# Monitor backend logs
journalctl --user -u plasma-hue-backend -f
```

## Technical Details

- **Capture:** Native PipeWire (CGo bindings)
- **Protocol:** Entertainment API v2 with DTLS 1.2
- **Color Space:** 16-bit RGB per channel
- **Latency:** <100ms typical
- **Performance:** Sustained 30 FPS at 2560x1440

## See Also

- Full test report: `SCREEN_SYNC_TEST_REPORT.md`
- Config documentation: `README.md`
- Backend implementation: `internal/sync/engine.go`
