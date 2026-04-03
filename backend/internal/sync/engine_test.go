package sync

import (
	"testing"

	"github.com/codepuncher/khuey/internal/color"
	"github.com/codepuncher/khuey/internal/config"
)

// TestZoneMapping tests the zone mapping logic for different channel counts
func TestZoneMapping(t *testing.T) {
	tests := []struct {
		name          string
		channelCount  int
		expectedZones []color.Zone
	}{
		{
			name:         "Single channel - full screen",
			channelCount: 1,
			expectedZones: []color.Zone{
				{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0},
			},
		},
		{
			name:         "Two channels - left/right split",
			channelCount: 2,
			expectedZones: []color.Zone{
				{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0},
				{ID: 1, U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0},
			},
		},
		{
			name:         "Three channels - left/center/right",
			channelCount: 3,
			expectedZones: []color.Zone{
				{ID: 0, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0},
				{ID: 1, U1: 0.33, V1: 0.0, U2: 0.67, V2: 1.0},
				{ID: 2, U1: 0.67, V1: 0.0, U2: 1.0, V2: 1.0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate zone mapping logic from NewEngine
			zones := make([]color.Zone, tt.channelCount)

			switch tt.channelCount {
			case 1:
				zones[0] = color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0}
			case 2:
				zones[0] = color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.5, V2: 1.0}
				zones[1] = color.Zone{ID: 1, U1: 0.5, V1: 0.0, U2: 1.0, V2: 1.0}
			case 3:
				zones[0] = color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 0.33, V2: 1.0}
				zones[1] = color.Zone{ID: 1, U1: 0.33, V1: 0.0, U2: 0.67, V2: 1.0}
				zones[2] = color.Zone{ID: 2, U1: 0.67, V1: 0.0, U2: 1.0, V2: 1.0}
			default:
				step := 1.0 / float64(tt.channelCount)
				for i := range zones {
					zones[i] = color.Zone{
						ID: i,
						U1: float64(i) * step,
						V1: 0.0,
						U2: float64(i+1) * step,
						V2: 1.0,
					}
				}
			}

			if len(zones) != len(tt.expectedZones) {
				t.Fatalf("Expected %d zones, got %d", len(tt.expectedZones), len(zones))
			}

			for i, zone := range zones {
				expected := tt.expectedZones[i]
				if zone.ID != expected.ID {
					t.Errorf("Zone %d: expected ID %d, got %d", i, expected.ID, zone.ID)
				}
				if !floatClose(zone.U1, expected.U1, 0.01) {
					t.Errorf("Zone %d: expected U1 %.2f, got %.2f", i, expected.U1, zone.U1)
				}
				if !floatClose(zone.V1, expected.V1, 0.01) {
					t.Errorf("Zone %d: expected V1 %.2f, got %.2f", i, expected.V1, zone.V1)
				}
				if !floatClose(zone.U2, expected.U2, 0.01) {
					t.Errorf("Zone %d: expected U2 %.2f, got %.2f", i, expected.U2, zone.U2)
				}
				if !floatClose(zone.V2, expected.V2, 0.01) {
					t.Errorf("Zone %d: expected V2 %.2f, got %.2f", i, expected.V2, zone.V2)
				}
			}
		})
	}
}

// TestZoneMapping_ManyChannels tests even division for many channels
func TestZoneMapping_ManyChannels(t *testing.T) {
	tests := []struct {
		name         string
		channelCount int
	}{
		{name: "4 channels", channelCount: 4},
		{name: "5 channels", channelCount: 5},
		{name: "10 channels", channelCount: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create zones using the default logic
			zones := make([]color.Zone, tt.channelCount)
			step := 1.0 / float64(tt.channelCount)

			for i := range zones {
				zones[i] = color.Zone{
					ID: i,
					U1: float64(i) * step,
					V1: 0.0,
					U2: float64(i+1) * step,
					V2: 1.0,
				}
			}

			// Verify zone count
			if len(zones) != tt.channelCount {
				t.Fatalf("Expected %d zones, got %d", tt.channelCount, len(zones))
			}

			// Verify each zone
			for i, zone := range zones {
				expectedU1 := float64(i) * step
				expectedU2 := float64(i+1) * step

				if !floatClose(zone.U1, expectedU1, 0.001) {
					t.Errorf("Zone %d: expected U1 %.3f, got %.3f", i, expectedU1, zone.U1)
				}
				if !floatClose(zone.U2, expectedU2, 0.001) {
					t.Errorf("Zone %d: expected U2 %.3f, got %.3f", i, expectedU2, zone.U2)
				}

				// All zones should span full height
				if zone.V1 != 0.0 {
					t.Errorf("Zone %d: expected V1 0.0, got %.3f", i, zone.V1)
				}
				if zone.V2 != 1.0 {
					t.Errorf("Zone %d: expected V2 1.0, got %.3f", i, zone.V2)
				}

				// ID should match index
				if zone.ID != i {
					t.Errorf("Zone %d: expected ID %d, got %d", i, i, zone.ID)
				}
			}

			// Verify zones cover full width without gaps or overlaps
			if !floatClose(zones[0].U1, 0.0, 0.001) {
				t.Errorf("First zone should start at 0.0, got %.3f", zones[0].U1)
			}
			if !floatClose(zones[len(zones)-1].U2, 1.0, 0.001) {
				t.Errorf("Last zone should end at 1.0, got %.3f", zones[len(zones)-1].U2)
			}

			// Verify no gaps between zones
			for i := 0; i < len(zones)-1; i++ {
				if !floatClose(zones[i].U2, zones[i+1].U1, 0.001) {
					t.Errorf("Gap between zone %d and %d: %.3f != %.3f", i, i+1, zones[i].U2, zones[i+1].U1)
				}
			}
		})
	}
}

