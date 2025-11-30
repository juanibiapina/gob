# gob

> Background job management for humans and AI agents.

`gob` (pronounced job, of course) is a CLI for managing background processes with a shared interface for you and your AI coding agent.

Start a dev server with Claude Code, check its logs yourself. Or vice-versa. The agent can monitor what you started. Everyone has the same view.

No more "can you check if that's still running?" No more copy-pasting logs through chat. Just direct access to your processes, for everyone.

![demo](assets/demo.gif)

[View on asciinema](https://asciinema.org/a/OgSVPWybeSvXcQVyLQ0mie8P2)

## Features

- **Interactive TUI** - Full-screen terminal interface for managing jobs
- **Detached Process Execution** - Run commands that persist independently of the CLI
- **AI coding agent friendly** - Easy for Coding Agents to start and monitor background processes
- **Jobs per directory** - Jobs are scoped per directory, making it easier to maintain per project jobs
- **Smart job reuse** - `run` command reuses existing stopped jobs with same command

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

# Run a command and wait for it to complete
gob run make test
gob run pnpm --filter web typecheck

# Add a background job (for long-running processes)
gob add python -m http.server 8000

# List all jobs
gob list

# View stdout output
gob stdout V3x0QqI

# Stop a job
gob stop V3x0QqI

# Clean up all stopped jobs
gob cleanup
```

## Using with AI Coding Agents

To make `gob` available to AI coding agents, add the following instructions to your agent's configuration file (`CLAUDE.md`, `AGENTS.md`, etc).

```markdown
## Background Jobs with `gob`

Use `gob` to manage background processes.

**When to use `run` (blocks until complete):**
- Running tests: `gob run make test`
- Build commands: `gob run make build`
- Linting/formatting: `gob run npm run lint`
- Commands with flags work directly: `gob run pnpm --filter web typecheck`
- Any command where you need to see the result before proceeding

**When to use `add` (returns immediately):**
- Dev servers: `gob add npm run dev`
- Watch modes: `gob add npm run watch`
- Long-running services: `gob add -- python -m http.server`
- Any command that runs indefinitely

**Note:** `add` requires `--` before commands with flags (e.g., `gob add -- cmd --flag`). `run` does not need this.

**Commands:**
- `gob run <command>` - Run and wait for completion (reuses existing stopped job)
- `gob add <command>` - Add a background job (always creates new)
- `gob list` - List jobs with IDs and status
- `gob stdout <job_id>` - View stdout output
- `gob stderr <job_id>` - View stderr output
- `gob stop <job_id>` - Stop a job (use `--force` for SIGKILL)
- `gob start <job_id>` - Start a stopped job
- `gob restart <job_id>` - Restart a job (stop + start)
- `gob cleanup` - Remove metadata for stopped jobs
```

## Interactive TUI

Launch a full-screen terminal interface for managing jobs:

```bash
gob tui
```

![TUI Screenshot](assets/tui.png)

### Layout

The TUI has three panels:

- **Panel 1 (Jobs)**: List of all jobs with status (● running, ○ stopped)
- **Panel 2 (stdout)**: Standard output of selected job (80% height)
- **Panel 3 (stderr)**: Standard error of selected job (20% height)

### Key Bindings

**Panel Navigation:**
- `1` / `2` / `3` - Jump to specific panel
- `Tab` - Cycle through panels

**Jobs Panel (1):**
- `↑` / `k` - Move selection up
- `↓` / `j` - Move selection down
- `s` - Stop selected job (SIGTERM)
- `S` - Force kill selected job (SIGKILL)
- `r` - Restart selected job
- `d` - Delete stopped job
- `n` - Start new job (opens input prompt)
- `a` - Toggle show all directories

**Log Panels (2/3):**
- `↑` / `k` - Scroll up
- `↓` / `j` - Scroll down
- `g` - Go to top
- `G` - Go to bottom
- `f` - Toggle follow mode (auto-scroll)

**Global:**
- `?` - Show help
- `q` - Quit

## CLI Reference

### Job Management

#### `gob add <command> [args...]`

Create and start a new background job. The job runs detached and persists even after the CLI exits.

```bash
# Add a long-running server
gob add python -m http.server 8000

# Add with follow flag to watch output
gob add -f make build
```

**Flags:**
- `-f, --follow` - Follow output until job completes

#### `gob remove <job_id>`

Remove metadata for a single stopped job. Job must be stopped first.

```bash
gob remove V3x0QqI
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

### Process Control

#### `gob start <job_id>`

Start a stopped job. Fails if job is already running (use `restart` instead).

```bash
gob start V3x0QqI

# Start and follow output
gob start -f V3x0QqI
```

**Flags:**
- `-f, --follow` - Follow output until job completes

#### `gob stop <job_id>`

Stop a running job. Uses SIGTERM by default; use `--force` for SIGKILL.

```bash
# Graceful shutdown
gob stop V3x0QqI

# Force kill
gob stop V3x0QqI --force
```

#### `gob restart <job_id>`

Stop (if running) and start a job. Works on both running and stopped jobs.

```bash
gob restart V3x0QqI

# Restart and follow output
gob restart -f V3x0QqI
```

**Flags:**
- `-f, --follow` - Follow output until job completes

#### `gob signal <job_id> <signal>`

Send a custom signal to a job.

```bash
# Send SIGHUP (reload configuration)
gob signal V3x0QqI HUP

# Send SIGUSR1
gob signal V3x0QqI USR1
```

**Supported signals**: TERM, KILL, HUP, INT, QUIT, USR1, USR2, and more

### Convenience

#### `gob run <command> [args...]`

Run a command and follow its output until completion. Reuses existing stopped job with the same command instead of creating a new one. When reusing a job, previous logs are cleared so you only see output from the current run.

```bash
# Simple commands
gob run make test

# Commands with flags work directly
gob run pnpm --filter web typecheck
gob run ls -la
```

### Output

#### `gob logs`

Follow stdout and stderr for all jobs in the current directory.

```bash
gob logs
```

#### `gob stdout <job_id>`

Display stdout output for a job.

```bash
# View stdout
gob stdout V3x0QqI

# Tail stdout in real-time
gob stdout V3x0QqI --follow
```

#### `gob stderr <job_id>`

Display stderr output for a job.

```bash
# View stderr
gob stderr V3x0QqI

# Tail stderr in real-time
gob stderr V3x0QqI --follow
```

### Other

#### `gob list`

Display all jobs with their status (running/stopped), PID, and command.

```bash
gob list

# Show jobs from all directories
gob list --all
```

#### `gob overview`

Display usage patterns and common workflows. Also shown when running `gob` without arguments.

```bash
gob overview
```

## Contributing

Interested in contributing? Check out [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing instructions, and contribution guidelines.
