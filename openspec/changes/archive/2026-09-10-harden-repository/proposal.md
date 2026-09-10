# Repository hardening

## Why

The initial implementation has security edge cases, delayed resource limits, stale build inputs, and contradictory documentation. Resolve verified findings before public release while preserving the diagnostic-only v1 contract.

## What Changes

- Enforce secret protection for hard-link aliases and bound gateway admission before expensive request processing.
- Apply input and traversal limits before allocation; remove repeated immutable-policy work and unused code.
- Correct collector, token, lifecycle, and packaging failures established by the review panel.
- Separate portable authorization from Linux storage without adding future platform implementations.
- Verify the exact candidate exercised by the matrix; enforce specifications and documentation checks in CI.
- Replace repetitive or outdated guidance with concise current instructions and traceable validation evidence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `access-policy`: reject hard-linked diagnostic sources, including aliases hidden by secret masks.
- `network-transport`: bound admission before authentication and MCP construction.
- `configuration-lifecycle`: bound configuration reads and recursive explanation before allocation.
- `release-validation`: exclude stale staged files and bind matrix execution to verified artifacts; check documentation and specs automatically.

## Impact

Runtime, CLI, test harness, archives, CI, and documentation. Existing safety contracts remain authoritative. macOS, Windows, remediation, and release publication remain outside implementation scope. Review and improvements are limited to three local rounds, followed by at most three hosted-CI correction rounds.
