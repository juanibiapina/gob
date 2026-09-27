package daemon

import (
	"syscall"

	"github.com/shirou/gopsutil/v4/process"
)

// getProcessTreePIDs returns the root and its descendants from one process table snapshot.
// Returns an empty slice if the root is absent, and an error if inspection fails.
func getProcessTreePIDs(rootPID int) ([]int, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}
	children := make(map[int32][]int32)
	found := false
	for _, proc := range procs {
		if int(proc.Pid) == rootPID {
			found = true
		}
		parent, err := proc.Ppid()
		if err == nil {
			children[parent] = append(children[parent], proc.Pid)
		}
	}
	if !found {
		return nil, nil
	}

	pids := []int{rootPID}
	for i := 0; i < len(pids); i++ {
		for _, child := range children[int32(pids[i])] {
			pids = append(pids, int(child))
		}
	}
	return pids, nil
}

// captureProcessIdentities records the identity of each observed live descendant.
func captureProcessIdentities(pids []int) (map[int]int64, error) {
	identities := make(map[int]int64, len(pids))
	for _, pid := range pids {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			if processExists(pid) {
				return nil, err
			}
			continue
		}
		created, err := proc.CreateTime()
		if err != nil {
			if processExists(pid) {
				return nil, err
			}
			continue
		}
		identities[pid] = created
	}
	return identities, nil
}

// filterRunningPIDs ignores exited, zombie, and reused PIDs from a captured snapshot.
func filterRunningPIDs(identities map[int]int64) ([]int, error) {
	var running []int
	for pid, created := range identities {
		if syscall.Kill(pid, 0) != nil {
			continue
		}
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			if processExists(pid) {
				return nil, err
			}
			continue
		}
		current, err := proc.CreateTime()
		if err != nil {
			if processExists(pid) {
				return nil, err
			}
			continue
		}
		if current != created {
			continue
		}
		states, err := proc.Status()
		if err != nil {
			if processExists(pid) {
				return nil, err
			}
			continue
		}
		zombie := false
		for _, state := range states {
			if state == process.Zombie {
				zombie = true
				break
			}
		}
		if !zombie {
			running = append(running, pid)
		}
	}
	return running, nil
}

// killPIDs rechecks captured identities before signaling individual survivors.
func killPIDs(identities map[int]int64, sig syscall.Signal) error {
	pids, err := filterRunningPIDs(identities)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			if processExists(pid) {
				return err
			}
			continue
		}
		created, err := proc.CreateTime()
		if err != nil {
			if processExists(pid) {
				return err
			}
			continue
		}
		if created == identities[pid] {
			syscall.Kill(pid, sig)
		}
	}
	return nil
}
