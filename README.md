# gob

[![GitHub Release](https://img.shields.io/github/release/juanibiapina/gob.svg)](https://github.com/juanibiapina/gob/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/juanibiapina/gob)](https://goreportcard.com/report/github.com/juanibiapina/gob)
![Go](https://img.shields.io/github/languages/top/juanibiapina/gob)
![Languages](https://img.shields.io/github/languages/count/juanibiapina/gob)
[![Contributors](https://img.shields.io/github/contributors/juanibiapina/gob)](https://github.com/juanibiapina/gob/graphs/contributors)
![Last Commit](https://img.shields.io/github/last-commit/juanibiapina/gob)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/juanibiapina/gob)

> Process manager for AI agents (and humans).

`gob` (pronounced job, of course) is a CLI for managing background processes with a shared interface for you and your AI coding agent.

Start a dev server with Claude Code, check its logs yourself. Or vice-versa. The agent can monitor what you started. Everyone has the same view.

No more "can you check if that's still running?" No more copy-pasting logs through chat. Just direct access to your processes, for everyone.

![demo](assets/demo.gif)

[View on asciinema](https://asciinema.org/a/OgSVPWybeSvXcQVyLQ0mie8P2)

## Features

- **Interactive TUI** - Full-screen terminal interface with real-time job status and log streaming
- **MCP server** - Native Model Context Protocol support for AI agents like Claude Code
- **AI agent friendly** - Shared view of all processes for you and your coding agent
- **Real-time sync** - Changes from CLI instantly appear in TUI, and vice-versa
- **Per-directory jobs** - Jobs are scoped to directories, keeping projects organized
- **Process lifecycle control** - Start, stop, restart, send signals to any job

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
gob stdout abc

# Stop a job
gob stop abc

# Clean up all stopped jobs
gob cleanup
```

## Using with AI Coding Agents

For AI agents that don't support MCP, add the following instructions to your agent's configuration file (`CLAUDE.md`, `AGENTS.md`, etc).

<details>
<summary>General instructions for AI agents</summary>

````markdown
## Background Jobs with `gob`

Use `gob` to manage background processes.

### Running Commands

- `gob add -- <cmd>` - Starts command, returns job ID immediately
  - IMPORTANT: Always use `--` before the command
- `gob await <job_id>` - Wait for job to finish, stream output, return exit code

### Sequential Execution

For commands that must complete before proceeding:

```
gob add -- make build
```

Then immediately:

```
gob await <job_id>
```

Use for: builds, installs, any command where you need the result.

### Parallel Execution

For independent commands, start all jobs first:

```
gob add -- npm run lint
gob add -- npm run typecheck
gob add -- npm test
```

Then collect results using either:

- `gob await <job_id>` - Wait for a specific job by ID
- `gob await-any` - Wait for whichever job finishes first

Example with await-any:

```
gob await-any   # Returns when first job finishes
gob await-any   # Returns when second job finishes
gob await-any   # Returns when third job finishes
```

Use for: linting + typechecking, running tests across packages, independent build steps.

### Job Monitoring

**Status:**
- `gob list` - List jobs with IDs and status

**Output:**
- `gob await <job_id>` - Wait for completion, stream output (preferred)

**Control:**
- `gob stop <job_id>` - Graceful stop
- `gob stop --force <job_id>` - Force kill
- `gob restart <job_id>` - Stop + start
- `gob cleanup` - Remove stopped jobs

### Examples

Good:
  gob add -- make test
  gob await <job_id>
  gob add -- npm run dev

Bad:
  make test                 # Missing gob prefix
  npm run dev &             # Never use & - use gob add
  gob add npm run --flag    # Missing -- before flags
````

</details>

<details>
<summary>Instructions for Crush (AI assistant)</summary>

```
<shell_commands>
ALWAYS use `gob add` to run shell commands through the Bash tool.

- `gob add -- <cmd>` - Starts command, returns job ID immediately
  - IMPORTANT: Always use `--` before the command
- `gob await <job_id>` - Wait for job to finish, stream output, return exit code
</shell_commands>

<sequential_execution>
For commands that must complete before proceeding:

gob add -- make build

Then immediately:

gob await <job_id>

Use for: builds, installs, any command where you need the result.
</sequential_execution>

<parallel_execution>
For independent commands, start all jobs first:

gob add -- npm run lint
gob add -- npm run typecheck
gob add -- npm test

Then collect results using either:

- `gob await <job_id>` - Wait for a specific job by ID
- `gob await-any` - Wait for whichever job finishes first

Example with await-any:

gob await-any   # Returns when first job finishes
gob await-any   # Returns when second job finishes
gob await-any   # Returns when third job finishes

Use for: linting + typechecking, running tests across packages, independent build steps.
</parallel_execution>

<job_monitoring>
**Status:**
- `gob list` - List jobs with IDs and status

**Output:**
- `gob await <job_id>` - Wait for completion, stream output (preferred)

**Control:**
- `gob stop <job_id>` - Graceful stop
- `gob stop --force <job_id>` - Force kill
- `gob restart <job_id>` - Stop + start
- `gob cleanup` - Remove stopped jobs
</job_monitoring>

<auto_background_handling>
The Bash tool automatically backgrounds commands that exceed 1 minute.

When this happens, IGNORE the shell ID returned by the Bash tool. Instead:

1. Use `gob await <job_id>` to wait for completion again
2. Do NOT use Crush's job_output or job_kill tools
</auto_background_handling>

<examples>
Good:
  gob add -- make test
  gob await V3x
  gob add -- npm run dev
  gob add -- timeout 30 make build

Bad:
  make test                 # Missing gob prefix
  gob run make test         # Don't use run, use add + await
  npm run dev &             # Never use & - use gob add
  gob add npm run --flag    # Missing -- before flags
</examples>
```

</details>

## MCP Server

gob includes a native MCP (Model Context Protocol) server, allowing AI agents to manage background jobs directly through tool calls instead of CLI commands.

### Available Tools

| Tool | Description |
|------|-------------|
| `gob_add` | Create a new background job |
| `gob_list` | List jobs in current directory |
| `gob_stop` | Stop a running job |
| `gob_start` | Start a stopped job |
| `gob_remove` | Remove a stopped job |
| `gob_restart` | Stop and start a job |
| `gob_signal` | Send a signal to a job |
| `gob_await` | Wait for a job to complete |
| `gob_await_any` | Wait for any job to complete |
| `gob_await_all` | Wait for all jobs to complete |
| `gob_stdout` | Read stdout from a job |
| `gob_stderr` | Read stderr from a job |
| `gob_cleanup` | Remove all stopped jobs |
| `gob_nuke` | Stop all, remove all, shutdown daemon |

### Configuration

<details>
<summary>Claude Code</summary>

Create `.mcp.json` in your project directory:

```json
{
  "mcpServers": {
    "gob": {
      "command": "gob",
      "args": ["mcp"]
    }
  }
}
```

Or add to `~/.claude.json` for global availability.

</details>

<details>
<summary>Crush</summary>

Create `crush.json` in your project directory:

```json
{
  "$schema": "https://charm.land/crush.json",
  "mcp": {
    "gob": {
      "type": "stdio",
      "command": "gob",
      "args": ["mcp"]
    }
  }
}
```

</details>

<details>
<summary>Codex</summary>

Add to `~/.codex/config.toml`:

```toml
[mcp_servers.gob]
command = "gob"
args = ["mcp"]
```

</details>

<details>
<summary>OpenCode</summary>

Create `opencode.json` in your project directory:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "gob": {
      "type": "local",
      "command": ["gob", "mcp"]
    }
  }
}
```

</details>

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

Press `?` in the TUI to see all keyboard shortcuts.

## CLI Reference

Run `gob <command> --help` for detailed usage, examples, and flags.

| Command | Description |
|---------|-------------|
| `run <cmd>` | Run command, wait for completion (reuses stopped jobs) |
| `add <cmd>` | Start background job (use `--` before flags: `add -- cmd --flag`) |
| `await <id>` | Wait for job, stream output, show summary |
| `await-any` | Wait for any job to complete (`--timeout`) |
| `await-all` | Wait for all jobs to complete (`--timeout`) |
| `list` | List jobs (`--all` for all directories) |
| `stdout <id>` | View stdout (`--follow` for real-time) |
| `stderr <id>` | View stderr (`--follow` for real-time) |
| `logs` | Follow all output for current directory |
| `stop <id>` | Stop job (`--force` for SIGKILL) |
| `start <id>` | Start stopped job |
| `restart <id>` | Stop + start job |
| `signal <id> <sig>` | Send signal (HUP, USR1, etc.) |
| `remove <id>` | Remove stopped job |
| `cleanup` | Remove all stopped jobs |
| `nuke` | Stop all, remove all, shutdown daemon |
| `tui` | Launch interactive TUI |
| `mcp` | Start MCP server for AI agents |

## Telemetry

`gob` collects anonymous usage telemetry to help inform development priorities. Only usage metadata is collected; command arguments and output are never recorded.

You can opt out by setting `GOB_TELEMETRY_DISABLED=1` or `DO_NOT_TRACK=1` in your environment.

See [docs/telemetry.md](docs/telemetry.md) for details on what's collected.

## Contributing

Interested in contributing? Check out [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing instructions, and contribution guidelines.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=juanibiapina/gob&type=Date)](https://star-history.com/#juanibiapina/gob&Date)
