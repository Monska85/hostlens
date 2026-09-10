## MODIFIED Requirements

### Requirement: Shared selectable container matrix

Local and hosted acceptance SHALL use the same named platform and lifecycle cases. A target SHALL select exactly one case; omitted selection SHALL run all cases. Unknown targets SHALL fail before launching tests. Local execution SHALL run cases sequentially, expose their output, aggregate failures, and stop its active container when interrupted. Hosted execution MAY use the CI platform's native parallelism.

#### Scenario: Focused check

- **WHEN** a developer selects a valid native amd64 platform target
- **THEN** only that case runs and unrelated QEMU or systemd prerequisites are not required

#### Scenario: Invalid selection

- **WHEN** a target does not exist
- **THEN** the command fails before launching a container and identifies valid input

#### Scenario: Partial matrix failure

- **WHEN** one selected case fails
- **THEN** remaining cases report their results and the aggregate command fails with visible case outcomes

### Requirement: Executed candidate identity

Acceptance SHALL validate and extract the selected archive in a fresh disposable directory and execute those extracted files. Validation SHALL enforce the runtime manifest contract, extraction size ceilings, and complete gzip integrity. Test helpers SHALL be built from the selected checkout into disposable storage.

#### Scenario: Replaced expanded executable

- **WHEN** an unrelated old staging executable or reusable helper changes
- **THEN** acceptance ignores those files and uses the selected archive and freshly built helpers

#### Scenario: Damaged compressed stream

- **WHEN** an archive payload has valid member hashes but a missing or corrupt gzip trailer
- **THEN** verification and runtime extraction reject it

### Requirement: Visible validation progress

Validation SHALL show concise case context, stage progress, and outcomes in console output. Matrix cases SHALL identify the target, image, architecture, and native or emulated execution. Failures SHALL remain understandable from console output without downloading artifacts. Output SHALL exclude credentials and inspected payloads. Reporting SHALL preserve failure and cancellation status.

#### Scenario: Failed matrix stage

- **WHEN** a matrix case fails during a named stage
- **THEN** console output shows the case and stage output followed by its failure status

## REMOVED Requirements

### Requirement: Verified matrix candidate

**Reason**: Source fingerprints and independently mutable expanded candidates are unnecessary when acceptance explicitly tests selected archive bytes.
**Migration**: Use the explicit archive contract below; run packaging separately when testing current source. CI still builds once and gates delivery on every required case.

## ADDED Requirements

### Requirement: Explicit archive acceptance

Local acceptance SHALL identify the release version and archive directory it tests. It SHALL NOT rebuild release binaries or claim an existing archive matches current source. CI SHALL produce one candidate, share its archive bytes across acceptance jobs, and publish those same bytes only after every required check succeeds.

#### Scenario: Test a selected release

- **WHEN** the developer selects an existing archive version
- **THEN** acceptance tests that archive independently of unrelated working-tree changes

#### Scenario: Test current source

- **WHEN** the developer needs acceptance for current source
- **THEN** the documented workflow explicitly packages that source before running acceptance

#### Scenario: Failed hosted case

- **WHEN** any required case fails or is skipped
- **THEN** the final CI gate fails and release delivery cannot run
