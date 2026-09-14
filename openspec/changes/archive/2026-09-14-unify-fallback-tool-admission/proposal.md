# Proposal: unify-fallback-tool-admission

## Why

The backend-unavailable fallback middleware in `internal/gateway/gateway.go` returned the failure envelope for any `tools/call` whose role matched, skipping the `toolAdmitted` effect gate used by per-call admission. With today's all-diagnostic registry this is latent, but a future non-diagnostic tool definition would receive results through the fallback path without the read-only gate.

## What Changes

- The fallback condition now includes `toolAdmitted(definition, known, readOnly)`, sharing the per-call admission predicate.
- A test proves the gate: an admitted tool receives `backend_unavailable`; an unknown tool never receives the failure envelope.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None — no requirement change: fail-closed admission at every layer is already specified; this aligns implementation with the contract. `skip_specs: true`.

## Impact

- `internal/gateway/gateway.go` (fallback middleware predicate), `internal/gateway/gateway_test.go` (new test).