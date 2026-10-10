package daemon

import (
	"sync"
	"testing"
	"time"
)

func TestStopAcknowledgesBeforeProcessVerification(t *testing.T) {
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(t.TempDir(), nil, executor, nil)
	job, _, err := jm.AddJob([]string{"echo"}, "/workdir", "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	executor.LastHandle().SetTerminate(func(bool, <-chan struct{}) error { close(entered); <-release; return nil })
	d := &Daemon{jobManager: jm}
	result := d.handleStopRequest(&Request{Payload: map[string]any{"job_id": job.ID}})
	if !result.Success {
		t.Fatalf("stop request: %s", result.Error)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("verification did not start")
	}
	jobs := d.handleList(&Request{Payload: map[string]any{"workdir": "/workdir"}}).Data["jobs"].([]JobResponse)
	if len(jobs) != 1 || jobs[0].Status != "stopping" {
		t.Fatalf("expected stopping: %+v", jobs)
	}
	executor.LastHandle().Stop()
	// The root exiting cannot announce completion while verification is blocked.
	time.Sleep(10 * time.Millisecond)
	jobs = jm.ListJobResponses("/workdir")
	if jobs[0].Status != "stopping" {
		t.Fatalf("root exit prematurely stopped job: %+v", jobs)
	}
	unblock()
	deadline := time.After(time.Second)
	for jm.ListJobResponses("/workdir")[0].Status != "stopped" {
		select {
		case <-deadline:
			t.Fatal("stop did not complete")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
