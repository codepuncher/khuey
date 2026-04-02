#!/bin/bash
# Installation script for KDE Hue Control

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== KDE Hue Control Installation ==="
echo ""

# Build backend
echo "Building backend..."
cd "$PROJECT_ROOT/backend"
go build -o hue-sync ./cmd/hue-sync
echo "✓ Backend built: backend/hue-sync"

# Build tray app
echo ""
echo "Building tray application..."
cd "$PROJECT_ROOT/trayapp"
if [ ! -f "Makefile" ]; then
    echo "  Running cmake..."
    cmake .
fi
make
echo "✓ Tray app built: trayapp/hue-tray"

# Install systemd service for backend
echo ""
echo "Installing systemd service for backend..."
mkdir -p ~/.config/systemd/user/

# Use the pre-configured service file and update the path
sed "s|ExecStart=.*|ExecStart=$PROJECT_ROOT/backend/hue-sync|" \
    "$PROJECT_ROOT/systemd/hue-backend.service" \
    > ~/.config/systemd/user/hue-backend.service

systemctl --user daemon-reload
echo "✓ Systemd service installed: ~/.config/systemd/user/hue-backend.service"

# Install autostart for tray app
echo ""
echo "Installing tray app autostart..."
mkdir -p ~/.config/autostart/

# Use the pre-configured desktop file and update the path
sed "s|Exec=.*|Exec=$PROJECT_ROOT/trayapp/hue-tray|" \
    "$PROJECT_ROOT/systemd/hue-tray.desktop" \
    > ~/.config/autostart/hue-tray.desktop

echo "✓ Autostart installed: ~/.config/autostart/hue-tray.desktop"

# Enable and start services
echo ""
echo "Enabling and starting services..."
systemctl --user enable hue-backend.service
systemctl --user start hue-backend.service
echo "✓ Backend service started"

# Start tray app
echo ""
echo "Starting tray application..."
"$PROJECT_ROOT/trayapp/hue-tray" &
echo "✓ Tray app started"

echo ""
echo "=== Installation Complete! ==="
echo ""
echo "Services installed:"
echo "  - Backend: systemctl --user status hue-backend"
echo "  - Tray app: Will auto-start on next login"
echo ""
echo "The Hue Control icon should now appear in your system tray."
echo ""
echo "Configuration file: ~/.openhue/config.yaml"
echo "  (Run 'openhue setup' if not configured yet)"
echo ""
echo "Logs:"
echo "  Backend: journalctl --user -u hue-backend -f"
echo "  Tray app: Check manually if issues occur"
echo ""
