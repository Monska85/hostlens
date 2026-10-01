# Spec Delta

## ADDED Requirements

### Requirement: Positive backend activation acknowledgement

The gateway SHALL treat backend preparation and activation as successful only when each response is a bounded, single JSON object that positively acknowledges the requested step. An absent, false, null, malformed, or unrelated acknowledgement SHALL fail the reload or recovery attempt without claiming that the new generation is active.

#### Scenario: Backend declines preparation

- **WHEN** the backend returns HTTP 200 with `prepared: false`, a missing `prepared` member, or a null body
- **THEN** the gateway rejects the candidate and retains its previous active generation

#### Scenario: Backend declines activation

- **WHEN** the backend returns HTTP 200 with `active: false` or a missing `active` member
- **THEN** the gateway reports reload failure and does not activate the candidate locally

#### Scenario: Backend confirms both steps

- **WHEN** the backend returns positive preparation and activation acknowledgements for the requested generation
- **THEN** the gateway activates that generation after both replies are validated

#### Scenario: Matching generation with conflicting fingerprint

- **WHEN** backend status reports the expected generation with a different policy fingerprint
- **THEN** the gateway rejects synchronization without admitting diagnostic work or claiming a consistent active state

### Requirement: Observer configuration trust and bounds

The Docker observer SHALL load its system configuration under the same trusted ownership, path, and document-size limits as the diagnostic backend. It SHALL reject an unsafe or oversized configuration before opening its observer listener or connecting to Docker.

#### Scenario: Oversized observer configuration

- **WHEN** the observer starts with a configuration document exceeding the configured document ceiling
- **THEN** startup fails without opening an observer listener or connecting to Docker

#### Scenario: Linked observer configuration

- **WHEN** the observer configuration path is a symbolic link
- **THEN** startup rejects the untrusted source before obtaining Docker access
