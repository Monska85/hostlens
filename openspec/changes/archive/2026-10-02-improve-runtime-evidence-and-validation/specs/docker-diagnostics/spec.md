## MODIFIED Requirements

### Requirement: Daemon and container evidence

Docker diagnostics SHALL report selected daemon version, API, storage, cgroup, logging, security, and count information without registry credentials, proxy secrets, plugin configuration, or raw daemon configuration. Container tools SHALL provide bounded current identity, image reference and digest when available, lifecycle state, health state, restart count, timestamps, port mappings, mount type and destination, resource limits, and selected resource usage. They SHALL omit environment values, secret/config contents, commands and arguments, embedded files, raw inspect objects, and unrestricted labels or annotations. An unlimited process limit SHALL be omitted rather than interpreted as a finite limit or causing the whole stats observation to fail.

#### Scenario: Unhealthy container

- **WHEN** an allowed container reports an unhealthy state
- **THEN** HostLens returns the observed state and timestamps without exposing health-command output that contains unapproved payload data

#### Scenario: Resource snapshot

- **WHEN** an authorized client requests stats for a running container
- **THEN** HostLens returns one bounded point-in-time sample with accurate CPU, memory, block, network, and process measurements that the daemon supplies

#### Scenario: Unlimited PID limit

- **WHEN** Docker reports the unsigned maximum value for an unlimited PID limit
- **THEN** HostLens returns the valid stats observation and omits the finite `pids_limit` field

#### Scenario: Stopped container stats

- **WHEN** stats are unavailable for a stopped container
- **THEN** HostLens reports the unavailable measurement and retains the container's valid lifecycle evidence
