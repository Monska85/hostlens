# Token authorization

## Purpose

Define the observable HostLens v1 token authorization behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Opaque bearer credentials

Every MCP request SHALL require a valid bearer token. Tokens SHALL be randomly generated opaque secrets with separate public IDs, client names, roles, required expiration, and status. The server SHALL store only token hashes and metadata, separately from ordinary configuration. Token creation SHALL reveal the secret once.

#### Scenario: Creation

- **WHEN** an authorized local administrator creates a named token
- **THEN** a bearer secret is displayed once and only its hash and metadata persist

#### Scenario: Missing or expired credential

- **WHEN** an MCP request lacks a valid unexpired token
- **THEN** authentication fails before any diagnostic operation

### Requirement: Local token administration

The CLI SHALL support create, list, role update, revoke, and rotation. Listing SHALL show IDs, names, roles, expiry, and status without secrets, defaulting to active tokens with an option for all statuses. Rotation SHALL create a new secret with explicit bounded overlap. MCP bearer possession SHALL NOT authorize administrative operations.

#### Scenario: Role update

- **WHEN** the administrator replaces a token role set
- **THEN** the secret stays unchanged and the next request uses new permissions

#### Scenario: Revocation

- **WHEN** an active token is revoked
- **THEN** subsequent requests including existing-session requests are rejected; metadata remains for audit

#### Scenario: Rotation

- **WHEN** a replacement token is issued with an overlap deadline
- **THEN** both work only within their configured validity and the old token stops at its deadline

### Requirement: Additive fixed roles

V1 SHALL provide health, inspect, and diagnostics roles. health SHALL allow get_os_info, get_inventory, and get_health_snapshot. inspect alone SHALL include all health permissions plus list_services, get_service_status, and list_packages. diagnostics alone SHALL include all inspect permissions plus query_logs and read_config. Roles SHALL combine permissions and SHALL NOT imply future remediation access.

#### Scenario: Health scope

- **WHEN** a health token calls query_logs
- **THEN** authorization rejects it

#### Scenario: Diagnostics scope

- **WHEN** a diagnostics token reads a globally denied path
- **THEN** file policy denies the request despite tool permission

### Requirement: Current authorization and shared data policy

Status, expiration, and current role assignments SHALL be checked for every request without requiring restart after token changes. One effective file and journal policy SHALL apply per host in v1. Session state SHALL NOT preserve revoked authority or mix identities.

#### Scenario: Existing session

- **WHEN** an authenticated session makes another call after its token is revoked
- **THEN** the call is rejected

#### Scenario: Role removal

- **WHEN** a client caches diagnostics tools before its role becomes health
- **THEN** calling the remembered tool is denied
