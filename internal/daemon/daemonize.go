package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// StartDaemon starts the daemon process as a detached background process.
// It uses exec.Command with Setsid to create a new session, ensuring the
// daemon survives after the parent exits.
func StartDaemon() error {
	// Get path to current executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start daemon as detached process
	cmd := exec.Command(exe, "daemon")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for the process
	go cmd.Wait()

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
