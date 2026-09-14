# Installation lifecycle

## Purpose

Define the observable HostLens v1 installation lifecycle behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Archive distribution and explicit installation

V1 SHALL distribute amd64 and arm64 executable archives with example YAML, shipped profiles, and installation and upgrade instructions. Native packages SHALL be excluded. A local administrator install operation SHALL generate systemd definitions from the selected configuration, show its plan, record changes, preserve existing configuration, and refuse conflicts. Archives SHALL NOT need pre-rendered copies of those definitions. Services SHALL remain stopped unless explicitly requested.

#### Scenario: Fresh installation

- **WHEN** the administrator accepts the plan without a start option
- **THEN** accounts and services are installed but do not start

#### Scenario: Conflict

- **WHEN** an existing installation cannot be safely adopted
- **THEN** installation stops without overwriting it

#### Scenario: Configured service definitions

- **WHEN** the administrator installs either supported privilege mode
- **THEN** the installer generates the applicable hardened units without reading service templates from the archive

### Requirement: Service identities and modes

The installer SHALL automatically provision separate non-login Linux identities for gateway and diagnostics, without administrator-managed passwords. Standard mode SHALL grant CAP_DAC_READ_SEARCH only to the diagnostic process through systemd, not executable file capabilities. Restricted mode SHALL grant no additional capability and use ordinary OS permissions. Both SHALL retain backend policy enforcement.

#### Scenario: Standard mode

- **WHEN** the administrator selects standard installation
- **THEN** ordinary application file ownership and rotation need no modification, while broad-read risk is disclosed

#### Scenario: Restricted mode

- **WHEN** a log is not readable by the diagnostic identity
- **THEN** a collection issue is returned and no automatic escalation occurs

#### Scenario: Future systems

- **WHEN** platform support is extended
- **THEN** Linux capabilities are not claimed to exist on macOS or Windows

### Requirement: Constrained diagnostic privilege

Both modes SHALL apply compatible service hardening and restricted IPC. Standard mode SHALL isolate configured gateway token state and TLS private keys from backend access, failing startup if required isolation cannot be established, and SHALL hide other mandatory secret sources where feasible and constrain writes, devices, networking, and privilege acquisition. The service SHALL NOT describe filesystem read-only mounts or profiles as complete protection against a compromised backend.

#### Scenario: Credential containment

- **WHEN** the read-capable backend attempts to access the gateway secret store
- **THEN** the read is denied by OS containment, independently of the diagnostic policy implementation

#### Scenario: No general command surface

- **WHEN** an agent requests shell execution
- **THEN** no such diagnostic operation exists

### Requirement: Tracked complete uninstall

An installation manifest SHALL identify created accounts, groups, files, service definitions, links, and managed permission changes. Uninstall SHALL present a removal plan, stop and disable services, remove owned configuration, profiles, token state, runtime state, binaries and accounts, and report failures. It SHALL preserve pre-existing resources and externally supplied directories.

#### Scenario: Complete removal

- **WHEN** a clean managed installation is uninstalled
- **THEN** all installation-owned resources and identities are removed

#### Scenario: External profile directory

- **WHEN** an administrator supplied an existing profile directory
- **THEN** uninstall preserves it and reports that it was external

#### Scenario: Unexpected ownership

- **WHEN** a created account owns unexpected unrelated files
- **THEN** uninstall reports the conflict instead of deleting unrelated data

#### Scenario: Manual ACL

- **WHEN** an administrator changed log permissions outside the installer
- **THEN** uninstall identifies the limitation and gives cleanup guidance without guessing prior permissions

### Requirement: Explicit recoverable upgrades

Upgrade SHALL validate release compatibility before replacement, preserve configuration, profiles, identities and token state, retain previous binaries, record changes, and restart only previously running services. Failed activation SHALL roll back when state changes are reversible. Automatic updates and automatic incompatible migrations SHALL be excluded.

#### Scenario: Activation failure

- **WHEN** new services fail their activation check
- **THEN** the previous release is restored when rollback is safe and the failure is reported

#### Scenario: Policy update

- **WHEN** a bundled profile changes in an upgrade
- **THEN** the change is presented for administrator review before activation and never silently broadens access

#### Scenario: Irreversible change

- **WHEN** new state cannot be safely read by the previous release
- **THEN** upgrade stops before mutation and explains the required migration

### Requirement: Recoverable ownership tracking

Lifecycle operations SHALL durably record intended and completed resource changes so interrupted operations can be inspected and safely resumed or cleaned up. Later-created owned token state and retained upgrade binaries SHALL be covered by ownership tracking. Unexpected files within managed directories SHALL be preserved and reported rather than recursively deleted without classification.

#### Scenario: Interrupted install

- **WHEN** installation stops after creating an account but before completing service setup
- **THEN** a subsequent lifecycle operation identifies the partial installation and offers safe tracked recovery

#### Scenario: Unexpected directory content

- **WHEN** an administrator places an unrelated file in a managed directory
- **THEN** uninstall preserves and reports it while removing identifiable owned resources

### Requirement: Conservative interrupted installation cleanup

