package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLevel(t *testing.T) {
	got := DefaultLevel()
	if got != slog.LevelInfo {
		t.Errorf("DefaultLevel() = %v, want %v", got, slog.LevelInfo)
	}
}

func TestNewReturnsNonNil(t *testing.T) {
	logger := New("[Auth][ValidateToken]")
	if logger == nil {
		t.Fatal("New() returned nil logger")
	}
}

func TestNew_ComponentAttribute(t *testing.T) {
	logger := New("[Download]")
	if logger == nil {
		t.Fatal("New() returned nil logger")
	}
	// Verify the logger is usable — call a no-op log to ensure it doesn't panic.
	// slog may output to stderr, but the test only checks that it doesn't crash.
	logger.Debug("test message")
}

func TestSetup_CreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	err := Setup(slog.LevelDebug, dir)
	if err != nil {
		t.Fatalf("Setup() returned error: %v", err)
	}

	logPath := filepath.Join(dir, "log.txt")
	_, err = os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestSetup_InvalidDirectory(t *testing.T) {
	// A path that cannot be created as a directory.
	err := Setup(slog.LevelInfo, "/dev/null/impossible")
	if err == nil {
		t.Fatal("expected error for invalid directory, got nil")
	}
}
