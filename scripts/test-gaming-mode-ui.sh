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
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print test result
test_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
    else
        echo -e "${RED}✗${NC} $2"
        exit 1
    fi
}

# 1. Check backend is running
echo "1. Checking backend service..."
systemctl --user is-active hue-backend > /dev/null 2>&1
test_result $? "Backend service is running"

# 2. Test DBus service availability
echo ""
echo "2. Testing DBus service..."
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetStatus > /dev/null 2>&1
test_result $? "DBus service is available"

# 3. Test Gaming Mode methods
echo ""
echo "3. Testing Gaming Mode DBus methods..."

# Test IsGamingModeEnabled
ENABLED=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')
test_result $? "IsGamingModeEnabled method works (current: $ENABLED)"

# Test IsGamingModeActive
ACTIVE=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeActive 2>&1 | grep boolean | awk '{print $2}')
test_result $? "IsGamingModeActive method works (current: $ACTIVE)"

# 4. Test Gaming Mode toggle
echo ""
echo "4. Testing Gaming Mode toggle..."

# Get current state
INITIAL=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')
echo "   Initial state: $INITIAL"

# Test toggle (flip state and verify)
if [ "$INITIAL" = "true" ]; then
    # Disable
    dbus-send --session --dest=org.kde.plasma.hue \
      /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:false > /dev/null 2>&1
    test_result $? "SetGamingMode(false) call succeeded"
    
    sleep 3  # Give it time to save and update
    
    AFTER=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
      /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')
    [ "$AFTER" = "false" ]
    test_result $? "Gaming Mode successfully disabled"
    
    # Re-enable for next run
    dbus-send --session --dest=org.kde.plasma.hue \
      /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true > /dev/null 2>&1
    sleep 3
else
    # Enable
    dbus-send --session --dest=org.kde.plasma.hue \
      /org/kde/plasma/hue org.kde.plasma.hue.SetGamingMode boolean:true > /dev/null 2>&1
    test_result $? "SetGamingMode(true) call succeeded"
    
    sleep 3  # Give it time to save and update
    
    AFTER=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
      /org/kde/plasma/hue org.kde.plasma.hue.IsGamingModeEnabled 2>&1 | grep boolean | awk '{print $2}')
    [ "$AFTER" = "true" ]
    test_result $? "Gaming Mode successfully enabled"
fi

# 5. Check tray app is running
echo ""
echo "5. Checking tray app..."
ps aux | grep hue-tray | grep -v grep > /dev/null 2>&1
test_result $? "Tray app is running"

# 6. Test Settings dialog methods
echo ""
echo "6. Testing Settings dialog integration..."

# GetSyncSettings should return a dict with gaming mode info
dbus-send --session --print-reply --dest=org.kde.plasma.hue \
  /org/kde/plasma/hue org.kde.plasma.hue.GetSyncSettings > /dev/null 2>&1
test_result $? "GetSyncSettings method works"

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
echo "10. Verify sync status shows '🎮 Gaming Mode' indicator"
echo "11. Verify tray tooltip updates to show gaming status"
echo ""
echo "To simulate game detection:"
echo "  sudo systemd-inhibit --what=handle-lid-switch:idle --who=CachyOS --why=game-performance:1 sleep 30 &"
echo ""
