## ADDED Requirements

### Requirement: Safe service-group adoption

Installation SHALL reject service groups with zero, malformed, or colliding numeric group identities, or explicitly listed unrelated members. Checks SHALL run before planning any resource mutation and again before installing service files after identity creation.

#### Scenario: Privileged group alias

- **WHEN** an existing HostLens group resolves to GID zero
- **THEN** installation rejects the identity without installing files or changing permissions

#### Scenario: Unrelated explicit member

- **WHEN** a service group lists an unrelated local account
- **THEN** installation reports the group conflict instead of granting that account access to HostLens state
