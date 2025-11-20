package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// StartDetached starts a command as a detached background process
// It uses Setpgid to create a new process group, ensuring the process
// continues running after the parent CLI exits
// Returns the PID and any error
func StartDetached(command string, args []string, jobID int64, storageDir string) (int, error) {
	cmd := exec.Command(command, args...)

	// Create a new process group to detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Create log files for stdout and stderr
	stdoutPath := fmt.Sprintf("%s/%d.stdout.log", storageDir, jobID)
	stderrPath := fmt.Sprintf("%s/%d.stderr.log", storageDir, jobID)

	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open stdout log file: %w", err)
	}

	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		stdoutFile.Close()
		return 0, fmt.Errorf("failed to open stderr log file: %w", err)
	}

	// Redirect stdin to /dev/null
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return 0, fmt.Errorf("failed to open /dev/null: %w", err)
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.Stdin = devNull

	// Start the process (non-blocking)
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		devNull.Close()
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	// Get the PID
	pid := cmd.Process.Pid

	// Close the file descriptors in the parent process
	// The child process keeps them open
	stdoutFile.Close()
	stderrFile.Close()
	devNull.Close()

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

// StopProcess sends SIGTERM to a process group for graceful termination
// Uses negative PID to target the entire process group, ensuring all child processes are terminated
func StopProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// KillProcess sends SIGKILL to forcefully terminate a process group
// Uses negative PID to target the entire process group, ensuring all child processes are terminated
func KillProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// StopProcessWithTimeout attempts graceful termination with SIGTERM, then escalates to SIGKILL
// It polls for process termination and ensures the process is actually dead before returning
// Returns an error only if the process cannot be terminated even with SIGKILL
func StopProcessWithTimeout(pid int, gracefulTimeout time.Duration, forceTimeout time.Duration) error {
	// Check if process is already dead
	if !IsProcessRunning(pid) {
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := StopProcess(pid); err != nil {
		// If the process doesn't exist, that's fine
		if err == syscall.ESRCH {
			return nil
		}
		// For other errors, continue to try SIGKILL
	}

	// Poll for graceful termination
	pollInterval := 100 * time.Millisecond
	deadline := time.Now().Add(gracefulTimeout)
	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return nil
		}
		time.Sleep(pollInterval)
	}

	// Process didn't terminate gracefully, escalate to SIGKILL
	if err := KillProcess(pid); err != nil {
		// If the process doesn't exist, that's fine
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("failed to send SIGKILL: %w", err)
	}

	// Poll for forced termination
	deadline = time.Now().Add(forceTimeout)
	for time.Now().Before(deadline) {
		if !IsProcessRunning(pid) {
			return nil
		}
		time.Sleep(pollInterval)
	}

	// If still running after SIGKILL, something is very wrong
	if IsProcessRunning(pid) {
		return fmt.Errorf("process %d still running after SIGKILL", pid)
	}

	return nil
}
