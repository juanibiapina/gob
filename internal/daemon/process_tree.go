package daemon

import (
	"syscall"

	"github.com/shirou/gopsutil/v4/process"
)

// getProcessTreePIDs returns all PIDs in a process tree (root + all descendants).
// Returns empty slice if root process doesn't exist.
func getProcessTreePIDs(rootPID int) []int {
	var pids []int

	var walk func(pid int32)
	walk = func(pid int32) {
		proc, err := process.NewProcess(pid)
		if err != nil {
			return // Process gone
		}

		pids = append(pids, int(pid))

		children, _ := proc.Children()
		for _, child := range children {
			walk(child.Pid)
		}
	}

	walk(int32(rootPID))
	return pids
}

// filterRunningPIDs returns only the PIDs that are still running.
func filterRunningPIDs(pids []int) []int {
	var running []int
	for _, pid := range pids {
		if syscall.Kill(pid, 0) == nil {
			running = append(running, pid)
		}
	}
	return running
}

// killPIDs sends the given signal to each PID individually.
// Ignores errors (e.g., ESRCH if process already gone).
func killPIDs(pids []int, sig syscall.Signal) {
	for _, pid := range pids {
		syscall.Kill(pid, sig)
	}
}
