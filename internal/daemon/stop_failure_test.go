package daemon

import (
	"errors"
	"testing"
	"time"
)

func TestStopScanFailureCanBeRetried(t *testing.T) {
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(t.TempDir(), nil, executor, nil)
	job, _, err := jm.AddJob([]string{"echo"}, "/workdir", "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	attempts := 0
	executor.LastHandle().SetTerminate(func(bool, <-chan struct{}) error {
		attempts++
		if attempts == 1 {
			return errors.New("process table unavailable")
		}
		return nil
	})
	if err := jm.StopJob(job.ID, false); err == nil {
		t.Fatal("scan failure was reported as success")
	}
	if got := jm.ListJobResponses("")[0].Status; got != "stopping" {
		t.Fatalf("after failure = %s", got)
	}
	executor.LastHandle().Stop()
	done := make(chan error, 1)
	go func() { done <- jm.StopJob(job.ID, true) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry did not finish")
	}
	if got := jm.ListJobResponses("")[0].Status; got != "stopped" {
		t.Fatalf("after retry = %s", got)
	}
}
