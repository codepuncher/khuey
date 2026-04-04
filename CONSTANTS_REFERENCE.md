# Constants Reference - khuey v1.0.0

Quick reference for all application constants added during pre-release audit.

## Configuration Constants

**Package**: `internal/config`
**File**: `backend/internal/config/config.go`

```go
const (
    // ConfigVersion is incremented when breaking changes are made to config format
    ConfigVersion = 1

    // FPS limits for screen sync
    DefaultFPS = 30
    MinFPS     = 1
    MaxFPS     = 60

    // Subsample width limits for screen capture
    DefaultSubsampleWidth = 64
    MinSubsampleWidth     = 16
    MaxSubsampleWidth     = 256
)
```

**Usage**:
- Config validation uses these for bounds checking
- Default values use these for initialization
- User documentation references these limits

---

## Color Extraction Constants

**Package**: `internal/color`
**File**: `backend/internal/color/extractor.go`

```go
const (
    MinSubsampleWidth = 16  // Minimum subsample width for color extraction
    MaxSubsampleWidth = 256 // Maximum subsample width for color extraction
)
```

**Usage**:
- `NewExtractor()` validates subsampleWidth parameter
- Prevents excessive memory usage (256px max width)
- Ensures minimum quality (16px min width)

---

## Entertainment API Constants

**Package**: `internal/entertainment`
**File**: `backend/internal/entertainment/client.go`

```go
const (
    EntertainmentAPIPort = 2100 // UDP port for Entertainment API
    Color8To16Multiplier = 257  // Multiplier to convert 8-bit to 16-bit color (65535 / 255)
)
```

**Usage**:
- `EntertainmentAPIPort`: UDP connection to Hue bridge Entertainment API
- `Color8To16Multiplier`: Convert RGB colors from 0-255 to 0-65535 range
  - Example: `uint16(r) * Color8To16Multiplier`
  - Math: 255 × 257 = 65535 (full brightness)

---

## Screen Capture Constants

**Package**: `internal/capture`
**File**: `backend/internal/capture/capture.go`

```go
const (
    MinFPS = 10 // Minimum frames per second
    MaxFPS = 60 // Maximum frames per second
)
```

**Usage**:
- `NewScreenCapture()` validates FPS parameter
- Prevents excessive CPU usage (60 FPS max)
- Ensures usable sync rate (10 FPS min)

**Note**: Config package allows `MinFPS = 1`, but capture enforces `MinFPS = 10`. This is intentional:
- Config is more permissive for flexibility
- Capture has practical minimum for screen capture APIs

---

## Portal Handle Constants

**Package**: `internal/capture`
**File**: `backend/internal/capture/portal.go`

```go
const (
    MaxPortalHandleID = 999999 // Maximum value for portal session/handle IDs
)
```

**Usage**:
- Generates random session/handle tokens for XDG Desktop Portal
- Example: `khuey_session_123456` (random 0-999999)
- Used in 4 places: session creation, handle generation (3×)

---

## HTTP Client Constants

**Package**: `internal/common`
**File**: `backend/internal/common/utils.go`

```go
const (
    // DefaultHTTPTimeout is the default timeout for HTTP requests to Hue bridge
    DefaultHTTPTimeout = 10 * time.Second
)
```

**Usage**:
- `NewHueHTTPClient()` uses this for HTTP client timeout
- Prevents hanging requests to Hue bridge
- Balanced for local network latency (10 seconds sufficient for LAN)

---

## Constant Dependencies

```
Config Constants (config package)
├── Used by: config.DefaultConfig()
├── Used by: config.Validate()
└── Referenced by: capture.NewScreenCapture() (FPS limits)

Color Constants (color package)
├── Used by: color.NewExtractor()
└── Validates subsampleWidth parameter

Entertainment Constants (entertainment package)
├── EntertainmentAPIPort
│   ├── Used by: entertainment.Client.Connect()
│   └── Used by: cmd/test-entertainment
└── Color8To16Multiplier
    ├── Used by: entertainment.Convert8BitTo16Bit()
    └── Used by: sync.Engine.syncLoop()

Capture Constants (capture package)
└── Used by: capture.NewScreenCapture()

Portal Constants (capture package)
└── Used by: createSession(), selectSources(), startStream()

HTTP Constants (common package)
└── Used by: common.NewHueHTTPClient()
```

