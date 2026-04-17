package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// DefaultLevel returns the standard log level for the application.
func DefaultLevel() slog.Level {
	return slog.LevelInfo
}

// Setup configures the global slog default logger with a multi-writer
// that outputs to both stderr and a log file in logDir.
// Returns an error if the log directory or file cannot be created.
func Setup(level slog.Level, logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log directory %s: %w", logDir, err)
	}

	logPath := filepath.Join(logDir, "log.txt")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", logPath, err)
	}

	multiWriter := io.MultiWriter(os.Stderr, logFile)
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	return nil
}

// New creates a named logger scoped to a specific component.
// The component string is prepended to every log message via the
// "component" attribute, enabling easy filtering in log output.
func New(component string) *slog.Logger {
	return slog.Default().With("component", component)
}
