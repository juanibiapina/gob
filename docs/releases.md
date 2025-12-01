# Releases

## Creating a Release

Releases are automated via GitHub Actions and triggered by git tags.

1. Update `CHANGELOG.md`: move `[Unreleased]` entries to new version section with date
2. Commit the changelog update
3. Create and push a version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow automatically:
- Builds binaries for Linux and macOS (amd64, arm64)
- Creates a GitHub release
- Uploads binaries and checksums as release assets

After pushing the tag, the coding agent should:
1. Wait for the GitHub Actions workflow to complete successfully
2. Use `gh release view` to verify the release was created
3. Use `gh release edit` to add release notes describing the changes
4. Create an announcement in GitHub Discussions using `gh api graphql` (see repo discussions for format)

## Version Format

Follow [semantic versioning](https://semver.org/):

- **Production**: `v1.2.3`
- **Pre-release**: `v1.0.0-beta.1`, `v1.0.0-rc.1`, `v1.0.0-alpha.1`

Pre-release tags (containing `-`) are automatically marked as pre-releases on GitHub.
