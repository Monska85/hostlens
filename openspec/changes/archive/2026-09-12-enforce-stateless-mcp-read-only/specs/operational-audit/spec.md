## ADDED Requirements

### Requirement: Diagnostic retention boundary

Operational logs MAY retain payload-free control-plane metadata already required for audit, including request identifiers, authenticated identity, tool name, active generation, duration, outcome, and bounded counts. Service metrics MAY retain aggregate counters in process memory for the process lifetime. Neither operational logs nor service metrics SHALL contain inspected source content, returned diagnostic data, resource inventories, cleanup candidates, remediation plans, credentials, or secrets. Configuration, credential records, installation ownership records, and active request coordination SHALL be treated as control-plane state and SHALL NOT be used as a diagnostic evidence store.

#### Scenario: Successful diagnostic call

- **WHEN** successful-call auditing is enabled and a diagnostic tool returns inspected data
- **THEN** the audit record contains permitted request and outcome metadata without the inspected data or derived inventory

#### Scenario: Service metrics update

- **WHEN** a diagnostic call updates aggregate request counters
- **THEN** the counters reveal no source content, resource identity, diagnostic result, cleanup candidate, or remediation plan

#### Scenario: Later diagnostic request

- **WHEN** a later request asks for evidence observed by an earlier request
- **THEN** HostLens performs a live observation and does not reconstruct the answer from audit records, service metrics, or control-plane state
