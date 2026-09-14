# Proposal: refuse-world-accessible-docker-socket

## Why

`validateEngineSocket` (`internal/observerapp/client_linux.go`) rejects only world-*writable* engine sockets. A world-readable Docker socket is root-equivalent on the host: any local user can query the engine API through it. The observer must refuse any socket granting other-user access.

## What Changes

- `validateEngineSocket` requires `mode&0o007 == 0` (no world access), keeping the group-write requirement, root ownership, and group-match checks; the error message now says world access is refused.
- Tests gain a `0644` failure case alongside the existing `0666` and passing `0660` cases.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `platform-runtime` — the observer socket-validation sentence now requires that the configured socket grants no access to other users.

## Impact

- `internal/observerapp/client_linux.go`, `internal/observerapp/bounds_test.go`.
- Deployments with a world-readable `/var/run/docker.sock` now fail closed at observer startup instead of accepting root-equivalent exposure; this matches Docker's own default socket mode (0660 root:docker).