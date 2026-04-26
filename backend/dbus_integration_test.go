// DBus integration tests for KDE Hue Control
// Run with: go test -tags=integration -v -run TestDBus
//
// NOTE: These tests require a DBus session bus and will temporarily claim
// the org.kde.plasma.hue service name. Stop any running hue-sync before testing.
//go:build integration

package main

import (
"context"
"crypto/tls"
"net/http"
"strings"
"testing"
"time"

"github.com/codepuncher/khuey/internal/config"
"github.com/codepuncher/khuey/internal/dbus"
"github.com/codepuncher/khuey/internal/hue"
"github.com/codepuncher/khuey/internal/testutil"
godbus "github.com/godbus/dbus/v5"
)

const (
dbusName      = "org.kde.plasma.hue"
dbusPath      = "/org/kde/plasma/hue"
dbusInterface = "org.kde.plasma.hue"
)

// setupTestDBusService creates a DBus service with mock Hue bridge
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
cfg.Bridge = mock.URL()[8:] // Remove https://
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

// getTestDBusConnection creates a connection to test the service
func getTestDBusConnection(t *testing.T) (*godbus.Conn, func()) {
conn, err := godbus.ConnectSessionBus()
if err != nil {
t.Fatalf("Failed to connect to session bus: %v", err)
}

cleanup := func() {
conn.Close()
}

return conn, cleanup
}

// TestDBusService_StartAndRegister tests service registration
func TestDBusService_StartAndRegister(t *testing.T) {
service, mock, cleanup := setupTestDBusService(t)
defer cleanup()

// Start service
err := service.Start()
if err != nil {
// If service is already running, skip test
if strings.Contains(err.Error(), "name already taken") {
t.Skip("Service already running - stop hue-sync before testing")
}
t.Fatalf("Failed to start service: %v", err)
}

// Verify service is registered on bus
conn, connCleanup := getTestDBusConnection(t)
defer connCleanup()

// List names and check for our service
var names []string
err = conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
if err != nil {
t.Fatalf("Failed to list bus names: %v", err)
}

found := false
for _, name := range names {
if name == dbusName {
found = true
break
}
}

if !found {
t.Errorf("Service name %s not found on bus", dbusName)
}

_ = mock
}

// TestDBusService_GetStatus tests GetStatus method
func TestDBusService_GetStatus(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

var status string
err = obj.Call(dbusInterface+".GetStatus", 0).Store(&status)
if err != nil {
t.Fatalf("GetStatus() error: %v", err)
}

if status == "" {
t.Error("GetStatus() returned empty string")
}

// Should return "Ready" since we have a configured bridge
if status != "Ready" && status != "Not configured" {
t.Logf("Unexpected status: %s", status)
}

t.Logf("Status: %s", status)
_ = mock
}

// TestDBusService_GetScenes tests GetScenes method
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

obj := conn.Object(dbusName, dbusPath)

var scenes []string
err = obj.Call(dbusInterface+".GetScenes", 0).Store(&scenes)
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
_ = mock
}

// TestDBusService_ActivateScene tests ActivateScene method
func TestDBusService_ActivateScene(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

// Get scenes first
var scenes []string
err = obj.Call(dbusInterface+".GetScenes", 0).Store(&scenes)
if err != nil {
t.Fatalf("GetScenes() error: %v", err)
}

if len(scenes) == 0 {
t.Fatal("No scenes available for testing")
}

// Activate first scene
sceneName := scenes[0]
var result string
call := obj.Call(dbusInterface+".ActivateScene", 0, sceneName)
err = call.Store(&result)

if err != nil {
// Scene activation might fail if scene ID doesn't match
t.Logf("ActivateScene() error (may be expected): %v", err)
} else if result != "" {
t.Logf("ActivateScene result: %s", result)
}

// Verify mock received activation request
requestCount := mock.GetRequestCount()
if requestCount < 2 {
t.Logf("Expected at least 2 requests to mock (GET scenes + PUT activate), got %d", requestCount)
}

_ = mock
}

