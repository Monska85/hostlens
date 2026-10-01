# Review record

## Baseline

Production Go under `internal/` is 12,669 lines; Go tests are 14,376 lines. The previous pass measured 86.8% internal statement coverage. This change begins from commit `e6aa342` with a clean working tree.

The prior snapshot binaries measure 21.70 MB (gateway), 19.24 MB (diagnostics), and 12.32 MB (observer) on amd64; the arm64 counterparts measure 20.35 MB, 17.95 MB, and 11.45 MB. Binary size is a secondary regression signal because linker and toolchain details also affect it.

The new `make benchmark` and `just benchmark` run inside the repository's disposable, networkless checks container. Five 200 ms samples on Linux amd64, Go 1.27.0, and an AMD Ryzen 9 PRO 7940HS gave these approximate medians:

| Workload                                | Time per operation | Bytes per operation | Allocations |
| --------------------------------------- | -----------------: | ------------------: | ----------: |
| Backend status, 10,000 rules            |             2.1 µs |               2,048 |          15 |
| Gateway `tools/list`, metrics off       |             224 µs |             414,846 |         797 |
| Gateway `tools/list`, metrics on        |             226 µs |             414,989 |         799 |
| Policy decision, 10,000 unique patterns |             308 µs |                   0 |           0 |
| Docker container inventory, 5,000 items |            2.22 ms |           2,627,256 |      20,036 |
| Docker image analysis, 5,000 items      |            3.10 ms |           3,798,804 |      20,202 |

These are controlled microbenchmarks, not production throughput or tail latency. They provide like-for-like regression fixtures for this refactor.

## Deletion and risk map

- Gateway: each HTTP request constructs a fresh MCP server, middleware, and tool registrations. Measure the cost of registration and schema serialization before changing the stateless request flow; any reuse must keep token revocation and role checks live.
- Policy: 10,000 unique patterns consume about 308 µs per access decision with zero allocations. Consider indexing only if denial precedence and pattern semantics remain exact.
- Docker: `docker_tools_linux.go` has 891 lines, including repeated image, volume, and network correlation and response handling. Consolidate only their common bounds and issue semantics; keep resource-specific authorization and incomplete-reference behavior visible.
- General Linux collection: `collect_linux.go` has 787 lines, with service, package, log, and file flows. Split or delete duplicated validation and paging work without replacing source-specific evidence gaps with generic success.
- Lifecycle: `lifecycle_linux.go` has 803 lines, with installation, removal, and upgrade transactions. Seek repeated resource tracking and rollback patterns, preserving the manifest's crash-recovery states.
- Typed payload files are large because stable result schemas are explicit. Do not replace them with maps or reflection solely to hit a line target.
- Backend: admission, bounded worker ownership, and generation activation live in `backend`; simplify duplicate IPC decoding only when cancellation and generation tests still show one bounded worker per admitted request.
- Contract and Docker contract: registry schemas, effect classes, protocol ceilings, and typed payloads are compatibility boundaries. Reduce only redundant projection work and helpers; keep every published JSON name and read-only operation fixed.
- Observer: private fixed GET path and query tables, per-connection socket validation, endpoint byte ceilings, and typed decoding are the authority boundary. Eliminate repeated parsing without opening generic request construction or retaining observations.
- Configuration and token: strict YAML/JSON parsing and trusted file checks belong in their existing packages; reuse the bounded `jsondoc` decoder for the token store instead of maintaining a second decoder.
- CLI and lifecycle: CLI dispatch should stay thin; lifecycle resource ownership, intent records, rollback, and safe uninstall checks are coupled. Remove repetition only with transaction tests, and do not turn a host lifecycle operation into a local test.
- Telemetry: bounded aggregate metrics and payload-free audit data are allowed retained state. Avoid caching diagnostic results while optimizing hot paths.
- Diagnostics app: keep Linux privilege and socket ownership in the native process entry point; no cross-platform runtime adapter belongs in this change. JSON document decoding: keep one bounded decoder for complete documents and reuse it at every compatible trust boundary.

## Round 1: collector and token paths

The first change moves Docker derived reference fields into page projection so the collector no longer allocates them for every filtered or off-page item. Docker policy denial forms are built only when an active deny exists, and the broad collection grant is checked before item-specific allow forms. The token store now reuses the bounded strict JSON decoder.

The complete disposable container race, vet, and Linux/portable build suite passed after these changes. Five-sample benchmarks on the same fixture show the container case at about 0.75 ms, 1,588,856 B, and 36 allocations; the image case at about 2.23 ms, 2,919,312 B, and 266 allocations. Gateway service requests remained about 225–260 µs and 415 kB, while the 10,000-rule policy decision remained about 310 µs. These measurements support the collector gain; they do not establish production throughput or tail latency.

## Round 2: projection and native parsing

