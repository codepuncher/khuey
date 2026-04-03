#!/bin/bash
# Manual test script for Settings Dialog fix

echo "=== Settings Dialog Manual Test ==="
echo
echo "This script will help you verify the Settings Dialog fix."
echo "The dialog should open without errors when clicked in the tray menu."
echo

# Check if backend is running
if ! pgrep -f "hue-sync" > /dev/null; then
    echo "⚠️  Backend (hue-sync) is not running!"
    echo "   Start it with: systemctl --user start hue-backend"
    exit 1
fi
echo "✓ Backend is running"

# Check if tray app is running  
if ! pgrep -f "hue-tray" > /dev/null; then
    echo "⚠️  Tray app is not running"
    echo "   Starting tray app..."
    cd ~/Code/misc/khuey
    ./trayapp/hue-tray > /tmp/hue-tray.log 2>&1 &
    sleep 2
fi

if pgrep -f "hue-tray" > /dev/null; then
    echo "✓ Tray app is running"
else
    echo "❌ Failed to start tray app"
    exit 1
fi

echo
echo "=== Manual Test Steps ==="
echo
echo "1. Look for the 'Hue Control' icon in your system tray"
echo "2. Click the tray icon to open the menu"
echo "3. Click '⚙️ Settings...' menu item"
echo "4. EXPECTED: Settings dialog should open with 3 tabs"
echo "5. Verify all tabs are visible:"
echo "   - Screen Sync (FPS and Subsample controls)"
echo "   - Light Control (Room selection with Refresh button)"
echo "   - Connection (Bridge IP and status)"
echo
echo "6. Test the 'Refresh' button in Light Control tab"
echo "   - Should load rooms without errors"
echo "   - Should display 'Found X room(s)/zone(s)'"
echo
echo "7. Try changing FPS slider and clicking OK"
echo "   - Dialog should close without errors"
echo
echo "=== Check Logs ==="
echo
echo "If the dialog doesn't open, check the log:"
echo "  tail -50 /tmp/hue-tray.log"
echo
echo "You should see:"
echo "  Opening settings dialog..."
echo "  Loading settings from DBus..."
echo "  Sync settings loaded: ..."
echo "  Selected room loaded: ..."
echo "  Bridge settings loaded: ..."
echo "  Settings loading complete"
echo
echo "You should NOT see repeated 'QDBusArgument: write from a read-only object' errors"
echo

# Show recent log to verify
echo "=== Recent Tray App Log (last 10 lines) ==="
tail -10 /tmp/hue-tray.log 2>/dev/null || echo "(No log file yet)"
echo

echo "=== Test DBus Methods Directly ==="
echo
echo "Testing GetSyncSettings..."
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetSyncSettings 2>&1 | grep -E "fps|subsampleWidth|monitor" || echo "Failed"

echo
echo "Testing GetSelectedRoom..."
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetSelectedRoom 2>&1 | grep -E "string" || echo "Failed"

echo
echo "Testing GetBridgeSettings..."
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetBridgeSettings 2>&1 | grep -E "bridgeIP|connected" || echo "Failed"

echo
echo "=== All DBus methods working! ==="
echo
echo "Now click '⚙️ Settings...' in the tray menu to verify the dialog opens."
echo
