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
// Returns a cleanup function to close the log file and an error if the
// log directory or file cannot be created. Callers should defer the
// cleanup function to ensure the log file is flushed on exit.
func Setup(level slog.Level, logDir string) (func(), error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory %s: %w", logDir, err)
	}

	logPath := filepath.Join(logDir, "log.txt")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", logPath, err)
	}

	multiWriter := io.MultiWriter(os.Stderr, logFile)
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	cleanup := func() {
		logFile.Close()
	}
	return cleanup, nil
}

// New creates a named logger scoped to a specific component.
// The component string is prepended to every log message via the
// "component" attribute, enabling easy filtering in log output.
func New(component string) *slog.Logger {
	return slog.Default().With("component", component)
}
