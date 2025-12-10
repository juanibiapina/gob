# Daemon Architecture

## Overview

`gob` uses a daemon-based architecture similar to tmux. The daemon runs in the background and manages all job processes, while CLI commands and the TUI act as clients that communicate with the daemon via a Unix socket.

**Platform Support:** Unix-like systems only (Linux, macOS, BSD). Windows is not supported due to reliance on Unix sockets, signals, process groups, and setsid.

## Components

### Daemon Process

The daemon is a long-running background process that:

- **Manages all job processes**: Creates, monitors, and terminates jobs
- **Maintains job state**: Holds all job metadata in memory (PID, status, command, logs, etc.)
- **Listens on Unix socket**: Accepts connections from multiple clients simultaneously
- **Handles client requests**: Processes commands (add, stop, list, etc.) and sends responses
- **Broadcasts events**: Notifies subscribed clients of job state changes
- **Manages job output**: Writes stdout/stderr to log files

The daemon is an internal implementation detail—users interact only with regular commands (`add`, `run`, `list`, etc.), which handle daemon lifecycle transparently.

### Client Commands

All commands (`add`, `run`, `list`, `stop`, etc.) are clients that:

1. Check if daemon is running (probe the socket)
2. Auto-start daemon if not running
3. Connect to Unix socket
4. Send request and receive response
5. Close connection (or keep open for event subscriptions)

## File Locations

All files are stored under `$XDG_RUNTIME_DIR/gob/`. Path resolution uses the [adrg/xdg](https://github.com/adrg/xdg) library—see [`internal/daemon/paths.go`](../internal/daemon/paths.go) for implementation.

| File | Path |
|------|------|
| Unix socket | `daemon.sock` |
| PID file | `daemon.pid` |
| Daemon log | `daemon.log` |
| Job stdout | `{job_id}.stdout.log` |
| Job stderr | `{job_id}.stderr.log` |

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

Log files are removed when the job is removed (`gob remove` or `gob nuke`).

## State Management

- **In-memory only**: All job metadata lives in daemon memory
- **No persistence**: If daemon crashes, all state is lost
- **Log files persist**: Job output written continuously to disk
- **Clean slate on crash**: Next command auto-starts a fresh daemon

Jobs are children of the daemon process. If the daemon crashes, jobs may continue running as orphans.

## Daemon Lifecycle

### Auto-start

When a client command runs:

1. Client attempts to connect to socket
2. If connection fails, client starts daemon as a detached process (setsid)
3. Daemon creates socket and starts listening
4. Client retries connection

### Shutdown

`gob nuke` performs a clean shutdown:

1. Stops all running jobs (SIGTERM)
2. Removes all log files
3. Removes all jobs
4. Shuts down the daemon

The daemon also shuts down gracefully when it receives SIGTERM or SIGINT.

### Signal Handling

The daemon handles SIGTERM and SIGINT for graceful shutdown, cleaning up resources before exit.

## Limitations

- **No crash recovery**: Daemon crash loses all job metadata
- **Orphaned processes**: Jobs may keep running if daemon crashes
- **Unix-only**: Windows not supported
- **Unbounded logs**: Job logs can grow without limit
