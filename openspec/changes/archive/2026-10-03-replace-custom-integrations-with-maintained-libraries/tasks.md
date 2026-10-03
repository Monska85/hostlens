# Tasks

## 1. Versions and compatibility

- [x] 1.1 Verify latest stable upstream tags, Go requirements, licenses, and APIs for every candidate and update `go.mod`, `go.sum`, Go/tool pins; verify `go mod tidy`, module graph, and cross-builds.
- [x] 1.2 Update release and development documentation for changed versions and verify their commands against the pinned tools.

## 2. Docker observer

- [x] 2.1 Replace suitable custom Docker response types with Moby API types behind fixed read-only operations; verify socket validation, GET-only transport, byte ceilings, version compatibility, and all observer fixtures in containers.
- [x] 2.2 Evaluate Moby client and log framing against mutation isolation and memory bounds; retain guarded code where incompatible and verify TTY, multiplexed, malformed, and oversized streams in container tests.
- [x] 2.3 Update observer documentation and run private live Docker acceptance with the packaged binaries.

## 3. Linux and lifecycle helpers

- [x] 3.1 Evaluate procfs parsing against policy-owned reads and bounds; adopt only compatible APIs and verify process, network, and health fixture tests in containers.
- [x] 3.2 Evaluate go-systemd unit helpers and use `os.Root` for archive extraction; verify archive traversal and systemd acceptance in disposable containers.
- [x] 3.3 Document any retained custom safety behavior and verify the operator guidance is accurate.

## 4. Final integration

- [x] 4.1 Run formatting, lint, race, vet, portable build, vulnerability, package, and strict OpenSpec checks; record exact passes and limits in validation documentation.
- [x] 4.2 Review the complete diff against contracts and security boundaries and verify it is ready for the single final commit.
