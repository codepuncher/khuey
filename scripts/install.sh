#!/bin/bash
# Installation script for KDE Hue Control

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Dry-run mode (set via --dry-run flag)
DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo -e "${YELLOW}Running in DRY-RUN mode (no changes will be made)${NC}"
    echo ""
fi

# Helper functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║        KDE Hue Control - Installation Script              ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_section() {
    echo -e "${BLUE}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗ Error:${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_error "$2"
        return 1
    fi
    return 0
}

# Same lookup as getConfigPath in backend/internal/config/config.go, which
# follows openhue-cli
config_file_for() {
    if [ -n "$1" ]; then
        echo "$1/openhue/config.yaml"
        return
    fi
    echo "$HOME/.openhue/config.yaml"
}

# Print header
print_header

# Check dependencies
print_section "Checking dependencies..."

MISSING_DEPS=()

if ! check_command "go" "Go is not installed. Install with: sudo pacman -S go"; then
    MISSING_DEPS+=("go")
fi

if ! check_command "cmake" "CMake is not installed. Install with: sudo pacman -S cmake"; then
    MISSING_DEPS+=("cmake")
fi

if ! check_command "make" "Make is not installed. Install with: sudo pacman -S base-devel"; then
    MISSING_DEPS+=("make")
fi

if ! check_command "pkg-config" "pkg-config is not installed. Install with: sudo pacman -S pkgconf"; then
    MISSING_DEPS+=("pkg-config")
fi

# Check for Qt6
if ! pkg-config --exists Qt6Core 2>/dev/null; then
    print_error "Qt6 is not installed. Install with: sudo pacman -S qt6-base"
    MISSING_DEPS+=("qt6")
fi

# Check for KF6
if ! pkg-config --exists KF6StatusNotifierItem 2>/dev/null; then
    print_warning "KF6StatusNotifierItem not found. Install with: sudo pacman -S kstatusnotifieritem"
    print_warning "Tray app may not build without this."
fi

if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
    echo ""
    print_error "Missing required dependencies: ${MISSING_DEPS[*]}"
    echo ""
    echo "Install all dependencies with:"
    echo "  sudo pacman -S go cmake base-devel pkgconf qt6-base kstatusnotifieritem"
    echo ""
    exit 1
fi

print_success "All required dependencies found"
echo ""

# Check for config file
print_section "Checking configuration..."

CONFIG_FILE=$(config_file_for "$XDG_CONFIG_HOME")

if [ ! -f "$CONFIG_FILE" ]; then
    print_warning "Config file not found at $CONFIG_FILE"
    echo "  You'll need to configure the Hue bridge before the backend will work."
    echo "  Create the config file or run: openhue setup"
    echo ""
    if [ "$DRY_RUN" = false ]; then
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Installation cancelled."
            exit 1
        fi
    fi
else
    print_success "Config file found at $CONFIG_FILE"
    
    # The backend reads keys case-insensitively and saves them lowercase
    if ! grep -qi "^Bridge:" "$CONFIG_FILE"; then
        print_warning "Config file missing 'Bridge' field"
    fi
    if ! grep -qi "^Key:" "$CONFIG_FILE"; then
        print_warning "Config file missing 'Key' field"
    fi
fi
echo ""

# Build backend
print_section "Building backend..."
cd "$PROJECT_ROOT/backend"

if [ "$DRY_RUN" = true ]; then
    echo "  Would run: CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -o hue-sync ./cmd/hue-sync"
    print_success "Backend build (dry-run)"
else
    # pkg-config for libpipewire emits -fno-strict-overflow, which cgo rejects
    # unless it is allowlisted.
    if CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -o hue-sync ./cmd/hue-sync; then
        print_success "Backend built: backend/hue-sync"
    else
        print_error "Backend build failed"
        exit 1
    fi
fi
echo ""

# Build tray app
print_section "Building tray application..."
cd "$PROJECT_ROOT/trayapp"

if [ "$DRY_RUN" = true ]; then
    echo "  Would run: cmake . && make"
    print_success "Tray app build (dry-run)"
else
    if [ ! -f "Makefile" ]; then
        echo "  Running cmake..."
        if ! cmake .; then
            print_error "CMake configuration failed"
            exit 1
        fi
    fi
    
    if make; then
        print_success "Tray app built: trayapp/hue-tray"
    else
        print_error "Tray app build failed"
        exit 1
    fi
fi
echo ""

# Install systemd service for backend
print_section "Installing systemd service..."
mkdir -p ~/.config/systemd/user/

if [ "$DRY_RUN" = true ]; then
    echo "  Would install: ~/.config/systemd/user/hue-backend.service"
    print_success "Systemd service (dry-run)"
else
    # Use the pre-configured service file and update the path
    sed "s|ExecStart=.*|ExecStart=$PROJECT_ROOT/backend/hue-sync|" \
        "$PROJECT_ROOT/systemd/hue-backend.service" \
        > ~/.config/systemd/user/hue-backend.service
    
    systemctl --user daemon-reload
    print_success "Systemd service: ~/.config/systemd/user/hue-backend.service"
fi
echo ""

# Install autostart for tray app
print_section "Installing tray app autostart..."
mkdir -p ~/.config/autostart/

if [ "$DRY_RUN" = true ]; then
    echo "  Would install: ~/.config/autostart/hue-tray.desktop"
    print_success "Autostart (dry-run)"
else
    # Use the pre-configured desktop file and update the path
    sed "s|Exec=.*|Exec=$PROJECT_ROOT/trayapp/hue-tray|" \
        "$PROJECT_ROOT/systemd/hue-tray.desktop" \
        > ~/.config/autostart/hue-tray.desktop
    
    print_success "Autostart: ~/.config/autostart/hue-tray.desktop"
fi
echo ""

# Enable and start services
print_section "Starting services..."

if [ "$DRY_RUN" = true ]; then
    echo "  Would enable: hue-backend.service"
    echo "  Would start: hue-backend.service"
    echo "  Would start: hue-tray"
    print_success "Services (dry-run)"
else
    # Check if service is already running
    if systemctl --user is-active --quiet hue-backend.service; then
        print_warning "Backend service already running, restarting..."
        systemctl --user restart hue-backend.service
    else
        systemctl --user enable hue-backend.service
        systemctl --user start hue-backend.service
    fi
    
    # Verify service started
    sleep 1
    if systemctl --user is-active --quiet hue-backend.service; then
        print_success "Backend service started"
    else
        print_error "Backend service failed to start"
        echo "  Check logs: journalctl --user -u hue-backend -n 50"
        exit 1
    fi
    
    # Start tray app (kill existing if running)
    if pgrep -f "hue-tray" > /dev/null; then
        print_warning "Tray app already running, restarting..."
        pkill -f "hue-tray"
        sleep 1
    fi
    
    "$PROJECT_ROOT/trayapp/hue-tray" &
    print_success "Tray app started"
fi
echo ""

# Print completion summary
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║            Installation Complete!                          ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Services installed:"
echo "  Backend:  systemctl --user status hue-backend"
echo "  Tray app: Will auto-start on next login"
echo ""
echo "Configuration:"
echo "  Config file: $CONFIG_FILE"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "  ${YELLOW}⚠ Not configured yet!${NC} Run: openhue setup"
fi
echo ""
echo "Useful commands:"
echo "  View logs:    journalctl --user -u hue-backend -f"
echo "  Restart:      systemctl --user restart hue-backend"
echo "  Stop:         systemctl --user stop hue-backend"
echo "  Uninstall:    $SCRIPT_DIR/uninstall.sh"
echo ""
echo "The Hue Control icon should now appear in your system tray."
echo ""
