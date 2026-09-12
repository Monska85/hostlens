# Operational audit

## Purpose

Define the observable HostLens v1 operational audit behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Structured operational records

HostLens SHALL write structured operational logs to stdout or stderr for service-manager capture. Records SHALL cover authentication failures, denied operations, reloads, backend failures, and tool calls with identity when known, tool name, duration, outcome, and correlated request identifiers. Verbosity and successful-call auditing SHALL be configurable.

#### Scenario: Correlated call

- **WHEN** a gateway request reaches the backend
- **THEN** records can be linked by the same request identifier

#### Scenario: Failed authentication

- **WHEN** a token cannot be validated
- **THEN** the record includes failure context without inventing a client identity

### Requirement: Sensitive data exclusion

Logs SHALL NOT contain bearer secrets, token hashes, private keys, returned configuration contents, or inspected log bodies. Untrusted values SHALL be safely encoded. Ordinary operational logs SHALL NOT become a diagnostic data retention store.

#### Scenario: Sensitive payload

- **WHEN** a tool reads a configuration containing a password
- **THEN** audit records contain outcome metadata without the configuration body

#### Scenario: Malicious log input

- **WHEN** a request contains newline characters in a user-controlled value
- **THEN** structured output remains a valid record without forged entries

### Requirement: Retention ownership

V1 SHALL delegate operational log retention to the host service manager and SHALL NOT require a separate log rotation system. Uninstall SHALL disclose that shared journal records remain until normal retention and SHALL NOT delete unrelated system logs.

#### Scenario: Uninstall history

- **WHEN** HostLens is removed from a systemd machine
- **THEN** its owned files are removed but the shared journal is preserved

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
