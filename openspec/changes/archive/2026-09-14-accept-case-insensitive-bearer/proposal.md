# Proposal: accept-case-insensitive-bearer

## Why

The gateway MCP transport and the metrics endpoint match the Authorization header prefix `"Bearer "` case-sensitively (`internal/gateway/transport.go`, `internal/gateway/metrics.go`). RFC 6750 defines the auth-scheme as case-insensitive token68 grammar, so clients sending `bearer` or `BEARER` — valid per the RFC — are rejected with 401. Fail-closed behavior is preserved; the fix widens acceptance to RFC-compliant spellings only.

## What Changes

- Parse with `strings.Cut` on the first space plus `strings.EqualFold(scheme, "bearer")` at both sites.
- Keep single-header requirement, empty-token handling (empty secret still fails verification with 401), and all status codes identical.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `network-transport` — bearer authentication requirements now state scheme matching is case-insensitive per RFC 6750 while the single-header and verification rules stay unchanged.

## Impact

- `internal/gateway/transport.go`, `internal/gateway/metrics.go`, gateway tests.
- No error-text or status changes; previously valid requests behave identically.