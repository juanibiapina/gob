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
	jm.AddJob([]string{"echo", "hello"}, "/workdir", "", nil)

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

func TestDaemon_handleAdd_ReturnsAction(t *testing.T) {
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

	action, ok := resp.Data["action"].(string)
	if !ok {
		t.Fatal("expected action in response")
	}
	if action != "created" {
		t.Errorf("expected action 'created', got %s", action)
	}
}

func TestDaemon_handleAdd_AlreadyRunning(t *testing.T) {
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

	// First add
	resp := d.handleRequest(req)
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	firstJob := resp.Data["job"].(JobResponse)

	// Second add - same command while running
	resp = d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success for already running job, got error: %s", resp.Error)
	}

	action, ok := resp.Data["action"].(string)
	if !ok {
		t.Fatal("expected action in response")
	}
	if action != "already_running" {
		t.Errorf("expected action 'already_running', got %s", action)
	}

	secondJob := resp.Data["job"].(JobResponse)
	if secondJob.ID != firstJob.ID {
		t.Errorf("expected same job ID, got %s and %s", firstJob.ID, secondJob.ID)
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

	job, _, _ := jm.AddJob([]string{"echo"}, "/workdir", "", nil)

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

	job, _, _ := jm.AddJob([]string{"echo"}, "/workdir", "", nil)

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
	jm.AddJob([]string{"echo", "1"}, "/workdir", "", nil)
	jm.AddJob([]string{"echo", "2"}, "/workdir", "", nil)

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

func TestDaemon_handlePorts(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	job, _, _ := jm.AddJob([]string{"echo"}, "/workdir", "", nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypePorts,
		Payload: map[string]interface{}{
			"job_id": job.ID,
		},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Ports will be empty since FakeProcessExecutor doesn't have real PIDs
	ports := resp.Data["ports"]
	if ports == nil {
		t.Error("expected ports in response")
	}
}

func TestDaemon_handlePorts_StoppedJob(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	job, _, _ := jm.AddJob([]string{"echo"}, "/workdir", "", nil)

	// Stop the job
	executor.LastHandle().Stop()
	time.Sleep(10 * time.Millisecond)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypePorts,
		Payload: map[string]interface{}{
			"job_id": job.ID,
		},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// For stopped jobs, should get a JobPorts with status="stopped"
	portsData := resp.Data["ports"].(*JobPorts)
	if portsData.Status != "stopped" {
		t.Errorf("expected status 'stopped', got %s", portsData.Status)
	}
}

func TestDaemon_handlePorts_AllJobs(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	// Add multiple jobs
	jm.AddJob([]string{"echo", "1"}, "/workdir", "", nil)
	jm.AddJob([]string{"echo", "2"}, "/workdir", "", nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type:    RequestTypePorts,
		Payload: map[string]interface{}{},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Should return a slice of ports for all running jobs
	portsData := resp.Data["ports"]
	if portsData == nil {
		t.Error("expected ports in response")
	}
}

func TestDaemon_handleRemoveRun(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	// Add a job (which creates a run)
	job, _, _ := jm.AddJob([]string{"echo"}, "/workdir", "", nil)

	// Get the run ID
	runs, _ := jm.ListRunsForJob(job.ID)
	runID := runs[0].ID

	// Stop the process first
	executor.LastHandle().Stop()
	time.Sleep(10 * time.Millisecond)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeRemoveRun,
		Payload: map[string]interface{}{
			"run_id": runID,
		},
	}

	resp := d.handleRequest(req)

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if resp.Data["run_id"] != runID {
		t.Errorf("expected run_id %s in response, got %v", runID, resp.Data["run_id"])
	}

	// Verify run is actually removed
	runs, _ = jm.ListRunsForJob(job.ID)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs after removal, got %d", len(runs))
	}
}

func TestDaemon_handleRemoveRun_MissingRunID(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type:    RequestTypeRemoveRun,
		Payload: map[string]interface{}{},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error for missing run_id")
	}
}

func TestDaemon_handleRemoveRun_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeRemoveRun,
		Payload: map[string]interface{}{
			"run_id": "nonexistent-1",
		},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error for nonexistent run")
	}
}

func TestDaemon_handleRemoveRun_Running(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewFakeProcessExecutor()
	jm := NewJobManagerWithExecutor(tmpDir, nil, executor, nil)

	// Add a job (which creates a running run)
	job, _, _ := jm.AddJob([]string{"echo"}, "/workdir", "", nil)

	// Get the run ID while it's still running
	runs, _ := jm.ListRunsForJob(job.ID)
	runID := runs[0].ID

	d := &Daemon{jobManager: jm}
	req := &Request{
		Type: RequestTypeRemoveRun,
		Payload: map[string]interface{}{
			"run_id": runID,
		},
	}

	resp := d.handleRequest(req)

	if resp.Success {
		t.Error("expected error for running run")
	}
}
