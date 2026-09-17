## MODIFIED Requirements

### Requirement: Explicit audit authority

New audit tools SHALL require the diagnostics role and an explicit active grant for their audit domain, except get_hostlens_info, which SHALL require the health role and an explicit active grant for the `hostlens` audit domain because it reports HostLens's own configuration rather than host evidence. Existing file and journal denials and mandatory exclusions SHALL remain effective. The existing OS privilege model SHALL remain unchanged.

#### Scenario: Existing broad file grant

- **WHEN** a diagnostics client has broad file access but no audit grant
- **THEN** new audit collection is denied without executing its collector

#### Scenario: Denial overrides grant

- **WHEN** an included profile grants an audit domain and another active rule denies it
- **THEN** collection is denied, including direct calls to a cached tool

#### Scenario: Health client without grant

- **WHEN** a health-only client calls get_hostlens_info and no active rule grants the `hostlens` audit domain
- **THEN** the call is denied without executing its collector, exactly as for a diagnostics client
