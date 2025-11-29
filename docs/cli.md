# Gob CLI Documentation

## Overview

`gob` is a command-line tool for managing background jobs. It allows you to start long-running commands as detached background processes, monitor their status, send signals, and clean up finished jobs.

## Installation

```bash
make build
# Binary will be available at ./gob
```

## Usage

```
gob [command] [flags]
```

## Data Storage

Job metadata is stored in `.local/share/gob/` relative to the current working directory where commands are executed.

- **Metadata files**: `.local/share/gob/<job_id>.json`
- **Job ID format**: Base62-encoded timestamp (e.g., `V3x0QqI`)

Each metadata file contains:
- `id`: Unique job identifier (base62-encoded timestamp)
- `command`: Array of command and arguments
- `pid`: Process ID of the background job

**Important:** All job commands operate on job IDs, not PIDs. The job ID is a stable identifier that persists even if the process stops. The PID is only used internally for process management.

## Commands

### start

Start a command as a background job.

**Syntax:**
```bash
gob start <command> [args...]
```

**Arguments:**
- `command`: Command to execute (required)
- `args`: Arguments to pass to the command (optional)

**Behavior:**
- Starts the command as a detached background process
- Creates a new process group (prevents SIGHUP propagation)
- Redirects stdout, stderr, and stdin to `/dev/null`
- Saves job metadata with unique job ID
- Returns immediately after starting the job

**Output:**
```
Started job <job_id> running: <command>
```

**Examples:**
```bash
# Start a long-running sleep
gob start sleep 3600

# Start a server
gob start python -m http.server 8080

# Start a background compilation
gob start make build
```

**Exit Codes:**
- `0`: Job started successfully
- `1`: Error (missing command, failed to start process, failed to save metadata)

---

### list

List all background jobs with their current status.

**Syntax:**
```bash
gob list
```

**Behavior:**
- Reads all job metadata from `.local/share/gob/`
- Checks if each process is still running
- Displays jobs sorted by start time (newest first)

**Output Format:**
```
<job_id>: [<pid>] <status>: <command>
```

Where:
- `job_id`: Unique job identifier
- `pid`: Process ID
- `status`: Either `running` or `stopped`
- `command`: Original command that was executed

**Example Output:**
```
V3x0QqI: [12345] running: sleep 3600
V3x0PrH: [12344] stopped: python server.py
V3x0NsG: [12343] running: make watch
```

**Empty State:**
```
No jobs found
```

**Exit Codes:**
- `0`: Success
- `1`: Error reading job directory

---

### stop

Stop a background job by sending SIGTERM (or SIGKILL with --force).

**Syntax:**
```bash
gob stop<job_id> [flags]
```

**Arguments:**
- `job_id`: ID of the job to stop (required)

**Flags:**
- `-f, --force`: Send SIGKILL instead of SIGTERM for forceful termination

**Behavior:**
- Reads job metadata to get the PID
- Sends SIGTERM to the process (graceful shutdown)
- With `--force`: Sends SIGKILL (immediate termination)
- Verifies the job exists before attempting to stop

**Output:**
```
Stopped job <job_id> (PID <pid>)
```

Or with `--force`:
```
Force stopped job <job_id> (PID <pid>)
```

**Examples:**
```bash
# Gracefully stop a job
gob stop V3x0QqI

# Forcefully kill a stubborn job
gob stop V3x0QqI --force
```

**Exit Codes:**
- `0`: Job stopped successfully (or already stopped)
- `1`: Error (job not found, invalid job ID, failed to send signal)

**Notes:**
- Stopping an already-stopped job is not an error (idempotent)
- Use `--force` if the job doesn't respond to SIGTERM
- Job metadata is NOT removed by this command (use `cleanup`)

---

### restart

Restart a job by stopping it (if running) and starting it again.

**Syntax:**
```bash
gob restart <job_id>
```

**Arguments:**
- `job_id`: ID of the job to restart (required)

**Behavior:**
- Reads job metadata to get the PID and saved command
- If the job is running, sends SIGTERM to stop it
- If the job is already stopped, skips the stop step
- Starts the process with the saved command
- Updates the PID in metadata while preserving the job ID

**Output:**
```
Restarted job <job_id> with new PID <pid> running: <command>
```

**Examples:**
```bash
# Restart a running job
gob restart V3x0QqI

# Restart a stopped job
gob restart V3x0QqI
```

**Exit Codes:**
- `0`: Job restarted successfully
- `1`: Error (job not found, failed to stop/start process)

