#!/bin/bash
# Uninstallation script for KDE Hue Control

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Same lookup as getConfigPath in backend/internal/config/config.go, which
# follows openhue-cli
config_file_for() {
    if [ -n "$1" ]; then
        echo "$1/openhue/config.yaml"
        return
    fi
    echo "$HOME/.openhue/config.yaml"
}

CONFIG_FILE=$(config_file_for "$XDG_CONFIG_HOME")

# Dry-run mode
DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo -e "${YELLOW}Running in DRY-RUN mode (no changes will be made)${NC}"
    echo ""
fi

print_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_section() {
    echo -e "${BLUE}==>${NC} $1"
}

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        KDE Hue Control - Uninstallation Script            ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ "$DRY_RUN" = false ]; then
    print_warning "This will remove KDE Hue Control from your system."
    echo "  - Backend service will be stopped and removed"
    echo "  - Tray app will be stopped and autostart removed"
    echo "  - Config file ($CONFIG_FILE) will NOT be removed"
    echo ""
    read -p "Continue? (y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Uninstallation cancelled."
        exit 0
    fi
    echo ""
fi

# Stop and remove backend service
print_section "Stopping backend service..."

if [ "$DRY_RUN" = true ]; then
    echo "  Would stop: hue-backend.service"
    echo "  Would disable: hue-backend.service"
    echo "  Would remove: ~/.config/systemd/user/hue-backend.service"
    print_success "Backend service (dry-run)"
else
    if systemctl --user is-active --quiet hue-backend.service; then
        systemctl --user stop hue-backend.service
        print_success "Stopped backend service"
    else
        print_warning "Backend service not running"
    fi
    
    if [ -f ~/.config/systemd/user/hue-backend.service ]; then
        systemctl --user disable hue-backend.service 2>/dev/null || true
        rm ~/.config/systemd/user/hue-backend.service
        systemctl --user daemon-reload
        print_success "Removed backend service"
    else
        print_warning "Backend service file not found"
    fi
fi
echo ""

# Stop and remove tray app
print_section "Stopping tray app..."

if [ "$DRY_RUN" = true ]; then
    echo "  Would kill: hue-tray processes"
    echo "  Would remove: ~/.config/autostart/hue-tray.desktop"
    print_success "Tray app (dry-run)"
else
    if pgrep -f hue-tray > /dev/null; then
        pkill -f hue-tray
        print_success "Stopped tray app"
    else
        print_warning "Tray app not running"
    fi
    
    if [ -f ~/.config/autostart/hue-tray.desktop ]; then
        rm ~/.config/autostart/hue-tray.desktop
        print_success "Removed tray app autostart"
    else
        print_warning "Tray app autostart not found"
    fi
fi
echo ""

echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Uninstallation Complete!                         ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "What was removed:"
echo "  - Backend systemd service"
echo "  - Tray app autostart"
echo "  - Running processes"
echo ""
echo "What was NOT removed:"
echo "  - Config file: $CONFIG_FILE (contains Hue credentials)"
echo "  - Build artifacts: backend/hue-sync, trayapp/hue-tray"
echo ""
echo "To reinstall, run: ./scripts/install.sh"
echo ""
