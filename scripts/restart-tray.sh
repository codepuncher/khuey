#!/bin/bash
# Restart the Hue tray app

set -e

TRAY_APP="/home/lee/Code/misc/khuey/trayapp/hue-tray"
LOG_FILE="/tmp/hue-tray.log"

# Kill existing process
PID=$(pgrep -f hue-tray || true)
if [ -n "$PID" ]; then
    echo "Stopping tray app (PID: $PID)..."
    kill $PID
    sleep 1
fi

# Start new process
echo "Starting tray app..."
nohup "$TRAY_APP" > "$LOG_FILE" 2>&1 &
sleep 2

# Check if running
NEW_PID=$(pgrep -f hue-tray || true)
if [ -n "$NEW_PID" ]; then
    echo "✅ Tray app running (PID: $NEW_PID)"
    echo "Log: $LOG_FILE"
else
    echo "❌ Failed to start tray app"
    echo "Check log: $LOG_FILE"
    exit 1
fi
