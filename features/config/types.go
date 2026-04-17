// Package config handles loading and saving application configuration.
// Auth tokens are stored as plaintext in the Config struct but base64-encoded
// in the JSON file for basic obfuscation.
package config

// Config holds the application configuration.
type Config struct {
	AuthToken       string
	CourseDirectory string
	Quality         string
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return "./config.json"
}
