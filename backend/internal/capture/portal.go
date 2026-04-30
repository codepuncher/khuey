package capture

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalDest      = "org.freedesktop.portal.Desktop"
	portalPath      = "/org/freedesktop/portal/desktop"
	screenCastIface = "org.freedesktop.portal.ScreenCast"

	// MaxPortalHandleID is the maximum value for portal session/handle IDs
	MaxPortalHandleID = 999999
)

// PortalError represents an error from the XDG Desktop Portal
type PortalError struct {
	Type string // "connection", "permission_denied", "session_failed", etc.
	Msg  string
	Hint string // User-friendly guidance
}

func (e *PortalError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s (Hint: %s)", e.Type, e.Msg, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Msg)
}

// createSession creates a new ScreenCast session via XDG Desktop Portal
func (sc *ScreenCapture) createSession() (string, error) {
	obj := sc.conn.Object(portalDest, portalPath)

	// Generate unique session token using crypto/rand
	sessionNum, err := rand.Int(rand.Reader, big.NewInt(MaxPortalHandleID))
	if err != nil {
		return "", &PortalError{
			Type: "session_failed",
			Msg:  "Failed to generate session token",
			Hint: "Cryptographic random number generation failed",
		}
	}
	sessionToken := fmt.Sprintf("khuey_session_%d", sessionNum.Int64())

	handleNum, err := rand.Int(rand.Reader, big.NewInt(MaxPortalHandleID))
	if err != nil {
		return "", &PortalError{
			Type: "session_failed",
			Msg:  "Failed to generate handle token",
			Hint: "Cryptographic random number generation failed",
		}
	}

	// Prepare options for CreateSession
	options := map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(sessionToken),
		"handle_token":         dbus.MakeVariant(fmt.Sprintf("khuey_%d", handleNum.Int64())),
	}

	var requestPath dbus.ObjectPath
	err = obj.Call(screenCastIface+".CreateSession", 0, options).Store(&requestPath)
	if err != nil {
		return "", &PortalError{
			Type: "session_failed",
			Msg:  "Failed to create portal session",
			Hint: "Ensure xdg-desktop-portal is running and configured correctly",
		}
	}

	// Wait for response via Request interface
	result, err := sc.waitForResponse(requestPath)
	if err != nil {
		return "", err // Already wrapped by waitForResponse
	}

	// Extract session handle from result
	sessionHandleVariant, ok := result["session_handle"]
	if !ok {
		return "", &PortalError{
			Type: "session_failed",
			Msg:  "Session handle not found in portal response",
			Hint: "This may indicate a portal configuration issue",
		}
	}

	sessionHandle, ok := sessionHandleVariant.(string)
	if !ok {
		return "", &PortalError{
			Type: "session_failed",
			Msg:  "Session handle has incorrect type",
			Hint: "This may indicate a portal version mismatch",
		}
	}

	return sessionHandle, nil
}

// selectSources selects which monitors to capture
// restoreToken: Optional token from previous session to skip permission dialog
func (sc *ScreenCapture) selectSources(sessionHandle string, restoreToken string) error {
	obj := sc.conn.Object(portalDest, portalPath)

	handleNum, err := rand.Int(rand.Reader, big.NewInt(MaxPortalHandleID))
	if err != nil {
		return &PortalError{
			Type: "select_sources_failed",
			Msg:  "Failed to generate handle token",
			Hint: "Cryptographic random number generation failed",
		}
	}

	options := map[string]dbus.Variant{
		"types":        dbus.MakeVariant(uint32(1)), // 1 = monitor, 2 = window
		"multiple":     dbus.MakeVariant(false),     // Single monitor for now
		"persist_mode": dbus.MakeVariant(uint32(2)), // 2 = persist until explicitly revoked
		"handle_token": dbus.MakeVariant(fmt.Sprintf("khuey_%d", handleNum.Int64())),
	}

	// Add restore token if available (skip permission dialog)
	if restoreToken != "" {
		options["restore_token"] = dbus.MakeVariant(restoreToken)
	}

	var requestPath dbus.ObjectPath
	err = obj.Call(screenCastIface+".SelectSources", 0, dbus.ObjectPath(sessionHandle), options).Store(&requestPath)
	if err != nil {
		return &PortalError{
			Type: "select_sources_failed",
			Msg:  "Failed to call SelectSources",
			Hint: "Ensure screen sharing is supported by your desktop environment",
		}
	}

	// Wait for user to select source
	_, err = sc.waitForResponse(requestPath)
	if err != nil {
		return err // Already wrapped by waitForResponse
	}

	return nil
}

