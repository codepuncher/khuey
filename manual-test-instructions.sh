#!/bin/bash

# Simple manual test for Screen Sync restart
# This will show you what commands to run manually

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== Manual Screen Sync Restart Test ===${NC}"
echo ""
echo "Follow these steps manually:"
echo ""
echo -e "${YELLOW}1. Check initial state:${NC}"
echo "   dbus-send --session --print-reply --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing"
echo "   Expected: boolean false"
echo ""
echo -e "${YELLOW}2. Start Screen Sync (first time):${NC}"
echo "   dbus-send --session --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.StartSync"
echo "   ⚠️  GUI DIALOG WILL APPEAR - APPROVE IT"
echo "   Wait 5 seconds after approving"
echo ""
echo -e "${YELLOW}3. Verify it's running:${NC}"
echo "   dbus-send --session --print-reply --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing"
echo "   Expected: boolean true"
echo ""
echo -e "${YELLOW}4. Stop Screen Sync:${NC}"
echo "   dbus-send --session --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.StopSync"
echo "   Wait 2 seconds"
echo ""
echo -e "${YELLOW}5. Verify it's stopped:${NC}"
echo "   dbus-send --session --print-reply --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing"
echo "   Expected: boolean false"
echo ""
echo -e "${YELLOW}6. Start Screen Sync (SECOND TIME - THE CRITICAL TEST):${NC}"
echo "   dbus-send --session --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.StartSync"
echo "   ⚠️  GUI DIALOG MAY APPEAR AGAIN - APPROVE IT"
echo "   Wait 5 seconds"
echo ""
echo -e "${YELLOW}7. Verify it's running again:${NC}"
echo "   dbus-send --session --print-reply --dest=org.kde.plasma.hue /org/kde/plasma/hue org.kde.plasma.hue.IsSyncing"
echo "   Expected: boolean true"
echo ""
echo -e "${GREEN}If step 7 returns 'boolean true' - THE BUG IS FIXED!${NC}"
echo ""
echo "To watch backend logs in another terminal:"
echo "   journalctl --user -u hue-backend -f"
