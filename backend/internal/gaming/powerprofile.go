package gaming

import (
	"os/exec"
	"strings"
)

// PowerProfileDetector checks system power profile
// CachyOS automatically switches to "performance" profile when gaming
type PowerProfileDetector struct{}

// NewPowerProfileDetector creates a new power profile detector
func NewPowerProfileDetector() *PowerProfileDetector {
	return &PowerProfileDetector{}
}

// IsPerformanceMode checks if power profile is set to performance
// Used as secondary validation with Steam AppId check
func (p *PowerProfileDetector) IsPerformanceMode() bool {
	cmd := exec.Command("powerprofilesctl", "get")
	output, err := cmd.Output()
	if err != nil {
		// powerprofilesctl not available - return false gracefully
		return false
	}

	profile := strings.TrimSpace(string(output))
	return profile == "performance"
}
