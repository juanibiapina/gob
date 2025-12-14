# Environment Variables

## Overview

When a job runs, it executes with a specific set of environment variables. `gob` captures the environment from the client (CLI, TUI, or MCP) at the time of each operation and passes it to the daemon, which starts the process with that exact environment.

**Key principle:** Jobs run with the client's environment, not the daemon's environment.

## Behavior

### Environment Capture

Every time a job runs, the client captures its current environment (`os.Environ()`) and sends it to the daemon:

| Command | Environment Used |
|---------|------------------|
| `gob run` | Current client environment |
| `gob add` | Current client environment |
| `gob start` | Current client environment |
| `gob restart` | Current client environment |

The environment is **not persisted**. Each operation captures a fresh environment from the client.

### Clean Environment

The spawned process receives **only** the environment passed by the client. It does not inherit any environment variables from the daemon process. This ensures:

- Jobs run with predictable, explicit environments
- The daemon's environment (set when it started) doesn't leak into jobs
- Different clients can run the same job with different environments

### Examples

```bash
# Job runs with FOO=bar
export FOO=bar
gob run ./my-script   # Runs with FOO=bar
gob add ./my-script   # Starts with FOO=bar

# Later, change the environment
export FOO=baz

# Start runs with FOO=baz (current environment, not original)
gob start <job_id>

# Restart also uses FOO=baz
gob restart <job_id>
```

## Common Environment Variables

Standard environment variables like `PATH`, `HOME`, `USER`, etc., are included automatically since they're part of the client's environment.

## TUI Behavior

The TUI captures the environment when it starts. Operations initiated from the TUI (add, restart) use the environment that was captured at TUI startup, not the current shell environment.

## MCP Server Behavior

The MCP server captures the environment when it starts. All jobs created through MCP tools use the environment that was present when the MCP server was launched.

## Implementation Details

1. **Client side**: Calls `os.Environ()` and includes the result in the request payload
2. **Protocol**: Environment is sent as a JSON array of strings (`["KEY=value", ...]`)
3. **Daemon side**: Extracts environment from request and passes to job manager
4. **Executor**: Sets `cmd.Env` to the provided environment (overrides Go's default of inheriting parent environment)

See [`internal/daemon/executor.go`](../internal/daemon/executor.go) for the process execution implementation.
