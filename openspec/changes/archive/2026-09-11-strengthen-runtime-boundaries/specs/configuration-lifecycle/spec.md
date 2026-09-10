## ADDED Requirements

### Requirement: Bounded backend configuration preparation

Backend generation preparation SHALL admit at most one underlying configuration load at a time. A cancelled or timed-out request SHALL stop waiting without releasing admission until its load returns. Abandoned, invalid, or mismatched candidates SHALL NOT become pending state. Preparation SHALL NOT hold shared state locks during source reads or prevent unrelated state access.

#### Scenario: Cancelled preparation with stalled source

- **WHEN** a preparation request is cancelled while a configuration read remains blocked
- **THEN** subsequent preparation requests fail promptly without starting additional loads, and the abandoned candidate is not staged when the read returns

#### Scenario: Preparation recovers

- **WHEN** the previous underlying load has returned
- **THEN** a new valid preparation can complete and activate through the existing generation checks
