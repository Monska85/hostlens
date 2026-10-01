# Spec Delta

## ADDED Requirements

### Requirement: Distinct local IPC paths

The diagnostic backend and local administrative interfaces SHALL use distinct filesystem socket paths in every valid Linux configuration, whether or not Docker diagnostics are enabled.

#### Scenario: IPC path collision without Docker

- **WHEN** Docker diagnostics are disabled and the configured diagnostic and administrative socket paths are identical
- **THEN** configuration validation rejects the candidate before either endpoint is bound

#### Scenario: Distinct IPC paths

- **WHEN** the diagnostic and administrative paths are distinct and otherwise valid
- **THEN** their separation does not prevent startup or reload
