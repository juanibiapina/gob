# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Shell completion support for Bash, Zsh, Fish, and PowerShell with dynamic job ID suggestions

## [0.3.0] - 2025-11-21

### Added

- `--all` flag to `list`, `cleanup`, and `nuke` commands to operate across all directories
- `--workdir` flag to `list` command to display working directory for each job

### Changed

- **BREAKING**: Storage migrated to XDG-compliant directory (`~/Library/Application Support/gob` on macOS, `~/.local/share/gob` on Linux)
- Job IDs now use nanosecond precision timestamps instead of second precision for better uniqueness
- `nuke` command now removes log files in addition to metadata files

### Migration Notes

- Existing jobs in project-local `.local/share/gob` directories will no longer be visible after upgrade
- Old jobs continue running but won't appear in `gob list`
- Users should manually clean up old `.local` directories in projects when ready

## [0.2.0] - 2025-11-20

### Fixed

- Fix process hanging by ensuring child processes are properly terminated

## [0.1.1] - 2025-11-20

- Add support for installing with homebrew

## [0.1.0] - 2025-11-17

First stable release.

## [0.1.0-beta.1] - 2025-11-17

Initial test release.
