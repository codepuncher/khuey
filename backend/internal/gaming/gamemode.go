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
	// gamemoded is a per-user daemon on the session bus.
	conn, err := dbus.ConnectSessionBus()
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

	// QueryStatus is non-zero while any client holds GameMode.
	obj := g.conn.Object("com.feralinteractive.GameMode", "/com/feralinteractive/GameMode")
	// NoAutoStart keeps polling from launching a gamemoded that outlives hue-sync.
	call := obj.Call("com.feralinteractive.GameMode.QueryStatus", dbus.FlagNoAutoStart, int32(0))

	if call.Err != nil {
		// GameMode not available or error occurred
		// This is not fatal - just means we can't detect via GameMode
		return false
	}

	var status int32
	if err := call.Store(&status); err != nil {
		log.Printf("[WARN] Failed to parse GameMode status: %v", err)
		return false
	}

	// Status > 0 means GameMode is active
	return status > 0
}

// Close releases the DBus connection
func (g *GameModeDetector) Close() {
	if g.conn != nil {
		if err := g.conn.Close(); err != nil {
			log.Printf("warn: failed to close GameMode DBus connection: %v", err)
		}
	}
}
