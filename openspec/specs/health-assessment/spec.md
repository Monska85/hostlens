# Health assessment

## Purpose

Define the observable HostLens v1 health assessment behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Current host measurements

get_health_snapshot SHALL report available memory pressure, swap usage, filesystem space and inode usage, failed systemd services, Linux load averages over 1, 5, and 15 minutes, logical CPU availability, and sampled CPU utilization. Measurement windows and sources SHALL be explicit where needed. Unavailable values SHALL be omitted under the diagnostic data contract.

#### Scenario: Load interpretation

- **WHEN** load averages are returned
- **THEN** CPU availability accompanies them and high load is not asserted to prove CPU saturation

#### Scenario: Sampling failure

- **WHEN** CPU utilization cannot be sampled within the call budget
- **THEN** utilization is omitted and a collection issue is returned

### Requirement: Configurable assessments

Health thresholds, required checks, exclusions, and sampling windows SHALL be configurable in YAML. Each evaluated check SHALL report OK, warning, critical, or unknown with observed evidence. These status values SHALL NOT be used as placeholders for absent measurements.

#### Scenario: Excluded filesystem

- **WHEN** an administrator excludes a filesystem
- **THEN** it does not generate a health alert and the configured coverage remains inspectable

#### Scenario: Threshold exceeded

- **WHEN** an observed metric crosses a configured critical threshold
- **THEN** the check reports critical with its observation and applied threshold

### Requirement: Incomplete coverage

Overall severity and coverage completeness SHALL be represented separately. Failed required checks SHALL prevent an unqualified healthy result. Host health SHALL NOT claim application health, incident absence, or historical capacity without corresponding evidence.

#### Scenario: Mixed findings

- **WHEN** disk usage is warning and required service inspection fails
- **THEN** the result preserves warning severity and marks coverage incomplete

#### Scenario: Complete checks

- **WHEN** all required checks complete successfully within configured thresholds
- **THEN** the snapshot reports OK within the performed scope and observation time

### Requirement: Service health preserves observed failures

Health assessment SHALL evaluate all service observations collected within its inspection budget, independently of the public list page size. Partial service coverage SHALL retain observed failed units and their severity while marking coverage incomplete.

#### Scenario: More services than a public page

- **WHEN** service discovery returns more units than the configured list page and a unit is failed
- **THEN** health assessment includes the failed unit without treating pagination as missing inspection coverage

#### Scenario: Failure observed with partial coverage

- **WHEN** one unit is observed failed while another service observation is denied or unavailable
- **THEN** the failed-unit severity remains visible and coverage is incomplete
