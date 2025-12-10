# Version Negotiation

## Goal

Automatically restart the daemon when the CLI version changes, unless there are running jobs.

## How It Works

1. On connect, client sends a `version` request to daemon
2. Old daemons return `"unknown request type: version"` → treated as version mismatch
3. New daemons return `{"version": "x.y.z", "running_jobs": N}`
4. If versions match → continue
5. If versions differ and no running jobs → restart daemon automatically
6. If versions differ and has running jobs → error (user must run `gob nuke`)

Exception: `nuke` command skips version check entirely.

## Key Files

- `internal/daemon/protocol.go` - `RequestTypeVersion` constant
- `internal/daemon/daemon.go` - `handleVersion()` returns version and job count
- `internal/daemon/client.go` - `CheckDaemonVersion()` implements the logic above
- `cmd/nuke.go` - uses `ConnectSkipVersionCheck()` to bypass

## Adding New Protocol Features

When adding features that require daemon changes:
1. The version check ensures clients always talk to a matching daemon
2. No need for backward compatibility - mismatched versions trigger restart
3. If restart isn't possible (running jobs), user is prompted to `nuke`
