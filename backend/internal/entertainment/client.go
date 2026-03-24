package entertainment

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"

	"github.com/pion/dtls/v2"
)

// Client handles streaming to Hue Entertainment API
type Client struct {
	bridgeIP         string
	username         string
	clientKey        string
	entertainmentID  string
	
	conn             net.Conn
	sequenceID       uint8
	channelCount     int
	
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	connected        bool
}

// Config holds Entertainment API configuration
type Config struct {
	BridgeIP        string // Bridge IP address
	Username        string // Hue username
	ClientKey       string // Client key for DTLS PSK
	EntertainmentID string // Entertainment Configuration ID
	ChannelCount    int    // Number of channels (lights)
}

// ChannelColor represents RGB color for a channel
type ChannelColor struct {
	ChannelID int
	R         uint16 // 0-65535
	G         uint16 // 0-65535
	B         uint16 // 0-65535
}

// NewClient creates a new Entertainment API client
func NewClient(cfg Config) (*Client, error) {
	if cfg.BridgeIP == "" {
		return nil, fmt.Errorf("bridge IP is required")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if cfg.ClientKey == "" {
		return nil, fmt.Errorf("client key is required")
	}
	if cfg.EntertainmentID == "" {
		return nil, fmt.Errorf("entertainment ID is required")
	}
	if cfg.ChannelCount <= 0 {
		return nil, fmt.Errorf("channel count must be positive")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		bridgeIP:        cfg.BridgeIP,
		username:        cfg.Username,
		clientKey:       cfg.ClientKey,
		entertainmentID: cfg.EntertainmentID,
		channelCount:    cfg.ChannelCount,
		sequenceID:      0,
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

// Connect establishes DTLS connection to Entertainment API
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	// Entertainment API uses UDP port 2100
	addr := fmt.Sprintf("%s:2100", c.bridgeIP)

	// Configure DTLS with PSK (Pre-Shared Key)
	// Important: clientKey must be hex-decoded before use!
	pskBytes, err := hex.DecodeString(c.clientKey)
	if err != nil {
		return fmt.Errorf("failed to decode client key: %w", err)
	}

	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			// Return decoded PSK bytes
			return pskBytes, nil
		},
		PSKIdentityHint: []byte(c.username), // Username as identity
		CipherSuites:    []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_GCM_SHA256},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	// Resolve UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	// Dial with DTLS
	conn, err := dtls.Dial("udp", udpAddr, config)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.connected = true
	c.sequenceID = 0

	return nil
}

// StreamColors sends colors to the Entertainment API
func (c *Client) StreamColors(colors []ChannelColor) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// Build HueStream v2 packet
	packet := c.buildPacket(colors)

	// Send via DTLS
	_, err := c.conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	// Increment sequence ID
	c.sequenceID++

	return nil
}

// buildPacket creates a HueStream v2 protocol packet
func (c *Client) buildPacket(colors []ChannelColor) []byte {
	// HueStream v2 packet format:
	// Header: 16 bytes
	//   - Protocol (9 bytes): "HueStream"
	//   - Version (2 bytes): 0x0200 (v2.0)
	//   - Sequence (1 byte)
	//   - Reserved (2 bytes): 0x0000
	//   - Color space (1 byte): 0x00 (RGB)
	//   - Reserved (1 byte): 0x00
	// Body: 7 bytes per channel
	//   - Channel ID (1 byte)
	//   - R (2 bytes, big-endian)
	//   - G (2 bytes, big-endian)
	//   - B (2 bytes, big-endian)

	headerSize := 16
	bodySize := 7 * len(colors)
	packet := make([]byte, headerSize+bodySize)

	// Header
	copy(packet[0:9], "HueStream")
	packet[9] = 0x02  // Version major
	packet[10] = 0x00 // Version minor
	packet[11] = c.sequenceID
	packet[12] = 0x00 // Reserved
	packet[13] = 0x00 // Reserved
	packet[14] = 0x00 // Color space: RGB
	packet[15] = 0x00 // Reserved

	// Body: per-channel colors
	offset := headerSize
	for _, color := range colors {
		packet[offset] = byte(color.ChannelID)
		binary.BigEndian.PutUint16(packet[offset+1:], color.R)
		binary.BigEndian.PutUint16(packet[offset+3:], color.G)
		binary.BigEndian.PutUint16(packet[offset+5:], color.B)
		offset += 7
	}

	return packet
}

// Close closes the Entertainment API connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.cancel()
	c.connected = false

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Convert8BitTo16Bit converts 8-bit RGB (0-255) to 16-bit (0-65535)
func Convert8BitTo16Bit(value uint8) uint16 {
	// Scale from 0-255 to 0-65535
	return uint16(value) * 257
}
