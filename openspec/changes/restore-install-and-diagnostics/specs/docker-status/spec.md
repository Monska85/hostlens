# Spec Delta

## ADDED Requirements

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
