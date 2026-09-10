# Bounded local configuration operations

## ADDED Requirements

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

## MODIFIED Requirements

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
