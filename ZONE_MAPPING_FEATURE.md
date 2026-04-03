# Zone-Based Screen Mapping Feature

## Overview

Enables true Ambilight-style effects where each Hue light mirrors a specific region of your screen, creating dynamic color gradients that follow on-screen content.

## What Changed

### Before
- All lights showed the **same averaged screen color**
- Hardcoded zone splitting in engine.go
- No user control over zone layout

### After
- Each light shows **its own screen region's color**
- User-configurable UV coordinates per light
- Flexible zone layouts (edges, corners, custom regions)
- Backward compatible: auto-split when UV not configured

## Implementation Details

### Config Structure (Already Existed)

The UV coordinate fields were already defined in `config.go`:

```go
type ChannelConfig struct {
    ID          uint8   `mapstructure:"id"`
    Active      bool    `mapstructure:"active"`
    DeviceName  string  `mapstructure:"deviceName"`
    GammaFactor float32 `mapstructure:"gammaFactor"`
    UVA         UV      `mapstructure:"uvA"` // Top-left corner
    UVB         UV      `mapstructure:"uvB"` // Bottom-right corner
}

type UV struct {
    X float32 `mapstructure:"x"`
    Y float32 `mapstructure:"y"`
}
```

### Engine Changes

**File:** `backend/internal/sync/engine.go`

Replaced hardcoded zone creation (lines 62-89) with:

1. **`createZonesFromConfig(cfg)`** - Reads UV coordinates from config
   - Checks if UV coordinates are configured
   - Validates coordinates (0.0-1.0 range, valid area)
   - Falls back to auto-split if invalid/missing
   - Logs zone configuration for debugging

2. **`validateUVCoordinates(uvA, uvB)`** - Ensures valid zones
   - Bounds checking: all values in [0.0, 1.0]
   - Area validation: uvB > uvA (non-zero area)
   - Clear error messages for debugging

3. **`createDefaultZone(index, total)`** - Auto-split fallback
   - Preserves original behavior when UV not configured
   - Backward compatible with existing configs

### Test Coverage

**File:** `backend/internal/sync/engine_test.go`

Added comprehensive tests:

- `TestValidateUVCoordinates` - 8 test cases
  - Valid coordinates (full screen, partial regions)
  - Out of bounds (negative, >1.0)
  - Invalid areas (zero width, inverted)

- `TestCreateZonesFromConfig` - 4 test cases
  - With UV coordinates (uses config values)
  - Without UV coordinates (auto-split)
  - Inactive channels (filtered out)
  - Invalid UV fallback (uses auto-split)

- `TestCreateDefaultZone` - 5 test cases
  - 1-4 channel auto-split layouts
  - Validates correct zone boundaries

All tests passing ✅

## UV Coordinate System

Screen coordinates use normalized UV values (0.0-1.0):

```
     0.0                    0.5                    1.0
      ├──────────────────────┼──────────────────────┤
 0.0  ┌──────────────────────┬──────────────────────┐
      │                      │                      │
      │    Left Half         │    Right Half        │
      │    uvA: (0.0, 0.0)   │    uvA: (0.5, 0.0)   │
 0.5  │    uvB: (0.5, 1.0)   │    uvB: (1.0, 1.0)   │
      │                      │                      │
      │                      │                      │
 1.0  └──────────────────────┴──────────────────────┘
```

## Configuration Examples

### Example 1: Three Lights (User's Setup)

```yaml
channels:
  - id: 0
    active: true
    deviceName: "Left Light"
    gammaFactor: 2.2
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.33, y: 1.0}
  - id: 1
    active: true
    deviceName: "Center Light"
    gammaFactor: 2.2
    uvA: {x: 0.33, y: 0.0}
    uvB: {x: 0.67, y: 1.0}
  - id: 2
    active: true
    deviceName: "Right Light"
    gammaFactor: 2.2
    uvA: {x: 0.67, y: 0.0}
    uvB: {x: 1.0, y: 1.0}
```

### Example 2: TV Ambilight (4 Edge Lights)

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

### Example 3: Four Corners

