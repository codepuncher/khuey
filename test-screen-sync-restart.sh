#!/bin/bash

# Test script for Screen Sync restart issue
# Tests: Start → Stop → Start → Stop → Start cycle

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Screen Sync Restart Test ===${NC}"
echo ""

# Function to call DBus method and check result
call_dbus() {
    local method=$1
    local expected=$2
    echo -e "${YELLOW}Calling: $method${NC}"
    
    result=$(dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        org.kde.plasma.hue.$method 2>&1)
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}✗ DBus call failed!${NC}"
        echo "$result"
        return 1
    fi
    
    echo "$result"
    
    if [ -n "$expected" ]; then
        if echo "$result" | grep -q "$expected"; then
            echo -e "${GREEN}✓ Got expected result${NC}"
        else
            echo -e "${RED}✗ Unexpected result (expected: $expected)${NC}"
            return 1
        fi
    fi
    
    return 0
}

check_syncing() {
    local expected=$1
    echo -e "${YELLOW}Checking IsSyncing (expect: $expected)${NC}"
    
    result=$(dbus-send --session --print-reply \
        --dest=org.kde.plasma.hue \
        /org/kde/plasma/hue \
        org.kde.plasma.hue.IsSyncing 2>&1)
    
    if echo "$result" | grep -q "boolean $expected"; then
        echo -e "${GREEN}✓ IsSyncing = $expected${NC}"
        return 0
    else
        echo -e "${RED}✗ IsSyncing != $expected${NC}"
        echo "$result"
        return 1
    fi
}

echo "Step 1: Check backend status"
call_dbus "GetStatus" "Ready"
echo ""

echo "Step 2: Verify NOT syncing initially"
check_syncing "false"
echo ""

echo "Step 3: START Screen Sync (1st time)"
echo -e "${YELLOW}⚠️  A GUI dialog will appear - please approve it!${NC}"
call_dbus "StartSync"
sleep 3
check_syncing "true"
echo -e "${GREEN}✓ First start successful${NC}"
echo ""

echo "Step 4: STOP Screen Sync"
call_dbus "StopSync"
sleep 2
check_syncing "false"
echo -e "${GREEN}✓ Stop successful${NC}"
echo ""

echo "Step 5: START Screen Sync (2nd time) - CRITICAL TEST"
echo -e "${YELLOW}⚠️  This is where the bug was - testing the fix...${NC}"
echo -e "${YELLOW}⚠️  The GUI dialog may appear again - please approve it!${NC}"
call_dbus "StartSync"
sleep 3
check_syncing "true"
echo -e "${GREEN}✓✓✓ Second start successful - BUG FIXED! ✓✓✓${NC}"
echo ""

echo "Step 6: STOP Screen Sync again"
call_dbus "StopSync"
sleep 2
check_syncing "false"
echo -e "${GREEN}✓ Second stop successful${NC}"
echo ""

echo "Step 7: START Screen Sync (3rd time) - Extra verification"
echo -e "${YELLOW}⚠️  Testing one more cycle...${NC}"
call_dbus "StartSync"
sleep 3
check_syncing "true"
echo -e "${GREEN}✓✓✓ Third start successful - Multiple cycles work! ✓✓✓${NC}"
echo ""

echo "Step 8: Final STOP"
call_dbus "StopSync"
sleep 2
check_syncing "false"
echo -e "${GREEN}✓ Final stop successful${NC}"
echo ""

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}✓ ALL TESTS PASSED!${NC}"
echo -e "${GREEN}✓ Screen Sync can be started/stopped indefinitely${NC}"
echo -e "${GREEN}✓ No backend restart needed${NC}"
echo -e "${GREEN}================================${NC}"
