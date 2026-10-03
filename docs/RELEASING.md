# Release HostLens

The repository publishes from a version tag on the default branch. The tag workflow reruns CI, builds the exact candidate, verifies both Linux archives, checksums, and the MCP contract snapshot, then publishes a final GitHub release with changelog notes. Do not create a separate release manually.

## Toolchain

Use the Go version in `go.mod`, Docker, Python 3, GoReleaser v2, ShellCheck, and the repository's locked formatting and workflow tools. `make deps` installs the Go modules and local development tools; `make test-image` prepares the disposable check image. `make help` and `just help` show equivalent task entry points. Check current tool versions in `tools/dev` and CI before a release.

## Local validation

1. Run `make fmt-check`, `make lint`, and `make test`. These run formatting and static checks plus the Go race suite, vet, both Linux builds, portable package builds, and an SDK-generated contract comparison. The Go tests run in a disposable container.
2. Run `make prepare-live-docker` and `make test-live-docker` for the pinned fixture inside a private nested Engine. The fixture runner rejects the administrator's Docker socket.
3. Run `make package`, `make verify-archives`, `make systemd-image`, and `make test-systemd`. The systemd case starts a disposable container and exercises the archived binary and unit files. Run the arm64 case on a native arm64 runner; a cross-build alone is not runtime evidence.
4. Run `make scan-vulnerabilities` and strict OpenSpec validation. Report unavailable external scans or container capabilities as unavailable, never as passes.

All installation, upgrade, uninstall, Docker mutation, and systemd tests belong in disposable containers. Never run lifecycle tests on the administrator's host.

## Prepare a version

Choose SemVer from the observable contract and migration impact. Add a dated section to `CHANGELOG.md` following Keep a Changelog, with a compare link immediately after its heading. Keep `Unreleased` on top. Sweep release examples with the command in `AGENTS.md`, updating stale example versions while preserving historical records.

Commit the changelog and examples on the default branch. Verify its CI before creating and pushing an unused `vMAJOR.MINOR.PATCH` tag at that commit. The release workflow checks that the tag belongs to default-branch history, reruns CI, packages the tested candidate, verifies assets, and publishes the final release. Inspect checksums, release notes, CI, and native arm64 evidence after publication. If a transient workflow step fails, rerun it against the same tag. If code or assets need correction, keep the tag immutable and ship the correction under a new patch version.

A branch build uses the `0.1.0-dev` package version placeholder. GoReleaser replaces it in candidate binaries and shipped docs. It is not a published version.
