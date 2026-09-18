#!/bin/bash
# DBus Testing Utility for KDE Hue Control
# Tests all DBus methods and validates responses

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters. A test is a function; a check is one assertion inside one.
TESTS_RUN=0
CHECKS_PASSED=0
CHECKS_FAILED=0

# Print functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          KDE Hue Control - DBus Test Suite                ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_test() {
    echo -e "${BLUE}==>${NC} Testing: $1"
}

print_pass() {
    echo -e "${GREEN}  [OK]${NC} $1"
    CHECKS_PASSED=$((CHECKS_PASSED + 1))
}

print_fail() {
    echo -e "${RED}  [FAIL]${NC} $1"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
}

print_warning() {
    echo -e "${YELLOW}  [WARN]${NC} $1"
}

# DBus helper
call_dbus() {
    local method="$1"
    shift
    # Callers assert on the reply text, which is why stderr is merged in. An
    # error reply exits 1, and set -e would kill the script at the caller's
    # assignment before it could report the failure.
    dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        "org.kde.plasma.hue.$method" \
        "$@" 2>&1 || true
}

# Test functions
test_service_available() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "Service availability"
    
    if dbus-send --session --dest=org.freedesktop.DBus \
        --print-reply /org/freedesktop/DBus \
        org.freedesktop.DBus.ListNames 2>/dev/null | grep -q "org.kde.plasma.hue"; then
        print_pass "Service org.kde.plasma.hue is registered"
        return 0
    else
        print_fail "Service org.kde.plasma.hue not found"
        echo ""
        echo "Backend is not running. Start it with:"
        echo "  systemctl --user start hue-backend"
        return 1
    fi
}

test_get_status() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "GetStatus()"
    
    local result
    result=$(call_dbus "GetStatus")
    
    if echo "$result" | grep -q "string"; then
        local status
        status=$(echo "$result" | grep "string" | sed 's/.*string "\(.*\)"/\1/')
        print_pass "Returned: $status"
        
        case "$status" in
            "Ready")
                print_pass "Backend is ready"
                ;;
            "Not Configured")
                print_warning "Backend needs configuration (openhue setup)"
                ;;
            "Bridge Unreachable")
                print_warning "Cannot reach Hue bridge"
                ;;
            *)
                print_warning "Unknown status: $status"
                ;;
        esac
        return 0
    else
        print_fail "No valid response"
        return 1
    fi
}

test_get_scenes() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "GetScenes()"
    
    local result
    result=$(call_dbus "GetScenes")
    
    if echo "$result" | grep -q "array"; then
        local scene_count
        # grep -c exits 1 on no matches, having already printed its 0.
        scene_count=$(echo "$result" | grep -c "string" || true)
        print_pass "Returned $scene_count scenes"
        
        if [ "$scene_count" -gt 0 ]; then
            print_pass "Scene data structure valid"
        else
            print_warning "No scenes found (bridge may need setup)"
        fi
        return 0
    else
        print_fail "No valid response"
        return 1
    fi
}

test_is_syncing() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "IsSyncing()"
    
    local result
    result=$(call_dbus "IsSyncing")
    
    if echo "$result" | grep -q "boolean"; then
        local syncing
        syncing=$(echo "$result" | grep "boolean" | awk '{print $2}')
        print_pass "Returned: $syncing"
        
        if [ "$syncing" = "true" ]; then
            print_pass "Screen sync is currently active"
        else
            print_pass "Screen sync is currently inactive"
        fi
        return 0
    else
        print_fail "No valid response"
        return 1
    fi
}

test_introspection() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "Introspection (DBus interface)"
    
    local result
    result=$(dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        org.freedesktop.DBus.Introspectable.Introspect 2>&1 || true)
    
    if echo "$result" | grep -q "<interface"; then
        print_pass "Introspection data available"
        
        # Check for key methods
        local methods=("GetStatus" "GetScenes" "ActivateScene" "StartSync" "StopSync" "IsSyncing")
        local found=0
        for method in "${methods[@]}"; do
            if echo "$result" | grep -q "method name=\"$method\""; then
                found=$((found + 1))
            fi
        done
        
        if [ $found -eq ${#methods[@]} ]; then
            print_pass "All expected methods present ($found/${#methods[@]})"
        else
            print_warning "Some methods missing ($found/${#methods[@]})"
        fi
        return 0
    else
        print_fail "No valid introspection data"
        return 1
    fi
}

test_response_time() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "Response time (GetStatus)"
    
    local result start end
    start=$(date +%s%N)
    result=$(call_dbus "GetStatus")
    end=$(date +%s%N)
    
    if ! echo "$result" | grep -q "string"; then
        print_fail "No valid response"
        return 1
    fi

    local duration_ns=$((end - start))
    local duration_ms=$((duration_ns / 1000000))
    
    if [ $duration_ms -lt 100 ]; then
        print_pass "Response time: ${duration_ms}ms (excellent)"
    elif [ $duration_ms -lt 500 ]; then
        print_pass "Response time: ${duration_ms}ms (good)"
    elif [ $duration_ms -lt 1000 ]; then
        print_warning "Response time: ${duration_ms}ms (slow)"
    else
        print_warning "Response time: ${duration_ms}ms (very slow)"
    fi
}

test_error_handling() {
    TESTS_RUN=$((TESTS_RUN + 1))
    print_test "Error handling (invalid method)"
    
    local result
    result=$(dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        org.kde.plasma.hue.InvalidMethod 2>&1 || true)
    
    if echo "$result" | grep -q "Error"; then
        print_pass "Invalid method rejected properly"
        return 0
    else
        print_fail "Invalid method not handled"
        return 1
    fi
}

# Run tests
print_header

# Check backend is running first
if ! test_service_available; then
    echo ""
    echo "Cannot run tests without backend running."
    exit 1
fi

echo ""

# Run test suite. Each test records its own result and returns 1 when it fails,
# so the failures have to be tolerated here for the summary below to be reached.
test_get_status || true
echo ""

test_get_scenes || true
echo ""

test_is_syncing || true
echo ""

test_introspection || true
echo ""

test_response_time || true
echo ""

test_error_handling || true
echo ""

# Print summary
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Results:${NC}"
echo -e "  Tests run:      $TESTS_RUN"
echo -e "  ${GREEN}Checks passed:  $CHECKS_PASSED${NC}"

if [ $CHECKS_FAILED -gt 0 ]; then
    echo -e "  ${RED}Checks failed:  $CHECKS_FAILED${NC}"
else
    echo -e "  ${GREEN}Checks failed:  $CHECKS_FAILED${NC}"
fi

if [ $CHECKS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
