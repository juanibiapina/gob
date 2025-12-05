# Daemon Architecture Design

## Overview

This document describes the redesign of `gob` to use a daemon-based architecture similar to tmux. The daemon runs in the background and manages all job processes, while CLI commands and the TUI act as clients that communicate with the daemon via a Unix socket.

**Platform Support:** This architecture is designed for Unix-like systems (Linux, macOS, BSD) only. Windows is not supported due to reliance on Unix-specific features (Unix sockets, signals, process groups, setsid).

## Goals

1. **Single source of truth**: All job state lives in the daemon process
2. **Real-time updates**: Multiple clients can observe job state changes simultaneously
3. **Simplified architecture**: Eliminate file-based state synchronization issues
4. **Automatic daemon lifecycle**: Daemon auto-starts when needed, shuts down with `gob nuke`
5. **Process isolation**: Jobs are children of the daemon, not individual CLI invocations

## Architecture Components

### 1. Daemon Process (Internal)

The daemon is a long-running background process that:

- **Manages all job processes**: Creates, monitors, and terminates jobs
- **Maintains job state**: Holds all job metadata in memory (PID, status, command, logs, etc.)
- **Listens on Unix socket**: Accepts connections from multiple clients simultaneously
- **Handles client requests**: Processes commands (add, stop, list, etc.) and sends responses
- **Manages job output**: Writes stdout/stderr to log files, provides paths to clients
- **Auto-shutdown**: Exits when `gob nuke` is called

**Process lifecycle:**
```
Start → Listen on socket → Accept client connections → Process requests → Shutdown (on nuke)
                              ↑                            ↓
                              └────────────────────────────┘
                                   (loop until shutdown)
```

**Data structures (in-memory):**
```go
type Job struct {
    ID         string
    Command    []string
    PID        int
    Status     JobStatus // Running, Stopped
    Workdir    string
    CreatedAt  time.Time
    StdoutPath string    // Path to stdout log file
    StderrPath string    // Path to stderr log file
}

type Daemon struct {
    jobs      map[string]*Job
    clients   []*Client        // Connected clients
    socket    net.Listener
    mu        sync.RWMutex     // Protects jobs and clients
}
```

**Daemon startup (internal, auto-daemonizes):**
The daemon is started as a detached process with `setsid`, ensuring it survives after the parent exits. No manual backgrounding needed.

### 2. Client Commands (CLI and TUI)

All existing commands (`add`, `run`, `list`, `stop`, etc.) become clients that:

1. **Check if daemon is running** (probe the socket)
2. **Auto-start daemon** if not running (daemon auto-daemonizes)
3. **Connect to Unix socket**
4. **Send request** with command and parameters
5. **Receive response** (immediate for most commands)
6. **Close connection** (or keep open for event subscriptions)

**Commands that auto-start daemon:**
- `gob add`
- `gob run`
- `gob list`
- `gob start`
- `gob stop`
- `gob restart`
- `gob signal`
- `gob logs`
- `gob stdout`
- `gob stderr`
- `gob tui`
- `gob cleanup`
- `gob nuke` (auto-starts then immediately shuts down)
- `gob remove`
- `gob events` (subscribe to job events)
- `gob ping` (test daemon connectivity)

