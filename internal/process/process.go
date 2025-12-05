package process

import (
	"syscall"
)

// IsProcessRunning checks if a process with the given PID is currently running
// It uses kill(pid, 0) which sends no signal but checks if the process exists
func IsProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	// This doesn't actually send a signal, just checks permissions and existence
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}
