# gob

> Background job management for HUMANs and AGENTs.

`gob` (pronounced job) is a lightweight CLI tool for managing background processes, because you and Claude Code both need to check the logs!

When an AI coding AGENT like Claude Code starts background processes, the output is difficult to inspect.
`gob` solves this by giving both you and the AGENT a shared interface to start, stop, monitor, and inspect background process.
Start a dev server with Claude Code, check its logs yourself and vice-versa.
The AGENT can monitor what you started.
Everyone has the same view.

No more "can you check if that's still running?" No more copy-pasting logs through chat. No more reading tmux outputs. Just direct access to your processes, for everyone.

## Features

- **Detached Process Execution** - Run commands that persist independently of the CLI
- **AI coding agent friendly** - Easy for Coding Agents to start and monitor background processes
- **Jobs per directory** - Jobs are scoped per directory, making it easier to maintain per project jobs

## Installation

### Using Homebrew

Install gob via Homebrew:

```bash
brew tap juanibiapina/taps
brew install gob
```

### Using Go Install

If you have Go installed, you can install `gob` with a single command:

```bash
go install github.com/juanibiapina/gob@latest
```

Requirements:
- Go 1.25.4 or later

The binary will be installed to `$GOPATH/bin` (or `$GOBIN` if set). Make sure this directory is in your `PATH`.

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

For build instructions, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Shell Completion

`gob` supports shell completion for Bash, Zsh, Fish, and PowerShell. Completions include dynamic job ID suggestions with command descriptions.

### Bash

```bash
# Add to ~/.bashrc
source <(gob completion bash)
```

### Zsh

```bash
# Add to ~/.zshrc
source <(gob completion zsh)
```

If you get "command not found: compdef", add this before the source line:
```bash
autoload -Uz compinit && compinit
```

### Fish

```bash
# Add to ~/.config/fish/config.fish
gob completion fish | source
```

### PowerShell

```powershell
# Add to your PowerShell profile
gob completion powershell | Out-String | Invoke-Expression
```

## Quick Start

```bash
# Usage overview
gob

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

## Using with Claude Code

To make gob available to Claude Code or other AI coding assistants, add it to your global `~/.claude/CLAUDE.md`:

```markdown
# Available CLI Tools

- `gob` - Background process manager

# Usage Expectations

- Use `gob` to start and monitor background processes like servers and other long running tasks (run `gob` for overview)
```

## Usage

### Core Commands

#### `gob overview`

Display usage patterns and common workflows. Also shown when running `gob` without arguments.

```bash
gob overview
```

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

**Output**: Job ID (e.g., `V3x0QqI`)

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

## Contributing

Interested in contributing? Check out [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing instructions, and contribution guidelines.
