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
