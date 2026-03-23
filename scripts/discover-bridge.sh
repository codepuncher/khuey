#!/bin/bash
# Philips Hue Bridge Discovery

echo "=== Philips Hue Bridge Discovery ==="
echo ""

# Check openhue-cli
if command -v openhue &> /dev/null; then
    echo "✓ openhue-cli found! Easiest method:"
    echo "  openhue setup"
    exit 0
fi

# Try Philips discovery service
echo "Querying Philips discovery service..."
curl -s https://discovery.meethue.com/ | python3 -m json.tool 2>/dev/null
echo ""

echo "=== Manual Discovery ==="
echo ""
echo "Try: for i in {2..20}; do curl -k -m 1 https://192.168.1.\$i/api/config 2>/dev/null && echo \"Found at 192.168.1.\$i\"; done"
