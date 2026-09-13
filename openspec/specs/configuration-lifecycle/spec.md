# Configuration lifecycle

## Purpose

Define the observable HostLens v1 configuration lifecycle behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Validated YAML configuration

HostLens SHALL use YAML and reject malformed, unknown, incompatible, or unsafe active settings with actionable source locations. The effective configuration SHALL define profiles and direct source policies, listeners, limits, health checks, logging, and privilege mode. Roles SHALL be fixed in v1 and membership SHALL reside in token metadata. Secret records SHALL remain in a separate protected store.

#### Scenario: Invalid startup

- **WHEN** a configured bind or profile is invalid
- **THEN** startup fails before accepting MCP requests

#### Scenario: Remediation setting

- **WHEN** v1 configuration attempts to enable remediation
- **THEN** validation explains that remediation is unavailable

### Requirement: Atomic reload

Linux SIGHUP SHALL invoke a platform-independent reload operation. Reloadable policy, limits, and assessment settings SHALL be fully resolved and validated before coordinated activation across gateway and backend. Failure SHALL preserve the previous active generation. Listener, TLS, identity, process privilege, and connection idle-timeout changes SHALL require restart in v1. Other request limits SHALL apply to newly admitted requests after reload, including transport read and write deadlines.

#### Scenario: Valid reload

- **WHEN** all reloadable changes pass validation
- **THEN** new operations use one new consistent generation

#### Scenario: Failed reload

- **WHEN** a newly included profile is missing
- **THEN** the old generation remains active and failure is logged

#### Scenario: In-flight read

- **WHEN** a read already started when policy changes
- **THEN** it can finish under its original generation; new operations use the new policy

#### Scenario: Restart-only change

- **WHEN** SIGHUP encounters a changed TLS key or listener
- **THEN** reload rejects the mixed candidate with restart guidance rather than partially applying it

### Requirement: Policy fingerprint comparison

A local administrative interface SHALL expose active effective-policy fingerprint, instance identity, and generation without exposing secrets. policy explain SHALL calculate the same fingerprint from resolved on-disk policy including evaluator version and mandatory exclusions. Formatting, comments, and irrelevant ordering SHALL NOT affect the fingerprint.

#### Scenario: Match

- **WHEN** disk and the queried instance have equivalent policies
- **THEN** the report prints MATCH and identifies the evaluated configuration and instance

#### Scenario: Difference

- **WHEN** disk policy differs from active policy
- **THEN** the report prints DIFFERENT and states it evaluates disk policy

#### Scenario: Service unavailable

- **WHEN** no authorized live comparison is possible
- **THEN** the report prints UNKNOWN and states it evaluates disk policy

#### Scenario: Scope of match

- **WHEN** policy matches but a certificate differs or OS permissions block access
- **THEN** the report does not claim full configuration equality or successful file access

### Requirement: Policy explanation

The local policy explain command SHALL report decisions for a file or directory, matched allows and denies, source files, profile names, inclusion chains, mandatory exclusions, and resolved paths. Installed inactive matches SHALL be distinguishable and SHALL NOT affect the decision. Denied and unevaluable results SHALL be distinct.

#### Scenario: Conflicting rules

- **WHEN** nginx permits a file but a direct deny excludes it
- **THEN** the report names both matches and explains the denial

#### Scenario: Directory scope

- **WHEN** a directory is evaluated without recursion
- **THEN** the report does not assert every descendant is allowed

#### Scenario: Optional recursion

- **WHEN** recursive explanation is requested
- **THEN** existing descendants are evaluated under configurable count and time limits with explicit truncation

### Requirement: Recovery and consistent status

A restarted component SHALL NOT admit diagnostic work until gateway and backend agree on a validated generation. A full service restart SHALL load and validate disk configuration. After an isolated backend restart, the gateway SHALL synchronize its active generation before admitting work; unresolvable mismatch SHALL fail closed. Administrative status SHALL read generation and fingerprint as one consistent snapshot.

#### Scenario: Backend restarts after rejected reload

- **WHEN** disk contains a rejected candidate and the backend restarts while the gateway remains active
- **THEN** work remains blocked until the gateway's active validated generation is restored consistently

#### Scenario: Whole service restart

