#!/bin/bash
# Quick Test Runner for KDE Hue Control
# Builds and tests backend + tray app

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Options
RUN_TESTS=true
RUN_BUILD=true
RUN_LINT=false
RUN_INTEGRATION=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --no-tests)
            RUN_TESTS=false
            shift
            ;;
        --no-build)
            RUN_BUILD=false
            shift
            ;;
        --lint)
            RUN_LINT=true
            shift
            ;;
        --integration)
            RUN_INTEGRATION=true
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --no-tests      Skip unit tests"
            echo "  --no-build      Skip build step"
            echo "  --lint          Run linters (go vet, gofmt)"
            echo "  --integration   Run integration tests (requires bridge)"
            echo "  --help          Show this help"
            echo ""
            echo "Examples:"
            echo "  $0                      # Build + test"
            echo "  $0 --no-build           # Test only"
            echo "  $0 --lint               # Build + test + lint"
            echo "  $0 --integration        # Full suite including integration"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Run with --help for usage"
            exit 1
            ;;
    esac
done

# Print functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          KDE Hue Control - Quick Test Runner              ║${NC}"
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

# Track results
ERRORS=0

print_header

# Backend Build
if [ "$RUN_BUILD" = true ]; then
    print_section "Building backend..."
    cd "$PROJECT_ROOT/backend"
    
    if CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -o hue-sync ./cmd/hue-sync; then
        print_success "Backend built successfully"
    else
        print_error "Backend build failed"
        ((ERRORS++))
    fi
    echo ""
fi

# Backend Tests
if [ "$RUN_TESTS" = true ]; then
    print_section "Running backend tests..."
    cd "$PROJECT_ROOT/backend"
    
    if CGO_CFLAGS_ALLOW='-fno-strict-overflow' go test ./...; then
        print_success "All backend tests passed"
    else
        print_error "Some backend tests failed"
        ((ERRORS++))
    fi
    echo ""
fi

# Linting
if [ "$RUN_LINT" = true ]; then
    print_section "Running linters..."
    cd "$PROJECT_ROOT/backend"
    
    # go vet
    if CGO_CFLAGS_ALLOW='-fno-strict-overflow' go vet ./...; then
        print_success "go vet: clean"
    else
        print_error "go vet: issues found"
        ((ERRORS++))
    fi
    
    # gofmt
    UNFORMATTED=$(gofmt -l .)
    if [ -z "$UNFORMATTED" ]; then
        print_success "gofmt: all files formatted"
    else
        print_error "gofmt: unformatted files found:"
        echo "$UNFORMATTED" | sed 's/^/    /'
        ((ERRORS++))
    fi
    echo ""
fi

# Tray App Build
if [ "$RUN_BUILD" = true ]; then
    print_section "Building tray application..."
    cd "$PROJECT_ROOT/trayapp"
    
    if [ ! -f "Makefile" ]; then
        echo "  Running cmake..."
        if ! cmake . > /dev/null 2>&1; then
            print_error "CMake configuration failed"
            ((ERRORS++))
            echo ""
        fi
    fi
    
    if [ -f "Makefile" ]; then
        if make > /dev/null 2>&1; then
            print_success "Tray app built successfully"
        else
            print_error "Tray app build failed"
            ((ERRORS++))
        fi
    fi
    echo ""
fi

# Integration Tests
if [ "$RUN_INTEGRATION" = true ]; then
    print_section "Running integration tests..."
    
    if [ -f "$PROJECT_ROOT/scripts/test-integration.sh" ]; then
        if bash "$PROJECT_ROOT/scripts/test-integration.sh"; then
            print_success "Integration tests passed"
        else
            print_error "Integration tests failed"
            ((ERRORS++))
        fi
    else
        print_warning "Integration test script not found"
    fi
    echo ""
fi

# Summary
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Summary:${NC}"

if [ "$RUN_BUILD" = true ]; then
    echo "  ✓ Backend build"
    echo "  ✓ Tray app build"
fi

if [ "$RUN_TESTS" = true ]; then
    echo "  ✓ Unit tests"
fi

if [ "$RUN_LINT" = true ]; then
    echo "  ✓ Linting"
fi

if [ "$RUN_INTEGRATION" = true ]; then
    echo "  ✓ Integration tests"
fi

echo ""

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}All checks passed!${NC} 🎉"
    exit 0
else
    echo -e "${RED}$ERRORS error(s) found.${NC}"
    exit 1
fi
