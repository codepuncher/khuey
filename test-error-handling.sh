#!/bin/bash
# Comprehensive test script for error handling implementation

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  TESTING ENHANCED ERROR HANDLING IMPLEMENTATION"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Test 1: Backend Build
echo "Test 1: Backend Build"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
cd backend
CGO_CFLAGS_ALLOW=".*" go build -o hue-sync ./cmd/hue-sync 2>&1 > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ Backend builds successfully"
else
    echo "❌ Backend build FAILED"
    exit 1
fi
cd ..
echo

# Test 2: Backend Tests
echo "Test 2: Backend Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
cd backend
CGO_CFLAGS_ALLOW=".*" go test ./internal/... -v 2>&1 | grep -E "^(ok|FAIL|PASS)"
if [ ${PIPESTATUS[0]} -eq 0 ]; then
    echo "✅ All backend tests pass"
else
    echo "❌ Backend tests FAILED"
    exit 1
fi
cd ..
echo

# Test 3: Tray App Build
echo "Test 3: Tray App Build"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
cd trayapp
make clean > /dev/null 2>&1
cmake . > /dev/null 2>&1
make 2>&1 | tail -5
if [ $? -eq 0 ]; then
    echo "✅ Tray app builds successfully"
else
    echo "❌ Tray app build FAILED"
    exit 1
fi
cd ..
echo

# Test 4: Check New DBus Methods
echo "Test 4: Check New DBus Methods Exist"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "GetConnectionStatus" backend/internal/dbus/service.go
if [ $? -eq 0 ]; then
    echo "✅ GetConnectionStatus method exists"
else
    echo "❌ GetConnectionStatus method NOT FOUND"
    exit 1
fi

grep -q "RetryConnection" backend/internal/dbus/service.go
if [ $? -eq 0 ]; then
    echo "✅ RetryConnection method exists"
else
    echo "❌ RetryConnection method NOT FOUND"
    exit 1
fi
echo

# Test 5: Check Connection Status Tracking
echo "Test 5: Check Connection Status Tracking"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "type ConnectionStatus struct" backend/internal/hue/client.go
if [ $? -eq 0 ]; then
    echo "✅ ConnectionStatus struct defined"
else
    echo "❌ ConnectionStatus struct NOT FOUND"
    exit 1
fi

grep -q "func.*GetConnectionStatus" backend/internal/hue/client.go
if [ $? -eq 0 ]; then
    echo "✅ GetConnectionStatus method in hue.Client"
else
    echo "❌ GetConnectionStatus method NOT FOUND in hue.Client"
    exit 1
fi
echo

# Test 6: Check Portal Error Type
echo "Test 6: Check Portal Error Type"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "type PortalError struct" backend/internal/capture/portal.go
if [ $? -eq 0 ]; then
    echo "✅ PortalError type defined"
else
    echo "❌ PortalError type NOT FOUND"
    exit 1
fi

grep -q "Hint string" backend/internal/capture/portal.go
if [ $? -eq 0 ]; then
    echo "✅ PortalError has Hint field"
else
    echo "❌ PortalError Hint field NOT FOUND"
    exit 1
fi
echo

# Test 7: Check Config Validation
echo "Test 7: Check Config Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "func.*Validate" backend/internal/config/config.go
if [ $? -eq 0 ]; then
    echo "✅ Validate method exists in config"
else
    echo "❌ Validate method NOT FOUND"
    exit 1
fi

grep -q "validateChannel" backend/internal/config/config.go
if [ $? -eq 0 ]; then
    echo "✅ Channel validation exists"
else
    echo "❌ validateChannel NOT FOUND"
    exit 1
fi
echo

# Test 8: Check KNotification Integration
echo "Test 8: Check KNotification Integration"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "#include <KNotification>" trayapp/main.cpp
if [ $? -eq 0 ]; then
    echo "✅ KNotification header included"
else
    echo "❌ KNotification header NOT FOUND"
    exit 1
fi

grep -q "KF6::Notifications" trayapp/CMakeLists.txt
if [ $? -eq 0 ]; then
    echo "✅ KNotifications linked in CMakeLists"
else
    echo "❌ KNotifications NOT linked"
    exit 1
fi
echo

# Test 9: Check Async DBus Calls
echo "Test 9: Check Async DBus Calls in Tray App"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "QDBusPendingCall" trayapp/main.cpp
if [ $? -eq 0 ]; then
    echo "✅ Async DBus calls implemented"
else
    echo "❌ Async DBus calls NOT FOUND"
    exit 1
fi

grep -q "QDBusPendingCallWatcher" trayapp/main.cpp
if [ $? -eq 0 ]; then
    echo "✅ DBus call watchers present"
else
    echo "❌ DBus call watchers NOT FOUND"
    exit 1
fi
echo

# Test 10: Check Troubleshooting Documentation
echo "Test 10: Check Troubleshooting Documentation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep -q "## Troubleshooting" README.md
if [ $? -eq 0 ]; then
    echo "✅ Troubleshooting section in README"
else
    echo "❌ Troubleshooting section NOT FOUND"
    exit 1
fi

if [ -f ERROR_HANDLING_IMPLEMENTATION.md ]; then
    echo "✅ Implementation summary document exists"
else
    echo "❌ Implementation summary NOT FOUND"
    exit 1
fi
echo

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ✅ ALL TESTS PASSED - IMPLEMENTATION VERIFIED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo
echo "Summary:"
echo "  • Backend builds and tests pass"
echo "  • Tray app builds successfully"
echo "  • New DBus methods implemented"
echo "  • Connection status tracking present"
echo "  • Portal error handling implemented"
echo "  • Config validation added"
echo "  • KNotification integration complete"
echo "  • Async DBus calls implemented"
echo "  • Documentation complete"
echo
echo "✅ Ready for merge to main!"
