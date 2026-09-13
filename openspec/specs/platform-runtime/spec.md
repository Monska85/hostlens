# Platform runtime

## Purpose

Define the observable HostLens v1 platform runtime behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Supported runtime and capability discovery

HostLens SHALL run on Linux amd64 and arm64 and detect available collectors from system interfaces rather than reject unlisted distribution versions. Debian, Ubuntu, and Arch SHALL be representative test environments; systemd SHALL be the v1 installation integration.

#### Scenario: New distribution release

- **WHEN** the OS version is not in the recorded test matrix but required interfaces are available
- **THEN** HostLens runs the supported collectors without a distribution-version gate

#### Scenario: Missing interface

- **WHEN** systemd or a package manager interface is unavailable
- **THEN** the affected capability is reported unavailable without a fabricated substitute

### Requirement: Independent local components

The gateway and diagnostic backend SHALL run as separate identities and independently supervised processes. The gateway SHALL have no additional host-read capability. Backend IPC SHALL be local, access-controlled, structured, and restricted to supported diagnostic operations.

#### Scenario: Unauthorized local caller

- **WHEN** an unrelated local user attempts a backend connection
- **THEN** access is rejected

#### Scenario: Backend unavailable

- **WHEN** the backend cannot be reached
- **THEN** the gateway reports diagnostic unavailability rather than healthy results

### Requirement: Platform extension boundaries

Shared behavior SHALL remain independent of platform-specific collectors, filesystem semantics, service identities, IPC, and reload triggers. V1 SHALL expose no macOS or Windows implementation claims.

#### Scenario: Future platform

- **WHEN** a Windows collector is added later
- **THEN** it can use named pipes and native event sources without changing the meaning of Linux journal or file rules

### Requirement: Diagnostic-only release

V1 SHALL expose no remediation backend or remediation tools. Enabling remediation SHALL fail configuration validation. Known or cached remediation tool calls SHALL be rejected.

#### Scenario: Attempted enablement

- **WHEN** configuration enables remediation
- **THEN** startup or reload rejects that configuration and no remediation process starts

#### Scenario: Cached tool

- **WHEN** a client calls a remembered remediation tool name
- **THEN** the gateway rejects the operation

### Requirement: Client-independent host access

HostLens SHALL allow independently authorized compatible MCP clients to connect to the same host gateway. The host SHALL provide observed identity in diagnostic results; client-configured connection labels SHALL NOT substitute for the target's identity. Core access SHALL require neither a particular model provider nor Ansible provisioning.

#### Scenario: Interactive and scheduled clients

- **WHEN** an interactive agent and an externally scheduled application connect with different tokens
- **THEN** both use the same MCP endpoint under their individual roles and shared host policy

#### Scenario: Misnamed connection

- **WHEN** a client labels a connection with another server's name
- **THEN** HostLens results still identify the observed target rather than copying the client label

### Requirement: Bounded capability discovery

Capability discovery SHALL use a consistent active configuration snapshot without holding shared state locks during native observation. Stalled discovery SHALL retain a bounded amount of underlying work and SHALL NOT prevent unrelated admission decisions or validated state activation.

#### Scenario: Native discovery stalls

- **WHEN** a capability observation stalls in a native interface
- **THEN** further discovery cannot create unbounded abandoned work and state access remains responsive

### Requirement: MCP and local administration authority separation

The MCP read-only setting SHALL govern only MCP tool discovery and execution. Authenticated local administrator operations, including installation, upgrade, uninstall, token administration, configuration validation, reload, status, and telemetry, SHALL remain available under their existing operating-system and local IPC controls regardless of `mcp.read_only`. Local administrator operations SHALL NOT be registered as MCP tools.

#### Scenario: Local operation in MCP read-only mode

- **WHEN** `mcp.read_only` is `true` and an authorized local administrator reloads configuration or manages a token
- **THEN** the operation remains available through the local administrative boundary

#### Scenario: Remote administrative name

- **WHEN** an MCP client calls a name used by a local administrative operation
- **THEN** the gateway rejects it because the operation is not part of the MCP tool surface

### Requirement: Separated mutation authority

The MCP gateway and diagnostic backend SHALL hold no mutation credential, generic command channel, or unrestricted external API pass-through. A diagnostic integration with an external interface that combines reads and writes SHALL use a constrained observer boundary that permits only the documented read operations. Any future remediation SHALL run through a separately authorized component, use distinct operation contracts and credentials, and revalidate relevant live state immediately before mutation. It SHALL NOT execute from retained diagnostic evidence or a retained remediation plan.

#### Scenario: Mixed read-write external API

- **WHEN** a diagnostic collector needs evidence from an API that also supports mutation
- **THEN** its runtime authority exposes only the approved observation operations and no generic API forwarding

