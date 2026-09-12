## Why

HostLens needs explicit architectural guarantees before remediation and container runtime support can expand its MCP surface. Operators must be able to enable one global MCP read-only setting and know that HostLens retains no diagnostic evidence or plans between requests.

## What Changes

- Add `mcp.read_only`, enabled by default, as a global safety ceiling for MCP tool discovery and execution.
- Classify every MCP tool as diagnostic or remediation in one fail-closed registry shared by discovery and dispatch.
- Keep authenticated local administrator operations outside the MCP read-only gate.
- Define stateless diagnostics as live, request-scoped collection with no retained observations, inspected payloads, inventories, histories, cleanup candidates, or remediation plans.
- Preserve configuration, credentials, installation records, bounded request coordination, service metrics, and payload-free operational audit records as control-plane state rather than diagnostic evidence.
- Require future remediation to use separate authority, revalidate live state before mutation, and remain unavailable in v1.
- Record these invariants in `AGENTS.md` so later features preserve them.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `configuration-lifecycle`: Define the default, validation, reload, and atomic activation behavior of `mcp.read_only`.
- `mcp-diagnostics`: Require stateless live collection and effect-based MCP tool filtering and rejection.
- `platform-runtime`: Separate diagnostic observation authority from future remediation authority while excluding local administration from the MCP gate.
- `operational-audit`: Define which payload-free operational records may remain after a request and prohibit diagnostic evidence retention.
- `release-validation`: Require observable, adversarial validation of the stateless and MCP read-only invariants.

## Impact

The change affects YAML configuration, effective configuration fingerprints, coordinated reload, MCP tool registration and dispatch, backend IPC operation contracts, service logging, tests, operator documentation, and agent instructions. It adds no dependency and exposes no remediation tool or backend in v1.
