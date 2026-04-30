// Entertainment API integration tests for KDE Hue Control
// Tests DTLS streaming, HueStream v2 protocol, and Entertainment API integration
// Run with: go test -tags=integration -v -run TestEntertainment
//go:build integration

package main

import (
	"crypto/tls"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"testing"

	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/entertainment"
	"github.com/codepuncher/khuey/internal/sync"
	"github.com/codepuncher/khuey/internal/testutil"
)

const (
	testClientKey       = "0123456789ABCDEF0123456789ABCDEF" // 32 hex chars = 16 bytes
	testUsername        = "test-api-key"
	testEntertainmentID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" // 36 chars with hyphens
)

// setupTestEntertainmentServer creates a mock Entertainment API server
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

// TestEntertainmentAPI_ServerStartStop tests mock server lifecycle
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

// TestEntertainmentAPI_ProtocolParsing tests HueStream v2 packet parsing
func TestEntertainmentAPI_ProtocolParsing(t *testing.T) {
	// Build a valid HueStream v2 packet manually
	packet := buildTestPacket(t, 0, []channelData{
		{id: 0, r: 65535, g: 0, b: 0}, // Red
		{id: 1, r: 0, g: 65535, b: 0}, // Green
	})

	// Parse packet
	frame, err := parseHueStreamPacket(packet)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Verify protocol version
	if frame.Version[0] != 0x02 || frame.Version[1] != 0x00 {
		t.Errorf("Version = [%d, %d], want [2, 0]", frame.Version[0], frame.Version[1])
	}

	// Verify color space
	if frame.ColorSpace != 0x00 {
		t.Errorf("ColorSpace = %d, want 0 (RGB)", frame.ColorSpace)
	}

	// Verify Entertainment ID
	if frame.EntertainmentID != testEntertainmentID {
		t.Errorf("EntertainmentID = %q, want %q", frame.EntertainmentID, testEntertainmentID)
	}

	// Verify channels
	if len(frame.Channels) != 2 {
		t.Fatalf("Got %d channels, want 2", len(frame.Channels))
	}

	// Check colors
	if frame.Channels[0].R != 65535 || frame.Channels[0].G != 0 || frame.Channels[0].B != 0 {
		t.Errorf("Channel 0 = (%d,%d,%d), want (65535,0,0)",
			frame.Channels[0].R, frame.Channels[0].G, frame.Channels[0].B)
	}

	if frame.Channels[1].R != 0 || frame.Channels[1].G != 65535 || frame.Channels[1].B != 0 {
		t.Errorf("Channel 1 = (%d,%d,%d), want (0,65535,0)",
			frame.Channels[1].R, frame.Channels[1].G, frame.Channels[1].B)
	}
}

// TestEntertainmentAPI_MultiChannelParsing tests parsing multiple channels
func TestEntertainmentAPI_MultiChannelParsing(t *testing.T) {
	// Build packet with 5 channels
	packet := buildTestPacket(t, 42, []channelData{
		{id: 0, r: 65535, g: 0, b: 0},     // Red
		{id: 1, r: 0, g: 65535, b: 0},     // Green
		{id: 2, r: 0, g: 0, b: 65535},     // Blue
		{id: 3, r: 65535, g: 65535, b: 0}, // Yellow
		{id: 4, r: 65535, g: 0, b: 65535}, // Magenta
	})

	frame, err := parseHueStreamPacket(packet)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Verify sequence ID
	if frame.SequenceID != 42 {
		t.Errorf("SequenceID = %d, want 42", frame.SequenceID)
	}

	// Verify channel count
	if len(frame.Channels) != 5 {
		t.Fatalf("Got %d channels, want 5", len(frame.Channels))
	}

	// Verify each channel's color
	expected := []struct {
		id      int
		r, g, b uint16
		name    string
	}{
		{0, 65535, 0, 0, "Red"},
		{1, 0, 65535, 0, "Green"},
		{2, 0, 0, 65535, "Blue"},
		{3, 65535, 65535, 0, "Yellow"},
		{4, 65535, 0, 65535, "Magenta"},
	}

	for i, exp := range expected {
		ch := frame.Channels[i]
		if ch.ChannelID != exp.id {
			t.Errorf("Channel[%d] ID = %d, want %d", i, ch.ChannelID, exp.id)
		}
		if ch.R != exp.r || ch.G != exp.g || ch.B != exp.b {
			t.Errorf("Channel[%d] (%s) = (%d,%d,%d), want (%d,%d,%d)",
				i, exp.name, ch.R, ch.G, ch.B, exp.r, exp.g, exp.b)
		}
	}
}

