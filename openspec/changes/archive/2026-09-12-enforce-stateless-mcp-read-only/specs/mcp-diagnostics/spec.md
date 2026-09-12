## ADDED Requirements

### Requirement: Fail-closed MCP tool effect classification

Every MCP tool SHALL have exactly one centrally defined effect classification: `diagnostic` or `remediation`. Tool discovery and direct dispatch SHALL use the same classification. An unclassified tool SHALL be unavailable and SHALL NOT reach a collector, backend, external integration, or mutation authority. In MCP read-only mode, only diagnostic tools SHALL be discoverable or executable. When read-only mode is disabled, role authorization, policy, capability availability, and remediation enablement SHALL still apply.

#### Scenario: Read-only discovery

- **WHEN** an authorized client lists tools while `mcp.read_only` is `true`
- **THEN** the response contains only available diagnostic tools permitted by its roles and policy

#### Scenario: Remembered remediation call

- **WHEN** a client directly calls a known remediation tool while `mcp.read_only` is `true`
- **THEN** the gateway rejects the call before contacting any backend or external integration

#### Scenario: Unclassified tool

- **WHEN** a tool is registered without an effect classification
- **THEN** startup or registration fails and the tool cannot be discovered or dispatched

#### Scenario: Read-only disabled

- **WHEN** `mcp.read_only` is `false`
- **THEN** a remediation tool remains unavailable unless every separate role, policy, capability, and remediation enablement check permits it

### Requirement: Stateless diagnostic evidence

Each diagnostic request SHALL fetch or calculate evidence from live permitted sources for that request. HostLens SHALL NOT retain diagnostic observations, inspected payloads, metric histories, host inventories, container inventories, cleanup candidates, or remediation plans for reuse after collection work completes. Request-scoped sampling and pagination state MAY remain in memory only while the admitted collection worker is active and SHALL be released when that worker exits. A client timeout or cancellation SHALL cancel the worker and make any still-active, bounded worker visible through operational admission metrics; an uninterruptible OS read MAY retain transient in-flight memory until it returns, but that evidence SHALL NOT be exposed to another request or copied into control-plane state. HostLens SHALL report source-retention gaps instead of substituting a retained HostLens copy.

#### Scenario: Repeated observation

- **WHEN** a source changes between two diagnostic requests
- **THEN** the second result reflects a new source observation rather than a cached result from the first request

#### Scenario: Request completes

- **WHEN** a diagnostic request returns or fails and its admitted collection worker exits
- **THEN** HostLens retains no inspected content, derived inventory, cleanup candidate, or diagnostic result for a later request

#### Scenario: Historical evidence unavailable

- **WHEN** the live source no longer retains evidence for a requested interval
- **THEN** HostLens reports the coverage gap and does not answer from a private history

#### Scenario: Cancelled sampling

- **WHEN** a diagnostic request is cancelled during sampling
- **THEN** cancellation is delivered within its bound, unfinished work remains admission-bounded and visible until it exits, and its request-scoped observations are not retained for reuse
