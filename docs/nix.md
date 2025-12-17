# Nix Flake

## Updating vendorHash

When `go.mod` changes (dependencies added, removed, or updated), the Nix build will fail with a hash mismatch error like:

```
error: hash mismatch in fixed-output derivation:
         specified: sha256-OLD...
            got:    sha256-NEW...
```

To fix:

1. Copy the `got:` hash from the error message
2. Replace `vendorHash` in `flake.nix` with the new hash
3. Run `make nix` to verify the fix
4. Commit both `go.mod`/`go.sum` and `flake.nix`
