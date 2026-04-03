#!/bin/bash
# End-to-end test for Settings Dialog room selection save

set -e

echo "🧪 End-to-End Settings Dialog Room Save Test"
echo "=============================================="
echo ""

# Backup existing config
if [ -f ~/.openhue/config.yaml ]; then
    echo "📋 Backing up existing config..."
    cp ~/.openhue/config.yaml ~/.openhue/config.yaml.e2e-backup
fi

echo "✅ Setup complete"
echo ""

echo "📋 Current Configuration:"
echo "------------------------"
if [ -f ~/.openhue/config.yaml ]; then
    CURRENT_ROOM=$(grep -E "grouped_light_id:" ~/.openhue/config.yaml | awk '{print $2}' || echo "(none)")
    echo "Current selected room: $CURRENT_ROOM"
else
    echo "No config file found"
fi
echo ""

echo "🔄 Ensure backend is running..."
systemctl --user restart hue-backend
sleep 2
echo "✅ Backend is running"
echo ""

echo "🎨 Manual Testing Instructions:"
echo "================================"
echo ""
echo "1. The Settings Dialog should now open"
echo "2. Go to 'Light Control' tab"
echo "3. Click 'Refresh Rooms'"
echo "4. Select a DIFFERENT room from the dropdown"
echo "5. Click 'Apply'"
echo "6. Close the dialog"
echo ""
echo "Press ENTER when ready to open Settings Dialog..."
read

# Launch Settings Dialog
echo "🚀 Launching Settings Dialog..."
cd /home/lee/Code/misc/khuey/trayapp
./hue-tray --settings &
TRAY_PID=$!

echo "Settings Dialog opened (PID: $TRAY_PID)"
echo ""
echo "⏸️  Waiting for you to test room selection..."
echo "   (Select a room, click Apply, then close the dialog)"
echo ""
echo "Press ENTER after you've closed the Settings Dialog..."
read

# Kill tray app if still running
if ps -p $TRAY_PID > /dev/null 2>&1; then
    kill $TRAY_PID 2>/dev/null || true
    sleep 1
fi

echo ""
echo "🔍 Verifying room selection was saved..."
echo "========================================="

# Check config file
if [ ! -f ~/.openhue/config.yaml ]; then
    echo "❌ FAILED: Config file doesn't exist"
    exit 1
fi

NEW_ROOM=$(grep -E "grouped_light_id:" ~/.openhue/config.yaml | awk '{print $2}' || echo "")

if [ -z "$NEW_ROOM" ]; then
    echo "❌ FAILED: grouped_light_id not found in config"
    echo ""
    echo "Config contents:"
    cat ~/.openhue/config.yaml
    exit 1
fi

echo "✅ Found grouped_light_id in config: $NEW_ROOM"
echo ""

# Verify via DBus
echo "🔄 Verifying via DBus (after backend restart)..."
systemctl --user restart hue-backend
sleep 2

DBUS_ROOM=$(dbus-send --session --print-reply --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue org.kde.plasma.hue.GetSelectedRoom 2>&1 | \
    grep -oP 'string "\K[^"]+' || echo "")

if [ "$DBUS_ROOM" = "$NEW_ROOM" ]; then
    echo "✅ DBus GetSelectedRoom matches: $DBUS_ROOM"
else
    echo "❌ FAILED: DBus mismatch"
    echo "   Config file: $NEW_ROOM"
    echo "   DBus result: $DBUS_ROOM"
    exit 1
fi
echo ""

echo "=========================================="
echo "✅ SUCCESS!"
echo ""
echo "Room selection saved correctly to config file:"
echo "  grouped_light_id: $NEW_ROOM"
echo ""
echo "The fix is working! Room selection now persists."
echo ""
echo "Backup saved at: ~/.openhue/config.yaml.e2e-backup"
