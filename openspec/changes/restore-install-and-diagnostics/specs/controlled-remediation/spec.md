# Spec Delta

## ADDED Requirements

### Requirement: Payload-free repair audit

The repair boundary SHALL record a structured event for each attempted repair with a request identifier, credential identifier when known, decision, outcome, and time. It SHALL include the named operation and target only after credential, role, and profile authorization succeeds. It SHALL record denied and failed attempts as well as completed attempts without storing bearer secrets, diagnostic payloads, journal content, or untrusted response bodies. Failure to write an event SHALL be reported and SHALL not falsely imply the repair was audited.

#### Scenario: Successful restart

- **WHEN** an authorized restart completes
- **THEN** the repair worker emits an event naming the operation, target, credential ID, and completed outcome

#### Scenario: Denied restart

- **WHEN** a repair request fails role or profile authorization
- **THEN** an event records the denial without disclosing the bearer secret or unapproved operation and target
