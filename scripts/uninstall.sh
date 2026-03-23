#!/bin/bash

# Hue Control Uninstallation Script
# ==================================
# Removes the Hue Control services and stops running processes

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          Hue Control - Uninstallation Script                   ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Stop services
echo "Stopping services..."
systemctl --user stop hue-backend.service 2>/dev/null || true

# Kill tray app
if pgrep -f hue-tray > /dev/null; then
    pkill -f hue-tray
    echo "✓ Stopped tray app"
fi

# Remove systemd service
if [ -f ~/.config/systemd/user/hue-backend.service ]; then
    systemctl --user disable hue-backend.service 2>/dev/null || true
    rm ~/.config/systemd/user/hue-backend.service
    systemctl --user daemon-reload
    echo "✓ Removed backend service"
fi

# Remove autostart
if [ -f ~/.config/autostart/hue-tray.desktop ]; then
    rm ~/.config/autostart/hue-tray.desktop
    echo "✓ Removed tray app autostart"
fi

echo ""
echo "✅ Uninstallation complete!"
echo ""
echo "Note: Config file at ~/.openhue/config.yaml was NOT removed."
echo "      Remove it manually if you want to delete Hue bridge credentials."
echo ""
