# Extended audit record

This audit follows the archived repository hardening review. Findings require a reproducible failure or a concrete contract mismatch; a review with no findings is valid. Publication is held until both authorized review cycles and final validation finish.

## Method and limits

The current panel divides runtime implementation and contract/documentation/CI review between independent reviewers. The delegated security/adversarial reviewer was blocked by the platform before producing findings. Local defensive verification continues, but does not count as an independent security-panel pass.

Local adversarial probes include scale changes (more than one service page and 10,000 policy rules), reused service identities with conflicting IDs or unrelated members, stalled native discovery, and malformed service records. Tests run in disposable containers. These checks establish specific behavior, not a guarantee that the repository has no defects.

## Cycle one, round one

| ID    | Severity | Verified finding and correction                                                                                                                                                          | Evidence                                                                                                                                               |
| ----- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| R1-01 | Medium   | Health lost failed services beyond the public page limit. Health now consumes bounded collection before pagination.                                                                      | Service-health fixtures retain failures beyond 200 units.                                                                                              |
| R1-02 | High     | Capability discovery held state locks during native work. Discovery now shares one bounded worker and releases state locks before observation.                                           | Backend race tests cover stalled discovery, cancellation, admission, and activation.                                                                   |
| R1-03 | High     | Installation adopted service groups without checking their IDs or explicit members. Reject zero/colliding IDs and unrelated explicit members before mutation and after account creation. | Lifecycle race tests verify rejection before installing files. This does not enumerate every unrelated account's primary group.                        |
| R1-04 | Medium   | Policy decisions allocated provenance records for each observation. Direct decision evaluation preserves denial precedence without those allocations.                                    | Policy semantics and race tests pass; focused 10,000-rule benchmarks report zero decision-path allocations. Timing depends on workload and contention. |
| R1-05 | Low      | MCP tools advertised irrelevant arguments and accepted them silently. Each tool now advertises and validates its own arguments.                                                          | Real SDK discovery/call tests reject irrelevant and missing source arguments before collector execution.                                               |
| R1-06 | Medium   | Malformed service records were silently skipped and partial health could erase observed severity. Preserve valid observations and aggregate malformed-record coverage issues.            | Fixtures cover malformed rows and simultaneous observed failures and denied coverage.                                                                  |

Focused container race tests passed for backend, gateway, lifecycle, Linux collection, and policy. Contract/documentation/CI review returned no additional verified findings.

## User-directed tooling correction

OpenSpec is installed externally at `/usr/bin/openspec`. Project requirements and history remain in `openspec/`; repository npm manifests, local dependencies, and Prettier configuration/integration were removed. Make and Just retain equivalent Go, Python, and shell commands. Agents validate specifications with the external CLI. The user also removed specification validation from CI and project task runners, so CI needs neither OpenSpec nor Node.js. Generated local OpenSpec skills were removed after verifying that global replacements exist. Markdown formatting belongs to the harness.

The initial cleanup passed static checks, specification validation, equivalent task-runner help, and two container developer-tool tests. The later removal of the specification dispatcher also removes its now-obsolete test; final validation records the retained suite.

## Cycle one, round two

| ID    | Severity | Verified finding and correction                                                                                                                                                                             | Evidence                                                                                                           |
| ----- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| R2-01 | Medium   | IPC client/server deadlines clipped a permitted maximum-duration operation. Separate body-read, operation, and response budgets.                                                                            | Real Unix-socket regression with scaled production budgets and a delayed body returns the explicit timeout result. |
| R2-02 | Medium   | Gateway discarded deadline/cancellation causes as backend failures. Preserve and classify causes through RPC and MCP responses.                                                                             | Direct calls and real SDK requests distinguish timeout, cancellation, and unavailable backend.                     |
| R2-03 | Low      | Mount decoding omitted the kernel's newline escape, losing an existing filesystem observation. Decode the escape with the other mount-field escapes.                                                        | An actual newline-named temporary directory resolves through a mount-table fixture and native Statfs.              |
| R2-04 | High     | The SDK detached tool execution from HTTP cancellation; coincident response deadlines could also erase timeout JSON. Link tool work to the request's operation context and retain a bounded response grace. | Real HTTP MCP requests verify timeout JSON and propagation of client disconnect to backend work.                   |

Focused race tests passed for gateway, backend, Linux collection, diagnostics startup, configuration, and contract packages. Independent contract review returned no findings. A concern about diagnostics membership in the gateway group was rejected: the installed diagnostics unit deliberately uses that group for IPC access.

### Native ARM64 CI

R2-05, Medium, user-identified finding: the ARM64 case uses an amd64 runner and QEMU even though GitHub supports standard native ARM64 runners for private and public repositories. The workflow also explicitly pulls an amd64 container image. Native execution removes the CI emulator dependency and tests against an ARM64 kernel.

