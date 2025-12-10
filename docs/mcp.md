# MCP Server

gob includes an MCP (Model Context Protocol) server that exposes job management to AI agents and MCP-compatible clients.

## Quick Start

```bash
# Start the MCP server (used by AI agents, not directly)
gob mcp
```

### Claude Code Configuration

Create `.mcp.json` in your project:

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

## Tools

### Job Creation

#### `gob_add`

Create a new background job in the current directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | `array` | Yes | Command and arguments (e.g. `["make", "build"]`) |

```json
// Returns
{"job_id": "V3x", "status": "running", "pid": 12345}
```

### Job Listing

#### `gob_list`

List jobs in current directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `all` | `boolean` | No | Include jobs from all directories (default: false) |

```json
// Returns
{"jobs": [{"job_id": "V3x", "status": "running", "command": ["make", "build"], "pid": 12345}]}
```

### Job Control

#### `gob_stop`

Stop a running job.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |
| `force` | `boolean` | No | Use SIGKILL instead of SIGTERM |

#### `gob_start`

Start a stopped job.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |

#### `gob_restart`

Stop and start a job.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |

#### `gob_signal`

Send a signal to a running job.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |
| `signal` | `string` | Yes | Signal name (HUP, SIGTERM) or number (1, 15) |

#### `gob_remove`

Remove a stopped job.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |

### Awaiting Completion

#### `gob_await`

Wait for a job to complete and return its output.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |
| `timeout` | `integer` | No | Timeout in seconds (default: 300) |

```json
// Returns
{"job_id": "V3x", "status": "stopped", "exit_code": 0, "stdout": "...", "stderr": "..."}
```

Output is truncated at 100KB per stream.

#### `gob_await_any`

Wait for any job in current directory to complete.

| Parameter | Type | Required | Description |
|-----------|------|----------|---------|
| `timeout` | `integer` | No | Timeout in seconds (default: 300) |

#### `gob_await_all`

Wait for all jobs in current directory to complete.

| Parameter | Type | Required | Description |
|-----------|------|----------|---------|
| `timeout` | `integer` | No | Timeout in seconds (default: 300) |

```json
// Returns
{"jobs": [{"job_id": "V3x", "exit_code": 0}], "all_succeeded": true}
```

### Reading Output

#### `gob_stdout`

Read stdout from a job (running or completed).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |

```json
// Returns
{"job_id": "V3x", "content": "..."}
```

#### `gob_stderr`

Read stderr from a job (running or completed).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | `string` | Yes | Job ID |

### Bulk Operations

#### `gob_cleanup`

Remove all stopped jobs in current directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|---------|
| `all` | `boolean` | No | Remove from all directories (default: false) |
