# delivery-migration Specification

## Purpose

Ship verifiable Linux amd64 and arm64 artifacts and a safe path from installed 0.5.0 systems to the breaking HostLens contract.

## Requirements

### Requirement: Checked migration

The migration command SHALL inspect the existing configuration and credentials, preserve bearer credentials when safely convertible, and report old profile names whose grants require manual review. It SHALL create a separate candidate without changing the installed release or starting services. Unsupported source data SHALL fail before the candidate directory is published. Operators SHALL validate and activate the candidate only after preserving the previous binary and state for rollback.
The profile report SHALL include active profiles reached through legacy includes, and a missing or invalid profile SHALL prevent candidate publication.

#### Scenario: Compatible existing installation

- **WHEN** an administrator prepares a supported 0.5.0 installation for upgrade
- **THEN** HostLens writes a separate version 2 candidate with preserved bearer hashes and observe-only roles, leaving the running installation untouched

#### Scenario: Unmappable policy

- **WHEN** an old profile cannot be represented without broadening access
- **THEN** the migration report names that profile, grants no replacement access, and leaves the installed system untouched

### Requirement: Isolated acceptance

All automated install, rollback, systemd, and Docker fixture tests SHALL run inside disposable containers. Fixture tests SHALL use a private nested Docker Engine rather than the administrator's Docker socket. The build SHALL produce verifiable Linux amd64 and arm64 archives, and unavailable native platform evidence SHALL be reported honestly.

#### Scenario: Systemd acceptance

- **WHEN** an install or upgrade acceptance case runs
- **THEN** it modifies only the disposable container's services and filesystem

#### Scenario: Private Docker unavailable

- **WHEN** the private fixture Engine cannot start
- **THEN** the test fails without falling back to the administrator's Engine

### Requirement: Restored lifecycle acceptance

Release validation SHALL exercise the packaged binary's fresh install preview and apply, compatible upgrade, failed upgrade restoration, and uninstall in disposable systemd containers on amd64 and native arm64. It SHALL verify packaged documentation, tool schemas, and checksum coherence for the released candidate.

#### Scenario: Failed upgrade rehearsal

- **WHEN** the candidate fails validation in a disposable systemd host
- **THEN** acceptance confirms the previous binary and units remain usable and does not touch the administrator's host
