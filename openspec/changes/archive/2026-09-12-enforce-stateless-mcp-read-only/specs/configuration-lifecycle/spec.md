## ADDED Requirements

### Requirement: Global MCP read-only configuration

The effective configuration SHALL include the boolean `mcp.read_only` setting and SHALL treat an omitted value as `true`. The setting SHALL apply only to MCP tool availability and execution. Setting it to `false` SHALL make remediation tools eligible for later authorization checks but SHALL NOT create, enable, or authorize remediation. V1 SHALL continue to reject `remediation.enabled: true` and expose diagnostic tools only. The effective-policy fingerprint and administrative status SHALL include the active read-only value without exposing secrets.

#### Scenario: Setting omitted

- **WHEN** an otherwise valid configuration omits `mcp.read_only`
- **THEN** startup activates MCP read-only mode

#### Scenario: Read-only disabled in v1

- **WHEN** an otherwise valid v1 configuration sets `mcp.read_only: false`
- **THEN** startup succeeds but exposes no remediation tool because remediation remains unavailable

#### Scenario: Explicit remediation enablement

- **WHEN** a configuration sets `mcp.read_only: false` and `remediation.enabled: true` in v1
- **THEN** validation rejects the unavailable remediation setting

#### Scenario: Administrative status

- **WHEN** a local administrator reads active status
- **THEN** the response reports the active MCP read-only value and a fingerprint calculated from that value

### Requirement: Atomic activation of MCP read-only mode

`mcp.read_only` SHALL be reloadable through the coordinated configuration generation. A transition to `true` SHALL stop admission of new remediation operations before the new generation becomes active and SHALL NOT report success while an admitted remediation operation can still mutate the host. If admitted remediation work cannot finish or be cancelled within the reload bound, activation SHALL fail and restore the previous generation's admission behavior. Diagnostic reads admitted under the previous generation MAY finish without delaying activation.

#### Scenario: Enable read-only mode

- **WHEN** a valid reload changes `mcp.read_only` from `false` to `true` and admitted remediation work has drained
- **THEN** the new generation activates with no remediation operation admitted or running

#### Scenario: Remediation does not drain

- **WHEN** a transition to read-only mode cannot stop admitted remediation work within the reload bound
- **THEN** reload fails, reports the reason, and restores the previous active generation and admission behavior

#### Scenario: Concurrent diagnostic read

- **WHEN** a diagnostic read is running during a valid transition to read-only mode
- **THEN** that read can finish under its original generation while new MCP requests use the read-only generation
