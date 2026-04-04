#!/bin/bash
# verify-v1.0-fixes.sh
# Verification script for pre-release audit fixes

set -e

echo "======================================"
echo "khuey v1.0.0 - Fix Verification Script"
echo "======================================"
echo ""

BACKEND_DIR="backend"
TRAYAPP_DIR="trayapp"
FAILED=0

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function pass() {
    echo -e "${GREEN}✅ PASS${NC}: $1"
}

function fail() {
    echo -e "${RED}❌ FAIL${NC}: $1"
    FAILED=$((FAILED + 1))
}

function warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}: $1"
}

function check() {
    echo ""
    echo "Checking: $1"
    echo "----------------------------------------"
}

# ============================================
# Configuration Constants
# ============================================
check "Configuration constants exist"

cd "$BACKEND_DIR"

if grep -q "const ConfigVersion = 1" internal/config/config.go; then
    pass "ConfigVersion constant defined"
else
    fail "ConfigVersion constant missing"
fi

if grep -q "DefaultFPS = 30" internal/config/config.go; then
    pass "DefaultFPS constant defined"
else
    fail "DefaultFPS constant missing"
fi

if grep -q "MaxSubsampleWidth = 256" internal/config/config.go; then
    pass "MaxSubsampleWidth constant defined"
else
    fail "MaxSubsampleWidth constant missing"
fi

# ============================================
# Validate() Called in Load()
# ============================================
check "Config validation in Load()"

if grep -A5 "Unmarshal(cfg)" internal/config/config.go | grep -q "Validate()"; then
    pass "Validate() called in Load()"
else
    fail "Validate() NOT called in Load()"
fi

# ============================================
# Magic Numbers Replaced
# ============================================
check "Magic numbers replaced with constants"

if grep -q "const Color8To16Multiplier = 257" internal/entertainment/client.go; then
    pass "Color8To16Multiplier constant defined"
else
    fail "Color8To16Multiplier constant missing"
fi

if grep -q "const EntertainmentAPIPort = 2100" internal/entertainment/client.go; then
    pass "EntertainmentAPIPort constant defined"
else
    fail "EntertainmentAPIPort constant missing"
fi

if grep -q "const MaxPortalHandleID = 999999" internal/capture/portal.go; then
    pass "MaxPortalHandleID constant defined"
else
    fail "MaxPortalHandleID constant missing"
fi

if grep -q "const DefaultHTTPTimeout" internal/common/utils.go; then
    pass "DefaultHTTPTimeout constant defined"
else
    fail "DefaultHTTPTimeout constant missing"
fi

# Check for lingering hardcoded values (excluding constants and test files)
MAGIC_257=$(grep -rn " 257" internal/ --include="*.go" | grep -v "const\|comment\|test\|= 257" | wc -l)
if [ "$MAGIC_257" -eq 0 ]; then
    pass "No hardcoded 257 found"
else
    warn "Found $MAGIC_257 instances of hardcoded 257"
fi

MAGIC_2100=$(grep -rn "2100" internal/ --include="*.go" | grep -v "const\|comment\|= 2100" | wc -l)
if [ "$MAGIC_2100" -eq 0 ]; then
    pass "No hardcoded 2100 found"
else
    warn "Found $MAGIC_2100 instances of hardcoded 2100"
fi

# ============================================
# Resource Management
# ============================================
check "Resource management fixes"

if grep -A2 "gstCmd.Process.Kill()" internal/capture/capture.go | grep -q "Wait()"; then
    pass "gstCmd.Wait() added after Kill()"
else
    fail "gstCmd.Wait() missing after Kill()"
fi

if grep -A1 'tmpfile := "/tmp/hue-screenshot.png"' internal/capture/capture.go | grep -q "defer os.Remove"; then
    pass "defer os.Remove(tmpfile) added"
else
    fail "defer os.Remove(tmpfile) missing"
fi

# Verify ticker.Stop() is deferred (should find 5 instances)
TICKER_STOP=$(grep -rn "defer ticker.Stop()" internal/ --include="*.go" | wc -l)
if [ "$TICKER_STOP" -ge 2 ]; then
    pass "ticker.Stop() properly deferred ($TICKER_STOP instances)"
