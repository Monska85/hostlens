# Proposal

## Why

Observer reconciliation can claim an unrecorded binary, unit, or socket as HostLens-owned. It must reject such resources before granting the observer Docker access. Runtime cleanup can also avoid repeated whole-file reads and list-policy work without changing the published MCP contract.

## What Changes

- Reject unrecorded or unexpectedly replaced observer resources during both plan and apply, before any lifecycle mutation.
- Reject a backend IPC path that collides with the local administrative IPC path, including when Docker diagnostics are disabled.
- Stream hash-only lifecycle checks and evaluate Docker inventory collection grants once per response while preserving per-item denials.
- Preserve the existing Linux platforms, diagnostic results, wire schemas, roles, and future remediation boundary.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `installation-lifecycle`: Observer reconciliation must refuse unowned files and sockets before mutation.
- `configuration-lifecycle`: Local administrative and diagnostic IPC paths must be distinct in every configuration.

## Impact

Linux lifecycle, configuration validation, Docker policy evaluation, regression tests, operator guidance, and validation evidence. No new dependency or MCP tool is planned.
