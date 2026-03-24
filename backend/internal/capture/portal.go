package capture

import (
	"fmt"
	"math/rand"

	"github.com/godbus/dbus/v5"
)

const (
	portalDest      = "org.freedesktop.portal.Desktop"
	portalPath      = "/org/freedesktop/portal/desktop"
	screenCastIface = "org.freedesktop.portal.ScreenCast"
)

// createSession creates a new ScreenCast session via XDG Desktop Portal
func (sc *ScreenCapture) createSession() (string, error) {
	obj := sc.conn.Object(portalDest, portalPath)

	// Generate unique session token
	sessionToken := fmt.Sprintf("khuey_session_%d", rand.Intn(999999))
	
	// Prepare options for CreateSession
	options := map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(sessionToken),
		"handle_token":         dbus.MakeVariant(fmt.Sprintf("khuey_%d", rand.Intn(999999))),
	}

	var requestPath dbus.ObjectPath
	err := obj.Call(screenCastIface+".CreateSession", 0, options).Store(&requestPath)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	// Wait for response via Request interface
	result, err := sc.waitForResponse(requestPath)
	if err != nil {
		return "", fmt.Errorf("failed to get session handle: %w", err)
	}

	// Extract session handle from result
	sessionHandleVariant, ok := result["session_handle"]
	if !ok {
		return "", fmt.Errorf("session_handle not found in response")
	}

	sessionHandle, ok := sessionHandleVariant.(string)
	if !ok {
		return "", fmt.Errorf("session_handle is not a string")
	}

	return sessionHandle, nil
}

// selectSources selects which monitors to capture
func (sc *ScreenCapture) selectSources(sessionHandle string) error {
	obj := sc.conn.Object(portalDest, portalPath)

	options := map[string]dbus.Variant{
		"types":        dbus.MakeVariant(uint32(1)), // 1 = monitor, 2 = window
		"multiple":     dbus.MakeVariant(false),     // Single monitor for now
		"handle_token": dbus.MakeVariant(fmt.Sprintf("khuey_%d", rand.Intn(999999))),
	}

	var requestPath dbus.ObjectPath
	err := obj.Call(screenCastIface+".SelectSources", 0, dbus.ObjectPath(sessionHandle), options).Store(&requestPath)
	if err != nil {
		return fmt.Errorf("failed to select sources: %w", err)
	}

	// Wait for user to select source
	_, err = sc.waitForResponse(requestPath)
	if err != nil {
		return fmt.Errorf("failed to get source selection: %w", err)
	}

	return nil
}

// startStream starts the Pipewire stream
func (sc *ScreenCapture) startStream(sessionHandle string) (uint32, error) {
	obj := sc.conn.Object(portalDest, portalPath)

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(fmt.Sprintf("khuey_%d", rand.Intn(999999))),
	}

	var requestPath dbus.ObjectPath
	err := obj.Call(screenCastIface+".Start", 0, dbus.ObjectPath(sessionHandle), "", options).Store(&requestPath)
	if err != nil {
		return 0, fmt.Errorf("failed to start stream: %w", err)
	}

	// Wait for stream to start
	result, err := sc.waitForResponse(requestPath)
	if err != nil {
		return 0, fmt.Errorf("failed to start stream: %w", err)
	}

	// Extract stream node from result
	// Format is: streams: [[node_id, properties], ...]
	streamsOuter, ok := result["streams"].([][]interface{})
	if !ok {
		return 0, fmt.Errorf("streams field not found or wrong type")
	}
	
	if len(streamsOuter) == 0 {
		return 0, fmt.Errorf("no streams in response")
	}

	// Get first stream [node_id, properties]
	firstStream := streamsOuter[0]
	if len(firstStream) < 1 {
		return 0, fmt.Errorf("stream has no elements")
	}

	// Node ID is the first element
	nodeID, ok := firstStream[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("node_id not found or wrong type")
	}

	return nodeID, nil
}

// waitForResponse waits for a portal Request response
func (sc *ScreenCapture) waitForResponse(requestPath dbus.ObjectPath) (map[string]interface{}, error) {
	// Add signal match for Response
	if err := sc.conn.AddMatchSignal(dbus.WithMatchObjectPath(requestPath)); err != nil {
		return nil, fmt.Errorf("failed to add match: %w", err)
	}
	defer sc.conn.RemoveMatchSignal(dbus.WithMatchObjectPath(requestPath))

	signals := make(chan *dbus.Signal, 10)
	sc.conn.Signal(signals)
	defer sc.conn.RemoveSignal(signals)

	// Wait for Response signal
	select {
	case sig := <-signals:
		if len(sig.Body) < 2 {
			return nil, fmt.Errorf("invalid response body")
		}

		responseCode, ok := sig.Body[0].(uint32)
		if !ok || responseCode != 0 {
			return nil, fmt.Errorf("request failed with code %d", responseCode)
		}

		results, ok := sig.Body[1].(map[string]dbus.Variant)
		if !ok {
			return nil, fmt.Errorf("invalid results format")
		}

		// Convert dbus.Variant map to regular map
		result := make(map[string]interface{})
		for k, v := range results {
			result[k] = v.Value()
		}

		return result, nil

	case <-sc.ctx.Done():
		return nil, fmt.Errorf("context cancelled")
	}
}
