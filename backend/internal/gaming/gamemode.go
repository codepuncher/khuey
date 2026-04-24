package gaming

import (
	"log"

	"github.com/godbus/dbus/v5"
)

// GameModeDetector checks if GameMode is active via DBus
type GameModeDetector struct {
	conn *dbus.Conn
}

// NewGameModeDetector creates a new GameMode detector
func NewGameModeDetector() (*GameModeDetector, error) {
	// Connect to system bus (GameMode runs as system service)
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}

	return &GameModeDetector{
		conn: conn,
	}, nil
}

// IsActive checks if GameMode is currently active
// Returns true if any client has requested GameMode
func (g *GameModeDetector) IsActive() bool {
	if g.conn == nil {
		return false
	}

	// Call GameMode DBus API: org.gamemode.QueryStatus
	// Method signature: QueryStatus(int32 pid) -> int32 status
	// Status: 0 = inactive, 1 = active, 2 = active (client registered)
	// We use pid=0 to check global status (any client active)
	obj := g.conn.Object("org.froggi.gamemode", "/org/froggi/gamemode")
	call := obj.Call("org.froggi.gamemode.QueryStatus", 0, int32(0))

	if call.Err != nil {
		// GameMode not available or error occurred
		// This is not fatal - just means we can't detect via GameMode
		return false
	}

	var status int32
	if err := call.Store(&status); err != nil {
		log.Printf("⚠️  Failed to parse GameMode status: %v", err)
		return false
	}

	// Status > 0 means GameMode is active
	return status > 0
}

// Close releases the DBus connection
func (g *GameModeDetector) Close() {
	if g.conn != nil {
		g.conn.Close()
	}
}
