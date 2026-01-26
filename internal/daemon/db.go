package daemon

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/shirou/gopsutil/v4/process"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// OpenDatabase opens the SQLite database and runs migrations
func OpenDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pragmas (must be done outside of transactions)
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set journal mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Run migrations
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	// Suppress goose logging
	goose.SetLogger(goose.NopLogger())

	if err := goose.Up(db, "migrations"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Store handles all database operations for job persistence
type Store struct {
	db         *sql.DB
	instanceID string
}

// NewStore creates a new store with a unique instance ID
func NewStore(db *sql.DB) *Store {
	instanceID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	return &Store{
		db:         db,
		instanceID: instanceID,
	}
}

// InstanceID returns the daemon's unique instance ID
func (s *Store) InstanceID() string {
	return s.instanceID
}

// WasCleanShutdown checks if the previous daemon instance shut down cleanly
func (s *Store) WasCleanShutdown() bool {
	var value string
	err := s.db.QueryRow("SELECT value FROM daemon_state WHERE key = 'shutdown_clean'").Scan(&value)
	if err != nil {
		// No record means first start, treat as clean
		return true
	}
	return value == "true"
}

// SetShutdownClean sets the shutdown_clean flag
func (s *Store) SetShutdownClean(clean bool) error {
	value := "false"
	if clean {
		value = "true"
	}
	_, err := s.db.Exec(`
		INSERT INTO daemon_state (key, value) VALUES ('shutdown_clean', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, value)
	return err
}

// SetInstanceID records the current daemon instance ID
func (s *Store) SetInstanceID() error {
	_, err := s.db.Exec(`
		INSERT INTO daemon_state (key, value) VALUES ('instance_id', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, s.instanceID)
	return err
}

// InsertJob persists a new job to the database
func (s *Store) InsertJob(job *Job) error {
	commandJSON, err := json.Marshal(job.Command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	blocked := 0
	if job.Blocked {
		blocked = 1
	}

	_, err = s.db.Exec(`
		INSERT INTO jobs (id, command_json, command_signature, workdir, description, blocked, next_run_seq, created_at,
			run_count, success_count, failure_count, success_total_duration_ms, failure_total_duration_ms, min_duration_ms, max_duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, string(commandJSON), job.CommandSignature, job.Workdir, nullableString(job.Description), blocked, job.NextRunSeq,
		job.CreatedAt.Format(time.RFC3339), job.RunCount, job.SuccessCount, job.FailureCount,
		job.SuccessTotalDurationMs, job.FailureTotalDurationMs, nullableInt64(job.MinDurationMs), nullableInt64(job.MaxDurationMs))
	return err
}

// UpdateJob updates an existing job in the database
func (s *Store) UpdateJob(job *Job) error {
	blocked := 0
	if job.Blocked {
		blocked = 1
	}

	_, err := s.db.Exec(`
		UPDATE jobs SET
			next_run_seq = ?,
			run_count = ?,
			success_count = ?,
			failure_count = ?,
			success_total_duration_ms = ?,
			failure_total_duration_ms = ?,
			min_duration_ms = ?,
			max_duration_ms = ?,
			description = ?,
			blocked = ?
		WHERE id = ?
	`, job.NextRunSeq, job.RunCount, job.SuccessCount, job.FailureCount,
		job.SuccessTotalDurationMs, job.FailureTotalDurationMs, nullableInt64(job.MinDurationMs), nullableInt64(job.MaxDurationMs),
		nullableString(job.Description), blocked, job.ID)
	return err
}

// DeleteJob removes a job from the database (runs cascade)
func (s *Store) DeleteJob(jobID string) error {
	_, err := s.db.Exec("DELETE FROM jobs WHERE id = ?", jobID)
	return err
}

// InsertRun persists a new run to the database
func (s *Store) InsertRun(run *Run) error {
	_, err := s.db.Exec(`
		INSERT INTO runs (id, job_id, pid, status, exit_code, stdout_path, stderr_path, started_at, stopped_at, daemon_instance_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.ID, run.JobID, run.PID, run.Status, run.ExitCode, run.StdoutPath, run.StderrPath,
		run.StartedAt.Format(time.RFC3339), nil, s.instanceID)
	return err
}

// UpdateRun updates a run in the database
func (s *Store) UpdateRun(run *Run) error {
	var stoppedAt *string
	if run.StoppedAt != nil {
		t := run.StoppedAt.Format(time.RFC3339)
		stoppedAt = &t
	}

	_, err := s.db.Exec(`
		UPDATE runs SET status = ?, exit_code = ?, stopped_at = ?
		WHERE id = ?
	`, run.Status, run.ExitCode, stoppedAt, run.ID)
	return err
}

// DeleteRun removes a run from the database
func (s *Store) DeleteRun(runID string) error {
	_, err := s.db.Exec("DELETE FROM runs WHERE id = ?", runID)
	return err
}

// LoadJobs loads all jobs from the database
func (s *Store) LoadJobs() ([]*Job, error) {
	rows, err := s.db.Query(`
		SELECT id, command_json, command_signature, workdir, description, blocked, next_run_seq, created_at,
			run_count, success_count, failure_count, success_total_duration_ms, failure_total_duration_ms, min_duration_ms, max_duration_ms
		FROM jobs
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var (
			id                     string
			commandJSON            string
			commandSignature       string
			workdir                string
			description            sql.NullString
			blocked                int
			nextRunSeq             int
			createdAtStr           string
			runCount               int
			successCount           int
			failureCount           int
			successTotalDurationMs int64
			failureTotalDurationMs int64
			minDurationMs          sql.NullInt64
			maxDurationMs          sql.NullInt64
		)

		if err := rows.Scan(&id, &commandJSON, &commandSignature, &workdir, &description, &blocked, &nextRunSeq, &createdAtStr,
			&runCount, &successCount, &failureCount, &successTotalDurationMs, &failureTotalDurationMs, &minDurationMs, &maxDurationMs); err != nil {
			return nil, err
		}

		var command []string
		if err := json.Unmarshal([]byte(commandJSON), &command); err != nil {
			return nil, fmt.Errorf("failed to unmarshal command: %w", err)
		}

		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}

		job := &Job{
			ID:                     id,
			Command:                command,
			CommandSignature:       commandSignature,
			Workdir:                workdir,
			Description:            description.String, // Empty if NULL
			Blocked:                blocked != 0,
			NextRunSeq:             nextRunSeq,
			CreatedAt:              createdAt,
			RunCount:               runCount,
			SuccessCount:           successCount,
			FailureCount:           failureCount,
			SuccessTotalDurationMs: successTotalDurationMs,
			FailureTotalDurationMs: failureTotalDurationMs,
			MinDurationMs:          minDurationMs.Int64,
			MaxDurationMs:          maxDurationMs.Int64,
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// LoadRuns loads all runs from the database
func (s *Store) LoadRuns() ([]*Run, error) {
	rows, err := s.db.Query(`
		SELECT id, job_id, pid, status, exit_code, stdout_path, stderr_path, started_at, stopped_at
		FROM runs
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		var (
			id           string
			jobID        string
			pid          int
			status       string
			exitCode     sql.NullInt64
			stdoutPath   string
			stderrPath   string
			startedAtStr string
			stoppedAtStr sql.NullString
		)

		if err := rows.Scan(&id, &jobID, &pid, &status, &exitCode, &stdoutPath, &stderrPath, &startedAtStr, &stoppedAtStr); err != nil {
			return nil, err
		}

		startedAt, err := time.Parse(time.RFC3339, startedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse started_at: %w", err)
		}

		run := &Run{
			ID:         id,
			JobID:      jobID,
			PID:        pid,
			Status:     status,
			StdoutPath: stdoutPath,
			StderrPath: stderrPath,
			StartedAt:  startedAt,
		}

		if exitCode.Valid {
			code := int(exitCode.Int64)
			run.ExitCode = &code
		}

		if stoppedAtStr.Valid {
			stoppedAt, err := time.Parse(time.RFC3339, stoppedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("failed to parse stopped_at: %w", err)
			}
			run.StoppedAt = &stoppedAt
		}

		runs = append(runs, run)
	}

	return runs, rows.Err()
}

// OrphanRun represents a run that may need cleanup after a crash
type OrphanRun struct {
	Run     *Run
	Command []string // From joined jobs table
}

// FindOrphanRuns finds all runs marked as 'running' with their commands
func (s *Store) FindOrphanRuns() ([]*OrphanRun, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.job_id, r.pid, r.status, r.exit_code, r.stdout_path, r.stderr_path, r.started_at, r.stopped_at, j.command_json
		FROM runs r
		JOIN jobs j ON r.job_id = j.id
		WHERE r.status = 'running'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orphans []*OrphanRun
	for rows.Next() {
		var (
			id           string
			jobID        string
			pid          int
			status       string
			exitCode     sql.NullInt64
			stdoutPath   string
			stderrPath   string
			startedAtStr string
			stoppedAtStr sql.NullString
			commandJSON  string
		)

		if err := rows.Scan(&id, &jobID, &pid, &status, &exitCode, &stdoutPath, &stderrPath, &startedAtStr, &stoppedAtStr, &commandJSON); err != nil {
			return nil, err
		}

		startedAt, err := time.Parse(time.RFC3339, startedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse started_at: %w", err)
		}

		var command []string
		if err := json.Unmarshal([]byte(commandJSON), &command); err != nil {
			return nil, fmt.Errorf("failed to unmarshal command: %w", err)
		}

		run := &Run{
			ID:         id,
			JobID:      jobID,
			PID:        pid,
			Status:     status,
			StdoutPath: stdoutPath,
			StderrPath: stderrPath,
			StartedAt:  startedAt,
		}

		if exitCode.Valid {
			code := int(exitCode.Int64)
			run.ExitCode = &code
		}

		orphans = append(orphans, &OrphanRun{
			Run:     run,
			Command: command,
		})
	}

	return orphans, rows.Err()
}

// MarkRunStopped marks a run as stopped without a known exit code
func (s *Store) MarkRunStopped(runID string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE runs SET status = 'stopped', stopped_at = ?
		WHERE id = ?
	`, now, runID)
	return err
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// ProcessInfo holds information about a running process
type ProcessInfo struct {
	StartTime time.Time
	Command   []string
}

// getProcessInfo retrieves start time and command for a PID using gopsutil
func getProcessInfo(pid int) (*ProcessInfo, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err // Process doesn't exist
	}

	// Get creation time (milliseconds since epoch)
	createTimeMs, err := proc.CreateTime()
	if err != nil {
		return nil, err
	}
	startTime := time.UnixMilli(createTimeMs)

	// Get command line arguments
	cmdline, err := proc.CmdlineSlice()
	if err != nil {
		return nil, err
	}

	return &ProcessInfo{StartTime: startTime, Command: cmdline}, nil
}

// processExists checks if a process with the given PID exists
func processExists(pid int) bool {
	// Signal 0 checks if process exists without sending signal
	return syscall.Kill(pid, 0) == nil
}

// isOurProcess verifies that a PID belongs to a process we started.
// This prevents killing unrelated processes if PIDs were reused.
func isOurProcess(pid int, expectedStartTime time.Time, expectedCmd []string) bool {
	if !processExists(pid) {
		return false
	}

	info, err := getProcessInfo(pid)
	if err != nil {
		return false
	}

	// Check 1: Start time must match (within tolerance for clock differences)
	// Process start times have ~1 second granularity on most systems
	timeDiff := info.StartTime.Sub(expectedStartTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 2*time.Second {
		return false
	}

	// Check 2: Command must match (at least the executable name)
	if len(info.Command) == 0 || len(expectedCmd) == 0 {
		return false
	}
	if info.Command[0] != expectedCmd[0] {
		return false
	}

	return true
}

// nullableInt64 returns nil for zero values, otherwise the pointer
func nullableInt64(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

// nullableString returns nil for empty strings, otherwise the string
func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}
