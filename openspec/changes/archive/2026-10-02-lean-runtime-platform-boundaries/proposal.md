# Proposal

## Why

The previous hardening pass increased production Go by 155 lines while claiming simplification. HostLens needs a measured, whole-codebase reduction that preserves its public diagnostic contract and security boundaries.

## What Changes

- Replace repeated runtime orchestration, collection, and lifecycle patterns with smaller typed paths where their behavior is genuinely shared.
- Reduce production Go lines and measured request cost without weakening policy, budgets, privilege separation, or failure reporting.
- Keep Linux amd64 and arm64 behavior compatible and portable packages buildable for future macOS and Windows implementations.
- Preserve the separate authority required for future remediation without adding a mutation operation or credential to diagnostic processes.
- Update affected documentation, contributor instructions, changelog, and validation evidence.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

Docker diagnostics: make the existing dangling-image fact consistent in the image inventory and disk-usage analysis. Installation lifecycle and release validation: reject oversized or trailing manifests before acting on them. Other refactoring preserves current behavior.

## Impact

Gateway, backend, collectors, Docker observer, policy, configuration, CLI, lifecycle, tests, validation tooling, and documentation. The published MCP tool schemas and supported runtime scope remain fixed.
