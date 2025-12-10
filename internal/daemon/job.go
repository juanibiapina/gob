package daemon

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Job represents a managed background job
type Job struct {
	ID         string    `json:"id"`
	Command    []string  `json:"command"`
	PID        int       `json:"pid"`
	Workdir    string    `json:"workdir"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at"`
	StoppedAt  time.Time `json:"stopped_at,omitempty"`
	StdoutPath string    `json:"stdout_path"`
	StderrPath string    `json:"stderr_path"`
	ExitCode   *int      `json:"exit_code,omitempty"` // nil while running, set when process exits

	// Internal fields for process management
	process ProcessHandle // The running process
}

// JobManager manages all jobs in the daemon
type JobManager struct {
	jobs       map[string]*Job
	mu         sync.RWMutex
	runtimeDir string
	onEvent    func(Event)
	executor   ProcessExecutor
}

// NewJobManager creates a new job manager
func NewJobManager(runtimeDir string, onEvent func(Event)) *JobManager {
	return &JobManager{
		jobs:       make(map[string]*Job),
		runtimeDir: runtimeDir,
		onEvent:    onEvent,
		executor:   &RealProcessExecutor{},
	}
}

// NewJobManagerWithExecutor creates a new job manager with a custom executor (for testing)
func NewJobManagerWithExecutor(runtimeDir string, onEvent func(Event), executor ProcessExecutor) *JobManager {
	return &JobManager{
		jobs:       make(map[string]*Job),
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

// emitEvent sends an event if a callback is registered
func (jm *JobManager) emitEvent(event Event) {
	if jm.onEvent != nil {
		jm.onEvent(event)
	}
}

// jobToResponse converts a Job to JobResponse
func (jm *JobManager) jobToResponse(job *Job) JobResponse {
	resp := JobResponse{
		ID:         job.ID,
		PID:        job.PID,
		Status:     job.Status(),
		Command:    job.Command,
		Workdir:    job.Workdir,
		CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		StartedAt:  job.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		StdoutPath: job.StdoutPath,
		StderrPath: job.StderrPath,
		ExitCode:   job.ExitCode,
	}
	if !job.StoppedAt.IsZero() {
		resp.StoppedAt = job.StoppedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return resp
}

// IsRunning checks if the job's process is still running
func (j *Job) IsRunning() bool {
	if j.process == nil {
		return false
	}
	return j.process.IsRunning()
}

// Status returns "running" or "stopped" based on the process state
func (j *Job) Status() string {
	if j.IsRunning() {
		return "running"
	}
	return "stopped"
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

// AddJob creates and starts a new job
func (jm *JobManager) AddJob(command []string, workdir string) (*Job, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	existingIDs := make(map[string]bool)
	for id := range jm.jobs {
		existingIDs[id] = true
	}
	jobID := generateJobID(existingIDs)

	// Create log file paths
	stdoutPath := fmt.Sprintf("%s/%s.stdout.log", jm.runtimeDir, jobID)
	stderrPath := fmt.Sprintf("%s/%s.stderr.log", jm.runtimeDir, jobID)

	// Start the process
	process, err := jm.executor.Start(command, workdir, stdoutPath, stderrPath)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	job := &Job{
		ID:         jobID,
		Command:    command,
		PID:        process.Pid(),
		Workdir:    workdir,
		CreatedAt:  now,
		StartedAt:  now,
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
		process:    process,
	}

	jm.jobs[jobID] = job

	// Start goroutine to wait for process exit and emit event
	go jm.waitForProcessExit(job)

	// Emit job added event
	jm.emitEvent(Event{
		Type:     EventTypeJobAdded,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
		JobCount: len(jm.jobs),
	})

	return job, nil
}

// waitForProcessExit waits for a job's process to exit and emits an event
func (jm *JobManager) waitForProcessExit(job *Job) {
	if job.process == nil {
		return
	}

	// Wait for process to exit (this blocks until the process terminates)
	err := job.process.Wait()

	// Record stop time
	job.StoppedAt = time.Now()

	// Extract exit code from the error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				// Only get exit code if process exited normally (not killed by signal)
				if status.Exited() {
					code := status.ExitStatus()
					job.ExitCode = &code
				}
				// If killed by signal, leave ExitCode as nil
			}
		}
		// If we couldn't extract exit code, leave it as nil (killed/unknown)
	} else {
		// No error means exit code 0
		code := 0
		job.ExitCode = &code
	}

	// Get job count while holding lock
	jm.mu.RLock()
	jobCount := len(jm.jobs)
	jm.mu.RUnlock()

	// Emit stopped event - this handles both explicit stops and natural exits
	jm.emitEvent(Event{
		Type:     EventTypeJobStopped,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
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
	jm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Check if already stopped
	if !job.IsRunning() {
		return nil
	}

	if force {
		// Send SIGKILL immediately
		if err := syscall.Kill(-job.PID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	} else {
		// Send SIGTERM for graceful shutdown
		if err := syscall.Kill(-job.PID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to stop process: %w", err)
		}
	}

	// Wait for process to terminate (event will be emitted by waitForProcessExit)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !job.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// If still running after timeout and we used SIGTERM, escalate to SIGKILL
	if !force && job.IsRunning() {
		if err := syscall.Kill(-job.PID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to kill process: %w", err)
		}

		// Wait again for SIGKILL
		deadline = time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !job.IsRunning() {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	if job.IsRunning() {
		return fmt.Errorf("process %d still running after SIGKILL", job.PID)
	}

	return nil
}

// StartJob starts a stopped job
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

	// Clear logs before restart
	if err := os.Truncate(job.StdoutPath, 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear stdout log: %w", err)
	}
	if err := os.Truncate(job.StderrPath, 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear stderr log: %w", err)
	}

	// Start the process
	process, err := jm.executor.Start(job.Command, job.Workdir, job.StdoutPath, job.StderrPath)
	if err != nil {
		return err
	}

	job.process = process
	job.PID = process.Pid()
	job.ExitCode = nil     // Clear previous exit code
	job.StartedAt = time.Now()
	job.StoppedAt = time.Time{} // Clear previous stop time

	// Start goroutine to wait for process exit
	go jm.waitForProcessExit(job)

	// Emit started event
	jm.emitEvent(Event{
		Type:     EventTypeJobStarted,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
		JobCount: len(jm.jobs),
	})

	return nil
}

// RestartJob stops (if running) and starts a job
func (jm *JobManager) RestartJob(jobID string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Stop if running (waitForProcessExit will emit the stopped event)
	if job.IsRunning() {
		if err := syscall.Kill(-job.PID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("failed to stop process: %w", err)
		}

		// Wait for termination
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if !job.IsRunning() {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Escalate to SIGKILL if needed
		if job.IsRunning() {
			if err := syscall.Kill(-job.PID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				return fmt.Errorf("failed to kill process: %w", err)
			}

			deadline = time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if !job.IsRunning() {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	// Clear logs before restart
	if err := os.Truncate(job.StdoutPath, 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear stdout log: %w", err)
	}
	if err := os.Truncate(job.StderrPath, 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear stderr log: %w", err)
	}

	// Start the process
	process, err := jm.executor.Start(job.Command, job.Workdir, job.StdoutPath, job.StderrPath)
	if err != nil {
		return err
	}

	job.process = process
	job.PID = process.Pid()
	job.ExitCode = nil     // Clear previous exit code
	job.StartedAt = time.Now()
	job.StoppedAt = time.Time{} // Clear previous stop time

	// Start goroutine to wait for process exit
	go jm.waitForProcessExit(job)

	// Emit started event
	jm.emitEvent(Event{
		Type:     EventTypeJobStarted,
		JobID:    job.ID,
		Job:      jm.jobToResponse(job),
		JobCount: len(jm.jobs),
	})

	return nil
}

// RemoveJob removes a stopped job
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

	// Remove log files
	os.Remove(job.StdoutPath)
	os.Remove(job.StderrPath)

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
	var runningJobs []*Job
	for _, job := range jm.jobs {
		if workdirFilter != "" && job.Workdir != workdirFilter {
			continue
		}
		jobsToNuke = append(jobsToNuke, job)
		if job.IsRunning() {
			runningJobs = append(runningJobs, job)
		}
	}

	// Stop running jobs with SIGTERM
	for _, job := range runningJobs {
		syscall.Kill(-job.PID, syscall.SIGTERM)
	}

	// Wait for graceful termination
	if len(runningJobs) > 0 {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			allStopped := true
			for _, job := range runningJobs {
				if job.IsRunning() {
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
		for _, job := range runningJobs {
			if job.IsRunning() {
				syscall.Kill(-job.PID, syscall.SIGKILL)
			}
		}

		// Wait for SIGKILL to take effect
		time.Sleep(100 * time.Millisecond)
	}

	stopped = len(runningJobs)

	// Note: stopped events will be emitted by waitForProcessExit goroutines
	// when cmd.Wait() returns for each killed process

	// Delete log files and remove jobs
	for _, job := range jobsToNuke {
		if err := os.Remove(job.StdoutPath); err == nil {
			logsDeleted++
		}
		if err := os.Remove(job.StderrPath); err == nil {
			logsDeleted++
		}
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

// Signal sends a signal to a job
func (jm *JobManager) Signal(jobID string, signal syscall.Signal) error {
	jm.mu.RLock()
	job, ok := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Send signal to process group
	err := syscall.Kill(-job.PID, signal)
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	return nil
}

// FindJobByCommand finds a job with matching command in the given workdir
func (jm *JobManager) FindJobByCommand(command []string, workdir string) *Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	for _, job := range jm.jobs {
		if job.Workdir != workdir {
			continue
		}
		if commandsEqual(job.Command, command) {
			return job
		}
	}
	return nil
}

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
