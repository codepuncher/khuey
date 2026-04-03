package common

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"
)

// SanitizeForLog sanitizes sensitive strings for logging by masking the middle
func SanitizeForLog(s string) string {
	if s == "" {
		return "****"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// ValidateDBusString validates a DBus string parameter
func ValidateDBusString(name, value string, maxLen int) error {
	if len(value) > maxLen {
		return fmt.Errorf("%s exceeds maximum length %d", name, maxLen)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid UTF-8", name)
	}
	return nil
}

// NewHueHTTPClient creates a standard HTTP client for Hue bridge communication
//
// SECURITY NOTE: InsecureSkipVerify is required for Philips Hue bridges which use
// self-signed certificates. This disables TLS certificate verification, making the
// connection vulnerable to man-in-the-middle attacks.
//
// MITIGATION: This is generally acceptable for local IoT devices on trusted networks
// because:
// 1. Hue bridges only communicate on local network (192.168.x.x)
// 2. Attack requires physical network access
// 3. Bridge API keys are user-specific and rate-limited
//
// FUTURE ENHANCEMENT: Consider implementing certificate pinning by storing the bridge's
// certificate fingerprint during initial setup and verifying it on subsequent connections.
func NewHueHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Required for Hue bridge self-signed certs
			},
		},
	}
}
