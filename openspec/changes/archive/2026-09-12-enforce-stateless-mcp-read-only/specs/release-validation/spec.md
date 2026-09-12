## ADDED Requirements

### Requirement: MCP safety invariant validation

Release validation SHALL prove that all registered MCP tools have exactly one effect classification and that discovery and direct dispatch enforce the same active configuration, role, policy, capability, and remediation gates. Validation SHALL exercise both values of `mcp.read_only`, omitted-setting default behavior, direct remembered calls, unclassified registration, coordinated reload, and backend non-invocation on rejection. The v1 test surface SHALL continue to contain no remediation implementation.

#### Scenario: Exhaustive registry check

- **WHEN** the maintained MCP tool registry is validated
- **THEN** every tool has one recognized effect and no discovered or dispatched tool exists outside that registry

#### Scenario: Direct-call bypass attempt

- **WHEN** a test directly calls a remediation or unclassified tool while MCP read-only mode is active
- **THEN** validation observes rejection before any backend or external integration invocation

#### Scenario: Default configuration

- **WHEN** acceptance runs with a configuration that omits `mcp.read_only`
- **THEN** validation observes the same diagnostic-only MCP surface as explicit `mcp.read_only: true`

#### Scenario: Local administrator regression

- **WHEN** acceptance runs local administrative lifecycle and token operations with MCP read-only mode active
- **THEN** those operations retain their existing authorization and behavior without becoming MCP tools

### Requirement: Stateless diagnostic acceptance

Release validation SHALL exercise every diagnostic tool against mutable fixtures and verify a new source observation on each request. Tests SHALL inspect HostLens-managed files, component state, operational logs, and service metrics after success, failure, timeout, and cancellation for retained diagnostic content, inventories, cleanup candidates, or plans. Timeout and cancellation checks SHALL wait for the admitted collector worker to acknowledge cancellation; an intentionally stalled worker SHALL remain bounded and visible as active rather than being reported as completed. Unique sensitive markers SHALL remain absent from logs and metrics. Container limitations SHALL be recorded without converting an unverified persistence or mutation boundary into a pass.

#### Scenario: Mutable source fixture

- **WHEN** a permitted source changes after one request and before the next
- **THEN** the next diagnostic result reflects the new live value and no HostLens cache supplies the old value

#### Scenario: Sensitive evidence marker

- **WHEN** a diagnostic source and result contain a unique test marker
- **THEN** post-request inspection finds the marker only in the source and client response, not in HostLens-managed files, audit logs, service metrics, or retained component state

#### Scenario: Cancellation and failure

- **WHEN** collection fails or is cancelled after observing partial evidence
- **THEN** validation observes bounded active work until the collector exits and then finds no reusable partial result or inspected payload

#### Scenario: Observer mutation attempt

- **WHEN** a diagnostic integration attempts an operation outside its read contract
- **THEN** validation observes rejection and no target-state change
