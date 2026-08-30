---
name: gob
description: >
  Use gob to run and manage background processes: dev servers, watchers, builds, and any long-running command you want to start now and check on later.
---

# Background Jobs with `gob`

Run any process that outlives a single command through `gob`: start it now, then
read its output on a later turn instead of blocking. The user shares your view of
every job, its logs, and its status, so a running process can pass back and forth.
Every job has a 3-character ID (e.g. `abc`).

## When: long-running or background, not one-shots

Use `gob` for:
- **Servers**: `gob add -- npm run dev`
- **Long-running processes**: `gob add -- npm run watch`
- **Builds**: `gob run -- make build`
- **Parallel steps**: start several builds concurrently, then await each

Do NOT use `gob` for:
- Quick commands: `git status`, `ls`, `cat`
- CLI tools: `jira`, `kubectl`, `todoist`
- File operations: `mv`, `cp`, `rm`

## How: pass an argv, not a shell line

The `<command>` is a binary and its arguments, not a shell line. No shell interprets
it, so pipes, `&&`, globs, and `$VAR` do not expand. Use `--` to separate `gob` flags
from the command. For shell features, invoke a shell explicitly.

```
gob add -- make test
gob run -- npm run build
gob add -- bash -c "a && b"       # when you actually need the shell
```

## Which: run to wait, add to background, then inspect by ID

- `gob run <cmd>` — run and wait for completion (output shown on failure)
- `gob run --description "context" <cmd>` — run with a description for context
- `gob add <cmd>` — start in the background, returns a job ID
- `gob add --description "context" <cmd>` — start with a description
- `gob await <job_id>` — wait for a job to finish, streaming output in real time
- `gob start <job_id>` — start a stopped job
- `gob stop <job_id>` — graceful stop (`--force` for SIGKILL)
- `gob restart <job_id>` — stop then start
- `gob signal <job_id> <sig>` — send a signal to a job
- `gob list` — list jobs with IDs, status, and descriptions
- `gob logs [<job_id>]` — view/follow stdout and stderr (stdout→stdout, stderr→stderr)
- `gob stdout <job_id>` — view stdout (`--follow` to stream)
- `gob stderr <job_id>` — view stderr (`--follow` to stream)
- `gob remove <job_id>` — remove a stopped job
- `gob shutdown` — stop all running jobs and shut down the daemon

`gob tui` opens a full-screen interface for the human to monitor jobs. It is
interactive and blocks, so do not run it yourself.

## What: on a stall, inspect and keep waiting

`gob run` and `gob await` flag a job as potentially stuck when there is no output for
1 min past its expected duration (average duration + 1 min, or 5 min with no history).
The job keeps running regardless. Inspect with `gob logs <id>` or `gob stdout <id>`,
then `gob await <id>` to keep waiting.

## Examples

```
gob add -- npm run dev                                 # start a dev server
gob add --description "File watcher" -- npm run watch  # background, with context
gob run -- make build                                  # run a build, wait for it
gob run --description "Type check" -- npm run typecheck
```
