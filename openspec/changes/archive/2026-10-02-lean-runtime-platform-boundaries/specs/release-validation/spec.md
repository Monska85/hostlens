## ADDED Requirements

### Requirement: Bounded release manifest decoding

Archive extraction SHALL accept `release.json` only as one complete JSON document within a 64 KiB byte ceiling before trusting its member checksums or executable names.

#### Scenario: Oversized or trailing release manifest

- **WHEN** `release.json` exceeds 64 KiB or contains a second JSON document
- **THEN** archive extraction fails without accepting the candidate release
