## 1. Configuration and authorization

- [x] 1.1 Add metrics defaults and atomic reload support; verify omitted settings, explicit false, malformed values, successful access transitions, failed reload rollback, and in-flight snapshot behavior in container tests.
- [x] 1.2 Add the independent metrics role to token validation and CLI help; verify every existing role alone and combined with metrics, existing-token expansion without secret replacement, revocation, expiry, and rotation overlap. Prove metrics alone cannot execute any tool or administrative operation.
- [x] 1.3 Add bounded metrics routing and authorization; verify 404, 405, 401, 403, and 503 behavior, anonymous requests with absent or invalid credentials, and unchanged MCP authentication behind proxies and on loopback.

## 2. Telemetry

- [x] 2.1 Verify a maintained Prometheus client against official release and compatibility information, record the dependency decision, and implement isolated bounded registries. Verify exposition parsing, content type, metric types, and duplicate-registration safety in container tests.
- [x] 2.2 Instrument request and tool outcomes with finite labels and fixed histogram buckets; verify success, authorization failure, overload, cancellation, timeout, partial results, unavailable evidence, and counter ownership without double counting. Verify arbitrary names and sensitive inputs cannot create series or leak into metrics.
- [x] 2.3 Add bounded private backend telemetry collection with existing peer authorization; verify normal aggregation, unavailable or malformed backend responses, output overflow, restart resets, and telemetry availability during diagnostic generation mismatch without bypassing diagnostic safeguards.
- [x] 2.4 Add available gateway/backend runtime measurements and enforce scrape deadlines, byte ceilings, cancellation, and independent admission. Verify unsupported measurements are omitted and stalled work cannot accumulate workers or consume MCP admission slots.

## 3. Integration and operator guidance

- [x] 3.1 Extend the established disposable-container integration workflow with authenticated and anonymous scrapes across reloads and backend restarts. Parse real HTTP exposition and verify successful MCP traffic during a bounded scrape storm; include the checks in existing CI entry points.
- [x] 3.2 Compare request throughput, latency, and allocations against the unchanged baseline and with instrumentation disabled. Record workload, concurrency, series count, output size, and measured overhead; investigate regressions before declaring validation complete.
- [x] 3.3 Update existing operator documentation and configuration examples with the complete metric catalog, role-update and scrape examples, anonymous exposure scope, limits, partial backend semantics, and downgrade steps. Verify examples against the container deployment and format the edited Markdown through the harness.
- [x] 3.4 Run the repository's required container test, race, lint, build, and specification checks. Record actual outcomes and environment limitations in the validation artifact; check off tasks only after their verification succeeds.
