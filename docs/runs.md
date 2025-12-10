# Runs

## Problem Statement

When users run the same command multiple times (e.g., `make test`), each execution is independent with no relationship to previous runs. This loses valuable information:

- No history of previous runs
- No duration statistics or time estimation
- No success/failure rate tracking
- No easy way to compare current logs with previous runs

## Goals

1. Track history of runs for the same command
2. Provide statistics (average duration, success rate)
3. Estimate duration for next run based on history
4. Easy access to current and previous logs
5. Maintain good usability in both CLI and TUI

## Non-Goals

- Cross-directory history by default
- Changing the user-facing concept of "job"
- Parallel runs of the same job

## Design

### Core Concept

**Job** = A command you run repeatedly (e.g., "make test")  
**Run** = A single execution of that job (internal concept, not user-facing)

Jobs are the user-facing identifier. Runs track execution history internally.

### Single Run Model

A job can have **at most one run executing at a time**:

```bash
gob add make test    # Job abc created, run starts
gob add make test    # Error: job abc is already running
# ... wait for job to finish ...
gob add make test    # New run starts under same job
```

This keeps the mental model simple: one job = one process (running or not).

### User-Facing: Job IDs Only

Users interact exclusively with Job IDs. Run IDs are internal implementation details used for:
- Log file storage paths
- Database keys for history
- TUI historical run navigation

| Command | Behavior |
|---------|----------|
| `stop abc` | Stop the running job |
| `await abc` | Wait for the job to complete |
| `signal abc TERM` | Signal the running job |
| `stdout abc` | Show current/latest run's stdout |
| `stderr abc` | Show current/latest run's stderr |
| `start abc` | Start new run (error if already running) |
| `restart abc` | Stop (if running) + start new run |
| `remove abc` | Remove job and all its history |
| `runs abc` | List run history |
| `stats abc` | Show statistics |

### Data Model

```
Job {
  id: string              // user-facing identifier (e.g., "abc")
  command: []string       // the command + args
  command_signature: string // hash for lookups
  workdir: string         // directory scope
  current_run_id: *string // nil if not running, points to active run
  next_run_seq: int       // counter for internal run IDs
  
  // Cached statistics (updated on run completion)
  run_count: int
  success_count: int
  total_duration_ms: int64
  min_duration_ms: int64
  max_duration_ms: int64
}

Run {
  id: string              // internal identifier (e.g., "abc-1", "abc-2")
  job_id: string          // reference to Job
  pid: int                // process ID (0 if stopped)
  status: string          // "running" | "stopped"
  exit_code: *int         // nil if running or killed
  stdout_path: string     // path to stdout log
  stderr_path: string     // path to stderr log
  started_at: timestamp
  stopped_at: *timestamp  // nil if running
}
```

**On `gob add <cmd>`:**
1. Compute `command_signature` from command array
2. Look up existing Job by (signature, workdir)
3. If found and running → error "job is already running"
4. If found and not running → create new Run, start process
5. If not found → create new Job, create Run, start process

**On run completion:**
1. Update Run (exit_code, stopped_at)
2. Clear Job's current_run_id
3. Update Job's cached statistics

### CLI Interface

**Principle: Backwards Compatible**

CLI output formats unchanged. All commands use Job IDs only.

#### Adding Jobs

```bash
gob add make test        # Creates Job abc + starts run
gob add make test        # Error if abc still running, else starts new run
gob add make build       # Creates Job def + starts run
```

When a job has previous runs, `add` returns statistics alongside the job ID:
- Expected duration (based on average of previous runs)
- Success rate
- Run count

This lets users know what to expect without running a separate `stats` command.

#### Listing

```bash
# Output unchanged - shows job status
gob list
# abc  running: make test
# def  stopped (0): make build

# List run history for a job
gob runs abc
# Run    Started      Duration  Status
# abc-5  2 min ago    running   ◉
# abc-4  1 hour ago   2m15s     ✓ (0)
# abc-3  2 hours ago  2m45s     ✗ (1)
```

#### Process Control

```bash
gob start abc            # Start new run (error if running)
gob stop abc             # Stop running job
gob restart abc          # Stop (if running) + start new run
gob signal abc TERM      # Signal running job
gob await abc            # Wait for job to complete
```

#### Batch Operations

```bash
gob await-any            # Wait for any job to complete
gob await-all            # Wait for all jobs to complete
gob logs                 # Follow output from all running jobs
```

#### Viewing Output

```bash
gob stdout abc           # Current/latest run's stdout
gob stderr abc           # Current/latest run's stderr
gob logs                 # Follow all output from all running jobs
```

Historical run output is accessible via TUI only (see TUI section).

