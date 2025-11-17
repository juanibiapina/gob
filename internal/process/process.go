package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// StartDetached starts a command as a detached background process
// It uses Setpgid to create a new process group, ensuring the process
// continues running after the parent CLI exits
func StartDetached(command string, args []string) (int, error) {
	cmd := exec.Command(command, args...)

	// Create a new process group to detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Redirect stdout and stderr to /dev/null to prevent blocking
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/null: %w", err)
	}

	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull

	// Start the process (non-blocking)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	// Get the PID
	pid := cmd.Process.Pid

	// Release the process so Go stops tracking it
	// This allows the parent to exit without waiting for the child
	if err := cmd.Process.Release(); err != nil {
		return 0, fmt.Errorf("failed to release process: %w", err)
	}

	return pid, nil
}

// IsProcessRunning checks if a process with the given PID is currently running
// It uses kill(pid, 0) which sends no signal but checks if the process exists
func IsProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	// This doesn't actually send a signal, just checks permissions and existence
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}

// StopProcess sends SIGTERM to a process for graceful termination
func StopProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

// KillProcess sends SIGKILL to forcefully terminate a process
func KillProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
