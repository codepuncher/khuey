// Package testutil provides test utilities including mock Entertainment API server
package testutil

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/pion/dtls/v2"
)

// MockEntertainmentServer simulates a Hue Entertainment API DTLS endpoint for testing
type MockEntertainmentServer struct {
	listener net.Listener
	addr     *net.UDPAddr
	mu       sync.RWMutex

	// Configuration
	clientKey string // Hex-encoded PSK
	username  string // Expected username/identity

	// State tracking
	running         bool
	frames          []ReceivedFrame
	lastFrame       *ReceivedFrame
	frameCount      int
	startTime       time.Time
	connectionCount int

	// For graceful shutdown
	done chan struct{}
	wg   sync.WaitGroup
}

// ReceivedFrame represents a parsed HueStream v2 frame
type ReceivedFrame struct {
	Timestamp            time.Time
	SequenceID           uint8
	Version              [2]byte // Major, Minor
	ColorSpace           uint8
	EntertainmentID      string
	Channels             []ChannelColor
	RawPacket            []byte
	PacketSize           int
	TimeSinceLastFrame   time.Duration
	FramesSinceLastCheck int
}

// ChannelColor represents RGB color for a channel in received data
type ChannelColor struct {
	ChannelID int
	R         uint16 // 0-65535
	G         uint16 // 0-65535
	B         uint16 // 0-65535
}

// NewMockEntertainmentServer creates a new mock Entertainment API server
func NewMockEntertainmentServer(clientKey, username string) *MockEntertainmentServer {
	return &MockEntertainmentServer{
		clientKey: clientKey,
		username:  username,
		frames:    make([]ReceivedFrame, 0, 100),
		done:      make(chan struct{}),
	}
}

// Start begins listening for DTLS connections
func (m *MockEntertainmentServer) Start(t *testing.T) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("server already running")
	}

	// Decode PSK from hex
	pskBytes, err := hex.DecodeString(m.clientKey)
	if err != nil {
		return fmt.Errorf("failed to decode client key: %w", err)
	}

	// Configure DTLS with PSK
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			// Verify identity matches expected username
			if string(hint) != m.username {
				t.Logf("[Mock Entertainment] PSK requested with wrong identity: %q (expected %q)", string(hint), m.username)
			}
			return pskBytes, nil
		},
		PSKIdentityHint:      []byte(m.username),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_GCM_SHA256},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	// Listen on random UDP port
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	listener, err := dtls.Listen("udp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to start DTLS listener: %w", err)
	}

	m.listener = listener
	m.addr = listener.Addr().(*net.UDPAddr)
	m.running = true
	m.startTime = time.Now()

	// Start accept loop in goroutine
	m.wg.Add(1)
	go m.acceptLoop(t)

	t.Logf("[Mock Entertainment] DTLS server started on %s", m.addr)
	return nil
}

// Stop gracefully shuts down the server
func (m *MockEntertainmentServer) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	// Signal shutdown
	close(m.done)

	// Close listener to unblock Accept()
	if m.listener != nil {
		m.listener.Close()
	}

	// Wait for goroutines
	m.wg.Wait()
}

// GetPort returns the UDP port the server is listening on
func (m *MockEntertainmentServer) GetPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.addr == nil {
		return 0
	}
	return m.addr.Port
}

// GetAddress returns the server address
func (m *MockEntertainmentServer) GetAddress() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.addr == nil {
		return ""
	}
	return m.addr.String()
}

// GetLastFrame returns the most recently received frame
func (m *MockEntertainmentServer) GetLastFrame() *ReceivedFrame {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastFrame
}

// GetFrameCount returns the total number of frames received
func (m *MockEntertainmentServer) GetFrameCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.frameCount
}

// GetFrames returns all received frames (up to last 100)
func (m *MockEntertainmentServer) GetFrames() []ReceivedFrame {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ReceivedFrame, len(m.frames))
	copy(result, m.frames)
	return result
}

// GetFrameRate calculates the average frame rate
func (m *MockEntertainmentServer) GetFrameRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.frameCount == 0 {
		return 0.0
	}

	elapsed := time.Since(m.startTime).Seconds()
	if elapsed == 0 {
		return 0.0
	}

	return float64(m.frameCount) / elapsed
}

