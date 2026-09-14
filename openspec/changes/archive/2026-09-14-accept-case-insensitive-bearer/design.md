# Design: accept-case-insensitive-bearer

## Context

Both parse sites use `strings.HasPrefix(auth[0], "Bearer ")` + `strings.TrimPrefix`. RFC 6750 (§2.1) treats the auth-scheme as case-insensitive. See proposal.md — Why.

## Goals / Non-Goals

- Goals: RFC-compliant scheme matching at both sites; identical behavior for every already-accepted request.
- Non-Goals: no token68 (no `Bearer<token>` without space), no multiple-header leniency, no error-text changes.

## Decisions

- **`strings.Cut` + `EqualFold`** instead of lowercasing the whole header: preserves the exact credential bytes (secrets are case-sensitive) and keeps the space-delimited form strict.
- **Both sites share the same parsing shape** without introducing a helper: two one-line sites; a shared helper would add indirection without removing duplication of logic beyond the two lines.

## Risks / Trade-offs

- [Clients relying on rejection of lowercase `bearer` (none plausible)] → Widening acceptance only affects previously-rejected RFC-compliant spellings.

## Migration Plan

None. Rollback is a plain revert.

## Open Questions

None.