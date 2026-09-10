# Platform runtime

## Purpose

Define the observable HostLens v1 platform runtime behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## ADDED Requirements

### Requirement: Supported runtime and capability discovery

HostLens SHALL run on Linux amd64 and arm64 and detect available collectors from system interfaces rather than reject unlisted distribution versions. Debian, Ubuntu, and Arch SHALL be representative test environments; systemd SHALL be the v1 installation integration.

#### Scenario: New distribution release

- **WHEN** the OS version is not in the recorded test matrix but required interfaces are available
- **THEN** HostLens runs the supported collectors without a distribution-version gate

#### Scenario: Missing interface

- **WHEN** systemd or a package manager interface is unavailable
- **THEN** the affected capability is reported unavailable without a fabricated substitute

### Requirement: Independent local components

The gateway and diagnostic backend SHALL run as separate identities and independently supervised processes. The gateway SHALL have no additional host-read capability. Backend IPC SHALL be local, access-controlled, structured, and restricted to supported diagnostic operations.

#### Scenario: Unauthorized local caller

- **WHEN** an unrelated local user attempts a backend connection
- **THEN** access is rejected

#### Scenario: Backend unavailable

- **WHEN** the backend cannot be reached
- **THEN** the gateway reports diagnostic unavailability rather than healthy results

### Requirement: Platform extension boundaries

Shared behavior SHALL remain independent of platform-specific collectors, filesystem semantics, service identities, IPC, and reload triggers. V1 SHALL expose no macOS or Windows implementation claims.

#### Scenario: Future platform

- **WHEN** a Windows collector is added later
- **THEN** it can use named pipes and native event sources without changing the meaning of Linux journal or file rules

### Requirement: Diagnostic-only release

V1 SHALL expose no remediation backend or remediation tools. Enabling remediation SHALL fail configuration validation. Known or cached remediation tool calls SHALL be rejected.

#### Scenario: Attempted enablement

- **WHEN** configuration enables remediation
- **THEN** startup or reload rejects that configuration and no remediation process starts

#### Scenario: Cached tool

- **WHEN** a client calls a remembered remediation tool name
- **THEN** the gateway rejects the operation

### Requirement: Client-independent host access

HostLens SHALL allow independently authorized compatible MCP clients to connect to the same host gateway. The host SHALL provide observed identity in diagnostic results; client-configured connection labels SHALL NOT substitute for the target's identity. Core access SHALL require neither a particular model provider nor Ansible provisioning.

#### Scenario: Interactive and scheduled clients

- **WHEN** an interactive agent and an externally scheduled application connect with different tokens
- **THEN** both use the same MCP endpoint under their individual roles and shared host policy

#### Scenario: Misnamed connection

- **WHEN** a client labels a connection with another server's name
- **THEN** HostLens results still identify the observed target rather than copying the client label
