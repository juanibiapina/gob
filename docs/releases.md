# Releases

## Creating a Release

Releases are automated via GitHub Actions and triggered by git tags.

```bash
# 1. Ensure all changes are committed
git status

# 2. Create a version tag (must start with 'v')
git tag v1.0.0

# 3. Push the tag to trigger the release workflow
git push origin v1.0.0
```

The workflow automatically:
- Builds binaries for Linux and macOS (amd64, arm64)
- Creates a GitHub release with changelog
- Uploads binaries and checksums as release assets

After pushing the tag, the coding agent should:
1. Wait for the GitHub Actions workflow to complete successfully
2. Use `gh release view` to verify the release was created
3. Use `gh release edit` to add release notes describing the changes
4. Create an announcement in GitHub Discussions using `gh discussion create`

## Version Format

Follow [semantic versioning](https://semver.org/):

- **Production**: `v1.2.3`
- **Pre-release**: `v1.0.0-beta.1`, `v1.0.0-rc.1`, `v1.0.0-alpha.1`

Pre-release tags (containing `-`) are automatically marked as pre-releases on GitHub.

## Local Development

```bash
# Build with default version "dev"
make build

# Build with custom version
VERSION=1.2.3 make build
```

## Supported Platforms

- Linux: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)

**Note**: Windows is not supported due to Unix-specific process management APIs.
