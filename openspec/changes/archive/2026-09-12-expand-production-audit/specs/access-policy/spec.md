## MODIFIED Requirements

### Requirement: Typed profile structure

Each YAML profile SHALL support profiles, allow, and deny. Allow and deny SHALL support files, journal and audit categories in v1. Main configuration SHALL support direct typed rules and profile inclusions. Active unsupported categories such as future windows_events SHALL produce a clear validation error in v1.

#### Scenario: Complete application profile

- **WHEN** nginx includes web-common, allows configuration and logs, and denies private files
- **THEN** all included and direct typed rules contribute to the effective policy

#### Scenario: Future rule activated

- **WHEN** an active profile contains windows_events in Linux v1
- **THEN** configuration validation fails explicitly

#### Scenario: Audit domain grammar

- **WHEN** an audit rule contains a known audit domain or the wildcard `*`
- **THEN** it participates in include provenance, denial evaluation and the effective fingerprint; unknown domain names fail validation
