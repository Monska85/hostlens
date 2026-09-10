# Operational audit

## Purpose

Define the observable HostLens v1 operational audit behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## ADDED Requirements

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
