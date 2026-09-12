# Service metrics validation

Implementation, local acceptance and three panel rounds are complete with no unresolved findings. Final main publication and hosted CI are the remaining delivery gates.

## Executed checks

The established disposable-container race suite, vet, Linux amd64/arm64 builds, portable package builds, and all Python regressions passed. Initial full internal coverage was 79.5%. The dependency scanner reported no vulnerabilities. GoReleaser packaging and release configuration checks passed.

Tests exercise role isolation and unions, unchanged bearer secrets after role updates, revocation, rotation expiry, authentication/method/disabled precedence, anonymous access without credential interpretation, unchanged proxy/MCP authentication, reload rollback, admitted snapshot completion, separate admission before token reads, cancelled stalled workers, backend deadline and malformed responses, generation isolation, finite series, and MCP progress during eight concurrent scrape clients.

The first full matrix passed eight cases. ARM64 used an incorrect local emulator environment variable; the corrected `HOSTLENS_QEMU_AARCH64` invocation passed. Standard systemd acceptance completed all metrics checks but then hit `Result=start-limit-hit` after the deliberately expanded restart sequence. The fixture now clears only that test restart history after secret-containment recovery is verified. The corrected full matrix subsequently passed all ten cases, and passed again against the final audited archives.

## Request overhead

Workload: unchanged MCP `tools/list` handler with a fixed health identity, in-memory backend status response, real SDK parsing/schema setup, and recorder response. Token-file disk I/O and network transport are excluded to isolate instrumentation. Serial request concurrency is one; the container has two CPUs, 4 GiB memory, no network, a read-only source/module mount and disposable writable storage. Go 1.27.0, Linux amd64, AMD Ryzen 9 PRO 7940HS. Each row has three two-second samples. Baseline source is main `e8761947ec3b68137d36bfeded297978ecae9713`; its enabled/disabled labels execute the identical unchanged path.

| Variant                  | Mean ns/request | Requests/second | Bytes/request | Allocations/request |
| ------------------------ | --------------: | --------------: | ------------: | ------------------: |
| Baseline, first group    |         303,657 |           3,293 |       530,994 |               2,786 |
| Baseline, repeated group |         308,211 |           3,245 |       531,022 |               2,786 |
| Instrumented, disabled   |         307,521 |           3,252 |       531,148 |               2,787 |
| Instrumented, enabled    |         307,220 |           3,255 |       531,243 |               2,789 |

Enabled latency lies within the baseline sample range (296,463 to 309,164 ns). The observed extra three allocations and roughly 249 bytes per request are small relative to this SDK workload. These samples do not establish zero cost, a speed improvement, or production network throughput. Disabled instrumentation skips observation updates; enabled adds fixed-vector observations without I/O. No performance rewrite is justified by these measurements.

`TestMaximumCatalogSeriesAndSize` fills all documented tool/outcome/gap combinations and both routes, verifies exactly 1,205 maximum series, validates backend wire decoding, and measured 112,644 bytes before shared HELP/TYPE deduplication and the backend-up family, well below the 1 MiB ceiling. Typical scrapes expose only observed vector combinations. `TestScrapeStormAllowsMCPProgress` completes ten successful MCP calls during 120 concurrent scrape attempts; overload responses are expected and independently admitted.

## Review and fixes

Round 1 used security, specification, adversarial, performance, maintainability and Go-idiom reviewers. Two distinct findings were verified:

- **Incomplete HTTP request bodies:** A write deadline alone does not bound Go's HTTP/1 request-body drain. Both gateway and backend telemetry now set read and write deadlines before response/rejection paths. A raw TCP regression with a withheld declared body verifies protected and anonymous gateway completion within the scrape budget.
- **Missing backend values:** Protobuf getters default absent values to zero. Decoding now requires explicit scalar pointers and rejects floating/native histogram variants. Malformed-family and HTTP partial-availability regressions verify backend-up zero and omission of backend samples.

The complete container suite passed after both fixes. Subsequent panel rounds are recorded below; final publication remains gated on exact-commit hosted CI. Shared-kernel container limitations remain the same as the existing validation guide; no VM or release was created.

Round 2 returned no actionable findings from all reviewers. The final matrix then passed all ten cases, including both systemd modes and ARM64, and full coverage reached 79.8%. The coordinator's cross-check against the existing operational-audit specification identified missing scrape authentication/authorization/backend-failure records. Structured records now carry request correlation and known identity without bearer material, with a JSON audit regression. Round 3 reviews that final addition and the complete result.

Round 3 returned no actionable findings from every panel expert. The final audit and downgrade regressions passed the complete container race/coverage suite at 79.8%. The repackaged final implementation passed all ten acceptance cases, archive checks, formatting, lint, packaging configuration checks and strict OpenSpec validation. No validation was waived; no further review round was needed.

## Contract verification

| Contract                                   | Implementation and observed verification                                                                                                                                    |
| ------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Configurable endpoint                      | Metrics defaults in `internal/config`; routing and atomic snapshots in `internal/gateway`; strict YAML/default/false, reload rollback and admitted-snapshot tests.          |
| Scrape authentication                      | Existing token verifier and explicit role membership; HTTP tests cover credentials, all roles, role updates, revocation and rotation expiry.                                |
| Service telemetry and partial availability | Component registries and private `/telemetry`; Prometheus parser tests, malformed/timeout fallback, real backend stop/restart and counter reset.                            |
| Bounded collection and minimization        | Separate flight admission, read/write deadlines, fixed catalog and bounded encoding; raw TCP, stalled-worker, maximum-series and sensitive-input regressions.               |
| Additive fixed roles                       | Existing diagnostic hierarchy unchanged; metrics-only discovery/direct-call rejection and every role union tested; inactive metadata changes cannot reactivate credentials. |
| TLS and explicit plaintext exposure        | Existing listener/TLS path retained; anonymous exception is route-local; default and anonymous proxy/MCP authentication verified.                                           |
| Proxy trust and auditing                   | Existing client-address resolver retained; protected routes still verify bearer state; structured scrape failure audit excludes secrets.                                    |

All 11 implementation tasks and all seven delta requirements are covered. The existing operational-audit contract is also preserved. The implementation follows the planned independent role, isolated registries, private transport, resource ceilings and absence of background diagnostic polling.