- **WHEN** both components restart with invalid disk configuration
- **THEN** startup fails rather than silently restoring a different configuration

### Requirement: Bounded configuration input

Configuration and profile loading SHALL reject non-regular files and read no more than the 1 MiB document ceiling plus one overflow byte. Each profile directory SHALL be rejected after observing more than 1,024 entries without enumerating the remaining entries. A complete load SHALL reject more than 8 MiB of combined configuration/profile input, 1,024 profile definitions, or 1,024 configured profile directories before retaining an unbounded candidate.

#### Scenario: Oversized document

- **WHEN** a configuration or profile exceeds 1 MiB
- **THEN** loading stops at the ceiling and rejects the candidate without reading its full contents

#### Scenario: Oversized profile directory

- **WHEN** a profile directory contains more than 1,024 entries
- **THEN** loading rejects the candidate after a bounded enumeration

#### Scenario: Aggregate candidate overflow

- **WHEN** individually valid profile files exceed the combined input or definition ceiling across directories
- **THEN** loading rejects the candidate before further decoding and the previous generation remains active

### Requirement: Bounded explanation traversal

Recursive policy explanation SHALL check count and deadline limits before enumerating descendants, use bounded directory batches, and bound recorded traversal failures. Hitting any limit SHALL produce explicit truncation.

#### Scenario: Count exhausted at root

- **WHEN** the root consumes the configured entry ceiling
- **THEN** the command reports truncation without enumerating its descendants

### Requirement: Bounded backend configuration preparation

Backend generation preparation SHALL admit at most one underlying configuration load at a time. A cancelled or timed-out request SHALL stop waiting without releasing admission until its load returns. Abandoned, invalid, or mismatched candidates SHALL NOT become pending state. Preparation SHALL NOT hold shared state locks during source reads or prevent unrelated state access.

#### Scenario: Cancelled preparation with stalled source

- **WHEN** a preparation request is cancelled while a configuration read remains blocked
- **THEN** subsequent preparation requests fail promptly without starting additional loads, and the abandoned candidate is not staged when the read returns

#### Scenario: Preparation recovers

- **WHEN** the previous underlying load has returned
- **THEN** a new valid preparation can complete and activate through the existing generation checks

### Requirement: Global MCP read-only configuration

The effective configuration SHALL include the boolean `mcp.read_only` setting and SHALL treat an omitted value as `true`. The setting SHALL apply only to MCP tool availability and execution. Setting it to `false` SHALL make remediation tools eligible for later authorization checks but SHALL NOT create, enable, or authorize remediation. V1 SHALL continue to reject `remediation.enabled: true` and expose diagnostic tools only. The effective-policy fingerprint and administrative status SHALL include the active read-only value without exposing secrets.

#### Scenario: Setting omitted

- **WHEN** an otherwise valid configuration omits `mcp.read_only`
- **THEN** startup activates MCP read-only mode

#### Scenario: Read-only disabled in v1

- **WHEN** an otherwise valid v1 configuration sets `mcp.read_only: false`
- **THEN** startup succeeds but exposes no remediation tool because remediation remains unavailable

#### Scenario: Explicit remediation enablement

- **WHEN** a configuration sets `mcp.read_only: false` and `remediation.enabled: true` in v1
- **THEN** validation rejects the unavailable remediation setting

#### Scenario: Administrative status

- **WHEN** a local administrator reads active status
- **THEN** the response reports the active MCP read-only value and a fingerprint calculated from that value

### Requirement: Atomic activation of MCP read-only mode

`mcp.read_only` SHALL be reloadable through the coordinated configuration generation. A transition to `true` SHALL stop admission of new remediation operations before the new generation becomes active and SHALL NOT report success while an admitted remediation operation can still mutate the host. If admitted remediation work cannot finish or be cancelled within the reload bound, activation SHALL fail and restore the previous generation's admission behavior. Diagnostic reads admitted under the previous generation MAY finish without delaying activation.

#### Scenario: Enable read-only mode

- **WHEN** a valid reload changes `mcp.read_only` from `false` to `true` and admitted remediation work has drained
- **THEN** the new generation activates with no remediation operation admitted or running

#### Scenario: Remediation does not drain

