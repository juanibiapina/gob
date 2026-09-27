package daemon

import (
	"sync"
	"testing"
	"time"
)

func TestListDuringPortScan(t *testing.T) {
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(t.TempDir(), nil, executor, nil)
	job, _, err := jm.AddJob([]string{"echo"}, "/workdir", "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := jm.GetCurrentRun(job.ID)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	jm.scanPorts = func(pid int) ([]PortInfo, error) {
		close(entered)
		<-release
		return []PortInfo{{PID: pid, Port: 8080, Protocol: "tcp"}}, nil
	}
	scanned := make(chan struct{})
	go func() { jm.refreshPorts(job.ID, run.ID); close(scanned) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("scan did not start")
	}

	listed := make(chan *Response, 1)
	d := &Daemon{jobManager: jm}
	go func() { listed <- d.handleList(&Request{Payload: map[string]any{"workdir": "/workdir"}}) }()
	select {
	case resp := <-listed:
		jobs := resp.Data["jobs"].([]JobResponse)
		if len(jobs) != 1 || jobs[0].Status != "running" {
			t.Fatalf("unexpected list: %+v", jobs)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("list blocked by port scan")
	}

	// A completed scan must not restore ports on a run that stopped in the meantime.
	executor.LastHandle().Stop()
	deadline := time.After(time.Second)
	for jm.HasRunningJobs() {
		select {
		case <-deadline:
			t.Fatal("run did not stop")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	unblock()
	<-scanned
	if got := jm.GetLatestRun(job.ID).Ports; len(got) != 0 {
		t.Fatalf("stale ports after stop: %+v", got)
	}
}
