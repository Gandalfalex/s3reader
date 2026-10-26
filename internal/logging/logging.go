// Package logging writes the application log to a file (the TUI owns the terminal).
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// DefaultPath is $XDG_STATE_HOME/s3reader/s3reader.log (~/.local/state/...).
func DefaultPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "s3reader.log")
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "s3reader", "s3reader.log")
}

// Setup installs a file-backed slog default logger. The TUI owns the
// terminal, so nothing may be written to stdout/stderr while it runs.
func Setup(path string, debug bool) (io.Closer, error) {
	if path == "-" || path == "off" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return io.NopCloser(nil), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})))
	return f, nil
}
