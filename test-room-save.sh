#!/bin/bash
# Test script to verify room selection saves correctly

set -e

echo "🧪 Testing Room Selection Save Fix"
echo "=================================="
echo ""

# Backup existing config
if [ -f ~/.openhue/config.yaml ]; then
    echo "📋 Backing up existing config..."
    cp ~/.openhue/config.yaml ~/.openhue/config.yaml.backup
fi

# Restart backend with new build
echo "🔄 Restarting backend..."
systemctl --user restart hue-backend
sleep 2

# Check backend is running
if ! systemctl --user is-active --quiet hue-backend; then
    echo "❌ Backend failed to start"
    exit 1
fi
echo "✅ Backend is running"
echo ""

# Get current config content BEFORE
echo "📄 Config BEFORE SetSelectedRoom:"
if [ -f ~/.openhue/config.yaml ]; then
    grep -E "(grouped_light_id|GroupedLightID)" ~/.openhue/config.yaml || echo "(no grouped_light_id set)"
else
    echo "(config file doesn't exist)"
fi
echo ""

# Get available rooms
echo "🏠 Getting available rooms..."
ROOMS=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue org.kde.plasma.hue.GetGroupedLights 2>/dev/null | \
    grep -oP 'string "\K[^"]+' | head -1)

if [ -z "$ROOMS" ]; then
    echo "❌ No rooms found via DBus (may be normal if no rooms configured)"
    echo "   Using test room ID: test-room-123"
    TEST_ROOM_ID="test-room-123"
else
    echo "✅ Found rooms via DBus"
    TEST_ROOM_ID="$ROOMS"
fi
echo ""

# Test SetSelectedRoom
echo "💾 Setting selected room to: $TEST_ROOM_ID"
RESULT=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue org.kde.plasma.hue.SetSelectedRoom \
    string:"$TEST_ROOM_ID" 2>&1)

if echo "$RESULT" | grep -q "boolean true"; then
    echo "✅ SetSelectedRoom returned true"
else
    echo "❌ SetSelectedRoom failed:"
    echo "$RESULT"
    exit 1
fi
echo ""

# Wait for file to be written
sleep 1

# Check config content AFTER
echo "📄 Config AFTER SetSelectedRoom:"
if [ -f ~/.openhue/config.yaml ]; then
    SAVED_ROOM=$(grep -E "grouped_light_id:" ~/.openhue/config.yaml | awk '{print $2}' || echo "")
    
    if [ -z "$SAVED_ROOM" ]; then
        echo "❌ grouped_light_id NOT FOUND in config.yaml"
        echo ""
        echo "Full config:"
        cat ~/.openhue/config.yaml
        exit 1
    elif [ "$SAVED_ROOM" = "$TEST_ROOM_ID" ]; then
        echo "✅ grouped_light_id: $SAVED_ROOM"
        echo "✅ VALUE MATCHES! Room selection was saved correctly!"
    else
        echo "❌ grouped_light_id: $SAVED_ROOM"
        echo "❌ VALUE MISMATCH! Expected: $TEST_ROOM_ID"
        exit 1
    fi
else
    echo "❌ Config file doesn't exist"
    exit 1
fi
echo ""

# Verify it persists after backend restart
echo "🔄 Testing persistence after backend restart..."
systemctl --user restart hue-backend
sleep 2

GET_RESULT=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue org.kde.plasma.hue.GetSelectedRoom 2>&1 | \
    grep -oP 'string "\K[^"]+' || echo "")

if [ "$GET_RESULT" = "$TEST_ROOM_ID" ]; then
    echo "✅ GetSelectedRoom returned: $GET_RESULT"
    echo "✅ PERSISTENCE VERIFIED! Room selection survived restart!"
else
    echo "❌ GetSelectedRoom returned: $GET_RESULT"
    echo "❌ Expected: $TEST_ROOM_ID"
    exit 1
fi
echo ""

echo "=================================="
echo "✅ ALL TESTS PASSED!"
echo "Room selection save is working correctly."
echo ""
echo "Backup saved at: ~/.openhue/config.yaml.backup"
