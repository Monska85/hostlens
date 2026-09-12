# Release validation

## Purpose

Define the observable HostLens v1 release validation behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Representative compatibility evidence

Release validation SHALL cover Linux amd64 and arm64 and record representative Debian, Ubuntu, and Arch environments. Tests SHALL exercise capability detection rather than enforce a distribution release allowlist. Native execution evidence SHALL be distinguished from emulation; Arch Linux ARM SHALL not be conflated with official Arch Linux.

#### Scenario: New version

- **WHEN** a new distribution version preserves interfaces
- **THEN** HostLens needs no release solely to recognize its version

#### Scenario: Emulated coverage

- **WHEN** arm64 tests run under emulation
- **THEN** the report labels that evidence accurately

### Requirement: Container test execution

All tests SHALL run in disposable containers by default, including userspace detection, package parsing, policy, authentication, protocol, systemd, journal permissions, privilege containment, and lifecycle checks where supported. Reports SHALL state the shared kernel and namespace scope and identify checks that the container environment cannot establish. Privileged lifecycle tests SHALL NOT run on the administrator host.

A VM SHALL be used only when strictly necessary for a concrete acceptance check. The agent SHALL explain the necessity and obtain explicit user confirmation before provisioning, downloading, or starting the VM. Test tooling SHALL NOT provision VMs automatically.

#### Scenario: Container resource view

- **WHEN** tests run within namespaces or cgroups
- **THEN** results identify that scope and do not claim to describe or isolate the physical host

#### Scenario: Service validation limitation

- **WHEN** container tests pass but the container cannot exercise required systemd or privilege containment behavior
- **THEN** the report identifies the unverified behavior and does not declare it validated

#### Scenario: Strictly necessary VM

- **WHEN** a concrete acceptance check cannot be established in a disposable container and strictly requires a VM
- **THEN** the agent explains the limitation and waits for explicit user confirmation before any VM provisioning, image download, or startup

#### Scenario: Container default

- **WHEN** an implementation or validation task starts
- **THEN** its test commands use disposable containers and perform no privileged lifecycle changes on the administrator host

### Requirement: Security and failure acceptance

Before release, validation SHALL exercise denied-source access through every tool, symlink and path races, special files, includes and cycles, reload consistency, token updates and revocation, trusted-proxy spoofing, listener failures, quotas, and lifecycle rollback. Failed required checks SHALL block release claims.

#### Scenario: Boundary regression

- **WHEN** a denied source can be returned through a second tool
- **THEN** release acceptance fails

#### Scenario: No invented evidence

- **WHEN** a collector loses access to its source
- **THEN** tests require omitted measurements with explicit collection issues, not zeros or healthy output

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

### Requirement: Shared selectable container matrix

Local and hosted acceptance SHALL use the same named platform and lifecycle cases. A target SHALL select exactly one case; omitted selection SHALL run all cases. Unknown targets SHALL fail before launching tests. Local execution SHALL run cases sequentially, expose their output, aggregate failures, and stop its active container when interrupted. Hosted execution MAY use the CI platform's native parallelism.

#### Scenario: Focused check

- **WHEN** a developer selects a valid native amd64 platform target
- **THEN** only that case runs and unrelated QEMU or systemd prerequisites are not required

#### Scenario: Invalid selection

- **WHEN** a target does not exist
- **THEN** the command fails before launching a container and identifies valid input

#### Scenario: Partial matrix failure

- **WHEN** one selected case fails
- **THEN** remaining cases report their results and the aggregate command fails with visible case outcomes

### Requirement: Native local target selection

The local `native` target SHALL resolve to Debian for the connected Linux Docker engine's architecture, using engine metadata rather than assuming the client host architecture. Native arm64 selection SHALL use arm64 container images and Linux arm64 test helpers without requiring QEMU. Explicit cross-architecture cases SHALL retain their declared coverage and label emulated execution. Unsupported engine architectures SHALL fail clearly.

#### Scenario: macOS arm64 development client

- **WHEN** a developer uses the native target with an arm64 Linux Docker engine from macOS
- **THEN** the command selects Debian arm64 and does not require an amd64 image or QEMU for that case

