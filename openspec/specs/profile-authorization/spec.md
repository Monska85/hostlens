# profile-authorization Specification

## Purpose

Let administrators grant exactly which diagnostic and repair tools a token holder may use against particular resources, with global denial precedence and a simple auditable configuration.

## Requirements

### Requirement: Activated profiles

HostLens SHALL load only explicitly activated local profiles. The active set is installation-wide: every token with the same role receives the same profile grants, subject to the global denials. Profiles SHALL list typed tool and resource grants and denials; denials SHALL override grants regardless of profile order. Unknown tools, invalid selectors, missing profiles, and cycles SHALL fail configuration validation.

#### Scenario: Unactivated broad profile

- **WHEN** a permissive profile exists but is not activated
- **THEN** it grants no access

#### Scenario: Conflicting profiles

- **WHEN** one active profile grants and another denies the same tool and resource
- **THEN** HostLens denies the operation

#### Scenario: Two tokens with the same role

- **WHEN** two valid tokens have the observe role and one active profile grants an observation
- **THEN** both tokens receive that grant; profiles do not isolate individual clients

### Requirement: Independent roles and effect gate

Every MCP tool SHALL have one typed input, one typed output, one effect, and a required role. Discovery and direct calls SHALL use the same fail-closed role, profile, capability, and effect decision. Read-only mode SHALL prevent repair tools from being discovered or run. Disabling read-only mode alone SHALL grant no repair authority.

#### Scenario: Cached repair tool

- **WHEN** a client calls a previously discovered repair tool while read-only mode is active
- **THEN** HostLens rejects it before contacting a repair process

#### Scenario: Missing repair grant

- **WHEN** read-only mode is disabled but the token lacks the repair role or target grant
- **THEN** discovery omits the tool and direct calls are denied

### Requirement: Managed credentials

HostLens SHALL authenticate each MCP request with a revocable, independently managed bearer credential. Credential material SHALL not enter diagnostic responses, profiles, or audit records. Invalid, expired, or revoked credentials SHALL fail before host collection.

#### Scenario: Revoked credential

- **WHEN** a revoked token calls a diagnostic or repair tool
- **THEN** HostLens rejects the call without accessing host or Docker state

### Requirement: Explicit token lifetime

`token create` and `token rotate` SHALL require an explicit finite RFC3339 expiry or an explicit `never` choice. Omission SHALL fail before writing credentials. Existing token validity SHALL remain unchanged.

#### Scenario: Omitted expiry

- **WHEN** an administrator creates or rotates a token without specifying `--expires`
- **THEN** the command rejects it without changing the token store

### Requirement: Local policy explanation

`policy explain` SHALL report the effect, tool, resource, matching grants and denials, and final profile decision for an administrator-supplied target using the same compiled policy as MCP calls. The command SHALL state that an MCP call also requires a valid token role, read-only and capability checks. It SHALL not reveal credentials or collect diagnostic evidence.

#### Scenario: Conflicting grants

- **WHEN** one active profile grants a resource and another denies it
- **THEN** the explanation names the denial and reports a denied decision
