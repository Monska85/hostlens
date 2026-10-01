# Spec Delta

## ADDED Requirements

### Requirement: Observer resource ownership preflight

Before planning or applying an enabled Docker observer reconciliation, HostLens SHALL verify ownership of the observer binary, service units, and IPC socket. An existing resource without a matching owned manifest record, or an owned file replaced by a symlink or other unexpected type, SHALL cause a conflict without changing identities, files, or services. A missing observer binary that requires an extracted release source SHALL be rejected before mutation when no source is supplied.

#### Scenario: Unrecorded observer file

- **WHEN** a binary or service unit already exists but its ownership is absent from the installation manifest
- **THEN** both dry run and apply report a conflict and preserve the resource without granting Docker access

#### Scenario: Unrecorded observer socket

- **WHEN** the configured observer IPC path exists without a matching owned manifest record
- **THEN** both dry run and apply reject reconciliation before service or identity changes

#### Scenario: Unexpected type at an owned path

- **WHEN** an owned observer binary or unit is replaced by a symlink or directory
- **THEN** reconciliation reports the unexpected resource and preserves it

#### Scenario: Older installation lacks observer binary

- **WHEN** the observer binary is missing and the manifest has no completed observer binary record, while no extracted release source is supplied
- **THEN** apply fails before creating the observer identity or units and explains the required source

#### Scenario: Unusable observer binary source

- **WHEN** a missing observer binary requires a source that is absent, non-regular, empty, or larger than the release member ceiling
- **THEN** apply fails before creating the observer identity or units

#### Scenario: Interrupted owned write

- **WHEN** an interrupted observer binary or unit write left an owned intent record and matching recorded or generated content
- **THEN** reconciliation can finish that resource without adopting different content

#### Scenario: Owned observer resource

- **WHEN** the observer binary, units, and socket have matching owned records and expected resource types
- **THEN** reconciliation may update or remove those resources under the existing lifecycle rules

#### Scenario: Repeated custom observer socket moves

- **WHEN** an owned observer IPC socket moves between custom path names
- **THEN** reconciliation removes and records the old socket path before enabling the new one