// startStream starts the Pipewire stream
// Returns: (nodeID, newRestoreToken, error)
// newRestoreToken should be saved for next session to skip permission dialog
func (sc *ScreenCapture) startStream(sessionHandle string) (uint32, string, error) {
	obj := sc.conn.Object(portalDest, portalPath)

	handleNum, err := rand.Int(rand.Reader, big.NewInt(MaxPortalHandleID))
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate handle token: %w", err)
	}

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(fmt.Sprintf("khuey_%d", handleNum.Int64())),
	}

	var requestPath dbus.ObjectPath
	err = obj.Call(screenCastIface+".Start", 0, dbus.ObjectPath(sessionHandle), "", options).Store(&requestPath)
	if err != nil {
		return 0, "", fmt.Errorf("failed to start stream: %w", err)
	}

	// Wait for stream to start
	result, err := sc.waitForResponse(requestPath)
	if err != nil {
		return 0, "", fmt.Errorf("failed to start stream: %w", err)
	}

	// Extract stream node from result
	// Format is: streams: [[node_id, properties], ...]
	streamsOuter, ok := result["streams"].([][]interface{})
	if !ok {
		return 0, "", fmt.Errorf("streams field not found or wrong type")
	}

	if len(streamsOuter) == 0 {
		return 0, "", fmt.Errorf("no streams in response")
	}

	// Get first stream [node_id, properties]
	firstStream := streamsOuter[0]
	if len(firstStream) < 1 {
		return 0, "", fmt.Errorf("stream has no elements")
	}

	// Node ID is the first element
	nodeID, ok := firstStream[0].(uint32)
	if !ok {
		return 0, "", fmt.Errorf("node_id not found or wrong type")
	}

	// Extract restore token for next session (optional - may not be present)
	var newRestoreToken string
	if tokenVariant, ok := result["restore_token"]; ok {
		if token, ok := tokenVariant.(string); ok {
			newRestoreToken = token
		}
	}

	return nodeID, newRestoreToken, nil
}

// waitForResponse waits for a portal Request response
func (sc *ScreenCapture) waitForResponse(requestPath dbus.ObjectPath) (map[string]interface{}, error) {
	// Add signal match for Response
	if err := sc.conn.AddMatchSignal(dbus.WithMatchObjectPath(requestPath)); err != nil {
		return nil, &PortalError{
			Type: "connection",
			Msg:  "Failed to add signal match",
			Hint: "DBus connection may be unstable",
		}
	}
	defer func() {
		if err := sc.conn.RemoveMatchSignal(dbus.WithMatchObjectPath(requestPath)); err != nil {
			log.Printf("warn: failed to remove signal match: %v", err)
		}
	}()

	signals := make(chan *dbus.Signal, 10)
	sc.conn.Signal(signals)
	defer sc.conn.RemoveSignal(signals)

	// Wait for Response signal
	select {
	case sig := <-signals:
		if len(sig.Body) < 2 {
			return nil, &PortalError{
				Type: "invalid_response",
				Msg:  "Portal response has invalid format",
				Hint: "This may indicate a portal version mismatch",
			}
		}

		responseCode, ok := sig.Body[0].(uint32)
		if !ok {
			return nil, &PortalError{
				Type: "invalid_response",
				Msg:  "Portal response code has wrong type",
				Hint: "This may indicate a portal version mismatch",
			}
		}

		if responseCode != 0 {
			// Response code != 0 means user denied or cancelled
			if responseCode == 1 {
				return nil, &PortalError{
					Type: "permission_denied",
					Msg:  "Screen sharing permission was denied",
					Hint: "Please approve the screen sharing dialog when prompted. You can try again by clicking 'Start Screen Sync'.",
				}
			}
			return nil, &PortalError{
				Type: "request_failed",
				Msg:  fmt.Sprintf("Portal request failed with code %d", responseCode),
				Hint: "The screen sharing request was cancelled or failed",
			}
		}

		results, ok := sig.Body[1].(map[string]dbus.Variant)
		if !ok {
			return nil, &PortalError{
				Type: "invalid_response",
				Msg:  "Portal results have wrong type",
				Hint: "This may indicate a portal version mismatch",
			}
		}

		// Convert dbus.Variant map to regular map
		result := make(map[string]interface{})
		for k, v := range results {
			result[k] = v.Value()
		}

		return result, nil

	case <-sc.ctx.Done():
		return nil, &PortalError{
			Type: "cancelled",
			Msg:  "Portal request was cancelled",
			Hint: "The operation was interrupted",
		}

	case <-time.After(2 * time.Minute):
		return nil, &PortalError{
			Type: "timeout",
			Msg:  "User did not respond to permission dialog within 2 minutes",
			Hint: "Please approve screen sharing when prompted",
		}
	}
}