- **WHEN** a transition to read-only mode cannot stop admitted remediation work within the reload bound
- **THEN** reload fails, reports the reason, and restores the previous active generation and admission behavior

#### Scenario: Concurrent diagnostic read

- **WHEN** a diagnostic read is running during a valid transition to read-only mode
- **THEN** that read can finish under its original generation while new MCP requests use the read-only generation

### Requirement: Validated Docker observer configuration

Configuration SHALL provide an opt-in Docker diagnostics section that defaults to disabled and selects only an absolute local Unix socket for a system-wide engine. It SHALL identify the expected existing socket-owning access group, defaulting to the standard `docker` group, so reconciliation can prepare an enabled-waiting installation before Docker exists. Enabling Docker SHALL require an observer IPC path distinct from existing gateway administration and diagnostic IPC. Startup SHALL validate the group name, path type, ownership and write boundaries, unsupported transport schemes, IPC collisions, and settings compatible with the selected installation mode before accepting MCP traffic.

#### Scenario: Docker omitted

- **WHEN** an existing configuration has no Docker section
- **THEN** HostLens starts without Docker authority or Docker tools

#### Scenario: Remote endpoint

- **WHEN** Docker configuration selects TCP, SSH, TLS, or another remote endpoint
- **THEN** validation rejects the unsupported transport

#### Scenario: IPC collision

- **WHEN** the Docker observer path equals the administration or diagnostic socket path
- **THEN** validation rejects the configuration before any component starts

#### Scenario: Nonstandard Docker group

- **WHEN** the administrator configures a valid existing group for a Docker socket that is not owned by the standard `docker` group
- **THEN** reconciliation uses that group only for the observer service's process-scoped `SupplementaryGroups=` setting and records it in the plan

### Requirement: Restart-only Docker topology

Enabling or disabling the Docker observer and changing its daemon or IPC socket SHALL require a coordinated service restart because these settings change process authority and topology. Reload SHALL reject such changes while preserving the active generation. Docker policy and shared request ceilings SHALL remain reloadable for newly admitted requests.

#### Scenario: Observer enabled on reload

- **WHEN** SIGHUP encounters a change from disabled to enabled Docker diagnostics
- **THEN** reload rejects the candidate with restart guidance and preserves the current surface

#### Scenario: Docker policy reload

- **WHEN** a valid reload removes access to a Docker resource without changing observer topology
- **THEN** new requests enforce the new denial while an admitted read may finish under its original generation

### Requirement: Docker desired state and reconciliation

Docker enablement SHALL represent administrator intent separately from current daemon availability. `hostlens reconcile --system` SHALL calculate and display a complete plan without mutation. `hostlens reconcile --system --apply` SHALL transactionally move the optional observer topology between disabled, waiting, provisioned, available, and unavailable states using existing lifecycle ownership, conflict, rollback, and interruption protections. Reconciliation SHALL NOT install, configure, start, stop, or restart Docker, and SHALL NOT grant Docker access to the gateway or diagnostic backend.

#### Scenario: Docker installed after HostLens

- **WHEN** Docker diagnostics are enabled and reconciled while the supported daemon is absent and an administrator later installs and starts Docker with the configured socket and group
- **THEN** the next Docker diagnostic request activates the observer and makes Docker diagnostics available without reinstalling or reconciling HostLens again

#### Scenario: Docker remains absent

- **WHEN** reconciliation finds no supported local daemon or cannot establish the required isolated access
- **THEN** Docker remains in an explicit waiting or unavailable state and non-Docker HostLens services remain operational

#### Scenario: Docker disappears

- **WHEN** a provisioned Docker daemon stops or its socket disappears
- **THEN** the observer stops, Docker diagnostics become unavailable without clearing configured intent, and HostLens does not restart Docker

#### Scenario: Nonstandard Docker access

- **WHEN** the daemon uses a socket or owning group that differs from the reconciled configuration
- **THEN** the observer refuses access with actionable status and the administrator must update configuration and rerun reconciliation

#### Scenario: Docker integration disabled

- **WHEN** an administrator applies a reconciliation plan that disables Docker diagnostics
- **THEN** HostLens stops and disables the observer service and socket unit, exposes no Docker tools, and leaves the dormant account without Docker group authority