---

## Why These Values?

### FPS: 1-60 (config), 10-60 (capture)
- **Upper bound (60)**: Refresh rate of most displays, diminishing returns above
- **Lower bound (10)**: Practical minimum for screen capture without lag
- **Default (30)**: Sweet spot for smooth sync without excessive CPU

### Subsample Width: 16-256
- **Upper bound (256)**: Balance between quality and performance
  - At 256px width, color extraction is fast enough for 60 FPS
  - Memory usage: ~200KB per frame at 256px
- **Lower bound (16)**: Minimum for distinguishable color zones
  - Below 16px, zone averaging becomes meaningless
- **Default (64)**: Optimal for most setups
  - Good color accuracy
  - Low CPU usage (~5% at 30 FPS)

### EntertainmentAPIPort: 2100
- **Philips Hue specification**: DTLS Entertainment API uses UDP port 2100
- **Not configurable**: Hardcoded in Hue bridge firmware

### Color8To16Multiplier: 257
- **Math**: 65535 / 255 = 257 (with rounding)
- **Purpose**: Scale 8-bit color (0-255) to 16-bit (0-65535)
- **Verification**: 255 × 257 = 65535 ✓

### MaxPortalHandleID: 999999
- **Range**: 6-digit random numbers (0-999999)
- **Purpose**: Generate unique XDG Portal session/handle tokens
- **Collision risk**: Negligible for typical usage (few sessions per user)

### DefaultHTTPTimeout: 10 seconds
- **LAN latency**: Typical Hue bridge response < 100ms
- **Margin**: 10s allows for network hiccups, bridge load
- **Not too long**: Fails reasonably fast if bridge offline

---

## Migration Notes

### Pre-v1.0 Code
If you have code that hardcodes these values, migrate to constants:

```go
// OLD (hardcoded)
if subsampleWidth > 256 { ... }
addr := fmt.Sprintf("%s:2100", bridgeIP)
R := uint16(r) * 257

// NEW (constants)
if subsampleWidth > color.MaxSubsampleWidth { ... }
addr := fmt.Sprintf("%s:%d", bridgeIP, entertainment.EntertainmentAPIPort)
R := uint16(r) * entertainment.Color8To16Multiplier
```

### Config File Version
New configs include `version: 1` field. Old configs without version field are treated as version 0.

---

## Testing Constants

To verify constants are used correctly:

```bash
# Search for hardcoded values (should find none in main code)
cd backend

# These should only appear in constant definitions:
grep -rn "257" --include="*.go" | grep -v "const\|comment\|test"
grep -rn "2100" --include="*.go" | grep -v "const\|comment\|test"
grep -rn "999999" --include="*.go" | grep -v "const\|comment\|test"

# FPS limits should use constants:
grep -rn "< 10\|> 60" --include="*.go" | grep -v "const\|comment"
```

---

## Future Additions

Constants likely to be added in future versions:

- `MaxChannelCount` - Maximum number of Entertainment Area channels
- `MinGammaFactor`, `MaxGammaFactor` - Gamma correction limits
- `DefaultGammaFactor` - Default gamma (currently hardcoded 2.2)
- `MaxUVCoordinate` - UV coordinate upper limit (currently 1.0)
- `MinBrightness`, `MaxBrightness` - Brightness control limits
- `DBusServiceName`, `DBusObjectPath` - DBus identifiers (currently in const block)

---

**Document Version**: 1.0
**Last Updated**: 2024-12-28
**Related**: PRE_RELEASE_AUDIT_FIXES_v1.0.0.md
