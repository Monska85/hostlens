# Design: fix-observer-release-version

## Context

`internal/contract` already carries the single version injected by both goreleaser before-hook builds. The observer duplicates it and no `-X` flag targets it, so tagged releases report `0.1.0-dev` from `hostlens-docker-observer version`. See proposal.md — Why.

## Goals / Non-Goals

- Goals: one version literal; acceptance assertions that would have caught the defect on every candidate.
- Non-Goals: no changes to archive layout, manifest schema, or the gateway/backend version handling; no new build flags.

## Decisions

- **Delete the duplicate instead of adding a second `-X` injection**: `contract` is a leaf package (no import cycle), so printing `contract.Version` keeps a single source of truth and cannot drift again. Two injection sites would duplicate the release contract the audit already flagged.
- **Assert in both platform smoke and systemd acceptance**: the platform smoke loop covers archive contents on every distro case; the systemd case checks the installed `/usr/local/bin` observer, which is the binary that actually runs on a host. Cheap assertions, no new tooling.

## Risks / Trade-offs

- [`observerapp` now depends on `internal/contract`] → Leaf dependency, same dependency the gateway and CLI already take; no cycle.
- [Systemd assertion depends on `HOSTLENS_VERSION` being set] → `tools/test-matrix/main.py` already exports it for every matrix case; a missing value would fail loudly rather than silently skip.

## Migration Plan

None. Rollback is a plain revert.

## Open Questions

None.