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
