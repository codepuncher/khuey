# KDE Hue Control Testing Guide

Comprehensive guide for writing, running, and debugging tests in the khuey codebase.

**Last Updated:** 2025-01-14
**Target Audience:** Contributors, developers adding new features

---

## Table of Contents

1. [Overview & Testing Philosophy](#overview--testing-philosophy)
2. [Test Organization](#test-organization)
3. [Running Tests](#running-tests)
4. [Unit Testing Patterns](#unit-testing-patterns)
5. [Integration Testing Patterns](#integration-testing-patterns)
6. [Benchmark Testing Patterns](#benchmark-testing-patterns)
7. [Mock Infrastructure](#mock-infrastructure)
8. [Writing Tests for New Features](#writing-tests-for-new-features)
9. [Test Coverage](#test-coverage)
10. [CI/CD Testing Workflow](#cicd-testing-workflow)
11. [Debugging Failing Tests](#debugging-failing-tests)
12. [Best Practices](#best-practices)

---

## Overview & Testing Philosophy

### Testing Principles

KDE Hue Control follows these testing principles:

1. **Test Behavior, Not Implementation**: Tests should validate observable behavior and contracts
2. **Fast Unit Tests**: Unit tests should run in milliseconds without external dependencies
3. **Comprehensive Integration Tests**: Integration tests validate real-world workflows with mock infrastructure
4. **Performance Awareness**: Benchmark tests track performance for critical paths (color extraction, streaming)
5. **CI-First**: All tests must pass in CI before merge

### Testing Pyramid

```
         ┌─────────────┐
         │ Integration │  ~10 tests  (seconds)
         │   Tests     │  Full workflows with mock bridge
         └─────────────┘
       ┌─────────────────┐
       │  Unit Tests     │  ~50+ tests (milliseconds)
       │  Fast, Focused  │  Individual components
       └─────────────────┘
     ┌───────────────────────┐
     │  Benchmarks           │  Performance tracking
     │  Performance Tests    │  Color extraction, DBus, etc.
     └───────────────────────┘
```

### What Gets Tested

- **Config parsing and validation** (UV coordinates, FPS, subsample width)
- **DBus service methods** (GetStatus, GetScenes, ActivateScene, etc.)
- **Hue API client** (scene retrieval, activation, light control)
- **Color extraction** (zone mapping, gamma correction, subsampling)
- **Entertainment API** (DTLS streaming, HueStream v2 protocol)
- **Concurrency safety** (concurrent DBus calls, scene activations)
- **Error handling** (bridge unreachable, invalid parameters)

---

## Test Organization

### Directory Structure

```
backend/
├── *_test.go                      # Integration tests (build tag: integration)
├── integration_test.go            # Hue client integration tests
├── dbus_integration_test.go       # DBus service integration tests
├── entertainment_integration_test.go  # Entertainment API tests
├── e2e_integration_test.go        # End-to-end workflow tests
├── internal/
│   ├── config/
│   │   ├── config_test.go         # Unit tests
│   │   └── config_bench_test.go   # Benchmarks
│   ├── dbus/
│   │   ├── service_test.go        # Unit tests
│   │   └── service_bench_test.go  # Benchmarks
│   ├── hue/
│   │   ├── client_test.go         # Unit tests
│   │   └── client_bench_test.go   # Benchmarks
│   ├── color/
│   │   ├── extractor_test.go      # Unit tests
│   │   ├── extractor_bench_test.go # Benchmarks
│   │   └── performance_test.go    # Performance validation
│   ├── sync/
│   │   └── engine_test.go         # Unit tests
│   ├── gaming/
│   │   └── detector_test.go       # Unit tests
│   ├── capture/
│   │   └── capture_test.go        # Unit tests (PipeWire)
│   └── testutil/
│       ├── mock_bridge.go         # Mock Hue bridge HTTP server
│       └── mock_entertainment.go  # Mock Entertainment API DTLS server
└── INTEGRATION_TESTS.md           # Integration test documentation
```

### Test File Naming

- **Unit tests**: `<package>_test.go` (e.g., `config_test.go`)
- **Benchmark tests**: `<package>_bench_test.go` (e.g., `config_bench_test.go`)
- **Integration tests**: `<feature>_integration_test.go` with `//go:build integration` tag

### Build Tags

Integration tests use build tags to separate them from fast unit tests:

```go
//go:build integration

package main
```

**Why?** Integration tests are slower and may require network access, so they're opt-in via `-tags=integration`.

---

## Running Tests

### Quick Reference

```bash
# Unit tests only (fast, no external dependencies)
cd backend
go test ./...

# Integration tests (requires mock bridge, slower)
go test -tags=integration -v

# Specific package
go test ./internal/config -v

# Specific test function
go test ./internal/config -v -run TestValidate

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# With race detector (recommended)
go test -race ./...

# Benchmarks
go test -bench=. -benchmem ./internal/color
go test -bench=BenchmarkExtractColors_TwoZones_1440p -benchtime=10s ./internal/color
```

### Command Breakdown

#### Unit Tests (Default)

```bash
cd backend
go test ./...
```

**What it does:**
- Runs all `*_test.go` files WITHOUT build tags
- Skips integration tests (they have `//go:build integration`)
- Fast execution (~1-5 seconds)
- No external dependencies required

**Use when:**
- Local development (TDD workflow)
- Quick validation before commit
- CI pipeline (runs on every PR)

#### Integration Tests

```bash
cd backend
go test -tags=integration -v
```

**What it does:**
- Includes files with `//go:build integration` tag
- Sets up mock HTTP/DTLS servers
- Tests full workflows (scene activation, DBus calls, Entertainment API)
- Slower execution (~10-30 seconds)

**Use when:**
- Testing end-to-end workflows
- Validating API client behavior
- Before major releases

**Common flags:**
- `-v`: Verbose output (shows t.Logf messages)
- `-timeout 30s`: Set timeout (default 10m)
- `-run TestName`: Run specific test
- `-p 1`: Disable parallel execution

#### Race Detection

```bash
go test -race ./...
```

**What it does:**
- Detects data races in concurrent code
- Uses Go's race detector (adds ~5-10x slowdown)

**Use when:**
- Testing concurrent code (DBus service, sync engine)
- Before merge (CI runs this automatically)
- Debugging intermittent failures

**Important:** Always run with `-race` for code that uses goroutines or mutexes.

#### Coverage Analysis

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# View in browser (interactive HTML)
go tool cover -html=coverage.out

# Per-package coverage
go test -coverprofile=coverage.out ./internal/config
go tool cover -func=coverage.out
```

**Output example:**
```
github.com/codepuncher/khuey/internal/config/config.go:42:     Load            85.7%
github.com/codepuncher/khuey/internal/config/config.go:67:     Save            100.0%
github.com/codepuncher/khuey/internal/config/config.go:89:     Validate        95.2%
total:                                                          (statements)    89.4%
```

#### Benchmarks

```bash
# Run all benchmarks in package
go test -bench=. -benchmem ./internal/color

# Run specific benchmark
go test -bench=BenchmarkExtractColors_TwoZones_1440p ./internal/color

# Longer benchmark time for accuracy
go test -bench=. -benchtime=10s ./internal/color

# Compare benchmarks (save baseline first)
go test -bench=. -benchmem ./internal/color > old.txt
# Make changes...
go test -bench=. -benchmem ./internal/color > new.txt
benchcmp old.txt new.txt  # Requires golang.org/x/tools/cmd/benchcmp
```

**Output example:**
```
BenchmarkExtractColors_TwoZones_1440p-16        5000      234512 ns/op      65536 B/op      12 allocs/op
BenchmarkExtractColors_FourZones_1440p-16       4500      267823 ns/op      65536 B/op      16 allocs/op
BenchmarkExtractColors_TwoZones_4K-16           3000      398456 ns/op      98304 B/op      12 allocs/op
```

Interpretation:
- `5000` = iterations run
- `234512 ns/op` = nanoseconds per operation (~0.23ms)
- `65536 B/op` = bytes allocated per operation
- `12 allocs/op` = allocations per operation

### Makefile Shortcuts

```bash
# Build backend
make build

# Run unit tests
make test

# Build test utilities
make test-capture

# Clean build artifacts
make clean
```

The Makefile handles CGo flags automatically (`CGO_CFLAGS_ALLOW=-fno-strict-overflow` for PipeWire).

---

## Unit Testing Patterns

Unit tests validate individual functions/methods in isolation without external dependencies.

### Table-Driven Tests

**Pattern:** Use table-driven tests for testing multiple input/output scenarios.

**Example from `internal/config/config_test.go`:**

```go
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        cfg     *config.Config
        wantErr bool
        errMsg  string
    }{
        {
            name: "Valid config",
            cfg: &config.Config{
                Bridge: "192.168.1.100",
                Key:    "test-key",
                Sync: config.SyncConfig{
                    FPS:            30,
                    SubsampleWidth: 64,
                },
            },
            wantErr: false,
        },
        {
            name: "Missing bridge",
            cfg: &config.Config{
                Key: "test-key",
                Sync: config.SyncConfig{
                    FPS:            30,
                    SubsampleWidth: 64,
                },
            },
            wantErr: true,
            errMsg:  "bridge IP not configured",
        },
        {
            name: "FPS too low",
            cfg: &config.Config{
                Bridge: "192.168.1.100",
                Key:    "test-key",
                Sync: config.SyncConfig{
                    FPS:            0,
                    SubsampleWidth: 64,
                },
            },
            wantErr: true,
            errMsg:  "sync.fps must be between",
        },
        {
            name: "Invalid UV coordinates - uvA.X negative",
            cfg: &config.Config{
                Bridge: "192.168.1.100",
                Key:    "test-key",
                Sync: config.SyncConfig{
                    FPS:            30,
                    SubsampleWidth: 64,
                },
                Channels: []config.ChannelConfig{
                    {
                        ID:     0,
                        Active: true,
                        UVA:    config.UV{X: -0.1, Y: 0.0},
                        UVB:    config.UV{X: 0.5, Y: 1.0},
                    },
                },
            },
            wantErr: true,
            errMsg:  "uvA.x must be 0.0-1.0",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.cfg.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if err != nil && tt.errMsg != "" {
                if !strings.Contains(err.Error(), tt.errMsg) {
                    t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
                }
            }
        })
    }
}
```

**Benefits:**
- Tests multiple scenarios in one function
- Easy to add new test cases
- Clear separation of test data and logic
- Named subtests (`t.Run`) for better failure messages

### Testing Constructor Functions

**Example from `internal/color/extractor_test.go`:**

```go
func TestNewExtractor(t *testing.T) {
    tests := []struct {
        name           string
        subsampleWidth int
        gamma          float64
        expectError    bool
        errorContains  string
    }{
        {
            name:           "Valid parameters",
            subsampleWidth: 64,
            gamma:          2.2,
            expectError:    false,
        },
        {
            name:           "Subsample width too small",
            subsampleWidth: 15,
            gamma:          2.2,
            expectError:    true,
            errorContains:  "must be between 16 and 256",
        },
        {
            name:           "Zero gamma",
            subsampleWidth: 64,
            gamma:          0,
            expectError:    true,
            errorContains:  "gamma must be positive",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ext, err := color.NewExtractor(tt.subsampleWidth, tt.gamma)

            if tt.expectError {
                if err == nil {
                    t.Errorf("Expected error but got none")
                } else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
                    t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
                if ext == nil {
                    t.Errorf("Expected extractor but got nil")
                }
            }
        })
    }
}
```

**Key points:**
- Test both success and error paths
- Validate error messages with `strings.Contains()`
- Check returned values are non-nil when expected

### Testing State Methods

**Example from `internal/dbus/service_test.go`:**

```go
func TestGetStatus(t *testing.T) {
    tests := []struct {
        name     string
        cfg      *config.Config
        expected string
    }{
        {
            name:     "Not configured",
            cfg:      config.DefaultConfig(),
            expected: "Not configured",
        },
        {
            name: "Configured",
            cfg: &config.Config{
                Bridge: "192.168.1.100",
                Key:    "test-key",
            },
            expected: "Ready",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            s := &dbus.Service{config: tt.cfg}
            status, err := s.GetStatus()
            if err != nil {
                t.Errorf("GetStatus() returned unexpected error: %v", err)
            }
            if status != tt.expected {
                t.Errorf("GetStatus() = %q, want %q", status, tt.expected)
            }
        })
    }
}
```

### Testing Configuration

**Pattern:** Test config loading, saving, validation, and defaults.

**Example from `internal/config/config_test.go`:**

```go
func TestDefaultConfig(t *testing.T) {
    cfg := config.DefaultConfig()

    if cfg.Sync.FPS != 30 {
        t.Errorf("Expected default FPS of 30, got %d", cfg.Sync.FPS)
    }

    if cfg.Sync.SubsampleWidth != 64 {
        t.Errorf("Expected default subsample width of 64, got %d", cfg.Sync.SubsampleWidth)
    }

    if cfg.Sync.Enabled {
        t.Error("Expected sync to be disabled by default")
    }

    if cfg.LogLevel != "info" {
        t.Errorf("Expected default log level 'info', got '%s'", cfg.LogLevel)
    }
}

func TestConfigValidation(t *testing.T) {
    cfg := config.DefaultConfig()

    // Test IsConfigured
    if cfg.IsConfigured() {
        t.Error("Expected config to be unconfigured without bridge/key")
    }

    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-key"

    if !cfg.IsConfigured() {
        t.Error("Expected config to be configured with bridge and key")
    }

    // Test HasEntertainmentConfig
    if cfg.HasEntertainmentConfig() {
        t.Error("Expected entertainment config to be incomplete")
    }

    cfg.ClientKey = "test-client-key"
    cfg.EntertainmentConfigurationID = "test-id"

    if !cfg.HasEntertainmentConfig() {
        t.Error("Expected entertainment config to be complete")
    }
}
```

### Testing with Temporary Files

**Pattern:** Use `t.TempDir()` for file-based tests (config save/load).

**Example from `internal/config/config_test.go`:**

```go
func TestSave(t *testing.T) {
    // Create temporary directory for test config
    tempDir := t.TempDir()
    configFile := filepath.Join(tempDir, "config.yaml")

    // Override getConfigFile for testing
    originalGetConfigFile := getConfigFile
    getConfigFile = func() string {
        return configFile
    }
    defer func() {
        getConfigFile = originalGetConfigFile
    }()

    cfg := config.DefaultConfig()
    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-api-key"
    cfg.ClientKey = "test-client-key"
    cfg.EntertainmentConfigurationID = "test-ent-id"

    // Save config
    err := cfg.Save()
    if err != nil {
        t.Fatalf("Save() failed: %v", err)
    }

    // Verify file exists
    if _, err := os.Stat(configFile); os.IsNotExist(err) {
        t.Fatal("Config file was not created")
    }

    // Verify file permissions (should be 0600)
    info, err := os.Stat(configFile)
    if err != nil {
        t.Fatalf("Failed to stat config file: %v", err)
    }
    if info.Mode().Perm() != 0600 {
        t.Errorf("Config file has wrong permissions: got %o, want 0600", info.Mode().Perm())
    }

    // Verify file content by loading it back
    loadedCfg, err := config.Load()
    if err != nil {
        t.Fatalf("Failed to load saved config: %v", err)
    }

    if loadedCfg.Bridge != cfg.Bridge {
        t.Errorf("Loaded Bridge = %q, want %q", loadedCfg.Bridge, cfg.Bridge)
    }
}
```

**Key points:**
- `t.TempDir()` automatically cleans up after test
- Override functions that access filesystem paths
- Always restore original functions with `defer`
- Verify file permissions for security-sensitive files

### Testing Logic Without Dependencies

**Pattern:** Test pure logic (like scene name matching) in isolation.

**Example from `internal/dbus/service_test.go`:**

```go
func TestSceneNameMatching(t *testing.T) {
    scenes := []hue.Scene{
        {ID: "scene-1", Name: "Relax", RoomName: "Living Room"},
        {ID: "scene-2", Name: "Bright", RoomName: "Kitchen"},
        {ID: "scene-3", Name: "Concentrate", RoomName: ""},
    }

    tests := []struct {
        name        string
        displayName string
        expectFound bool
        expectID    string
    }{
        {name: "Match with room name", displayName: "Living Room - Relax", expectFound: true, expectID: "scene-1"},
        {name: "Match scene name only", displayName: "Concentrate", expectFound: true, expectID: "scene-3"},
        {name: "No match", displayName: "Nonexistent Scene", expectFound: false},
        {name: "Partial match should not work", displayName: "Living Room", expectFound: false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var foundID string
            found := false

            for _, scene := range scenes {
                sceneDisplayName := scene.Name
                if scene.RoomName != "" {
                    sceneDisplayName = scene.RoomName + " - " + scene.Name
                }

                if sceneDisplayName == tt.displayName || scene.Name == tt.displayName {
                    foundID = scene.ID
                    found = true
                    break
                }
            }

            if found != tt.expectFound {
                t.Errorf("Scene found = %v, want %v", found, tt.expectFound)
            }
            if found && foundID != tt.expectID {
                t.Errorf("Scene ID = %v, want %v", foundID, tt.expectID)
            }
        })
    }
}
```

**Why?** This validates the matching logic that `ActivateScene` uses without needing a real Hue client or mock.

---

## Integration Testing Patterns

Integration tests validate full workflows with mock infrastructure. They use the `//go:build integration` build tag.

### Setting Up Integration Tests

**File header:**

```go
//go:build integration

package main

import (
    "testing"
    "github.com/codepuncher/khuey/internal/testutil"
    // ...
)
```

**Running:**

```bash
go test -tags=integration -v
```

### Mock Bridge Pattern

**Example from `backend/integration_test.go`:**

```go
func TestHueClient_GetScenes(t *testing.T) {
    // Setup mock bridge with TLS
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.SetupDefaultScenario()

    bridgeAddr := mock.URL()[8:] // Remove https://
    client := setupTestClient(t, bridgeAddr, "test-api-key")

    scenes, err := client.GetScenes()
    if err != nil {
        t.Fatalf("GetScenes() error = %v", err)
    }

    if len(scenes) != 3 {
        t.Errorf("GetScenes() returned %d scenes, want 3", len(scenes))
    }

    // Verify scene data
    sceneNames := make(map[string]bool)
    for _, scene := range scenes {
        sceneNames[scene.Name] = true
    }

    expected := []string{"Relax", "Bright", "Concentrate"}
    for _, name := range expected {
        if !sceneNames[name] {
            t.Errorf("Expected scene %q not found", name)
        }
    }
}
```

**Key components:**

1. **Mock bridge setup**: `testutil.NewMockBridgeTLS()`
2. **Default scenario**: Pre-populated with scenes, lights, rooms
3. **Cleanup**: `defer mock.Close()`
4. **TLS handling**: Skip verify for test server

### Testing DBus Service

**Example from `backend/dbus_integration_test.go`:**

```go
func setupTestDBusService(t *testing.T) (*dbus.Service, *testutil.MockBridge, func()) {
    // Setup mock bridge with TLS
    mock := testutil.NewMockBridgeTLS()
    mock.SetupDefaultScenario()

    // Set TLS skip verify
    originalTransport := http.DefaultTransport
    http.DefaultTransport = &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }

    // Create config
    cfg := config.DefaultConfig()
    cfg.Bridge = mock.URL()[8:]
    cfg.Key = "test-api-key"

    // Create Hue client
    ctx := context.Background()
    client, err := hue.NewClient(ctx, cfg.Bridge, cfg.Key)
    if err != nil {
        t.Fatalf("Failed to create Hue client: %v", err)
    }

    // Create DBus service
    service, err := dbus.NewService(cfg, client)
    if err != nil {
        t.Fatalf("Failed to create DBus service: %v", err)
    }

    cleanup := func() {
        service.Stop()
        mock.Close()
        http.DefaultTransport = originalTransport
    }

    return service, mock, cleanup
}

func TestDBusService_GetScenes(t *testing.T) {
    service, mock, cleanup := setupTestDBusService(t)
    defer cleanup()

    err := service.Start()
    if err != nil {
        if strings.Contains(err.Error(), "name already taken") {
            t.Skip("Service already running")
        }
        t.Fatalf("Failed to start service: %v", err)
    }

    conn, connCleanup := getTestDBusConnection(t)
    defer connCleanup()

    obj := conn.Object("org.kde.plasma.hue", "/org/kde/plasma/hue")

    var scenes []string
    err = obj.Call("org.kde.plasma.hue.GetScenes", 0).Store(&scenes)
    if err != nil {
        t.Fatalf("GetScenes() error: %v", err)
    }

    if len(scenes) != 3 {
        t.Errorf("GetScenes() returned %d scenes, want 3", len(scenes))
    }

    // Verify scenes are formatted correctly
    for _, scene := range scenes {
        if scene == "" {
            t.Error("GetScenes() returned empty scene name")
        }
    }

    t.Logf("Scenes: %v", scenes)
}
```

**Important:**
- DBus tests require session bus
- Tests skip if service name already taken (`t.Skip()`)
- Stop any running `hue-sync` before testing
- Verify request count on mock bridge

### Testing Entertainment API

**Example from `backend/entertainment_integration_test.go`:**

```go
const (
    testClientKey       = "0123456789ABCDEF0123456789ABCDEF" // 32 hex chars
    testUsername        = "test-api-key"
    testEntertainmentID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

func setupTestEntertainmentServer(t *testing.T) (*testutil.MockEntertainmentServer, func()) {
    server := testutil.NewMockEntertainmentServer(testClientKey, testUsername)
    err := server.Start(t)
    if err != nil {
        t.Fatalf("Failed to start mock Entertainment server: %v", err)
    }

    cleanup := func() {
        server.Stop()
    }

    return server, cleanup
}

func TestEntertainmentAPI_ServerStartStop(t *testing.T) {
    server, cleanup := setupTestEntertainmentServer(t)
    defer cleanup()

    // Verify server is listening
    port := server.GetPort()
    if port == 0 {
        t.Error("Server port should be non-zero")
    }

    addr := server.GetAddress()
    if addr == "" {
        t.Error("Server address should not be empty")
    }

    t.Logf("Mock server listening on %s", addr)

    // Verify initial state
    if server.GetFrameCount() != 0 {
        t.Errorf("Initial frame count = %d, want 0", server.GetFrameCount())
    }

    if server.GetConnectionCount() != 0 {
        t.Errorf("Initial connection count = %d, want 0", server.GetConnectionCount())
    }
}
```

**Entertainment mock features:**
- DTLS server with PSK authentication
- Parses HueStream v2 protocol packets
- Tracks frames received (count, rate, channels)
- Validates RGB values per channel
- `WaitForFrames(minFrames, timeout)` helper

### Concurrency Testing

**Example from `backend/integration_test.go`:**

```go
func TestConcurrency_GetScenes(t *testing.T) {
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.SetupDefaultScenario()

    bridgeAddr := mock.URL()[8:]
    client := setupTestClient(t, bridgeAddr, "test-api-key")

    // Launch 10 concurrent GetScenes requests
    errs := make(chan error, 10)
    for i := 0; i < 10; i++ {
        go func() {
            _, err := client.GetScenes()
            errs <- err
        }()
    }

    // Collect results
    for i := 0; i < 10; i++ {
        if err := <-errs; err != nil {
            t.Errorf("Concurrent GetScenes #%d error: %v", i, err)
        }
    }
}
```

**Why?** Validates thread-safety of HTTP client, mock bridge mutexes, and DBus service.

### Error Injection

**Example:**

```go
func TestErrorHandling(t *testing.T) {
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.SetupDefaultScenario()

    // Inject error for scenes endpoint
    mock.SetResponseError("/clip/v2/resource/scene", fmt.Errorf("internal server error"))

    bridgeAddr := mock.URL()[8:]
    client := setupTestClient(t, bridgeAddr, "test-api-key")

    _, err := client.GetScenes()
    if err == nil {
        t.Error("Expected error from mock, got nil")
    }

    // Clear error and verify recovery
    mock.ClearResponseErrors()
    scenes, err := client.GetScenes()
    if err != nil {
        t.Errorf("Expected success after clearing errors, got: %v", err)
    }
    if len(scenes) != 3 {
        t.Errorf("Got %d scenes after recovery, want 3", len(scenes))
    }
}
```

**Use cases:**
- Test error handling paths
- Validate retry logic
- Ensure graceful degradation

---

## Benchmark Testing Patterns

Benchmark tests track performance of critical paths. They use `testing.B` and `go test -bench=`.

### Basic Benchmark Structure

```go
func BenchmarkFunction(b *testing.B) {
    // Setup (not measured)
    data := setupTestData()

    // Reset timer (excludes setup time)
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        // Code being benchmarked
        result := expensiveFunction(data)
        _ = result // Prevent compiler optimization
    }
}
```

### Color Extraction Benchmarks

**Example from `internal/color/extractor_bench_test.go`:**

```go
func BenchmarkExtractColors_TwoZones_1440p(b *testing.B) {
    img := createBenchImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

    zones := []color.Zone{
        {U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0}, // Left half
        {U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0}, // Right half
    }

    extractor, err := color.NewExtractor(64, 2.2)
    if err != nil {
        b.Fatal(err)
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := extractor.ExtractColors(img, zones)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkExtractColors_FourZones_1440p(b *testing.B) {
    img := createBenchImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

    zones := []color.Zone{
        {U1: 0.0, V1: 0.0, U2: 0.5, V2: 0.5}, // Top-left
        {U1: 0.5, V1: 0.0, U2: 1.0, V2: 0.5}, // Top-right
        {U1: 0.0, V1: 0.5, U2: 0.5, V2: 1.0}, // Bottom-left
        {U1: 0.5, V1: 0.5, U2: 1.0, V2: 1.0}, // Bottom-right
    }

    extractor, err := color.NewExtractor(64, 2.2)
    if err != nil {
        b.Fatal(err)
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := extractor.ExtractColors(img, zones)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

**Running:**

```bash
# Run all color benchmarks
go test -bench=. -benchmem ./internal/color

# Output:
BenchmarkExtractColors_TwoZones_1440p-16    5000    234512 ns/op    65536 B/op    12 allocs/op
BenchmarkExtractColors_FourZones_1440p-16   4500    267823 ns/op    65536 B/op    16 allocs/op
```

**Interpretation:**
- `234512 ns/op` = 0.23 ms per extraction (meets 30 FPS target: 33ms budget)
- `65536 B/op` = 64 KB allocated (subsample buffer)
- `12 allocs/op` = number of heap allocations

### Parameterized Benchmarks

**Pattern:** Test performance across different parameters.

**Example:**

```go
func BenchmarkExtractColors_Subsample32(b *testing.B) {
    benchmarkSubsample(b, 32)
}

func BenchmarkExtractColors_Subsample64(b *testing.B) {
    benchmarkSubsample(b, 64)
}

func BenchmarkExtractColors_Subsample128(b *testing.B) {
    benchmarkSubsample(b, 128)
}

func benchmarkSubsample(b *testing.B, subsampleWidth int) {
    img := createBenchImage(2560, 1440, color.RGBA{R: 128, G: 64, B: 200, A: 255})

    zones := []color.Zone{
        {U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
        {U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
    }

    extractor, err := color.NewExtractor(subsampleWidth, 2.2)
    if err != nil {
        b.Fatal(err)
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := extractor.ExtractColors(img, zones)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

**Running:**

```bash
go test -bench=BenchmarkExtractColors_Subsample -benchmem ./internal/color
```

### DBus Service Benchmarks

**Example from `internal/dbus/service_bench_test.go`:**

```go
func BenchmarkGetStatus(b *testing.B) {
    cfg := config.DefaultConfig()
    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-key"

    client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
    if err != nil {
        b.Fatalf("Failed to create client: %v", err)
    }

    service := &dbus.Service{
        config:    cfg,
        hueClient: client,
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        status, err := service.GetStatus()
        if err != nil {
            b.Fatalf("GetStatus() failed: %v", err)
        }
        if status == "" {
            b.Fatal("GetStatus() returned empty string")
        }
    }
}
```

**Use cases:**
- Validate DBus method call overhead is low
- Ensure config/state access is fast
- Track performance regressions

### Benchmark Best Practices

1. **Reset timer after setup**: `b.ResetTimer()`
2. **Use realistic data sizes**: Test with 1440p/4K images
3. **Run with `-benchmem`**: Track allocations
4. **Use `-benchtime=10s`**: Longer runs for accuracy
5. **Save baselines**: Compare before/after changes
6. **Don't optimize prematurely**: Benchmark first

---

## Mock Infrastructure

The `internal/testutil/` package provides mock implementations for testing without hardware.

### Mock Hue Bridge

**File:** `internal/testutil/mock_bridge.go`

**Features:**
- HTTP/HTTPS server with TLS support
- Simulates Hue Bridge REST API (CLIP v2)
- Request logging for verification
- Error injection for testing failure paths
- Pre-configured scenarios

**Creating a mock bridge:**

```go
// HTTP mock (for quick tests)
mock := testutil.NewMockBridge()
defer mock.Close()

// HTTPS mock (matches real bridge)
mock := testutil.NewMockBridgeTLS()
defer mock.Close()

// Setup default data
mock.SetupDefaultScenario()
// Adds:
// - 3 scenes: "Relax", "Bright", "Concentrate"
// - 3 lights: "Ceiling Light 1", "Ceiling Light 2", "Floor Lamp"
// - 3 rooms: "Living Room", "Kitchen", "Office"
// - 1 zone: "TV Area"

// Get bridge URL for client
bridgeAddr := mock.URL()[8:] // Remove "https://" prefix
```

**Adding custom data:**

```go
mock := testutil.NewMockBridge()

// Add scenes
mock.AddScene("scene-1", "Sunset", "Bedroom")
mock.AddScene("scene-2", "Arctic", "Kitchen")

// Add lights
mock.AddLight("light-1", "Desk Lamp")
mock.AddLight("light-2", "Strip Light")

// Add grouped lights (rooms/zones)
mock.AddGroupedLight("room-1", "Bedroom", "room")
mock.AddGroupedLight("zone-1", "Desk Area", "zone")

// Add rooms
mock.AddRoom("room-1", "Bedroom")

// Add zones
mock.AddZone("zone-1", "Desk Area")
```

**Request logging:**

```go
// Clear log before test
mock.ClearRequestLog()

// Make API calls...
client.GetScenes()
client.ActivateScene("scene-1")

// Verify requests
count := mock.GetRequestCount()
log := mock.GetRequestLog()

// Example log:
// ["GET /clip/v2/resource/scene", "GET /clip/v2/resource/room", "PUT /clip/v2/resource/scene/scene-1"]
```

**Error injection:**

```go
// Inject error for specific endpoint
mock.SetResponseError("/clip/v2/resource/scene", fmt.Errorf("internal server error"))

// This call will fail
_, err := client.GetScenes()
if err == nil {
    t.Error("Expected error from mock")
}

// Clear errors
mock.ClearResponseErrors()

// This call will succeed
scenes, err := client.GetScenes()
```

**API endpoints supported:**
- `GET /clip/v2/resource/scene` → List scenes
- `GET /clip/v2/resource/room` → List rooms
- `GET /clip/v2/resource/zone` → List zones
- `GET /clip/v2/resource/light` → List lights
- `GET /clip/v2/resource/grouped_light` → List grouped lights
- `GET /clip/v2/resource/entertainment_configuration` → List Entertainment areas
- `PUT /clip/v2/resource/scene/{id}` → Activate scene
- `PUT /clip/v2/resource/grouped_light/{id}` → Control lights

### Mock Entertainment Server

**File:** `internal/testutil/mock_entertainment.go`

**Features:**
- DTLS server with PSK authentication
- Parses HueStream v2 protocol packets
- Tracks frames received (count, rate, RGB values)
- Validates Entertainment API streaming
- Frame analysis and verification

**Creating a mock Entertainment server:**

```go
const (
    testClientKey = "0123456789ABCDEF0123456789ABCDEF" // 32 hex chars
    testUsername  = "test-api-key"
)

server := testutil.NewMockEntertainmentServer(testClientKey, testUsername)
err := server.Start(t)
if err != nil {
    t.Fatalf("Failed to start: %v", err)
}
defer server.Stop()

// Get server details
port := server.GetPort()
addr := server.GetAddress() // "127.0.0.1:12345"
```

**Tracking frames:**

```go
// Wait for frames (with timeout)
success := server.WaitForFrames(30, 5*time.Second)
if !success {
    t.Error("Did not receive 30 frames in 5 seconds")
}

// Get frame count
count := server.GetFrameCount()
t.Logf("Received %d frames", count)

// Get frame rate
fps := server.GetFrameRate()
t.Logf("Average FPS: %.2f", fps)

// Get last frame
frame := server.GetLastFrame()
if frame != nil {
    t.Logf("Last frame: %d channels, seq %d", len(frame.Channels), frame.SequenceID)
}
```

**Verifying colors:**

```go
// Get color for specific channel
r, g, b, found := server.GetChannelColor(0)
if !found {
    t.Error("Channel 0 not found")
}
t.Logf("Channel 0 RGB: (%d, %d, %d)", r, g, b)

// Verify color with tolerance
err := server.VerifyChannelColor(
    0,              // channel ID
    65535, 0, 0,    // expected RGB (red)
    1000,           // tolerance
)
if err != nil {
    t.Errorf("Color verification failed: %v", err)
}
```

**Frame history:**

```go
// Get all frames (last 100)
frames := server.GetFrames()
for i, frame := range frames {
    t.Logf("Frame %d: seq=%d, channels=%d", i, frame.SequenceID, len(frame.Channels))
}

// Clear history
server.ClearFrames()
```

**Statistics:**

```go
server.LogStats(t)
// Output:
// [Mock Entertainment Stats]
//   Connections: 1
//   Frames Received: 150
//   Average FPS: 30.12
//   Runtime: 4.98s
//   Last Frame:
//     Sequence ID: 149
//     Channels: 2
//     Packet Size: 66 bytes
//     Channel 0 RGB: (32768, 16384, 49152)
```

**HueStream v2 packet structure:**

```
Header (52 bytes):
  - Protocol (9 bytes): "HueStream"
  - Version (2 bytes): Major=0x02, Minor=0x00
  - Sequence ID (1 byte): Frame counter
  - Reserved (2 bytes)
  - Color space (1 byte): 0x00=RGB
  - Reserved (1 byte)
  - Entertainment Config ID (36 bytes): UUID

Body (7 bytes per channel):
  - Channel ID (1 byte)
  - R (2 bytes, big-endian, 0-65535)
  - G (2 bytes, big-endian, 0-65535)
  - B (2 bytes, big-endian, 0-65535)
```

### Using Mocks in Tests

**Complete example:**

```go
func TestSceneActivationWorkflow(t *testing.T) {
    // Setup mock bridge
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.SetupDefaultScenario()

    // Create Hue client
    bridgeAddr := mock.URL()[8:]
    client, err := hue.NewClient(context.Background(), bridgeAddr, "test-key")
    if err != nil {
        t.Fatalf("Failed to create client: %v", err)
    }

    // Clear request log
    mock.ClearRequestLog()

    // Get scenes
    scenes, err := client.GetScenes()
    if err != nil {
        t.Fatalf("GetScenes() error: %v", err)
    }
    if len(scenes) != 3 {
        t.Fatalf("Expected 3 scenes, got %d", len(scenes))
    }

    // Activate first scene
    err = client.ActivateScene(scenes[0].ID)
    if err != nil {
        t.Fatalf("ActivateScene() error: %v", err)
    }

    // Verify requests
    requests := mock.GetRequestLog()
    expectedRequests := []string{
        "GET /clip/v2/resource/scene",
        "GET /clip/v2/resource/room",
        "GET /clip/v2/resource/zone",
        "PUT /clip/v2/resource/scene/" + scenes[0].ID,
    }

    if len(requests) != len(expectedRequests) {
        t.Errorf("Expected %d requests, got %d", len(expectedRequests), len(requests))
    }

    for i, expected := range expectedRequests {
        if i < len(requests) && requests[i] != expected {
            t.Errorf("Request %d: got %q, want %q", i, requests[i], expected)
        }
    }
}
```

---

## Writing Tests for New Features

### Adding a New DBus Method

**Scenario:** You're adding a new DBus method `SetLightTemperature(temp int32)`.

**Step 1: Write unit test for logic**

```go
// internal/dbus/service_test.go

func TestSetLightTemperature(t *testing.T) {
    tests := []struct {
        name        string
        temp        int32
        expectError bool
        errorMsg    string
    }{
        {
            name:        "Valid warm temperature",
            temp:        2700,
            expectError: false,
        },
        {
            name:        "Valid cool temperature",
            temp:        6500,
            expectError: false,
        },
        {
            name:        "Temperature too low",
            temp:        1000,
            expectError: true,
            errorMsg:    "temperature must be between 2200 and 6500",
        },
        {
            name:        "Temperature too high",
            temp:        10000,
            expectError: true,
            errorMsg:    "temperature must be between 2200 and 6500",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := config.DefaultConfig()
            cfg.Bridge = "192.168.1.100"
            cfg.Key = "test-key"
            cfg.GroupedLightID = "room-1"

            s := &dbus.Service{config: cfg}

            // Note: Testing logic only, not actual API call
            // Validate input
            if tt.temp < 2200 || tt.temp > 6500 {
                if !tt.expectError {
                    t.Errorf("Expected error for temp=%d", tt.temp)
                }
                return
            }

            if tt.expectError {
                t.Errorf("Expected error for temp=%d but got none", tt.temp)
            }
        })
    }
}
```

**Step 2: Write integration test with mock**

```go
// backend/dbus_integration_test.go

func TestDBusService_SetLightTemperature(t *testing.T) {
    service, mock, cleanup := setupTestDBusService(t)
    defer cleanup()

    err := service.Start()
    if err != nil {
        if strings.Contains(err.Error(), "name already taken") {
            t.Skip("Service already running")
        }
        t.Fatalf("Failed to start service: %v", err)
    }

    conn, connCleanup := getTestDBusConnection(t)
    defer connCleanup()

    obj := conn.Object("org.kde.plasma.hue", "/org/kde/plasma/hue")

    // Test valid temperature
    var result bool
    err = obj.Call("org.kde.plasma.hue.SetLightTemperature", 0, int32(2700)).Store(&result)
    if err != nil {
        t.Fatalf("SetLightTemperature() error: %v", err)
    }
    if !result {
        t.Error("SetLightTemperature() returned false")
    }

    // Verify mock received PUT request
    requestCount := mock.GetRequestCount()
    if requestCount < 1 {
        t.Error("Expected at least 1 request to mock bridge")
    }

    // Test invalid temperature (should fail validation)
    err = obj.Call("org.kde.plasma.hue.SetLightTemperature", 0, int32(10000)).Store(&result)
    if err == nil {
        t.Error("Expected error for invalid temperature")
    }
}
```

**Step 3: Add benchmark if performance-critical**

```go
// internal/dbus/service_bench_test.go

func BenchmarkSetLightTemperature(b *testing.B) {
    cfg := config.DefaultConfig()
    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-key"
    cfg.GroupedLightID = "room-1"

    client, err := hue.NewClient(context.Background(), cfg.Bridge, cfg.Key)
    if err != nil {
        b.Fatalf("Failed to create client: %v", err)
    }

    service := &dbus.Service{
        config:    cfg,
        hueClient: client,
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := service.SetLightTemperature(2700)
        if err != nil {
            // Expected to fail without real bridge, but we're benchmarking call overhead
        }
    }
}
```

### Adding a New Config Option

**Scenario:** You're adding a new config field `AutoAdjustBrightness bool`.

**Step 1: Add field to config struct**

```go
// internal/config/config.go

type Config struct {
    Bridge                       string `mapstructure:"Bridge"`
    Key                          string `mapstructure:"Key"`
    AutoAdjustBrightness         bool   `mapstructure:"autoAdjustBrightness"`
    // ... other fields
}

func DefaultConfig() *Config {
    return &Config{
        AutoAdjustBrightness: false, // Default value
        // ... other defaults
    }
}
```

**Step 2: Write unit tests**

```go
// internal/config/config_test.go

func TestDefaultConfig_AutoAdjustBrightness(t *testing.T) {
    cfg := config.DefaultConfig()

    if cfg.AutoAdjustBrightness {
        t.Error("Expected AutoAdjustBrightness to be false by default")
    }
}

func TestLoadConfig_AutoAdjustBrightness(t *testing.T) {
    tempDir := t.TempDir()
    configFile := filepath.Join(tempDir, "config.yaml")

    configYAML := `Bridge: 192.168.1.100
Key: test-key
autoAdjustBrightness: true
`
    err := os.WriteFile(configFile, []byte(configYAML), 0600)
    if err != nil {
        t.Fatalf("Failed to write config: %v", err)
    }

    // Override config path
    originalGetConfigFile := getConfigFile
    getConfigFile = func() string {
        return configFile
    }
    defer func() {
        getConfigFile = originalGetConfigFile
    }()

    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load() error: %v", err)
    }

    if !cfg.AutoAdjustBrightness {
        t.Error("Expected AutoAdjustBrightness to be true from config file")
    }
}

func TestSaveConfig_AutoAdjustBrightness(t *testing.T) {
    tempDir := t.TempDir()
    configFile := filepath.Join(tempDir, "config.yaml")

    originalGetConfigFile := getConfigFile
    getConfigFile = func() string {
        return configFile
    }
    defer func() {
        getConfigFile = originalGetConfigFile
    }()

    cfg := config.DefaultConfig()
    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-key"
    cfg.AutoAdjustBrightness = true

    err := cfg.Save()
    if err != nil {
        t.Fatalf("Save() error: %v", err)
    }

    // Load back and verify
    loadedCfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load() error: %v", err)
    }

    if !loadedCfg.AutoAdjustBrightness {
        t.Error("Expected AutoAdjustBrightness to persist after save/load")
    }
}
```

### Adding a New Scene Feature

**Scenario:** You're adding support for scene groups (multiple scenes in a group).

**Step 1: Define data structures**

```go
// internal/hue/scene.go

type SceneGroup struct {
    ID     string
    Name   string
    Scenes []string // Scene IDs in this group
}
```

**Step 2: Add mock support**

```go
// internal/testutil/mock_bridge.go

type MockSceneGroup struct {
    ID     string
    Name   string
    Scenes []string
}

func (mb *MockBridge) AddSceneGroup(id, name string, sceneIDs []string) {
    mb.mu.Lock()
    defer mb.mu.Unlock()
    mb.SceneGroups[id] = &MockSceneGroup{
        ID:     id,
        Name:   name,
        Scenes: sceneIDs,
    }
}
```

**Step 3: Write unit tests**

```go
// internal/hue/client_test.go

func TestSceneGroup_Structure(t *testing.T) {
    group := hue.SceneGroup{
        ID:     "group-1",
        Name:   "Evening Scenes",
        Scenes: []string{"scene-1", "scene-2", "scene-3"},
    }

    if group.ID != "group-1" {
        t.Errorf("ID = %q, want 'group-1'", group.ID)
    }

    if len(group.Scenes) != 3 {
        t.Errorf("Expected 3 scenes, got %d", len(group.Scenes))
    }
}
```

**Step 4: Write integration tests**

```go
// backend/integration_test.go

func TestHueClient_GetSceneGroups(t *testing.T) {
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()

    // Setup scenes
    mock.AddScene("scene-1", "Relax", "Living Room")
    mock.AddScene("scene-2", "Bright", "Living Room")

    // Setup scene group
    mock.AddSceneGroup("group-1", "Living Room Scenes", []string{"scene-1", "scene-2"})

    bridgeAddr := mock.URL()[8:]
    client := setupTestClient(t, bridgeAddr, "test-api-key")

    groups, err := client.GetSceneGroups()
    if err != nil {
        t.Fatalf("GetSceneGroups() error: %v", err)
    }

    if len(groups) != 1 {
        t.Errorf("Expected 1 group, got %d", len(groups))
    }

    group := groups[0]
    if group.Name != "Living Room Scenes" {
        t.Errorf("Group name = %q, want 'Living Room Scenes'", group.Name)
    }

    if len(group.Scenes) != 2 {
        t.Errorf("Expected 2 scenes in group, got %d", len(group.Scenes))
    }
}
```

---

## Test Coverage

### Measuring Coverage

```bash
# Generate coverage report
cd backend
go test -coverprofile=coverage.out ./...

# View summary
go tool cover -func=coverage.out

# View in browser (interactive)
go tool cover -html=coverage.out
```

### Coverage Targets

**Minimum coverage requirements:**

- **Overall:** 10% (CI enforces this)
- **Config package:** 80%+ (critical for correctness)
- **DBus service:** 60%+ (many methods, integration-heavy)
- **Hue client:** 50%+ (external API, error handling)
- **Color extraction:** 70%+ (performance-critical)
- **Entertainment API:** 40%+ (integration-heavy, DTLS)

**Priority for coverage:**

1. **Config validation** (UV coordinates, ranges)
2. **Error handling** (bridge unreachable, invalid IDs)
3. **State methods** (IsConfigured, HasEntertainmentConfig)
4. **Scene matching logic**
5. **Color extraction zones**

### Viewing Coverage by Package

```bash
# Config package
go test -coverprofile=coverage.out ./internal/config
go tool cover -func=coverage.out | grep config.go

# Output:
internal/config/config.go:42:  Load        85.7%
internal/config/config.go:67:  Save        100.0%
internal/config/config.go:89:  Validate    95.2%
```

### Improving Coverage

**Identify untested code:**

```bash
# Show uncovered lines
go tool cover -html=coverage.out
# Opens browser with red highlights for untested code
```

**Add tests for uncovered paths:**

```go
// Example: If error path is uncovered
func TestSaveConfig_WriteError(t *testing.T) {
    // Create read-only directory to trigger write error
    tempDir := t.TempDir()
    configFile := filepath.Join(tempDir, "readonly", "config.yaml")

    // Make directory read-only
    os.Mkdir(filepath.Join(tempDir, "readonly"), 0444)

    cfg := config.DefaultConfig()
    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-key"

    // Override config path
    originalGetConfigFile := getConfigFile
    getConfigFile = func() string {
        return configFile
    }
    defer func() {
        getConfigFile = originalGetConfigFile
    }()

    err := cfg.Save()
    if err == nil {
        t.Error("Expected error saving to read-only directory")
    }
}
```

### Coverage in CI

CI enforces minimum coverage:

```yaml
# .github/workflows/ci.yml

- name: Check test coverage
  working-directory: backend
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    echo "Test coverage: ${COVERAGE}%"
    if (( $(echo "$COVERAGE < 10" | bc -l) )); then
      echo "Coverage ${COVERAGE}% is below minimum 10%"
      exit 1
    fi
```

**Coverage report upload:**

```yaml
- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v4
  with:
    file: backend/coverage.out
    flags: backend
```

---

## CI/CD Testing Workflow

### GitHub Actions CI Pipeline

**File:** `.github/workflows/ci.yml`

**Jobs:**

1. **backend-test**: Run tests with coverage
2. **backend-build**: Build backend binary
3. **tray-build**: Build Qt tray app
4. **lint**: Run golangci-lint

### Backend Test Job

```yaml
backend-test:
  name: Backend Tests
  runs-on: ubuntu-latest

  steps:
  - name: Checkout code
    uses: actions/checkout@v4

  - name: Set up Go
    uses: actions/setup-go@v5
    with:
      go-version: '1.23'

  - name: Install dependencies
    run: |
      sudo apt-get update
      sudo apt-get install -y libpipewire-0.3-dev pkg-config

  - name: Cache Go modules
    uses: actions/cache@v4
    with:
      path: ~/go/pkg/mod
      key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}

  - name: Run go vet
    working-directory: backend
    run: go vet ./...

  - name: Run go fmt
    working-directory: backend
    run: |
      if [ "$(gofmt -s -l . | wc -l)" -gt 0 ]; then
        echo "The following files need formatting:"
        gofmt -s -l .
        exit 1
      fi

  - name: Run tests
    working-directory: backend
    run: CGO_CFLAGS_ALLOW='-fno-strict-overflow' go test -v -race -coverprofile=coverage.out ./...

  - name: Upload coverage to Codecov
    uses: codecov/codecov-action@v4
    with:
      file: backend/coverage.out

  - name: Check test coverage
    working-directory: backend
    run: |
      COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
      echo "Test coverage: ${COVERAGE}%"
      if (( $(echo "$COVERAGE < 10" | bc -l) )); then
        echo "Coverage ${COVERAGE}% is below minimum 10%"
        exit 1
      fi
```

**What runs:**
1. `go vet ./...` - Static analysis
2. `gofmt -s -l .` - Code formatting check
3. `go test -race -coverprofile=coverage.out ./...` - Tests with race detector
4. Coverage upload to Codecov
5. Coverage threshold check (minimum 10%)

**Note:** CI runs **unit tests only** (no `-tags=integration`). Integration tests are opt-in for local development.

### Build Jobs

```yaml
backend-build:
  steps:
  - name: Build backend
    working-directory: backend
    run: CGO_CFLAGS_ALLOW='-fno-strict-overflow' go build -v -o hue-sync ./cmd/hue-sync

  - name: Upload backend binary
    uses: actions/upload-artifact@v4
    with:
      name: hue-sync-binary
      path: backend/hue-sync
```

**Why?** Ensures code compiles on fresh environment with all dependencies.

### Lint Job

```yaml
lint:
  steps:
  - name: Run golangci-lint
    uses: golangci/golangci-lint-action@v4
    with:
      version: latest
      working-directory: backend
      args: --timeout=5m
```

**What it checks:**
- Code style issues
- Potential bugs (nil dereferences, unreachable code)
- Performance issues (unnecessary allocations)
- Security issues (weak crypto, SQL injection)

### Local Pre-Push Checks

**Run before pushing:**

```bash
# Format code
gofmt -s -w .

# Vet (static analysis)
go vet ./...

# Run tests with race detector
go test -race ./...

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total

# Build
go build -o hue-sync ./cmd/hue-sync
```

**Or use pre-commit hooks (lefthook):**

```bash
# Install lefthook
go install github.com/evilmartians/lefthook/v2@latest

# Install hooks
lefthook install

# Hooks run automatically on git commit
```

---

## Debugging Failing Tests

### Common Failure Patterns

#### 1. Race Condition Detected

**Symptom:**

```
==================
WARNING: DATA RACE
Write at 0x00c000126018 by goroutine 23:
  github.com/codepuncher/khuey/internal/sync.(*Engine).Start()
      /path/to/khuey/backend/internal/sync/engine.go:45 +0x123

Previous read at 0x00c000126018 by goroutine 8:
  github.com/codepuncher/khuey/internal/sync.(*Engine).IsRunning()
      /path/to/khuey/backend/internal/sync/engine.go:78 +0x45
==================
```

**Diagnosis:**
- Multiple goroutines accessing shared state without synchronization
- Common in `sync.Engine`, `dbus.Service`, mock bridge

**Fix:**
- Add mutex protection:

```go
type Engine struct {
    mu      sync.RWMutex
    running bool
}

func (e *Engine) IsRunning() bool {
    e.mu.RLock()
    defer e.mu.RUnlock()
    return e.running
}

func (e *Engine) Start() {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.running = true
}
```

**Prevention:**
- Always run tests with `-race`
- Use `sync.RWMutex` for read-heavy state
- Document thread-safety in godoc

#### 2. Test Times Out

**Symptom:**

```
panic: test timed out after 10m0s

goroutine 45 [select]:
testing.(*T).Run(0xc0001c2000, {0x7f8b2c, 0x18}, 0xc000182ea0)
```

**Diagnosis:**
- Test waiting indefinitely (deadlock, unreachable condition)
- Common in integration tests with mock servers

**Debug:**

```bash
# Run with shorter timeout
go test -timeout 30s -v -run TestProblemTest

# Add debug logging
func TestProblemTest(t *testing.T) {
    t.Log("Starting test...")

    server.Start()
    t.Log("Server started")

    // ...
    t.Log("Waiting for frames...")
    success := server.WaitForFrames(10, 5*time.Second)
    t.Logf("WaitForFrames result: %v", success)
}
```

**Common causes:**
- Mock server not starting (port conflict)
- DBus service name already taken (hue-sync running)
- Infinite loop in code
- Missing timeout in blocking call

**Fix:**
- Add timeouts to all blocking operations
- Check preconditions (server started, service not running)
- Use `t.Skip()` for conflicts

```go
err := service.Start()
if err != nil {
    if strings.Contains(err.Error(), "name already taken") {
        t.Skip("Service already running - stop hue-sync before testing")
    }
    t.Fatalf("Failed to start service: %v", err)
}
```

#### 3. Inconsistent Failures (Flaky Tests)

**Symptom:**
- Test passes locally, fails in CI
- Test fails randomly (~10% of time)

**Common causes:**
1. **Timing issues**: Test assumes operation completes instantly
2. **Race conditions**: `-race` not run locally
3. **Resource exhaustion**: CI has less memory/CPU
4. **Port conflicts**: Parallel tests using same port

**Debug:**

```bash
# Run test 100 times to reproduce
for i in {1..100}; do
    go test -run TestFlaky -count=1 || break
done

# Run with race detector
go test -race -run TestFlaky

# Run with verbose logging
go test -v -run TestFlaky
```

**Fix examples:**

**Timing issue:**

```go
// BAD: Assumes operation is instant
client.ActivateScene("scene-1")
scenes, _ := client.GetScenes()
if scenes[0].Active != true {
    t.Error("Scene not activated")
}

// GOOD: Add delay or polling
client.ActivateScene("scene-1")
time.Sleep(100 * time.Millisecond) // Allow propagation
scenes, _ := client.GetScenes()
```

**Resource cleanup:**

```go
// BAD: No cleanup, resources leak
func TestSomething(t *testing.T) {
    server := testutil.NewMockBridge()
    // ... test code ...
}

// GOOD: Cleanup with defer
func TestSomething(t *testing.T) {
    server := testutil.NewMockBridge()
    defer server.Close()
    // ... test code ...
}
```

#### 4. Mock Not Behaving Correctly

**Symptom:**

```
TestHueClient_GetScenes: integration_test.go:52: GetScenes() returned 0 scenes, want 3
```

**Diagnosis:**
- Mock not set up correctly
- Wrong mock method called
- Mock URL not extracted correctly

**Debug:**

```go
func TestHueClient_GetScenes(t *testing.T) {
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.SetupDefaultScenario()

    // DEBUG: Check mock state
    t.Logf("Mock has %d scenes", len(mock.Scenes))
    t.Logf("Mock URL: %s", mock.URL())

    bridgeAddr := mock.URL()[8:]
    t.Logf("Bridge address: %s", bridgeAddr)

    client := setupTestClient(t, bridgeAddr, "test-api-key")

    // Clear and check requests
    mock.ClearRequestLog()

    scenes, err := client.GetScenes()
    t.Logf("GetScenes returned %d scenes, error: %v", len(scenes), err)

    // Check what requests mock received
    requests := mock.GetRequestLog()
    t.Logf("Mock received %d requests:", len(requests))
    for i, req := range requests {
        t.Logf("  %d: %s", i, req)
    }
}
```

**Common fixes:**
- Ensure `SetupDefaultScenario()` called
- Check TLS config (InsecureSkipVerify)
- Verify URL extraction (`mock.URL()[8:]` removes `https://`)
- Check mock request log for actual calls

#### 5. File System Errors

**Symptom:**

```
config_test.go:45: Save() failed: open /root/.openhue/config.yaml: permission denied
```

**Diagnosis:**
- Test trying to write to real config path
- Missing `t.TempDir()` or path override

**Fix:**

```go
func TestConfigSave(t *testing.T) {
    // Use temporary directory
    tempDir := t.TempDir()
    configFile := filepath.Join(tempDir, "config.yaml")

    // Override config path functions
    originalGetConfigFile := getConfigFile
    originalGetConfigPath := getConfigPath
    getConfigFile = func() string {
        return configFile
    }
    getConfigPath = func() string {
        return tempDir
    }
    defer func() {
        getConfigFile = originalGetConfigFile
        getConfigPath = originalGetConfigPath
    }()

    // Now test can write safely
    cfg := config.DefaultConfig()
    cfg.Bridge = "192.168.1.100"
    cfg.Key = "test-key"
    err := cfg.Save()
    if err != nil {
        t.Fatalf("Save() failed: %v", err)
    }
}
```

### Debugging Tools

#### 1. Verbose Test Output

```bash
# Show all t.Logf() messages
go test -v -run TestSpecific

# Show test names as they run
go test -v ./...
```

#### 2. Test Binary Inspection

```bash
# Compile test binary without running
go test -c -o test.bin ./internal/config

# Run specific test
./test.bin -test.v -test.run TestValidate

# Run with debugger
dlv exec ./test.bin -- -test.v -test.run TestValidate
```

#### 3. Print Debugging

```go
func TestDebug(t *testing.T) {
    // Use t.Logf (only shows if -v or test fails)
    t.Logf("Debug: value=%d", value)

    // Use fmt.Printf (always shows, messes up test output)
    fmt.Printf("DEBUG: value=%d\n", value)

    // Use t.Helper() for helper functions
    func verify(t *testing.T, value int) {
        t.Helper() // Makes error point to caller, not this line
        if value != 42 {
            t.Errorf("Expected 42, got %d", value)
        }
    }
}
```

#### 4. Test Isolation

```bash
# Run single test
go test -run TestConfigValidate

# Run tests matching pattern
go test -run TestConfig  # Runs all tests starting with TestConfig

# Disable test parallelism
go test -p 1 -parallel 1

# Run with coverage to find untested paths
go test -coverprofile=coverage.out -run TestProblem
go tool cover -html=coverage.out
```

#### 5. Mock Debugging

```go
// Check mock state
t.Logf("Mock scenes: %+v", mock.Scenes)
t.Logf("Mock request log: %+v", mock.GetRequestLog())

// Add error to specific endpoint
mock.SetResponseError("/clip/v2/resource/scene", fmt.Errorf("forced error"))

// Check what mock returned
mock.ClearRequestLog()
client.GetScenes()
requests := mock.GetRequestLog()
t.Logf("Requests made: %+v", requests)
```

---

## Best Practices

### General Testing Principles

1. **Test behavior, not implementation**
   - ✅ Test what function returns/does
   - ❌ Don't test internal variables

2. **Keep tests independent**
   - Each test should set up its own state
   - No shared global state between tests
   - Use `t.Run()` for subtests

3. **Use descriptive test names**
   - ✅ `TestValidate_MissingBridge_ReturnsError`
   - ❌ `TestValidate1`

4. **Clean up resources**
   - Always `defer cleanup()`
   - Use `t.TempDir()` for file tests
   - Close mock servers

5. **Test error paths**
   - Don't just test happy path
   - Test validation failures
   - Test error handling

### Unit Test Best Practices

1. **Use table-driven tests**
   ```go
   tests := []struct {
       name string
       input int
       want int
   }{
       {"zero", 0, 0},
       {"positive", 5, 25},
       {"negative", -3, 9},
   }
   ```

2. **Test boundaries**
   - Min/max values
   - Zero values
   - Negative values
   - Empty strings/slices

3. **Use `t.Helper()` for test utilities**
   ```go
   func assertEqual(t *testing.T, got, want int) {
       t.Helper()
       if got != want {
           t.Errorf("got %d, want %d", got, want)
       }
   }
   ```

4. **Avoid test interdependence**
   - Don't rely on test execution order
   - Each test should be runnable in isolation

### Integration Test Best Practices

1. **Use build tags**
   ```go
   //go:build integration
   ```

2. **Skip gracefully on conflicts**
   ```go
   if strings.Contains(err.Error(), "already running") {
       t.Skip("Service already running")
   }
   ```

3. **Set reasonable timeouts**
   ```go
   go test -tags=integration -timeout 30s
   ```

4. **Mock external dependencies**
   - Don't depend on real Hue bridge
   - Use `testutil.MockBridge` and `testutil.MockEntertainmentServer`

5. **Verify side effects**
   ```go
   // Check mock received expected requests
   requests := mock.GetRequestLog()
   if len(requests) != 2 {
       t.Errorf("Expected 2 requests, got %d", len(requests))
   }
   ```

### Benchmark Best Practices

1. **Reset timer after setup**
   ```go
   b.ResetTimer()
   ```

2. **Use realistic data sizes**
   - Test with 1440p/4K images
   - Use typical scene counts
   - Match production workloads

3. **Prevent compiler optimization**
   ```go
   var result int
   for i := 0; i < b.N; i++ {
       result = expensiveFunc(data)
   }
   _ = result // Prevent optimization
   ```

4. **Run with -benchmem**
   ```bash
   go test -bench=. -benchmem
   ```

5. **Save baselines for comparison**
   ```bash
   go test -bench=. > old.txt
   # Make changes
   go test -bench=. > new.txt
   benchcmp old.txt new.txt
   ```

### Mock Best Practices

1. **Keep mocks simple**
   - Only mock what's needed
   - Don't over-engineer mocks

2. **Make mocks thread-safe**
   ```go
   type MockBridge struct {
       mu sync.RWMutex
       // ...
   }
   ```

3. **Provide setup helpers**
   ```go
   mock.SetupDefaultScenario() // Pre-populates common data
   ```

4. **Log requests for debugging**
   ```go
   mock.ClearRequestLog()
   // ... make calls ...
   t.Logf("Requests: %+v", mock.GetRequestLog())
   ```

### Code Review Checklist

Before submitting PR:

- [ ] All tests pass: `go test ./...`
- [ ] Race detector passes: `go test -race ./...`
- [ ] Code formatted: `gofmt -s -w .`
- [ ] Static analysis passes: `go vet ./...`
- [ ] Coverage doesn't drop: `go test -coverprofile=coverage.out ./...`
- [ ] Integration tests pass (if relevant): `go test -tags=integration -v`
- [ ] Benchmarks don't regress (if performance-critical)
- [ ] New features have tests
- [ ] Error paths tested
- [ ] Documentation updated

---

## Quick Reference

### Test Commands Cheat Sheet

```bash
# Unit tests (fast)
go test ./...
go test ./internal/config -v
go test -run TestValidate ./internal/config

# Integration tests (slower)
go test -tags=integration -v
go test -tags=integration -run TestDBus -v

# Race detection
go test -race ./...

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Benchmarks
go test -bench=. ./internal/color
go test -bench=BenchmarkExtractColors -benchmem -benchtime=10s

# Debug
go test -v -run TestSpecific
go test -timeout 30s -run TestProblem
```

### Test File Templates

**Unit test:**
```go
package mypackage

import "testing"

func TestFunction(t *testing.T) {
    tests := []struct {
        name string
        input int
        want int
    }{
        {"case1", 0, 0},
        {"case2", 5, 25},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Function(tt.input)
            if got != tt.want {
                t.Errorf("got %d, want %d", got, tt.want)
            }
        })
    }
}
```

**Integration test:**
```go
//go:build integration

package main

import (
    "testing"
    "github.com/codepuncher/khuey/internal/testutil"
)

func TestIntegration(t *testing.T) {
    mock := testutil.NewMockBridgeTLS()
    defer mock.Close()
    mock.SetupDefaultScenario()

    // Test code...
}
```

**Benchmark:**
```go
func BenchmarkFunction(b *testing.B) {
    data := setupData()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        Function(data)
    }
}
```

---

## Conclusion

This guide covers the complete testing workflow for KDE Hue Control:

- **Run tests early and often** during development
- **Use table-driven tests** for comprehensive coverage
- **Mock external dependencies** with `testutil` package
- **Test error paths** not just happy paths
- **Benchmark performance-critical code**
- **Run with `-race`** before committing
- **Maintain test coverage** (10% minimum, aim for 60%+)

**When in doubt:** Look at existing tests for examples. The codebase has extensive test coverage across unit, integration, and benchmark tests.

**Questions?** See existing test files:
- `internal/config/config_test.go` - Config validation patterns
- `internal/dbus/service_test.go` - DBus method testing
- `backend/integration_test.go` - Integration test examples
- `internal/color/extractor_bench_test.go` - Benchmark patterns

Happy testing! 🧪
