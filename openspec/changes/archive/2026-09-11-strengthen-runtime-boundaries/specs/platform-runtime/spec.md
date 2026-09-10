## ADDED Requirements

### Requirement: Bounded capability discovery

Capability discovery SHALL use a consistent active configuration snapshot without holding shared state locks during native observation. Stalled discovery SHALL retain a bounded amount of underlying work and SHALL NOT prevent unrelated admission decisions or validated state activation.

#### Scenario: Native discovery stalls

- **WHEN** a capability observation stalls in a native interface
- **THEN** further discovery cannot create unbounded abandoned work and state access remains responsive
