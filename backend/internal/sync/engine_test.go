package sync

import (
	"testing"

	"github.com/codepuncher/khuey/internal/color"
	"github.com/codepuncher/khuey/internal/config"
	"github.com/codepuncher/khuey/internal/entertainment"
)

// TestCreateDefaultZone tests the auto-split zone creation
func TestCreateDefaultZone(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		total    int
		expected color.Zone
	}{
		{
			name:     "Single channel",
			index:    0,
			total:    1,
			expected: color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0},
		},
		{
			name:     "Two channels - first",
			index:    0,
			total:    2,
			expected: color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		},
		{
			name:     "Two channels - second",
			index:    1,
			total:    2,
			expected: color.Zone{ID: 1, U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
		},
		{
			name:     "Three channels - first",
			index:    0,
			total:    3,
			expected: color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0},
		},
		{
			name:     "Three channels - middle",
			index:    1,
			total:    3,
			expected: color.Zone{ID: 1, U1: 0.33, V1: 0.0, U2: 0.67, V2: 1.0},
		},
		{
			name:     "Three channels - last",
			index:    2,
			total:    3,
			expected: color.Zone{ID: 2, U1: 0.67, V1: 0.0, U2: 1.0, V2: 1.0},
		},
		{
			name:     "Four channels - third",
			index:    2,
			total:    4,
			expected: color.Zone{ID: 2, U1: 0.5, V1: 0.0, U2: 0.75, V2: 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone := createDefaultZone(tt.index, tt.total)

			if zone.ID != tt.expected.ID {
				t.Errorf("Zone ID: expected %d, got %d", tt.expected.ID, zone.ID)
			}
			if !floatClose(zone.U1, tt.expected.U1, 0.01) {
				t.Errorf("Zone U1: expected %.2f, got %.2f", tt.expected.U1, zone.U1)
			}
			if !floatClose(zone.V1, tt.expected.V1, 0.01) {
				t.Errorf("Zone V1: expected %.2f, got %.2f", tt.expected.V1, zone.V1)
			}
			if !floatClose(zone.U2, tt.expected.U2, 0.01) {
				t.Errorf("Zone U2: expected %.2f, got %.2f", tt.expected.U2, zone.U2)
			}
			if !floatClose(zone.V2, tt.expected.V2, 0.01) {
				t.Errorf("Zone V2: expected %.2f, got %.2f", tt.expected.V2, zone.V2)
			}
		})
	}
}

// TestCreateDefaultZone_ManyChannels tests even division for many channels
func TestCreateDefaultZone_ManyChannels(t *testing.T) {
	for _, total := range []int{4, 5, 10} {
		zones := make([]color.Zone, total)
		for i := range zones {
			zones[i] = createDefaultZone(i, total)
		}

		// First zone starts at 0
		if !floatClose(zones[0].U1, 0.0, 0.001) {
			t.Errorf("%d channels: first zone should start at 0.0, got %.3f", total, zones[0].U1)
		}
		// Last zone ends at 1
		if !floatClose(zones[total-1].U2, 1.0, 0.001) {
			t.Errorf("%d channels: last zone should end at 1.0, got %.3f", total, zones[total-1].U2)
		}
		// No gaps between zones
		for i := 0; i < total-1; i++ {
			if !floatClose(zones[i].U2, zones[i+1].U1, 0.001) {
				t.Errorf("%d channels: gap between zone %d and %d: %.3f != %.3f", total, i, i+1, zones[i].U2, zones[i+1].U1)
			}
		}
		// All zones span full height
		for i, zone := range zones {
			if zone.V1 != 0.0 || zone.V2 != 1.0 {
				t.Errorf("Zone %d: expected full height, got V1=%.3f V2=%.3f", i, zone.V1, zone.V2)
			}
		}
	}
}