GitHub lists `ubuntu-24.04-arm` for both repository visibility modes in its [runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners). Private-repository support is confirmed in the [January 29, 2026 announcement](https://github.blog/changelog/2026-01-29-arm64-standard-runners-are-now-available-in-private-repositories/). The workflow now selects the native runner, asserts Docker engine architecture, and pulls the matching platform image. Eight existing matrix tests and static checks passed. Hosted native execution remains pending final publication.

## Cycle one, round three

R3-01, Low, user-identified CI usability finding: matrix and other validation steps conceal useful stage progress in retained logs or silent commands. Add concise case identity, fixed stage markers, and success/failure summaries to console output. Preserve full logs separately, exit status, cancellation, and credential confidentiality. Nine matrix tests and the retained developer-command test passed, including a regression proving progress is visible before completion and nonzero failure status is preserved. Static checks passed.

R3-02, Low, interface cleanup: `GatewayUnit` accepted configuration it never used. Removed the parameter and updated installation, renderer, and test callers. The lifecycle race suite and renderer compilation passed in a disposable container.

R3-03, High: concurrent backend generation preparation starts unbounded configuration loaders, and cancelled preparation can later stage its candidate. A container regression reproduced blocked cancelled requests, multiple stalled loaders, and post-cancellation staging. The correction uses one preparation slot retained until underlying loading ends, bounded request waits, and no staging after cancellation. The regression passed after correction, including recovery with a fresh valid preparation; backend and gateway race tests passed.

## Cycle one, rounds four and five

R4-01, Low: the new live matrix reporter read unbounded child-output lines and loaded the complete failure log before taking its tail. Reads now use 4 KiB chunks with explicit line-start tracking; failure excerpts read at most the final 16 KiB and show at most 40 lines. Complete artifacts remain available. A 4 MiB newline-free failing fixture verifies bounded console output, complete retained logs, and preservation of exit status. All ten matrix tests passed.

Runtime closure in round four and tooling/CI closure in round five reported no further findings. The first cycle closed 15 findings: four High, six Medium, and five Low. User-directed removal of harness tooling is recorded separately. No commit or push occurred during these rounds.

## Fresh-eyes cycle

Two new independent reviewer contexts inspected the whole current project without reading prior finding ledgers and without a preferred finding category.

Round one independently reproduced the same Medium finding, F1-01: Debian packages marked held remained installed but were omitted because the parser required desired selection and actual state to both be `i`. One reviewer held an installed package in a disposable Debian container; the other used an isolated temporary dpkg database. Both observed `hi` output from the exact production query. The parser now uses the actual installation-state column independently of desired selection, consistent with the [dpkg-query manual](https://manpages.debian.org/bookworm/dpkg/dpkg-query.1.en.html). The regression covers held, normal, removal-selected and other installed states, and excludes noninstalled states. The complete Linux collector race suite passed.

Round two reviewed the correction and adjacent behavior. Both reviewers returned **NO FINDINGS**. The cycle closed after two of the five permitted rounds; no extra findings or rounds were manufactured.

Across both cycles, 16 verified findings were corrected: four High, seven Medium, and five Low. No unresolved verified finding remains. The independent security-panel limitation described above remains part of the review evidence.

## Final local validation

- **Runtime:** Combined disposable-container Go race suite, vet, Linux amd64/arm64 builds, and portable macOS/Windows package builds passed. The later package correction passed the full Linux collector race suite.
- **Tooling:** Six release, ten matrix, and one developer-command container tests passed. Formatting, Ruff, ShellCheck, actionlint, and GoReleaser configuration validation passed.
- **Dependencies:** A fresh disposable-container govulncheck scan reported no known vulnerabilities at scan time.
- **Packaging:** Both final archives and their expanded contents/helpers passed verification. The same candidate passed all six local matrix cases: Debian/Ubuntu/Arch amd64, explicitly emulated Debian arm64, and native amd64 systemd restricted/standard lifecycle acceptance.
- **Documentation:** The external OpenSpec CLI validated current specifications and archived task completion. All 139 checked local Markdown link targets resolved. Project CI requires neither OpenSpec nor Prettier.
- **CI output:** The real matrix printed target/image/architecture, fixed stage progress, and clear per-case results in console output. Complete logs remained separate artifacts; bounded-read and failed-output regressions passed.

Native hosted ARM64 execution and the aggregate hosted gate remain pending the single authorized final push. Record the exact pushed SHA and hosted result in the final session report. Local emulation does not establish native ARM64 systemd behavior, and no macOS/Windows runtime or public release is claimed.