#### Scenario: Remote engine

- **WHEN** the Docker engine architecture differs from the client host
- **THEN** native selection follows the engine architecture

### Requirement: Fresh archive staging

Archive construction SHALL use only current declared source inputs and dependency notices. Files left in a previous staging tree SHALL NOT enter a newly built archive.

#### Scenario: Removed profile

- **WHEN** a profile is removed before rebuilding
- **THEN** the rebuilt archive excludes it even if an older staging directory contains it

### Requirement: Executed candidate identity

Acceptance SHALL validate and extract the selected archive in a fresh disposable directory and execute those extracted files. Validation SHALL enforce the runtime manifest contract, extraction size ceilings, and complete gzip integrity. Test helpers SHALL be built from the selected checkout into disposable storage.

#### Scenario: Replaced expanded executable

- **WHEN** an unrelated old staging executable or reusable helper changes
- **THEN** acceptance ignores those files and uses the selected archive and freshly built helpers

#### Scenario: Damaged compressed stream

- **WHEN** an archive payload has valid member hashes but a missing or corrupt gzip trailer
- **THEN** verification and runtime extraction reject it

### Requirement: Documentation and specification gate

CI SHALL gate source checks, builds, dependency scanning, and container tests. Agents SHALL validate current and archived project OpenSpec artifacts through an externally installed CLI before completing changes. OpenSpec and harness-owned document formatters SHALL NOT be repository dependencies or required CI checks.

#### Scenario: Invalid requirement

- **WHEN** an invalid requirement or scenario enters the repository
- **THEN** agent validation fails and the change remains incomplete until corrected

### Requirement: Consistent developer commands

Make and Just SHALL expose equivalent setup, formatting, linting, testing, and combined-check commands. Repository formatting SHALL cover maintained Go, Python, and POSIX shell. Repository-managed development tools SHALL use versioned manifests and integrity locks, install locally, and remain separate from runtime dependencies. OpenSpec and document formatting SHALL be supplied by the development environment or agent harness, without repository npm manifests or local copies. Checks SHALL NOT install missing tools or container images implicitly.

#### Scenario: Fresh contributor setup

- **WHEN** a contributor runs the dependency setup with the documented base tools
- **THEN** the repository installs its locked development tools locally and provides the same validation commands used by CI

#### Scenario: Missing formatter

- **WHEN** a required formatter is unavailable
- **THEN** the format check fails with setup guidance rather than silently skipping a language

### Requirement: Native hosted platform validation

Hosted platform acceptance SHALL run Linux amd64 and arm64 candidates on runners and container images matching their native architectures. Platform preparation SHALL reject an engine architecture mismatch instead of silently using emulation. Local cross-architecture checks MAY retain explicit, labeled emulation.

#### Scenario: Hosted arm64 candidate

- **WHEN** CI validates the Linux arm64 candidate
- **THEN** the shared archive and helper execute in an arm64 container on an arm64 runner without QEMU

### Requirement: Visible validation progress

Validation SHALL show concise case context, stage progress, and outcomes in console output. Matrix cases SHALL identify the target, image, architecture, and native or emulated execution. Failures SHALL remain understandable from console output without downloading artifacts. Output SHALL exclude credentials and inspected payloads. Reporting SHALL preserve failure and cancellation status.

#### Scenario: Failed matrix stage

- **WHEN** a matrix case fails during a named stage
- **THEN** console output shows the case and stage output followed by its failure status

### Requirement: Inspectable test coverage

Make and Just SHALL provide equivalent coverage commands that execute tests in disposable containers, print a readable summary, and save a local machine-readable profile and HTML report. Reports SHALL identify their measured code and test scope without treating a percentage as proof of security or completeness. CI SHALL execute the maintained regression and end-to-end checks and retain the coverage report. Test or report-generation failures SHALL fail the command; old output SHALL NOT be presented as a successful new result.

#### Scenario: Inspect local gaps

- **WHEN** a developer runs the coverage command with prepared prerequisites
- **THEN** the command reports measured coverage and identifies a local HTML report for inspecting uncovered code

