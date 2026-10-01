# Proposal

## Why

The public diagnostic path spans HTTP, local IPC, privileged collection, and installation. A full code review should remove repeated boundary code and reject ambiguous control replies while preserving the published diagnostic contract.

## What Changes

- Reject incomplete or false backend acknowledgements before a configuration generation becomes active.
- Consolidate bounded, single-document JSON decoding across local transport boundaries.
- Release inherited observer descriptors after adoption and remove verified redundant work in request paths.
- Add failure-path tests and record the validated limits of the improvement cycle.
- Fail closed on truncated Docker container inventories when resolving selectors or deriving resource usage.
- Recheck current container names against policy after detail, stats, and logs observations.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `configuration-lifecycle`: require positive backend acknowledgement for coordinated activation.
- `docker-diagnostics`: keep raw daemon and observer error bodies behind the observer boundary and require complete container references for derived conclusions.

## Impact

Gateway, backend, observer, Docker IPC client, local CLI, tests, specifications, changelog, and validation evidence. The MCP tool schema and supported platforms remain unchanged.
