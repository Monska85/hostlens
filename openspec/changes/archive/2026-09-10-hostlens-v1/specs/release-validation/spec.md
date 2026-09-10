# Release validation

## Purpose

Define the observable HostLens v1 release validation behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## ADDED Requirements

### Requirement: Representative compatibility evidence

Release validation SHALL cover Linux amd64 and arm64 and record representative Debian, Ubuntu, and Arch environments. Tests SHALL exercise capability detection rather than enforce a distribution release allowlist. Native execution evidence SHALL be distinguished from emulation; Arch Linux ARM SHALL not be conflated with official Arch Linux.

#### Scenario: New version

- **WHEN** a new distribution version preserves interfaces
- **THEN** HostLens needs no release solely to recognize its version

#### Scenario: Emulated coverage

- **WHEN** arm64 tests run under emulation
- **THEN** the report labels that evidence accurately

### Requirement: Container test execution

All tests SHALL run in disposable containers by default, including userspace detection, package parsing, policy, authentication, protocol, systemd, journal permissions, privilege containment, and lifecycle checks where supported. Reports SHALL state the shared kernel and namespace scope and identify checks that the container environment cannot establish. Privileged lifecycle tests SHALL NOT run on the administrator host.

A VM SHALL be used only when strictly necessary for a concrete acceptance check. The agent SHALL explain the necessity and obtain explicit user confirmation before provisioning, downloading, or starting the VM. Test tooling SHALL NOT provision VMs automatically.

#### Scenario: Container resource view

- **WHEN** tests run within namespaces or cgroups
- **THEN** results identify that scope and do not claim to describe or isolate the physical host

#### Scenario: Service validation limitation

- **WHEN** container tests pass but the container cannot exercise required systemd or privilege containment behavior
- **THEN** the report identifies the unverified behavior and does not declare it validated

#### Scenario: Strictly necessary VM

- **WHEN** a concrete acceptance check cannot be established in a disposable container and strictly requires a VM
- **THEN** the agent explains the limitation and waits for explicit user confirmation before any VM provisioning, image download, or startup

#### Scenario: Container default

- **WHEN** an implementation or validation task starts
- **THEN** its test commands use disposable containers and perform no privileged lifecycle changes on the administrator host

### Requirement: Security and failure acceptance

Before release, validation SHALL exercise denied-source access through every tool, symlink and path races, special files, includes and cycles, reload consistency, token updates and revocation, trusted-proxy spoofing, listener failures, quotas, and lifecycle rollback. Failed required checks SHALL block release claims.

#### Scenario: Boundary regression

- **WHEN** a denied source can be returned through a second tool
- **THEN** release acceptance fails

#### Scenario: No invented evidence

- **WHEN** a collector loses access to its source
- **THEN** tests require omitted measurements with explicit collection issues, not zeros or healthy output
