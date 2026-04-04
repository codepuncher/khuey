#!/bin/bash
# Rebuild and restart the KDE Hue Control backend and tray app

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
BACKEND_APP="$BACKEND_DIR/hue-sync"
TRAY_DIR="$REPO_ROOT/trayapp"
TRAY_APP="$TRAY_DIR/hue-tray"
BACKEND_LOG="/tmp/hue-backend.log"
TRAY_LOG="/tmp/hue-tray.log"

# Stop backend
echo "🛑 Stopping backend..."
systemctl --user stop hue-backend || true
sleep 1

# Stop tray app
echo "🛑 Stopping tray app..."
PID=$(pgrep -f hue-tray || true)
if [ -n "$PID" ]; then
    kill $PID
    sleep 1
fi

# Build backend
echo "🔨 Building backend..."
cd "$BACKEND_DIR"
go build -o hue-sync ./cmd/hue-sync

# Build tray app
echo "🔨 Building tray app..."
cd "$TRAY_DIR"
cmake . > /dev/null 2>&1
make

# Start backend
echo "🚀 Starting backend..."
systemctl --user start hue-backend
sleep 2

# Start tray app
echo "🚀 Starting tray app..."
nohup "$TRAY_APP" > "$TRAY_LOG" 2>&1 &
sleep 2

# Check if running
BACKEND_STATUS=$(systemctl --user is-active hue-backend || echo "inactive")
TRAY_PID=$(pgrep -f hue-tray || true)

if [ "$BACKEND_STATUS" = "active" ] && [ -n "$TRAY_PID" ]; then
    echo "✅ Backend: running"
    echo "✅ Tray app: running (PID: $TRAY_PID)"
    echo "📋 Backend log: journalctl --user -u hue-backend -f"
    echo "📋 Tray log: $TRAY_LOG"
else
    echo "❌ Something failed to start"
    [ "$BACKEND_STATUS" != "active" ] && echo "  Backend: $BACKEND_STATUS"
    [ -z "$TRAY_PID" ] && echo "  Tray app: not running"
    exit 1
fi