#### Statistics

```bash
gob stats abc
# Job: abc (make test)
# Total runs: 5
# Success rate: 80% (4/5)
# Average duration: 2m30s
# Fastest: 2m15s
# Slowest: 2m45s
# Estimated next run: ~2m30s
```

#### Removing Jobs

```bash
gob remove abc           # Remove Job and all its run history
```

- Only whole Jobs can be removed
- Individual runs are valuable history for statistics
- Use `nuke` for emergency full reset

### TUI Interface

#### Default View: Unchanged

The default view looks and works exactly like today:

```
┌─ Jobs ─────────────────────────────────────────────────────────────┐
│ ◉ abc  make test                                                   │
│ ✓ def  make build                                                  │
│ ✗ ghi  npm test                                                    │
└────────────────────────────────────────────────────────────────────┘
```

Log panels show the current/latest run's output. All existing key bindings work unchanged.

#### Expanded View with `i`: Run History

The `i` toggle shows **run history** for the selected job:

```
┌─ Jobs ─────────────────────────────────────────────────────────────┐
│ ◉ abc  make test                                                   │
│   5 runs | 80% success | avg: 2m30s | est: ~2m30s                  │
│   > abc-5  2min ago   running  ◉                                   │
│     abc-4  1hr ago    2m15s    ✓ (0)                               │
│     abc-3  2hr ago    2m45s    ✗ (1)                               │
│     abc-2  3hr ago    2m30s    ✓ (0)                               │
│ ✓ def  make build                                                  │
│ ✗ ghi  npm test                                                    │
└────────────────────────────────────────────────────────────────────┘
```

When expanded:
- Navigate runs with `j/k`
- Select a run to view its logs in the log panels
- Stats shown inline (success rate, avg duration, estimate)

#### Key Bindings

| Key | Action |
|-----|--------|
| `i` | Toggle expanded view (shows run history for selected job) |
| `j/k` | Navigate jobs (or runs when expanded) |
| `s` | Start new run (error if already running) |
| `x` | Stop current run |
| `r` | Restart (stop + start new run) |
| `d` | Remove Job and all its run history |

#### Viewing Historical Run Logs

With expanded view (`i`), select a historical run to view its logs:

