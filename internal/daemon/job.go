package daemon

import (
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
	StdoutPath string    `json:"stdout_path"`
	StderrPath string    `json:"stderr_path"`
}

// JobManager manages all jobs in the daemon
type JobManager struct {
	jobs       map[string]*Job
	mu         sync.RWMutex
	runtimeDir string
}

// NewJobManager creates a new job manager
func NewJobManager(runtimeDir string) *JobManager {
	return &JobManager{
		jobs:       make(map[string]*Job),
		runtimeDir: runtimeDir,
	}
}

// IsRunning checks if the job's process is still running
func (j *Job) IsRunning() bool {
	err := syscall.Kill(j.PID, syscall.Signal(0))
	return err == nil
}

// Status returns "running" or "stopped" based on the process state
func (j *Job) Status() string {
	if j.IsRunning() {
		return "running"
	}
	return "stopped"
}

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// generateJobID creates a unique job ID using base62-encoded millisecond timestamp
func generateJobID() string {
	n := time.Now().UnixMilli()
	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}

// AddJob creates and starts a new job
func (jm *JobManager) AddJob(command []string, workdir string) (*Job, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jobID := generateJobID()

	// Create log file paths
	stdoutPath := fmt.Sprintf("%s/%s.stdout.log", jm.runtimeDir, jobID)
	stderrPath := fmt.Sprintf("%s/%s.stderr.log", jm.runtimeDir, jobID)

	// Start the process
	pid, err := jm.startProcess(command, workdir, stdoutPath, stderrPath)
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:         jobID,
		Command:    command,
		PID:        pid,
		Workdir:    workdir,
		CreatedAt:  time.Now(),
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
	}

	jm.jobs[jobID] = job
	return job, nil
}

// startProcess starts a process as a child of the daemon
func (jm *JobManager) startProcess(command []string, workdir string, stdoutPath, stderrPath string) (int, error) {
	if len(command) == 0 {
		return 0, fmt.Errorf("empty command")
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = workdir

	// Create a new process group so we can signal all children together
	// But don't detach (no Setsid) - the daemon manages these processes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Create log files
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open stdout log file: %w", err)
	}

	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		stdoutFile.Close()
		return 0, fmt.Errorf("failed to open stderr log file: %w", err)
	}

	// Redirect stdin to /dev/null
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return 0, fmt.Errorf("failed to open /dev/null: %w", err)
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.Stdin = devNull

	// Start the process
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		devNull.Close()
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	pid := cmd.Process.Pid

	// Close file descriptors in daemon (child keeps them)
	stdoutFile.Close()
	stderrFile.Close()
	devNull.Close()

	// Reap the process in a goroutine to prevent zombies
	go cmd.Wait()

	return pid, nil
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

	// Wait for process to terminate
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
	pid, err := jm.startProcess(job.Command, job.Workdir, job.StdoutPath, job.StderrPath)
	if err != nil {
		return err
	}

	job.PID = pid
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

	// Stop if running
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
	pid, err := jm.startProcess(job.Command, job.Workdir, job.StdoutPath, job.StderrPath)
	if err != nil {
		return err
	}

	job.PID = pid
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

	// Remove log files
	os.Remove(job.StdoutPath)
	os.Remove(job.StderrPath)

	delete(jm.jobs, jobID)
	return nil
}

// Cleanup removes all stopped jobs
func (jm *JobManager) Cleanup(workdirFilter string) int {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	count := 0
	for id, job := range jm.jobs {
		if workdirFilter != "" && job.Workdir != workdirFilter {
			continue
		}
		if !job.IsRunning() {
			os.Remove(job.StdoutPath)
			os.Remove(job.StderrPath)
			delete(jm.jobs, id)
			count++
		}
	}

	return count
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

	// Delete log files and remove jobs
	for _, job := range jobsToNuke {
		if err := os.Remove(job.StdoutPath); err == nil {
			logsDeleted++
		}
		if err := os.Remove(job.StderrPath); err == nil {
			logsDeleted++
		}
		delete(jm.jobs, job.ID)
		cleaned++
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
