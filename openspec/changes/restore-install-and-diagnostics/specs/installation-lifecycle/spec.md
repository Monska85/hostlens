# Spec Delta

## ADDED Requirements

### Requirement: Previewable Linux installation

On Linux, `install` SHALL inspect an extracted, verified release directory and report the exact binary, configuration, profiles, identities, groups, and systemd units it would create or change. Preview SHALL make no mutation and SHALL reject conflicting existing objects. `install --apply` SHALL require root, preserve existing administrator configuration and credentials, install files with protected ownership and modes, and leave services stopped unless `--start` is explicit. It SHALL validate the resulting configuration before activation and restore files changed by a failed application.

#### Scenario: Fresh preview

- **WHEN** an administrator previews a valid extracted release on a fresh Linux host
- **THEN** HostLens reports the intended changes without creating identities, files, or services

#### Scenario: Apply and start

- **WHEN** root applies a valid fresh plan with `--start`
- **THEN** HostLens installs protected files and identities, validates configuration, and starts only the read-only gateway

#### Scenario: Conflict or failed activation

- **WHEN** a target is untrusted, a plan changed since preview, or service activation fails
- **THEN** HostLens refuses or restores its changed files and leaves the previous installation usable

### Requirement: Safe installed upgrade

`upgrade` SHALL preview the exact installed-file and unit changes, check release identity and configuration compatibility, and require root to apply. It SHALL preserve tokens, profiles, and administrator configuration unless an explicit migration candidate is selected. It SHALL retain a verified rollback copy until the replacement is validated and activated. A failed activation SHALL restore the previous binary and units and report the outcome.

#### Scenario: Compatible upgrade

- **WHEN** root applies a compatible release to a known HostLens installation
- **THEN** the installed binary and packaged units change, protected administrator data remains, and previously active services resume only after validation

#### Scenario: Failed replacement

- **WHEN** the new gateway fails to activate
- **THEN** the previous binary and units are restored and the failed upgrade is reported
