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

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

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
    ((TESTS_PASSED++))
}

print_fail() {
    echo -e "${RED}  [FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

print_warning() {
    echo -e "${YELLOW}  [WARN]${NC} $1"
}

# DBus helper
call_dbus() {
    local method="$1"
    shift
    dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        "org.kde.plasma.hue.$method" \
        "$@" 2>&1
}

# Test functions
test_service_available() {
    ((TESTS_RUN++))
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
    ((TESTS_RUN++))
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
    ((TESTS_RUN++))
    print_test "GetScenes()"
    
    local result
    result=$(call_dbus "GetScenes")
    
    if echo "$result" | grep -q "array"; then
        local scene_count
        scene_count=$(echo "$result" | grep -c "struct" || echo "0")
        # Remove any non-numeric characters
        scene_count=$(echo "$scene_count" | tr -cd '0-9')
        print_pass "Returned $scene_count scenes"
        
        if [ "${scene_count:-0}" -gt 0 ]; then
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
    ((TESTS_RUN++))
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
    ((TESTS_RUN++))
    print_test "Introspection (DBus interface)"
    
    local result
    result=$(dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        org.freedesktop.DBus.Introspectable.Introspect 2>&1)
    
    if echo "$result" | grep -q "<interface"; then
        print_pass "Introspection data available"
        
        # Check for key methods
        local methods=("GetStatus" "GetScenes" "ActivateScene" "StartSync" "StopSync" "IsSyncing")
        local found=0
        for method in "${methods[@]}"; do
            if echo "$result" | grep -q "method name=\"$method\""; then
                ((found++))
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
    ((TESTS_RUN++))
    print_test "Response time (GetStatus)"
    
    local start
    start=$(date +%s%N)
    call_dbus "GetStatus" > /dev/null 2>&1
    local end
    end=$(date +%s%N)
    
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
    ((TESTS_RUN++))
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

# Run test suite
test_get_status
echo ""

test_get_scenes
echo ""

test_is_syncing
echo ""

test_introspection
echo ""

test_response_time
echo ""

test_error_handling
echo ""

# Print summary
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Test Results:${NC}"
echo -e "  Tests run:    $TESTS_RUN"
echo -e "  ${GREEN}Passed:       $TESTS_PASSED${NC}"

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "  ${RED}Failed:       $TESTS_FAILED${NC}"
else
    echo -e "  ${GREEN}Failed:       $TESTS_FAILED${NC}"
fi

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