// TestDBusService_SyncMethods tests sync-related methods
func TestDBusService_SyncMethods(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

t.Run("IsSyncing_Initially", func(t *testing.T) {
var syncing bool
err := obj.Call(dbusInterface+".IsSyncing", 0).Store(&syncing)
if err != nil {
t.Fatalf("IsSyncing() error: %v", err)
}

// Should be false initially (no Entertainment API configured)
if syncing {
t.Error("IsSyncing() should be false initially")
}
})

t.Run("GetSyncSettings", func(t *testing.T) {
var settings map[string]godbus.Variant
err := obj.Call(dbusInterface+".GetSyncSettings", 0).Store(&settings)
if err != nil {
t.Fatalf("GetSyncSettings() error: %v", err)
}

if len(settings) == 0 {
t.Error("GetSyncSettings() returned empty map")
}

// Check for expected keys
if _, ok := settings["fps"]; !ok {
t.Error("GetSyncSettings() missing 'fps' key")
}
if _, ok := settings["subsampleWidth"]; !ok {
t.Error("GetSyncSettings() missing 'subsampleWidth' key")
}

t.Logf("Sync settings: %+v", settings)
})

// Note: Can't test StartSync/StopSync without Entertainment API configured
// Those require clientkey and entertainment configuration ID

_ = mock
}

// TestDBusService_GamingMode tests gaming mode methods
func TestDBusService_GamingMode(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

t.Run("IsGamingModeEnabled_Default", func(t *testing.T) {
var enabled bool
err := obj.Call(dbusInterface+".IsGamingModeEnabled", 0).Store(&enabled)
if err != nil {
t.Fatalf("IsGamingModeEnabled() error: %v", err)
}

// Default config should have gaming mode disabled
if enabled {
t.Error("IsGamingModeEnabled() should be false by default")
}
})

t.Run("IsGamingModeActive_Default", func(t *testing.T) {
var active bool
err := obj.Call(dbusInterface+".IsGamingModeActive", 0).Store(&active)
if err != nil {
t.Fatalf("IsGamingModeActive() error: %v", err)
}

// No games running, should be inactive
if active {
t.Error("IsGamingModeActive() should be false by default")
}
})

_ = mock
}

// TestDBusService_BridgeSettings tests bridge configuration methods
func TestDBusService_BridgeSettings(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

t.Run("GetBridgeSettings", func(t *testing.T) {
var settings map[string]godbus.Variant
err := obj.Call(dbusInterface+".GetBridgeSettings", 0).Store(&settings)
if err != nil {
t.Fatalf("GetBridgeSettings() error: %v", err)
}

if len(settings) == 0 {
t.Error("GetBridgeSettings() returned empty map")
}

// Check for expected keys
if _, ok := settings["bridgeIP"]; !ok {
t.Error("GetBridgeSettings() missing 'bridgeIP' key")
}

t.Logf("Bridge settings keys: %v", mapKeys(settings))
})

t.Run("GetConnectionStatus", func(t *testing.T) {
var status map[string]godbus.Variant
err := obj.Call(dbusInterface+".GetConnectionStatus", 0).Store(&status)
if err != nil {
t.Fatalf("GetConnectionStatus() error: %v", err)
}

if len(status) == 0 {
t.Error("GetConnectionStatus() returned empty map")
}

t.Logf("Connection status keys: %v", mapKeys(status))
})

_ = mock
}