The observer's image list and disk-usage paths now share one typed projection. This surfaced an existing inconsistency: untagged disk-usage images did not carry the dangling fact. The OpenSpec delta makes that scenario explicit, with observer and collector regression checks for stable and changing references. Native network parsers now parse ordered numeric fields into fixed arrays before building typed rows, removing repetitive string-key switches while retaining validation bounds and error classes.

## Round 3: process link memory

Process inspection now passes one request-owned read buffer across executable and descriptor link observations. The 200-link container benchmark moved from about 344 µs, 979,203 B, and 800 allocations to about 219 µs, 6,400 B, and 600 allocations. The returned target remains a separate string, and a regression test checks that a shorter second target cannot expose bytes from the first. The buffer is not cached or retained after the worker exits.

## Round 4: strict decoder review

The token store and backend telemetry both had local bounded JSON decoder implementations. They now call the shared strict decoder, which reads one complete document, rejects unknown fields, and counts trailing whitespace within the byte ceiling. Existing malformed, oversized, and trailing-document tests remain the acceptance checks. A policy regression now checks that a collection grant cannot bypass a denied Docker alias after the optimized grant ordering.

The current production Go total is 12,601 lines before the remaining review rounds, a reduction of only 68 lines from the 12,669-line baseline. The 15% target remains unmet; do not mark the change complete on benchmark gains alone.

## Round 5: authority and lifecycle state

Adversarial tracing found a second Docker GET constructor in the streaming logs path. Buffered and streaming observations now use one constructor for the fixed method, Host, and User-Agent headers; the log path also comes from the private typed endpoint table. No generic method or arbitrary path enters the observer IPC.

Lifecycle manifest loading previously read an unbounded file and decoded its first JSON document before install-state decisions. It now applies a 16 MiB complete-document ceiling before lifecycle mutation. The OpenSpec delta and a disposable fixture cover trailing and oversized state, including the absence of command calls on rejection. The existing install, upgrade, rollback, and uninstall matrix remains the compatibility gate.

## Round 6: future mutation boundary

The current typed registry allowed a hypothetical remediation-effect tool to declare a diagnostic role. The backend still rejected that call, but discovery could have presented the wrong authority model if future code added the tool. Registry validation now requires a reserved remediation role for remediation effects and rejects that role on diagnostic effects. V1 token issuance still rejects the reserved role, and the diagnostic backend still has no mutation operation. This keeps the future role distinct without claiming remediation support now.

## Round 7: archive input and gateway hot path

The gateway `tools/list` fixture still allocates about 415 kB per request. Source tracing through the SDK showed schema serialization and per-request server construction. Reusing a server would retain request identity and capability observations beyond the worker and require a new revocation model, so this round keeps the current stateless path and records the measured cost rather than hiding it.

Release extraction accepted a 64 MiB archive member for `release.json` and read the whole member before decoding the first document. A 64 KiB complete-document ceiling now applies before its checksum map is trusted. Disposable archive fixtures cover malformed, trailing, and oversized manifests; exact final archive checks remain pending.

## Rounds

Record each full development and review round here with concrete findings, fixes, measured deltas, and validation. Leave incomplete work visible.

## Round 8: complete-product verification

The last code pass removed repeated mount-field decoders and rewrote Docker's two-attempt correlation as one bounded loop. Diff review checked that every before/after inventory is still observed live, that truncation suppresses unused conclusions, and that a failed observer call still fails the tool. Three distinct adversarial stresses exercised malformed and oversized manifests before mutation, changing Docker names and references during observation, and cancellation or exhausted read budgets. No in-scope regression was found in those paths.

Production Go is 12,601 lines, including 11,389 nonblank, noncomment rows. That is 68 lines and 73 code rows below the 12,669-line starting commit, but 87 lines and 61 code rows above `origin/main`. The earlier hardening pass added the difference. The initial 15% stretch target is missed. Removing typed public schemas, trust checks, or source-specific evidence handling to reach it would increase risk, so the final artifact reports the miss instead of presenting code growth as a reduction from main.

The final source passed container race tests, vet, Linux builds, portable macOS/Windows package builds, formatting, lint, strict OpenSpec validation, and a Go vulnerability scan with no findings. Container coverage was 86.9% of internal statements. The coherent amd64 and arm64 archives passed checksum and shipped-link verification, and all 12 disposable archive acceptance cases passed. The arm64 platform smoke runs under emulation; systemd matrix cases use the Docker engine's native architecture. The matrix tests Docker-absent lifecycle and the unit suite uses a bounded fake observer; it does not prove behavior against a live Docker daemon. No host lifecycle operation was run.

The final isolated five-sample benchmark medians were 0.768 ms, 1,588,856 B and 36 allocations for 5,000 containers; 2.34 ms, 2,919,312 B and 266 allocations for 5,000 images; and 0.209 ms, 6,400 B and 600 allocations for 200 process links. The backend status, policy decision, and gateway `tools/list` fixtures stayed near their baseline costs. These measurements establish the fixture improvement only, not production tail latency.
