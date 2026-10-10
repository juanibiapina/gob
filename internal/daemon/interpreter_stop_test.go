package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func writeShebangScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "job-script")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForTree(t *testing.T, root int, size int) []int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pids, err := getProcessTreePIDs(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(pids) >= size {
			return pids
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("process tree of %d did not reach %d processes", root, size)
	return nil
}

func processGone(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return true
	}
	states, err := proc.Status()
	if err != nil {
		return true
	}
	for _, state := range states {
		if state == process.Zombie {
			return true
		}
	}
	return false
}

func assertGone(t *testing.T, pids []int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for _, pid := range pids {
		for !processGone(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("process %d survived", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestStopTerminatesInterpreterJobs(t *testing.T) {
	cases := []struct {
		name string
		body string
		size int
	}{
		{"shebang script with child", "sleep 30 & wait", 2},
		{"exec wrapper", "exec sleep 30", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jm := NewJobManager(t.TempDir(), nil, nil)
			job, _, err := jm.AddJob([]string{writeShebangScript(t, tc.body)}, t.TempDir(), "", false, os.Environ())
			if err != nil {
				t.Fatal(err)
			}
			root := jm.GetCurrentRun(job.ID).PID
			defer syscall.Kill(-root, syscall.SIGKILL)
			tree := waitForTree(t, root, tc.size)
			if tc.size == 1 {
				time.Sleep(200 * time.Millisecond)
			}

			if err := jm.StopJob(job.ID, false); err != nil {
				t.Fatalf("stop: %v", err)
			}
			if got := jm.ListJobResponses("")[0].Status; got != "stopped" {
				t.Fatalf("status = %s", got)
			}
			assertGone(t, tree)
		})
	}
}

func TestAdoptedProcessWithDifferentStartTimeIsNotSignaled(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); cmd.Wait() }()

	if err := adoptProcess(cmd.Process.Pid, time.Now().Add(-time.Hour)).terminate(true, nil); err != nil {
		t.Fatal(err)
	}
	if processGone(cmd.Process.Pid) {
		t.Fatal("signaled a process with a different start time")
	}
}

func TestCrashRecoveryTerminatesInterpreterOrphan(t *testing.T) {
	db, err := OpenDatabase(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)

	cmd := exec.Command(writeShebangScript(t, "sleep 30 & wait"))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now()
	root := cmd.Process.Pid
	defer syscall.Kill(-root, syscall.SIGKILL)
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	tree := waitForTree(t, root, 2)

	job := &Job{ID: "orp", Command: []string{cmd.Path}, Workdir: t.TempDir(), CreatedAt: startedAt}
	if err := store.InsertJob(job); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertRun(&Run{ID: "orp-1", JobID: job.ID, PID: root, Status: "running", StartedAt: startedAt}); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{store: store}
	if err := d.recoverFromCrash(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("orphan root survived crash recovery")
	}
	assertGone(t, tree)
}
