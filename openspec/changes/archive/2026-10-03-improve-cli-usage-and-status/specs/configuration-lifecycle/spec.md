# Spec Delta

## ADDED Requirements

### Requirement: Discoverable local CLI usage

The Linux CLI SHALL provide grouped top-level help and command-specific help for every supported command and subcommand. Help SHALL show only options that the selected command accepts, use the documented `--option` spelling, and explain required arguments and the configuration mode where relevant. `help COMMAND` and `COMMAND --help` SHALL produce guidance without loading configuration, requiring administrator identity, or changing host state. Unsupported options SHALL fail before configuration loading.

#### Scenario: Top-level help

- **WHEN** an operator invokes `hostlens --help`
- **THEN** HostLens lists commands with their purposes and tells the operator how to get command-specific help

#### Scenario: Command-specific help

- **WHEN** an operator invokes `hostlens help status` or `hostlens status --help`
- **THEN** both forms explain status usage and its accepted double-hyphen options without requiring a configuration file or admin socket

#### Scenario: Irrelevant option

- **WHEN** an operator supplies a token-creation option to `hostlens status`
- **THEN** HostLens rejects that option before attempting to load configuration or contact a service

### Requirement: Actionable status configuration and connection errors

Without explicit `--system` or `--config`, `status` SHALL select the system configuration for root and the user configuration for non-root callers. Explicit mode and path selections SHALL remain authoritative. A missing selected configuration SHALL identify its path and explain the system-installation command when relevant. An unavailable administrative socket SHALL identify the socket and how to check the service. These failures SHALL NOT expose status data or bypass the local administrator identity check.

#### Scenario: Root status on a system installation

- **WHEN** root invokes `hostlens status` with the system installation present
- **THEN** HostLens reads the system configuration and queries its administrative socket

#### Scenario: Missing user configuration

- **WHEN** a non-root operator invokes `hostlens status` without a user configuration
- **THEN** HostLens reports the missing user configuration and directs the operator to the root-authorized system status command

#### Scenario: Missing system socket

- **WHEN** a local administrator selects the system installation but its administration socket is unavailable
- **THEN** HostLens reports the socket path and a service-check action rather than only a filesystem error

#### Scenario: No privilege escalation

- **WHEN** a non-root operator explicitly selects system status
- **THEN** HostLens retains the existing local administrator identity check and does not disclose status
