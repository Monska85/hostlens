## ADDED Requirements

### Requirement: Explicit Docker observer installation

Release archives SHALL always contain the separate `hostlens-docker-observer` binary, but system installation SHALL provision its identity and units only when an administrator explicitly enables the integration. The plan SHALL disclose that access to the rootful Docker socket is root-equivalent, identify the socket and dedicated non-login service identity, and show every identity, process-scoped supplementary group, unit, socket, and configuration change before mutation. The installer SHALL NOT add the observer identity to a persistent supplementary group. The gateway and diagnostic identities SHALL NOT receive Docker socket access or Docker group membership.

#### Scenario: Docker disabled

- **WHEN** an administrator installs with Docker diagnostics disabled
- **THEN** installation creates no Docker observer identity, service credential, active unit, socket, or daemon access

#### Scenario: Docker enabled

- **WHEN** an administrator accepts a Docker-enabled installation plan
- **THEN** only the dedicated observer receives the declared daemon access and its MCP-facing IPC exposes the fixed observation contract

#### Scenario: Existing identity conflict

- **WHEN** the planned observer identity or IPC resource already exists without matching HostLens ownership
- **THEN** installation refuses the conflict and preserves the existing resource

### Requirement: Docker-preserving lifecycle operations

Install, reconcile, upgrade, rollback, and uninstall SHALL NOT edit Docker daemon configuration, control Docker, change Docker socket ownership or mode, or create, modify, stop, or delete Docker resources. Lifecycle ownership records SHALL cover only HostLens-created observer resources and service configuration. Uninstall SHALL remove confirmed HostLens-owned observer access while preserving pre-existing identities, memberships, units, sockets, Docker data, and unrelated files.

#### Scenario: Uninstall Docker-enabled HostLens

- **WHEN** an owned Docker observer installation is uninstalled
- **THEN** HostLens removes its owned observer service and access records without changing any container, image, volume, network, Docker configuration, or daemon state

#### Scenario: Pre-existing account access

- **WHEN** the observer identity had Docker access before HostLens recorded ownership
- **THEN** cleanup preserves that access and reports it for administrator review rather than editing persistent account membership

#### Scenario: Interrupted provisioning

- **WHEN** installation is interrupted after some observer resources are created
- **THEN** retry or cleanup uses durable ownership evidence and does not remove uncertain or pre-existing Docker access

### Requirement: Version-coherent observer lifecycle

Release archives SHALL ship the observer as a separate binary in the same versioned checksum manifest and artifact set as the gateway and diagnostic backend. Install, upgrade, and rollback SHALL verify the complete artifact set and apply one coherent version. They SHALL restart the observer only when it is active or required by a pending request and SHALL preserve enabled-waiting state when Docker is absent.

#### Scenario: Upgrade before Docker installation

- **WHEN** HostLens is upgraded while Docker diagnostics are enabled but Docker is absent
- **THEN** the observer binary and lifecycle metadata advance with the other HostLens components while the integration remains waiting and Docker is not installed or started

#### Scenario: Mixed observer version

- **WHEN** the candidate observer protocol or binary version does not match the rest of the HostLens artifact set
- **THEN** lifecycle validation rejects activation and preserves the last coherent installation
