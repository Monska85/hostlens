# Access policy hardening

## ADDED Requirements

### Requirement: Alias-safe diagnostic files

General file readers and built-in file observations SHALL reject regular files with multiple hard links. Mandatory source protection SHALL remain effective when the original credential pathname is hidden by an OS secret mask.

#### Scenario: Masked secret alias

- **WHEN** a configured secret has a hard-link alias outside its masked directory
- **THEN** requesting that alias through configuration, log, or built-in file readers returns no secret bytes

#### Scenario: Ordinary hard-linked log

- **WHEN** an allowed log has multiple hard links
- **THEN** the reader rejects it explicitly rather than claiming successful collection
