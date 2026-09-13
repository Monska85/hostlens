# docker-diagnostics Specification

## Purpose
Provide bounded, policy-controlled evidence from the local system-wide Docker Engine without retaining observations or exposing Docker mutation authority through MCP.

## Requirements

### Requirement: System-wide Docker capability

HostLens SHALL support an explicitly enabled local rootful Docker Engine on Linux through its Unix socket. It SHALL discover daemon and API capabilities at runtime and SHALL report the integration unavailable when disabled, absent, inaccessible, incompatible, or not a local system-wide engine. Rootless and remote engines SHALL remain unsupported in this change.

#### Scenario: Compatible local engine

- **WHEN** an administrator enables Docker diagnostics and the observer reaches a compatible system-wide engine
- **THEN** authorized Docker tools are discoverable with the observed engine identity and API scope

#### Scenario: Docker unavailable

- **WHEN** Docker is disabled, absent, stopped, inaccessible, or incompatible
- **THEN** Docker tools are omitted from discovery or return explicit unavailability without affecting non-Docker diagnostics

#### Scenario: Rootless socket configured

- **WHEN** configuration selects a user-session or rootless Docker socket
- **THEN** validation rejects it as unsupported rather than claiming system-wide coverage

### Requirement: Docker diagnostic tool surface

HostLens SHALL expose `get_docker_info`, `list_docker_containers`, `get_docker_container`, `get_docker_container_stats`, `list_docker_images`, `list_docker_volumes`, `list_docker_networks`, `get_docker_disk_usage`, and `query_docker_logs` as diagnostic-effect MCP tools. They SHALL require the diagnostics role, explicit Docker policy grants, active runtime capability, and the global MCP read-only gate. No generic Docker request, command, exec, attach, file-copy, event-stream, registry, build, or plugin tool SHALL be exposed.

#### Scenario: Authorized discovery

- **WHEN** a diagnostics client has an explicit grant for available Docker resources
- **THEN** discovery lists only the Docker tools supported by that grant and the live engine

#### Scenario: Direct ungranted call

- **WHEN** a client directly calls a Docker tool outside its role or policy
- **THEN** the call is rejected before the observer or Docker Engine is contacted

#### Scenario: Generic Docker operation

- **WHEN** an MCP client supplies an arbitrary Docker API path, method, command, or exec request
- **THEN** schema or dispatch rejects it without forwarding any part to Docker

### Requirement: Daemon and container evidence

Docker diagnostics SHALL report selected daemon version, API, storage, cgroup, logging, security, and count information without registry credentials, proxy secrets, plugin configuration, or raw daemon configuration. Container tools SHALL provide bounded current identity, image reference and digest when available, lifecycle state, health state, restart count, timestamps, port mappings, mount type and destination, resource limits, and selected resource usage. They SHALL omit environment values, secret/config contents, commands and arguments, embedded files, raw inspect objects, and unrestricted labels or annotations.

#### Scenario: Unhealthy container

- **WHEN** an allowed container reports an unhealthy state
- **THEN** HostLens returns the observed state and timestamps without exposing health-command output that contains unapproved payload data

#### Scenario: Resource snapshot

- **WHEN** an authorized client requests stats for a running container
- **THEN** HostLens returns one bounded point-in-time sample with accurate CPU, memory, block, network, and process measurements that the daemon supplies

#### Scenario: Stopped container stats

- **WHEN** stats are unavailable for a stopped container
- **THEN** HostLens reports the unavailable measurement and retains the container's valid lifecycle evidence

### Requirement: Image, volume, network, and disk evidence

Docker diagnostics SHALL return bounded, deduplicated image identities and tags, current container reference counts, content and unique size when available, volume names and drivers, current container references, volume size when available, network identity and selected non-secret configuration, and daemon disk totals and reclaimable estimates. Results SHALL preserve Docker's shared-layer accounting semantics and SHALL NOT add per-resource sizes into a fabricated physical total.

#### Scenario: Shared image layers

- **WHEN** images share content layers
- **THEN** HostLens distinguishes shared, unique, and reported content sizes where Docker supplies them and does not sum virtual sizes as physical usage

#### Scenario: Unmeasurable volume

- **WHEN** the volume driver or daemon does not provide a size
- **THEN** HostLens omits the size and reports the measurement gap without substituting zero

#### Scenario: Denied resource within a list

- **WHEN** a list grant is active but policy denies one image, volume, network, or container
- **THEN** the denied resource and its derived counts are omitted and the result reports filtered coverage

### Requirement: Honest unused-resource analysis

HostLens SHALL classify an image or volume as currently unused only when the live daemon inventory shows no reference from any container, including stopped containers. It SHALL classify dangling and unused as distinct facts. It MAY return creation time when Docker supplies it, but SHALL NOT infer last-use time or unused duration from creation time, image age, daemon uptime, filesystem timestamps, or a previous HostLens request. Cleanup-candidate output SHALL state its live criteria and remain advisory.

#### Scenario: Volume referenced by a stopped container

- **WHEN** a stopped container references a volume
- **THEN** HostLens does not classify that volume as unused

#### Scenario: Image has no container references

- **WHEN** no current or stopped container references an image identity
- **THEN** HostLens reports it as currently unused at the observation time without claiming how long it has been unused

#### Scenario: Last-use timestamp unavailable

- **WHEN** Docker supplies creation time but no reliable last-reference time
- **THEN** HostLens returns creation time separately, omits `unused_since` and `unused_duration`, and reports that last-use evidence is unavailable

#### Scenario: Concurrent Docker change

- **WHEN** a container reference changes while an inventory is collected
- **THEN** HostLens marks the analysis as non-atomic or retries within its bound and never presents an uncertain resource as a guaranteed cleanup target

### Requirement: Bounded Docker log queries

`query_docker_logs` SHALL require one policy-permitted container resolved to a stable identity and bounded since, until, count, and byte limits. It SHALL return stdout and stderr records only when the daemon logging interface supports retrieval, preserve stream and timestamp meaning, treat content as untrusted data, and report rotation and driver coverage. It SHALL NOT request follow mode or retain returned records.

#### Scenario: Supported log driver

- **WHEN** an allowed container and log driver provide records for the requested window
- **THEN** HostLens returns bounded timestamped records with explicit truncation and source scope

#### Scenario: Unsupported log driver

- **WHEN** the configured driver cannot return container logs through Docker
- **THEN** HostLens reports the interface gap without claiming that no logs or incidents exist

#### Scenario: Container identity changes

- **WHEN** a name resolves to a different container before log retrieval
- **THEN** HostLens rejects or reauthorizes the new stable identity and returns no records from an unapproved container

### Requirement: Stateless and read-only Docker collection

Each Docker result SHALL be calculated from live daemon responses within the active MCP request. HostLens SHALL NOT retain Docker inventories, log records, stats history, cleanup candidates, last-use estimates, or remediation plans. Docker diagnostic operations SHALL use only the fixed observation contract and SHALL NOT create, start, stop, restart, pause, update, rename, exec into, remove, prune, pull, push, tag, build, or otherwise mutate Docker or its resources.

#### Scenario: Resource changes between calls

- **WHEN** Docker state changes between two requests
- **THEN** the second result reflects a new daemon observation rather than retained HostLens state

#### Scenario: Mutation-shaped argument

- **WHEN** a client adds an action, force, prune, delete, exec, or arbitrary API argument to a Docker diagnostic tool
- **THEN** schema validation rejects the request before observer access

#### Scenario: Request completes

- **WHEN** a Docker request succeeds, fails, times out, or is cancelled
- **THEN** HostLens releases its observations and retains only payload-free operational metadata and aggregate service counters
