package daemon

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Job represents a managed background job (a command that can be run repeatedly)
type Job struct {
	ID               string  `json:"id"`                // user-facing identifier (e.g., "abc")
	Command          []string `json:"command"`          // the command + args
	CommandSignature string  `json:"command_signature"` // hash for lookups
	Workdir          string  `json:"workdir"`           // directory scope
	CurrentRunID     *string `json:"current_run_id"`    // nil if not running, points to active run
	NextRunSeq       int     `json:"next_run_seq"`      // counter for internal run IDs
	CreatedAt        time.Time `json:"created_at"`

	// Cached statistics (updated on run completion)
	RunCount        int   `json:"run_count"`
	SuccessCount    int   `json:"success_count"`
	TotalDurationMs int64 `json:"total_duration_ms"`
	MinDurationMs   int64 `json:"min_duration_ms"`
	MaxDurationMs   int64 `json:"max_duration_ms"`
}

// IsRunning checks if the job has a currently running process
func (j *Job) IsRunning() bool {
	return j.CurrentRunID != nil
}

// Status returns "running" or "stopped" based on whether there's an active run
func (j *Job) Status() string {
	if j.IsRunning() {
		return "running"
	}
	return "stopped"
}

// AverageDurationMs returns the average duration in milliseconds, or 0 if no runs
func (j *Job) AverageDurationMs() int64 {
	if j.RunCount == 0 {
		return 0
	}
	return j.TotalDurationMs / int64(j.RunCount)
}

// SuccessRate returns the success rate as a percentage (0-100)
func (j *Job) SuccessRate() float64 {
	if j.RunCount == 0 {
		return 0
	}
	return float64(j.SuccessCount) / float64(j.RunCount) * 100
}

// ComputeCommandSignature creates a hash from command array for lookups
func ComputeCommandSignature(command []string) string {
	// Join with null byte separator (can't appear in command args)
	joined := strings.Join(command, "\x00")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])
}

// JobManager manages all jobs and runs in the daemon
type JobManager struct {
	jobs       map[string]*Job        // keyed by job ID
	runs       map[string]*Run        // keyed by run ID
	jobIndex   map[string]string      // signature+workdir -> job ID for quick lookup
	mu         sync.RWMutex
	runtimeDir string
	onEvent    func(Event)
	executor   ProcessExecutor
}

// NewJobManager creates a new job manager
func NewJobManager(runtimeDir string, onEvent func(Event)) *JobManager {
	return &JobManager{
		jobs:       make(map[string]*Job),
		runs:       make(map[string]*Run),
		jobIndex:   make(map[string]string),
		runtimeDir: runtimeDir,
		onEvent:    onEvent,
		executor:   &RealProcessExecutor{},
	}
}

// NewJobManagerWithExecutor creates a new job manager with a custom executor (for testing)
func NewJobManagerWithExecutor(runtimeDir string, onEvent func(Event), executor ProcessExecutor) *JobManager {
	return &JobManager{
		jobs:       make(map[string]*Job),
		runs:       make(map[string]*Run),
		jobIndex:   make(map[string]string),
		runtimeDir: runtimeDir,
		onEvent:    onEvent,
		executor:   executor,
	}
}

// JobCount returns the number of jobs
func (jm *JobManager) JobCount() int {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return len(jm.jobs)
}

// makeJobIndexKey creates the lookup key for finding jobs by command+workdir
func makeJobIndexKey(signature, workdir string) string {
	return signature + "\x00" + workdir
}

// emitEvent sends an event if a callback is registered
func (jm *JobManager) emitEvent(event Event) {
	if jm.onEvent != nil {
		jm.onEvent(event)
	}
}

