# Spec Delta

## ADDED Requirements

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
