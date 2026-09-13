## ADDED Requirements

### Requirement: Docker diagnostic acceptance

Release validation SHALL exercise the Docker tool surface against disposable rootful Docker fixtures with running, stopped, unhealthy, and name-reused containers; referenced and unreferenced images and volumes; shared layers; networks; bounded logs; resource stats; unsupported fields; daemon loss; cancellation; and concurrent inventory changes. Tests SHALL compare selected observations with the daemon's live state and SHALL distinguish Docker, container, and HostLens namespace limits.

#### Scenario: Representative engines

- **WHEN** Docker acceptance runs on representative Debian and Arch system-wide engines
- **THEN** the report records daemon and API versions, architecture, storage driver, cgroup mode, native or nested execution, and unavailable host-level evidence

#### Scenario: Live inventory change

- **WHEN** a disposable resource is added, referenced, unreferenced, and removed between requests
- **THEN** each result matches the new live state without relying on retained HostLens inventory

#### Scenario: Engine interruption

- **WHEN** the disposable Docker endpoint becomes unavailable during collection
- **THEN** Docker results report failure or partial coverage and other HostLens diagnostics remain usable

### Requirement: Docker no-mutation proof

Validation SHALL record every daemon request issued by the observer and fail on any method, endpoint, header, body, or option outside the maintained observation allowlist. It SHALL snapshot containers, images, volumes, networks, daemon configuration, and Docker service state before and after every Docker MCP tool, including malformed, unauthorized, timed-out, and cancelled calls, and SHALL require no HostLens-caused change. A read-only socket mount SHALL NOT count as proof because Docker Unix-socket mount mode does not restrict API methods.

#### Scenario: Every Docker tool

- **WHEN** the complete registered Docker tool set runs against the recording fixture
- **THEN** every upstream request matches the read allowlist and the Docker state snapshot remains unchanged

#### Scenario: Bypass inputs

- **WHEN** adversarial arguments attempt exec, copy, build, registry, container lifecycle, resource removal, prune, method override, path traversal, or request smuggling
- **THEN** rejection occurs before a disallowed daemon request and no Docker state changes

#### Scenario: MCP read-only toggle

- **WHEN** acceptance runs Docker tools with MCP read-only mode enabled and disabled
- **THEN** the Docker surface and upstream observation-only request set remain identical

### Requirement: Docker lifecycle acceptance

Lifecycle validation SHALL cover fresh install, reconciliation, upgrade, rollback, interrupted install, uninstall, Docker installed after HostLens, absent Docker, stopped Docker, access denial, and pre-existing resource conflicts. It SHALL prove that only HostLens-owned observer resources change and that Docker daemon configuration, daemon lifecycle, containers, images, volumes, networks, existing identities, and unrelated memberships remain unchanged.

#### Scenario: Install and uninstall

- **WHEN** a Docker-enabled HostLens installation completes and is then uninstalled
- **THEN** Docker state matches its pre-install snapshot and no HostLens-owned observer access remains

#### Scenario: Docker service remains running

- **WHEN** HostLens installs, upgrades, rolls back, or uninstalls beside a running Docker daemon
- **THEN** Docker is not restarted or reconfigured and its existing workloads remain in their prior states

#### Scenario: Late Docker installation

- **WHEN** HostLens is installed and reconciled with Docker diagnostics enabled before Docker is installed
- **THEN** installing and starting Docker allows the next diagnostic request to activate the observer with process-scoped access, without another HostLens reconciliation or persistent account membership change

#### Scenario: Socket activation and dependency direction

- **WHEN** lifecycle acceptance requests Docker evidence with Docker running, stopped, restarted, and removed
- **THEN** it proves on-demand observer activation, observer shutdown with Docker, no `ConditionPathIsSocket=` dependency, and no HostLens-caused Docker start or restart

### Requirement: Deferred platform boundary validation

Release validation SHALL keep portable observer contracts free of Linux lifecycle and transport fields and SHALL cross-build portable packages for the existing Windows and macOS targets. These checks SHALL be reported as structural evidence only. They SHALL NOT claim Windows or macOS runtime support without native security, lifecycle, and end-to-end acceptance in a future platform change.

#### Scenario: Portable cross-build succeeds

- **WHEN** shared observer contracts and MCP projections compile for Windows and macOS
- **THEN** validation reports portable compilation while continuing to identify both platforms as runtime-unsupported
