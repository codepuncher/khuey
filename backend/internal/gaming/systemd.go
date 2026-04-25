package gaming

import (
	"os/exec"
	"strings"
)

// SystemdDetector checks for game-related systemd inhibitor locks
// This detector is optimized for CachyOS game-performance utility
type SystemdDetector struct{}

// NewSystemdDetector creates a new systemd-inhibit detector
func NewSystemdDetector() *SystemdDetector {
	return &SystemdDetector{}
}

// IsActive checks if gaming is active via systemd-inhibit
// CachyOS automatically creates inhibitor locks when games are running
func (s *SystemdDetector) IsActive() bool {
	cmd := exec.Command("systemd-inhibit", "--list")
	output, err := cmd.Output()
	if err != nil {
		// systemd-inhibit not available or error - return false gracefully
		return false
	}

	outStr := strings.ToLower(string(output))

	// Check for CachyOS game-performance utility (highest confidence)
	if strings.Contains(outStr, "cachyos") && strings.Contains(outStr, "game") {
		return true
	}

	// Check for generic game-performance inhibitor
	if strings.Contains(outStr, "game-performance") {
		return true
	}

	// Check for gaming-related inhibitor with "block" mode
	// Pattern: "game" or "gaming" + "block" mode
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if (strings.Contains(lower, "game") || strings.Contains(lower, "gaming")) &&
			strings.Contains(lower, "block") {
			return true
		}
	}

	return false
}
