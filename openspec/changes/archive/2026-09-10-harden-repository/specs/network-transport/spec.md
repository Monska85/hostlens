# Gateway admission

## ADDED Requirements

### Requirement: Bounded gateway processing

The gateway SHALL bound active MCP HTTP handlers before reading token state or constructing protocol handlers. Excess requests SHALL receive HTTP 503 without waiting for an admission slot. Accepted requests SHALL have a bounded context lifetime, including backend capability discovery.

#### Scenario: Authentication flood

- **WHEN** concurrent requests reach the configured operation ceiling
- **THEN** further requests receive an overload response without additional token-store reads or backend calls

#### Scenario: Backend stalls during discovery

- **WHEN** capability discovery does not complete before the request deadline
- **THEN** the request terminates and releases its gateway admission slot
