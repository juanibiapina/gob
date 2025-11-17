# gob

A lightweight CLI tool for managing background processes. `gob` allows you to start commands as detached background processes, monitor their status, lifecycle and output.

## Features

- **Detached Process Execution** - Run commands that persist independently of the CLI
- **AI coding agent friendly** - Easy for Coding Agents to start and monitor background processes

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/juanibiapina/gob/releases).

**Available platforms**: Linux, macOS (both amd64 and arm64)

```bash
# Download the appropriate binary for your platform
# For example, macOS Apple Silicon (arm64):
curl -LO https://github.com/juanibiapina/gob/releases/latest/download/gob_VERSION_darwin_arm64.tar.gz

# Extract the archive
tar -xzf gob_VERSION_darwin_arm64.tar.gz

# Move to your PATH
sudo mv gob /usr/local/bin/

# Verify installation
gob --version
```

### Build from Source

Requirements:
- Go 1.25.4 or later
- Make

```bash
# Clone the repository
git clone https://github.com/juanibiapina/gob.git
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
gob start sleep 300

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

#### `gob start <command> [args...]`

Start a command as a background job. The job runs detached and persists even after the CLI exits.

```bash
# Start a long-running server
gob start python -m http.server 8000

# Run a background process
gob start ./script.sh --verbose

# Execute with arguments
gob start ffmpeg -i input.mp4 output.avi
```

**Output**: Job ID (Unix timestamp)

#### `gob list`

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

#### `gob stop <job_id> [--force]`

Stop a running job. Uses SIGTERM by default; use `--force` for SIGKILL.

```bash
# Graceful shutdown
gob stop 1234567890

# Force kill
gob stop 1234567890 --force
```

#### `gob restart <job_id>`

Stop and then start a job with the same command.

```bash
gob restart 1234567890
```

### Job Cleanup

#### `gob remove <job_id>`

Remove metadata for a single stopped job. Job must be stopped first.

```bash
gob remove 1234567890
```

#### `gob cleanup`

Remove metadata for all stopped jobs.

```bash
gob cleanup
```

#### `gob nuke`

Stop all running jobs and remove all metadata. Use with caution.

```bash
gob nuke
```

### Output Management

#### `gob stdout <job_id> [--follow]`

Display stdout output for a job.

```bash
# View stdout
gob stdout 1234567890

# Tail stdout in real-time
gob stdout 1234567890 --follow
```

#### `gob stderr <job_id> [--follow]`

Display stderr output for a job.

```bash
# View stderr
gob stderr 1234567890

# Tail stderr in real-time
gob stderr 1234567890 --follow
```

### Advanced

#### `gob signal <job_id> <signal>`

Send a custom signal to a job.

```bash
# Send SIGHUP (reload configuration)
gob signal 1234567890 HUP

# Send SIGUSR1
gob signal 1234567890 USR1
```

**Supported signals**: TERM, KILL, HUP, INT, QUIT, USR1, USR2, and more

#### `gob overview`

Display usage patterns and common workflows. Also shown when running `gob` without arguments.

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

### Contributing

When making changes to the project:
- Update `CHANGELOG.md` under `[Unreleased]` section for user-facing changes
- Follow [Keep a Changelog](https://keepachangelog.com/) format
- Categorize changes as: Added, Changed, Deprecated, Removed, Fixed, Security

## Requirements

**Runtime**:
- Unix-like operating system (Linux, macOS, BSD)
- **Note**: Windows is not supported due to Unix-specific process management APIs

**Build**:
- Go 1.25.4+

**Testing**:
- BATS framework (included)
- `jq` command-line tool
