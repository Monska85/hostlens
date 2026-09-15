# Design: never-expiring-tokens

## Context

`Record.Expires` is a `time.Time` (`internal/token/token.go:17`); `Create` requires a future expiry, `Verify` checks `time.Now().Before(r.Expires)`, `List` auto-expires past-due rows, and `Rotate` imposes the overlap on the old token. See proposal.md — Why.

## Goals / Non-Goals

- Goals: first-class non-expiring credentials with explicit rotation/revocation semantics; zero token-store schema change.
- Non-Goals: no change to role checks, read-only enforcement, or the rotation protocol itself; no per-role restrictions on non-expiring creation (administrator action).

## Decisions

- **Zero timestamp as the sentinel** over a new JSON field: adding a field makes strict old readers reject the whole store (availability, not fail-closed in the useful sense - every token becomes unreadable), while the zero sentinel is simply "already expired" to binaries predating this change. The store schema, manifests, and upgrade tooling stay untouched.
- **`Verify` checks `r.Expires.IsZero() || time.Now().Before(r.Expires)`**: bypasses only the date comparison; revocation, status, and role checks are untouched and remain per-call.
- **Rotation of a non-expiring token shortens the old token to the overlap deadline**: treating zero as an infinite expiry makes `overlap.After(r.Expires)` always true, so rotation must explicitly allow the check. The forced retirement date preserves the rotation safety net instead of leaving two indefinite credentials.
- **The replacement may itself be non-expiring**: rotation becomes the retirement tool (old gets a deadline) rather than a prohibition.
- **Friendly display (`expires: "never"`)** at the CLI output boundary only: a display transform in the CLI converts zero expiry in `create`, `list`, and `rotate` outputs. The store file and the token-record JSON shape stay untouched.
- **`parseTime` sentinel**: the literal `never` maps to the zero time in the CLI only; the token package keeps time.Time semantics so future programmatic callers cannot accidentally pass unparseable dates.

## Risks / Trade-offs

- [Indefinite credential weakens credential hygiene] → Administrator opt-in, documented; revocation remains the instant kill and rotation forces a retirement date; the spec records the tradeoff.
- [Older binaries read the sentinel as expired] → Intended fail-closed direction; recorded as a spec scenario.
- [CLI output shape changes (expires becomes a string)] → Documented in the changelog; `expires` was already display-only for humans in the JSON output.

## Migration Plan

No store migration. Upgrade path: normal binary upgrade; existing tokens (all finite) behave identically. Rollback: plain revert; a store containing sentinels then fails closed on old binaries.

## Open Questions

None.