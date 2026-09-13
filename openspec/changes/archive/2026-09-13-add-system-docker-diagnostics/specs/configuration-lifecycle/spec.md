## ADDED Requirements

### Requirement: Validated Docker observer configuration

Configuration SHALL provide an opt-in Docker diagnostics section that defaults to disabled and selects only an absolute local Unix socket for a system-wide engine. It SHALL identify the expected existing socket-owning access group, defaulting to the standard `docker` group, so reconciliation can prepare an enabled-waiting installation before Docker exists. Enabling Docker SHALL require an observer IPC path distinct from existing gateway administration and diagnostic IPC. Startup SHALL validate the group name, path type, ownership and write boundaries, unsupported transport schemes, IPC collisions, and settings compatible with the selected installation mode before accepting MCP traffic.

#### Scenario: Docker omitted

- **WHEN** an existing configuration has no Docker section
- **THEN** HostLens starts without Docker authority or Docker tools

#### Scenario: Remote endpoint

- **WHEN** Docker configuration selects TCP, SSH, TLS, or another remote endpoint
- **THEN** validation rejects the unsupported transport

#### Scenario: IPC collision

- **WHEN** the Docker observer path equals the administration or diagnostic socket path
- **THEN** validation rejects the configuration before any component starts

#### Scenario: Nonstandard Docker group

- **WHEN** the administrator configures a valid existing group for a Docker socket that is not owned by the standard `docker` group
- **THEN** reconciliation uses that group only for the observer service's process-scoped `SupplementaryGroups=` setting and records it in the plan

### Requirement: Restart-only Docker topology

Enabling or disabling the Docker observer and changing its daemon or IPC socket SHALL require a coordinated service restart because these settings change process authority and topology. Reload SHALL reject such changes while preserving the active generation. Docker policy and shared request ceilings SHALL remain reloadable for newly admitted requests.

#### Scenario: Observer enabled on reload

- **WHEN** SIGHUP encounters a change from disabled to enabled Docker diagnostics
- **THEN** reload rejects the candidate with restart guidance and preserves the current surface

#### Scenario: Docker policy reload

- **WHEN** a valid reload removes access to a Docker resource without changing observer topology
- **THEN** new requests enforce the new denial while an admitted read may finish under its original generation

### Requirement: Docker desired state and reconciliation

Docker enablement SHALL represent administrator intent separately from current daemon availability. `hostlens reconcile --system` SHALL calculate and display a complete plan without mutation. `hostlens reconcile --system --apply` SHALL transactionally move the optional observer topology between disabled, waiting, provisioned, available, and unavailable states using existing lifecycle ownership, conflict, rollback, and interruption protections. Reconciliation SHALL NOT install, configure, start, stop, or restart Docker, and SHALL NOT grant Docker access to the gateway or diagnostic backend.

#### Scenario: Docker installed after HostLens

- **WHEN** Docker diagnostics are enabled and reconciled while the supported daemon is absent and an administrator later installs and starts Docker with the configured socket and group
- **THEN** the next Docker diagnostic request activates the observer and makes Docker diagnostics available without reinstalling or reconciling HostLens again

#### Scenario: Docker remains absent

- **WHEN** reconciliation finds no supported local daemon or cannot establish the required isolated access
- **THEN** Docker remains in an explicit waiting or unavailable state and non-Docker HostLens services remain operational

#### Scenario: Docker disappears

- **WHEN** a provisioned Docker daemon stops or its socket disappears
- **THEN** the observer stops, Docker diagnostics become unavailable without clearing configured intent, and HostLens does not restart Docker

#### Scenario: Nonstandard Docker access

- **WHEN** the daemon uses a socket or owning group that differs from the reconciled configuration
- **THEN** the observer refuses access with actionable status and the administrator must update configuration and rerun reconciliation

#### Scenario: Docker integration disabled

- **WHEN** an administrator applies a reconciliation plan that disables Docker diagnostics
- **THEN** HostLens stops and disables the observer service and socket unit, exposes no Docker tools, and leaves the dormant account without Docker group authority
