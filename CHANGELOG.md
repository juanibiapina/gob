# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **TUI progress bar**: Shows a progress bar in the Runs panel when a job is running
  - Displays elapsed time vs average duration with Unicode gradient bar (`▓▓▓▓▓▓▓▓▒▒▒▒▒▒▒▒`)
  - Only shown when the job has historical run data (needs average duration)

- **Description in `run` output**: `gob run` now displays the job description (if set) on an indented line below the job header, matching `gob list` format

- **Stuck job detection**: `gob run` and `gob await` now detect potentially stuck jobs and return early
  - Timeout: average successful duration + 1 minute (or 5 minutes if no historical data)
  - Triggers when: elapsed time exceeds timeout AND no output for 1 minute
  - Displays stuck detection timeout when starting a job (e.g., "Stuck detection: timeout after 5m")
  - When triggered, shows helpful commands: `gob stdout`, `gob await`, `gob stop`
  - Job continues running in background - only the wait is aborted
  - Also applies to `gob start -f` and `gob restart -f`

### Changed

- **Stats by outcome**: `gob run` and `gob add` now show separate expected durations for successful and failed runs
  - Shows "Expected duration if success: ~Xm" when there are 3+ previous successful runs
  - Shows "Expected duration if failure: ~Xs" when there are 3+ previous failed runs
  - Killed processes (SIGKILL/SIGTERM) are excluded from duration statistics but still count toward total runs
  - Average duration calculation now uses only successful runs (previously used all runs)
- **Version negotiation no longer auto-restarts daemon**: When client and daemon versions mismatch, the client now returns an error instead of auto-restarting the daemon. This prevents old TUIs from restarting the daemon with their old binary, undoing upgrades. Users must now run `gob shutdown` and restart their clients when upgrading.
- **TUI quits on daemon disconnect**: The TUI no longer attempts to reconnect when the daemon stops. Instead, it quits with a message asking the user to restart. This complements the version negotiation change by preventing old TUIs from resurrecting the daemon.
- **Daemon no longer auto-shuts down when idle**: The daemon now runs indefinitely until explicitly stopped with `gob shutdown`.
- **BREAKING: Gobfile `autostart` now defaults to `false`**: Jobs in the gobfile no longer auto-start by default. Add `autostart = true` to jobs that should start when the TUI opens and stop when it exits. Jobs without `autostart` (or with `autostart = false`) are created but not started, giving you manual control over their lifecycle.

### Fixed

- **TUI disconnecting after 30 seconds of inactivity**: The daemon's subscription handler had a 30-second read timeout that was incorrectly treating timeouts as disconnections. Since the client never sends data after subscribing, the connection would be closed after 30 seconds of no events, causing the TUI to quit. Removed the timeout so the connection now blocks indefinitely until the client actually disconnects.
- **Runs panel column misalignment**: Fixed status column padding using byte length instead of visual width, causing misaligned columns when Unicode status icons (✓, ◉, ◼) were displayed.
- **Gobfile descriptions not displayed**: Fixed job descriptions from gobfile not appearing in TUI when job already existed without a description.
- **Gobfile auto-stop killing manually started jobs**: Jobs with `autostart = false` in the gobfile are no longer stopped when the TUI exits. Previously, if you defined a job with `autostart = false` and manually started it, it would be killed on TUI exit. Now only jobs with `autostart = true` are auto-stopped.

## [3.0.0-rc.2] - 2026-01-25

### Changed

- **Idempotent `add` and `run` commands**: These commands no longer error when a job is already running
  - `gob add` returns success with "Job abc already running (since 2m ago)" message
  - `gob run` returns success with "Job abc already running (since 2m ago), attaching..." and follows output
  - Both commands update the job's description if a different one is provided
  - This improves UX and allows gobfiles to sync descriptions without requiring job restarts

### Added

- **`job_updated` event**: New event type emitted when a job's metadata changes (e.g., description update for running job)
  - TUI automatically refreshes job descriptions when this event is received

## [3.0.0-rc.1] - 2026-01-25

**Breaking change:** Gobfile format changed from plain text to TOML (`.config/gobfile` → `.config/gobfile.toml`).

### Added