// GetConnectionCount returns the number of accepted connections
func (m *MockEntertainmentServer) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionCount
}

// ClearFrames clears the frame history
func (m *MockEntertainmentServer) ClearFrames() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frames = make([]ReceivedFrame, 0, 100)
	m.frameCount = 0
	m.lastFrame = nil
	m.startTime = time.Now()
}

// WaitForFrames waits for at least N frames to be received (with timeout)
func (m *MockEntertainmentServer) WaitForFrames(minFrames int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.GetFrameCount() >= minFrames {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// acceptLoop accepts incoming DTLS connections
func (m *MockEntertainmentServer) acceptLoop(t *testing.T) {
	defer m.wg.Done()

	for {
		// Check if we should stop
		select {
		case <-m.done:
			return
		default:
		}

		// Accept with timeout to allow checking done channel
		conn, err := m.listener.Accept()
		if err != nil {
			// Check if error is due to shutdown
			select {
			case <-m.done:
				return
			default:
				// Log non-shutdown errors
				if !isClosedNetworkError(err) {
					t.Logf("[Mock Entertainment] Accept error: %v", err)
				}
				continue
			}
		}

		m.mu.Lock()
		m.connectionCount++
		m.mu.Unlock()

		t.Logf("[Mock Entertainment] Connection accepted from %s", conn.RemoteAddr())

		// Handle connection in separate goroutine
		m.wg.Add(1)
		go m.handleConnection(conn, t)
	}
}

// handleConnection processes data from a DTLS connection
func (m *MockEntertainmentServer) handleConnection(conn net.Conn, t *testing.T) {
	defer m.wg.Done()
	defer conn.Close()

	buffer := make([]byte, 4096)

	for {
		// Check if we should stop
		select {
		case <-m.done:
			return
		default:
		}

		// Set read deadline to allow periodic checking of done channel
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected - continue to check done channel
				continue
			}
			// Connection closed or other error
			select {
			case <-m.done:
				return
			default:
				if !isClosedNetworkError(err) {
					t.Logf("[Mock Entertainment] Read error: %v", err)
				}
				return
			}
		}

		if n == 0 {
			continue
		}

		// Parse HueStream v2 packet
		frame, err := m.parsePacket(buffer[:n])
		if err != nil {
			t.Logf("[Mock Entertainment] Parse error: %v", err)
			continue
		}

		// Store frame
		m.mu.Lock()
		m.frameCount++
		frame.Timestamp = time.Now()
		frame.RawPacket = make([]byte, n)
		copy(frame.RawPacket, buffer[:n])
		frame.PacketSize = n

		// Calculate time since last frame
		if m.lastFrame != nil {
			frame.TimeSinceLastFrame = frame.Timestamp.Sub(m.lastFrame.Timestamp)
		}

		m.lastFrame = &frame

		// Keep last 100 frames
		m.frames = append(m.frames, frame)
		if len(m.frames) > 100 {
			m.frames = m.frames[1:]
		}
		m.mu.Unlock()
	}
}

// parsePacket parses a HueStream v2 protocol packet
func (m *MockEntertainmentServer) parsePacket(data []byte) (ReceivedFrame, error) {
	frame := ReceivedFrame{}

	// HueStream v2 packet format:
	// Header: 52 bytes
	//   - Protocol (9 bytes): "HueStream"
	//   - Version (2 bytes): Major, Minor
	//   - Sequence (1 byte)
	//   - Reserved (2 bytes)
	//   - Color space (1 byte)
	//   - Reserved (1 byte)
	//   - Entertainment Configuration ID (36 bytes)
	// Body: 7 bytes per channel
	//   - Channel ID (1 byte)
	//   - R (2 bytes, big-endian)
	//   - G (2 bytes, big-endian)
	//   - B (2 bytes, big-endian)

	const headerSize = 52
	if len(data) < headerSize {
		return frame, fmt.Errorf("packet too small: %d bytes (need at least %d)", len(data), headerSize)
	}

	// Validate protocol signature
	protocol := string(data[0:9])
	if protocol != "HueStream" {
		return frame, fmt.Errorf("invalid protocol: %q", protocol)
	}

	// Parse header
	frame.Version[0] = data[9]  // Major
	frame.Version[1] = data[10] // Minor
	frame.SequenceID = data[11]
	frame.ColorSpace = data[14]
	frame.EntertainmentID = string(data[16:52])

	// Parse body (channels)
	bodySize := len(data) - headerSize
	if bodySize%7 != 0 {
		return frame, fmt.Errorf("invalid body size: %d (must be multiple of 7)", bodySize)
	}

	channelCount := bodySize / 7
	frame.Channels = make([]ChannelColor, channelCount)

	offset := headerSize
	for i := 0; i < channelCount; i++ {
		frame.Channels[i] = ChannelColor{
			ChannelID: int(data[offset]),
			R:         binary.BigEndian.Uint16(data[offset+1 : offset+3]),
			G:         binary.BigEndian.Uint16(data[offset+3 : offset+5]),
			B:         binary.BigEndian.Uint16(data[offset+5 : offset+7]),
		}
		offset += 7
	}

	return frame, nil
}

