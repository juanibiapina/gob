package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// ProcessHandle represents a running process
type ProcessHandle interface {
	Pid() int
	Wait() error
	Signal(sig syscall.Signal) error
	IsRunning() bool
}

// ProcessExecutor handles process creation
type ProcessExecutor interface {
	Start(command []string, workdir string, env []string, stdoutPath, stderrPath string) (ProcessHandle, error)
}

// RealProcessExecutor implements ProcessExecutor using os/exec
type RealProcessExecutor struct{}

// realProcessHandle wraps exec.Cmd to implement ProcessHandle
type realProcessHandle struct {
	cmd *exec.Cmd
}

func (h *realProcessHandle) Pid() int {
	return h.cmd.Process.Pid
}

func (h *realProcessHandle) Wait() error {
	return h.cmd.Wait()
}

func (h *realProcessHandle) Signal(sig syscall.Signal) error {
	// Send to process group (negative PID)
	return syscall.Kill(-h.cmd.Process.Pid, sig)
}

func (h *realProcessHandle) IsRunning() bool {
	err := syscall.Kill(h.cmd.Process.Pid, syscall.Signal(0))
	return err == nil
}

// Start starts a process with the given command and environment
func (e *RealProcessExecutor) Start(command []string, workdir string, env []string, stdoutPath, stderrPath string) (ProcessHandle, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = workdir

	// Use the provided environment (clean, not inherited from daemon)
	// This ensures the process runs with the client's environment
	cmd.Env = env

	// Create a new process group so we can signal all children together
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Create log files
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout log file: %w", err)
	}

	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		stdoutFile.Close()
		return nil, fmt.Errorf("failed to open stderr log file: %w", err)
	}

	// Redirect stdin to /dev/null
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return nil, fmt.Errorf("failed to open /dev/null: %w", err)
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.Stdin = devNull

	// Start the process
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		devNull.Close()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Close file descriptors in daemon (child keeps them)
	stdoutFile.Close()
	stderrFile.Close()
	devNull.Close()

	return &realProcessHandle{cmd: cmd}, nil
}
