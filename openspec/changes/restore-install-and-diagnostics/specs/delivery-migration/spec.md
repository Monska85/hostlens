# Spec Delta

## ADDED Requirements

### Requirement: Restored lifecycle acceptance

Release validation SHALL exercise the packaged binary's fresh install preview and apply, compatible upgrade, failed upgrade restoration, and uninstall in disposable systemd containers on amd64 and native arm64. It SHALL verify packaged documentation, tool schemas, and checksum coherence for the released candidate.

#### Scenario: Failed upgrade rehearsal

- **WHEN** the candidate fails validation in a disposable systemd host
- **THEN** acceptance confirms the previous binary and units remain usable and does not touch the administrator's host
