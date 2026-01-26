# Version Negotiation

## Goal

Detect version mismatches between client and daemon to prevent compatibility issues
and avoid version conflicts where old clients restart the daemon to an old version.

## How It Works

1. On connect, client sends a `version` request to daemon
2. Old daemons return `"unknown request type: version"` → treated as version mismatch
3. New daemons return `{"version": "x.y.z", "running_jobs": N}`
4. If versions match → continue
5. If versions differ → return `ErrVersionMismatch` error

Exception: `shutdown` command skips version check entirely (uses `ConnectSkipVersionCheck()`).

## Behavior on Version Mismatch

**CLI commands:** Display error message with both versions and instruction to run `gob shutdown`.

**TUI:** Quit gracefully with error message displayed to stderr.

This design prevents version conflicts where:
1. User upgrades gob and runs `gob shutdown`
2. Old TUIs running in other terminals detect disconnection
3. Old TUIs would previously restart daemon with their old binary
4. New commands would fail or restart with new version, creating a cycle

By never auto-restarting the daemon on version mismatch, the user must explicitly
run `gob shutdown` and then start fresh with the new version.

## Key Files

- `internal/daemon/protocol.go` - `RequestTypeVersion` constant
- `internal/daemon/daemon.go` - `handleVersion()` returns version and job count
- `internal/daemon/client.go` - `CheckDaemonVersion()`, `ErrVersionMismatch` type
- `internal/tui/tui.go` - handles `ErrVersionMismatch` by quitting
- `cmd/shutdown.go` - uses `ConnectSkipVersionCheck()` to bypass

## Adding New Protocol Features

When adding features that require daemon changes:
1. The version check ensures clients always talk to a matching daemon
2. No need for backward compatibility - mismatched versions return errors
3. User is prompted to run `gob shutdown` and restart
