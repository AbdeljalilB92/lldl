// Package config handles loading and saving application configuration.
// Auth tokens are stored as plaintext in the Config struct but base64-encoded
// in the JSON file for basic obfuscation.
//
// SECURITY: The config file contains sensitive authentication tokens.
// It MUST be written with restrictive file permissions (0600) and stored
// in a user-local directory. Never commit config files to version control.
package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration.
type Config struct {
	AuthToken       string
	CourseDirectory string
	Quality         string
}

// DefaultConfigPath returns the default config file path using the OS-specific
// user config directory (~/.config/lldl/config.json on Linux, equivalent on
// macOS/Windows).
func DefaultConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to current directory when home directory is unavailable.
		// Degraded state: config may not be in a secure location.
		return "config.json"
	}
	return filepath.Join(configDir, "lldl", "config.json")
}
