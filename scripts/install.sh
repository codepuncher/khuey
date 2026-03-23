#!/bin/bash
# Installation script for Plasma Hue Widget

set -e

echo "=== Plasma Hue Widget Installation ==="
echo ""

# Check if backend is built
if [ ! -f "backend/hue-sync" ]; then
    echo "Building backend..."
    cd backend
    go build -o hue-sync ./cmd/hue-sync
    cd ..
    echo "✓ Backend built"
fi

# Install plasmoid
echo "Installing plasmoid..."
if kpackagetool6 --list | grep -q "org.kde.plasma.hue"; then
    echo "Widget already installed, upgrading..."
    kpackagetool6 --upgrade plasmoid --type Plasma/Applet
else
    kpackagetool6 --install plasmoid --type Plasma/Applet
fi
echo "✓ Plasmoid installed"

# Create systemd user service for backend
echo "Creating systemd service..."
mkdir -p ~/.config/systemd/user/

cat > ~/.config/systemd/user/plasma-hue-backend.service << EOF
[Unit]
Description=Plasma Hue Widget Backend
After=graphical-session.target

[Service]
Type=simple
ExecStart=$(pwd)/backend/hue-sync
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
echo "✓ Systemd service created"

echo ""
echo "=== Installation Complete! ==="
echo ""
echo "To start the backend:"
echo "  systemctl --user start plasma-hue-backend"
echo ""
echo "To enable auto-start on login:"
echo "  systemctl --user enable plasma-hue-backend"
echo ""
echo "To add widget to system tray:"
echo "  1. Right-click system tray"
echo "  2. Configure System Tray..."
echo "  3. Add Widgets..."
echo "  4. Search for 'Hue Control'"
echo ""
echo "Restarting plasmashell..."
kquitapp6 plasmashell && plasmashell &
echo ""
echo "Done! Check your system tray for the Hue Control widget."
