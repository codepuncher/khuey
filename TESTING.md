# Testing Guide

## Testing Without Installation

### Backend Components

You can test the backend components without having a full Hue setup:

```bash
cd backend

# Test configuration loading
go run test/main.go -config

# If you have a Hue bridge configured
go run test/main.go -hue -bridge 192.168.1.100 -key YOUR_API_KEY
```

### QML UI Components

The plasmoid UI can be tested individually with `qmlscene`:

```bash
# Test individual QML files (will show errors about missing Plasmoid context - this is expected)
qmlscene plasmoid/contents/ui/CompactRepresentation.qml
qmlscene plasmoid/contents/ui/FullRepresentation.qml
```

## Installing the Plasmoid for Testing

To test the full widget in your Plasma desktop:

```bash
# Option 1: Build with CMake (recommended)
mkdir build && cd build
cmake ..
sudo make install
kquitapp6 plasmashell && plasmashell &

# Option 2: Manual package installation
kpackagetool6 --install plasmoid --type Plasma/Applet
# Or to update:
kpackagetool6 --upgrade plasmoid --type Plasma/Applet

# To remove:
kpackagetool6 --remove org.kde.plasma.hue --type Plasma/Applet
```

After installation:
1. Right-click on your system tray
2. Click "Configure System Tray..."
3. Click "Add Widgets..."
4. Search for "Hue Control"
5. Add it to your system tray

## Current Status

**What Works:**
- ✅ Backend configuration loading/saving
- ✅ Config compatibility with openhue-cli
- ✅ Hue client initialization
- ✅ Basic Hue API operations (scenes, power, brightness)
- ✅ Plasmoid UI (visual only, no backend connection yet)

**What's Missing (To Be Implemented):**
- ⏳ DBus service (for plasmoid ↔ backend communication)
- ⏳ Wayland screen capture
- ⏳ DTLS Entertainment API streaming
- ⏳ Screen color analysis and zone mapping
- ⏳ Setup wizard for first-time configuration

## Testing with a Real Hue Bridge

If you have a Philips Hue bridge, you can test basic functionality:

1. **Get your bridge IP and create an API key:**
   ```bash
   # If you have openhue-cli installed:
   openhue setup
   
   # Or manually:
   # 1. Find bridge IP on your network
   # 2. Press link button on bridge
   # 3. Send POST to https://BRIDGE_IP/api with {"devicetype":"test#user"}
   ```

2. **Create config file:**
   ```bash
   mkdir -p ~/.openhue
   cat > ~/.openhue/config.yaml << EOF
   Bridge: "192.168.1.100"  # Your bridge IP
   Key: "YOUR-API-KEY-HERE"
   sync:
     enabled: false
     fps: 30
     subsampleWidth: 64
   EOF
   ```

3. **Test the connection:**
   ```bash
   cd backend
   go run test/main.go -hue
   ```

## Development Workflow

```bash
# 1. Make changes to backend code
vim backend/internal/hue/client.go

# 2. Test it
cd backend
go run test/main.go -config

# 3. Build main binary
go build -o hue-sync ./cmd/hue-sync
./hue-sync

# 4. For UI changes, reinstall plasmoid
kpackagetool6 --upgrade plasmoid --type Plasma/Applet
kquitapp6 plasmashell && plasmashell &
```

## Troubleshooting

**"Bridge unreachable"**
- Check your bridge IP is correct
- Ensure you're on the same network
- Try pinging the bridge: `ping BRIDGE_IP`

**"Status 401 Unauthorized"**
- Your API key is invalid or expired
- Run setup again to create a new key

**Plasmoid doesn't appear**
- Check it's installed: `kpackagetool6 --list | grep hue`
- Check for QML errors: `journalctl -f | grep plasmashell`

**QML syntax errors**
- Check individual files: `qmlscene plasmoid/contents/ui/main.qml`