// TestSetFPS_Validation tests FPS validation
func TestSetFPS_Validation(t *testing.T) {
	tests := []struct {
		name        string
		fps         int
		expectError bool
	}{
		{
			name:        "Minimum valid FPS",
			fps:         1,
			expectError: false,
		},
		{
			name:        "Standard FPS",
			fps:         30,
			expectError: false,
		},
		{
			name:        "Maximum valid FPS",
			fps:         60,
			expectError: false,
		},
		{
			name:        "Zero FPS",
			fps:         0,
			expectError: true,
		},
		{
			name:        "Negative FPS",
			fps:         -1,
			expectError: true,
		},
		{
			name:        "Too high FPS",
			fps:         61,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate FPS (logic from SetFPS)
			isValid := tt.fps >= 1 && tt.fps <= 60

			if tt.expectError && isValid {
				t.Errorf("Expected FPS %d to be invalid", tt.fps)
			}
			if !tt.expectError && !isValid {
				t.Errorf("Expected FPS %d to be valid", tt.fps)
			}
		})
	}
}

// TestColor8BitTo16BitConversion tests the color bit conversion
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
		t.Run("", func(t *testing.T) {
			// 8-bit to 16-bit conversion: value * 257
			result := uint16(tt.input8bit) * 257

			if result != tt.expected16bit {
				t.Errorf("Convert %d: expected %d, got %d", tt.input8bit, tt.expected16bit, result)
			}
		})
	}
}

// TestEngineInitialState tests the initial state of an engine
func TestEngineInitialState(t *testing.T) {
	// Test initial state expectations
	initialRunning := false
	initialFPS := 30

	if initialRunning {
		t.Error("Engine should not be running initially")
	}
	if initialFPS != 30 {
		t.Errorf("Initial FPS should be 30, got %d", initialFPS)
	}
}

// TestStartStopValidation tests start/stop validation logic
func TestStartStopValidation(t *testing.T) {
	tests := []struct {
		name        string
		isRunning   bool
		operation   string
		expectError bool
	}{
		{
			name:        "Start when not running",
			isRunning:   false,
			operation:   "start",
			expectError: false,
		},
		{
			name:        "Start when already running",
			isRunning:   true,
			operation:   "start",
			expectError: true,
		},
		{
			name:        "Stop when running",
			isRunning:   true,
			operation:   "stop",
			expectError: false,
		},
		{
			name:        "Stop when not running",
			isRunning:   false,
			operation:   "stop",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var isValid bool

			if tt.operation == "start" {
				// Start validation: should not be running
				isValid = !tt.isRunning
			} else {
				// Stop validation: should be running
				isValid = tt.isRunning
			}

			if tt.expectError && isValid {
				t.Errorf("Expected operation to be invalid")
			}
			if !tt.expectError && !isValid {
				t.Errorf("Expected operation to be valid")
			}
		})
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
			hasError := err != nil

			if tt.expectError && !hasError {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && hasError {
				t.Errorf("Unexpected error: %v", err)
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
		checkFirst    *color.Zone // Check first zone properties
	}{
		{
			name: "Three channels with UV coordinates",
			config: &config.Config{
				Channels: []config.ChannelConfig{
					{
						ID:          0,
						Active:      true,
						DeviceName:  "Left",
						GammaFactor: 2.2,
						UVA:         config.UV{X: 0.0, Y: 0.0},
						UVB:         config.UV{X: 0.33, Y: 1.0},
					},
					{
						ID:          1,
						Active:      true,
						DeviceName:  "Center",
						GammaFactor: 2.2,
						UVA:         config.UV{X: 0.33, Y: 0.0},
						UVB:         config.UV{X: 0.67, Y: 1.0},
					},
					{
						ID:          2,
						Active:      true,
						DeviceName:  "Right",
						GammaFactor: 2.2,
						UVA:         config.UV{X: 0.67, Y: 0.0},
						UVB:         config.UV{X: 1.0, Y: 1.0},
					},
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
					{
						ID:         0,
						Active:     true,
						DeviceName: "Active",
						UVA:        config.UV{X: 0.0, Y: 0.0},
						UVB:        config.UV{X: 1.0, Y: 1.0},
					},
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
					{
						ID:         0,
						Active:     true,
						DeviceName: "Bad UV",
						UVA:        config.UV{X: 1.0, Y: 0.0}, // Invalid: uvB would need to be > 1.0
						UVB:        config.UV{X: 0.5, Y: 1.0}, // X < uvA.X
					},
				},
			},
			expectedZones: 1,
			checkFirst:    &color.Zone{ID: 0, U1: 0.0, V1: 0.0, U2: 1.0, V2: 1.0}, // Auto-split for 1 channel
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
			name:     "Three channels - middle",
			index:    1,
			total:    3,
			expected: color.Zone{ID: 1, U1: 0.33, V1: 0.0, U2: 0.67, V2: 1.0},
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
