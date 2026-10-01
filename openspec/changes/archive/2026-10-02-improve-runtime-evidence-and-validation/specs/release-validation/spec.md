## ADDED Requirements

### Requirement: Disposable live Docker acceptance

Release validation SHALL exercise the Docker observer and backend collector against a disposable local Docker daemon with fixture containers, images, volumes, and networks. The test daemon SHALL use separate temporary storage and a private socket, with no access to the administrator's Docker socket or workloads. An explicitly requested live acceptance run SHALL fail when its daemon, fixture image, or required namespace features are unavailable; it SHALL NOT silently skip or fall back to the host daemon. Reports SHALL identify the shared-kernel and privileged-container limits of this evidence.

#### Scenario: Live engine observation

- **WHEN** the disposable daemon and fixture image are available
- **THEN** validation executes the supported Docker observations and checks that diagnostic requests leave the fixture state unchanged

#### Scenario: Missing disposable engine

- **WHEN** the daemon cannot initialize or the fixture image is absent
- **THEN** the explicit acceptance target fails with the missing prerequisite and never contacts another daemon

## MODIFIED Requirements

### Requirement: Native hosted platform validation

Hosted platform acceptance SHALL run Linux amd64 and arm64 candidates on runners and container images matching their native architectures. Platform preparation SHALL reject an engine architecture mismatch instead of silently using emulation. Local cross-architecture checks MAY retain explicit, labeled emulation, but systemd lifecycle cases SHALL require a matching native engine.

#### Scenario: Hosted arm64 candidate

- **WHEN** CI validates the Linux arm64 candidate
- **THEN** the shared archive and helper execute in an arm64 container on an arm64 runner without QEMU

#### Scenario: Arm64 systemd case on an amd64 engine

- **WHEN** a local amd64 Docker engine selects an arm64 systemd lifecycle case
- **THEN** the case fails before starting a container and reports that native arm64 execution is required

### Requirement: Bounded release manifest decoding

Archive extraction SHALL accept `release.json` only as one complete JSON document within a 64 KiB byte ceiling before trusting its member checksums or executable names. Unknown release manifest fields SHALL be rejected.

#### Scenario: Oversized or trailing release manifest

- **WHEN** `release.json` exceeds 64 KiB or contains a second JSON document
- **THEN** archive extraction fails without accepting the candidate release

#### Scenario: Unknown release manifest field

- **WHEN** `release.json` contains a field outside its declared schema
- **THEN** archive extraction fails without accepting the candidate release
