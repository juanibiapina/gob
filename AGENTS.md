# Project Notes

## Build and Test

- `make test` already depends on `make build`
- No need to run `make build` separately before running tests

## Release Process

- See [docs/releases.md](docs/releases.md) for the release process

## Nix

- When changing Go dependencies (`go.mod`), update the Nix flake `vendorHash`
- If the build fails, the error message contains the correct hash to use

See [docs/nix.md](docs/nix.md) for details.
