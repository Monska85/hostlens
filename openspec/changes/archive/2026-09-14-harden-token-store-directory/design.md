# Design: harden-token-store-directory

## Context

`Store.read()` validates only the store file; `Store.change()` separately validates the parent directory (type + write bits) before taking the lock. Bearer verification runs `read()` on every request, so a directory-level swap injects credentials with arbitrary roles. See proposal.md — Why.

## Goals / Non-Goals

- Goals: one shared directory-trust check executed on both paths; fail-closed precondition errors; unchanged file-level checks.
- Non-Goals: no change to store format, hashing, rotation semantics, or the lock protocol; no path-confinement changes beyond the parent directory (`openat2` confinement elsewhere is untouched).

## Decisions

- **Shared `verifyDirTrusted` helper** instead of duplicating the check: `read()` and `change()` must never drift; the helper takes the directory path and returns a single precondition error.
- **Check order in `read()`: directory first, then file.** Rejecting an untrusted directory before opening the file avoids reading attacker-provided content at all and makes the denial reason independent of file state.
- **`Lstat` (not `Stat`)** so a symlinked directory is rejected, mirroring the plan and the file-level `O_NOFOLLOW` stance.
- **Owner predicate matches the file check** (`uid != 0 && uid != euid` rejected), which also strengthens `change()` with the owner check it previously lacked. Root-owned directories stay acceptable for euid 0 and non-root, matching the file policy and existing deployments.
- **Error message** reuses `protected token directory required` for both paths so operators see one consistent precondition failure.

## Risks / Trade-offs

- [Legitimate deployments with a group-writable token directory stop working] → Intended fail-closed hardening; the condition already violated the file-level policy, and the scenario is recorded in the spec delta.
- [`change()` now fails where it previously succeeded (non-root-owned dir)] → Consistent with `read()`; the owner of a swappable directory cannot be trusted regardless of euid.
- [TOCTOU between directory check and file open] → Unchanged exposure class for read (the audit's hard-link aliasing fix covers file-level aliasing; the directory check removes the swap-injection primitive). A fully race-free read path would require directory fds and `openat2`, recorded as future direction, not in scope for 0.2.x.

## Migration Plan

No data migration. Deployments passing the existing file checks virtually always have a root-owned, 0750-or-tighter directory (the packaged unit sets it); none are affected. Rollback is a plain revert.

## Open Questions

None.