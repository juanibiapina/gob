# Agent Instructions

This file contains instructions for AI agents working on this codebase.

## Running shell commands with `gob`

Use `gob` to run shell commands.

**When to use `run` (blocks until complete):**
- Running tests: `gob run make test`
- Build commands: `gob run make build`
- Linting/formatting: `gob run npm run lint`
- Any command where you need to see the result before proceeding

**When to use `add` (returns immediately):**
- Dev servers: `gob add npm run dev`
- Watch modes: `gob add npm run watch`
- Long-running services: `gob add -- python -m http.server`
- Any command that runs indefinitely

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
