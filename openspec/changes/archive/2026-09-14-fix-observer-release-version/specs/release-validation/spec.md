## MODIFIED Requirements

### Requirement: Versioned release delivery

A valid `vMAJOR.MINOR.PATCH` tag, optionally carrying a SemVer prerelease identifier, on a commit in the default branch history SHALL trigger validation and draft GitHub Release delivery. Delivery SHALL use the same archives produced and tested by that workflow run. Every shipped executable (gateway, diagnostics backend, and Docker observer) SHALL report the archive version, and binary version, archive filename, and upgrade manifest version SHALL agree. Archives SHALL preserve the existing upgrade contract, licenses, and checksums. Host rollout SHALL remain an explicit administrator action.

#### Scenario: Valid candidate

- **WHEN** a valid tag passes all required validation
- **THEN** its tested amd64 and arm64 archives and checksum files are attached to a draft release

#### Scenario: Observer version coherence

- **WHEN** the acceptance suite checks installed archive binaries
- **THEN** `hostlens-docker-observer version` reports the candidate version exactly like the gateway and diagnostics binaries

#### Scenario: Invalid tag or archive

- **WHEN** the tag is malformed, outside default branch history, or the archive version or checksums disagree
- **THEN** delivery fails before creating a release

#### Scenario: Local configuration only

- **WHEN** configuration has only been validated locally
- **THEN** documentation distinguishes those results from an actual hosted workflow or release delivery