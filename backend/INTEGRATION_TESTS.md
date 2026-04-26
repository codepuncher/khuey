# Integration Tests

This directory contains integration tests for KDE Hue Control using a mock Hue bridge.

## Running Integration Tests

Integration tests are disabled by default and must be explicitly enabled with a build tag:

```bash
# Run all integration tests
go test -tags=integration -v

# Run specific integration test
go test -tags=integration -v -run TestHueClient_GetScenes

# Run with timeout
go test -tags=integration -v -timeout 30s
```

## Test Structure

### Mock Bridge (`internal/testutil/mock_bridge.go`)

The `MockBridge` simulates a Philips Hue bridge HTTP API for testing without hardware:

- **TLS Support**: Uses `httptest.NewTLSServer` for HTTPS mocking
- **Request Logging**: Tracks all API calls for verification
- **Error Injection**: Can simulate bridge errors via `SetResponseError()`
- **Configurable Data**: Add scenes, lights, rooms, zones dynamically

**Example Usage:**

```go
mock := testutil.NewMockBridgeTLS()
defer mock.Close()
mock.SetupDefaultScenario() // Adds 3 scenes, 3 lights, 3 rooms, 1 zone

bridgeAddr := mock.URL()[8:] // Remove "https://"
client, _ := hue.NewClient(ctx, bridgeAddr, "test-api-key")
```

### Integration Test Coverage

**Current Tests (integration_test.go):**

1. **TestHueClient_GetScenes** - Scene retrieval workflow
2. **TestHueClient_ActivateScene** - Scene activation workflow
3. **TestHueClient_ErrorHandling** - Bridge unreachable, invalid IDs
4. **TestConfig_DefaultAndConfiguredStates** - Config state management
5. **TestConcurrency_GetScenes** - Concurrent API requests (10 parallel)
6. **TestConcurrency_SceneActivation** - Concurrent scene activation (5 parallel)
7. **TestMockBridge_RequestLogging** - Request tracking verification
8. **TestMockBridge_Setup** - Default scenario validation

## Design Decisions

### Why Build Tags?

Integration tests are isolated with `//go:build integration` because they:
- Are slower than unit tests (TLS handshakes, HTTP requests)
- Test cross-package integration, not single units
- Should not run in CI on every commit (expensive)

### Why Mock Bridge vs Real Bridge?

**Mock Bridge Advantages:**
- **Fast**: No network latency, instant responses
- **Reliable**: No hardware dependencies, always available
- **Controlled**: Can simulate error conditions easily
- **Parallel Safe**: Each test gets its own isolated mock

**Real Bridge Testing:**
- Use `scripts/test-integration.sh` for manual E2E testing with real hardware
- Not suitable for automated CI/CD

### TLS Handling

The mock bridge uses `httptest.NewTLSServer()` with self-signed certificates. Tests must configure:

```go
http.DefaultTransport = &http.Transport{
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}
```

This is acceptable for tests because:
1. Mock server is localhost-only
2. Certificate validation doesn't add test value
3. Real bridge uses self-signed certs anyway

## Adding New Tests

### Test a New API Endpoint

1. **Add handler to MockBridge:**

```go
// In mock_bridge.go
case r.Method == "GET" && r.URL.Path == "/clip/v2/resource/RESOURCE":
    mb.handleGetResource(w, r)
```

2. **Implement handler:**

```go
func (mb *MockBridge) handleGetResource(w http.ResponseWriter, r *http.Request) {
    // Return mock API response
}
```

3. **Write integration test:**

```go
func TestHueClient_GetResource(t *testing.T) {
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.AddResource("res-1", "Test Resource")

    client := setupTestClient(t, mock.URL()[8:], "test-key")
    resources, err := client.GetResources()
    // assertions...
}
```

### Test Error Scenarios

```go
mock.SetResponseError("/clip/v2/resource/scene", fmt.Errorf("bridge error"))
_, err := client.GetScenes()
// Verify error handling
```

### Test Concurrency

```go
errs := make(chan error, 10)
for i := 0; i < 10; i++ {
    go func() {
        _, err := client.SomeMethod()
        errs <- err
    }()
}
// Collect and verify results
```

## Future Enhancements

### Phase 2 (Planned):
- DBus service integration tests (test full backend<->tray workflow)
- Entertainment API mock server and streaming tests
- Config file loading and migration tests
- Gaming mode detector integration tests

### Phase 3 (Planned):
- End-to-end workflow tests (scene activation → light control)
- Performance regression detection in integration tests
- Load testing (sustained concurrent requests)
- Mock bridge fuzz testing

## Troubleshooting

### Tests Hang on TLS Handshake

**Symptom:** Tests timeout waiting for TLS handshake

**Solution:** Ensure `http.DefaultTransport` is set with `InsecureSkipVerify: true` before creating client

### "use of closed network connection" Errors

**Symptom:** TLS handshake errors in logs after tests pass

**Cause:** Mock server closes while rate-limited requests are still queued

**Impact:** Cosmetic only - tests still pass, can be ignored

### Rate Limit Errors

**Symptom:** `rate limit error: context deadline exceeded`

**Cause:** Context timeout shorter than rate limiter wait time

**Solution:** Use longer context timeout or test without rate limiting

