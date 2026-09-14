# Design: refuse-world-accessible-docker-socket

## Context

`validateEngineSocket` enforced group access and refused world-*writable* sockets, leaving world-readable (root-equivalent) sockets accepted. See proposal.md — Why.

## Goals / Non-Goals

- Goals: fail closed on any other-user access bit; keep every other precondition identical.
- Non-Goals: no change to socket discovery, group management, or the reconcile-time socket disclosure; no new policy plumbing.

## Decisions

- **Single `mode&0o007 != 0` refusal** replacing the world-writable check: simpler, strictly stronger, and one error class instead of two near-duplicates. Docker's documented default mode is 0660 `root:docker`, so stock installs are unaffected.

## Risks / Trade-offs

- [Hardened hosts that deliberately expose a world-readable socket to containers] → Such exposure is root-equivalent and contradicts the observer's rootful-engine premise; failing closed is the intended outcome.

## Migration Plan

None. Rollback is a plain revert.

## Open Questions

None.