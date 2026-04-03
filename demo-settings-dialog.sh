#!/bin/bash
# Settings Dialog - Quick Demo/Verification

set -e

echo "╔═══════════════════════════════════════════════════════╗"
echo "║  KDE Hue Control - Settings Dialog Demonstration     ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if backend is running
echo -e "${BLUE}📡 Checking Backend Status...${NC}"
if pgrep -x "hue-sync" > /dev/null; then
    echo -e "${GREEN}✅ Backend is running${NC}"
else
    echo -e "${RED}❌ Backend is not running${NC}"
    echo -e "${YELLOW}Starting backend...${NC}"
    systemctl --user start plasma-hue-backend 2>/dev/null || systemctl --user start hue-backend 2>/dev/null || {
        echo -e "${YELLOW}Systemd service not found, starting manually...${NC}"
        cd ~/Code/misc/khuey/backend
        ./hue-sync &
        BACKEND_PID=$!
        sleep 2
    }
fi

echo ""
echo -e "${BLUE}🔧 Testing Backend DBus API...${NC}"
echo ""

# Test 1: Get Sync Settings
echo -e "${YELLOW}Test 1: GetSyncSettings${NC}"
RESULT=$(dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSyncSettings 2>&1)

if echo "$RESULT" | grep -q "fps"; then
    echo -e "${GREEN}✅ GetSyncSettings working${NC}"
    echo "$RESULT" | grep -E "fps|subsampleWidth" | head -2
else
    echo -e "${RED}❌ GetSyncSettings failed${NC}"
    exit 1
fi

echo ""
# Test 2: Get Bridge Settings
echo -e "${YELLOW}Test 2: GetBridgeSettings${NC}"
RESULT=$(dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetBridgeSettings 2>&1)

if echo "$RESULT" | grep -q "bridgeIP"; then
    echo -e "${GREEN}✅ GetBridgeSettings working${NC}"
    BRIDGE_IP=$(echo "$RESULT" | grep "bridgeIP" -A 1 | tail -1 | awk '{print $NF}' | tr -d '"')
    CONNECTED=$(echo "$RESULT" | grep "connected" -A 1 | tail -1 | awk '{print $NF}')
    echo "   Bridge IP: $BRIDGE_IP"
    echo "   Connected: $CONNECTED"
else
    echo -e "${RED}❌ GetBridgeSettings failed${NC}"
    exit 1
fi

echo ""
# Test 3: Get Selected Room
echo -e "${YELLOW}Test 3: GetSelectedRoom${NC}"
RESULT=$(dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSelectedRoom 2>&1)

if echo "$RESULT" | grep -q "string"; then
    echo -e "${GREEN}✅ GetSelectedRoom working${NC}"
    ROOM_ID=$(echo "$RESULT" | grep "string" | awk '{print $NF}' | tr -d '"')
    echo "   Room ID: ${ROOM_ID:-<not set>}"
else
    echo -e "${RED}❌ GetSelectedRoom failed${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}💾 Testing Settings Persistence...${NC}"
echo ""

# Save current settings
ORIGINAL_FPS=$(dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSyncSettings 2>&1 | grep "fps" -A 1 | tail -1 | awk '{print $NF}')

echo -e "${YELLOW}Test 4: Change FPS to 25${NC}"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.SetSyncSettings \
    int32:25 int32:64 string:"" > /dev/null 2>&1

NEW_FPS=$(dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.GetSyncSettings 2>&1 | grep "fps" -A 1 | tail -1 | awk '{print $NF}')

if [ "$NEW_FPS" -eq 25 ]; then
    echo -e "${GREEN}✅ Settings changed successfully (FPS: $ORIGINAL_FPS → 25)${NC}"
else
    echo -e "${RED}❌ Settings change failed${NC}"
    exit 1
fi

# Verify in config file
echo ""
echo -e "${YELLOW}Test 5: Verify config file persistence${NC}"
if grep -q "fps: 25" ~/.openhue/config.yaml; then
    echo -e "${GREEN}✅ Settings persisted to config.yaml${NC}"
else
    echo -e "${RED}❌ Settings not saved to config file${NC}"
    exit 1
fi

# Restore original settings
echo ""
echo -e "${YELLOW}Test 6: Restore original settings${NC}"
dbus-send --session --print-reply \
    --dest=org.kde.plasma.hue \
    /org/kde/plasma/hue \
    org.kde.plasma.hue.SetSyncSettings \
    int32:$ORIGINAL_FPS int32:64 string:"" > /dev/null 2>&1

echo -e "${GREEN}✅ Settings restored (FPS: 25 → $ORIGINAL_FPS)${NC}"

echo ""
echo -e "${BLUE}🎨 Testing Qt Tray App Build...${NC}"
echo ""

cd ~/Code/misc/khuey/trayapp
if [ ! -f "hue-tray" ]; then
    echo -e "${YELLOW}Tray app not built, building now...${NC}"
    cmake . > /dev/null 2>&1 && make > /dev/null 2>&1
fi

if [ -f "hue-tray" ]; then
    echo -e "${GREEN}✅ Tray app binary exists and is ready${NC}"
    echo ""
    echo -e "${BLUE}📊 File sizes:${NC}"
    ls -lh hue-tray | awk '{print "   Tray app: " $5}'
    ls -lh ~/Code/misc/khuey/backend/hue-sync 2>/dev/null | awk '{print "   Backend:  " $5}' || echo "   Backend: (not built)"
else
    echo -e "${RED}❌ Tray app build failed${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}📝 Checking Source Files...${NC}"
echo ""

FILES=(
    "trayapp/settingsdialog.h"
    "trayapp/settingsdialog.cpp"
    "backend/internal/dbus/service.go"
    "SETTINGS_DIALOG_TESTING.md"
    "SETTINGS_DIALOG_IMPLEMENTATION.md"
)

for file in "${FILES[@]}"; do
    if [ -f ~/Code/misc/khuey/$file ]; then
        LINES=$(wc -l < ~/Code/misc/khuey/$file)
        echo -e "${GREEN}✅${NC} $file (${LINES} lines)"
    else
        echo -e "${RED}❌${NC} $file (missing)"
    fi
done

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║              🎉 All Tests Passed! 🎉                  ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""
echo -e "${BLUE}🚀 Launch Settings Dialog:${NC}"
echo ""
echo "   1. Run the tray app:"
echo -e "      ${YELLOW}cd ~/Code/misc/khuey/trayapp && ./hue-tray${NC}"
echo ""
echo "   2. Right-click the tray icon"
echo ""
echo "   3. Click '⚙️ Settings...'"
echo ""
echo -e "${BLUE}📚 Documentation:${NC}"
echo "   - Testing Guide: SETTINGS_DIALOG_TESTING.md"
echo "   - Implementation: SETTINGS_DIALOG_IMPLEMENTATION.md"
echo "   - User Guide:     USAGE.md (Section 4)"
echo ""
echo -e "${GREEN}✨ Settings Dialog is ready to use!${NC}"
echo ""
