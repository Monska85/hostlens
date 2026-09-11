# Production audit

## Purpose

Provide bounded, application-independent operating-system evidence for agent-led server audits and investigations through MCP.

## Requirements

### Requirement: Explicit audit authority

New audit tools SHALL require the diagnostics role and an explicit active grant for their audit domain. Existing file and journal denials and mandatory exclusions SHALL remain effective. The existing OS privilege model SHALL remain unchanged.

#### Scenario: Existing broad file grant

- **WHEN** a diagnostics client has broad file access but no audit grant
- **THEN** new audit collection is denied without executing its collector

#### Scenario: Denial overrides grant

- **WHEN** an included profile grants an audit domain and another active rule denies it
- **THEN** collection is denied, including direct calls to a cached tool

### Requirement: General process and service investigation

Audit tools SHALL expose bounded process lists, selected process identity/resource facts and selected native service runtime, dependency and hardening properties. They SHALL omit process environments, command arguments and unrestricted service properties. PID and service selectors SHALL be validated and snapshots SHALL NOT claim atomicity.

#### Scenario: Unknown application

- **WHEN** an authorized agent investigates a service without an application integration
- **THEN** it can correlate service identity and main process with available generic process observations

#### Scenario: Process disappeared

- **WHEN** a selected process exits or its identity changes during observation
- **THEN** the result reports the race or absence without fabricating process values

### Requirement: Generic OS audit evidence

Audit tools SHALL provide available network interface/address/route/socket evidence, local account/group metadata, storage device/mount evidence, local software-maintenance evidence and selected OS security controls. Each domain SHALL state source and namespace scope, coverage and unavailable evidence. No privileged helper, network scan, repository refresh or modifying native command SHALL run.

#### Scenario: Firewall inaccessible

- **WHEN** the existing privileges or sandbox prevent active firewall inspection
- **THEN** network results identify the missing evidence and do not infer active firewall rules from configuration files

#### Scenario: Maintenance metadata

- **WHEN** update information is obtained from local package metadata
- **THEN** the response identifies its freshness limitations and does not claim a current vulnerability assessment

#### Scenario: Sensitive account material

- **WHEN** account metadata is collected
- **THEN** no password hash or authentication secret is returned

### Requirement: Policy-bound metadata and self inspection

An explicit path inspection SHALL return selected metadata for a safely resolved allowed file or directory without reading its content or traversing descendants. HostLens self inspection SHALL return selected effective non-secret security settings and runtime facts, never token state, private keys or administrative source contents.

#### Scenario: Protected path

- **WHEN** a client requests metadata for a mandatory protected source or an alias escaping policy
- **THEN** the operation fails without revealing protected metadata

#### Scenario: Deployment inspection

- **WHEN** an authorized client inspects HostLens
- **THEN** configured privilege, relevant limits and selected security settings are available without credential retrieval

### Requirement: Bounded honest audit results

Audit operations SHALL honor shared time, concurrency, input, output and pagination ceilings. Results SHALL distinguish policy denial, insufficient OS authority, unavailable sources, malformed observations and truncation. Partial evidence SHALL remain usable without implying complete audit coverage.

#### Scenario: Malformed or oversized observation

- **WHEN** a native source exceeds a bound or cannot be parsed
- **THEN** the collector reports the failure or truncation explicitly and does not manufacture an empty successful observation

#### Scenario: Invalid selector

- **WHEN** a client supplies unrelated fields, a nonpositive PID or a command-shaped service selector
- **THEN** the request is rejected before native collection

### Requirement: Application-independent acceptance

Acceptance SHALL demonstrate investigation of PostgreSQL, Apache, nginx and MySQL installations through generic MCP tools and administrator-approved configuration/log sources, without application-specific collector code or SQL execution. It SHALL identify inaccessible evidence and distinguish OS/configuration findings from database-internal conclusions.

#### Scenario: Database investigation

- **WHEN** an agent inspects the disposable database fixture
- **THEN** service/process, network, resource, approved configuration/log and permission evidence are obtained through MCP or reported as explicit permission/interface gaps
