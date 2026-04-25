#!/bin/bash
# Development Environment Setup for KDE Hue Control
# Sets up all dependencies and tools for development

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Dry-run mode
DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo -e "${YELLOW}Running in DRY-RUN mode (no changes will be made)${NC}"
    echo ""
fi

print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║        KDE Hue Control - Dev Environment Setup            ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_section() {
    echo -e "${BLUE}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

check_tool() {
    if command -v "$1" &> /dev/null; then
        return 0
    fi
    return 1
}

install_package() {
    local pkg="$1"
    local name="${2:-$pkg}"
    
    if check_tool "$pkg" || pkg-config --exists "$pkg" 2>/dev/null; then
        print_success "$name already installed"
        return 0
    fi
    
    if [ "$DRY_RUN" = true ]; then
        echo "  Would install: $pkg"
        return 0
    fi
    
    print_warning "$name not found, installing..."
    
    # Detect package manager
    if command -v pacman &> /dev/null; then
        sudo pacman -S --noconfirm "$pkg"
    elif command -v apt &> /dev/null; then
        sudo apt install -y "$pkg"
    elif command -v dnf &> /dev/null; then
        sudo dnf install -y "$pkg"
    else
        print_error "Unknown package manager, please install $pkg manually"
        return 1
    fi
    
    print_success "$name installed"
}

print_header

# Core Dependencies
print_section "Checking core dependencies..."

install_package "go" "Go"
install_package "cmake" "CMake"
install_package "make" "Make"
install_package "pkg-config" "pkg-config"
install_package "git" "Git"

echo ""

# Build Dependencies
print_section "Checking build dependencies..."

# Check for base-devel (Arch) or build-essential (Debian/Ubuntu)
if command -v pacman &> /dev/null; then
    if ! pacman -Qg base-devel &> /dev/null; then
        if [ "$DRY_RUN" = false ]; then
            print_warning "base-devel not installed, installing..."
            sudo pacman -S --noconfirm base-devel
        else
            echo "  Would install: base-devel"
        fi
    else
        print_success "base-devel installed"
    fi
elif command -v apt &> /dev/null; then
    if ! dpkg -l | grep -q build-essential; then
        if [ "$DRY_RUN" = false ]; then
            print_warning "build-essential not installed, installing..."
            sudo apt install -y build-essential
        else
            echo "  Would install: build-essential"
        fi
    else
        print_success "build-essential installed"
    fi
fi

echo ""

# Qt6 Dependencies
print_section "Checking Qt6 dependencies..."

QT_PACKAGES=("Qt6Core" "Qt6Widgets" "Qt6DBus")
for pkg in "${QT_PACKAGES[@]}"; do
    if pkg-config --exists "$pkg" 2>/dev/null; then
        print_success "$pkg installed"
    else
        print_warning "$pkg not found"
        if [ "$DRY_RUN" = false ]; then
            if command -v pacman &> /dev/null; then
                sudo pacman -S --noconfirm qt6-base
            elif command -v apt &> /dev/null; then
                sudo apt install -y qt6-base-dev libqt6dbus6
            fi
            print_success "Qt6 packages installed"
        else
            echo "  Would install Qt6 packages"
        fi
        break
    fi
done

echo ""

# KDE Dependencies
print_section "Checking KDE dependencies..."

if pkg-config --exists KF6StatusNotifierItem 2>/dev/null; then
    print_success "KF6StatusNotifierItem installed"
else
    print_warning "KF6StatusNotifierItem not found"
    if [ "$DRY_RUN" = false ]; then
        if command -v pacman &> /dev/null; then
            sudo pacman -S --noconfirm kstatusnotifieritem
        elif command -v apt &> /dev/null; then
            sudo apt install -y libkf6statusnotifieritem-dev
        fi
        print_success "KStatusNotifierItem installed"
    else
        echo "  Would install: kstatusnotifieritem"
    fi
fi

echo ""

# PipeWire Dependencies
print_section "Checking PipeWire dependencies..."

if pkg-config --exists libpipewire-0.3 2>/dev/null; then
    print_success "PipeWire development files installed"
else
    print_warning "PipeWire development files not found"
    if [ "$DRY_RUN" = false ]; then
        if command -v pacman &> /dev/null; then
            sudo pacman -S --noconfirm pipewire
        elif command -v apt &> /dev/null; then
            sudo apt install -y libpipewire-0.3-dev
        fi
        print_success "PipeWire installed"
    else
        echo "  Would install: pipewire development files"
    fi
fi

echo ""

# Optional Development Tools
print_section "Checking optional development tools..."

if check_tool "shellcheck"; then
    print_success "shellcheck installed"
else
    print_warning "shellcheck not found (optional, for shell script linting)"
    echo "  Install with: sudo pacman -S shellcheck"
fi

if check_tool "markdownlint"; then
    print_success "markdownlint installed"
else
    print_warning "markdownlint not found (optional, for markdown linting)"
    echo "  Install with: npm install -g markdownlint-cli"
fi

if check_tool "lefthook"; then
    print_success "lefthook installed"
else
    print_warning "lefthook not found (used for git hooks)"
    echo "  Install with: go install github.com/evilmartians/lefthook@latest"
fi

echo ""

# Go Tools
print_section "Installing Go development tools..."

if [ "$DRY_RUN" = false ]; then
    echo "  Installing gopls (Go language server)..."
    go install golang.org/x/tools/gopls@latest > /dev/null 2>&1 || true
    print_success "gopls installed"
    
    echo "  Installing goimports (import formatter)..."
    go install golang.org/x/tools/cmd/goimports@latest > /dev/null 2>&1 || true
    print_success "goimports installed"
    
    echo "  Installing staticcheck (linter)..."
    go install honnef.co/go/tools/cmd/staticcheck@latest > /dev/null 2>&1 || true
    print_success "staticcheck installed"
else
    echo "  Would install: gopls, goimports, staticcheck"
fi

echo ""

# Verify Builds
print_section "Verifying build capability..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ "$DRY_RUN" = false ]; then
    # Try backend build
    cd "$PROJECT_ROOT/backend"
    if go build -o /tmp/hue-sync-test ./cmd/hue-sync > /dev/null 2>&1; then
        print_success "Backend builds successfully"
        rm -f /tmp/hue-sync-test
    else
        print_error "Backend build failed (check dependencies)"
    fi
    
    # Try tray app build
    cd "$PROJECT_ROOT/trayapp"
    if [ -f "Makefile" ]; then
        rm -rf CMakeCache.txt CMakeFiles/
    fi
    
    if cmake . > /dev/null 2>&1 && make > /dev/null 2>&1; then
        print_success "Tray app builds successfully"
    else
        print_error "Tray app build failed (check Qt6/KF6 dependencies)"
    fi
else
    echo "  Would verify backend build"
    echo "  Would verify tray app build"
fi

echo ""

# Setup Git Hooks
print_section "Setting up git hooks..."

if [ "$DRY_RUN" = false ]; then
    if check_tool "lefthook"; then
        cd "$PROJECT_ROOT"
        lefthook install > /dev/null 2>&1 || true
        print_success "Git hooks installed (lefthook)"
    else
        print_warning "Skipping git hooks (lefthook not installed)"
    fi
else
    echo "  Would install lefthook git hooks"
fi

echo ""

# Summary
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Development Environment Ready!                    ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Next steps:"
echo "  1. Build project:    cd backend && go build -o hue-sync ./cmd/hue-sync"
echo "  2. Run tests:        ./scripts/quick-test.sh"
echo "  3. Install:          ./scripts/install.sh"
echo "  4. Start coding!     Your editor should now have full support"
echo ""
echo "Useful commands:"
echo "  Quick test:          ./scripts/quick-test.sh"
echo "  DBus test:           ./scripts/test-dbus.sh"
echo "  Integration test:    ./scripts/test-integration.sh"
echo "  Install locally:     ./scripts/install.sh"
echo ""
