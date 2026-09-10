# Candidate integrity

## ADDED Requirements

### Requirement: Fresh archive staging

Archive construction SHALL use only current declared source inputs and dependency notices. Files left in a previous staging tree SHALL NOT enter a newly built archive.

#### Scenario: Removed profile

- **WHEN** a profile is removed before rebuilding
- **THEN** the rebuilt archive excludes it even if an older staging directory contains it

### Requirement: Executed candidate identity

Matrix preflight SHALL verify that expanded release files match the verified archive and that test helpers match their build-recorded hashes. Archive verification SHALL enforce extraction size ceilings and complete gzip integrity before accepting a candidate.

#### Scenario: Replaced expanded executable

- **WHEN** an expanded executable or helper changes after building
- **THEN** preflight fails before running any container case

#### Scenario: Damaged compressed stream

- **WHEN** an archive payload has valid member hashes but a missing or corrupt gzip trailer
- **THEN** verification and runtime extraction reject it

### Requirement: Documentation and specification gate

CI SHALL validate current and archived OpenSpec artifacts and check Markdown formatting alongside source checks. Invalid specifications or documentation formatting SHALL fail the aggregate gate.

#### Scenario: Invalid requirement

- **WHEN** an invalid requirement or scenario enters the repository
- **THEN** automated validation fails before release delivery

### Requirement: Consistent developer commands

Make and Just SHALL expose equivalent setup, formatting, linting, testing, and combined-check commands. Formatting SHALL cover maintained Go, Python, POSIX shell, Markdown, JSON, and YAML. Development tools SHALL use versioned manifests and integrity locks, install locally, and remain separate from runtime dependencies. Checks SHALL NOT install missing tools or container images implicitly.

#### Scenario: Fresh contributor setup

- **WHEN** a contributor runs the dependency setup with the documented base tools
- **THEN** the repository installs its locked development tools locally and provides the same validation commands used by CI

#### Scenario: Missing formatter

- **WHEN** a required formatter is unavailable
- **THEN** the format check fails with setup guidance rather than silently skipping a language
