# Gobfile

## Overview

A gobfile is a TOML configuration file that defines jobs for your project. When you launch the TUI (`gob tui`), jobs defined in the gobfile are automatically added and optionally started.

**Location:** `.config/gobfile.toml` in your project directory

## File Format

```toml
[[job]]
command = "npm run dev"
description = "Development server for the frontend app"

[[job]]
command = "npm run build:watch"
description = "Watches TypeScript and rebuilds on change"
autostart = false

[[job]]
command = "docker compose up db"
# description and autostart are optional
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `command` | string | Yes | - | The command to run |
| `description` | string | No | - | Context about the job, shown in TUI and CLI |
| `autostart` | boolean | No | `true` | Whether to auto-start when TUI opens and auto-stop when TUI exits |

## Behavior

### When TUI Opens

1. Gobfile is parsed from `.config/gobfile.toml`
2. For each job in the file:
   - If a job with the same command exists and is running: update description if different
   - If a job with the same command exists and is stopped: restart it (if `autostart = true`)
   - If no matching job exists: create and start it (if `autostart = true`)
   - If `autostart = false`: create but don't start (shows as stopped)
3. Jobs are started asynchronously (TUI doesn't wait for them)

### When TUI Exits

Jobs with `autostart = true` (the default) are stopped when the TUI exits:
- Normal exit (pressing `q`)
- Terminal close
- SIGTERM/SIGINT signals

Jobs with `autostart = false` are **not** stopped. This allows you to define jobs that are manually controlled - you start them when needed and they keep running after the TUI exits.

### Description Updates

Descriptions from the gobfile are synced to jobs every time the TUI opens:
- New jobs get the description from the gobfile
- Existing jobs (running or stopped) have their description updated if the gobfile specifies a different one
- The TUI receives a `job_updated` event and refreshes the display automatically

## Use Cases

### Development Environment

Define all services needed for local development:

```toml
[[job]]
command = "npm run dev"
description = "Next.js dev server on port 3000"

[[job]]
command = "npm run api:dev"
description = "API server on port 4000"

[[job]]
command = "docker compose up postgres redis"
description = "Database and cache services"
```

### Build Watchers

Add watchers that rebuild on file changes:

```toml
[[job]]
command = "npm run typecheck:watch"
description = "TypeScript type checking in watch mode"

[[job]]
command = "npm run test:watch"
description = "Jest tests in watch mode"
autostart = false  # Start manually, keeps running after TUI exits
```

### AI Agent Context

Descriptions help AI agents understand what each job does:

```toml
[[job]]
command = "npm run dev"
description = "Frontend dev server. Check this for UI errors. Runs on http://localhost:3000"

[[job]]
command = "npm run api"
description = "Backend API. Check logs here for request/response debugging"

[[job]]
command = "npm run storybook"
description = "Component library browser. Use for visual component testing"
autostart = false
```

When an AI agent runs `gob list`, it sees the descriptions and understands the purpose of each job.

## CLI Descriptions

Descriptions can also be set via CLI flags, independently of the gobfile:

```bash
# Add a job with description
gob add --description "Build output watcher" npm run build:watch

# Run a command with description
gob run --description "Running full test suite" npm test
```

This is useful for:
- One-off jobs that aren't in the gobfile
- Providing additional context for specific runs
- AI agents documenting why they started a job

## Version Control

Consider whether to commit your gobfile:

**Commit it** if:
- The jobs are standard for all developers
- The configuration doesn't contain machine-specific paths
- You want consistent development environments

**Don't commit it** if:
- Jobs vary by developer preferences
- Contains machine-specific configuration
- You want each developer to customize their setup

To exclude from version control:
```bash
echo ".config/gobfile.toml" >> .gitignore
```

## Troubleshooting

### Jobs not starting

1. Check the file location: must be `.config/gobfile.toml` (not project root)
2. Verify TOML syntax: use a TOML validator
3. Check daemon logs: `$XDG_STATE_HOME/gob/daemon.log`

### Jobs restarting unexpectedly

Jobs are restarted if they were stopped and match a gobfile entry with `autostart = true`. To prevent this, either:
- Set `autostart = false` for jobs you want to control manually
- Remove the job from the gobfile

### Description not showing

- Ensure the job was created with a description (either from gobfile or `--description` flag)
- In CLI: descriptions appear on the second line of `gob list` output
- In TUI: description panel only appears when the selected job has one