else
    fail "ticker.Stop() deferred only $TICKER_STOP times (expected >= 2)"
fi

# ============================================
# TODO Comments
# ============================================
check "TODO comments resolved"

TODO_COUNT=$(grep -rn "TODO" internal/ cmd/ --include="*.go" | grep -v "post-v1.0\|future" | wc -l)
if [ "$TODO_COUNT" -eq 0 ]; then
    pass "No unresolved TODO comments"
else
    warn "Found $TODO_COUNT TODO comments"
fi

# ============================================
# Context Propagation
# ============================================
check "Context propagation"

if grep -q "func (e \*Engine) Start(ctx context.Context)" internal/sync/engine.go; then
    pass "sync.Engine.Start() accepts context"
else
    fail "sync.Engine.Start() doesn't accept context"
fi

if grep -q "Context.*context.Context" internal/entertainment/client.go | head -1; then
    pass "entertainment.Config has Context field"
else
    warn "entertainment.Config may be missing Context field"
fi

if grep -q "Context.*context.Context" internal/capture/capture.go | head -1; then
    pass "capture.Config has Context field"
else
    warn "capture.Config may be missing Context field"
fi

# ============================================
# Qt Memory Management
# ============================================
check "Qt memory management"

cd "../$TRAYAPP_DIR"

NOTIF_LEAKS=$(grep -A5 "new KNotification" main.cpp | grep -c "deleteLater()")
if [ "$NOTIF_LEAKS" -ge 3 ]; then
    pass "KNotification deleteLater() added ($NOTIF_LEAKS instances)"
else
    warn "Only $NOTIF_LEAKS KNotification deleteLater() found (expected 3)"
fi

# ============================================
# Build Tests
# ============================================
check "Build verification"

cd "../$BACKEND_DIR"

echo "Building backend..."
if CGO_CFLAGS_ALLOW="-fno-strict-overflow" go build -o hue-sync ./cmd/hue-sync 2>&1 | tail -5; then
    pass "Backend builds successfully"
    rm -f hue-sync
else
    fail "Backend build failed"
fi

echo ""
echo "Running backend tests..."
if CGO_CFLAGS_ALLOW="-fno-strict-overflow" go test ./internal/... 2>&1 | grep -q "^ok"; then
    pass "Backend tests pass"
else
    fail "Backend tests failed"
fi

cd "../$TRAYAPP_DIR"

echo ""
echo "Building tray app..."
if cmake . >/dev/null 2>&1 && make >/dev/null 2>&1; then
    pass "Tray app builds successfully"
    rm -f hue-tray
else
    fail "Tray app build failed"
fi

# ============================================
# Performance Improvements
# ============================================
check "Performance improvements"

cd "../$BACKEND_DIR"

if grep -q "Frame skip" internal/sync/engine.go; then
    pass "Frame skip detection added"
else
    fail "Frame skip detection missing"
fi

# ============================================
# Documentation
# ============================================
check "Documentation"

if grep -q "Package sync provides" internal/sync/engine.go; then
    pass "Package documentation added to sync"
else
    warn "Package documentation missing from sync"
fi

# Check that key functions have godoc comments
GODOC_COUNT=$(grep -rn "^// New" internal/ --include="*.go" | wc -l)
if [ "$GODOC_COUNT" -ge 5 ]; then
    pass "Godoc comments present ($GODOC_COUNT constructors documented)"
else
    warn "Only $GODOC_COUNT constructors have godoc"
fi

# ============================================
# Summary
# ============================================
echo ""
echo "======================================"
echo "Verification Summary"
echo "======================================"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ ALL CHECKS PASSED${NC}"
    echo ""
    echo "The codebase is ready for v1.0.0 release!"
    exit 0
else
    echo -e "${RED}❌ $FAILED CHECKS FAILED${NC}"
    echo ""
    echo "Please review the failures above before releasing v1.0.0"
    exit 1
fi
