package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// StartDaemon starts the daemon process as a fully detached background process.
// The daemon command uses go-daemon internally to properly daemonize with PPID=1.
func StartDaemon() error {
	// Get path to current executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start daemon - it will daemonize itself using go-daemon
	cmd := exec.Command(exe, "daemon")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for socket to appear
	socketPath, err := GetSocketPath()
	if err != nil {
		return fmt.Errorf("failed to get socket path: %w", err)
	}

	for i := 0; i < 20; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start (socket not created)")
}
