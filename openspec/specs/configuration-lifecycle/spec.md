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
