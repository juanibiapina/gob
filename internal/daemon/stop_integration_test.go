package daemon

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestForceUpgradesGracefulStop(t *testing.T) {
	jm := NewJobManager(t.TempDir(), nil, nil)
	job, _, err := jm.AddJob([]string{"/bin/sh", "-c", "trap '' TERM; while :; do sleep 1; done"}, t.TempDir(), "", false, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	pid := jm.GetCurrentRun(job.ID).PID
	defer syscall.Kill(-pid, syscall.SIGKILL)
	_, first, err := jm.RequestStop(job.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := jm.ListJobResponses("")[0].Status; got != "stopping" {
		t.Fatalf("status = %s", got)
	}
	_, forced, err := jm.RequestStop(job.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if first != forced {
		t.Fatal("force created a second operation")
	}
	select {
	case <-forced:
		if got := jm.ListJobResponses("")[0].Status; got != "stopped" {
			t.Fatalf("verified status = %s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("force did not finish pending stop")
	}
}
