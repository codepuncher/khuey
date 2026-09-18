#!/bin/bash
# Test script for Gaming Mode UI Integration

set -e

echo "========================================="
echo "Gaming Mode UI Integration Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print test result
test_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}[OK]${NC} $2"
    else
        echo -e "${RED}[FAIL]${NC} $2"
        exit 1
    fi
}

# 1. Check backend is running
echo "1. Checking backend service..."
if systemctl --user is-active hue-backend > /dev/null 2>&1; then
    test_result 0 "Backend service is running"
else
    test_result 1 "Backend service is running"
fi

# 2. Test DBus service availability
echo ""
echo "2. Testing DBus service..."
if dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetStatus > /dev/null 2>&1; then
    test_result 0 "DBus service is available"
else
    test_result 1 "DBus service is available"
fi

# 3. Test Gaming Mode methods
echo ""
echo "3. Testing Gaming Mode DBus methods..."

# Test IsGamingModeEnabled
ENABLED=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')
if [ "$ENABLED" = "true" ] || [ "$ENABLED" = "false" ]; then
    test_result 0 "IsGamingModeEnabled method works (current: $ENABLED)"
else
    test_result 1 "IsGamingModeEnabled method returned '$ENABLED'"
fi

# Test IsGamingModeActive
ACTIVE=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeActive 2>&1 | grep boolean | awk '{print $2}')
if [ "$ACTIVE" = "true" ] || [ "$ACTIVE" = "false" ]; then
    test_result 0 "IsGamingModeActive method works (current: $ACTIVE)"
else
    test_result 1 "IsGamingModeActive method returned '$ACTIVE'"
fi

# 4. Test Gaming Mode toggle
echo ""
echo "4. Testing Gaming Mode toggle..."

# Get current state
INITIAL=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')
echo "   Initial state: $INITIAL"

# The restore below sends this value back, so a bad parse would flip the
# setting and still report a pass.
if [ "$INITIAL" != "true" ] && [ "$INITIAL" != "false" ]; then
    test_result 1 "Read initial Gaming Mode state (got '$INITIAL')"
fi

# Test toggle (flip state, verify, restore)
if [ "$INITIAL" = "true" ]; then
    TARGET=false
else
    TARGET=true
fi

# --print-reply is load-bearing: without it dbus-send sets NO_REPLY_EXPECTED,
# the backend never runs the handler, and the call still exits 0.
if dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode "boolean:$TARGET" > /dev/null 2>&1; then
    test_result 0 "SetGamingMode($TARGET) call succeeded"
else
    test_result 1 "SetGamingMode($TARGET) call succeeded"
fi

sleep 3  # Give it time to save and update

AFTER=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')

# Restore before asserting, so a mismatch does not leave the setting flipped.
# `|| true` keeps set -e from killing the script before the assertion below.
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode "boolean:$INITIAL" > /dev/null 2>&1 || true
sleep 3

if [ "$AFTER" = "$TARGET" ]; then
    test_result 0 "Gaming Mode toggled to $TARGET"
else
    test_result 1 "Gaming Mode toggled to $TARGET (read back '$AFTER')"
fi

# 5. Check tray app is running
echo ""
echo "5. Checking tray app..."
if ps aux | grep hue-tray | grep -v grep > /dev/null 2>&1; then
    test_result 0 "Tray app is running"
else
    test_result 1 "Tray app is running"
fi

# 6. Test Settings dialog methods
echo ""
echo "6. Testing Settings dialog integration..."

# GetSyncSettings should return a dict with gaming mode info
if dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetSyncSettings > /dev/null 2>&1; then
    test_result 0 "GetSyncSettings method works"
else
    test_result 1 "GetSyncSettings method works"
fi

echo ""
echo "========================================="
echo -e "${GREEN}All tests passed!${NC}"
echo "========================================="
echo ""

echo "Manual Testing Checklist:"
echo "-------------------------"
echo "1. Open Settings dialog (right-click tray icon → Settings)"
echo "2. Go to 'Screen Sync' tab"
echo "3. Verify Gaming Mode checkbox is present"
echo "4. Toggle Gaming Mode checkbox and click Apply"
echo "5. Verify setting persists after reopening Settings"
echo ""
echo "6. Open Control Panel (click tray icon)"
echo "7. Verify sync status label shows Gaming Mode state"
echo "8. Launch a game (e.g., Skyrim)"
echo "9. Wait 5-7 seconds"
echo "10. Verify the sync status line ends with '(Gaming)'"
echo "11. Verify tray tooltip updates to show gaming status"
echo ""
echo "To simulate game detection:"
echo "  sudo systemd-inhibit --what=handle-lid-switch:idle --who=CachyOS --why=game-performance:1 sleep 30 &"
echo ""
