# Testing Guide

## Testing Without Installation

### Backend Components

You can test the backend components without having a full Hue setup:

```bash
cd backend

# Run backend tests
go test ./...

# Run specific package tests
go test ./internal/config -v
go test ./internal/hue -v
go test ./internal/dbus -v

# Build backend
go build -o hue-sync ./cmd/hue-sync
```

### Tray Application

To test the tray app:

```bash
cd trayapp

# Build tray app
cmake . && make

# Run tray app manually (backend must be running)
./hue-tray
```

## Testing the Full Application

To test with both backend and tray app:

```bash
# Terminal 1: Start backend
cd backend
./hue-sync

# Terminal 2: Start tray app
cd trayapp
./hue-tray
```

The tray icon should appear in your system tray and connect to the backend via DBus.

## Current Status

**What Works:**
- ✅ Backend configuration loading/saving
- ✅ Config compatibility with openhue-cli
- ✅ Hue client initialization
- ✅ Basic Hue API operations (scenes, power, brightness)
- ✅ Tray app UI with system tray integration
- ✅ DBus communication (tray app ↔ backend)
- ✅ Scene control, power, and brightness controls

**What's In Development:**
- ⏳ Wayland screen capture
- ⏳ DTLS Entertainment API streaming
- ⏳ Screen color analysis and zone mapping
- ⏳ Settings dialog for configuration

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
go test ./internal/hue

# 3. Build and restart backend
go build -o hue-sync ./cmd/hue-sync
systemctl --user restart hue-backend

# 4. For tray app changes, rebuild and restart
cd trayapp
cmake . && make
killall hue-tray && ./hue-tray &
```

## Troubleshooting

**"Backend not running"**
- Check backend is running: `systemctl --user status hue-backend`
- Check logs: `journalctl --user -u hue-backend -f`
- Try manual start: `./backend/hue-sync`

**"Bridge unreachable"**
- Check your bridge IP is correct
- Ensure you're on the same network
- Try pinging the bridge: `ping BRIDGE_IP`

**"Status 401 Unauthorized"**
- Your API key is invalid or expired
- Run setup again to create a new key

**"Tray icon doesn't appear"**
- Check tray app is running: `ps aux | grep hue-tray`
- Try running manually: `./trayapp/hue-tray`
- Check for Qt errors in terminal output

**DBus communication errors**
- Verify backend is running and DBus service is registered
- Test DBus: `dbus-send --session --print-reply --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.GetStatus`
