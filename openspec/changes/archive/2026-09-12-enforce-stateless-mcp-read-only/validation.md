# Stateless MCP and read-only validation

Implementation, local acceptance, and four sf-peer-review rounds are complete with no unresolved findings. Publication and exact-commit hosted CI remain delivery gates until the signed commit is pushed.

## Executed checks

Both `make check` and `just check` passed the repository formatter, lint, full Go race suite, vet, Linux amd64/arm64 builds, portable build boundaries, and Python release, matrix, and developer-command tests. Both entry points also passed the dependency vulnerability scan and coverage workflow. Final aggregate statement coverage was 80.1% through Make and 80.2% through Just.

The candidate was built with the CI-pinned GoReleaser OSS v2.18.1 after its official checksum passed. Release configuration and both amd64/arm64 archives passed validation. The final shared matrix passed all ten disposable cases: Debian amd64, Ubuntu amd64, Arch amd64, emulated Debian arm64, restricted and standard systemd lifecycle, and the PostgreSQL, Nginx, Apache, and MySQL generic audit fixtures.

Strict validation passed for this active change, all current repository specs, both active changes, and all 14 archived changes. `CLAUDE.md` remains a relative symlink to `AGENTS.md`, and every modified Markdown file passed the external formatter.

## Invariant evidence

- Configuration tests cover omitted, explicit true, explicit false, invalid, and unknown MCP fields. Effective fingerprints and administrative status include the active value, and real-process reload acceptance covers both transitions.
- One immutable registry classifies every current tool with closed effect, role, and schema types. Gateway discovery, transport preflight, direct calls, backend IPC, authorization, capabilities, schemas, audit domains, and metric labels derive from its accessors.
- Denied and unclassified HTTP calls are rejected before backend capability discovery. Deterministic reload tests prove discovery holds one generation through response completion and that a queued reload cannot deadlock a nested tool call.
- Admission changes use an explicit prepare, commit, and rollback transaction. Tests cover both directions, backend activation failure, drain failure, partial preparation failure, and a missing transition without backend invocation.
- Every diagnostic registry entry observes a changed mutable source on the next request. Marker tests cover success, partial result, failure, timeout, and cancellation, then inspect backend state, audit logs, aggregate metrics, and managed files. Unknown tool names are normalized before logging.
- Client timeout and cancellation signal the collection worker. An uninterruptible OS read remains bounded by its occupied admission slot and active-work metric until it exits; no test or documentation claims synchronous kernel-I/O termination or memory erasure.
- Local installation, upgrade, uninstall, token administration, validation, reload, status, and telemetry remain outside MCP discovery and dispatch. The two systemd modes exercised their lifecycle and privilege-containment paths.

## Review and fixes

Round 1 used security, specification, adversarial, Go-idiom, maintainability, and performance reviewers. Verified findings covered stale read-only admission across reload, backend discovery before denied calls, non-transactional rollback, mutable registry views, unchecked string metadata, untested known-remediation logic, vacuous marker coverage, and overbroad worker-lifetime wording. All were corrected and focused race tests passed.

Round 2 rechecked every original finding. It identified stale discovery between server construction and response processing, acceptance of a nil admission transition, and four remaining stale worker-lifetime statements. All were corrected.

Round 3 returned a clean security verdict and found two medium adversarial gaps: partial preparation needed rollback on error, and the generation-lock fix needed deterministic concurrency tests. Both were corrected and focused race tests passed.

Round 4 targeted only those last fixes and returned `Reviewed: OK, nothing to change`. No fifth round was required.

## Environmental limits

All lifecycle and platform tests used disposable containers. They share the Docker engine kernel and therefore do not prove behavior under a distinct VM kernel, host init system, host cgroup hierarchy, or host mandatory-access-control policy. Arm64 execution used user-mode emulation on the amd64 engine. The matrix reports namespace-visible evidence only, and no unavailable native boundary is recorded as passed.
