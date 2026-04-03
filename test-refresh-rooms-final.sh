#!/bin/bash

# Test script to verify the Refresh Rooms button works correctly

echo "==================================================================="
echo "Testing Refresh Rooms Functionality"
echo "==================================================================="
echo ""

# Check if backend is running
echo "1. Checking backend status..."
if ! systemctl --user is-active --quiet hue-backend; then
    echo "   ❌ Backend not running. Start it with: systemctl --user start hue-backend"
    exit 1
fi
echo "   ✅ Backend is running"
echo ""

# Test DBus call directly
echo "2. Testing GetGroupedLights DBus method..."
RESULT=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue org.kde.plasma.hue.GetGroupedLights 2>&1)

if echo "$RESULT" | grep -q "method return"; then
    echo "   ✅ DBus method works"
    echo "   Rooms/zones found:"
    echo "$RESULT" | grep "string" | head -20
else
    echo "   ❌ DBus call failed:"
    echo "$RESULT"
    exit 1
fi
echo ""

# Test with our C++ test program
echo "3. Testing struct extraction with test program..."
cd /home/lee/Code/misc/khuey
if [ ! -f ./test-refresh-rooms ]; then
    echo "   Rebuilding test program..."
    g++ -o test-refresh-rooms test-refresh-rooms.cpp -fPIC \
        -I/usr/include/qt6/QtDBus -I/usr/include/qt6 -DQT_DBUS_LIB \
        -I/usr/include/qt6/QtCore -DQT_CORE_LIB \
        -I/usr/lib/qt6/mkspecs/linux-g++ -lQt6DBus -lQt6Core 2>&1
fi

OUTPUT=$(timeout 5 ./test-refresh-rooms 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -eq 124 ]; then
    echo "   ❌ Test program timed out (infinite loop - extraction failed)"
    echo "$OUTPUT" | head -20
    exit 1
elif [ $EXIT_CODE -ne 0 ]; then
    echo "   ❌ Test program failed with exit code $EXIT_CODE"
    echo "$OUTPUT"
    exit 1
fi

if echo "$OUTPUT" | grep -q "Successfully extracted.*rooms/zones"; then
    echo "   ✅ Struct extraction works correctly"
    echo ""
    echo "   Extracted data:"
    echo "$OUTPUT" | grep -E "(ID:|Name:|Type:)"
else
    echo "   ❌ Extraction failed - no success message"
    echo "$OUTPUT"
    exit 1
fi
echo ""

# Instructions for manual testing
echo "==================================================================="
echo "Manual Testing Instructions"
echo "==================================================================="
echo ""
echo "Now test the Settings Dialog:"
echo "  1. Run the tray app: cd trayapp && ./hue-tray &"
echo "  2. Click the tray icon"
echo "  3. Click '⚙️ Settings...'"
echo "  4. Go to 'Light Control' tab"
echo "  5. Click 'Refresh Rooms' button"
echo ""
echo "Expected result:"
echo "  ✅ Dropdown populates with actual room names"
echo "  ✅ No 'QDBusArgument: write from a read-only object' errors"
echo "  ✅ Status shows 'Found X room(s)/zone(s)'"
echo ""
echo "If you see empty strings or errors, the fix didn't work."
echo ""
echo "==================================================================="
echo "All automated tests PASSED ✅"
echo "==================================================================="
