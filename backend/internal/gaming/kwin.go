package gaming

import (
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
)

// KWinDetector checks if a fullscreen window is active via KWin DBus
type KWinDetector struct {
	conn *dbus.Conn
}

// NewKWinDetector creates a new KWin fullscreen detector
func NewKWinDetector() (*KWinDetector, error) {
	// Connect to session bus (KWin runs on session bus)
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	return &KWinDetector{
		conn: conn,
	}, nil
}

// IsFullscreenActive checks if the active window is fullscreen
// Uses KWin Scripting API to evaluate JavaScript
func (k *KWinDetector) IsFullscreenActive() bool {
	if k.conn == nil {
		return false
	}

	// Use KWin Scripting API to check if active window is fullscreen
	// Script checks workspace.activeClient.fullScreen property
	script := `
		(function() {
			var client = workspace.activeClient;
			if (!client) return false;
			return client.fullScreen;
		})();
	`

	obj := k.conn.Object("org.kde.KWin", "/Scripting")
	call := obj.Call("org.kde.kwin.Scripting.loadScript", 0, script, "gaming-detector")

	if call.Err != nil {
		// KWin scripting not available
		return false
	}

	var scriptId int32
	if err := call.Store(&scriptId); err != nil {
		log.Printf("[WARN] Failed to load KWin script: %v", err)
		return false
	}

	// Run the script
	scriptPath := dbus.ObjectPath("/Scripting/Script" + string(rune(scriptId)))
	scriptObj := k.conn.Object("org.kde.KWin", scriptPath)
	runCall := scriptObj.Call("org.kde.kwin.Script.run", 0)

	if runCall.Err != nil {
		return false
	}

	// Get the result - KWin returns the script output as a variant
	var result dbus.Variant
	if err := runCall.Store(&result); err != nil {
		return false
	}

	// Parse the result - should be "true" or "false" string
	resultStr, ok := result.Value().(string)
	if !ok {
		return false
	}

	return strings.TrimSpace(resultStr) == "true"
}

// Close releases the DBus connection
func (k *KWinDetector) Close() {
	if k.conn != nil {
		if err := k.conn.Close(); err != nil {
			log.Printf("warn: failed to close KWin DBus connection: %v", err)
		}
	}
}
