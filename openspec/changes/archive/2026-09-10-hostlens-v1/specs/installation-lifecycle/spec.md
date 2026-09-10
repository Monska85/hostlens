# Installation lifecycle

## Purpose

Define the observable HostLens v1 installation lifecycle behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## ADDED Requirements

### Requirement: Archive distribution and explicit installation

V1 SHALL distribute amd64 and arm64 executable archives with example YAML, shipped profiles, systemd definitions, and installation and upgrade instructions. Native packages SHALL be excluded. A local administrator install operation SHALL show its plan, record changes, preserve existing configuration, and refuse conflicts. Services SHALL remain stopped unless explicitly requested.

#### Scenario: Fresh installation

- **WHEN** the administrator accepts the plan without a start option
- **THEN** accounts and services are installed but do not start

#### Scenario: Conflict

- **WHEN** an existing installation cannot be safely adopted
- **THEN** installation stops without overwriting it

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