#### Scenario: Failed coverage run

- **WHEN** a test fails or report generation fails
- **THEN** the command returns failure and does not claim that previous reports validate the current source

#### Scenario: End-to-end scope

- **WHEN** an acceptance test executes separate gateway and diagnostic processes
- **THEN** validation distinguishes that behavioral evidence from code included in coverage instrumentation

### Requirement: Separate compilation and distribution

Developer build commands SHALL compile the project binaries using the Go toolchain without requiring Python, Docker, or release-publishing tools. A distinct packaging command SHALL produce the installable archives through the same implementation used in CI. Packaging SHALL use GoReleaser OSS for archive creation and a standard SHA-256 checksum list. It SHALL preserve the existing upgrade manifest, licenses, and permissions. The checksum list SHALL cover both architecture archives and support standard checksum verification.

#### Scenario: Compile for development

- **WHEN** a contributor builds the binaries with Go and prepared module dependencies
- **THEN** compilation does not run archive creation, container tests, or release publication

#### Scenario: Create a distributable candidate

- **WHEN** a maintainer runs the packaging command
- **THEN** its archives can enter the shared validation matrix and retain the existing installation and upgrade contract

### Requirement: Container-owned validation toolchain

The prepared test image SHALL supply its Linux Go toolchain, race-test prerequisites, Python runtime, and dependency vulnerability scanner. Test execution SHALL NOT require mounting a separate host Linux Go installation or host-native scanner executable. Image preparation SHALL remain explicit, and tests SHALL preserve resource bounds, offline runtime-module access, and disposable filesystem isolation.

#### Scenario: Non-Linux development client

- **WHEN** a contributor has a prepared test image and source/module mounts available to a Linux Docker engine
- **THEN** container validation uses executable tools from the image rather than binaries built for the client operating system

### Requirement: Explicit archive acceptance

Local acceptance SHALL identify the release version and archive directory it tests. It SHALL NOT rebuild release binaries or claim an existing archive matches current source. CI SHALL produce one candidate, share its archive bytes across acceptance jobs, and publish those same bytes only after every required check succeeds.

#### Scenario: Test a selected release

- **WHEN** the developer selects an existing archive version
- **THEN** acceptance tests that archive independently of unrelated working-tree changes

#### Scenario: Test current source

- **WHEN** the developer needs acceptance for current source
- **THEN** the documented workflow explicitly packages that source before running acceptance

#### Scenario: Failed hosted case

- **WHEN** any required case fails or is skipped
- **THEN** the final CI gate fails and release delivery cannot run

### Requirement: Scoped CI supersession

Ordinary branch and pull-request CI SHALL cancel obsolete runs superseded within the same workflow and branch or pull request. Unrelated work and release validation SHALL remain unaffected, including validation invoked through a reusable workflow. Cancellation SHALL NOT be reported as successful validation or permit release delivery.

#### Scenario: Updated candidate

- **WHEN** a newer update starts ordinary CI for the same branch or pull request
- **THEN** the obsolete run is cancelled and the new run executes the required checks

#### Scenario: Independent release

- **WHEN** ordinary CI is superseded while tag-triggered release validation is running
- **THEN** release validation and its caller are not cancelled by the ordinary CI concurrency group

#### Scenario: Interrupted acceptance

- **WHEN** a running container case is cancelled
- **THEN** cleanup terminates its test resources and cancellation remains visible rather than becoming a successful gate result

### Requirement: Safe optional validation caches

Validation MAY reuse compatible image layers and compiler outputs but SHALL execute every required test for the current candidate. Cached outputs SHALL NOT substitute for source, credentials, test verdicts or selected release archives. Untrusted contributions SHALL NOT gain authority to populate caches consumed as trusted release inputs. Unavailable or incompatible caches SHALL allow ordinary uncached validation without weakening checks.

#### Scenario: Warm cache with failing test

- **WHEN** compatible cached compilation exists and a current required test fails
- **THEN** the test still executes and the CI gate fails

#### Scenario: Cache unavailable

- **WHEN** cache restoration is disabled, misses or cannot access its service
- **THEN** validation performs normal preparation and all required checks, reporting actual failures

