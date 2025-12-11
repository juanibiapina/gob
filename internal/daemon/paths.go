package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// GetRuntimeDir returns the runtime directory for gob daemon files
func GetRuntimeDir() (string, error) {
	return filepath.Join(xdg.RuntimeDir, "gob"), nil
}

// GetStateDir returns the state directory for persistent data (survives reboots)
func GetStateDir() (string, error) {
	return filepath.Join(xdg.StateHome, "gob"), nil
}

// GetSocketPath returns the path to the daemon Unix socket
func GetSocketPath() (string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "daemon.sock"), nil
}

// GetPIDPath returns the path to the daemon PID file
func GetPIDPath() (string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "daemon.pid"), nil
}

// GetDatabasePath returns the path to the SQLite database file
func GetDatabasePath() (string, error) {
	stateDir, err := GetStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "state.db"), nil
}

// GetLogDir returns the directory for run log files
func GetLogDir() (string, error) {
	stateDir, err := GetStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "logs"), nil
}

// EnsureRuntimeDir creates the runtime directory if it doesn't exist
func EnsureRuntimeDir() (string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create runtime directory: %w", err)
	}

	return runtimeDir, nil
}

// EnsureStateDir creates the state directory if it doesn't exist
func EnsureStateDir() (string, error) {
	stateDir, err := GetStateDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}

	return stateDir, nil
}

// EnsureLogDir creates the log directory if it doesn't exist
func EnsureLogDir() (string, error) {
	logDir, err := GetLogDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	return logDir, nil
}