// isClosedNetworkError checks if error is due to closed network connection
func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common closed connection error messages
	errStr := err.Error()
	return errStr == "use of closed network connection" ||
		errStr == "closed network connection" ||
		errStr == "network connection closed"
}

// GetChannelColor returns the color for a specific channel from the last frame
func (m *MockEntertainmentServer) GetChannelColor(channelID int) (r, g, b uint16, found bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.lastFrame == nil {
		return 0, 0, 0, false
	}

	for _, ch := range m.lastFrame.Channels {
		if ch.ChannelID == channelID {
			return ch.R, ch.G, ch.B, true
		}
	}

	return 0, 0, 0, false
}

// VerifyChannelColor checks if a channel has expected RGB values (with tolerance)
func (m *MockEntertainmentServer) VerifyChannelColor(channelID int, expectedR, expectedG, expectedB uint16, tolerance uint16) error {
	r, g, b, found := m.GetChannelColor(channelID)
	if !found {
		return fmt.Errorf("channel %d not found in last frame", channelID)
	}

	if abs(int(r)-int(expectedR)) > int(tolerance) {
		return fmt.Errorf("channel %d: R mismatch: got %d, expected %d (tolerance %d)", channelID, r, expectedR, tolerance)
	}
	if abs(int(g)-int(expectedG)) > int(tolerance) {
		return fmt.Errorf("channel %d: G mismatch: got %d, expected %d (tolerance %d)", channelID, g, expectedG, tolerance)
	}
	if abs(int(b)-int(expectedB)) > int(tolerance) {
		return fmt.Errorf("channel %d: B mismatch: got %d, expected %d (tolerance %d)", channelID, b, expectedB, tolerance)
	}

	return nil
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// LogStats logs server statistics
func (m *MockEntertainmentServer) LogStats(t *testing.T) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	elapsed := time.Since(m.startTime).Seconds()
	fps := 0.0
	if elapsed > 0 {
		fps = float64(m.frameCount) / elapsed
	}

	t.Logf("[Mock Entertainment Stats]")
	t.Logf("  Connections: %d", m.connectionCount)
	t.Logf("  Frames Received: %d", m.frameCount)
	t.Logf("  Average FPS: %.2f", fps)
	t.Logf("  Runtime: %.2fs", elapsed)

	if m.lastFrame != nil {
		t.Logf("  Last Frame:")
		t.Logf("    Sequence ID: %d", m.lastFrame.SequenceID)
		t.Logf("    Channels: %d", len(m.lastFrame.Channels))
		t.Logf("    Packet Size: %d bytes", m.lastFrame.PacketSize)
		if len(m.lastFrame.Channels) > 0 {
			ch := m.lastFrame.Channels[0]
			t.Logf("    Channel 0 RGB: (%d, %d, %d)", ch.R, ch.G, ch.B)
		}
	}
}

// MockEntertainmentActivation simulates Entertainment Area activation on bridge
func MockEntertainmentActivation(mb *MockBridge, entID string) {
	// Add Entertainment configuration to mock if not exists
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.Entertainment[entID]; !exists {
		mb.Entertainment[entID] = &MockEntertainment{
			ID:     entID,
			Name:   "Test Entertainment Area",
			Type:   "entertainment_configuration",
			Status: "active",
		}
		log.Printf("[Mock Bridge] Entertainment Area %s activated", entID)
	}
}
