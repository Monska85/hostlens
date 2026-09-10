## Why

HostLens has local container acceptance tests and upgrade-compatible archives, but no automated checks or release delivery. GitHub Actions and GoReleaser will validate changes and deliver versioned archives through GitHub Releases.

## What Changes

- **CI.** Run static checks, vulnerability scanning, Go race tests, distribution smoke tests, and both systemd lifecycle modes in disposable containers.
- **CD.** Validate version tags, test the candidate, and use GoReleaser OSS to upload the same archives to a draft GitHub Release.
- **Packaging.** Keep binary versions, archive names, and upgrade manifests consistent while preserving licenses and checksums.
- **Developer commands.** Add independent Make and Just commands for release configuration and archive verification.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation`: Automated checks and gated delivery of tested, versioned release archives.

## Impact

Adds local GitHub workflow and GoReleaser configuration, release verification tooling, and release documentation. Changes the build version injection and archive packager. No host rollout, repository publication, commits, pushes, tags, or remote settings are performed by this implementation.