- **Job descriptions**: Jobs can now have descriptions that provide context for AI agents and users
  - Add via CLI: `gob add --description "Dev server" npm run dev`
  - Run via CLI: `gob run --description "Build" make build`
  - Add via gobfile: `description = "Dev server"`
  - Displayed in `gob list` output (indented below job)
  - Displayed in TUI as a dedicated panel (when selected job has description)
  - Descriptions are updated when running the same command with a new description
  - Running without `--description` preserves the existing description

- **TOML gobfile format**: Gobfile now uses TOML format (`.config/gobfile.toml`) with support for:
  - `command`: The command to run (required)
  - `description`: Context for AI agents and users (optional)
  - `autostart`: Whether to start on TUI launch, defaults to true (optional)
  - Jobs with `autostart = false` are added but not started

### Fixed

- **Restarted job showing old exit code**: Fixed a race condition where a restarted job could briefly appear as "stopped" with the previous run's exit code. This happened when the old run's cleanup goroutine ran after the new run had started, incorrectly clearing the job's current run reference.

### Changed

- **Gobfile location**: Changed from `.config/gobfile` to `.config/gobfile.toml`

## [2.3.0] - 2026-01-23

### Added

- **Gobfile auto-start/stop**: TUI automatically starts jobs listed in `.config/gobfile` on launch and stops them on exit
  - One command per line in the gobfile
  - Already-running jobs are skipped (not restarted)
  - Stopped jobs matching gobfile commands are started
  - Handles SIGHUP for cleanup when terminal/tmux pane is killed

### Changed

- **AI agent instructions**: Updated to recommend selective gob usage - only for servers, long-running commands, and builds. Quick commands like `git status` should run directly without gob.

### Removed

- **MCP server**: Removed the Model Context Protocol server (`gob mcp` command) and all related functionality. AI agents should use the CLI directly instead.

## [2.2.2] - 2026-01-09

### Fixed

- **TUI: Fix log flicker when switching jobs**: When switching between jobs with j/k keys, the log panel no longer briefly shows logs from an old run. Removed legacy `StdoutPath`/`StderrPath` fields from TUI's Job struct that became stale when jobs were restarted. Logs are now cleared immediately when switching jobs and loaded after runs are fetched.

## [2.2.1] - 2025-12-19

### Added

- Repo is now a nix flake

### Fixed

- **TUI: New jobs now appear in the job list**: Fixed a bug where jobs added via the TUI (pressing 'N') wouldn't appear in the job list due to incorrect scroll state management. The new job is now selected and visible immediately.

## [2.2.0] - 2025-12-16

### Changed

- **TUI visual refresh**:
  - Updated panel styling with new color scheme
  - Add info panel showing directory and version
  - Remove top banners
  - Runs panel columns now have fixed percentage-based widths with proper truncation
  - fix: Remove daemon restart message that was printed to stdout, corrupting the TUI display
  - Help and new job modals now render on top of the existing TUI content instead of hiding the entire background
- **TUI scrolling for Jobs and Runs panels**: Jobs and Runs panels now scroll when the list exceeds the visible area, matching the existing Ports panel behavior. Scroll logic refactored into reusable `ScrollState` abstraction.

### Fixed

- **TUI ports panel scrolling**: Fixed off-by-one scroll timing where the panel would scroll after the cursor was already one line out of view instead of when about to leave the visible area

## [2.1.1] - 2025-12-16

### Fixed

- **Telemetry no longer prints errors to stderr**: When telemetry requests fail (e.g., blocked by pihole, network issues), errors are now silently discarded instead of being logged. This prevents TUI corruption and unwanted console output for users who block analytics.

## [2.1.0] - 2025-12-16

### Added

- **Port tracking**: Track listening ports for running jobs and their child processes
  - `ports [job_id]` command - list listening ports for a job or all running jobs
  - TUI: New Ports panel (panel 2) showing listening ports for selected job
  - Daemon polls ports at 2s, 5s, 10s after job starts and emits `ports_updated` events
  - Supports Linux and macOS

### Changed

- **Process tree verification on stop**: Stop commands now verify that all child processes terminate, not just the parent
  - `stop`, `restart`, and `shutdown` now snapshot the entire process tree before signaling
  - After SIGKILL, survivors are killed individually (handles processes that escaped the process group)
  - Returns detailed error with surviving PIDs if processes cannot be terminated

## [2.0.4] - 2025-12-15

### Changed