**Notes:**
- Works on both running and stopped jobs
- Uses SIGTERM for graceful shutdown (not SIGKILL)
- Preserves the job ID while updating the PID
- Useful for applying configuration changes or recovering from issues
- If job is already stopped, behaves like `start`

---

### remove

Remove metadata for a single stopped job.

**Syntax:**
```bash
gob remove <job_id>
```

**Arguments:**
- `job_id`: ID of the job to remove (required)

**Behavior:**
- Reads job metadata to verify the job exists
- Checks if the process is stopped
- Removes the metadata file only if the job is stopped
- Returns an error if the job is still running

**Output:**
```
Removed job <job_id> (PID <pid>)
```

**Examples:**
```bash
# Remove a specific stopped job
gob remove V3x0QqI
```

**Exit Codes:**
- `0`: Job metadata removed successfully
- `1`: Error (job not found, job still running, failed to remove)

**Notes:**
- Only works on stopped jobs (use `job stop` first if needed)
- For removing multiple stopped jobs at once, use `cleanup` instead
- Unlike `cleanup`, this is not idempotent - removing a non-existent job returns an error

---

### cleanup

Remove metadata for stopped jobs.

**Syntax:**
```bash
gob cleanup
```

**Behavior:**
- Scans all job metadata files
- Checks if each job's process is still running
- Removes metadata files for stopped processes only
- Leaves running jobs untouched

**Output:**
```
Cleaned up <n> stopped job(s)
```

**Example Output:**
```
Cleaned up 3 stopped job(s)
```

Or if nothing to clean:
```
Cleaned up 0 stopped job(s)
```

**Examples:**
```bash
# Remove all stopped job metadata
gob cleanup
```

**Exit Codes:**
- `0`: Cleanup completed successfully
- `1`: Error reading job directory

**Notes:**
- Only removes metadata for processes that are no longer running
- Does NOT stop any running jobs
- Safe to run at any time

---

### nuke

Stop all running jobs and remove all job metadata.

**Syntax:**
```bash
gob nuke
```

**Behavior:**
1. Scans all job metadata files
2. For each running job:
   - Sends SIGTERM to stop the process
3. Removes ALL job metadata files (running and stopped)

**Output:**
```
Stopped <n> running job(s)
Cleaned up <m> total job(s)
```

**Example Output:**
```
Stopped 2 running job(s)
Cleaned up 5 total job(s)
```

**Examples:**
```bash
# Stop everything and start fresh
gob nuke
```

**Exit Codes:**
- `0`: Nuke completed successfully
- `1`: Error (failed to read jobs, failed to stop some jobs)

**Notes:**
- ⚠️ **Destructive command** - stops ALL jobs and removes ALL metadata
- Uses SIGTERM (graceful) not SIGKILL
- If jobs don't respond to SIGTERM, use `gob stop --force` individually first
- Useful for cleaning up test environments or complete resets

---

### signal

Send a specific signal to a background job.

**Syntax:**
```bash
gob signal <job_id> <signal>
```

**Arguments:**
- `job_id`: ID of the job to signal (required)
- `signal`: Signal to send (required)

**Signal Format:**
Accepts both signal names and numbers:
- **Names**: `TERM`, `SIGTERM`, `HUP`, `SIGHUP`, `INT`, `SIGINT`, `KILL`, `SIGKILL`, `USR1`, `SIGUSR1`, `USR2`, `SIGUSR2`, etc.
- **Numbers**: `1` (SIGHUP), `2` (SIGINT), `9` (SIGKILL), `15` (SIGTERM), etc.

**Behavior:**
- Reads job metadata to get the PID
- Sends the specified signal to the process
- Verifies the job exists before sending signal

**Output:**
```
Sent signal <signal> to job <job_id> (PID <pid>)
```

**Examples:**
```bash
# Reload configuration (common for servers)
gob signal V3x0QqI HUP

# Interrupt a job
gob signal V3x0QqI INT

# Send custom signal by number
gob signal V3x0QqI 10

# Forcefully kill
gob signal V3x0QqI KILL
```

**Exit Codes:**
- `0`: Signal sent successfully (or job already stopped)
- `1`: Error (job not found, invalid signal, failed to send signal)

**Notes:**
- More flexible than `stop` command
- Useful for signals like HUP (reload), USR1/USR2 (custom handlers)
- Sending a signal to a stopped job is not an error (idempotent)
- Common signals: HUP (1), INT (2), TERM (15), KILL (9), USR1 (10), USR2 (12)

---

### tui

Launch an interactive terminal user interface for managing jobs.

**Syntax:**
```bash
gob tui
```

