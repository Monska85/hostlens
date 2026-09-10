# Validation evidence

This page records tested behavior and limits. It is not a production certification or a published release record. Reproduce checks through the repository's `docs/RELEASING.md`.

## Repository hardening audit

The repository-wide audit began on 2026-09-10, covering runtime security, performance, maintainability, specifications, documentation, and CI. Its 41 finding dispositions are recorded in `openspec/changes/archive/2026-09-10-harden-repository/review.md`. The audit used three review/improvement rounds with nine review perspectives.

The audit exercised these checks successfully:

| Check                       | Evidence                                                                                                                                     |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Runtime                     | Disposable-container Go race suite, vet, Linux amd64/arm64 builds, and focused boundary/failure regressions                                  |
| Packaging and tooling       | Six release tests, eight matrix tests, and one developer-command test in disposable containers                                               |
| Compatibility and lifecycle | Debian, Ubuntu, Arch amd64, emulated Debian arm64, and native systemd restricted/standard acceptance                                         |
| Security regression         | Masked hard-link denial, token parsing/limits, overload/deadlines, configuration ceilings, and upgrade rollback                              |
| Static validation           | Go/Python/shell/document formatting, Ruff, ShellCheck, actionlint, strict current/archived OpenSpec validation, and GoReleaser configuration |
| Dependencies                | Fresh runtime `govulncheck`: no vulnerabilities found at scan time                                                                           |
| Documentation               | 138 local Markdown links resolved before archival; archived links and specifications were reconciled                                         |

The systemd cases exercise real service identities, privilege masks, and install/upgrade/removal in disposable containers. The standard case also verifies upgrade readiness on a nondefault port and rejection of a pre-existing alias to a masked key.

A real bind-mask probe reproduced retrieval through a pre-existing hard-link alias. The fixed reader rejected the same alias in a disposable mount-capable container. This requires a pre-existing alias, normally created by an administrator; it is not an unprivileged link-creation exploit.

Portable config, policy, contract, gateway, backend, and token packages cross-built for Windows amd64 and macOS arm64. This verifies compilation boundaries only, not platform runtime support.

## Performance evidence

The first audit's Linux amd64 container benchmark on an AMD Ryzen 9 PRO 7940HS compared cached backend status with the previous fingerprint computation at 10,000 policy rules. Across three runs, cached status used 111 to 114 ns/op, 256 B/op, and two allocations. Uncached fingerprint calculation used 10.8 to 13.5 ms/op, about 7.5 MB/op, and 120,210 allocations. These timings predate the extended audit's bounded discovery worker.

This isolates immutable-policy overhead with a fixture capability collector. It is not an end-to-end throughput measurement. `BenchmarkStatus` retains the policy-size regression check.

The extended audit removed provenance allocation from policy access decisions. Focused 10,000-rule benchmarks measured zero allocations for unique and repeated patterns. They do not establish end-to-end request throughput.

## Extended audit

The extended review exercised service health beyond pagination, malformed service records, unsafe group adoption, stalled discovery and configuration preparation, MCP argument validation, IPC deadlines, and real HTTP cancellation. Regression tests reproduce the affected behavior; the [audit records](../../openspec/changes) retain findings and dispositions.

The combined disposable-container suite passed Go race tests, vet, both Linux builds, portable macOS/Windows compilation checks, six release utility tests, ten matrix utility tests, and one developer-command test. Static checks and GoReleaser configuration validation passed; a fresh vulnerability scan found no known vulnerabilities at scan time. Specification validation runs through the agent's external OpenSpec CLI, separately from project CI.

Hosted ARM64 platform acceptance now selects a native ARM64 runner and image. Local ARM64 emulation remains separate evidence. Inspect the exact commit's hosted run below for execution results; configuration alone is not a native test pass.

## Reduction and QA review

The reduction cycle removed redundant command wrappers, populated-directory placeholders, and unreachable branches without changing defined behavior. Its checkpoint passed all nine hosted CI jobs.

QA strengthened role-removal, rotation-overlap, and deadline-growth assertions. Added checks cover audit correlation and payload exclusion, HTTP origin/body limits, policy rule ceilings, and coverage failure/cancellation behavior. Systemd acceptance now proves rejection of unrelated UIDs even when socket filesystem permissions permit connection, followed by authorized positive controls.

Go statement coverage measures unit/integration execution and instrumented CLI subprocesses in `internal/...`; Python tests and platform/systemd helper execution are separate evidence. Use the [coverage command](../RELEASING.md#coverage) for the current source's report. The initial 67.7% baseline excluded subprocess counters and cross-package execution, so changes in percentage cannot be attributed solely to new tests.

The [review records](../../openspec/changes) retain panel findings, corrected test assumptions, measured results, and final validation status. No duplicate test was removed merely because another layer exercises related behavior.

## Delivery toolchain review

GoReleaser OSS snapshot packaging passed archive/member verification and the six-case container matrix. Independent Make and Just runs produced identical archive checksums with the same source and toolchain. Container regression tests retained 76.6% internal Go statement coverage; the container-owned vulnerability scanner found no known reachable vulnerabilities at scan time.

The [delivery review](../../openspec/changes/archive/2026-09-11-simplify-delivery-toolchain/review.md) records removed custom tools, retained responsibilities, sources, and limits. This stage has local evidence only; the new validation image was executed on amd64, and local ARM64 matrix acceptance used emulation.

The subsequent [validation machinery reduction](../../openspec/changes/archive/2026-09-11-reduce-validation-machinery/review.md) passed the container race/vet/build suite, ten Python regressions, both archive verifications and all six acceptance cases. Internal statement coverage was 76.7%. Safe publication fixtures checked asset selection and failure propagation; no release was uploaded. The independent review's systemd context correction also passed a focused case.

The [follow-up acceptance review](../../openspec/changes/archive/2026-09-11-tighten-acceptance-checks/review.md) additionally verified structured observations, selected-image propagation, standard checksum failure handling and bounded readiness. Its final six-case matrix passed; eleven Python regressions passed, and internal coverage remained 76.7%.

The [final validation-gap fixes](../../openspec/changes/archive/2026-09-11-close-validation-gaps/review.md) enforce complete checksum lists and bounded status probes. Retained checksum, HTTP readiness and stalled-probe regressions passed, alongside all six acceptance cases. Thirteen Python regressions passed across the full suite and final focused rerun.

## Hosted evidence

Use the [CI workflow](https://github.com/Monska85/hostlens/actions/workflows/ci.yml) to inspect the exact commit being evaluated. A green run for an older revision does not validate later changes. The audit session report records the final pushed commit and its hosted result; this archive does not assert future workflow success or release delivery.

## Limits

- **Native coverage:** Container acceptance shares the Linux host kernel. Emulated arm64 smoke does not prove native arm64 systemd behavior; macOS and Windows runtime support is unimplemented.
- **Blocked I/O:** Kernel-stalled filesystem work can retain bounded backend slots after client timeout. Context cancellation cannot forcibly repair uninterruptible kernel I/O.
- **Load:** Focused benchmarks and concurrency regressions do not establish sustained production capacity across large hosts, slow disks, or unreliable remote mounts.
- **Privilege:** Standard mode has broad read capability. Secret masks and diagnostic policy protect documented paths, not every secret copy or every consequence of a compromised backend.
- **Delivery:** No draft-release upload or public release is established by local tests. Archives currently have integrity checksums, without publisher signatures or provenance attestations.

Failed required checks block release claims. Record newly executed evidence with its scope rather than replacing an unverified result with an assumption.