## CI Integration

### GitHub Actions Workflow (Planned)

```yaml
name: Integration Tests

on:
  pull_request:
    branches: [ main ]
  workflow_dispatch: # Manual trigger only

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - name: Run Integration Tests
        run: |
          cd backend
          go test -tags=integration -v -timeout 5m
```

**Recommendation:** Run integration tests:
- On PR merge (not every push)
- Nightly builds
- Manual trigger for debugging

---

**Test Coverage**: Phase 1 complete with 8 integration tests covering core Hue API workflows and concurrency patterns.

## DBus Integration Tests (Phase 2)

Added in PR #52: DBus service integration tests that validate backend <-> tray app communication without requiring the Qt tray application.

### Test Coverage

**DBus Service Tests (10 tests):**
1. **TestDBusService_StartAndRegister** - Service registration on session bus
2. **TestDBusService_GetStatus** - Status method returns correct state
3. **TestDBusService_GetScenes** - Scene listing via DBus
4. **TestDBusService_ActivateScene** - Scene activation workflow
5. **TestDBusService_SyncMethods** - Sync settings and state queries
6. **TestDBusService_GamingMode** - Gaming mode enable/active state
7. **TestDBusService_BridgeSettings** - Bridge configuration retrieval
8. **TestDBusService_TrayIcons** - Icon name retrieval for tray app
9. **TestDBusService_Introspection** - DBus introspection XML validation
10. **TestDBusService_ConcurrentCalls** - 10 parallel DBus method calls

**What's Tested:**
- DBus service lifecycle (start/stop)
- Method signatures and return values
- Concurrent method calls (thread safety)
- Introspection data completeness
- Default configuration values
- Integration with mock Hue bridge

**What's NOT Tested:**
- Screen Sync (requires Entertainment API + user permission dialog)
- Gaming Mode activation (requires real game process detection)
- Grouped light control (requires proper room/zone setup in mock)

### Running DBus Tests

```bash
# Stop any running hue-sync first
systemctl --user stop hue-backend

# Run DBus tests only
cd backend
go test -tags=integration -v -run TestDBusService

# All tests should pass in ~0.25 seconds
```

**If hue-sync is running:**
Tests will detect it and skip with message: "Service already running - stop hue-sync before testing"

### How It Works

**Test Setup:**
1. Creates mock Hue bridge with TLS
2. Creates Hue client pointing to mock
3. Creates DBus service with test config
4. Starts service on session bus

**Test Execution:**
1. Connect to session bus as client
2. Call DBus methods via `godbus` library
3. Verify responses match expectations
4. Check mock bridge received requests

**Test Cleanup:**
1. Stop DBus service
2. Close mock bridge
3. Restore HTTP transport

### Architecture

```
Test Process
│
├─ Mock Bridge (HTTPS server)
│  └─ Returns test data (3 scenes, 3 rooms, etc.)
│
├─ DBus Service (backend/internal/dbus)
│  ├─ Registered as org.kde.plasma.hue
│  ├─ Connected to mock bridge
│  └─ Exports methods on session bus
│
└─ Test Client (godbus connection)
   └─ Calls methods, verifies responses
```

### Key Features

**Session Bus Integration:**
- Uses real DBus session bus (not mock)
- Tests actual IPC mechanism
- Validates service registration
- Tests introspection works

**Thread Safety:**
- Tests 10 concurrent method calls
- Verifies no race conditions
- Confirms mutex protection works

**Service Lifecycle:**
- Tests Start() registration
- Tests Stop() cleanup
- Verifies name release

### Example Test

```go
func TestDBusService_GetScenes(t *testing.T) {
    // Setup service with mock bridge
    service, mock, cleanup := setupTestDBusService(t)
    defer cleanup()

    err := service.Start()
    if err != nil {
        t.Fatalf("Failed to start service: %v", err)
    }

    // Connect as DBus client
    conn, _ := godbus.ConnectSessionBus()
    obj := conn.Object("org.kde.plasma.hue", "/org/kde/plasma/hue")

    // Call method
    var scenes []string
    err = obj.Call("org.kde.plasma.hue.GetScenes", 0).Store(&scenes)

    // Verify
    if len(scenes) != 3 {
        t.Errorf("Got %d scenes, want 3", len(scenes))
    }
}
```

### Troubleshooting

**"Service already running"**
- Stop hue-sync: `systemctl --user stop hue-backend`
- Or run without DBus tests: `go test -tags=integration -run TestHue`

**"Failed to connect to session bus"**
- Ensure DBUS_SESSION_BUS_ADDRESS is set
- Check `dbus-daemon --session` is running
- Try: `systemctl --user status dbus`

**Tests hang on Stop()**
- Normal - service cleanup can take 200ms
- Tests include timeout: `-timeout 30s`

### Phase 2 vs Phase 1

**Phase 1 (Mock Bridge):**
- Tests Hue API client
- HTTP/TLS mocking
- No system dependencies

**Phase 2 (DBus Service):**
- Tests DBus service layer
- Real session bus required
- Tests IPC mechanism
- Validates method signatures

**Phase 3 (Planned):**
- End-to-end workflows
- Entertainment API mocking
- Full tray app simulation
