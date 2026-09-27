package daemon

import (
	"testing"
	"time"
)

func TestListDuringStateWrite(t *testing.T) {
	jm := NewJobManagerWithExecutor(t.TempDir(), nil, NewFakeProcessExecutor(), nil)
	job, _, err := jm.AddJob([]string{"echo"}, "/workdir", "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	jm.mu.Lock()
	defer jm.mu.Unlock()
	done := make(chan []JobResponse, 1)
	go func() { done <- jm.ListJobResponses("/workdir") }()
	select {
	case jobs := <-done:
		if len(jobs) != 1 || jobs[0].ID != job.ID {
			t.Fatalf("unexpected list: %+v", jobs)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("list waited on state writer")
	}
}
