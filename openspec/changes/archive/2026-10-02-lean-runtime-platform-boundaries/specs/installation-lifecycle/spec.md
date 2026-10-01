## ADDED Requirements

### Requirement: Bounded installation state decoding

Before any lifecycle mutation, HostLens SHALL read the installation manifest as exactly one JSON document within a 16 MiB byte ceiling. It SHALL reject malformed, trailing, or oversized state instead of acting on a partial manifest.

#### Scenario: Oversized or trailing manifest

- **WHEN** the installation manifest exceeds 16 MiB or contains a second JSON document
- **THEN** lifecycle commands reject it before changing installed resources
