## MODIFIED Requirements

### Requirement: Opaque bearer credentials

Every MCP request SHALL require a valid bearer token. Tokens SHALL be randomly generated opaque secrets with separate public IDs, client names, roles, status, and either a required future expiration or an administrator-selected non-expiring sentinel. The server SHALL store only token hashes and metadata, separately from ordinary configuration. Token creation SHALL reveal the secret once. A non-expiring token SHALL pass verification regardless of its creation date until it is revoked, loses its roles, or is rotated with a finite overlap deadline. Rotation of a non-expiring token SHALL impose a finite overlap deadline on the old token while the replacement MAY itself be non-expiring. Revocation SHALL immediately disable a non-expiring token like any other. Verification SHALL re-check status, expiration, and roles on every request; older binaries that do not know the sentinel SHALL treat its stored timestamp as expired and fail closed. Administrative listing SHALL render non-expiring tokens with a `never` expiry display instead of the raw sentinel timestamp.

#### Scenario: Creation

- **WHEN** an authorized local administrator creates a named token with a future expiry or with the non-expiring sentinel
- **THEN** a bearer secret is displayed once and only its hash and metadata persist

#### Scenario: Missing or expired credential

- **WHEN** an MCP request lacks a valid unexpired token
- **THEN** authentication fails before any diagnostic operation

#### Scenario: Non-expiring credential

- **WHEN** a client authenticates with a non-expiring token whose status is active
- **THEN** verification ignores only the date check, and every other check (revocation, role assignment) applies on every call

#### Scenario: Rotation imposes retirement

- **WHEN** an administrator rotates a non-expiring token with an overlap deadline
- **THEN** the old token expires at the overlap deadline and the replacement works until its own expiry or the non-expiring sentinel

#### Scenario: Revocation still kills

- **WHEN** an active non-expiring token is revoked
- **THEN** subsequent requests, including existing sessions, are rejected

#### Scenario: Older binaries fail closed

- **WHEN** a release that requires expirations reads a store containing a non-expiring sentinel
- **THEN** it treats the token as already expired and rejects it instead of granting indefinite validity