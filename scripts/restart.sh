#!/bin/bash
# Rebuild and restart the KDE Hue Control backend and tray app

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
TRAY_DIR="$REPO_ROOT/trayapp"
TRAY_APP="$TRAY_DIR/hue-tray"
TRAY_LOG="/tmp/hue-tray.log"

# Build before stopping anything. set -e aborts on a build failure, and with
# the stop steps below that would leave both services down with no recovery.
# Replacing a running binary is safe: the running process keeps its inode.

# Build backend. pkg-config for libpipewire emits -fno-strict-overflow, which
# cgo rejects unless it is allowlisted.
echo "Building backend..."
cd "$BACKEND_DIR"
CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -o hue-sync ./cmd/hue-sync

# Build tray app. cmake's stdout is noise, but its stderr is the only clue
# when a missing KF6 package fails the build.
echo "Building tray app..."
cd "$TRAY_DIR"
cmake . > /dev/null
make

# KNotification resolves the tray's event ids against this file and looks for
# it only under a data dir, so a rebuilt binary alone leaves notifications
# dead. install.sh does the same.
NOTIFYRC_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/knotifications6"
mkdir -p "$NOTIFYRC_DIR"
cp "$TRAY_DIR/hue-tray.notifyrc" "$NOTIFYRC_DIR/"

# Stop backend
echo "Stopping backend..."
systemctl --user stop hue-backend || true
sleep 1

# Stop tray app. Exact name, since -f would also match a tail of the tray log
# or an editor with the file open, and own-uid so this cannot signal another
# user. pkill rather than kill: more than one instance may be running, and a
# multi-PID list is not a single kill argument.
echo "Stopping tray app..."
if pkill -x -u "$(id -u)" hue-tray; then
    sleep 1
fi

# Start backend
echo "Starting backend..."
# Failure is reported by the status check below rather than aborting here,
# which would leave the tray killed and never restarted.
systemctl --user start hue-backend || true
sleep 2

# Start tray app
echo "Starting tray app..."
nohup "$TRAY_APP" > "$TRAY_LOG" 2>&1 &
sleep 2

# Check if running
BACKEND_STATUS=$(systemctl --user is-active hue-backend || echo "inactive")
TRAY_PID=$(pgrep -x -u "$(id -u)" hue-tray || true)

if [ "$BACKEND_STATUS" = "active" ] && [ -n "$TRAY_PID" ]; then
    echo "Backend: running"
    echo "Tray app: running (PID: $TRAY_PID)"
    echo "Backend log: journalctl --user -u hue-backend -f"
    echo "Tray log: $TRAY_LOG"
else
    echo "Something failed to start"
    [ "$BACKEND_STATUS" != "active" ] && echo "  Backend: $BACKEND_STATUS"
    [ -z "$TRAY_PID" ] && echo "  Tray app: not running"
    exit 1
fi