// jobToResponse converts a Job to JobResponse (for backward compatibility)
func (jm *JobManager) jobToResponse(job *Job) JobResponse {
	resp := JobResponse{
		ID:        job.ID,
		Status:    job.Status(),
		Command:   job.Command,
		Workdir:   job.Workdir,
		CreatedAt: job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// If there's a current run, include its details
	if job.CurrentRunID != nil {
		if run, ok := jm.runs[*job.CurrentRunID]; ok {
			resp.PID = run.PID
			resp.StartedAt = run.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			resp.StdoutPath = run.StdoutPath
			resp.StderrPath = run.StderrPath
			resp.ExitCode = run.ExitCode
			if run.StoppedAt != nil {
				resp.StoppedAt = run.StoppedAt.Format("2006-01-02T15:04:05Z07:00")
			}
		}
	} else {
		// Use latest run for stopped jobs
		latestRun := jm.getLatestRunForJobLocked(job.ID)
		if latestRun != nil {
			resp.PID = latestRun.PID
			resp.StartedAt = latestRun.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			resp.StdoutPath = latestRun.StdoutPath
			resp.StderrPath = latestRun.StderrPath
			resp.ExitCode = latestRun.ExitCode
			if latestRun.StoppedAt != nil {
				resp.StoppedAt = latestRun.StoppedAt.Format("2006-01-02T15:04:05Z07:00")
			}
		}
	}

	return resp
}

// getLatestRunForJobLocked returns the most recent run for a job (caller must hold lock)
func (jm *JobManager) getLatestRunForJobLocked(jobID string) *Run {
	var latest *Run
	for _, run := range jm.runs {
		if run.JobID == jobID {
			if latest == nil || run.StartedAt.After(latest.StartedAt) {
				latest = run
			}
		}
	}
	return latest
}

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const jobIDLength = 3

// generateJobID creates a unique 3-character job ID using cryptographic randomness
// It checks against existing IDs to avoid collisions
func generateJobID(existingIDs map[string]bool) string {
	for {
		bytes := make([]byte, jobIDLength)
		if _, err := rand.Read(bytes); err != nil {
			// Fallback should never happen, but use timestamp if crypto fails
			n := time.Now().UnixNano()
			for i := 0; i < jobIDLength; i++ {
				bytes[i] = byte(n % 256)
				n /= 256
			}
		}
		result := make([]byte, jobIDLength)
		for i := 0; i < jobIDLength; i++ {
			result[i] = base62Chars[int(bytes[i])%62]
		}
		id := string(result)
		if !existingIDs[id] {
			return id
		}
	}
}

// AddJob finds or creates a job for the command, then starts a new run
func (jm *JobManager) AddJob(command []string, workdir string) (*Job, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	jm.mu.Lock()
	defer jm.mu.Unlock()

	signature := ComputeCommandSignature(command)
	indexKey := makeJobIndexKey(signature, workdir)

	// Check if job already exists for this command+workdir
	if existingJobID, ok := jm.jobIndex[indexKey]; ok {
		job := jm.jobs[existingJobID]
		if job.IsRunning() {
			return nil, fmt.Errorf("job %s is already running", job.ID)
		}
		// Start a new run for existing job
		run, err := jm.startRunLocked(job)
		if err != nil {
			return nil, err
		}

		// Emit job started event (reusing existing job)
		jm.emitEvent(Event{
			Type:     EventTypeJobStarted,
			JobID:    job.ID,
			Job:      jm.jobToResponse(job),
			JobCount: len(jm.jobs),
		})

		_ = run // run is stored, job's CurrentRunID is updated
		return job, nil
	}

	// Create new job
	existingIDs := make(map[string]bool)
	for id := range jm.jobs {
		existingIDs[id] = true
	}
	jobID := generateJobID(existingIDs)

	now := time.Now()
	job := &Job{
		ID:               jobID,
		Command:          command,
		CommandSignature: signature,
		Workdir:          workdir,
		NextRunSeq:       1,
		CreatedAt:        now,
	}

	jm.jobs[jobID] = job
	jm.jobIndex[indexKey] = jobID

	// Start first run
	run, err := jm.startRunLocked(job)
	if err != nil {
		// Clean up job if run failed to start
		delete(jm.jobs, jobID)
		delete(jm.jobIndex, indexKey)
		return nil, err
	}

	// Emit job added event
	jm.emitEvent(Event{
		Type:     EventTypeJobAdded,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
		JobCount: len(jm.jobs),
	})

	_ = run // run is stored
	return job, nil
}

// startRunLocked creates and starts a new run for a job (caller must hold lock)
func (jm *JobManager) startRunLocked(job *Job) (*Run, error) {
	runID := fmt.Sprintf("%s-%d", job.ID, job.NextRunSeq)
	job.NextRunSeq++

	// Create log file paths
	stdoutPath := fmt.Sprintf("%s/%s.stdout.log", jm.runtimeDir, runID)
	stderrPath := fmt.Sprintf("%s/%s.stderr.log", jm.runtimeDir, runID)

	// Start the process
	process, err := jm.executor.Start(job.Command, job.Workdir, stdoutPath, stderrPath)
	if err != nil {
		job.NextRunSeq-- // Rollback sequence number
		return nil, err
	}

	now := time.Now()
	run := &Run{
		ID:         runID,
		JobID:      job.ID,
		PID:        process.Pid(),
		Status:     "running",
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
		StartedAt:  now,
		process:    process,
	}

	jm.runs[runID] = run
	job.CurrentRunID = &runID

	// Start goroutine to wait for process exit
	go jm.waitForProcessExit(job, run)

	return run, nil
}

// waitForProcessExit waits for a run's process to exit and updates state
func (jm *JobManager) waitForProcessExit(job *Job, run *Run) {
	if run.process == nil {
		return
	}

	// Wait for process to exit (this blocks until the process terminates)
	err := run.process.Wait()

	jm.mu.Lock()

	// Record stop time
	now := time.Now()
	run.StoppedAt = &now
	run.Status = "stopped"

	// Extract exit code from the error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				// Only get exit code if process exited normally (not killed by signal)
				if status.Exited() {
					code := status.ExitStatus()
					run.ExitCode = &code
				}
				// If killed by signal, leave ExitCode as nil
			}
		}
		// If we couldn't extract exit code, leave it as nil (killed/unknown)
	} else {
		// No error means exit code 0
		code := 0
		run.ExitCode = &code
	}

	// Clear job's current run pointer
	job.CurrentRunID = nil

	// Update job statistics
	durationMs := run.StoppedAt.Sub(run.StartedAt).Milliseconds()
	job.RunCount++
	job.TotalDurationMs += durationMs

	if run.ExitCode != nil && *run.ExitCode == 0 {
		job.SuccessCount++
	}

	if job.RunCount == 1 {
		job.MinDurationMs = durationMs
		job.MaxDurationMs = durationMs
	} else {
		if durationMs < job.MinDurationMs {
			job.MinDurationMs = durationMs
		}
		if durationMs > job.MaxDurationMs {
			job.MaxDurationMs = durationMs
		}
	}

	jobCount := len(jm.jobs)
	jobResp := jm.jobToResponse(job)

	jm.mu.Unlock()

	// Emit stopped event
	jm.emitEvent(Event{
		Type:     EventTypeJobStopped,
		JobID:    job.ID,
		Job:      jobResp,
		JobCount: jobCount,
	})
}