// TestValidateUVCoordinates tests UV coordinate validation
func TestValidateUVCoordinates(t *testing.T) {
	tests := []struct {
		name        string
		uvA         config.UV
		uvB         config.UV
		expectError bool
	}{
		{
			name:        "Valid coordinates",
			uvA:         config.UV{X: 0.0, Y: 0.0},
			uvB:         config.UV{X: 0.5, Y: 1.0},
			expectError: false,
		},
		{
			name:        "Full screen",
			uvA:         config.UV{X: 0.0, Y: 0.0},
			uvB:         config.UV{X: 1.0, Y: 1.0},
			expectError: false,
		},
		{
			name:        "uvA.X out of bounds (negative)",
			uvA:         config.UV{X: -0.1, Y: 0.0},
			uvB:         config.UV{X: 0.5, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvA.X out of bounds (>1)",
			uvA:         config.UV{X: 1.5, Y: 0.0},
			uvB:         config.UV{X: 2.0, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.X out of bounds (>1)",
			uvA:         config.UV{X: 0.0, Y: 0.0},
			uvB:         config.UV{X: 1.5, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.X <= uvA.X (zero width)",
			uvA:         config.UV{X: 0.5, Y: 0.0},
			uvB:         config.UV{X: 0.5, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.X < uvA.X (inverted)",
			uvA:         config.UV{X: 0.7, Y: 0.0},
			uvB:         config.UV{X: 0.3, Y: 1.0},
			expectError: true,
		},
		{
			name:        "uvB.Y <= uvA.Y (zero height)",
			uvA:         config.UV{X: 0.0, Y: 0.5},
			uvB:         config.UV{X: 1.0, Y: 0.5},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUVCoordinates(&tt.uvA, &tt.uvB)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error=%v, got %v", tt.expectError, err)
			}
		})
	}
}

// TestCreateZonesFromConfig tests zone creation from config
func TestCreateZonesFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		expectedZones int
		checkFirst    *color.Zone
	}{
		{
			name: "Three channels with UV coordinates",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Left", GammaFactor: 2.2, UVA: config.UV{X: 0.0, Y: 0.0}, UVB: config.UV{X: 0.33, Y: 1.0}},
					{ID: 1, Active: true, DeviceName: "Center", GammaFactor: 2.2, UVA: config.UV{X: 0.33, Y: 0.0}, UVB: config.UV{X: 0.67, Y: 1.0}},
					{ID: 2, Active: true, DeviceName: "Right", GammaFactor: 2.2, UVA: config.UV{X: 0.67, Y: 0.0}, UVB: config.UV{X: 1.0, Y: 1.0}},
				},
			},
			expectedZones: 3,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0, Name: "Left"},
		},
		{
			name: "Two channels without UV (auto-split)",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Left"},
					{ID: 1, Active: true, DeviceName: "Right"},
				},
			},
			expectedZones: 2,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
		},
		{
			name: "One inactive channel filtered out",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Active", UVA: config.UV{X: 0.0, Y: 0.0}, UVB: config.UV{X: 1.0, Y: 1.0}},
					{ID: 1, Active: false, DeviceName: "Inactive"},
				},
			},
			expectedZones: 1,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0, Name: "Active"},
		},
		{
			name: "Invalid UV coordinates fall back to auto-split",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{ID: 0, Active: true, DeviceName: "Bad UV", UVA: config.UV{X: 1.0, Y: 0.0}, UVB: config.UV{X: 0.5, Y: 1.0}},
				},
			},
			expectedZones: 1,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zones := createZonesFromConfig(tt.config)

			if len(zones) != tt.expectedZones {
				t.Errorf("Expected %d zones, got %d", tt.expectedZones, len(zones))
			}

			if tt.checkFirst != nil && len(zones) > 0 {
				zone := zones[0]
				if zone.ID != tt.checkFirst.ID {
					t.Errorf("Zone ID: expected %d, got %d", tt.checkFirst.ID, zone.ID)
				}
				if !floatClose(zone.U1, tt.checkFirst.U1, 0.01) {
					t.Errorf("Zone U1: expected %.2f, got %.2f", tt.checkFirst.U1, zone.U1)
				}
				if !floatClose(zone.V1, tt.checkFirst.V1, 0.01) {
					t.Errorf("Zone V1: expected %.2f, got %.2f", tt.checkFirst.V1, zone.V1)
				}
				if !floatClose(zone.U2, tt.checkFirst.U2, 0.01) {
					t.Errorf("Zone U2: expected %.2f, got %.2f", tt.checkFirst.U2, zone.U2)
				}
				if !floatClose(zone.V2, tt.checkFirst.V2, 0.01) {
					t.Errorf("Zone V2: expected %.2f, got %.2f", tt.checkFirst.V2, zone.V2)
				}
				if zone.Name != tt.checkFirst.Name {
					t.Errorf("Zone Name: expected %s, got %s", tt.checkFirst.Name, zone.Name)
				}
			}
		})
	}
}

// TestColor8BitTo16BitConversion tests the color bit conversion via the real function
func TestColor8BitTo16BitConversion(t *testing.T) {
	tests := []struct {
		input8bit     uint8
		expected16bit uint16
	}{
		{input8bit: 0, expected16bit: 0},
		{input8bit: 255, expected16bit: 65535},
		{input8bit: 128, expected16bit: 32896},
		{input8bit: 64, expected16bit: 16448},
		{input8bit: 192, expected16bit: 49344},
	}

	for _, tt := range tests {
		result := entertainment.Convert8BitTo16Bit(tt.input8bit)
		if result != tt.expected16bit {
			t.Errorf("Convert8BitTo16Bit(%d): expected %d, got %d", tt.input8bit, tt.expected16bit, result)
		}
	}
}

// Helper functions

func floatClose(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}
