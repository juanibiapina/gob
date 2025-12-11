# Daemon Architecture

## Overview

`gob` uses a daemon-based architecture similar to tmux. The daemon runs in the background and manages all job processes, while CLI commands and the TUI act as clients that communicate with the daemon via a Unix socket.

**Platform Support:** Unix-like systems only (Linux, macOS, BSD). Windows is not supported due to reliance on Unix sockets, signals, process groups, and setsid.

## Components

### Daemon Process

The daemon is a long-running background process that:

- **Manages all job processes**: Creates, monitors, and terminates jobs
- **Maintains job state**: Persists job metadata to SQLite database
- **Listens on Unix socket**: Accepts connections from multiple clients simultaneously
- **Handles client requests**: Processes commands (add, stop, list, etc.) and sends responses
- **Broadcasts events**: Notifies subscribed clients of job state changes
- **Manages job output**: Writes stdout/stderr to log files
- **Handles crash recovery**: Detects unclean shutdowns and cleans up orphaned processes

The daemon is an internal implementation detail—users interact only with regular commands (`add`, `list`, etc.), which handle daemon lifecycle transparently.

### Client Commands

All commands (`add`, `list`, `stop`, etc.) are clients that:

1. Check if daemon is running (probe the socket)
2. Auto-start daemon if not running
3. Connect to Unix socket
4. Send request and receive response
5. Close connection (or keep open for event subscriptions)

## File Locations

Files are stored in two locations based on their persistence requirements. Path resolution uses the [adrg/xdg](https://github.com/adrg/xdg) library—see [`internal/daemon/paths.go`](../internal/daemon/paths.go) for implementation.

### Runtime Files (`$XDG_RUNTIME_DIR/gob/`)

Ephemeral files that don't need to survive reboots:

| File | Path |
|------|------|
| Unix socket | `daemon.sock` |
| PID file | `daemon.pid` |

### State Files (`$XDG_STATE_HOME/gob/`)

Persistent files that survive reboots:

| File | Path |
|------|------|
| SQLite database | `state.db` |
| Daemon log | `daemon.log` |
| Job stdout | `logs/{job_id}-{run_seq}.stdout.log` |
| Job stderr | `logs/{job_id}-{run_seq}.stderr.log` |

## Communication Protocol

See [`internal/daemon/protocol.go`](../internal/daemon/protocol.go) for the full protocol specification, including request types, response formats, and event types.

## Multiple Clients

The daemon handles multiple simultaneous clients:

- **TUI + CLI**: TUI subscribes to events; CLI commands trigger state changes that broadcast to the TUI
- **Multiple TUIs**: All stay in sync via event broadcasts
- **Event-driven updates**: No polling required for job state changes

## Job Output

The daemon writes job output to log files, and clients tail those files directly:

1. Daemon spawns job and captures stdout/stderr
2. Daemon writes to log files continuously
3. Clients request job metadata (includes log paths)
4. Clients tail log files directly

Log files are removed when the job is removed (`gob remove`).

## State Management

- **SQLite persistence**: All job and run metadata stored in `state.db`
- **Crash recovery**: Unclean shutdowns detected via `shutdown_clean` flag
- **Orphan handling**: Orphaned processes killed on daemon restart
- **Log files persist**: Job output written continuously to disk in state directory

Jobs are children of the daemon process. If the daemon crashes, the database retains job metadata. On restart, the daemon:
1. Detects the unclean shutdown
2. Finds runs still marked as "running" in the database
3. Verifies if the processes still exist (checking PID, start time, and command)
4. Kills any orphaned processes that match
5. Marks all runs as stopped

## Daemon Lifecycle

### Auto-start

When a client command runs:

1. Client attempts to connect to socket
2. If connection fails, client starts daemon as a detached process (setsid)
3. Daemon opens database and runs migrations
4. Daemon performs crash recovery if previous shutdown was unclean
5. Daemon loads jobs and runs from database
6. Daemon creates socket and starts listening
7. Client retries connection

### Idle Shutdown

The daemon automatically shuts down after 5 minutes with no **running** jobs:

- Stopped jobs remain in the database
- Log files remain in the state directory
- Next command auto-starts a fresh daemon
- Jobs are loaded from the database on startup

This allows the daemon to conserve resources while preserving job history.

### Graceful Shutdown

`gob shutdown` performs a clean shutdown:

1. Stops all running jobs (SIGTERM, then SIGKILL after timeout)
2. Sets `shutdown_clean = true` in database
3. Shuts down the daemon

Job history and log files are preserved.

The daemon also shuts down gracefully when it receives SIGTERM or SIGINT:

1. Stops all running jobs
2. Updates all runs to "stopped" in database
3. Sets `shutdown_clean = true`
4. Closes database and removes socket/PID files

### Signal Handling

The daemon handles SIGTERM and SIGINT for graceful shutdown, cleaning up resources before exit.

## Database Schema

The SQLite database (`state.db`) contains three tables:

- **daemon_state**: Key-value store for daemon metadata (`instance_id`, `shutdown_clean`)
- **jobs**: Job definitions (ID, command, workdir, statistics)
- **runs**: Run history (ID, job reference, PID, status, exit code, timestamps)

Schema migrations are managed by [goose](https://github.com/pressly/goose) with embedded SQL files. See [`internal/daemon/migrations/`](../internal/daemon/migrations/) for migration files.

## Limitations

- **Unix-only**: Windows not supported
- **Unbounded logs**: Job logs can grow without limit
- **No automatic restart**: Jobs don't restart automatically after daemon restart (they remain stopped)