```yaml
channels:
  - id: 0  # Top-left
    uvA: {x: 0.0, y: 0.0}
    uvB: {x: 0.2, y: 0.2}
  - id: 1  # Top-right
    uvA: {x: 0.8, y: 0.0}
    uvB: {x: 1.0, y: 0.2}
  - id: 2  # Bottom-left
    uvA: {x: 0.0, y: 0.8}
    uvB: {x: 0.2, y: 1.0}
  - id: 3  # Bottom-right
    uvA: {x: 0.8, y: 0.8}
    uvB: {x: 1.0, y: 1.0}
```

## Verification

### Backend Logs

When starting the backend with UV coordinates configured:

```
2026/04/03 15:52:29 📍 Zone 0 (Left Light): UV [0.00,0.00] to [0.33,1.00]
2026/04/03 15:52:29 📍 Zone 1 (Center Light): UV [0.33,0.00] to [0.67,1.00]
2026/04/03 15:52:29 📍 Zone 2 (Right Light): UV [0.67,0.00] to [1.00,1.00]
2026/04/03 15:52:29 ✅ Sync engine initialized
```

### Visual Test

To verify the Ambilight effect:

1. Display a test pattern with distinct colors:
   - Left third: Red
   - Center third: Green
   - Right third: Blue

2. Start Screen Sync

3. Observe:
   - Light 0 (left) → Red
   - Light 1 (center) → Green
   - Light 2 (right) → Blue

## Backward Compatibility

**Old configs (no UV coordinates):**
```yaml
channels:
  - id: 0
    active: true
  - id: 1
    active: true
```

**Behavior:** Auto-splits screen evenly (same as before)

**Log output:**
```
📍 Zone 0 (): Auto-split [0.00,0.00] to [0.50,1.00]
📍 Zone 1 (): Auto-split [0.50,0.00] to [1.00,1.00]
```

## Benefits

1. **True Ambilight Effect** - Each light responds to its screen region
2. **Flexible Layouts** - TV edges, corners, custom zones
3. **Monitor-Agnostic** - UV coordinates scale to any resolution
4. **Gaming/Movies** - Dynamic lighting follows action on screen
5. **Easy Configuration** - Simple YAML syntax
6. **Validation** - Clear errors for invalid coordinates
7. **Backward Compatible** - Existing configs work unchanged

## Performance

- **Zero overhead** - Zones created once at startup
- **No runtime impact** - Same color extraction performance
- **Validated upfront** - No per-frame validation needed

## Future Enhancements

Potential improvements:

- **GUI configurator** - Visual zone editor
- **Presets** - Common layouts (TV, monitor, desk setup)
- **Per-channel gamma** - Already supported in config!
- **Zone overlap** - Multiple lights for smooth blending
- **Circular zones** - For curved monitors/TVs

## Testing Commands

```bash
# Build and test
cd backend
go test ./internal/sync -v -run "TestValidateUV|TestCreateZones|TestCreateDefault"
go build -o hue-sync ./cmd/hue-sync

# Restart backend
systemctl --user restart hue-backend

# Check zone initialization
journalctl --user -u hue-backend --since "10 seconds ago" | grep "📍 Zone"

# Test screen sync
dbus-send --session --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.StartSync
```

## Files Modified

1. `backend/internal/sync/engine.go` - Zone creation from config
2. `backend/internal/sync/engine_test.go` - Comprehensive tests
3. `README.md` - Zone mapping documentation
4. `SCREEN_SYNC_QUICKSTART.md` - Configuration examples
5. `~/.openhue/config.yaml` - User's config with UV coordinates

## Git History

```
7db73bd feat: Wire config UV coordinates to zone creation
6b1fe05 docs: Add zone-based screen mapping documentation
```

## Success Metrics

✅ Config accepts uvA/uvB fields
✅ Engine creates zones from config UV coordinates
✅ Fallback to auto-split if UV not configured
✅ Invalid UV coordinates are rejected with clear errors
✅ All tests passing (8 validation + 4 config + 5 auto-split)
✅ Backend logs show zone configuration at startup
✅ Documentation with examples (2/3/4 lights)
✅ Backward compatible with existing configs

## Feature Complete ✅

The zone-based screen mapping feature is **production-ready** and enables true Ambilight-style effects with configurable screen regions per light.
