## ADDED Requirements

### Requirement: MCP and local administration authority separation

The MCP read-only setting SHALL govern only MCP tool discovery and execution. Authenticated local administrator operations, including installation, upgrade, uninstall, token administration, configuration validation, reload, status, and telemetry, SHALL remain available under their existing operating-system and local IPC controls regardless of `mcp.read_only`. Local administrator operations SHALL NOT be registered as MCP tools.

#### Scenario: Local operation in MCP read-only mode

- **WHEN** `mcp.read_only` is `true` and an authorized local administrator reloads configuration or manages a token
- **THEN** the operation remains available through the local administrative boundary

#### Scenario: Remote administrative name

- **WHEN** an MCP client calls a name used by a local administrative operation
- **THEN** the gateway rejects it because the operation is not part of the MCP tool surface

### Requirement: Separated mutation authority

The MCP gateway and diagnostic backend SHALL hold no mutation credential, generic command channel, or unrestricted external API pass-through. A diagnostic integration with an external interface that combines reads and writes SHALL use a constrained observer boundary that permits only the documented read operations. Any future remediation SHALL run through a separately authorized component, use distinct operation contracts and credentials, and revalidate relevant live state immediately before mutation. It SHALL NOT execute from retained diagnostic evidence or a retained remediation plan.

#### Scenario: Mixed read-write external API

- **WHEN** a diagnostic collector needs evidence from an API that also supports mutation
- **THEN** its runtime authority exposes only the approved observation operations and no generic API forwarding

#### Scenario: Diagnostic component compromise boundary

- **WHEN** the gateway or diagnostic backend attempts a mutation operation
- **THEN** the local IPC or observer boundary rejects the operation because that component has no mutation contract or authority

#### Scenario: Future remediation state changes

- **WHEN** an authorized future remediation operation reaches its execution boundary
- **THEN** it re-fetches and validates the relevant live state before applying the requested change