- **Job ordering**: Jobs are now ordered by most recent run everywhere (CLI, TUI), not just when a job starts in the TUI

## [2.0.3] - 2025-12-15

### Changed

- **TUI job ordering**: Jobs are now ordered by most recent run, so whenever a job starts running it moves to the top of the list

## [2.0.2] - 2025-12-14

### Added

- `run` command - Add a job and wait for completion in one command
  - Combines `add` + `await` for convenience
  - Shows stats from previous runs, streams output, exits with job's exit code
- **TUI horizontal scrolling**: Navigate long log lines with `h`/`l` keys in log panels, or `H`/`L` from jobs/runs panels
- **TUI line wrap toggle**: Press `w` to toggle between truncated and wrapped log display

### Fixed

- **TUI rendering corruption**: Log output containing cursor movement sequences (from progress bars, spinners) no longer breaks the TUI display

## [2.0.1] - 2025-12-14

### Added

- **Environment passing**: Jobs now run with the client's environment, not the daemon's
  - `add`, `start`, and `restart` capture the client's environment and pass it to the job
  - Jobs run in a clean environment containing only what the client provides
  - See `docs/environment.md` for details

## [2.0.0] - 2025-12-12

**Jobs are now persistent entities, not ephemeral processes.**

Previously, each `gob add make test` created a new, independent job. Now, gob recognizes that you're running the same command repeatedly and tracks it as a single job with multiple runs. This unlocks:

- **Execution history**: See all previous runs of a command with `gob runs <id>`
- **Statistics**: Success rates, average duration, time estimates with `gob stats <id>`
- **Smarter feedback**: `gob add` tells you what to expect based on history
- **Historical logs**: Browse and compare output from previous runs in the TUI

The mental model shift: a "job" is now "a command you run repeatedly" rather than "a process running in the background". Each execution is a "run" of that job.

**Jobs now persist across daemon restarts.**

Job state is stored in a SQLite database and survives daemon crashes or restarts. When the daemon starts:
- Previously stopped jobs are restored (can be restarted with `gob start`)
- Orphaned processes from crashes are detected and cleaned up
- The daemon exits cleanly when idle (no running jobs) for 5 minutes, but jobs remain in the database

### Added

- **Job persistence**: Jobs survive daemon restarts via SQLite database
- **Crash recovery**: Daemon detects unclean shutdowns and kills orphaned processes
- `runs <job_id>` command - show run history for a job
- `stats <job_id>` command - show statistics (success rate, avg/min/max duration)
- `add` shows previous run count, success rate, and expected duration for repeat jobs
- TUI: Dedicated Runs panel showing run history for selected job
  - Panel navigation: 1=Jobs, 2=Runs, 3=Stdout, 4=Stderr
  - Select historical runs to view their logs
- Version negotiation: daemon auto-restarts when CLI version changes
  - If no running jobs, restart happens automatically
  - If jobs are running, commands are blocked with guidance to run `gob shutdown`
  - `shutdown` command always works (bypasses version check)

### Changed

- **Idle shutdown**: Daemon now exits when no *running* jobs for 5 minutes (stopped jobs persist)
- **Log location**: Job logs moved from `$XDG_RUNTIME_DIR/gob/` to `$XDG_STATE_HOME/gob/logs/` for persistence across reboots
- **Database location**: Job state stored in `$XDG_STATE_HOME/gob/state.db`
- `add` reuses existing job for same command in same directory (creates new run)
- `add` no longer requires `--` separator for commands with flags
- `add` supports quoted command strings (e.g., `gob add "make test"`)
- `signal` requires job to be running (returns error for stopped jobs)
- **`nuke` renamed to `shutdown`**: Now only stops running jobs and shuts down daemon (no longer removes jobs or logs)
- `shutdown` always operates on all jobs (removed `--all` flag)
- Removed `-f/--follow` flag from `add` (use `gob add` + `gob await`)

### Removed

- `cleanup` command - use `gob remove <id>` to remove individual jobs
- `nuke` command - replaced by `shutdown` (which preserves job history)
- `run` command - use `gob add` + `gob await`

### Fixed

- `shutdown` (formerly `nuke`) now properly shuts down the daemon after stopping all jobs
- Daemon now properly daemonizes with PPID=1 using go-daemon library (was incorrectly keeping parent PID)

## [1.2.3] - 2025-12-09

