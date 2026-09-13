## ADDED Requirements

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
