# Proposal: fix-observer-release-version

## Why

`hostlens-docker-observer` carries its own `Version = "0.1.0-dev"` literal (`internal/observerapp/main_linux.go:20`) that no release build injects, so tagged releases ship an observer reporting a stale development version while `hostlens` and `hostlens-diagnostics` report the release version. The comment claiming the archive builder sets it is false. `release-validation` requires binary, archive, and manifest versions to agree; the observer currently violates that on tagged builds.

## What Changes

- Delete `observerapp.Version`; the `version` subcommand prints `contract.Version` (single source of truth; no import cycle — `contract` is a leaf).
- Add `hostlens-docker-observer` to the version assertion loop in `scripts/platform-smoke.sh`.
- Add an observer version assertion to the systemd acceptance script where the installed observer binary is already checked.
- CHANGELOG gains a `Fixed` entry: tagged-release observer binary reported a stale development version.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation` — the versioned-delivery requirement now names every shipped executable (gateway, diagnostics backend, Docker observer) in the version-agreement clause, with a matching scenario.

## Impact

- `internal/observerapp/main_linux.go`, `scripts/platform-smoke.sh`, `scripts/systemd-acceptance.sh`.
- Archive contents unchanged; only the printed observer version changes on tagged builds. Development builds of all three executables keep reporting `0.1.0-dev`.