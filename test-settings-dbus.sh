#!/bin/bash
# Test Settings Dialog DBus Methods

set -e

echo "🧪 Testing Settings Dialog DBus Methods"
echo "========================================"
echo ""

# Check if backend is running
if ! pgrep -x "hue-sync" > /dev/null; then
    echo "⚠️  Backend not running. Starting it..."
    cd ~/Code/misc/khuey/backend
    ./hue-sync &
    BACKEND_PID=$!
    sleep 2
    echo "✅ Backend started (PID: $BACKEND_PID)"
else
    echo "✅ Backend already running"
    BACKEND_PID=""
fi

echo ""
echo "Test 1: GetSyncSettings"
echo "-----------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSyncSettings || echo "❌ Failed"

echo ""
echo "Test 2: SetSyncSettings (FPS=25, SubsampleWidth=80)"
echo "----------------------------------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.SetSyncSettings \
    int32:25 int32:80 string:"" || echo "❌ Failed"

echo ""
echo "Test 3: Verify settings were saved"
echo "-----------------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSyncSettings || echo "❌ Failed"

echo ""
echo "Test 4: GetBridgeSettings"
echo "-------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetBridgeSettings || echo "❌ Failed"

echo ""
echo "Test 5: GetSelectedRoom"
echo "-----------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSelectedRoom || echo "❌ Failed"

echo ""
echo "Test 6: TestBridgeConnection"
echo "----------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.TestBridgeConnection || echo "❌ Failed"

echo ""
echo "Test 7: Validate invalid FPS (should fail)"
echo "-------------------------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.SetSyncSettings \
    int32:999 int32:64 string:"" 2>&1 | grep -q "FPS must be between" && echo "✅ Correctly rejected" || echo "❌ Should have failed"

echo ""
echo "Test 8: Validate invalid subsample (should fail)"
echo "-------------------------------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.SetSyncSettings \
    int32:30 int32:999 string:"" 2>&1 | grep -q "subsample width must be between" && echo "✅ Correctly rejected" || echo "❌ Should have failed"

echo ""
echo "Test 9: Restore default settings"
echo "---------------------------------"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.SetSyncSettings \
    int32:30 int32:64 string:"" || echo "❌ Failed"

# Cleanup
if [ -n "$BACKEND_PID" ]; then
    echo ""
    echo "🧹 Cleaning up (stopping backend)"
    kill $BACKEND_PID 2>/dev/null || true
fi

echo ""
echo "✅ All tests completed!"