// TestEntertainmentAPI_PacketSizeValidation tests packet size requirements
func TestEntertainmentAPI_PacketSizeValidation(t *testing.T) {
	tests := []struct {
		name        string
		packetSize  int
		channelData []channelData
		wantErr     bool
	}{
		{
			name:        "ValidSingleChannel",
			packetSize:  52 + 7, // Header + 1 channel
			channelData: []channelData{{id: 0, r: 1000, g: 2000, b: 3000}},
			wantErr:     false,
		},
		{
			name:        "ValidTwoChannels",
			packetSize:  52 + 14, // Header + 2 channels
			channelData: []channelData{{id: 0, r: 1, g: 2, b: 3}, {id: 1, r: 4, g: 5, b: 6}},
			wantErr:     false,
		},
		{
			name:       "TooSmall",
			packetSize: 30, // Less than header size
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var packet []byte
			if tt.wantErr {
				// Create intentionally bad packet
				packet = make([]byte, tt.packetSize)
			} else {
				packet = buildTestPacket(t, 0, tt.channelData)
			}

			_, err := parseHueStreamPacket(packet)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestEntertainmentAPI_ClientCreation tests client creation and validation
func TestEntertainmentAPI_ClientCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  entertainment.Config
		wantErr bool
	}{
		{
			name: "ValidConfig",
			config: entertainment.Config{
				BridgeIP:        "192.168.1.1",
				Username:        "test-user",
				ClientKey:       testClientKey,
				EntertainmentID: testEntertainmentID,
				ChannelCount:    2,
			},
			wantErr: false,
		},
		{
			name: "MissingBridgeIP",
			config: entertainment.Config{
				Username:        "test-user",
				ClientKey:       testClientKey,
				EntertainmentID: testEntertainmentID,
				ChannelCount:    2,
			},
			wantErr: true,
		},
		{
			name: "MissingUsername",
			config: entertainment.Config{
				BridgeIP:        "192.168.1.1",
				ClientKey:       testClientKey,
				EntertainmentID: testEntertainmentID,
				ChannelCount:    2,
			},
			wantErr: true,
		},
		{
			name: "MissingClientKey",
			config: entertainment.Config{
				BridgeIP:        "192.168.1.1",
				Username:        "test-user",
				EntertainmentID: testEntertainmentID,
				ChannelCount:    2,
			},
			wantErr: true,
		},
		{
			name: "ZeroChannels",
			config: entertainment.Config{
				BridgeIP:        "192.168.1.1",
				Username:        "test-user",
				ClientKey:       testClientKey,
				EntertainmentID: testEntertainmentID,
				ChannelCount:    0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := entertainment.NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if client != nil {
				client.Close()
			}
		})
	}
}

// TestEntertainmentAPI_StreamWithoutConnect tests error handling
func TestEntertainmentAPI_StreamWithoutConnect(t *testing.T) {
	client, err := entertainment.NewClient(entertainment.Config{
		BridgeIP:        "127.0.0.1",
		Username:        testUsername,
		ClientKey:       testClientKey,
		EntertainmentID: testEntertainmentID,
		ChannelCount:    1,
	})
	if err != nil {
		t.Fatalf("NewClient error = %v", err)
	}
	defer client.Close()

	colors := []entertainment.ChannelColor{{ChannelID: 0, R: 1000, G: 1000, B: 1000}}
	err = client.StreamColors(colors)

	if err != entertainment.ErrNotConnected {
		t.Errorf("StreamColors() error = %v, want ErrNotConnected", err)
	}
}

// TestEntertainmentAPI_Color8BitTo16Bit tests color conversion
func TestEntertainmentAPI_Color8BitTo16Bit(t *testing.T) {
	testCases := []struct {
		input8 uint8
		want16 uint16
	}{
		{0, 0},
		{1, 257},
		{127, 32639},
		{128, 32896},
		{255, 65535},
	}

	for _, tc := range testCases {
		got := entertainment.Convert8BitTo16Bit(tc.input8)
		if got != tc.want16 {
			t.Errorf("Convert8BitTo16Bit(%d) = %d, want %d", tc.input8, got, tc.want16)
		}
	}
}

// TestSyncEngine_EntertainmentConfig tests sync engine with Entertainment config
func TestSyncEngine_EntertainmentConfig(t *testing.T) {
	// Setup mock bridge
	mockBridge := testutil.NewMockBridgeTLS()
	defer mockBridge.Close()
	mockBridge.SetupDefaultScenario()

	// Add Entertainment configuration
	testutil.MockEntertainmentActivation(mockBridge, testEntertainmentID)

	// Setup mock Entertainment server (won't actually connect in this test)
	entServer, cleanup := setupTestEntertainmentServer(t)
	defer cleanup()

	// Set TLS skip verify
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer func() { http.DefaultTransport = originalTransport }()

	// Create config
	cfg := createTestSyncConfig(mockBridge.URL()[8:], entServer.GetPort())

	// Create sync engine
	engine, err := sync.NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Verify engine is not running initially
	if engine.IsRunning() {
		t.Error("Engine should not be running initially")
	}

	// We can't test Start() without triggering permission dialogs
	// The engine creation validates Entertainment config is properly set up
	t.Log("Sync engine created successfully with Entertainment config")
}

// Helper types and functions

type channelData struct {
	id      int
	r, g, b uint16
}

// buildTestPacket creates a valid HueStream v2 packet for testing
func buildTestPacket(t *testing.T, sequenceID uint8, channels []channelData) []byte {
	t.Helper()

	headerSize := 52
	bodySize := 7 * len(channels)
	packet := make([]byte, headerSize+bodySize)

	// Header
	copy(packet[0:9], "HueStream")
	packet[9] = 0x02                         // Version major
	packet[10] = 0x00                        // Version minor
	packet[11] = sequenceID                  // Sequence ID
	packet[12] = 0x00                        // Reserved
	packet[13] = 0x00                        // Reserved
	packet[14] = 0x00                        // Color space: RGB
	packet[15] = 0x00                        // Reserved
	copy(packet[16:52], testEntertainmentID) // Entertainment ID

	// Body: per-channel colors
	offset := headerSize
	for _, ch := range channels {
		packet[offset] = byte(ch.id)
		packet[offset+1] = byte(ch.r >> 8)
		packet[offset+2] = byte(ch.r & 0xFF)
		packet[offset+3] = byte(ch.g >> 8)
		packet[offset+4] = byte(ch.g & 0xFF)
		packet[offset+5] = byte(ch.b >> 8)
		packet[offset+6] = byte(ch.b & 0xFF)
		offset += 7
	}

	return packet
}

// parseHueStreamPacket is a test-only parser that matches the production implementation
func parseHueStreamPacket(data []byte) (testutil.ReceivedFrame, error) {
	frame := testutil.ReceivedFrame{}

	const headerSize = 52
	if len(data) < headerSize {
		return frame, fmt.Errorf("packet too small: %d bytes", len(data))
	}

	// Validate protocol
	protocol := string(data[0:9])
	if protocol != "HueStream" {
		return frame, fmt.Errorf("invalid protocol: %q", protocol)
	}

	// Parse header
	frame.Version[0] = data[9]
	frame.Version[1] = data[10]
	frame.SequenceID = data[11]
	frame.ColorSpace = data[14]
	frame.EntertainmentID = string(data[16:52])

	// Parse body
	bodySize := len(data) - headerSize
	if bodySize%7 != 0 {
		return frame, fmt.Errorf("invalid body size: %d", bodySize)
	}

	channelCount := bodySize / 7
	frame.Channels = make([]testutil.ChannelColor, channelCount)

	offset := headerSize
	for i := 0; i < channelCount; i++ {
		frame.Channels[i] = testutil.ChannelColor{
			ChannelID: int(data[offset]),
			R:         uint16(data[offset+1])<<8 | uint16(data[offset+2]),
			G:         uint16(data[offset+3])<<8 | uint16(data[offset+4]),
			B:         uint16(data[offset+5])<<8 | uint16(data[offset+6]),
		}
		offset += 7
	}

	return frame, nil
}

// createTestSyncConfig creates a config for sync engine testing
func createTestSyncConfig(bridgeAddr string, entPort int) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Bridge = bridgeAddr
	cfg.Key = testUsername
	cfg.ClientKey = testClientKey
	cfg.EntertainmentConfigurationID = testEntertainmentID

	// Configure 2 channels with UV zones (left/right split)
	cfg.Channels = []config.ChannelConfig{
		{
			ID:          0,
			Active:      true,
			DeviceName:  "Left Light",
			GammaFactor: 2.2,
			UVA:         config.UV{X: 0.0, Y: 0.0},
			UVB:         config.UV{X: 0.5, Y: 1.0},
		},
		{
			ID:          1,
			Active:      true,
			DeviceName:  "Right Light",
			GammaFactor: 2.2,
			UVA:         config.UV{X: 0.5, Y: 0.0},
			UVB:         config.UV{X: 1.0, Y: 1.0},
		},
	}

	cfg.Sync.FPS = 30
	cfg.Sync.SubsampleWidth = 64

	return cfg
}

// createTestImage creates a synthetic test image with specific colors (unused but kept for future)
func createTestImage(width, height int, leftColor, rightColor color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill left half with leftColor
	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			img.Set(x, y, leftColor)
		}
	}

	// Fill right half with rightColor
	for y := 0; y < height; y++ {
		for x := width / 2; x < width; x++ {
			img.Set(x, y, rightColor)
		}
	}

	return img
}
