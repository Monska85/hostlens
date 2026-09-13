## ADDED Requirements

### Requirement: Docker resource policy

Docker diagnostics SHALL require the diagnostics role and explicit active `docker` grants. Policy SHALL distinguish daemon information, container inventory, individual containers, stats, logs, image inventory, individual images, volume inventory, individual volumes, network inventory, individual networks, and disk usage. Denial SHALL win across stable IDs, current names, and list membership. A collection grant SHALL NOT authorize mutation, generic Docker API access, configuration or secret payloads, or another diagnostic source class.

#### Scenario: Role without Docker grant

- **WHEN** a diagnostics token has no active Docker grant
- **THEN** no Docker observation reaches the observer

#### Scenario: Denied container alias

- **WHEN** a container matches an allowed list rule and a deny rule through its stable ID or current name
- **THEN** the container, its logs, stats, and derived counts are excluded

#### Scenario: Inventory without logs

- **WHEN** policy permits container inventory but not container logs
- **THEN** container metadata is available while log discovery and direct log calls are rejected

#### Scenario: Docker grant cannot mutate

- **WHEN** an allow rule covers every Docker diagnostic resource
- **THEN** it still grants no create, update, exec, copy, registry, build, removal, or prune authority

### Requirement: Race-safe Docker identity authorization

Name selectors SHALL resolve to stable Docker identities before resource access. HostLens SHALL evaluate policy against both the requested selector and resolved identity, then verify the identity has not changed before returning resource-scoped evidence. Unresolvable, ambiguous, or changed identity SHALL fail without falling back to a broader list grant.

#### Scenario: Reused container name

- **WHEN** an allowed container name is removed and reused for a different identity during collection
- **THEN** HostLens returns no evidence for the replacement until that stable identity is independently authorized

#### Scenario: Abbreviated identifier collision

- **WHEN** an identifier prefix is ambiguous
- **THEN** HostLens rejects the selector without choosing one resource
