# gob

A lightweight CLI tool for managing background processes. `gob` allows you to start commands as detached background processes, monitor their status, lifecycle and output.

## Features

- **Detached Process Execution** - Run commands that persist independently of the CLI
- **AI coding agent friendly** - Easy for Coding Agents to start and monitor background processes

## Installation

### Build from Source

Requirements:
- Go 1.25.4 or later
- Make

```bash
# Clone the repository
git clone https://github.com/yourusername/gob.git
cd gob

# Build the binary
make build

# The binary will be available at dist/gob
# Optionally, move it to your PATH
sudo cp dist/gob /usr/local/bin/
```

## Quick Start

```bash
# Start a background job
gob add sleep 300

# List all jobs
gob list

# View stdout output
gob stdout 1234567890

# Stop a job
gob stop 1234567890

# Clean up all stopped jobs
gob cleanup
```

## Usage

### Core Commands

#### `gobadd <command> [args...]`

Start a command as a background job. The job runs detached and persists even after the CLI exits.

```bash
# Start a long-running server
gob addpython -m http.server 8000

# Run a background process
gob add./script.sh --verbose

# Execute with arguments
gob addffmpeg -i input.mp4 output.avi
```

**Output**: Job ID (Unix timestamp)

#### `goblist`

Display all jobs with their status (running/stopped), PID, and command.

```bash
gob list
```

**Output format**:
```
ID          STATUS   PID    COMMAND
1234567890  running  12345  sleep 300
1234567891  stopped  -      python server.py
```

#### `gobstop <job_id> [--force]`

Stop a running job. Uses SIGTERM by default; use `--force` for SIGKILL.

```bash
# Graceful shutdown
gob stop1234567890

# Force kill
gob stop1234567890 --force
```

#### `gobstart <job_id>`

Restart a stopped job with the same command.

```bash
gob start1234567890
```

#### `gobrestart <job_id>`

Stop and then start a job (combines stop + start).

```bash
gob restart1234567890
```

### Job Cleanup

#### `gobremove <job_id>`

Remove metadata for a single stopped job. Job must be stopped first.

```bash
gob remove1234567890
```

#### `gobcleanup`

Remove metadata for all stopped jobs.

```bash
gob cleanup
```

#### `gobnuke`

Stop all running jobs and remove all metadata. Use with caution.

```bash
gob nuke
```

### Output Management

#### `gobstdout <job_id> [--follow]`

Display stdout output for a job.

```bash
# View stdout
gob stdout1234567890

# Tail stdout in real-time
gob stdout1234567890 --follow
```

#### `gobstderr <job_id> [--follow]`

Display stderr output for a job.

```bash
# View stderr
gob stderr1234567890

# Tail stderr in real-time
gob stderr1234567890 --follow
```

### Advanced

#### `gobsignal <job_id> <signal>`

Send a custom signal to a job.

```bash
# Send SIGHUP (reload configuration)
gob signal1234567890 HUP

# Send SIGUSR1
gob signal1234567890 USR1
```

**Supported signals**: TERM, KILL, HUP, INT, QUIT, USR1, USR2, and more

#### `goboverview`

Display usage patterns and common workflows. Also shown when running `job` without arguments.

```bash
gob overview
```

## Development

### Building

```bash
make build
```

Binary output: `dist/gob`

### Testing

Requirements:
- BATS (included as git submodule)
- `jq` (JSON processor)

```bash
# Initialize git submodules (first time only)
git submodule update --init --recursive

# Run tests (automatically builds first)
make test
```

Tests are located in `test/*.bats` and verify end-to-end functionality.

## Requirements

**Runtime**:
- Unix-like operating system (Linux, macOS, BSD)

**Build**:
- Go 1.25.4+

**Testing**:
- BATS framework (included)
- `jq` command-line tool
