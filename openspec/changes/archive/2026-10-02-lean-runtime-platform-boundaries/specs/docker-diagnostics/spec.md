## MODIFIED Requirements

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

#### Scenario: Disk-usage container lacks a live name record

- **WHEN** a disk-usage container is absent from the correlated container inventory
- **THEN** HostLens omits its row, reports the inventory gap, and suppresses unused-resource certainty

#### Scenario: Dangling image in disk usage

- **WHEN** a disk-usage observation includes an image with no repository tags or digests
- **THEN** HostLens reports that image as dangling in the disk-usage projection, and includes its identity among dangling cleanup candidates only when the reference observation is stable and policy permits it
