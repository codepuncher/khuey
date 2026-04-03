#!/bin/bash

# Quick manual test for the Settings Dialog Refresh Rooms button

echo "====================================================================="
echo "Settings Dialog - Refresh Rooms Test"
echo "====================================================================="
echo ""

# Kill any existing tray app
EXISTING_PID=$(ps aux | grep hue-tray | grep -v grep | awk '{print $2}' | head -1)
if [ -n "$EXISTING_PID" ]; then
    echo "Killing existing tray app (PID: $EXISTING_PID)..."
    kill $EXISTING_PID
    sleep 1
fi

# Check backend
if ! systemctl --user is-active --quiet hue-backend; then
    echo "❌ Backend not running. Start with: systemctl --user start hue-backend"
    exit 1
fi

echo "Starting tray app with debug output..."
echo ""
echo "Watch for these messages when you click 'Refresh Rooms':"
echo "  ✅ 'onRefreshRoomsClicked called'"
echo "  ✅ 'GetGroupedLights replied successfully'"
echo "  ✅ 'Room/Zone: Living room (room) ID: ...'"
echo "  ✅ 'Loaded X room(s)/zone(s)'"
echo ""
echo "❌ Should NOT see: 'QDBusArgument: write from a read-only object'"
echo "❌ Should NOT see: 'Room/Zone: \"\" ( \"\" ) ID: \"\"'"
echo ""
echo "====================================================================="
echo ""
echo "Starting tray app now..."
echo "1. Click the system tray icon"
echo "2. Click '⚙️ Settings...'"
echo "3. Go to 'Light Control' tab"
echo "4. Click 'Refresh Rooms' button"
echo "5. Check the dropdown has actual room names (not empty)"
echo ""
echo "Press Ctrl+C to stop the tray app when done testing."
echo ""
echo "====================================================================="
echo ""

cd /home/lee/Code/misc/khuey/trayapp
exec ./hue-tray
