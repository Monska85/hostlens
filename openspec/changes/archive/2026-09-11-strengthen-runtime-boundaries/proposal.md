# Runtime boundary and consistency audit

## Why

A second repository audit targets security, idiomatic implementation, performance, and consistency after the first hardening pass. Existing pagination, synchronization, and identity-adoption paths still need behavior-level checks beyond compilation and happy-path acceptance.

## What Changes

- Preserve observed service failures when health coverage is partial or exceeds a public page.
- Keep native capability work outside shared state locks and bound its resource use.
- Validate adopted service groups as carefully as service users.
- Fix additional verified runtime, tooling, and documentation findings within five review/improvement rounds; record refuted findings and validation evidence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `health-assessment`: preserve observed service severity independently of pagination and incomplete coverage.
- `platform-runtime`: bound capability discovery without blocking unrelated admission or configuration state.
- `configuration-lifecycle`: bound concurrent backend preparation and reject abandoned candidates.
- `installation-lifecycle`: reject unsafe service-group adoption before resource mutation.
- `mcp-diagnostics`: advertise and enforce only arguments applicable to each tool.
- `release-validation`: keep harness-owned OpenSpec and document formatting outside repository dependencies.

## Impact

Affects Linux collectors, shared backend coordination, lifecycle identity checks, their regression tests, and affected operational guidance. Linux remains the implemented runtime; macOS, Windows, and remediation remain future work. Development dependency changes require evidence and remain separate from runtime dependencies.