// GetJob returns a job by ID
func (jm *JobManager) GetJob(jobID string) (*Job, error) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return job, nil
}

// GetCurrentRun returns the current run for a job, or nil if not running
func (jm *JobManager) GetCurrentRun(jobID string) *Run {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	job, ok := jm.jobs[jobID]
	if !ok || job.CurrentRunID == nil {
		return nil
	}
	return jm.runs[*job.CurrentRunID]
}

// GetLatestRun returns the most recent run for a job (running or completed)
func (jm *JobManager) GetLatestRun(jobID string) *Run {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	return jm.getLatestRunForJobLocked(jobID)
}

// ListJobs returns all jobs, optionally filtered by workdir
func (jm *JobManager) ListJobs(workdirFilter string) []*Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	var jobs []*Job
	for _, job := range jm.jobs {
		if workdirFilter != "" && job.Workdir != workdirFilter {
			continue
		}
		jobs = append(jobs, job)
	}

	// Sort by CreatedAt, newest first
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})

	return jobs
}

// StopJob stops a running job
func (jm *JobManager) StopJob(jobID string, force bool) error {
	jm.mu.RLock()
	job, ok := jm.jobs[jobID]
	if !ok {
		jm.mu.RUnlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.CurrentRunID == nil {
		jm.mu.RUnlock()
		return nil // Already stopped
	}

	run := jm.runs[*job.CurrentRunID]
	pid := run.PID
	jm.mu.RUnlock()

	if force {
		// Send SIGKILL immediately
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	} else {
		// Send SIGTERM for graceful shutdown
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to stop process: %w", err)
		}
	}

	// Wait for process to terminate (event will be emitted by waitForProcessExit)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		jm.mu.RLock()
		stillRunning := job.CurrentRunID != nil
		jm.mu.RUnlock()
		if !stillRunning {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// If still running after timeout and we used SIGTERM, escalate to SIGKILL
	if !force {
		jm.mu.RLock()
		stillRunning := job.CurrentRunID != nil
		jm.mu.RUnlock()
		if stillRunning {
			if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				return fmt.Errorf("failed to kill process: %w", err)
			}

			// Wait again for SIGKILL
			deadline = time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				jm.mu.RLock()
				stillRunning := job.CurrentRunID != nil
				jm.mu.RUnlock()
				if !stillRunning {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	jm.mu.RLock()
	stillRunning := job.CurrentRunID != nil
	jm.mu.RUnlock()
	if stillRunning {
		return fmt.Errorf("process %d still running after SIGKILL", pid)
	}

	return nil
}

// StartJob starts a new run for a stopped job
func (jm *JobManager) StartJob(jobID string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Check if already running
	if job.IsRunning() {
		return fmt.Errorf("job %s is already running (use 'gob restart' to restart a running job)", jobID)
	}

	// Start new run
	_, err := jm.startRunLocked(job)
	if err != nil {
		return err
	}

	// Emit started event
	jm.emitEvent(Event{
		Type:     EventTypeJobStarted,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
		JobCount: len(jm.jobs),
	})

	return nil
}

// RestartJob stops (if running) and starts a new run
func (jm *JobManager) RestartJob(jobID string) error {
	jm.mu.Lock()

	job, ok := jm.jobs[jobID]
	if !ok {
		jm.mu.Unlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Stop if running
	if job.CurrentRunID != nil {
		run := jm.runs[*job.CurrentRunID]
		pid := run.PID
		jm.mu.Unlock()

		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to stop process: %w", err)
		}

		// Wait for termination
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			jm.mu.RLock()
			stillRunning := job.CurrentRunID != nil
			jm.mu.RUnlock()
			if !stillRunning {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Escalate to SIGKILL if needed
		jm.mu.RLock()
		stillRunning := job.CurrentRunID != nil
		jm.mu.RUnlock()
		if stillRunning {
			if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				return fmt.Errorf("failed to kill process: %w", err)
			}

			deadline = time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				jm.mu.RLock()
				stillRunning := job.CurrentRunID != nil
				jm.mu.RUnlock()
				if !stillRunning {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		jm.mu.Lock()
	}

	// Start new run
	_, err := jm.startRunLocked(job)
	if err != nil {
		jm.mu.Unlock()
		return err
	}

	// Emit started event
	jm.emitEvent(Event{
		Type:     EventTypeJobStarted,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
		JobCount: len(jm.jobs),
	})

	jm.mu.Unlock()
	return nil
}

// RemoveJob removes a stopped job and all its runs
func (jm *JobManager) RemoveJob(jobID string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.IsRunning() {
		return fmt.Errorf("cannot remove running job: %s (use 'stop' first)", jobID)
	}

	// Capture job info for event before deletion
	jobResp := jm.jobToResponse(job)

	// Remove all runs for this job and their log files
	for runID, run := range jm.runs {
		if run.JobID == jobID {
			os.Remove(run.StdoutPath)
			os.Remove(run.StderrPath)
			delete(jm.runs, runID)
		}
	}

	// Remove from index
	indexKey := makeJobIndexKey(job.CommandSignature, job.Workdir)
	delete(jm.jobIndex, indexKey)

	delete(jm.jobs, jobID)

	// Emit removed event
	jm.emitEvent(Event{
		Type:     EventTypeJobRemoved,
		JobID:    jobID,
		Job:      jobResp,
		JobCount: len(jm.jobs),
	})

	return nil
}

// Nuke stops all jobs and removes all data
func (jm *JobManager) Nuke(workdirFilter string) (stopped, logsDeleted, cleaned int) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	// Collect jobs to nuke
	var jobsToNuke []*Job
	var runningRuns []*Run
	for _, job := range jm.jobs {
		if workdirFilter != "" && job.Workdir != workdirFilter {
			continue
		}
		jobsToNuke = append(jobsToNuke, job)
		if job.CurrentRunID != nil {
			if run, ok := jm.runs[*job.CurrentRunID]; ok {
				runningRuns = append(runningRuns, run)
			}
		}
	}

	// Stop running jobs with SIGTERM
	for _, run := range runningRuns {
		syscall.Kill(-run.PID, syscall.SIGTERM)
	}

	// Wait for graceful termination
	if len(runningRuns) > 0 {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			allStopped := true
			for _, run := range runningRuns {
				if run.IsRunning() {
					allStopped = false
					break
				}
			}
			if allStopped {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// SIGKILL any remaining
		for _, run := range runningRuns {
			if run.IsRunning() {
				syscall.Kill(-run.PID, syscall.SIGKILL)
			}
		}

		// Wait for SIGKILL to take effect
		time.Sleep(100 * time.Millisecond)
	}

	stopped = len(runningRuns)

	// Delete log files and remove runs for nuked jobs
	for _, job := range jobsToNuke {
		for runID, run := range jm.runs {
			if run.JobID == job.ID {
				if err := os.Remove(run.StdoutPath); err == nil {
					logsDeleted++
				}
				if err := os.Remove(run.StderrPath); err == nil {
					logsDeleted++
				}
				delete(jm.runs, runID)
			}
		}

		// Remove job from index
		indexKey := makeJobIndexKey(job.CommandSignature, job.Workdir)
		delete(jm.jobIndex, indexKey)

		// Capture job info for event
		jobResp := jm.jobToResponse(job)
		delete(jm.jobs, job.ID)
		cleaned++

		// Emit removed event
		jm.emitEvent(Event{
			Type:     EventTypeJobRemoved,
			JobID:    job.ID,
			Job:      jobResp,
			JobCount: len(jm.jobs),
		})
	}

	return stopped, logsDeleted, cleaned
}

// Signal sends a signal to a running job
func (jm *JobManager) Signal(jobID string, signal syscall.Signal) error {
	jm.mu.RLock()
	job, ok := jm.jobs[jobID]
	if !ok {
		jm.mu.RUnlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.CurrentRunID == nil {
		jm.mu.RUnlock()
		return fmt.Errorf("job %s is not running", jobID)
	}

	run := jm.runs[*job.CurrentRunID]
	pid := run.PID
	jm.mu.RUnlock()

	// Send signal to process group
	err := syscall.Kill(-pid, signal)
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	return nil
}

// FindJobByCommand finds a job with matching command in the given workdir
func (jm *JobManager) FindJobByCommand(command []string, workdir string) *Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	signature := ComputeCommandSignature(command)
	indexKey := makeJobIndexKey(signature, workdir)

	if jobID, ok := jm.jobIndex[indexKey]; ok {
		return jm.jobs[jobID]
	}
	return nil
}

// commandsEqual compares two command arrays for equality
func commandsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ListRunsForJob returns all runs for a job, sorted by start time (newest first)
func (jm *JobManager) ListRunsForJob(jobID string) ([]*Run, error) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	if _, ok := jm.jobs[jobID]; !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	var runs []*Run
	for _, run := range jm.runs {
		if run.JobID == jobID {
			runs = append(runs, run)
		}
	}

	// Sort by StartedAt, newest first
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})

	return runs, nil
}

// runToResponse converts a Run to RunResponse
func runToResponse(run *Run) RunResponse {
	resp := RunResponse{
		ID:         run.ID,
		JobID:      run.JobID,
		PID:        run.PID,
		Status:     run.Status,
		ExitCode:   run.ExitCode,
		StdoutPath: run.StdoutPath,
		StderrPath: run.StderrPath,
		StartedAt:  run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		DurationMs: run.Duration().Milliseconds(),
	}
	if run.StoppedAt != nil {
		resp.StoppedAt = run.StoppedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return resp
}

// jobToStats converts a Job to StatsResponse
func jobToStats(job *Job) StatsResponse {
	return StatsResponse{
		JobID:           job.ID,
		Command:         job.Command,
		RunCount:        job.RunCount,
		SuccessCount:    job.SuccessCount,
		SuccessRate:     job.SuccessRate(),
		TotalDurationMs: job.TotalDurationMs,
		AvgDurationMs:   job.AverageDurationMs(),
		MinDurationMs:   job.MinDurationMs,
		MaxDurationMs:   job.MaxDurationMs,
	}
}
