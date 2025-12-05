package daemon

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Logger is the daemon's structured logger
var Logger *slog.Logger

func init() {
	// Default to discarding logs until InitLogger is called
	Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

// InitLogger initializes the daemon logger with the specified log file path.
// If logPath is empty, logs are discarded.
// The log file is created with mode 0600 (user-only).
func InitLogger(logPath string) error {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Log everything, filter at output
	}

	if logPath == "" {
		handler = slog.NewTextHandler(io.Discard, opts)
	} else {
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
			return err
		}

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		handler = slog.NewTextHandler(file, opts)
	}

	Logger = slog.New(handler)
	return nil
}

// GetLogPath returns the path to the daemon log file
func GetLogPath() (string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "daemon.log"), nil
}
