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

## Generic production-audit expansion

On 2026-09-12, ten explicitly authorized audit tools passed the Go race/static/build suite, archive verification and all ten container matrix cases. Application acceptance runs real PostgreSQL, nginx, Apache and MySQL processes and investigates their process identity, listeners, approved configuration/logs and path metadata through generic MCP calls. It also checks meaningful account, mount, kernel-control and HostLens evidence and verifies source denial.

Three sf-peer-review rounds ended with no further findings. The [review record](../../openspec/changes/archive/2026-09-12-expand-production-audit/review.md) contains corrected issues, test assumptions and measured limits. This expansion adds evidence for investigation, not a complete security assessment: active firewall state, pending upgrade candidates, effective sudo/SSH authorization, device health and application-internal conclusions remain explicit gaps.

## Docker diagnostics validation

The [Docker validation record](../../openspec/changes/archive/2026-09-13-add-system-docker-diagnostics/validation.md) documents implementation, live engines, native systemd installations and resolved findings for the Docker diagnostics integration. The recording daemon proved the GET-only observation allowlist; secret-bearing metadata never crossed the observer IPC; unused-resource semantics kept stopped references, dangling distinction, race suppression and creation-time honesty. Three real engines (Docker 26.1.5 on Debian 13; Docker 29.7.2 and 29.8.0 on Arch) ran the full read-only registry, disposable fixtures on representative engines exercised the reference lifecycle, and native socket activation, upgrade, disablement and uninstall preserved Docker state on representative Debian and Arch installations. The disposable checks container, coverage and vulnerability scans passed; hosted CI validates the exact pushed revision.

## Hosted evidence

Use the [CI workflow](https://github.com/Monska85/hostlens/actions/workflows/ci.yml) to inspect the exact commit being evaluated. A green run for an older revision does not validate later changes. The audit session report records the final pushed commit and its hosted result; this archive does not assert future workflow success or release delivery.

## Limits

- **Native coverage:** Container acceptance shares the Linux host kernel. Emulated arm64 smoke does not prove native arm64 systemd behavior; macOS and Windows runtime support is unimplemented.
- **Blocked I/O:** Kernel-stalled filesystem work can retain bounded backend slots after client timeout. Context cancellation cannot forcibly repair uninterruptible kernel I/O.
- **Load:** Focused benchmarks and concurrency regressions do not establish sustained production capacity across large hosts, slow disks, or unreliable remote mounts.
- **Privilege:** Standard mode has broad read capability. Secret masks and diagnostic policy protect documented paths, not every secret copy or every consequence of a compromised backend.
- **Delivery:** No draft-release upload or public release is established by local tests. Archives currently have integrity checksums, without publisher signatures or provenance attestations.

Failed required checks block release claims. Record newly executed evidence with its scope rather than replacing an unverified result with an assumption.

## CI cache and cancellation validation

The [CI execution record](../../openspec/changes/archive/2026-09-12-optimize-ci-execution/validation.md) contains three comparable baseline runs and three cold/three warm candidate runs, with revisions, job/step times, queue delays and image identities. Warm validation averaged 228.7 seconds and 10.21 job minutes versus 301.7 seconds and 12.45 job minutes at baseline. The measured revision is `bcfab568f24f9a97a080379a29386ca13d3c3116`; the report identifies the small portability/test-harness follow-up separately.

Every candidate run retained native hosted ARM64, both systemd privilege modes, all four application cases and the tested-archive handoff. A deliberately failing test failed after local compiler-cache warmup. Real local cancellation removed its container; hosted supersession cancelled only the obsolete ordinary run while both reusable release-mode probes and unrelated work passed. No release was published.

One warm-run failure exposed restored cache ownership incompatible with dropped container capabilities. The fixed path passed the scanner and full suite locally and in hosted warm runs. Local coverage was 78.1%, with all 15 Python regressions passing. The portable metadata guard removes a GNU-stat dependency, but native macOS client execution was not tested. Shared-kernel/container limits remain unchanged.

## Service metrics validation

The [metrics validation record](../../openspec/changes/archive/2026-09-12-add-service-metrics/validation.md) documents implementation, measurements and three cleanly resolved review rounds. The final implementation passed the complete disposable-container race/coverage suite (79.8%), vet, portable and Linux builds, all ten matrix cases, archive integrity, formatting/lint, and specification validation. The new dependency graph passed the vulnerability scan.

Real systemd cases verified protected and anonymous scrapes, reload transitions, backend loss and process-counter reset in both privilege modes. Adversarial tests cover withheld HTTP bodies, malformed/missing backend measurements, stalled-worker admission, role isolation, credential lifecycle, sensitive-data exclusion and successful MCP work during concurrent scrapes. The exhaustive catalog has 1,745 series and measured about 165 KB at the current tool count. Serial SDK request measurements remained within observed baseline timing variation, with three additional allocations; this does not establish zero overhead or network throughput.

## Post-release coverage and dead-code pass

After the v0.1.0 release, verified dead code was deleted from the observation contract (`ValidName`, `ImageSummary.UniqueSize`, `ContainerSummary.References`/`ResourceRefs`, `Response.Negotiated`, `EngineInfo.Rootless`/`DockerDesktop`, `ContainerSummary` size fields, and `Request.Offset`/`Limit`); the backend and observer ship version-coherent, so the field removals are safe within one artifact set. The pass raised internal statement coverage in the disposable checks container from 77.5% to 85.8%, with tests for the observer transport (success, HTTP refusal, dead socket, malformed body, cancellation), the socket-activated listener (including child-process coverage merging through `-test.gocoverdir`), archive extraction, lifecycle commands and readiness, CLI flag and plan paths, gateway envelopes and measured flushes, and the CPU health sample.

The pass also fixed a real defect the new tests exposed: `limitedBuffer` embedded `bytes.Buffer`, whose promoted `ReadFrom` let `exec`'s `io.Copy` bypass the inspection limit entirely, so oversized command output was captured unbounded. The buffer is now a plain `io.Writer` and the ceiling is enforced and tested.

Functions intentionally below 50% or uncovered, with reasons:

- Observer and CLI `Main` serve/signal loops: process lifetime, covered by native acceptance instead.
- `ListenInherited` and observer `Main` listener setup beyond refusal branches: socket activation success requires a real service manager handoff; the subprocess helper covers the path and merges its counters.
- `secretIsMasked` guards after the reference check (reference `SameFile` mismatch, non-empty file size, unclean path, success for file and directory masks): they need a real systemd inaccessible mount under `/run/systemd/inaccessible`; tests refuse to create one. Reachable guards (no matching mount, prefix match, missing `ro`, unstatable path, non-root uid, non-zero permissions) are covered.
- `docker_live_test.go` stays environment-gated and is excluded from coverage by design.
- `PeerUID`/`resolveUID`/`resolveGID` syscall-error and corrupted-system-file branches: unreachable without corrupting live system files.
