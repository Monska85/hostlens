# docker-status Specification

## Purpose

Provide live, bounded status for the local Docker Engine and its containers while keeping daemon credentials and unrestricted API operations outside the MCP interface.

## Requirements

### Requirement: Local Docker observation

HostLens SHALL expose typed Docker engine and container status for a configured local rootful Engine. Docker observations SHALL be request scoped, limited in size and time, and filtered by the client's active profile. Environment variables, commands, secrets, raw daemon objects, and unrestricted labels SHALL not be returned.
Direct container status SHALL require a full stable ID before profile authorization so that a short prefix cannot evade a denial on the resolved ID.

#### Scenario: Engine available

- **WHEN** an authorized client requests Docker status and the local Engine is reachable
- **THEN** `docker_status` returns the observed version and `list_docker_containers` returns bounded current states filtered by active grants

#### Scenario: Engine unavailable

- **WHEN** Docker is absent, stopped, inaccessible, or incompatible
- **THEN** HostLens reports unavailability and preserves other host observations

#### Scenario: Denied container

- **WHEN** an active profile denies a container
- **THEN** neither its details nor its derived counts appear in the response

#### Scenario: Bounded inventory with denied containers

- **WHEN** Docker has more containers than the fixed inventory limit and the profile denies their IDs
- **THEN** the result reports the fixed limit without exposing a count or truncation signal derived from those denied containers

### Requirement: Fixed Docker surface

Diagnostic tools SHALL expose named observations only. MCP clients SHALL not supply arbitrary Docker paths, methods, registry credentials, exec requests, or raw daemon payloads.

#### Scenario: Arbitrary API request

- **WHEN** a client sends a Docker API path or mutation-shaped argument to a diagnostic tool
- **THEN** HostLens rejects it before any Docker operation

### Requirement: Actionable container evidence

`container_status` SHALL include available native health and restart metadata without environment, command, labels, or health-check output. `container_stats` SHALL return a bounded current resource sample for one full stable container ID under a separate grant. `container_logs` SHALL return bounded recent stdout and stderr for one full stable ID under a separate grant, preserving source stream and ordering while treating content as untrusted. Each observation SHALL use the fixed Docker observer surface and report unavailable evidence.

#### Scenario: Unhealthy container

- **WHEN** a permitted container has a native unhealthy state
- **THEN** its status reports that state without returning health-check output

#### Scenario: Stats or logs denied

- **WHEN** the client can observe container state but lacks the distinct stats or logs grant
- **THEN** HostLens denies that evidence request before contacting Docker

#### Scenario: Bounded logs

- **WHEN** a granted log request would exceed its time, count, or byte ceiling
- **THEN** HostLens returns only bounded entries with an explicit truncation or gap indication
