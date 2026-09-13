## ADDED Requirements

### Requirement: Docker MCP integration

Available Docker tools SHALL participate in the same MCP effect registry, role checks, policy checks, capability discovery, argument validation, deadlines, concurrency limits, response ceilings, failure status, and payload-free audit path as other diagnostic tools. All Docker tools SHALL have the diagnostic effect. A failure or stall in Docker discovery or collection SHALL remain isolated from non-Docker tool discovery and execution.

#### Scenario: Read-only mode

- **WHEN** Docker diagnostics are enabled and `mcp.read_only` is `true`
- **THEN** authorized Docker diagnostic tools remain available because every exposed operation is read-only

#### Scenario: Docker observer stalls

- **WHEN** Docker capability discovery exceeds its deadline
- **THEN** Docker tools report unavailable without blocking non-Docker discovery, configuration activation, or unrelated requests

#### Scenario: Oversized Docker response

- **WHEN** a Docker inventory exceeds the configured response or pagination ceiling
- **THEN** HostLens returns a bounded page or explicit truncation without silently dropping policy or coverage information
