## ADDED Requirements

### Requirement: Automated candidate validation

Pull requests and branch updates SHALL run automated formatting, shell and workflow checks, dependency vulnerability scanning, Go race and vet checks, representative distribution smoke tests, and both supported systemd lifecycle modes. Executable tests SHALL use disposable containers. Required failures SHALL fail validation and block release delivery.

#### Scenario: Failed candidate

- **WHEN** a required check fails
- **THEN** the candidate is not delivered as a release

#### Scenario: Untrusted pull request

- **WHEN** checks execute pull request code
- **THEN** they receive no release write credentials or server credentials

### Requirement: Versioned release delivery

A valid `vMAJOR.MINOR.PATCH` tag, optionally carrying a SemVer prerelease identifier, on a commit in the default branch history SHALL trigger validation and draft GitHub Release delivery. Delivery SHALL use the same archives produced and tested by that workflow run. Binary version, archive filename, and upgrade manifest version SHALL agree. Archives SHALL preserve the existing upgrade contract, licenses, and checksums. Host rollout SHALL remain an explicit administrator action.

#### Scenario: Valid candidate

- **WHEN** a valid tag passes all required validation
- **THEN** its tested amd64 and arm64 archives and checksum files are attached to a draft release

#### Scenario: Invalid tag or archive

- **WHEN** the tag is malformed, outside default branch history, or the archive version or checksums disagree
- **THEN** delivery fails before creating a release

#### Scenario: Local configuration only

- **WHEN** configuration has only been validated locally
- **THEN** documentation distinguishes those results from an actual hosted workflow or release delivery
