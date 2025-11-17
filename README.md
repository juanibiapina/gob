# job

A lightweight CLI tool for managing background processes. `job` allows you to start commands as detached background processes, monitor their status, lifecycle and output.

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
git clone https://github.com/yourusername/job.git
cd job

# Build the binary
make build

# The binary will be available at dist/job
# Optionally, move it to your PATH
sudo cp dist/job /usr/local/bin/
```

## Quick Start

```bash
# Start a background job
job add sleep 300

# List all jobs
job list

# View stdout output
job stdout 1234567890

# Stop a job
job stop 1234567890

# Clean up all stopped jobs
job cleanup
```

## Usage

### Core Commands

#### `job add <command> [args...]`

Start a command as a background job. The job runs detached and persists even after the CLI exits.

```bash
# Start a long-running server
job add python -m http.server 8000

# Run a background process
job add ./script.sh --verbose

# Execute with arguments
job add ffmpeg -i input.mp4 output.avi
```

**Output**: Job ID (Unix timestamp)

#### `job list`

Display all jobs with their status (running/stopped), PID, and command.

```bash
job list
```

**Output format**:
```
ID          STATUS   PID    COMMAND
1234567890  running  12345  sleep 300
1234567891  stopped  -      python server.py
```

#### `job stop <job_id> [--force]`

Stop a running job. Uses SIGTERM by default; use `--force` for SIGKILL.

```bash
# Graceful shutdown
job stop 1234567890

# Force kill
job stop 1234567890 --force
```

#### `job start <job_id>`

Restart a stopped job with the same command.

```bash
job start 1234567890
```

#### `job restart <job_id>`

Stop and then start a job (combines stop + start).

```bash
job restart 1234567890
```

### Job Cleanup

#### `job remove <job_id>`

Remove metadata for a single stopped job. Job must be stopped first.

```bash
job remove 1234567890
```

#### `job cleanup`

Remove metadata for all stopped jobs.

```bash
job cleanup
```

#### `job nuke`

Stop all running jobs and remove all metadata. Use with caution.

```bash
job nuke
```

### Output Management

#### `job stdout <job_id> [--follow]`

Display stdout output for a job.

```bash
# View stdout
job stdout 1234567890

# Tail stdout in real-time
job stdout 1234567890 --follow
```

#### `job stderr <job_id> [--follow]`

Display stderr output for a job.

```bash
# View stderr
job stderr 1234567890

# Tail stderr in real-time
job stderr 1234567890 --follow
```

### Advanced

#### `job signal <job_id> <signal>`

Send a custom signal to a job.

```bash
# Send SIGHUP (reload configuration)
job signal 1234567890 HUP

# Send SIGUSR1
job signal 1234567890 USR1
```

**Supported signals**: TERM, KILL, HUP, INT, QUIT, USR1, USR2, and more

#### `job overview`

Display usage patterns and common workflows. Also shown when running `job` without arguments.

```bash
job overview
```

## Development

### Building

```bash
make build
```

Binary output: `dist/job`

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
