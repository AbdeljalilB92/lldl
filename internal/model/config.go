package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the application configuration.
type Config struct {
	AuthToken       string `json:"authToken"`       // base64-encoded li_at cookie
	CourseDirectory string `json:"courseDirectory"`
	Quality         string `json:"quality"`          // "720", "540", "360"
}

// LoadConfig reads a JSON config file and returns the decoded configuration.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// Save encodes the token to base64 and writes the config as JSON to the given path.
// Uses write-to-temp + rename for atomic save, with restrictive permissions
// since the auth token should not be world-readable.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming config file: %w", err)
	}

	return nil
}

// GetAuthToken decodes the base64-encoded auth token and returns the raw li_at cookie value.
func (c *Config) GetAuthToken() string {
	if c.AuthToken == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(c.AuthToken)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// SetAuthToken encodes the raw li_at cookie value to base64 and stores it.
func (c *Config) SetAuthToken(token string) {
	c.AuthToken = base64.StdEncoding.EncodeToString([]byte(token))
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return "./config.json"
}
