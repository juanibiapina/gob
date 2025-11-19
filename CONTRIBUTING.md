# Contributing to gob

**Thank you for your interest in `gob`. Your contributions are highly welcome.**

## Prerequisites

This project uses a `Makefile` to manage build scripts.
You will need `make` installed to run these scripts.
See [Makefile](Makefile) for a list of possible commands and what they do.

You will need to have a `go` installation - ideally compatible with the project's current go version (see [go.mod](go.mod)).

### macOS

In order to use `make`, install [apple developer tools](https://developer.apple.com/xcode/resources/).

### Getting Started

```bash
# Clone the repository
git clone https://github.com/juanibiapina/gob.git
cd gob

# Initialize git submodules (required for testing)
git submodule update --init --recursive
```

## Building

To build the project:

```bash
make build
```

Binary output: `dist/gob`

You can test the binary locally by running it directly:

```bash
./dist/gob --version
```

## Testing

### Requirements

- BATS (included as git submodule)
- `jq` (JSON processor)

### Running Tests

```bash
# Run the test suite
make test
```

Tests are located in `test/*.bats` and verify end-to-end functionality of the CLI.

## Making Changes

### Changelog Updates

When making user-facing changes to the project:

1. Update `CHANGELOG.md` under the `[Unreleased]` section
2. Follow the [Keep a Changelog](https://keepachangelog.com/) format
3. Use appropriate categories:
   - **Added** - New features
   - **Changed** - Changes to existing functionality
   - **Deprecated** - Soon-to-be removed features
   - **Removed** - Removed features
   - **Fixed** - Bug fixes
   - **Security** - Security improvements

## Release Process

For information about the release process, see [docs/releases.md](docs/releases.md).
