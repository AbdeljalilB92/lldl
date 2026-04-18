package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	apperr "github.com/AbdeljalilB92/lldl/shared/errors"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	store := NewJSONStore(path)

	original := &Config{
		AuthToken:       "li_at_secret_value",
		CourseDirectory: "/courses",
		Quality:         "720",
		CourseURL:       "https://www.linkedin.com/learning/go-essentials",
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.AuthToken != original.AuthToken {
		t.Errorf("AuthToken: got %q, want %q", loaded.AuthToken, original.AuthToken)
	}
	if loaded.CourseDirectory != original.CourseDirectory {
		t.Errorf("CourseDirectory: got %q, want %q", loaded.CourseDirectory, original.CourseDirectory)
	}
	if loaded.Quality != original.Quality {
		t.Errorf("Quality: got %q, want %q", loaded.Quality, original.Quality)
	}
	if loaded.CourseURL != original.CourseURL {
		t.Errorf("CourseURL: got %q, want %q", loaded.CourseURL, original.CourseURL)
	}
}

func TestLoadMissingFile(t *testing.T) {
	store := NewJSONStore("/nonexistent/path/config.json")

	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	var cfgErr *apperr.ConfigError
	if !isConfigError(t, err, cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("not valid json{{{"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewJSONStore(path)

	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}

	var cfgErr *apperr.ConfigError
	if !isConfigError(t, err, cfgErr) {
		t.Fatalf("expected ConfigError, got %T: %v", err, err)
	}
}

func TestTokenBase64EncodedInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	store := NewJSONStore(path)

	token := "raw_plaintext_token"
	cfg := &Config{AuthToken: token, CourseDirectory: "/dl", Quality: "540"}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read raw file and verify token is base64-encoded in JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	encodedInFile := raw["authToken"]

	// Token in file should be base64, not plaintext
	if encodedInFile == token {
		t.Error("token stored as plaintext in file, expected base64 encoding")
	}

	decoded, err := base64.StdEncoding.DecodeString(encodedInFile)
	if err != nil {
		t.Fatalf("token in file is not valid base64: %v", err)
	}
	if string(decoded) != token {
		t.Errorf("decoded token: got %q, want %q", string(decoded), token)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	got := DefaultConfigPath()
	// Should use the OS config directory, not the current directory.
	if got == "./config.json" {
		t.Error("DefaultConfigPath should not use current directory")
	}
	if got == "" {
		t.Error("DefaultConfigPath should not be empty")
	}
}

// isConfigError is a helper that type-asserts err to *apperr.ConfigError.
func isConfigError(t *testing.T, err error, _ *apperr.ConfigError) bool {
	t.Helper()
	var cfgErr *apperr.ConfigError
	return errors.As(err, &cfgErr)
}
