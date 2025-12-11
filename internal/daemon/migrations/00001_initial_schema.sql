-- +goose Up
-- Note: PRAGMA statements are set in db.go after opening the connection
-- because they cannot be run inside transactions.

-- Instance tracking for crash detection
CREATE TABLE daemon_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Keys: 'instance_id', 'last_heartbeat', 'shutdown_clean'

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    command_json TEXT NOT NULL,      -- JSON array of command args
    command_signature TEXT NOT NULL,
    workdir TEXT NOT NULL,
    next_run_seq INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,        -- RFC3339

    -- Cached statistics
    run_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    total_duration_ms INTEGER NOT NULL DEFAULT 0,
    min_duration_ms INTEGER,
    max_duration_ms INTEGER,

    UNIQUE(command_signature, workdir)
);

CREATE TABLE runs (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    pid INTEGER NOT NULL,
    status TEXT NOT NULL,            -- 'running', 'stopped'
    exit_code INTEGER,               -- NULL if running or killed
    stdout_path TEXT NOT NULL,
    stderr_path TEXT NOT NULL,
    started_at TEXT NOT NULL,        -- RFC3339
    stopped_at TEXT,                 -- RFC3339

    -- For orphan detection and PID verification
    daemon_instance_id TEXT NOT NULL
    -- Note: command retrieved via JOIN to jobs table for process verification
);

CREATE INDEX idx_runs_job_id ON runs(job_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_runs_job_status ON runs(job_id, status);

-- Note: "current run" for a job is derived via query, not stored as FK.
-- This avoids denormalization and simplifies crash recovery.
-- Query: SELECT * FROM runs WHERE job_id=? AND status='running' ORDER BY started_at DESC LIMIT 1

-- +goose Down
DROP INDEX idx_runs_job_status;
DROP INDEX idx_runs_status;
DROP INDEX idx_runs_job_id;
DROP TABLE runs;
DROP TABLE jobs;
DROP TABLE daemon_state;
