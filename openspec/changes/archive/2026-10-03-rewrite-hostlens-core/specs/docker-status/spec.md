# Spec Delta

## Purpose

Provide live, bounded status for the local Docker Engine and its containers while keeping daemon credentials and unrestricted API operations outside the MCP interface.

## ADDED Requirements

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
