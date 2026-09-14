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

V1 SHALL provide health, inspect, diagnostics, and metrics roles. health SHALL allow get_os_info, get_inventory, and get_health_snapshot. inspect alone SHALL include all health permissions plus list_services, get_service_status, and list_packages. diagnostics alone SHALL include all inspect permissions plus query_logs and read_config. Roles SHALL combine permissions and SHALL NOT imply future remediation access.

The metrics role SHALL grant only scraping of the metrics endpoint. It SHALL NOT grant MCP tool execution, host inspection, administration, or remediation. No other role SHALL implicitly grant metrics access. A combined role set SHALL grant the union of its explicitly assigned permissions. Existing token creation, role update, revocation, expiration, and rotation behavior SHALL apply to metrics tokens.

#### Scenario: Health scope

- **WHEN** a health token calls query_logs
- **THEN** authorization rejects it

#### Scenario: Diagnostics scope

- **WHEN** a diagnostics token reads a globally denied path
- **THEN** file policy denies the request despite tool permission

#### Scenario: Metrics isolation

- **WHEN** a token has only the metrics role
- **THEN** it can scrape an enabled authenticated metrics endpoint but cannot execute any MCP tool or administrative operation

#### Scenario: No implicit metrics inheritance

- **WHEN** a valid token has health, inspect, or diagnostics roles without metrics
- **THEN** the authenticated metrics endpoint rejects it with HTTP 403

#### Scenario: Combined roles

- **WHEN** a token has metrics and health roles
- **THEN** it can scrape metrics and execute health tools but cannot execute inspect or diagnostics tools

#### Scenario: Metrics authority removed

- **WHEN** a token is revoked, expires, loses its metrics role, or passes its rotation overlap deadline
- **THEN** the next authenticated scrape checks the current token state and rejects the request as applicable

#### Scenario: Extend an existing token

- **WHEN** an administrator updates an existing token role set to include metrics alongside its existing roles
- **THEN** its bearer secret remains unchanged and subsequent requests can scrape metrics while retaining the explicitly preserved diagnostic permissions

### Requirement: Current authorization and shared data policy

Status, expiration, and current role assignments SHALL be checked for every request without requiring restart after token changes. One effective file and journal policy SHALL apply per host in v1. Session state SHALL NOT preserve revoked authority or mix identities.

#### Scenario: Existing session

- **WHEN** an authenticated session makes another call after its token is revoked
- **THEN** the call is rejected

#### Scenario: Role removal

- **WHEN** a client caches diagnostics tools before its role becomes health
- **THEN** calling the remembered tool is denied

### Requirement: Protected token store location

The token store directory SHALL be verified on every read and every administrative change: it SHALL be a real directory (symlinks rejected), SHALL be owned by root or the effective user, and SHALL NOT grant write access to group or others. A store whose directory fails these preconditions SHALL be denied as an authentication failure, not accepted as an empty store. Existing token file checks (regular file opened with `O_NOFOLLOW`, owner root or the effective user, no group write and no other access, bounded size) SHALL remain unchanged.

#### Scenario: Swappable directory

- **WHEN** a local attacker with write access to the token store directory replaces the store file with a crafted credential set
- **THEN** the write access itself makes the directory fail the precondition and verification rejects the store before any role is honored

#### Scenario: Group-writable directory

- **WHEN** the token store directory grants group write access
- **THEN** bearer verification and administrative listing fail closed with a precondition error instead of reading its contents

#### Scenario: Directory owned by an untrusted user

- **WHEN** the token store directory is owned by neither root nor the effective user
- **THEN** reading the store is refused with the same precondition denial as permission failures

#### Scenario: Existing valid deployment unchanged

- **WHEN** the token store directory is owned by root or the effective user with no group or other write access
- **THEN** verification, administration, and rotation behave exactly as before