The installer SHALL establish identity existence before mutation and SHALL NOT mark unvisited planned identities as owned. Cleanup SHALL skip uncreated services and verify absence for missing created units, while preserving real failures to stop existing services. Uncertain interrupted identity creation SHALL be preserved and reported for administrator inspection.

#### Scenario: Interruption before unit creation

- **WHEN** installation stops before creating service units with a pre-existing service identity
- **THEN** cleanup removes confirmed owned resources, preserves the pre-existing identity, and allows retry after resolving external-file conflicts

#### Scenario: Existing service cannot stop

- **WHEN** a created service cannot be stopped or disabled
- **THEN** cleanup reports the failure and retains its resources

#### Scenario: Pre-existing group with newly created user

- **WHEN** removing an owned service user could implicitly delete a pre-existing same-name private group
- **THEN** cleanup preserves the user and group, reports the conflict, and supports retry after explicit administrator resolution

### Requirement: Project license and attribution in archives

HostLens SHALL use Apache-2.0 for project-owned code and documentation. Each executable archive SHALL include the full project license and a readable attribution notice identifying HostLens, Monska85, and the original repository. Dependency license and attribution notices SHALL retain their applicable terms. Archive checksums SHALL cover these files. Packaging SHALL fail if the project license or notice is missing.

#### Scenario: Redistributable archive

- **WHEN** an amd64 or arm64 archive is built
- **THEN** it includes the Apache-2.0 license, HostLens attribution, and dependency notices with valid manifest checksums

#### Scenario: Missing licensing material

- **WHEN** the project license or attribution notice is absent
- **THEN** packaging fails instead of producing an archive without that material

### Requirement: Safe service-group adoption

Installation SHALL reject service groups with zero, malformed, or colliding numeric group identities, or explicitly listed unrelated members. Checks SHALL run before planning any resource mutation and again before installing service files after identity creation.

#### Scenario: Privileged group alias

- **WHEN** an existing HostLens group resolves to GID zero
- **THEN** installation rejects the identity without installing files or changing permissions

#### Scenario: Unrelated explicit member

- **WHEN** a service group lists an unrelated local account
- **THEN** installation reports the group conflict instead of granting that account access to HostLens state

### Requirement: Ownership verification failures are explicit

Uninstall SHALL distinguish an unsuccessful ownership scan from a scan that discovers unrelated owned files. Either outcome SHALL preserve the affected account or group. A failed scan SHALL NOT be reported as proof of unexpected ownership.

#### Scenario: Inaccessible user filesystem

- **WHEN** root cannot traverse a user FUSE mount during ownership verification
- **THEN** uninstall reports that the ownership check failed and preserves the identity

#### Scenario: Retry after restoring visibility

- **WHEN** ownership can subsequently be checked and no unrelated owned files remain
- **THEN** a retry may remove the tracked identity

### Requirement: Rollback survives candidate restart exhaustion

Rollback SHALL permit a bounded restart attempt for restored binaries when a crashing candidate has exhausted the service manager's start-rate limit. It SHALL retain normal service rate limiting outside that recovery attempt and SHALL NOT report successful restoration until readiness succeeds.

#### Scenario: Candidate repeatedly exits

- **WHEN** candidate activation fails after exhausting systemd restart attempts
- **THEN** rollback restores previous binaries, resets the affected previously active service's failure counter, and verifies its restart

#### Scenario: Failure counter cannot be reset

- **WHEN** the service manager rejects recovery reset
- **THEN** rollback reports the recovery failure and retains incomplete installation state

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

### Requirement: Generated unit sandbox baseline

Generated systemd service units SHALL deny syscall surface beyond the repository baseline: every service SHALL declare `SystemCallArchitectures=native`, a `@system-service openat2` allow-list with a `@mount @reboot @swap @raw-io` deny-list, `LockPersonality=yes`, `RestrictRealtime=yes`, and `RestrictNamespaces=yes`. The gateway SHALL keep `AF_INET`, `AF_INET6`, and `AF_UNIX` address families; the diagnostics backend and observer SHALL keep `AF_UNIX` only. Every service SHALL declare `ProtectClock=yes`, `ProtectHostname=yes`, `ProtectKernelLogs=yes`, and `PrivateIPC=yes`. The gateway and observer SHALL additionally declare `ProtectProc=invisible` and `ProcSubset=pid`; the diagnostics backend SHALL NOT declare them because its audit evidence requires system-wide `/proc` files and other users' process entries.

#### Scenario: Gateway starts under the filter set

- **WHEN** the gateway service starts with the sandbox baseline applied
- **THEN** TLS and MCP traffic, token verification, and reloads work with no sandbox denials in service logs

#### Scenario: Diagnostics keep system-wide proc evidence

- **WHEN** the diagnostics backend audits network, storage, or process evidence while its unit omits `ProtectProc` and `ProcSubset`
- **THEN** system-wide `/proc` files and other users' process entries remain observable instead of being silently hidden

#### Scenario: Acceptance asserts the baseline

- **WHEN** the systemd acceptance suite inspects the installed units
- **THEN** every declared sandbox directive is present on each unit it is specified for