### Added

- CLI telemetry now includes command duration (`duration_ms`)

### Fixed

- Exclude `tui` command from CLI telemetry (has its own session/action telemetry)

## [1.2.2] - 2025-12-09

### Fixed

- Exclude `__complete` command from telemetry (Cobra's internal completion handler)

## [1.2.1] - 2025-12-09

### Fixed

- Telemetry no longer sent on shell completions

## [1.2.0] - 2025-12-08

### Added

- Anonymous telemetry to understand usage patterns
  - Tracks CLI commands and TUI actions
  - Disabled with `GOB_TELEMETRY_DISABLED=1` or `DO_NOT_TRACK=1`
  - See `docs/telemetry.md` for details

## [1.0.0] - 2025-12-06

### Added

- `await-any` command to wait for any running job to complete
  - Shows list of jobs being watched, then waits for first to finish
  - Displays completion summary (command, duration, exit code) and remaining jobs
  - `--timeout` flag to give up after N seconds (exits with code 124)
  - Exits with the completed job's exit code
- `await-all` command to wait for all running jobs to complete
  - Shows list of jobs being watched, then waits for all to finish
  - Displays brief status for each job as it completes
  - Shows final summary with succeeded/failed counts
  - `--timeout` flag to give up after N seconds (exits with code 124)
  - Exits with the first non-zero exit code, or 0 if all succeeded
- TUI: `i` key to toggle expanded job details view
  - Default view shows just status symbol and command
  - Expanded view adds job ID, PID, workdir, and timing info

### Changed

- `add` command now shows follow-up hints (`gob await` and `gob stop`) after adding a job
- TUI: Simplified default job list to show only status symbol and command

## [0.14.0] - 2025-12-06

### Added

- `await` command to wait for a job to complete, streaming output in real-time and showing a summary with command, duration, and exit code
  - For running jobs: streams stdout/stderr until completion
  - For stopped jobs: displays existing output then shows summary
  - Exits with the job's exit code

## [0.13.0] - 2025-12-05

### Added

- Job duration tracking: jobs now track start and stop times
  - TUI shows duration in stdout panel title (e.g., `2 stdout: ABC ◉ 5m30s`)
  - Running jobs show live duration, stopped jobs show total run time
  - `gob list --json` includes `started_at` and `stopped_at` fields
- Exit code tracking: jobs now capture and display exit codes
  - `gob list` shows exit codes: `stopped (0): cmd` for success, `stopped (1): cmd` for failure
  - `gob list --json` includes `exit_code` field
  - `gob run` returns the job's exit code as its own exit code (useful for CI/scripts)
  - `gob logs` shows exit code in "process stopped" message
- TUI: Semantic status symbols with colors
  - `◉` (green) - Running
  - `✓` (green) - Success (exit code 0)
  - `✗` (red) - Failed (non-zero exit code)
  - `◼` (gray) - Stopped/killed (no exit code)
- Daemon auto-shutdown: daemon exits after 5 minutes of inactivity (no jobs)
- TUI: `f` key in jobs panel to toggle follow mode for log panels without switching focus

### Changed

- UI colors now use terminal theme colors (ANSI 0-15) instead of hardcoded hex values, adapting to the user's color scheme
- `gob list --json` now includes `stdout_path` and `stderr_path` fields (useful for `tail -f`)
- Job IDs shortened from 7 characters to 3 characters (e.g., `abc` instead of `V3x0QqI`)
- System log prefix changed from `[monitor]` to `[gob]` (matches new 3-char job ID length)
- Use `github.com/adrg/xdg` package for runtime directory resolution instead of manual `XDG_RUNTIME_DIR` handling

### Fixed

- TUI: First job now appears selected when added to an empty job list
- Client reconnection: Fixed broken pipe errors when making multiple requests (daemon closes connections after each response)
- TUI: Selection background now spans entire line while preserving element colors

## [0.12.0] - 2025-12-05

### Changed

- **Backend rewrite**: Replaced detached process model with a tmux-style daemon architecture. Jobs are now children of the daemon instead of orphaned processes, enabling better lifetime control, reliable status tracking, and real-time multi-client updates. All commands work the same, but the foundation is now in place for upcoming features.
- Log files now stored in `$XDG_RUNTIME_DIR/gob/` (previously `$XDG_DATA_HOME/gob/`)

## [0.11.0] - 2025-12-04

### Added

- TUI: `c` key to copy selected job's command to clipboard

## [0.10.0] - 2025-12-01

### Added

- TUI: `J`/`K` (Shift+j/k) to scroll stdout panel while jobs panel is focused

### Changed

- TUI now shows `[following]` indicator on log panels regardless of which panel is focused

## [0.9.2] - 2025-12-01

### Changed

- `start` and `restart` commands now clear previous logs before starting, consistent with `run` command behavior

## [0.9.1] - 2025-12-01

### Changed

- `run` command now clears previous logs when reusing a stopped job, so you only see output from the current run
- TUI restart command now clears previous logs before restarting, consistent with `run` command behavior

## [0.8.0] - 2025-11-30

### Changed

- Job metadata now includes `created_at` timestamp for explicit sorting (decoupled from ID format)
- `run` command no longer requires `--` separator for commands with flags (e.g., `gob run pnpm --filter web typecheck`)
- `run` command now supports quoted command strings (e.g., `gob run "make test"`)

## [0.7.0] - 2025-11-30

### Added

- `run` command to run a command and follow output until completion, with smart job reuse
- `add` command to create and start new background jobs
- `-f/--follow` flag for `add`, `start`, and `restart` commands to follow output until completion

### Changed

- **BREAKING**: Reorganized commands into clearer groups:
  - Job Management: `add`, `remove`, `cleanup`, `nuke`
  - Process Control: `start`, `stop`, `restart`, `signal`
  - Convenience: `run`
- **BREAKING**: `start` command now starts a stopped job by ID (errors if already running)
- **BREAKING**: Use `add` instead of `start` to create new jobs
- `run` command reuses existing stopped job with same command+args
- Updated overview with new command grouping
- Improved README with AI agent instructions for when to use `run` vs `add`

## [0.6.0] - 2025-11-29

### Added

- Full-screen TUI (`gob tui`) for interactive job management

## [0.5.1] - 2025-11-29

### Changed

- Monitor messages now show command first, with pid and job id in parentheses: `process started: ./my-server (pid:12345 id:abc)`

## [0.5.0] - 2025-11-29

### Added

- `logs` command to follow combined stdout and stderr output in real-time with job ID prefix (orange for stderr)
- Dynamic job detection in `logs` command - automatically picks up new jobs that start while running
- System logging in `logs` command with `[monitor]` prefix (cyan) for process lifecycle events:
  - "process started" when new jobs are detected dynamically
  - "process stopped" when a tracked process terminates
  - "all processes stopped" when no more processes are running
  - "waiting for jobs..." when no jobs exist yet

### Changed

- **BREAKING**: `logs` command no longer accepts a job ID argument - it now only follows all jobs in the current directory
- **BREAKING**: Job IDs changed from 19-digit nanosecond timestamps to 7-character base62-encoded IDs (e.g., `V3x0QqI`)
- Simplified overview output to a concise reference card
- Replace external `tail` command with pure Go implementation for `--follow` flag

### Fixed

- Fix typos in help text examples (missing space between command and job ID)

## [0.4.0] - 2025-11-28

### Added

- Shell completion support for Bash, Zsh, Fish, and PowerShell with dynamic job ID suggestions

## [0.3.0] - 2025-11-21

### Added

- `--all` flag to `list`, `cleanup`, and `nuke` commands to operate across all directories
- `--workdir` flag to `list` command to display working directory for each job

### Changed

- **BREAKING**: Storage migrated to XDG-compliant directory (`~/Library/Application Support/gob` on macOS, `~/.local/share/gob` on Linux)
- Job IDs now use nanosecond precision timestamps instead of second precision for better uniqueness
- `nuke` command now removes log files in addition to metadata files

### Migration Notes

- Existing jobs in project-local `.local/share/gob` directories will no longer be visible after upgrade
- Old jobs continue running but won't appear in `gob list`
- Users should manually clean up old `.local` directories in projects when ready

## [0.2.0] - 2025-11-20

### Fixed

- Fix process hanging by ensuring child processes are properly terminated

## [0.1.1] - 2025-11-20

- Add support for installing with homebrew

## [0.1.0] - 2025-11-17

First stable release.

## [0.1.0-beta.1] - 2025-11-17

Initial test release.