#### Scenario: Diagnostic component compromise boundary

- **WHEN** the gateway or diagnostic backend attempts a mutation operation
- **THEN** the local IPC or observer boundary rejects the operation because that component has no mutation contract or authority

#### Scenario: Future remediation state changes

- **WHEN** an authorized future remediation operation reaches its execution boundary
- **THEN** it re-fetches and validates the relevant live state before applying the requested change

### Requirement: Isolated Docker observer

Docker Engine access SHALL reside in a separately built and supervised local observer with a dedicated service identity and IPC endpoint. The observer SHALL accept only a versioned, typed allowlist of Docker observation operations and construct fixed GET requests itself. It SHALL reject unknown operations, arbitrary paths or methods, non-GET methods, mutation verbs, registry authentication, raw response forwarding, and calls from identities other than the diagnostic backend. The gateway and diagnostic backend SHALL have no direct Docker socket access.

#### Scenario: Unauthorized local client

- **WHEN** an unrelated local process connects to the observer IPC
- **THEN** the observer rejects it before Docker access

#### Scenario: Arbitrary upstream request

- **WHEN** a caller supplies a Docker API path, HTTP method, header, or body outside a typed observation request
- **THEN** the observer rejects the request rather than proxying it

#### Scenario: Observer unavailable

- **WHEN** the observer exits or loses Docker access
- **THEN** Docker diagnostics become unavailable while the gateway and other diagnostics remain operational

### Requirement: Linux socket-activated observer

The Linux system installation SHALL expose the observer through a HostLens-owned systemd socket unit and SHALL start the observer service only when the diagnostic backend requests Docker evidence. The service SHALL use `SupplementaryGroups=` for the validated Docker socket group and SHALL declare `Requisite=docker.service`, `After=docker.service`, and `PartOf=docker.service`. It SHALL NOT use `ConditionPathIsSocket=`. The observer SHALL validate the configured Docker socket type, locality, ownership, group, and permissions before connecting.

#### Scenario: Docker request while active

- **WHEN** the observer is dormant, Docker is active, and the diagnostic backend connects to the HostLens observer socket
- **THEN** systemd starts the observer with process-scoped Docker group authority and the observer validates the Docker socket before issuing a GET observation

#### Scenario: Docker request while stopped

- **WHEN** Docker is stopped and a diagnostic request reaches the observer socket
- **THEN** the observer does not become available, HostLens reports Docker unavailable, and no HostLens unit starts Docker

#### Scenario: Docker stops after activation

- **WHEN** Docker stops or restarts while the observer is active
- **THEN** systemd stops the observer through the declared dependency and other HostLens processes remain operational

### Requirement: Docker runtime scope

The initial integration SHALL target the local rootful Docker Engine on supported Linux architectures and SHALL negotiate only a verified compatible Engine API range. Optional fields and endpoints SHALL be capability-detected. Rootless Docker, remote engines, Docker Desktop, Swarm administration, Kubernetes, Podman, containerd direct access, and Windows containers SHALL remain unclaimed.

#### Scenario: New compatible Docker release

- **WHEN** a newer daemon preserves the negotiated observation API
- **THEN** HostLens uses supported observations without an exact daemon-version allowlist

#### Scenario: API below required range

- **WHEN** the daemon cannot negotiate the minimum observation API
- **THEN** Docker capability is unavailable with the observed version and no fallback to CLI scraping

### Requirement: Portable observer contract with native adapters

The observer operation identifiers, typed request and response structures, bounds, capability states, and evidence-gap semantics SHALL remain independent of operating-system paths, identities, IPC, credential stores, and service managers. The Linux implementation SHALL isolate Unix-socket Docker transport, Unix peer authentication, systemd supervision, filesystem paths, and user and group discovery in Linux-specific composition. It SHALL NOT add unused Windows or macOS runtime adapters or claim that portable compilation proves native support.

#### Scenario: Shared contract inspection

- **WHEN** a shared observer request, response, or MCP projection is reviewed
- **THEN** it contains Docker observation semantics without Unix paths, numeric Unix identities, systemd units, Windows service details, or macOS launch details

#### Scenario: Future native platform

- **WHEN** Windows or macOS Docker support is proposed
- **THEN** its change supplies native identity, IPC, credential, supervision, packaging, lifecycle, threat, and acceptance designs while preserving the typed observation contract where semantics match

#### Scenario: Docker Desktop encountered

- **WHEN** the Linux implementation detects Docker Desktop or a per-user, remote, or virtualized endpoint outside its declared system-wide scope
- **THEN** it reports the mode as unsupported and does not weaken isolation or reinterpret it as a supported local rootful engine
