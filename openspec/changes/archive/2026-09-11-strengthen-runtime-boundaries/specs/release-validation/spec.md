## MODIFIED Requirements

### Requirement: Documentation and specification gate

CI SHALL gate source checks, builds, dependency scanning, and container tests. Agents SHALL validate current and archived project OpenSpec artifacts through an externally installed CLI before completing changes. OpenSpec and harness-owned document formatters SHALL NOT be repository dependencies or required CI checks.

#### Scenario: Invalid requirement

- **WHEN** an invalid requirement or scenario enters the repository
- **THEN** agent validation fails and the change remains incomplete until corrected

### Requirement: Consistent developer commands

Make and Just SHALL expose equivalent setup, formatting, linting, testing, and combined-check commands. Repository formatting SHALL cover maintained Go, Python, and POSIX shell. Repository-managed development tools SHALL use versioned manifests and integrity locks, install locally, and remain separate from runtime dependencies. OpenSpec and document formatting SHALL be supplied by the development environment or agent harness, without repository npm manifests or local copies. Checks SHALL NOT install missing tools or container images implicitly.

#### Scenario: Fresh contributor setup

- **WHEN** a contributor runs the dependency setup with the documented base tools
- **THEN** the repository installs its locked development tools locally and provides the same validation commands used by CI

#### Scenario: Missing formatter

- **WHEN** a required formatter is unavailable
- **THEN** the format check fails with setup guidance rather than silently skipping a language

## ADDED Requirements

### Requirement: Native hosted platform validation

Hosted platform acceptance SHALL run Linux amd64 and arm64 candidates on runners and container images matching their native architectures. Platform preparation SHALL reject an engine architecture mismatch instead of silently using emulation. Local cross-architecture checks MAY retain explicit, labeled emulation.

#### Scenario: Hosted arm64 candidate

- **WHEN** CI validates the Linux arm64 candidate
- **THEN** the shared archive and helper execute in an arm64 container on an arm64 runner without QEMU

### Requirement: Visible validation progress

Validation SHALL report concise stage progress and final outcomes in console output. Matrix cases SHALL identify the target, image, architecture, and native or emulated execution. Detailed case logs SHALL supplement this progress without requiring artifact downloads to determine which stage ran or failed. Progress output SHALL NOT contain credentials or inspected payloads, and reporting SHALL preserve command failure and cancellation outcomes.

#### Scenario: Failed matrix stage

- **WHEN** a matrix case fails during a named stage
- **THEN** console output identifies the case, latest stage, failure status, and retained log location
