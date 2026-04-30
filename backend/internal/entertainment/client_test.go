package entertainment

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestNewClient tests the client constructor
func TestNewClient(t *testing.T) {
	tests := []struct {
		name          string
		cfg           Config
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid configuration",
			cfg: Config{
				BridgeIP:        "192.168.1.100",
				Username:        "test-user",
				ClientKey:       "AABBCCDD",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    2,
			},
			expectError: false,
		},
		{
			name: "Missing BridgeIP",
			cfg: Config{
				Username:        "test-user",
				ClientKey:       "AABBCCDD",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    2,
			},
			expectError:   true,
			errorContains: "bridge IP is required",
		},
		{
			name: "Missing Username",
			cfg: Config{
				BridgeIP:        "192.168.1.100",
				ClientKey:       "AABBCCDD",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    2,
			},
			expectError:   true,
			errorContains: "username is required",
		},
		{
			name: "Missing ClientKey",
			cfg: Config{
				BridgeIP:        "192.168.1.100",
				Username:        "test-user",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    2,
			},
			expectError:   true,
			errorContains: "client key is required",
		},
		{
			name: "Missing EntertainmentID",
			cfg: Config{
				BridgeIP:     "192.168.1.100",
				Username:     "test-user",
				ClientKey:    "AABBCCDD",
				ChannelCount: 2,
			},
			expectError:   true,
			errorContains: "entertainment ID is required",
		},
		{
			name: "Zero ChannelCount",
			cfg: Config{
				BridgeIP:        "192.168.1.100",
				Username:        "test-user",
				ClientKey:       "AABBCCDD",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    0,
			},
			expectError:   true,
			errorContains: "channel count must be positive",
		},
		{
			name: "Negative ChannelCount",
			cfg: Config{
				BridgeIP:        "192.168.1.100",
				Username:        "test-user",
				ClientKey:       "AABBCCDD",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    -1,
			},
			expectError:   true,
			errorContains: "channel count must be positive",
		},
		{
			name: "Large ChannelCount",
			cfg: Config{
				BridgeIP:        "192.168.1.100",
				Username:        "test-user",
				ClientKey:       "AABBCCDD",
				EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
				ChannelCount:    100,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)

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
				if client == nil {
					t.Errorf("Expected client but got nil")
				} else {
					// Verify client fields
					if client.bridgeIP != tt.cfg.BridgeIP {
						t.Errorf("Expected bridgeIP %q, got %q", tt.cfg.BridgeIP, client.bridgeIP)
					}
					if client.username != tt.cfg.Username {
						t.Errorf("Expected username %q, got %q", tt.cfg.Username, client.username)
					}
					if client.clientKey != tt.cfg.ClientKey {
						t.Errorf("Expected clientKey %q, got %q", tt.cfg.ClientKey, client.clientKey)
					}
				}
				if client.entertainmentID != tt.cfg.EntertainmentID {
					t.Errorf("Expected entertainmentID %q, got %q", tt.cfg.EntertainmentID, client.entertainmentID)
				}
				if client.channelCount != tt.cfg.ChannelCount {
					t.Errorf("Expected channelCount %d, got %d", tt.cfg.ChannelCount, client.channelCount)
				}
				if client.sequenceID != 0 {
					t.Errorf("Expected initial sequenceID 0, got %d", client.sequenceID)
				}
				if client.connected {
					t.Errorf("Expected initial connected=false, got true")
				}
			}
		})
	}
}

