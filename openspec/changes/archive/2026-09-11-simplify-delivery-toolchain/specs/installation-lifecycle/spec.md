## MODIFIED Requirements

### Requirement: Archive distribution and explicit installation

V1 SHALL distribute amd64 and arm64 executable archives with example YAML, shipped profiles, and installation and upgrade instructions. Native packages SHALL be excluded. A local administrator install operation SHALL generate systemd definitions from the selected configuration, show its plan, record changes, preserve existing configuration, and refuse conflicts. Archives SHALL NOT need pre-rendered copies of those definitions. Services SHALL remain stopped unless explicitly requested.

#### Scenario: Fresh installation

- **WHEN** the administrator accepts the plan without a start option
- **THEN** accounts and services are installed but do not start

#### Scenario: Conflict

- **WHEN** an existing installation cannot be safely adopted
- **THEN** installation stops without overwriting it

#### Scenario: Configured service definitions

- **WHEN** the administrator installs either supported privilege mode
- **THEN** the installer generates the applicable hardened units without reading service templates from the archive
