# Spec Delta

## ADDED Requirements

### Requirement: Actionable host health

`host_status` SHALL add current swap, filesystem inode pressure, and load observations when available. It SHALL report measurement windows and unavailable sources, keep optional values absent when not observed, and SHALL not label incomplete evidence healthy. A bounded failed-service summary SHALL identify observed failing units without claiming completeness beyond its limit.

#### Scenario: Partial host measurements

- **WHEN** a filesystem or service-manager measurement fails while other sources succeed
- **THEN** HostLens returns the successful observations and an explicit issue for each failed source

### Requirement: Bounded service discovery and detail

`list_services` SHALL list only profile-permitted systemd units with a fixed bound and no denied-unit count. `service_status` SHALL include available failure and restart metadata without exposing environment, command-line secrets, or unit file bodies. Both tools SHALL use the same role and profile gate as other observations.

#### Scenario: Denied service

- **WHEN** a service is denied by an active profile
- **THEN** neither its listing nor its detail is returned

#### Scenario: Failed unit

- **WHEN** a permitted unit has failed
- **THEN** its result and relevant failure state are reported when available

### Requirement: Bounded service journal

`service_logs` SHALL read recent journal entries for one exact, permitted systemd service under a separate log grant. It SHALL bound count, age, and response bytes, preserve source timestamps and ordering, and report truncation or unavailable journal access. Journal text SHALL be treated as untrusted data.

#### Scenario: Allowed recent logs

- **WHEN** a client requests a granted unit and a bounded recent window
- **THEN** HostLens returns only entries associated with that unit within the requested bounds

#### Scenario: Denied logs

- **WHEN** status is granted but logs are not
- **THEN** HostLens denies `service_logs` before journal access
