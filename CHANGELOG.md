# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

- Monitor messages now show command first, with pid and job id in parentheses: `process started: ./my-server (pid:12345 id:V3x0QqI)`

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
