package gaming

import (
	"os/exec"
	"strings"
)

// SteamDetector checks for Steam game processes
// Steam launches games via reaper process with AppId parameter
type SteamDetector struct{}

// NewSteamDetector creates a new Steam game detector
func NewSteamDetector() *SteamDetector {
	return &SteamDetector{}
}

// IsActive checks if Steam game is running
// Detects reaper process with AppId parameter
func (s *SteamDetector) IsActive() bool {
	// Check for reaper process with AppId
	cmd := exec.Command("pgrep", "-a", "reaper")
	output, err := cmd.Output()
	if err != nil {
		// pgrep failed or no reaper process - not an error, just no Steam games
		return false
	}

	// Steam launches games via reaper with AppId parameter
	// Example: "reaper SteamLaunch AppId=489830 -- ..."
	return strings.Contains(string(output), "AppId=")
}

// GetAppId returns the Steam AppId of running game (if any)
// Returns empty string if no game is running
// Useful for logging and debugging
func (s *SteamDetector) GetAppId() string {
	cmd := exec.Command("pgrep", "-a", "reaper")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse AppId from: "reaper SteamLaunch AppId=489830 -- ..."
	outStr := string(output)
	if idx := strings.Index(outStr, "AppId="); idx != -1 {
		rest := outStr[idx+6:] // Skip "AppId="
		if end := strings.IndexAny(rest, " \n\t"); end != -1 {
			return rest[:end]
		}
		// No delimiter found - rest of string is AppId
		return strings.TrimSpace(rest)
	}

	return ""
}
