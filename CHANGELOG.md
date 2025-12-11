# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- `gob_runs` and `gob_stats` MCP tools
- TUI: Dedicated Runs panel showing run history for selected job
  - Panel navigation: 1=Jobs, 2=Runs, 3=Stdout, 4=Stderr
  - Select historical runs to view their logs
- Version negotiation: daemon auto-restarts when CLI version changes
  - If no running jobs, restart happens automatically
  - If jobs are running, commands are blocked with guidance to run `gob nuke`
  - `nuke` command always works (bypasses version check)

### Changed

- **Idle shutdown**: Daemon now exits when no *running* jobs for 5 minutes (stopped jobs persist)
- **Log location**: Job logs moved from `$XDG_RUNTIME_DIR/gob/` to `$XDG_STATE_HOME/gob/logs/` for persistence across reboots
- **Database location**: Job state stored in `$XDG_STATE_HOME/gob/state.db`
- `add` reuses existing job for same command in same directory (creates new run)
- `add` errors if same command is already running (no parallel runs of same job)
- `add` no longer requires `--` separator for commands with flags
- `add` supports quoted command strings (e.g., `gob add "make test"`)
- `signal` requires job to be running (returns error for stopped jobs)
- `nuke` always operates on all jobs (removed `--all` flag)
- Removed `-f/--follow` flag from `add` (use `gob add` + `gob await`)

### Removed

- `cleanup` command - use `gob remove <id>` or `gob nuke`
- `run` command - use `gob add` + `gob await`
- `gob_cleanup` and `gob_nuke` MCP tools

### Fixed

- `nuke` now properly shuts down the daemon after removing all jobs

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
  - Tracks CLI commands, MCP tool calls, and TUI actions
  - Disabled with `GOB_TELEMETRY_DISABLED=1` or `DO_NOT_TRACK=1`
  - See `docs/telemetry.md` for details

## [1.1.0] - 2025-12-08

### Added

- MCP (Model Context Protocol) server for AI agent integration (`gob mcp`)
  - 12 tools: `gob_add`, `gob_list`, `gob_stop`, `gob_start`, `gob_remove`, `gob_restart`, `gob_signal`, `gob_await`, `gob_await_any`, `gob_await_all`, `gob_stdout`, `gob_stderr`
  - All tools filter by current directory by default
  - Compatible with Claude Code and other MCP clients

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
