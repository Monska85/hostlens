# Proposal: never-expiring-tokens

## Why

Every v1 bearer token requires a future RFC3339 expiry (`internal/token/token_linux.go` `Create`, `internal/cli/cli_linux.go`). Long-lived automation credentials therefore need repeated rotation ceremonies or an arbitrary far-future date that defeats the rotation-overlap safety net. Administrators need a first-class non-expiring option with honest semantics: valid until revoked or rotated.

## What Changes

- `hostlens token create --expires never` creates a token that verification never rejects on its date; revocation, status, and role checks remain per-call and unchanged.
- The store sentinel is the zero `expires` timestamp: no token-record schema change, and older binaries fail closed (they read the sentinel as an already-expired token).
- `hostlens token rotate` on a never-expiring token imposes a finite overlap deadline on the old token (forced retirement), and the replacement may itself be never-expiring.
- `token create`, `token list`, and `token rotate` outputs render `expires` as `"never"` for zero-expiry records (CLI surface change, documented).
- Help text, SPEC, OPERATIONS, and INSTALL updated in the same change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `token-authorization` — the opaque-credentials requirement now allows explicitly non-expiring tokens at administrator discretion, with rotation and revocation semantics preserved.

## Impact

- `internal/token/token_linux.go`, `internal/token/token.go` (no schema change), `internal/cli/cli_linux.go`, tests, docs.
- Security note: the expiry is a credential-hygiene control; role, read-only, and policy enforcement are unchanged. Revocation remains the instant kill for any token.