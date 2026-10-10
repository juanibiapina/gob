package daemon

import (
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

type rootOwner interface {
	whileOwned(fn func()) bool
}

type reapOwner struct {
	mu     sync.Mutex
	reaped bool
}

func (o *reapOwner) whileOwned(fn func()) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.reaped {
		return false
	}
	fn()
	return true
}

func (o *reapOwner) markReaped() {
	o.mu.Lock()
	o.reaped = true
	o.mu.Unlock()
}

type identityOwner struct {
	pid       int
	startedAt time.Time
}

func (o identityOwner) whileOwned(fn func()) bool {
	created, err := processCreateTime(o.pid)
	if err != nil {
		return false
	}
	diff := time.UnixMilli(created).Sub(o.startedAt)
	if diff < 0 {
		diff = -diff
	}
	if diff > 2*time.Second {
		return false
	}
	fn()
	return true
}

type processTree struct {
	pid   int
	owner rootOwner
	mu    sync.Mutex
	known map[int]int64
}

func newProcessTree(pid int, owner rootOwner) *processTree {
	return &processTree{pid: pid, owner: owner, known: make(map[int]int64)}
}

func adoptProcess(pid int, startedAt time.Time) *processTree {
	return newProcessTree(pid, identityOwner{pid: pid, startedAt: startedAt})
}

func (t *processTree) signal(sig syscall.Signal) error {
	var err error
	t.owner.whileOwned(func() {
		if e := syscall.Kill(-t.pid, sig); e != nil && e != syscall.ESRCH {
			err = e
		}
	})
	return err
}

func (t *processTree) observe() error {
	if !t.owner.whileOwned(func() {}) {
		return nil
	}
	pids, err := getProcessTreePIDs(t.pid)
	if err != nil {
		return fmt.Errorf("failed to inspect process tree: %w", err)
	}
	observed, err := captureProcessIdentities(pids)
	if err != nil {
		return fmt.Errorf("failed to identify process tree: %w", err)
	}
	t.owner.whileOwned(func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		for pid, created := range observed {
			t.known[pid] = created
		}
	})
	return nil
}

func (t *processTree) knownIdentities() map[int]int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	identities := make(map[int]int64, len(t.known))
	for pid, created := range t.known {
		identities[pid] = created
	}
	return identities
}

func (t *processTree) terminate(force bool, escalate <-chan struct{}) error {
	if err := t.observe(); err != nil {
		return err
	}
	identities := t.knownIdentities()
	live := func() ([]int, error) { return filterRunningPIDs(identities) }
	send := func(sig syscall.Signal) error {
		if err := t.signal(sig); err != nil {
			return err
		}
		return killPIDs(identities, sig)
	}
	if force {
		if err := send(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process tree: %w", err)
		}
	} else {
		if err := send(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to stop process tree: %w", err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		survivors, err := live()
		if err != nil {
			return fmt.Errorf("failed to verify process tree: %w", err)
		}
		if len(survivors) == 0 {
			return nil
		}
		select {
		case <-escalate:
			deadline = time.Now()
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !force {
		if err := send(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process tree: %w", err)
		}
		deadline = time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			survivors, err := live()
			if err != nil {
				return fmt.Errorf("failed to verify process tree: %w", err)
			}
			if len(survivors) == 0 {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	survivors, err := live()
	if err != nil {
		return fmt.Errorf("failed to verify process tree: %w", err)
	}
	if len(survivors) > 0 {
		return fmt.Errorf("process tree has %d surviving processes after SIGKILL: %v", len(survivors), survivors)
	}
	return nil
}

func processCreateTime(pid int) (int64, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0, err
	}
	return proc.CreateTime()
}

func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

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
