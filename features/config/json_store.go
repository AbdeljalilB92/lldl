package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/AbdeljalilB92/lldl/lib/atomic"
	apperr "github.com/AbdeljalilB92/lldl/shared/errors"
)

// Compile-time guarantee that jsonConfigStore satisfies Store.
var _ Store = (*jsonConfigStore)(nil)

// jsonConfigStore persists configuration as JSON with base64-encoded auth tokens.
type jsonConfigStore struct {
	path string
}

// jsonConfig is the on-disk representation where AuthToken is base64-encoded.
type jsonConfig struct {
	AuthToken       string `json:"authToken"`
	CourseDirectory string `json:"courseDirectory"`
	Quality         string `json:"quality"`
}

// NewJSONStore creates a Store backed by a JSON file at the given path.
func NewJSONStore(path string) Store {
	return &jsonConfigStore{path: path}
}

// Load reads and decodes the JSON config file.
// The base64-encoded token is decoded before returning.
func (s *jsonConfigStore) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, &apperr.ConfigError{Path: s.path, Cause: fmt.Errorf("reading config file: %w", err)}
	}

	var raw jsonConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &apperr.ConfigError{Path: s.path, Cause: fmt.Errorf("parsing config file: %w", err)}
	}

	cfg := &Config{
		CourseDirectory: raw.CourseDirectory,
		Quality:         raw.Quality,
	}

	if raw.AuthToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw.AuthToken)
		if err != nil {
			return nil, &apperr.ConfigError{Path: s.path, Cause: fmt.Errorf("decoding auth token: %w", err)}
		}
		cfg.AuthToken = string(decoded)
	}

	return cfg, nil
}

// Save encodes the auth token to base64, marshals the config as indented JSON,
// and writes the file atomically with 0600 permissions.
func (s *jsonConfigStore) Save(cfg *Config) error {
	raw := jsonConfig{
		CourseDirectory: cfg.CourseDirectory,
		Quality:         cfg.Quality,
	}
	if cfg.AuthToken != "" {
		raw.AuthToken = base64.StdEncoding.EncodeToString([]byte(cfg.AuthToken))
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return &apperr.ConfigError{Path: s.path, Cause: fmt.Errorf("marshaling config: %w", err)}
	}

	if err := atomic.WriteFile(s.path, data, 0600); err != nil {
		return &apperr.ConfigError{Path: s.path, Cause: fmt.Errorf("writing config file: %w", err)}
	}

	return nil
}