#### Scenario: Incompatible or untrusted cache

- **WHEN** a cache belongs to an incompatible platform/toolchain or a less-trusted contributor context
- **THEN** it is not restored into a trusted validation context through a permissive fallback

#### Scenario: Local uncached execution

- **WHEN** a developer uses prepared prerequisites without hosted-cache credentials
- **THEN** equivalent Make and Just checks remain available with the existing isolation and resource limits

### Requirement: Measured CI optimization

CI optimization evidence SHALL compare equivalent validation scope and identify source revision, runner environment, cache state and sample count. Reports SHALL distinguish end-to-end elapsed time, queue delay and total job execution minutes. Improvements SHALL NOT be claimed from skipped checks, cached test verdicts or measurements that do not establish the claimed reduction.

#### Scenario: Cold and warm comparison

- **WHEN** the optimized workflow is evaluated
- **THEN** cold and warm runs execute the maintained checks and matrix cases, and the report records their timing differences and comparison limitations

#### Scenario: Parallelism trade-off

- **WHEN** matrix parallelism changes
- **THEN** the report evaluates elapsed time and runner usage separately and records why the selected limit is retained

#### Scenario: No demonstrated gain

- **WHEN** an optimization has no demonstrated benefit or adds disproportionate machinery
- **THEN** it is removed or retained with an explicit non-performance justification, without an unsupported speed claim

### Requirement: MCP safety invariant validation

Release validation SHALL prove that all registered MCP tools have exactly one effect classification and that discovery and direct dispatch enforce the same active configuration, role, policy, capability, and remediation gates. Validation SHALL exercise both values of `mcp.read_only`, omitted-setting default behavior, direct remembered calls, unclassified registration, coordinated reload, and backend non-invocation on rejection. The v1 test surface SHALL continue to contain no remediation implementation.

#### Scenario: Exhaustive registry check

- **WHEN** the maintained MCP tool registry is validated
- **THEN** every tool has one recognized effect and no discovered or dispatched tool exists outside that registry

#### Scenario: Direct-call bypass attempt

- **WHEN** a test directly calls a remediation or unclassified tool while MCP read-only mode is active
- **THEN** validation observes rejection before any backend or external integration invocation

#### Scenario: Default configuration

- **WHEN** acceptance runs with a configuration that omits `mcp.read_only`
- **THEN** validation observes the same diagnostic-only MCP surface as explicit `mcp.read_only: true`

#### Scenario: Local administrator regression

- **WHEN** acceptance runs local administrative lifecycle and token operations with MCP read-only mode active
- **THEN** those operations retain their existing authorization and behavior without becoming MCP tools

### Requirement: Stateless diagnostic acceptance

Release validation SHALL exercise every diagnostic tool against mutable fixtures and verify a new source observation on each request. Tests SHALL inspect HostLens-managed files, component state, operational logs, and service metrics after success, failure, timeout, and cancellation for retained diagnostic content, inventories, cleanup candidates, or plans. Timeout and cancellation checks SHALL wait for the admitted collector worker to acknowledge cancellation; an intentionally stalled worker SHALL remain bounded and visible as active rather than being reported as completed. Unique sensitive markers SHALL remain absent from logs and metrics. Container limitations SHALL be recorded without converting an unverified persistence or mutation boundary into a pass.

#### Scenario: Mutable source fixture

- **WHEN** a permitted source changes after one request and before the next
- **THEN** the next diagnostic result reflects the new live value and no HostLens cache supplies the old value

#### Scenario: Sensitive evidence marker

- **WHEN** a diagnostic source and result contain a unique test marker
- **THEN** post-request inspection finds the marker only in the source and client response, not in HostLens-managed files, audit logs, service metrics, or retained component state

#### Scenario: Cancellation and failure

- **WHEN** collection fails or is cancelled after observing partial evidence
- **THEN** validation observes bounded active work until the collector exits and then finds no reusable partial result or inspected payload

#### Scenario: Observer mutation attempt

- **WHEN** a diagnostic integration attempts an operation outside its read contract
- **THEN** validation observes rejection and no target-state change
