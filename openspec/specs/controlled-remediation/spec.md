# controlled-remediation Specification

## Purpose

Permit narrow, explicitly granted repair actions through a separate authority boundary without turning diagnostic access into arbitrary host mutation.

## Requirements

### Requirement: Separate repair authority

HostLens SHALL run repair operations through a distinct process identity and typed local contract. The gateway and observer SHALL possess no native mutation access or persistent repair credential. The gateway SHALL pass only the caller's request-scoped bearer credential over protected local IPC; the repair worker SHALL verify it afresh. The repair process SHALL accept only named operations, reauthorize the target, and revalidate live state immediately before acting.

#### Scenario: Diagnostic identity attempts repair

- **WHEN** the diagnostic worker sends a repair operation to its observation endpoint
- **THEN** the endpoint rejects it without invoking a native mutation API

#### Scenario: Token ID without bearer secret

- **WHEN** a local process knows a repair token ID but does not hold its bearer secret
- **THEN** the repair worker denies the request even if the process has the gateway UID

#### Scenario: Target changes before execution

- **WHEN** a selected service or container identity changes before repair
- **THEN** HostLens rejects the action or reauthorizes the new identity before mutation

### Requirement: Targeted restarts

The initial repair catalog SHALL contain restart of one approved native service and restart of one approved Docker container. HostLens SHALL reject arbitrary commands, file edits, Docker API paths, and unrestricted operations. Each result SHALL report whether the action ran and the observed post-action state without promising that the underlying problem was fixed.

The result SHALL distinguish a submitted restart, a completed restart, and an unavailable post-action observation. It SHALL omit post-action state when the read fails and identify that gap explicitly.
When a native mutation API response is lost, the result SHALL leave invocation and completion unknown rather than report either as false.

#### Scenario: Approved service restart

- **WHEN** read-only mode is disabled and a repair-role client requests restart of an explicitly granted service
- **THEN** the repair process checks current identity and state, requests one restart, and reports its outcome

#### Scenario: Unapproved container restart

- **WHEN** a client requests restart of a container without a matching repair grant
- **THEN** HostLens denies it before contacting Docker

#### Scenario: Post-restart read fails

- **WHEN** a restart completes but post-action inspection fails
- **THEN** the result reports invocation and completion, omits observed state, and identifies the observation failure

### Requirement: Payload-free repair audit

The repair boundary SHALL record a structured event for each attempted repair with a request identifier, credential identifier when known, decision, outcome, and time. It SHALL include the named operation and target only after credential, role, and profile authorization succeeds. It SHALL record denied and failed attempts as well as completed attempts without storing bearer secrets, diagnostic payloads, journal content, or untrusted response bodies. Failure to write an event SHALL be reported and SHALL not falsely imply the repair was audited.

#### Scenario: Successful restart

- **WHEN** an authorized restart completes
- **THEN** the repair worker emits an event naming the operation, target, credential ID, and completed outcome

#### Scenario: Denied restart

- **WHEN** a repair request fails role or profile authorization
- **THEN** an event records the denial without disclosing the bearer secret or unapproved operation and target
