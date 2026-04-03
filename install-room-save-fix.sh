#!/bin/bash
# Quick installation of the fix

set -e

echo "🔧 Installing Room Selection Save Fix"
echo "======================================"
echo ""

# Build backend
echo "🏗️  Building backend..."
cd ~/Code/misc/khuey/backend
go build -o hue-sync ./cmd/hue-sync
echo "✅ Backend built"
echo ""

# Restart service
echo "🔄 Restarting backend service..."
systemctl --user restart hue-backend
sleep 2

if systemctl --user is-active --quiet hue-backend; then
    echo "✅ Backend is running"
else
    echo "❌ Backend failed to start"
    echo "Check logs: journalctl --user -u hue-backend -n 50"
    exit 1
fi
echo ""

# Quick test
echo "🧪 Quick test..."
TEST_ROOM="quick-test-$(date +%s)"

dbus-send --session --print-reply --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue org.kde.plasma.hue.SetSelectedRoom \
    string:"$TEST_ROOM" > /dev/null 2>&1

sleep 1

SAVED=$(grep "^grouped_light_id:" ~/.openhue/config.yaml | awk '{print $2}')

if [ "$SAVED" = "$TEST_ROOM" ]; then
    echo "✅ Fix is working! Room selection saves to config."
else
    echo "⚠️  Test failed - expected: $TEST_ROOM, got: $SAVED"
    exit 1
fi
echo ""

echo "======================================"
echo "✅ FIX INSTALLED SUCCESSFULLY!"
echo ""
echo "You can now:"
echo "  1. Open Settings Dialog"
echo "  2. Select a room in Light Control tab"
echo "  3. Click Apply"
echo "  4. Room selection will save and persist!"
echo ""
echo "Run full tests with:"
echo "  ./test-all-config-saves.sh"