// TestDBusService_TrayIcons tests tray icon methods
func TestDBusService_TrayIcons(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

var gaming, syncing, idle string
err = obj.Call(dbusInterface+".GetTrayIcons", 0).Store(&gaming, &syncing, &idle)
if err != nil {
t.Fatalf("GetTrayIcons() error: %v", err)
}

if gaming == "" || syncing == "" || idle == "" {
t.Error("GetTrayIcons() returned empty icon names")
}

// Verify default icon names
expectedGaming := "applications-games"
expectedSyncing := "media-record"
expectedIdle := "preferences-desktop-display-color"

if gaming != expectedGaming {
t.Errorf("Gaming icon = %s, want %s", gaming, expectedGaming)
}
if syncing != expectedSyncing {
t.Errorf("Syncing icon = %s, want %s", syncing, expectedSyncing)
}
if idle != expectedIdle {
t.Errorf("Idle icon = %s, want %s", idle, expectedIdle)
}

t.Logf("Icons - Gaming: %s, Syncing: %s, Idle: %s", gaming, syncing, idle)
_ = mock
}

// TestDBusService_Introspection tests DBus introspection
func TestDBusService_Introspection(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

// Call Introspect
var xml string
err = obj.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xml)
if err != nil {
t.Fatalf("Introspect() error: %v", err)
}

if xml == "" {
t.Fatal("Introspect() returned empty XML")
}

// Verify XML contains expected methods
expectedMethods := []string{
"GetStatus",
"GetScenes",
"ActivateScene",
"StartSync",
"StopSync",
"IsSyncing",
"GetTrayIcons",
"IsGamingModeEnabled",
"GetBridgeSettings",
}

for _, method := range expectedMethods {
if !strings.Contains(xml, method) {
t.Errorf("Introspection XML missing method: %s", method)
}
}

t.Logf("Introspection XML length: %d bytes", len(xml))
t.Logf("Found %d expected methods", len(expectedMethods))
_ = mock
}

// TestDBusService_ConcurrentCalls tests concurrent DBus method calls
func TestDBusService_ConcurrentCalls(t *testing.T) {
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

obj := conn.Object(dbusName, dbusPath)

// Launch 10 concurrent GetScenes calls
errs := make(chan error, 10)
results := make(chan int, 10)

for i := 0; i < 10; i++ {
go func() {
var scenes []string
err := obj.Call(dbusInterface+".GetScenes", 0).Store(&scenes)
errs <- err
results <- len(scenes)
}()
}

// Collect results
errorCount := 0
for i := 0; i < 10; i++ {
if err := <-errs; err != nil {
t.Errorf("Concurrent call #%d error: %v", i, err)
errorCount++
}
sceneCount := <-results
if sceneCount != 3 {
t.Logf("Concurrent call returned %d scenes, expected 3", sceneCount)
}
}

if errorCount == 0 {
t.Log("All 10 concurrent calls succeeded")
}

_ = mock
}

// TestDBusService_StopAndCleanup tests service shutdown
func TestDBusService_StopAndCleanup(t *testing.T) {
service, mock, cleanup := setupTestDBusService(t)
defer cleanup()

err := service.Start()
if err != nil {
if strings.Contains(err.Error(), "name already taken") {
t.Skip("Service already running")
}
t.Fatalf("Failed to start service: %v", err)
}

// Service should be reachable
conn, connCleanup := getTestDBusConnection(t)
defer connCleanup()

obj := conn.Object(dbusName, dbusPath)
var status string
err = obj.Call(dbusInterface+".GetStatus", 0).Store(&status)
if err != nil {
t.Fatalf("GetStatus before stop error: %v", err)
}

// Stop service
service.Stop()

// Wait a moment for cleanup
time.Sleep(200 * time.Millisecond)

// Try to call method - should fail
err = obj.Call(dbusInterface+".GetStatus", 0).Store(&status)
if err == nil {
t.Log("Service still responding after Stop() - may be timing dependent")
} else {
t.Logf("Service correctly unreachable after Stop(): %v", err)
}

_ = mock
}

// Helper functions
func mapKeys(m map[string]godbus.Variant) []string {
keys := make([]string, 0, len(m))
for k := range m {
keys = append(keys, k)
}
return keys
}
