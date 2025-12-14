package daemon

import (
	"testing"
	"time"
)

func TestDaemon_handlePing(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{Type: RequestTypePing}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Error("expected success")
	}
	if resp.Data["message"] != "pong" {
		t.Errorf("expected pong, got %v", resp.Data["message"])
	}
}

func TestDaemon_handleList_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{Type: RequestTypeList, Payload: map[string]interface{}{}}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Error("expected success")
	}

	jobs, ok := resp.Data["jobs"]
	if !ok {
		t.Fatal("expected jobs in response")
	}

	if jobs != nil && len(jobs.([]JobResponse)) != 0 {
		t.Error("expected empty jobs list")
	}
}

func TestDaemon_handleList_WithJobs(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	// Add a job
	jm.AddJob([]string{"echo", "hello"}, "/workdir", nil)

	d := &Daemon{jobManager: jm}
	req := &Request{Type: RequestTypeList, Payload: map[string]interface{}{}}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Error("expected success")
	}

	jobs := resp.Data["jobs"].([]JobResponse)
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestDaemon_handleAdd(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeAdd,
		Payload: map[string]interface{}{
			"command": []interface{}{"echo", "hello"},
			"workdir": "/workdir",
		},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	job := resp.Data["job"].(JobResponse)
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.Status != "running" {
		t.Errorf("expected running, got %s", job.Status)
	}
}

func TestDaemon_handleAdd_MissingCommand(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type:    RequestTypeAdd,
		Payload: map[string]interface{}{},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error")
	}
	if resp.Error == "" {
		t.Error("expected error message")
	}
}

func TestDaemon_handleAdd_MissingWorkdir(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeAdd,
		Payload: map[string]interface{}{
			"command": []interface{}{"echo"},
		},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error")
	}
}

func TestDaemon_handleGetJob(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	job, _ := jm.AddJob([]string{"echo"}, "/workdir", nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeGetJob,
		Payload: map[string]interface{}{
			"job_id": job.ID,
		},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	respJob := resp.Data["job"].(JobResponse)
	if respJob.ID != job.ID {
		t.Error("job ID mismatch")
	}
}

func TestDaemon_handleGetJob_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeGetJob,
		Payload: map[string]interface{}{
			"job_id": "nonexistent",
		},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error for non-existent job")
	}
}

func TestDaemon_handleStop(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	job, _ := jm.AddJob([]string{"echo"}, "/workdir", nil)

	// Stop the fake process so Stop() can succeed
	executor.LastHandle().Stop()
	time.Sleep(10 * time.Millisecond)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeStop,
		Payload: map[string]interface{}{
			"job_id": job.ID,
			"force":  false,
		},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestDaemon_handleStop_MissingJobID(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type:    RequestTypeStop,
		Payload: map[string]interface{}{},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error for missing job_id")
	}
}

func TestDaemon_handleRequest_UnknownType(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{Type: "unknown"}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error for unknown request type")
	}
}

func TestDaemon_handleVersion(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type:    RequestTypeVersion,
		Payload: map[string]interface{}{},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Check version is present
	if _, ok := resp.Data["version"]; !ok {
		t.Error("expected version in response")
	}

	// Check running_jobs is present
	runningJobs, ok := resp.Data["running_jobs"]
	if !ok {
		t.Error("expected running_jobs in response")
	}
	if runningJobs != 0 {
		t.Errorf("expected 0 running jobs, got %v", runningJobs)
	}
}

func TestDaemon_handleVersion_WithRunningJobs(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	// Add running jobs
	jm.AddJob([]string{"echo", "1"}, "/workdir", nil)
	jm.AddJob([]string{"echo", "2"}, "/workdir", nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type:    RequestTypeVersion,
		Payload: map[string]interface{}{},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	runningJobs := resp.Data["running_jobs"]
	if runningJobs != 2 {
		t.Errorf("expected 2 running jobs, got %v", runningJobs)
	}
}
