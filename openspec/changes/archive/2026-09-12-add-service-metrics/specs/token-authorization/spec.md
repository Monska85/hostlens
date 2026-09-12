## MODIFIED Requirements

### Requirement: Additive fixed roles

V1 SHALL provide health, inspect, diagnostics, and metrics roles. health SHALL allow get_os_info, get_inventory, and get_health_snapshot. inspect alone SHALL include all health permissions plus list_services, get_service_status, and list_packages. diagnostics alone SHALL include all inspect permissions plus query_logs and read_config. Roles SHALL combine permissions and SHALL NOT imply future remediation access.

The metrics role SHALL grant only scraping of the metrics endpoint. It SHALL NOT grant MCP tool execution, host inspection, administration, or remediation. No other role SHALL implicitly grant metrics access. A combined role set SHALL grant the union of its explicitly assigned permissions. Existing token creation, role update, revocation, expiration, and rotation behavior SHALL apply to metrics tokens.

#### Scenario: Health scope

- **WHEN** a health token calls query_logs
- **THEN** authorization rejects it

#### Scenario: Diagnostics scope

- **WHEN** a diagnostics token reads a globally denied path
- **THEN** file policy denies the request despite tool permission

#### Scenario: Metrics isolation

- **WHEN** a token has only the metrics role
- **THEN** it can scrape an enabled authenticated metrics endpoint but cannot execute any MCP tool or administrative operation

#### Scenario: No implicit metrics inheritance

- **WHEN** a valid token has health, inspect, or diagnostics roles without metrics
- **THEN** the authenticated metrics endpoint rejects it with HTTP 403

#### Scenario: Combined roles

- **WHEN** a token has metrics and health roles
- **THEN** it can scrape metrics and execute health tools but cannot execute inspect or diagnostics tools

#### Scenario: Metrics authority removed

- **WHEN** a token is revoked, expires, loses its metrics role, or passes its rotation overlap deadline
- **THEN** the next authenticated scrape checks the current token state and rejects the request as applicable

#### Scenario: Extend an existing token

- **WHEN** an administrator updates an existing token role set to include metrics alongside its existing roles
- **THEN** its bearer secret remains unchanged and subsequent requests can scrape metrics while retaining the explicitly preserved diagnostic permissions