**Layout:**
```
┌─ Jobs ─────────────┬─ Logs ──────────────────┐
│ ● V3x0QqI [1234]   │ [V3x0QqI] output line 1 │
│ ○ V3x0PrH [1235]   │ [V3x0QqI] output line 2 │
└────────────────────┴─────────────────────────┘
```

The TUI uses a split-panel design:
- **Left panel (Jobs)**: List of jobs with status indicators (● running, ○ stopped)
- **Right panel (Logs)**: Live log output for selected job with [jobID] prefixes
- **Header**: Shows directory and job count statistics
- **Status bar**: Context-sensitive keybinding hints

**Keybindings:**

Navigation:
- `tab`: Switch between Jobs and Logs panels
- `↑` / `k`: Move cursor up (Jobs) or scroll up (Logs)
- `↓` / `j`: Move cursor down (Jobs) or scroll down (Logs)
- `g` / `G`: First/last job or top/bottom of logs

Job Actions (in Jobs panel):
- `s`: Stop job (SIGTERM)
- `S`: Force kill job (SIGKILL)
- `r`: Restart job
- `d`: Delete stopped job
- `n`: Start new job (opens dialog)

Log Viewer (in Logs panel):
- `PgUp` / `Ctrl+U`: Page up
- `PgDn` / `Ctrl+D`: Page down
- `f`: Toggle follow mode (auto-scroll to new content)

Global:
- `a`: Toggle between current directory and all directories
- `?`: Show help overlay
- `q`: Quit

**Features:**
- Real-time updates every 500ms
- Auto-follow mode for live log streaming
- [following] / [paused] indicator in log panel title
- Modal dialogs for new job input and help
- Color-coded status: green (●) for running, red (○) for stopped
- Workdir display when showing all directories

**Examples:**
```bash
# Launch the TUI
gob tui
```

**Exit Codes:**
- `0`: Clean exit (user pressed q)
- `1`: Error (terminal initialization failed)

**Notes:**
- Requires a terminal that supports alternate screen mode
- Best experience with 256-color terminal
- Works well with tmux and other terminal multiplexers
- Mouse scrolling supported in log panel

---

## Common Workflows

### Start and Monitor Jobs

```bash
# Start multiple jobs
gob start sleep 300
gob start python server.py
gob start npm run watch

# Check what's running
gob list
```

### Graceful Shutdown

```bash
# List jobs to find the ID
gob list

# Stop specific job
gob stop V3x0QqI

# Remove just that job's metadata
gob remove V3x0QqI

# Or clean up all stopped jobs at once
gob cleanup
```

### Force Kill Stubborn Process

```bash
# Try graceful stop first
gob stop V3x0QqI

# If it doesn't stop, force kill
gob stop V3x0QqI --force

# Clean up
gob cleanup
```

### Complete Reset

```bash
# Nuclear option - stop and clean everything
gob nuke
```

### Signal Handling

```bash
# Reload server configuration
gob signal V3x0QqI HUP

# Graceful shutdown with custom signal
gob signal V3x0QqI TERM

# Trigger custom handler
gob signal V3x0QqI USR1
```

## Exit Codes

All commands follow standard Unix exit code conventions:

- `0`: Success
- `1`: Error (see stderr for details)

## Error Handling

Errors are written to stderr with descriptive messages:

```bash
Error: job not found: V3x0QqI
Error: failed to stop job: permission denied
Error: invalid signal: INVALID
Error: command is required
```

## Technical Details

### Process Management

- Jobs are started with `syscall.SysProcAttr{Setpgid: true}` to create a new process group
- This prevents SIGHUP from terminal disconnection
- Processes are fully detached via `Process.Release()`

### Signal Zero Trick

The `list` command uses signal 0 (`syscall.Kill(pid, 0)`) to check if a process exists without actually sending a signal.

### Job ID vs PID

**Job ID:**
- Permanent identifier for a job (base62-encoded millisecond timestamp)
- Used by all CLI commands (`stop`, `signal`, `restart`, etc.)
- Stored in metadata and used as the filename (`<id>.json`)
- Remains constant even if the job is stopped and restarted
- Provides natural chronological ordering (newer jobs have larger IDs)

**PID (Process ID):**
- Operating system identifier for a running process
- Used internally for process management and signal delivery
- Changes if a job is restarted
- Only valid while the process is running

This separation allows jobs to be restarted with the same identity, making it easier to manage long-lived services.

### Idempotency

Commands are designed to be idempotent where sensible:
- Stopping an already-stopped job succeeds
- Signaling a stopped job succeeds
- Cleanup on empty job directory succeeds
