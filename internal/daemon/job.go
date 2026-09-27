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
	"sync/atomic"
	"syscall"
	"time"
)

// Job represents a managed background job (a command that can be run repeatedly)
type Job struct {
	ID               string    `json:"id"`                // user-facing identifier (e.g., "abc")
	Command          []string  `json:"command"`           // the command + args
	CommandSignature string    `json:"command_signature"` // hash for lookups
	Workdir          string    `json:"workdir"`           // directory scope
	Description      string    `json:"description"`       // optional human-readable description
	Blocked          bool      `json:"blocked"`           // if true, job cannot be started
	CurrentRunID     *string   `json:"current_run_id"`    // nil if not running, points to active run
	Stopping         bool      `json:"-"`                 // stop requested, awaiting verification
	StopError        string    `json:"-"`                 // last failed termination attempt
	NextRunSeq       int       `json:"next_run_seq"`      // counter for internal run IDs
	CreatedAt        time.Time `json:"created_at"`

	// Cached statistics (updated on run completion)
	RunCount               int   `json:"run_count"`
	SuccessCount           int   `json:"success_count"`
	FailureCount           int   `json:"failure_count"`
	SuccessTotalDurationMs int64 `json:"success_total_duration_ms"`
	FailureTotalDurationMs int64 `json:"failure_total_duration_ms"`
	MinDurationMs          int64 `json:"min_duration_ms"`
	MaxDurationMs          int64 `json:"max_duration_ms"`
}

// IsRunning checks if the job has a currently running process
func (j *Job) IsRunning() bool {
	return j.CurrentRunID != nil
}

// Status reports the current lifecycle state.
func (j *Job) Status() string {
	if j.Stopping {
		return "stopping"
	}
	if j.IsRunning() {
		return "running"
	}
	return "stopped"
}

// AverageDurationMs returns the average duration of successful runs in milliseconds, or 0 if no successes
func (j *Job) AverageDurationMs() int64 {
	if j.SuccessCount == 0 {
		return 0
	}
	return j.SuccessTotalDurationMs / int64(j.SuccessCount)
}

// FailureAverageDurationMs returns the average duration of failed runs in milliseconds, or 0 if no failures
func (j *Job) FailureAverageDurationMs() int64 {
	if j.FailureCount == 0 {
		return 0
	}
	return j.FailureTotalDurationMs / int64(j.FailureCount)
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
	jobs         map[string]*Job   // keyed by job ID
	runs         map[string]*Run   // keyed by run ID
	jobIndex     map[string]string // signature+workdir -> job ID for quick lookup
	latestRun    map[string]*Run   // latest run by job ID
	stops        map[string]*stopAttempt
	mu           sync.RWMutex
	runtimeDir   string
	onEvent      func(Event)
	executor     ProcessExecutor
	scanPorts    func(int) ([]PortInfo, error)
	snapshotTree func(int) ([]int, error)
	listView     atomic.Pointer[[]JobResponse]
	store        *Store // database store for persistence
}

// NewJobManager creates a new job manager
func NewJobManager(runtimeDir string, onEvent func(Event), store *Store) *JobManager {
	return &JobManager{
		jobs:         make(map[string]*Job),
		runs:         make(map[string]*Run),
		jobIndex:     make(map[string]string),
		latestRun:    make(map[string]*Run),
		stops:        make(map[string]*stopAttempt),
		runtimeDir:   runtimeDir,
		onEvent:      onEvent,
		executor:     &RealProcessExecutor{},
		scanPorts:    getProcessTreePorts,
		snapshotTree: getProcessTreePIDs,
		store:        store,
	}
}

