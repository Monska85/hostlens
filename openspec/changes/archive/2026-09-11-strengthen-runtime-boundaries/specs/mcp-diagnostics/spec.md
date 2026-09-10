## ADDED Requirements

### Requirement: Tool-specific argument contracts

Each MCP tool SHALL advertise only arguments that affect its behavior and reject unrelated arguments before collection. Source-reading tools SHALL require their documented source. Query logs SHALL require exactly one path or unit; list operations SHALL expose pagination without unrelated source filters.

#### Scenario: Observation without arguments

- **WHEN** a client discovers OS, inventory, or health tools
- **THEN** their schemas accept an empty object and reject file, journal, and pagination arguments

#### Scenario: Ambiguous log source

- **WHEN** a log request specifies both a path and a unit
- **THEN** schema validation rejects the request before collector execution

#### Scenario: Missing configuration source

- **WHEN** read_config lacks a nonempty path
- **THEN** the request fails validation before collector execution
