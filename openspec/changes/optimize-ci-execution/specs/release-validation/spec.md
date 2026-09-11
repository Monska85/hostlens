## ADDED Requirements

### Requirement: Scoped CI supersession

Ordinary branch and pull-request CI SHALL cancel obsolete runs superseded within the same workflow and branch or pull request. Unrelated work and release validation SHALL remain unaffected, including validation invoked through a reusable workflow. Cancellation SHALL NOT be reported as successful validation or permit release delivery.

#### Scenario: Updated candidate

- **WHEN** a newer update starts ordinary CI for the same branch or pull request
- **THEN** the obsolete run is cancelled and the new run executes the required checks

#### Scenario: Independent release

- **WHEN** ordinary CI is superseded while tag-triggered release validation is running
- **THEN** release validation and its caller are not cancelled by the ordinary CI concurrency group

#### Scenario: Interrupted acceptance

- **WHEN** a running container case is cancelled
- **THEN** cleanup terminates its test resources and cancellation remains visible rather than becoming a successful gate result

### Requirement: Safe optional validation caches

Validation MAY reuse compatible image layers and compiler outputs but SHALL execute every required test for the current candidate. Cached outputs SHALL NOT substitute for source, credentials, test verdicts or selected release archives. Untrusted contributions SHALL NOT gain authority to populate caches consumed as trusted release inputs. Unavailable or incompatible caches SHALL allow ordinary uncached validation without weakening checks.

#### Scenario: Warm cache with failing test

- **WHEN** compatible cached compilation exists and a current required test fails
- **THEN** the test still executes and the CI gate fails

#### Scenario: Cache unavailable

- **WHEN** cache restoration is disabled, misses or cannot access its service
- **THEN** validation performs normal preparation and all required checks, reporting actual failures

#### Scenario: Incompatible or untrusted cache

- **WHEN** a cache belongs to an incompatible platform/toolchain or a less-trusted contributor context
- **THEN** it is not restored into a trusted validation context through a permissive fallback

#### Scenario: Local uncached execution

- **WHEN** a developer uses prepared prerequisites without hosted-cache credentials
- **THEN** equivalent Make and Just checks remain available with the existing isolation and resource limits

### Requirement: Measured CI optimization

CI optimization evidence SHALL compare equivalent validation scope and identify source revision, runner environment, cache state and sample count. Reports SHALL distinguish end-to-end elapsed time, queue delay and total job execution minutes. Improvements SHALL NOT be claimed from skipped checks, cached test verdicts or measurements that do not establish the claimed reduction.

#### Scenario: Cold and warm comparison

- **WHEN** the optimized workflow is evaluated
- **THEN** cold and warm runs execute the maintained checks and matrix cases, and the report records their timing differences and comparison limitations

#### Scenario: Parallelism trade-off

- **WHEN** matrix parallelism changes
- **THEN** the report evaluates elapsed time and runner usage separately and records why the selected limit is retained

#### Scenario: No demonstrated gain

- **WHEN** an optimization has no demonstrated benefit or adds disproportionate machinery
- **THEN** it is removed or retained with an explicit non-performance justification, without an unsupported speed claim
