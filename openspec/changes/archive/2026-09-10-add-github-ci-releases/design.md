## Context

See the proposal for motivation. Existing archives contain both executables, `release.json`, systemd units, configuration, documentation, and upstream licenses. Upgrade validates the manifest and every member. There is no remote or commit history yet.

## Goals / Non-Goals

Deliver tested Linux archives through GitHub Releases while preserving local container validation and independent task runners. Server rollout, native packages, container publication, and production certification are excluded.

## Decisions

- **Build once.** A reusable validation workflow builds the candidate and uploads a tar bundle preserving executable modes. The release job downloads that exact run's bundle and verifies its version and checksums before publishing.
- **GoReleaser OSS.** Use its supported extra release files mechanism with builds disabled. Keep the existing reproducible packager because GoReleaser's prebuilt importer and per-archive hooks require Pro. This avoids duplicate archive implementations.
- **Trust boundaries.** Pull requests receive read-only permissions. Only a tag-triggered release job receives release write permission. Pin external actions to commits, disable persisted checkout credentials, and require tagged commits to belong to the repository's default branch.
- **Draft delivery.** Create drafts, including prerelease classification. The maintainer reviews documented validation limits before making a release public. No SSH credentials or server targets are needed.
- **Container execution.** Run acceptance tests in disposable Docker containers on GitHub-hosted Linux runners. The runner supplies the kernel; these checks do not establish physical host or native arm64 behavior. No workflow provisions a test VM.

## Risks / Trade-offs

- **Hosted systemd support.** Private writable cgroups and per-container capabilities must work on the hosted runner. A failure blocks the candidate; no skipped success or VM fallback is allowed.
- **No hosted evidence yet.** Local validation cannot establish GitHub permissions, runner behavior, or delivery. Record those checks as pending until the owner publishes the repository and runs the workflows.
- **Unsigned distribution.** Checksums detect corruption but do not independently authenticate the producer. Signing and provenance remain future work and must not be implied by a successful draft upload.

## Migration Plan

The owner can publish the repository, enable Actions, protect the default branch and version tags, and push a version tag after reviewing CI. Failed drafts can be corrected before publication. Host upgrades remain explicit operator actions.
