# Releasing

This document covers the practical steps for publishing `memctl` from this repository.

## Before The First Public Release

1. Confirm that the Go module path in `go.mod` matches the GitHub repository path.
2. Review release URLs in `README.md` and release commands after any repository rename.
3. Review `CHANGELOG.md` and move release-ready entries into a tagged version section if you want a frozen release note block.
4. Confirm that `.goreleaser.yaml` still matches the release matrix you want to ship.

## Pre-Release Checklist

Run these checks locally:

```bash
go test ./...

go build ./cmd/memctl

./memctl status
```

Recommended smoke test:

```bash
./memctl extract --history --apply

./memctl review

./memctl pack
```

What to verify:

- the binary starts and prints `memctl version`
- the default SQLite store opens correctly
- `memctl status` resolves the expected home/store paths
- `extract --history --apply` writes candidates without creating duplicates on repeat runs
- `review --accept-all` and `pack` behave as expected for your test workspace

## Release Procedure

Create and push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

That triggers `.github/workflows/release.yml`, which runs GoReleaser with:

- `CGO_ENABLED=0`
- `go test ./...`
- archives for `linux`, `darwin`, and `windows`
- architectures `amd64` and `arm64`
- checksum generation in `checksums.txt`

Expected archive names:

- `memctl_v0.1.0_linux_amd64.tar.gz`
- `memctl_v0.1.0_linux_arm64.tar.gz`
- `memctl_v0.1.0_darwin_amd64.tar.gz`
- `memctl_v0.1.0_darwin_arm64.tar.gz`
- `memctl_v0.1.0_windows_amd64.zip`
- `memctl_v0.1.0_windows_arm64.zip`

## Post-Release Checklist

After the GitHub Release is published:

1. Open the release page and verify all archives are attached.
2. Check that `checksums.txt` is present.
3. Download one archive and confirm the binary starts.
4. Copy or adapt the relevant installation commands into the release notes.
5. If you maintain package managers later, update Homebrew Tap, Scoop, or Winget manifests with the new version and checksums.

## Optional Package Manager Follow-Up

The repository is ready for direct GitHub Releases now. Package manager support can come after the first stable public tag.

Recommended order:

1. GitHub Releases
2. Homebrew Tap
3. Scoop
4. Winget

Keep the initial release path simple. Direct archives plus checksums are enough for `v0.1.0`.
