## MODIFIED Requirements

### Requirement: Independent local components

The gateway and diagnostic backend SHALL run as separate identities and independently supervised processes. The gateway SHALL have no additional host-read capability. Backend IPC SHALL be local, access-controlled, structured, and restricted to supported diagnostic operations. Each IPC diagnostic request SHALL carry the operation name and that operation's arguments in the shape advertised for the corresponding MCP tool; the backend SHALL decode arguments against the operation's typed contract and SHALL reject a request whose arguments contain unknown members or fail to decode, returning an `invalid_arguments` failure without admitting collection work. The backend SHALL NOT rely on the gateway having validated arguments.

#### Scenario: Unauthorized local caller

- **WHEN** an unrelated local user attempts a backend connection
- **THEN** access is rejected

#### Scenario: Backend unavailable

- **WHEN** the backend cannot be reached
- **THEN** the gateway reports diagnostic unavailability rather than healthy results

#### Scenario: Malformed IPC arguments

- **WHEN** an authenticated IPC request names a supported operation but supplies an argument member that operation does not define
- **THEN** the backend returns an `invalid_arguments` failure, admits no collection worker, and records no diagnostic evidence