// NewJobManagerWithExecutor creates a new job manager with a custom executor (for testing)
func NewJobManagerWithExecutor(runtimeDir string, onEvent func(Event), executor ProcessExecutor, store *Store) *JobManager {
	return &JobManager{
		jobs:         make(map[string]*Job),
		runs:         make(map[string]*Run),
		jobIndex:     make(map[string]string),
		latestRun:    make(map[string]*Run),
		stops:        make(map[string]*stopAttempt),
		runtimeDir:   runtimeDir,
		onEvent:      onEvent,
		executor:     executor,
		scanPorts:    getProcessTreePorts,
		snapshotTree: getProcessTreePIDs,
		store:        store,
	}
}

// JobCount returns the number of jobs
func (jm *JobManager) JobCount() int {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return len(jm.jobs)
}

// HasRunningJobs returns true if there are any running jobs
func (jm *JobManager) HasRunningJobs() bool {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	for _, job := range jm.jobs {
		if job.IsRunning() {
			return true
		}
	}
	return false
}

// countRunningJobsLocked returns the number of running jobs (caller must hold lock)
func (jm *JobManager) countRunningJobsLocked() int {
	count := 0
	for _, job := range jm.jobs {
		if job.IsRunning() {
			count++
		}
	}
	return count
}

// LoadFromStore loads jobs and runs from the database
func (jm *JobManager) LoadFromStore() error {
	if jm.store == nil {
		return nil
	}

	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()

	// Load jobs
	jobs, err := jm.store.LoadJobs()
	if err != nil {
		return fmt.Errorf("failed to load jobs: %w", err)
	}

	for _, job := range jobs {
		jm.jobs[job.ID] = job
		indexKey := makeJobIndexKey(job.CommandSignature, job.Workdir)
		jm.jobIndex[indexKey] = job.ID
	}

	// Load runs
	runs, err := jm.store.LoadRuns()
	if err != nil {
		return fmt.Errorf("failed to load runs: %w", err)
	}

	for _, run := range runs {
		jm.runs[run.ID] = run
		jm.recordLatestRunLocked(run)
		// Note: We don't restore CurrentRunID here because all runs
		// should be stopped after crash recovery
	}

	return nil
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

// jobToResponse converts a Job to JobResponse
func (jm *JobManager) jobToResponse(job *Job) JobResponse {
	resp := JobResponse{
		ID:          job.ID,
		Status:      job.Status(),
		StopError:   job.StopError,
		Command:     job.Command,
		Workdir:     job.Workdir,
		Description: job.Description,
		Blocked:     job.Blocked,
		CreatedAt:   job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),

		// Statistics
		RunCount:             job.RunCount,
		SuccessCount:         job.SuccessCount,
		FailureCount:         job.FailureCount,
		SuccessRate:          job.SuccessRate(),
		AvgDurationMs:        job.AverageDurationMs(),
		FailureAvgDurationMs: job.FailureAverageDurationMs(),
		MinDurationMs:        job.MinDurationMs,
		MaxDurationMs:        job.MaxDurationMs,
	}

	// If there's a current run, include its details
	if job.CurrentRunID != nil {
		if run, ok := jm.runs[*job.CurrentRunID]; ok {
			resp.PID = run.PID
			resp.StartedAt = run.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			resp.StdoutPath = run.StdoutPath
			resp.StderrPath = run.StderrPath
			resp.ExitCode = run.ExitCode
			resp.Ports = run.Ports // Include ports for running jobs
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
	return jm.latestRun[jobID]
}

func (jm *JobManager) recordLatestRunLocked(run *Run) {
	latest := jm.latestRun[run.JobID]
	if latest == nil || run.StartedAt.After(latest.StartedAt) || (run.StartedAt.Equal(latest.StartedAt) && run.ID > latest.ID) {
		jm.latestRun[run.JobID] = run
	}
}

func (jm *JobManager) rebuildLatestRunLocked(jobID string) {
	delete(jm.latestRun, jobID)
	for _, run := range jm.runs {
		if run.JobID == jobID {
			jm.recordLatestRunLocked(run)
		}
	}
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

// ErrJobBlocked is returned when trying to start a blocked job
type ErrJobBlocked struct {
	Description string
}

func (e *ErrJobBlocked) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("job is blocked: %s", e.Description)
	}
	return "job is blocked"
}

// AddJob finds or creates a job for the command, then starts a new run.
// Returns the job, the action taken ("created", "started", or "already_running"), and any error.
func (jm *JobManager) AddJob(command []string, workdir string, description string, blocked bool, env []string) (*Job, string, error) {
	if len(command) == 0 {
		return nil, "", fmt.Errorf("empty command")
	}

	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()

	signature := ComputeCommandSignature(command)
	indexKey := makeJobIndexKey(signature, workdir)

	// Check if job already exists for this command+workdir
	if existingJobID, ok := jm.jobIndex[indexKey]; ok {
		job := jm.jobs[existingJobID]

		// Update blocked status and description if provided
		jobChanged := false
		if job.Blocked != blocked {
			job.Blocked = blocked
			jobChanged = true
		}
		if description != "" && job.Description != description {
			job.Description = description
			jobChanged = true
		}

		// Persist changes to database
		if jobChanged && jm.store != nil {
			if err := jm.store.UpdateJob(job); err != nil {
				Logger.Warn("failed to update job", "id", job.ID, "error", err)
			}
		}

		// Check if job is blocked - return error with description
		if job.Blocked {
			return job, "", &ErrJobBlocked{Description: job.Description}
		}

		if job.IsRunning() {
			// Job is already running - emit update event if changed
			if jobChanged {
				jm.emitEvent(Event{
					Type:            EventTypeJobUpdated,
					JobID:           job.ID,
					Job:             jm.jobToResponse(job),
					JobCount:        len(jm.jobs),
					RunningJobCount: jm.countRunningJobsLocked(),
				})
			}

			return job, "already_running", nil
		}

		// Start a new run for existing job with the provided environment
		run, err := jm.startRunLocked(job, env)
		if err != nil {
			return nil, "", err
		}

		// Emit job started event (reusing existing job)
		jm.emitEvent(Event{
			Type:            EventTypeJobStarted,
			JobID:           job.ID,
			Job:             jm.jobToResponse(job),
			JobCount:        len(jm.jobs),
			RunningJobCount: jm.countRunningJobsLocked(),
		})

		// Emit run started event
		runResp := runToResponse(run)
		jm.emitEvent(Event{
			Type:            EventTypeRunStarted,
			JobID:           job.ID,
			Job:             jm.jobToResponse(job),
			Run:             &runResp,
			JobCount:        len(jm.jobs),
			RunningJobCount: jm.countRunningJobsLocked(),
		})

		return job, "started", nil
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
		Description:      description,
		Blocked:          blocked,
		NextRunSeq:       1,
		CreatedAt:        now,
	}

	jm.jobs[jobID] = job
	jm.jobIndex[indexKey] = jobID

	// Persist new job to database
	if jm.store != nil {
		if err := jm.store.InsertJob(job); err != nil {
			delete(jm.jobs, jobID)
			delete(jm.jobIndex, indexKey)
			return nil, "", fmt.Errorf("failed to persist job: %w", err)
		}
	}

	// Check if job is blocked - return error with description (after creating)
	if blocked {
		// Emit job added event for the blocked job
		jm.emitEvent(Event{
			Type:            EventTypeJobAdded,
			JobID:           job.ID,
			Job:             jm.jobToResponse(job),
			JobCount:        len(jm.jobs),
			RunningJobCount: jm.countRunningJobsLocked(),
		})
		return job, "", &ErrJobBlocked{Description: job.Description}
	}

	// Start first run with the provided environment
	run, err := jm.startRunLocked(job, env)
	if err != nil {
		// Clean up job if run failed to start
		if jm.store != nil {
			jm.store.DeleteJob(jobID)
		}
		delete(jm.jobs, jobID)
		delete(jm.jobIndex, indexKey)
		return nil, "", err
	}

	// Emit job added event
	jm.emitEvent(Event{
		Type:            EventTypeJobAdded,
		JobID:           job.ID,
		Job:             jm.jobToResponse(job),
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	// Emit run started event
	runResp := runToResponse(run)
	jm.emitEvent(Event{
		Type:            EventTypeRunStarted,
		JobID:           job.ID,
		Job:             jm.jobToResponse(job),
		Run:             &runResp,
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	return job, "created", nil
}

// CreateJob creates a job without starting it (for autostart=false in gobfile)
func (jm *JobManager) CreateJob(command []string, workdir string, description string, blocked bool) (*Job, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()

	signature := ComputeCommandSignature(command)
	indexKey := makeJobIndexKey(signature, workdir)

	// Check if job already exists for this command+workdir
	if existingJobID, ok := jm.jobIndex[indexKey]; ok {
		job := jm.jobs[existingJobID]

		// Update description and blocked status if different from current
		jobChanged := false
		if job.Blocked != blocked {
			job.Blocked = blocked
			jobChanged = true
		}
		if description != "" && job.Description != description {
			job.Description = description
			jobChanged = true
		}

		if jobChanged {
			// Persist updates to database
			if jm.store != nil {
				if err := jm.store.UpdateJob(job); err != nil {
					Logger.Warn("failed to update job", "id", job.ID, "error", err)
				}
			}
			// Emit event for changes
			jm.emitEvent(Event{
				Type:            EventTypeJobUpdated,
				JobID:           job.ID,
				Job:             jm.jobToResponse(job),
				JobCount:        len(jm.jobs),
				RunningJobCount: jm.countRunningJobsLocked(),
			})
		}

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
		Description:      description,
		Blocked:          blocked,
		NextRunSeq:       1,
		CreatedAt:        now,
	}

	jm.jobs[jobID] = job
	jm.jobIndex[indexKey] = jobID

	// Persist new job to database
	if jm.store != nil {
		if err := jm.store.InsertJob(job); err != nil {
			delete(jm.jobs, jobID)
			delete(jm.jobIndex, indexKey)
			return nil, fmt.Errorf("failed to persist job: %w", err)
		}
	}

	// Emit job added event
	jm.emitEvent(Event{
		Type:            EventTypeJobAdded,
		JobID:           job.ID,
		Job:             jm.jobToResponse(job),
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	return job, nil
}

// startRunLocked creates and starts a new run for a job (caller must hold lock)
func (jm *JobManager) startRunLocked(job *Job, env []string) (*Run, error) {
	runID := fmt.Sprintf("%s-%d", job.ID, job.NextRunSeq)
	job.NextRunSeq++

	// Create log file paths
	stdoutPath := fmt.Sprintf("%s/%s.stdout.log", jm.runtimeDir, runID)
	stderrPath := fmt.Sprintf("%s/%s.stderr.log", jm.runtimeDir, runID)

	// Start the process with the provided environment
	process, err := jm.executor.Start(job.Command, job.Workdir, env, stdoutPath, stderrPath)
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
		finalized:  make(chan struct{}),
	}

	jm.runs[runID] = run
	jm.recordLatestRunLocked(run)
	job.CurrentRunID = &runID

	// Persist run to database
	if jm.store != nil {
		if err := jm.store.InsertRun(run); err != nil {
			// Log but don't fail - in-memory state is still valid
			Logger.Warn("failed to persist run", "id", run.ID, "error", err)
		}
		// Update job's NextRunSeq in database
		if err := jm.store.UpdateJob(job); err != nil {
			Logger.Warn("failed to update job", "id", job.ID, "error", err)
		}
	}

	// Start goroutine to wait for process exit
	go jm.waitForProcessExit(job, run)

	// Schedule port polling at 2s, 5s, 10s
	jm.schedulePortPolling(job, run)

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
	verified := run.stopVerified
	jm.mu.Unlock()
	if verified != nil {
		<-verified
	}
	jm.mu.Lock()

	// Record stop time
	now := time.Now()
	run.StoppedAt = &now
	run.Status = "stopped"
	run.Ports = nil // Clear ports when run stops
	run.knownPIDs = nil

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

	// Clear job's current run pointer only if it still points to this run.
	// This prevents a race condition where a restart creates a new run before
	// this goroutine completes, and we would incorrectly clear the new run's ID.
	if job.CurrentRunID != nil && *job.CurrentRunID == run.ID {
		job.CurrentRunID = nil
		job.Stopping = false
		job.StopError = ""
	}

	// Update job statistics
	durationMs := run.StoppedAt.Sub(run.StartedAt).Milliseconds()
	job.RunCount++

	if run.ExitCode != nil && *run.ExitCode == 0 {
		job.SuccessCount++
		job.SuccessTotalDurationMs += durationMs
	} else if run.ExitCode != nil {
		job.FailureCount++
		job.FailureTotalDurationMs += durationMs
	}
	// Killed processes (ExitCode == nil) only increment RunCount

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

	// Persist run completion and job stats to database
	if jm.store != nil {
		if err := jm.store.UpdateRun(run); err != nil {
			Logger.Warn("failed to update run", "id", run.ID, "error", err)
		}
		if err := jm.store.UpdateJob(job); err != nil {
			Logger.Warn("failed to update job stats", "id", job.ID, "error", err)
		}
	}

	jobCount := len(jm.jobs)
	runningJobCount := jm.countRunningJobsLocked()
	jobResp := jm.jobToResponse(job)
	runResp := runToResponse(run)
	jm.publishListLocked()

	jm.mu.Unlock()

	// Emit run stopped event
	jm.emitEvent(Event{
		Type:            EventTypeRunStopped,
		JobID:           job.ID,
		Job:             jobResp,
		Run:             &runResp,
		JobCount:        jobCount,
		RunningJobCount: runningJobCount,
	})

	// Emit job stopped event (for backward compatibility)
	jm.emitEvent(Event{
		Type:            EventTypeJobStopped,
		JobID:           job.ID,
		Job:             jobResp,
		JobCount:        jobCount,
		RunningJobCount: runningJobCount,
	})
	if run.finalized != nil {
		close(run.finalized)
	}
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

	if len(jobs) < 2 {
		return jobs
	}

	sort.Slice(jobs, func(i, j int) bool {
		activity := func(job *Job) time.Time {
			if run := jm.latestRun[job.ID]; run != nil {
				return run.StartedAt
			}
			return job.CreatedAt
		}
		return activity(jobs[i]).After(activity(jobs[j]))
	})

	return jobs
}

// publishListLocked creates a read view after a state transition (caller holds the write lock).
func (jm *JobManager) publishListLocked() {
	jobs := make([]*Job, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		activity := func(job *Job) time.Time {
			if run := jm.latestRun[job.ID]; run != nil {
				return run.StartedAt
			}
			return job.CreatedAt
		}
		left, right := activity(jobs[i]), activity(jobs[j])
		if left.Equal(right) {
			return jobs[i].ID < jobs[j].ID
		}
		return left.After(right)
	})
	view := make([]JobResponse, 0, len(jobs))
	for _, job := range jobs {
		response := jm.jobToResponse(job)
		response.Command = append([]string(nil), response.Command...)
		response.Ports = append([]PortInfo(nil), response.Ports...)
		if response.ExitCode != nil {
			code := *response.ExitCode
			response.ExitCode = &code
		}
		view = append(view, response)
	}
	jm.listView.Store(&view)
}

// ListJobResponses reads the latest published view without waiting for writers.
func (jm *JobManager) ListJobResponses(workdir string) []JobResponse {
	view := jm.listView.Load()
	if view == nil {
		return []JobResponse{}
	}
	responses := make([]JobResponse, 0, len(*view))
	for _, job := range *view {
		if workdir != "" && job.Workdir != workdir {
			continue
		}
		job.Command = append([]string(nil), job.Command...)
		job.Ports = append([]PortInfo(nil), job.Ports...)
		if job.ExitCode != nil {
			code := *job.ExitCode
			job.ExitCode = &code
		}
		responses = append(responses, job)
	}
	return responses
}

// GetJobResponse reads one job from the published view.
func (jm *JobManager) GetJobResponse(jobID string) (JobResponse, error) {
	view := jm.listView.Load()
	if view != nil {
		for _, response := range *view {
			if response.ID == jobID {
				response.Command = append([]string(nil), response.Command...)
				response.Ports = append([]PortInfo(nil), response.Ports...)
				if response.ExitCode != nil {
					code := *response.ExitCode
					response.ExitCode = &code
				}
				return response, nil
			}
		}
	}
	return JobResponse{}, fmt.Errorf("job not found: %s", jobID)
}

// stopJobProcess performs the process-tree termination after a stop has been accepted.
func (jm *JobManager) stopJobProcess(jobID string, force bool, escalate <-chan struct{}) error {
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

	// Snapshot and retain identities before signaling. A retry still verifies descendants
	// captured before the root disappeared.
	treePIDs, err := jm.snapshotTree(pid)
	if err != nil {
		return fmt.Errorf("failed to inspect process tree: %w", err)
	}
	observed, err := captureProcessIdentities(treePIDs)
	if err != nil {
		return fmt.Errorf("failed to identify process tree: %w", err)
	}
	jm.mu.Lock()
	if run.knownPIDs == nil {
		run.knownPIDs = make(map[int]int64)
	}
	for child, created := range observed {
		run.knownPIDs[child] = created
	}
	identities := make(map[int]int64, len(run.knownPIDs))
	for child, created := range run.knownPIDs {
		identities[child] = created
	}
	jm.mu.Unlock()
	live := func() ([]int, error) { return filterRunningPIDs(identities) }
	signalGroup := true
	if _, real := run.process.(*realProcessHandle); real && !isOurProcess(pid, run.StartedAt, job.Command) {
		if processExists(pid) || syscall.Kill(-pid, 0) != syscall.ESRCH {
			return fmt.Errorf("cannot verify process identity for job %s (PID %d)", jobID, pid)
		}
		signalGroup = false
	}
	send := func(sig syscall.Signal) error {
		if signalGroup {
			if err := syscall.Kill(-pid, sig); err != nil && err != syscall.ESRCH {
				return err
			}
		}
		return killPIDs(identities, sig)
	}
	if force {
		if err := send(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process tree: %w", err)
		}
	} else {
		if err := send(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to stop process tree: %w", err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		survivors, err := live()
		if err != nil {
			return fmt.Errorf("failed to verify process tree: %w", err)
		}
		if len(survivors) == 0 {
			return nil
		}
		select {
		case <-escalate:
			deadline = time.Now()
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !force {
		if err := send(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process tree: %w", err)
		}
		deadline = time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			survivors, err := live()
			if err != nil {
				return fmt.Errorf("failed to verify process tree: %w", err)
			}
			if len(survivors) == 0 {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	survivors, err := live()
	if err != nil {
		return fmt.Errorf("failed to verify process tree: %w", err)
	}
	if len(survivors) > 0 {
		return fmt.Errorf("process tree has %d surviving processes after SIGKILL: %v", len(survivors), survivors)
	}
	return nil
}

// StartJob starts a new run for a stopped job with the provided environment
func (jm *JobManager) StartJob(jobID string, env []string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()

	job, ok := jm.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Check if blocked
	if job.Blocked {
		return &ErrJobBlocked{Description: job.Description}
	}

	// Check if already running
	if job.IsRunning() {
		return fmt.Errorf("job %s is already running (use 'gob restart' to restart a running job)", jobID)
	}

	// Start new run with the provided environment
	run, err := jm.startRunLocked(job, env)
	if err != nil {
		return err
	}

	// Emit started event
	jm.emitEvent(Event{
		Type:            EventTypeJobStarted,
		JobID:           job.ID,
		Job:             jm.jobToResponse(job),
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	// Emit run started event
	runResp := runToResponse(run)
	jm.emitEvent(Event{
		Type:            EventTypeRunStarted,
		JobID:           job.ID,
		Job:             jm.jobToResponse(job),
		Run:             &runResp,
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	return nil
}

// RestartJob waits for verified termination before starting a replacement run.
func (jm *JobManager) RestartJob(jobID string, env []string) error {
	jm.mu.RLock()
	job, ok := jm.jobs[jobID]
	if !ok {
		jm.mu.RUnlock()
		return fmt.Errorf("job not found: %s", jobID)
	}
	if job.Blocked {
		description := job.Description
		jm.mu.RUnlock()
		return &ErrJobBlocked{Description: description}
	}
	running := job.IsRunning()
	jm.mu.RUnlock()
	if running {
		if err := jm.StopJob(jobID, false); err != nil {
			return err
		}
	}
	return jm.StartJob(jobID, env)
}

// RemoveJob removes a stopped job and all its runs
func (jm *JobManager) RemoveJob(jobID string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()

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
	delete(jm.latestRun, jobID)

	// Delete from database (cascades to runs)
	if jm.store != nil {
		if err := jm.store.DeleteJob(jobID); err != nil {
			Logger.Warn("failed to delete job from database", "id", jobID, "error", err)
		}
	}

	// Emit removed event
	jm.emitEvent(Event{
		Type:            EventTypeJobRemoved,
		JobID:           jobID,
		Job:             jobResp,
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	return nil
}

// RemoveRun removes a stopped run and its log files
func (jm *JobManager) RemoveRun(runID string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()

	run, ok := jm.runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	// Check if run is currently running
	if run.Status != "stopped" {
		return fmt.Errorf("cannot remove active run: %s (stop the job first)", runID)
	}

	// Get the job for stats update
	job, jobExists := jm.jobs[run.JobID]

	// Capture run info for event before deletion
	runResp := runToResponse(run)

	// Update job statistics if job exists
	if jobExists && run.StoppedAt != nil {
		durationMs := run.StoppedAt.Sub(run.StartedAt).Milliseconds()

		// Decrement counts
		job.RunCount--
		if run.ExitCode != nil && *run.ExitCode == 0 {
			job.SuccessCount--
			job.SuccessTotalDurationMs -= durationMs
		} else if run.ExitCode != nil {
			job.FailureCount--
			job.FailureTotalDurationMs -= durationMs
		}
		// Killed processes (ExitCode == nil) only affect RunCount

		// Recalculate min/max duration from remaining runs
		jm.recalculateMinMaxDuration(job)
	}

	// Delete log files
	os.Remove(run.StdoutPath)
	os.Remove(run.StderrPath)

	// Remove from in-memory map
	delete(jm.runs, runID)
	if jm.latestRun[run.JobID] == run {
		jm.rebuildLatestRunLocked(run.JobID)
	}

	// Delete from database and update job stats
	if jm.store != nil {
		if err := jm.store.DeleteRun(runID); err != nil {
			Logger.Warn("failed to delete run from database", "id", runID, "error", err)
		}
		if jobExists {
			if err := jm.store.UpdateJob(job); err != nil {
				Logger.Warn("failed to update job stats", "id", job.ID, "error", err)
			}
		}
	}

	// Emit removed event with updated stats
	var jobResp JobResponse
	if jobExists {
		jobResp = jm.jobToResponse(job)
	}
	jm.emitEvent(Event{
		Type:            EventTypeRunRemoved,
		JobID:           run.JobID,
		Job:             jobResp,
		Run:             &runResp,
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})

	return nil
}

// recalculateMinMaxDuration recalculates min/max duration from all stopped runs for a job
func (jm *JobManager) recalculateMinMaxDuration(job *Job) {
	job.MinDurationMs = 0
	job.MaxDurationMs = 0

	first := true
	for _, run := range jm.runs {
		if run.JobID != job.ID || run.StoppedAt == nil {
			continue
		}
		durationMs := run.StoppedAt.Sub(run.StartedAt).Milliseconds()
		if first {
			job.MinDurationMs = durationMs
			job.MaxDurationMs = durationMs
			first = false
		} else {
			if durationMs < job.MinDurationMs {
				job.MinDurationMs = durationMs
			}
			if durationMs > job.MaxDurationMs {
				job.MaxDurationMs = durationMs
			}
		}
	}
}

// StopAll stops all running jobs without holding the state lock during termination.
func (jm *JobManager) StopAll() (stopped int) {
	jm.mu.RLock()
	ids := make([]string, 0)
	for id, job := range jm.jobs {
		if job.IsRunning() {
			ids = append(ids, id)
		}
	}
	jm.mu.RUnlock()
	attempts := make([]<-chan struct{}, 0, len(ids))
	for _, id := range ids {
		_, done, err := jm.RequestStop(id, false)
		if err != nil {
			Logger.Warn("failed to request stop", "id", id, "error", err)
			continue
		}
		attempts = append(attempts, done)
	}
	for _, done := range attempts {
		<-done
	}
	for _, id := range ids {
		jm.mu.RLock()
		op := jm.stops[id]
		if op != nil && op.err == nil {
			stopped++
		}
		if op != nil && op.err != nil {
			Logger.Warn("failed to stop job", "id", id, "error", op.err)
		}
		jm.mu.RUnlock()
	}
	return stopped
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

// schedulePortPolling schedules port polling at 2s, 5s, and 10s after run starts
func (jm *JobManager) schedulePortPolling(job *Job, run *Run) {
	delays := []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}
	for _, delay := range delays {
		go func(d time.Duration) {
			time.Sleep(d)
			jm.refreshPorts(job.ID, run.ID)
		}(delay)
	}
}

// refreshPorts queries ports for a run and emits an event if they changed
func (jm *JobManager) refreshPorts(jobID, runID string) {
	jm.mu.RLock()
	job, ok := jm.jobs[jobID]
	if !ok || job.CurrentRunID == nil || *job.CurrentRunID != runID {
		jm.mu.RUnlock()
		return // Job gone or different run now
	}
	pid := jm.runs[runID].PID
	jm.mu.RUnlock()

	ports, err := jm.scanPorts(pid)
	if err != nil {
		return
	}

	jm.mu.Lock()
	defer jm.mu.Unlock()
	defer jm.publishListLocked()
	job, ok = jm.jobs[jobID]
	if !ok || job.CurrentRunID == nil || *job.CurrentRunID != runID {
		return // Run stopped or replaced while scanning
	}
	run := jm.runs[runID]
	if portsEqual(run.Ports, ports) {
		return // No change
	}

	run.Ports = ports

	jm.emitEvent(Event{
		Type:            EventTypePortsUpdated,
		JobID:           jobID,
		Job:             jm.jobToResponse(job),
		Ports:           ports,
		JobCount:        len(jm.jobs),
		RunningJobCount: jm.countRunningJobsLocked(),
	})
}

// portsEqual compares two port slices for equality
func portsEqual(a, b []PortInfo) bool {
	if len(a) != len(b) {
		return false
	}
	// Create maps for comparison (order-independent)
	aMap := make(map[string]bool)
	for _, p := range a {
		key := fmt.Sprintf("%d:%s:%s:%d", p.Port, p.Protocol, p.Address, p.PID)
		aMap[key] = true
	}
	for _, p := range b {
		key := fmt.Sprintf("%d:%s:%s:%d", p.Port, p.Protocol, p.Address, p.PID)
		if !aMap[key] {
			return false
		}
	}
	return true
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
