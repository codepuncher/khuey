#!/bin/bash
# Integration test for KDE Hue Control

set -e

# Same lookup as getConfigPath in backend/internal/config/config.go, which
# follows openhue-cli
config_file_for() {
    if [ -n "$1" ]; then
        echo "$1/openhue/config.yaml"
        return
    fi
    echo "$HOME/.openhue/config.yaml"
}

CONFIG_FILE=$(config_file_for "$XDG_CONFIG_HOME")

echo "=== KDE Hue Control Integration Test ==="
echo ""

# Check prerequisites
echo "Checking prerequisites..."
command -v go >/dev/null 2>&1 || { echo "❌ go not found"; exit 1; }
echo "✓ Prerequisites OK"
echo ""

# Build backend
echo "Building backend..."
cd backend
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -o hue-sync ./cmd/hue-sync || { echo "❌ Build failed"; exit 1; }
cd ..
echo "✓ Backend built"
echo ""

# Test configuration
echo "Testing configuration..."
if [ ! -f "$CONFIG_FILE" ]; then
    echo "⚠ No config found at $CONFIG_FILE. Run 'openhue setup' to configure bridge."
    echo "  Tests will continue with limited functionality."
else
    echo "✓ Config found at $CONFIG_FILE"
fi
echo ""

# Start backend
echo "Starting backend..."
./backend/hue-sync &
BACKEND_PID=$!
sleep 3

# Test DBus interface
echo "Testing DBus interface..."
if dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetStatus >/dev/null 2>&1; then
    echo "✓ DBus service responding"
else
    echo "❌ DBus service not responding"
    kill $BACKEND_PID 2>/dev/null
    exit 1
fi

# Test GetScenes
echo "Testing GetScenes..."
SCENES=$(dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetScenes 2>&1)
if echo "$SCENES" | grep -q "array"; then
    SCENE_COUNT=$(echo "$SCENES" | grep -c "string" || echo "0")
    echo "✓ GetScenes returned $SCENE_COUNT scenes"
else
    echo "⚠ GetScenes failed (bridge may not be configured)"
fi

# Test tray app build
echo "Testing tray application build..."
cd trayapp
if [ -f "hue-tray" ]; then
    echo "✓ Tray app binary exists"
else
    echo "⚠ Tray app not built. Run 'cmake . && make' in trayapp/"
fi
cd ..

# Stop backend
echo ""
echo "Stopping backend..."
kill $BACKEND_PID 2>/dev/null
wait $BACKEND_PID 2>/dev/null || true
echo "✓ Backend stopped"

echo ""
echo "=== Integration Test Summary ==="
echo ""
echo "✅ Backend builds and runs"
echo "✅ DBus service works"
echo ""

if [ -f "$CONFIG_FILE" ]; then
    echo "Ready to install!"
    echo "Run: ./scripts/install.sh"
else
    echo "⚠ Setup required:"
    echo "  1. Run: openhue setup"
    echo "  2. Run: ./scripts/install.sh"
fi
echo ""