**Commands that don't auto-start:**
- `gob overview` (uses daemon but shows empty if unavailable)
- `gob completion` (doesn't need daemon)

### 3. Communication Protocol

**Socket location:**
```
Unix socket: $XDG_RUNTIME_DIR/gob/daemon.sock
  - Falls back to /tmp/gob-$UID/daemon.sock if XDG_RUNTIME_DIR not set
```

**Message format (JSON over socket):**

Client → Daemon (Request):
```json
{
  "type": "ping|add|run|list|stop|start|restart|remove|cleanup|nuke|signal|get_job|subscribe",
  "payload": {
    // Command-specific parameters
    "command": ["npm", "start"],
    "workdir": "/path/to/project",
    "job_id": "V3x0QqI",
    "stream": "stdout|stderr|both",
    // ...
  }
}
```

Daemon → Client (Response):
```json
{
  "success": true,
  "error": "",
  "data": {
    // Command-specific response data
    "job_id": "V3x0QqI",
    "stdout_path": "/path/to/stdout.log",
    "stderr_path": "/path/to/stderr.log",
    "jobs": [...],
    // ...
  }
}
```

Daemon → Client (Event Stream - for subscriptions):
```json
{
  "event": "job_added|job_started|job_stopped|job_removed",
  "job_id": "V3x0QqI",
  "data": {
    "status": "running|stopped",
    "pid": 12345,
    // ... full job metadata
  }
}
```

**Request Types:**

| Request Type | Description | Response |
|-------------|-------------|----------|
| `add` | Create and start new job | Job metadata with log paths |
| `run` | Create/reuse job, return metadata | Job metadata with log paths |
| `list` | Get all jobs (optionally filtered by workdir) | Array of job metadata |
| `stop` | Stop a running job | Success/error |
| `start` | Start a stopped job | Success/error |
| `restart` | Stop and start a job | Success/error |
| `remove` | Remove stopped job | Success/error |
| `cleanup` | Remove all stopped jobs | Count removed |
| `nuke` | Stop all jobs, remove all metadata, shutdown daemon | Count removed |
| `signal` | Send signal to job | Success/error |
| `get_job` | Get single job by ID | Job metadata |
| `subscribe` | Subscribe to job events (state changes only) | Event stream (long-lived connection) |
| `ping` | Test daemon connectivity | Success/error |

### 4. Auto-start Mechanism

**How auto-start works:**

1. Client attempts to connect to socket
2. If connection fails (daemon not running):
   - Client starts daemon as a detached process with `setsid`
   - Daemon creates socket and starts listening
   - Parent waits briefly for socket to appear
   - Retry connection
3. If connection succeeds, proceed with request

**Lock file location:**
```
PID file: $XDG_RUNTIME_DIR/gob/daemon.pid
```

**No user-facing daemon command:**
The daemon is purely an internal implementation detail. Users interact only with regular commands (`add`, `run`, `list`, etc.), which handle daemon lifecycle transparently.

### 5. Multiple Simultaneous Clients

The daemon handles multiple clients connected at the same time:

**Scenarios:**

1. **TUI viewing jobs** while CLI adds a new job
   - TUI has an open subscription to job events
   - CLI connects, sends `add` request, receives response
   - Daemon broadcasts `job_added` event to all subscribers
   - TUI receives event and updates display

2. **Multiple TUIs open simultaneously**
   - Each TUI subscribes to job events
   - Any state change (job added/stopped/removed) broadcasts to all
   - All TUIs stay in sync automatically

3. **CLI stopping a job** while TUI is viewing it
   - CLI sends `stop` request
   - Daemon stops the job
   - Daemon broadcasts `job_stopped` event to all subscribers
   - TUI receives event and updates display

**Implementation:**
```go
// Daemon maintains list of active client connections
type Client struct {
    conn       net.Conn
    encoder    *json.Encoder
    decoder    *json.Decoder
    subscribed bool           // Is this client subscribed to events?
    filter     *EventFilter   // Optional filter (workdir, job IDs, etc.)
}

// When job state changes, broadcast to all subscribed clients
func (d *Daemon) broadcastEvent(event Event) {
    d.mu.RLock()
    defer d.mu.RUnlock()
    
    for _, client := range d.clients {
        if client.subscribed && client.matchesFilter(event) {
            client.encoder.Encode(event)
        }
    }
}
```

### 6. Job Output Management

**Approach: Daemon writes, clients tail**

The daemon writes job output to log files, and clients tail those files directly. No streaming through the daemon needed.

**How it works:**
1. Daemon spawns job process and captures stdout/stderr
2. Daemon writes output to log files:
   - `$XDG_RUNTIME_DIR/gob/logs/{job_id}.stdout.log`
   - `$XDG_RUNTIME_DIR/gob/logs/{job_id}.stderr.log`
3. When client requests output (e.g., `gob run`, `gob stdout --follow`):
   - Client sends request
   - Daemon responds with log file paths
   - Client tails the files directly using standard file operations
4. TUI:
   - Subscribes to job events for state changes
   - Receives log file paths in job metadata
   - Tails log files directly (no daemon streaming)

**Benefits:**
- **Simpler daemon**: No output buffering or streaming logic needed
- **Standard tools**: Clients can use existing file-tailing libraries
- **No sync issues**: Single source of truth (the log file on disk)
- **Lower memory**: Daemon doesn't need to buffer output
- **Resilient**: If daemon restarts, log files persist

**Log file cleanup:**
- Log files removed when job is removed (`gob remove` or `gob cleanup`)
- All logs deleted on `gob nuke`

### 7. State Management

**In-memory only:**
- All job metadata lives only in daemon memory
- No JSON metadata files written to disk
- If daemon crashes, all state is lost

**What persists to disk:**
- **Log files only** - Job stdout/stderr written continuously
- **Daemon PID file** - For detecting if daemon is running

**Crash behavior:**
- Daemon crash → all job metadata lost, all jobs terminated
- Jobs are children of daemon (using Setpgid, not Setsid)
- Next client command auto-starts new daemon
- New daemon starts with empty state (no jobs)

**Clean shutdown (`gob nuke`):**
1. Client sends `nuke` request
2. Daemon:
   - Stops all running jobs (SIGTERM with escalation to SIGKILL)
   - Removes all log files
   - Closes all client connections
   - Removes PID file and socket
   - Exits

**Why no persistence:**
- **Simplicity**: No need to sync memory and disk state
- **No recovery complexity**: No stale PID detection or zombie cleanup
- **Clear contract**: Daemon crash = clean slate
- **User expectation**: Similar to tmux server - if it crashes, sessions are gone

## Migration Path

### Phase 1: Daemon Infrastructure ✅
- [x] Implement daemon process with socket listener
- [x] Implement auto-daemonization (setsid)
- [x] Implement client connection and protocol (basic request/response)
- [x] Implement auto-start mechanism

### Phase 2a: Test Refactoring ✅
- [x] Add `--json` flag to `list` command
- [x] Add `get_job` and `get_job_field` test helpers
- [x] Update all tests to use `gob list --json` instead of reading metadata files
- [x] Tests are now implementation-agnostic (ready for daemon migration)

Note: Two tests in start.bats and stdout_stderr.bats modify metadata files to test
log clearing behavior. These will need redesign when migrating to daemon.

### Phase 2b: Test Metadata Cleanup ✅
Clean up remaining direct metadata file access in tests.

**Log file access (OK - logs remain on disk after daemon migration):**
- `test/logs.bats`, `test/nuke.bats`, `test/start.bats`, `test/stdout_stderr.bats`
- Uses `wait_for_log_content` and checks for `.stdout.log`/`.stderr.log` files
- These are fine - daemon will still write logs to disk

**Completed:**
- [x] Refactored log clearing tests to use timestamp-based approach
- [x] Removed direct metadata file manipulation from `test/start.bats` and `test/stdout_stderr.bats`

The tests now use `date +%s%N` to output unique timestamps each run, verifying
logs are cleared by checking the old output is gone rather than modifying commands

### Phase 2c: Core Commands ✅
- [x] Add `Job` struct to daemon with in-memory job management
- [x] Migrate `list` command to client-server
- [x] Migrate `add` command to client-server
- [x] Migrate `stop/start/restart` commands to client-server
- [x] Migrate `signal` command to client-server
- [x] Migrate `remove/cleanup` commands to client-server
- [x] Migrate `stdout/stderr` commands to client-server
- [x] Migrate `logs` command to client-server
- [x] Migrate `nuke` command to client-server
- [x] Update tests to use `XDG_RUNTIME_DIR` for log file paths

**Note:** Log files are now stored in `$XDG_RUNTIME_DIR/gob/` instead of `$XDG_DATA_HOME/gob/`.

**Not migrated (deferred to later phases):**
- `tui` command - requires TUI daemon integration (Phase 3)

### Phase 2d: Run Command ✅
- [x] Implement `FindJobByCommand` in daemon for job reuse logic
- [x] Migrate `run` command to client-server
- [x] Handle restart existing vs create new job

### Phase 3: TUI Basic Daemon Integration ✅
Convert TUI to use daemon client instead of direct file access. This fixes the broken
state where TUI reads logs from `$XDG_DATA_HOME` but daemon writes to `$XDG_RUNTIME_DIR`.

- [x] Convert TUI to use `client.List()` for job list (replace `storage.ListJobMetadata()`)
- [x] Get log paths from daemon response (`StdoutPath`, `StderrPath` fields)
- [x] Remove dependency on `storage` package from TUI
- [x] Keep 500ms polling for now (works correctly, just uses daemon)

### Phase 4: Event Subscription + Real-time TUI ✅
Add event subscription to daemon and update TUI to use it instead of polling.

- [x] Implement `subscribe` request type in daemon protocol
- [x] Maintain list of subscribed clients in daemon
- [x] Broadcast job state changes (added/started/stopped/removed) to subscribers
- [x] Add client-side event handling (`Subscribe()` and `SubscribeChan()` methods)
- [x] Update TUI to subscribe to events instead of polling for job state
- [x] Keep log file polling for log content (500ms)
- [x] Automatic process exit detection via `cmd.Wait()` goroutines
- [x] Rewrite `gob logs` to use event subscription (no polling)

Notes:
- Added `gob events` command for testing/debugging subscriptions
- Events provide instant updates for all job state changes
- No polling for job state - daemon monitors processes via `cmd.Wait()` and emits events
- Log content still polled from files (daemon writes logs, clients tail them)

### Phase 5: Polish
- [x] Socket permissions (0600 user-only)
- [x] Signal handling (SIGTERM, SIGINT) for graceful daemon shutdown
- [x] Stale socket cleanup on daemon start
- [ ] Handle client disconnection gracefully (partial - subscribers removed on error)
- [ ] Proper error handling and logging
- [ ] Performance testing
- [ ] Documentation updates

### Phase 6: Testing & Robustness (Proposed)
- [ ] Unit tests for daemon package (`internal/daemon/*_test.go`)
- [ ] Integration tests for client-server communication
- [ ] Test daemon auto-start behavior
- [ ] Test event subscription/broadcasting
- [ ] Test signal forwarding to jobs
- [ ] Configure daemon process logging (TODO in `daemonize.go:23`)
- [ ] Handle rapid client connect/disconnect
- [ ] Stress test with many concurrent clients

### Phase 7: Edge Cases & UX (Proposed)
- [ ] Better error messages when daemon fails to start
- [ ] Timeout handling for unresponsive jobs
- [ ] Job output size limits / log rotation
- [ ] Detect and handle orphaned log files
- [ ] `gob status` command to show daemon health/stats
- [ ] Structured logging with log levels

## Benefits

1. **Simpler architecture**: No output streaming, no state persistence
2. **Better UX**: Changes from one client immediately visible in others
3. **Cleaner code**: Single source of truth (daemon memory)
4. **More robust**: Daemon manages process lifecycle
5. **Standard tools**: File tailing uses well-tested libraries
6. **Lower memory**: No output buffering in daemon

## Drawbacks & Considerations

1. **No crash recovery**: Daemon crash loses all job metadata (by design)
2. **Orphaned processes**: Jobs may keep running if daemon crashes (using Setpgid, not Setsid)
3. ~~**Socket permissions**~~: Addressed - socket set to 0600 (user-only)
4. **Breaking change**: Not backward compatible with previous file-based implementation
5. **Testing**: Need integration tests for client-server communication
6. **Unix-only**: Windows is not supported (Unix sockets, signals, setsid required)

## Alternative Approaches Considered

### 1. Stream output through daemon
- **Pro**: Clients don't need file access
- **Con**: Complex buffering logic, memory overhead, sync issues
- **Decision**: Not needed - file tailing is simple and reliable

### 2. Persist metadata to disk
- **Pro**: Daemon crash recovery possible
- **Con**: Complexity of syncing memory/disk, stale PID handling
- **Decision**: Not worth it - crashes should be rare, clean slate is simpler

### 3. Keep file-based architecture
- **Pro**: No daemon needed
- **Con**: Can't do real-time multi-client updates, polling required
- **Decision**: Daemon enables better UX for TUI and multi-client scenarios

### 4. Support Windows with named pipes
- **Pro**: Cross-platform support
- **Con**: Significant complexity, different IPC mechanisms, different process management
- **Decision**: Not worth it - focus on Unix-like systems where gob is primarily used

## Current Status

**Phases 1-4 complete.** The daemon architecture is fully functional:

- All CLI commands use the daemon client
- TUI uses event subscriptions for real-time updates
- Job state changes broadcast instantly to all clients
- Log files written to `$XDG_RUNTIME_DIR/gob/`

**Remaining work (Phase 5+):**

| Task | Priority | Notes |
|------|----------|-------|
| Unit tests for daemon package | High | No tests exist for `internal/daemon/` |
| Configure daemon logging | Medium | TODO in `daemonize.go:23` |
| Client disconnection handling | Medium | Subscribers removed on error, but could be cleaner |
| Performance/stress testing | Low | Not tested with many concurrent clients |
| Log rotation / size limits | Low | Job logs can grow unbounded |

## Conclusion

This daemon-based architecture provides a solid foundation for real-time job management while keeping complexity low. By avoiding output streaming through the daemon and not persisting metadata, we get a simpler implementation with clear failure modes.

The auto-start mechanism ensures users don't need to think about the daemon - it's an invisible implementation detail. `gob nuke` provides a clear "reset everything" command that shuts down the daemon cleanly.

The core migration is complete. Remaining work focuses on testing, robustness, and polish.
