# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Every MCP tool now publishes a result schema (`outputSchema`) next to its argument schema in `tools/list`, and the whole contract ships as the machine-readable snapshot `docs/v1/tools.json` in every release archive and as a standalone checksummed release asset; clients can validate requests and results against the published member names.
- Tool descriptions are specific, unique, and validated at startup, so a client can show what each tool observes without guessing.
- A backend result that fails its tool's published schema fails the call with a `response_shape` issue instead of returning an unvalidated payload.

### Changed

- Argument validation derives from the tool's input schema: malformed or unrelated arguments now fail with `invalid_arguments` before any backend contact.
- `get_hostlens_info` is available to `health`-role tokens whose holder has the `hostlens` audit grant (previously the `diagnostics` role).
- The diagnostics backend IPC now carries typed tool arguments; MCP clients are unaffected because the gateway owns the client-facing envelope.
- Audit and Docker tool failure paths return schema-valid empty payloads instead of bare empty `data` objects; issue codes and `isError` behavior are unchanged.

### Removed

- Nothing client-visible was removed.

## 0.3.0 - 2026-09-15

[Compare with previous release](https://github.com/Monska85/hostlens/compare/v0.2.0...v0.3.0)

### Added

- Create non-expiring bearer tokens with `hostlens token create --expires never`: the token stays valid until revoked or rotated, rotation imposes a finite retirement deadline on the old token, and `create`, `list`, and `rotate` display such tokens with an `expires` of `never`; binaries from before this change reject them instead of granting indefinite validity.

### Changed

- Overhaul the README into a landing page: workflow and fact badges, a copy-paste quick start with a minimal config, feature and refusal tables, a Mermaid architecture diagram, and a documentation matrix; all prose stays ASCII with plain punctuation.
- Document the maintainer-approved publish flow: release drafts are published on the GitHub Releases page after review instead of staying unpublished.

## 0.2.0 - 2026-09-14

[Compare with previous release](https://github.com/Monska85/hostlens/compare/v0.1.0...v0.2.0)

### Added

- Record build provenance attestations for every release asset so repository readers can verify that archive bytes were built by this repository's workflow with `gh attestation verify`; while the repository is private the workflow reports attestations as explicitly unavailable (GitHub does not offer them for user-owned private repositories) and delivery stays checksum-verified.
- Accept RFC 6750 bearer scheme spellings: `bearer` and `BEARER` authorization headers now authenticate like `Bearer` on the MCP gateway and metrics endpoint; rejection behavior for malformed credentials is unchanged.
- Refuse Docker engine sockets that grant world access: the observer previously accepted world-readable sockets, which are root-equivalent on the host; group-restricted sockets (mode 0660 root:group) are unaffected.
- Ship version-coherent documentation: archive copies of OPERATIONS.md, INSTALL.md, and VALIDATION.md now name the release version they ship in instead of stale development literals.
- Run native arm64 systemd acceptance: restricted and standard lifecycle cases execute on `ubuntu-24.04-arm` hosted runners instead of amd64-only coverage.
- Build release notes from the hand-maintained changelog section for the tagged version; the publish job fails loudly when the section is missing instead of falling back to a commit list.
- Archive documentation now ships link-clean: repository-only links to openspec records, RELEASING.md, and SPEC.md are rewritten to versioned GitHub URLs, and archive acceptance verifies every shipped link resolves.

### Changed

- Bring the gateway systemd unit to the full sandbox baseline: syscall filtering with native-architecture pinning, personality, realtime, and namespace locks now apply to the network-facing gateway, all three services protect clock, hostname, kernel logs, and IPC namespaces, and the gateway and observer additionally hide other users' processes and non-PID proc files; the diagnostic backend keeps system-wide `/proc` visibility because audit evidence reads it.
- Cut the internal test suite wall time from 18.3s to 11.9s with warm compiler cache by merging 42 zero-delta tests into their covering tests, shrinking deadline-driven waits to config values, and running independent tests in parallel; statement coverage stays at 85.8%.
- Delete verified dead code from the observer contract (`ValidName`, `UniqueSize`, `References`/`ResourceRefs`, `Response.Negotiated`, `EngineInfo` mode flags, summary size fields, `Request.Offset`/`Limit`); the diagnostic backend and observer ship version-coherent.
- Raise internal test coverage from 77.5% to 85.9% with observer transport, listener, archive, lifecycle, CLI, gateway, and CPU-health tests; document the intentionally uncovered surface in `docs/v1/VALIDATION.md`.
- Run CI once per pull request: workflow pushes now trigger only for the default branch instead of double-running the full suite for PR branches.
- Propose weekly Dependabot updates for GitHub Actions and the tools/dev Go modules; SHA-pinned action bumps remain deliberate reviews.
- Stop double-uploading goreleaser metadata to CI candidates: the tested-candidate artifact carries only archives and checksums.

### Fixed

- Ship a version-coherent Docker observer: tagged releases previously reported `0.1.0-dev` from `hostlens-docker-observer version` because the observer carried a duplicate version literal no release build injected; all three executables now share the release version and acceptance asserts it.
- Enforce the telemetry size limit for all writers: the exposition buffer embedded `bytes.Buffer`, whose promoted `ReadFrom` let `io.Copy`-style writers bypass the 1 MiB ceiling.
- Refuse bearer verification from an untrusted token store directory: reads now require the parent directory to be a real directory owned by root or the effective user without group or other write access, closing a directory-swap credential-injection path; administrative changes apply the same owner check.
- Enforce the collector inspection limit for command output: the bounded capture buffer embedded `bytes.Buffer`, whose promoted `ReadFrom` let `exec`'s copy path bypass the ceiling and capture unbounded output.
- Make release delivery idempotent: a retried publish run now refuses to create a duplicate draft when the release already exists.

## 0.1.0 - 2026-09-13

First public release: bearer-authenticated MCP diagnostics for Linux hosts with opt-in, isolated Docker observer diagnostics.

### Added

- Bearer-authenticated MCP gateway over Streamable HTTP with TLS, fixed roles (`health`, `inspect`, `diagnostics`, `metrics`), expiring tokens, and read-only enforcement of the diagnostic effect.
- Diagnostic tools for OS identity, inventory, resource health, services, and packages, plus bounded reads of approved configuration files, JSONL logs, raw tails, and journal units.
- Generic audit tools for processes, network, accounts, storage, updates, security, service units, path metadata, and HostLens runtime facts, each behind explicit policy grants.
- Source policy with profile includes, global denial precedence, provenance-tracked policy explanation, and protected built-in observation sources.
- Service metrics endpoint with a fixed, bounded Prometheus catalog and payload-free operational audit records.
- Stateless read-only MCP enforcement: exhaustive effect registry, default-on read-only gate, and cancellation-safe evidence release on every request.
- Opt-in Docker diagnostics: isolated `hostlens-docker-observer` with a typed GET-only observation contract, nine policy-controlled MCP tools, honest unused-resource analysis, and no retained Docker evidence.
- `hostlens reconcile --system` for Docker observer enablement and disablement with disclosed, ownership-tracked topology changes and socket activation under systemd.
- Installation lifecycle with durable ownership records, recoverable upgrade and rollback, conflict refusal, and Docker-preserving uninstall.
- Release pipeline with reproducible archives, per-member upgrade manifests, checksum verification, container acceptance matrix, and draft delivery to GitHub releases.
- Documentation: installation guide, operator reference, product specification, and validation evidence.
