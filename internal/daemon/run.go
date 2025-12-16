package daemon

import (
	"time"
)

// Run represents a single execution of a job
type Run struct {
	ID         string     `json:"id"`          // internal identifier (e.g., "abc-1", "abc-2")
	JobID      string     `json:"job_id"`      // reference to Job
	PID        int        `json:"pid"`         // process ID (0 if stopped)
	Status     string     `json:"status"`      // "running" | "stopped"
	ExitCode   *int       `json:"exit_code"`   // nil if running or killed
	StdoutPath string     `json:"stdout_path"` // path to stdout log
	StderrPath string     `json:"stderr_path"` // path to stderr log
	StartedAt  time.Time  `json:"started_at"`
	StoppedAt  *time.Time `json:"stopped_at,omitempty"` // nil if running

	// Internal fields for process management
	process ProcessHandle
	Ports   []PortInfo // In-memory only, not persisted - listening ports for this run
}

// IsRunning checks if the run's process is still running
func (r *Run) IsRunning() bool {
	if r.process == nil {
		return false
	}
	return r.process.IsRunning()
}

// GetStatus returns "running" or "stopped" based on the process state
func (r *Run) GetStatus() string {
	if r.IsRunning() {
		return "running"
	}
	return "stopped"
}

// Duration returns the duration of the run, or time since start if still running
func (r *Run) Duration() time.Duration {
	if r.StoppedAt != nil {
		return r.StoppedAt.Sub(r.StartedAt)
	}
	return time.Since(r.StartedAt)
}