// TestBuildPacket tests the HueStream v2 packet building
func TestBuildPacket(t *testing.T) {
	client, err := NewClient(Config{
		BridgeIP:        "192.168.1.100",
		Username:        "test-user",
		ClientKey:       "AABBCCDD",
		EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
		ChannelCount:    2,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name   string
		colors []ChannelColor
	}{
		{
			name:   "Empty colors",
			colors: []ChannelColor{},
		},
		{
			name: "Single channel",
			colors: []ChannelColor{
				{ChannelID: 0, R: 65535, G: 32768, B: 16384},
			},
		},
		{
			name: "Multiple channels",
			colors: []ChannelColor{
				{ChannelID: 0, R: 65535, G: 0, B: 0},
				{ChannelID: 1, R: 0, G: 65535, B: 0},
				{ChannelID: 2, R: 0, G: 0, B: 65535},
			},
		},
		{
			name: "Max values",
			colors: []ChannelColor{
				{ChannelID: 255, R: 65535, G: 65535, B: 65535},
			},
		},
		{
			name: "Min values",
			colors: []ChannelColor{
				{ChannelID: 0, R: 0, G: 0, B: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := client.buildPacket(tt.colors)

			// Check packet size
			expectedSize := 52 + (7 * len(tt.colors))
			if len(packet) != expectedSize {
				t.Errorf("Expected packet size %d, got %d", expectedSize, len(packet))
			}

			// Verify header
			// Protocol name: "HueStream"
			if !bytes.Equal(packet[0:9], []byte("HueStream")) {
				t.Errorf("Expected protocol 'HueStream', got %q", packet[0:9])
			}

			// Version: 0x0200
			if packet[9] != 0x02 || packet[10] != 0x00 {
				t.Errorf("Expected version 0x0200, got 0x%02x%02x", packet[9], packet[10])
			}

			// Sequence ID (should be current value)
			if packet[11] != client.sequenceID {
				t.Errorf("Expected sequence ID %d, got %d", client.sequenceID, packet[11])
			}

			// Reserved bytes
			if packet[12] != 0x00 || packet[13] != 0x00 {
				t.Errorf("Expected reserved 0x0000, got 0x%02x%02x", packet[12], packet[13])
			}

			// Color space: 0x00 (RGB)
			if packet[14] != 0x00 {
				t.Errorf("Expected color space 0x00, got 0x%02x", packet[14])
			}

			// Reserved byte
			if packet[15] != 0x00 {
				t.Errorf("Expected reserved 0x00, got 0x%02x", packet[15])
			}

			// Entertainment Configuration ID
			expectedID := []byte(client.entertainmentID)
			if !bytes.Equal(packet[16:52], expectedID) {
				t.Errorf("Expected entertainment ID %q, got %q", expectedID, packet[16:52])
			}

			// Verify body
			offset := 52
			for i, color := range tt.colors {
				// Channel ID
				if packet[offset] != byte(color.ChannelID) {
					t.Errorf("Color %d: expected channel ID %d, got %d", i, color.ChannelID, packet[offset])
				}

				// R value (big-endian)
				r := binary.BigEndian.Uint16(packet[offset+1 : offset+3])
				if r != color.R {
					t.Errorf("Color %d: expected R %d, got %d", i, color.R, r)
				}

				// G value (big-endian)
				g := binary.BigEndian.Uint16(packet[offset+3 : offset+5])
				if g != color.G {
					t.Errorf("Color %d: expected G %d, got %d", i, color.G, g)
				}

				// B value (big-endian)
				b := binary.BigEndian.Uint16(packet[offset+5 : offset+7])
				if b != color.B {
					t.Errorf("Color %d: expected B %d, got %d", i, color.B, b)
				}

				offset += 7
			}
		})
	}
}

// TestBuildPacket_SequenceIncrement tests sequence ID behavior
func TestBuildPacket_SequenceIncrement(t *testing.T) {
	client, err := NewClient(Config{
		BridgeIP:        "192.168.1.100",
		Username:        "test-user",
		ClientKey:       "AABBCCDD",
		EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
		ChannelCount:    1,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	colors := []ChannelColor{{ChannelID: 0, R: 100, G: 100, B: 100}}

	// Initial packet should have sequence ID 0
	packet1 := client.buildPacket(colors)
	if packet1[11] != 0 {
		t.Errorf("Expected initial sequence ID 0, got %d", packet1[11])
	}

	// Manually increment sequence ID
	client.sequenceID = 1
	packet2 := client.buildPacket(colors)
	if packet2[11] != 1 {
		t.Errorf("Expected sequence ID 1, got %d", packet2[11])
	}

	// Test overflow
	client.sequenceID = 255
	packet3 := client.buildPacket(colors)
	if packet3[11] != 255 {
		t.Errorf("Expected sequence ID 255, got %d", packet3[11])
	}
}

// TestIsConnected tests the IsConnected method
func TestIsConnected(t *testing.T) {
	client, err := NewClient(Config{
		BridgeIP:        "192.168.1.100",
		Username:        "test-user",
		ClientKey:       "AABBCCDD",
		EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
		ChannelCount:    1,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Should not be connected initially
	if client.IsConnected() {
		t.Error("Expected IsConnected()=false initially")
	}

	// Manually set connected state for testing
	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	if !client.IsConnected() {
		t.Error("Expected IsConnected()=true after setting connected")
	}
}

// TestConvert8BitTo16Bit tests the bit conversion utility
func TestConvert8BitTo16Bit(t *testing.T) {
	tests := []struct {
		input    uint8
		expected uint16
	}{
		{input: 0, expected: 0},
		{input: 1, expected: 257},
		{input: 128, expected: 32896},
		{input: 255, expected: 65535},
		{input: 100, expected: 25700},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := Convert8BitTo16Bit(tt.input)
			if result != tt.expected {
				t.Errorf("Convert8BitTo16Bit(%d): expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

// TestConvert8BitTo16Bit_Roundtrip tests that conversion is reversible
func TestConvert8BitTo16Bit_Roundtrip(t *testing.T) {
	for i := 0; i < 256; i++ {
		input := uint8(i)
		converted := Convert8BitTo16Bit(input)
		// Convert back (approximate)
		back := uint8(converted / 257)
		if back != input {
			t.Errorf("Roundtrip failed for %d: got %d", input, back)
		}
	}
}

// TestStreamColors_NotConnected tests that streaming fails when not connected
func TestStreamColors_NotConnected(t *testing.T) {
	client, err := NewClient(Config{
		BridgeIP:        "192.168.1.100",
		Username:        "test-user",
		ClientKey:       "AABBCCDD",
		EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
		ChannelCount:    1,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	colors := []ChannelColor{{ChannelID: 0, R: 100, G: 100, B: 100}}
	err = client.StreamColors(colors)

	if err == nil {
		t.Error("Expected error when streaming without connection")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got: %v", err)
	}
}

// TestClose_NotConnected tests that closing when not connected is safe
func TestClose_NotConnected(t *testing.T) {
	client, err := NewClient(Config{
		BridgeIP:        "192.168.1.100",
		Username:        "test-user",
		ClientKey:       "AABBCCDD",
		EntertainmentID: "550e8400-e29b-41d4-a716-446655440000",
		ChannelCount:    1,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Should not error when closing a non-connected client
	err = client.Close()
	if err != nil {
		t.Errorf("Unexpected error when closing non-connected client: %v", err)
	}
}
