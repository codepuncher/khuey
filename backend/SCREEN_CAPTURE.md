# Real Screen Capture Implementation

## Current Status

The Hue widget currently uses **mock gradient frames** for testing the Entertainment API pipeline. This works perfectly and demonstrates the full sync functionality with:

- ✅ 30 FPS streaming to Entertainment API
- ✅ Zero DTLS errors
- ✅ Color extraction per zone
- ✅ Lights syncing to colors

## Why Mock Frames?

Real screen capture from Wayland/Pipewire in Go is challenging because:

1. **XDG Desktop Portal sessions are temporary**
   - Portal creates a Pipewire stream node
   - Node expires after a timeout (no keep-alive)
   - Need to maintain active connection

2. **No mature Go libraries**
   - `gopipewire`: Requires CGo + libpipewire-0.3-dev
   - `pwnative`: Too new and experimental
   - Gstreamer: Element availability issues

3. **CGo complexity**
   - Would add build requirements
   - Cross-compilation becomes harder
   - Maintenance burden

## Implementation Options

### Option A: CGo + libpipewire (Recommended for production)

**Install dependencies:**
```bash
sudo apt install libpipewire-0.3-dev pkg-config
go get github.com/ik5/gopipewire
```

**Implement in `capture.go`:**
```go
// #cgo pkg-config: libpipewire-0.3
// #include <pipewire/pipewire.h>
// #include <spa/param/video/format.h>
import "C"

func (sc *ScreenCapture) connectPipewire() error {
    C.pw_init(nil, nil)
    
    // Create main loop
    loop := C.pw_main_loop_new(nil)
    
    // Create pw_stream connected to sc.streamNode
    stream := C.pw_stream_new_simple(
        C.pw_main_loop_get_loop(loop),
        C.CString("hue-screen-sync"),
        props,
        &callbacks,
    )
    
    // Connect to node
    C.pw_stream_connect(stream, ...)
    
    return nil
}

// Register callback for frame data
func onProcessFrame(data unsafe.Pointer) {
    buf := C.pw_stream_dequeue_buffer(stream)
    // Extract spa_buffer video data
    // Convert to image.RGBA
    // Update frameBuffer
}
```

### Option B: Keep Portal Session Alive

The portal session node expires. To keep it alive:

1. Poll the session every N seconds
2. Maintain active Pipewire connection
3. Handle reconnection on timeout

**Modify `portal.go`:**
```go
func (sc *ScreenCapture) keepSessionAlive() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        // Ping portal session to keep node active
        // Re-establish connection if needed
    }
}
```

### Option C: Use Wayland Screenshot + Timer

Simple but inefficient approach:

```go
func (sc *ScreenCapture) CaptureFrame() (*image.RGBA, error) {
    // Use grim (Wayland screenshot tool)
    cmd := exec.Command("grim", "-t", "ppm", "-")
    output, err := cmd.Output()
    // Parse PPM format
    return parseIntoRGBA(output), nil
}
```

**Pros:** No CGo, works immediately  
**Cons:** 30 FPS = 30 screenshots/sec (inefficient)

### Option D: FFmpeg Pipewire Recording

Use FFmpeg to continuously record screen:

```bash
ffmpeg -f pipewire -i <node-id> -f rawvideo -pix_fmt rgba -
```

Read raw RGBA frames from stdout.

**Pros:** No CGo  
**Cons:** FFmpeg overhead, buffer management

## Recommended Path Forward

1. **Short term**: Keep mock gradient frames (they work!)
2. **Medium term**: Implement Option C (screenshot approach) for real content
3. **Long term**: Implement Option A (CGo + libpipewire) for production quality

## Testing Real Capture

When implementing real capture, test with:

```bash
cd backend
go build ./cmd/test-capture

# Should show real desktop colors, not gradient
./test-capture
```

## References

- [Pipewire Capture Example (OBS Studio)](https://github.com/obsproject/obs-studio/blob/master/plugins/linux-pipewire/pipewire.c)
- [OpenJDK Screencast](https://github.com/openjdk/jdk/blob/master/src/java.desktop/unix/native/libawt_xawt/awt/screencast_pipewire.c)
- [gopipewire Examples](https://github.com/ik5/gopipewire/tree/main/examples)
