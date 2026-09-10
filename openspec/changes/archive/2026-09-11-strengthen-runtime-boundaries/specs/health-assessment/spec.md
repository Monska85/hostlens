## ADDED Requirements

### Requirement: Service health preserves observed failures

Health assessment SHALL evaluate all service observations collected within its inspection budget, independently of the public list page size. Partial service coverage SHALL retain observed failed units and their severity while marking coverage incomplete.

#### Scenario: More services than a public page

- **WHEN** service discovery returns more units than the configured list page and a unit is failed
- **THEN** health assessment includes the failed unit without treating pagination as missing inspection coverage

#### Scenario: Failure observed with partial coverage

- **WHEN** one unit is observed failed while another service observation is denied or unavailable
- **THEN** the failed-unit severity remains visible and coverage is incomplete
