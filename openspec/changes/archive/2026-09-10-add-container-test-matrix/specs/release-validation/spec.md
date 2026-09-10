## ADDED Requirements

### Requirement: Shared selectable container matrix

Local and hosted acceptance SHALL use the same named platform and lifecycle cases. An optional target SHALL select exactly one case; omitted selection SHALL run all cases. Unknown targets and invalid concurrency SHALL fail before launching tests. Local execution SHALL use bounded concurrency, retain individual case logs, and return failure if any selected case fails. Interrupted execution SHALL stop its own active containers.

#### Scenario: Focused check

- **WHEN** a developer selects a valid native amd64 platform target
- **THEN** only that case runs and unrelated QEMU or systemd prerequisites are not required

#### Scenario: Invalid selection

- **WHEN** a target does not exist or the worker count is invalid
- **THEN** the command fails before launching a container and identifies valid input

#### Scenario: Partial matrix failure

- **WHEN** one selected case fails
- **THEN** remaining cases report their results and the aggregate command fails with separate logs

### Requirement: Native local target selection

The local `native` target SHALL resolve to Debian for the connected Linux Docker engine's architecture, using engine metadata rather than assuming the client host architecture. Native arm64 selection SHALL use arm64 container images and Linux arm64 test helpers without requiring QEMU. Explicit cross-architecture cases SHALL retain their declared coverage and label emulated execution. Unsupported engine architectures SHALL fail clearly.

#### Scenario: macOS arm64 development client

- **WHEN** a developer uses the native target with an arm64 Linux Docker engine from macOS
- **THEN** the command selects Debian arm64 and does not require an amd64 image or QEMU for that case

#### Scenario: Remote engine

- **WHEN** the Docker engine architecture differs from the client host
- **THEN** native selection follows the engine architecture

### Requirement: Verified matrix candidate

CI SHALL build the candidate once for matrix acceptance and release delivery, and every matrix job SHALL consume that candidate. Local matrix checks SHALL reuse an existing build, reject missing or stale build inputs, and SHALL NOT rebuild or download prerequisites automatically. Failed or skipped required matrix jobs SHALL block the final CI gate and release delivery.

#### Scenario: Stale local build

- **WHEN** relevant build inputs differ from those recorded for the candidate
- **THEN** the matrix command fails before launching tests and instructs the developer to rebuild

#### Scenario: Failed hosted case

- **WHEN** any required case fails or is skipped
- **THEN** the final CI gate fails and release delivery cannot run