```
┌─ Jobs ─────────────────────────────────────────────────────────────┐
│ ◉ abc  make test                                                   │
│   5 runs | 80% success | avg: 2m30s                                │
│     abc-5  2min ago   running  ◉                                   │
│     abc-4  1hr ago    2m15s    ✓ (0)                               │
│   > abc-3  2hr ago    2m45s    ✗ (1)    <-- selected               │
│     abc-2  3hr ago    2m30s    ✓ (0)                               │
│ ✓ def  make build                                                  │
├─ stdout: abc-3 (historical) ───────────────────────────────────────┤
│ Running tests...                                                    │
│ ✓ test_login                                                        │
│ ✗ test_logout - FAILED                                              │
│   Expected: 200, Got: 500                                           │
└─────────────────────────────────────────────────────────────────────┘
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `gob_add` | Find or create Job, start new run (error if running) |
| `gob_list` | List Jobs with status |
| `gob_runs` | List run history for a Job |
| `gob_start` | Start new run (error if running) |
| `gob_stop` | Stop running job |
| `gob_restart` | Stop (if running) + start new run |
| `gob_await` | Wait for job to complete |
| `gob_stdout` | Get current/latest run's stdout |
| `gob_stderr` | Get current/latest run's stderr |
| `gob_stats` | Get statistics for a Job |
| `gob_remove` | Remove Job and all run history |

## Open Questions

### 1. Run ID Format

**Decision:** `abc-1`, `abc-2` (job ID + sequence number) - internal only

- Job tracks `next_run_seq` counter
- On new run: `run.ID = fmt.Sprintf("%s-%d", job.ID, job.NextRunSeq++)`
- Users never need to type these; shown only in TUI history view

### 2. What Does `gob list` Show?

**Decision:** Backwards compatible - same output as today.

- `gob list` - shows jobs with current/latest run status
- `gob runs <job_id>` - shows run history (new command)

### 3. Backwards Compatibility

**Decision:** Output formats backwards compatible, some behavior changes.

- `gob list` output format unchanged
- `gob add` now reuses existing Job (creates new run if not running)
- `gob add` errors if job is already running (no parallel runs)
- New commands: `gob runs`, `gob stats`
- See "Version & Migration" section for upgrade path

### 4. Run Retention

**Decision:** Keep all runs forever.

Every run is valuable history for statistics. Use `gob remove <job_id>` to remove a job and all its runs if needed.

### 5. Cross-Directory Jobs

Should `make test` in `/project-a` and `/project-b` be the same Job?

**Recommendation:** No - Jobs scoped by workdir (consistent with current behavior).

## Version & Migration

This feature requires **gob 2.0** (breaking change).

### Upgrade Path

For 2.0, users must kill the old daemon before using the new CLI:

```bash
gob nuke --all     # stops all jobs + shuts down daemon
```

The old daemon doesn't support the new Job/Run data model. Running jobs will be lost on upgrade.

### Future: Daemon Persistence

In a future version, we'll add:
- Job/Run persistence to disk (survives daemon restart)
- Version tracking in daemon protocol
- Automatic daemon restart on version mismatch
- Migration of old data to new format

For now, this is marked as "to be figured out" - the 2.0 release will document the manual upgrade step.

## Implementation Plan

**Note:** Each phase must include updated/reworked tests and pass CI before moving to the next phase.

### Prework: Fix `nuke` to shutdown daemon
- [x] Bug: `nuke` stops jobs but doesn't kill daemon
- [x] `client.Shutdown()` exists but isn't called
- [x] Update `nuke` to call `client.Shutdown()` after removing jobs
- [x] Required for clean 2.0 upgrade path
- [x] Update tests, CI must pass

### Phase 1: Data Model
- [x] Create `Run` struct (extracted from current Job)
- [x] Update `Job` to contain stats + `current_run_id` pointer
- [x] Implement `command_signature` hashing
- [x] Update daemon to maintain Jobs and Runs in memory
- [x] Update tests, CI must pass

**Implementation notes:**
- Log files now use run ID format: `{job_id}-{run_seq}.stdout.log` (e.g., `abc-1.stdout.log`)
- Each run gets its own log files (preserved for TUI history view)
- `signal` command now requires job to be running (returns error for stopped jobs)
- Phases 1 & 2 were implemented together since data model changes required behavior changes

### Phase 2: Core Behavior Changes
- [x] `add`: find or create Job, error if running, else create new Run
- [x] `start`: create new Run under existing Job (error if running)
- [x] `restart`: stop current run (if any) + start new Run
- [x] `stop`: stop the running job
- [x] `signal`: signal the running job (now errors if job not running)
- [x] `await`: wait for job to complete (no changes needed)
- [x] Update all daemon handlers
- [x] Update tests, CI must pass

### Phase 3: CLI Updates
- [x] `list`: output unchanged (shows jobs with current/latest run status)
- [x] `runs <job_id>`: show run history (new command)
- [x] `stdout/stderr`: show current/latest run's output (no changes needed)
- [x] `stats <job_id>`: show statistics (new command)
- [x] `await-any`: wait for any job to complete (no changes needed)
- [x] `await-all`: wait for all jobs to complete (no changes needed)
- [x] `logs`: follows all running jobs (no changes needed)
- [x] Update help text and examples
- [x] Update tests, CI must pass

**Implementation notes:**
- Reused existing `formatDuration()` from `cmd/await.go` for consistent duration formatting
- Phase 5 (MCP tools) was completed as part of Phase 3 due to CLI-MCP parity test requirement
- Added `RunResponse` and `StatsResponse` to protocol for clean API separation
- `runs` output shows: run ID, relative time, duration, status indicator (◉/✓/✗)
- `stats` output shows: run count, success rate, avg/min/max duration, estimated next run

### Phase 4: TUI Updates
- [ ] Default view unchanged (shows jobs with current/latest run status)
- [ ] Repurpose `i` toggle: show run history instead of job details
- [ ] Navigate runs within expanded view
- [ ] Log panels show selected run's output
- [ ] Update status indicators for historical runs
- [ ] Update tests, CI must pass

### Phase 5: MCP Updates
- [x] Update all tools for Job/Run model (done in Phase 1)
- [x] Add `gob_runs` tool
- [x] Add `gob_stats` tool
- [x] Update tests, CI must pass

**Implementation notes:**
- Completed as part of Phase 3 (CLI-MCP parity test enforces this)

### Phase 6: Remove `run` and `cleanup` Commands
- [x] Remove `cmd/run.go` and tests
- [x] Remove `cmd/cleanup.go` and tests
- [x] Update documentation
- [x] Remove `client.Run()` from daemon
- [x] Remove `gob_cleanup` MCP tool
- [x] Update tests, CI must pass

## Future Enhancements

- **Notifications:** Alert when a run takes unusually long
- **Trends:** Show if success rate is improving or degrading
- **Comparison:** Side-by-side diff of logs between runs
- **Pinned Runs:** Keep specific runs from being cleaned up
- **Export:** Export history/stats to JSON/CSV
