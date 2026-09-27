package daemon

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestStopRejectsMismatchedProcessIdentity(t *testing.T) {
	jm := NewJobManager(t.TempDir(), nil, nil)
	job, _, err := jm.AddJob([]string{"/bin/sleep", "30"}, t.TempDir(), "", false, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	run := jm.GetCurrentRun(job.ID)
	defer syscall.Kill(-run.PID, syscall.SIGKILL)
	jm.mu.Lock()
	original := run.StartedAt
	run.StartedAt = original.Add(-time.Hour)
	jm.mu.Unlock()
	if err := jm.StopJob(job.ID, false); err == nil {
		t.Fatal("accepted mismatched PID identity")
	}
	if syscall.Kill(run.PID, 0) != nil {
		t.Fatal("signaled the unrelated process")
	}
	jm.mu.Lock()
	run.StartedAt = original
	jm.mu.Unlock()
	if err := jm.StopJob(job.ID, true); err != nil {
		t.Fatalf("verified retry: %v", err)
	}
}
