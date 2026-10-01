# Spec Delta

## ADDED Requirements

### Requirement: Bounded Docker failure detail

Docker observation failures SHALL identify the failed boundary and error class without forwarding a Docker Engine or observer HTTP response body to the diagnostic backend or MCP client. Typed issue codes and HTTP status may be retained as payload-free failure metadata.

#### Scenario: Daemon returns sensitive error text

- **WHEN** Docker rejects an observation and its response body contains arbitrary text
- **THEN** HostLens reports the refusal class without returning that text

#### Scenario: Observer returns sensitive error text

- **WHEN** the observer IPC returns a non-success HTTP response containing arbitrary text
- **THEN** the diagnostic backend reports observer refusal without returning that body

### Requirement: Complete reference basis for Docker conclusions

Container selector resolution, derived reference counts, and unused or reclaimable classifications SHALL require a complete bounded container inventory. A truncated inventory SHALL be reported as an inventory ceiling. Correlated resource observations MAY still return directly observed fields, but SHALL omit derived reference counts and unused or reclaimable claims.

#### Scenario: Selector inventory is truncated

- **WHEN** the container inventory is truncated before a detail, stats, or logs observation or during identity recheck
- **THEN** the tool fails closed without releasing that observation as a resolved container

#### Scenario: Reference inventory is truncated

- **WHEN** a container inventory used to correlate images, volumes, networks, or disk usage is truncated
- **THEN** the result reports the ceiling and does not derive reference counts, unused state, or reclaimable candidates from incomplete references

### Requirement: Post-observation container authorization

For container detail, stats, and logs, the backend SHALL recheck the resolved identity and every current name against the applicable Docker policy after observation and before releasing evidence. This applies to both name and stable-ID selectors.

#### Scenario: A denied alias appears during collection

- **WHEN** a container gains a name denied by policy after the initial authorization but before its detail, stats, or logs result is returned
- **THEN** the backend reports `policy_denied` and does not release the collected evidence
