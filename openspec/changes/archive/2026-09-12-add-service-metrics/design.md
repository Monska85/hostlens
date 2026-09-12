## Context

The gateway currently routes only `/mcp`; it bounds admission before token verification. Token roles form a diagnostic hierarchy, and the backend exposes private operations through the existing local transport. See [proposal.md](proposal.md) for motivation.

## Goals / Non-Goals

Keep scrape authorization independent of diagnostic authority and telemetry independent of application adapters. Instrument existing work and collect service runtime state on demand.

This change does not add a host exporter, application monitoring, periodic collectors, dashboards, or another network listener.

## Decisions

### Separate authorization from the diagnostic hierarchy

Add `metrics` to recognized token roles and check explicit membership for scrapes. Do not assign it a numeric diagnostic level. Reuse token storage and lifecycle checks; a second credential mechanism would duplicate security-sensitive state.

Mixed roles retain their union. A metrics-only token may encounter normal MCP protocol discovery, but it must discover no executable tools and cannot run a cached or directly requested tool. It never grants local administrative access.

### Route metrics using an active configuration snapshot

Add a metrics configuration section with these defaults:

```yaml
metrics:
  enabled: true
  allow_anonymous: false
```

Route `/metrics` before MCP-specific handling. Check enabled state, method, bounded admission, and then authentication; only authorized or explicitly anonymous GET requests collect telemetry. Disabled responses take precedence over authentication, and anonymous mode ignores supplied bearer credentials.

Use the existing coordinated reload mechanism for both booleans. Keep listener and certificate changes restart-only. Existing configurations retain their diagnostic permissions and gain only a protected route.

### Use bounded component registries

Use separate registries owned by gateway and backend, with a fixed component label when combining their exposition. Prefer a maintained Prometheus Go client over a custom text serializer; verify its official release, compatibility, and dependency cost before selecting a version during implementation. Avoid global registration so tests and multiple coordinator instances cannot collide.

Use fixed histogram buckets and predefined tool/outcome labels. Count completed tool calls once at their owning execution boundary; count HTTP failures separately so transport rejection does not invent a tool execution. Keep gap reasons in a finite vocabulary derived from existing result envelopes.

Document the exact catalog during implementation and validate its series ceiling. Include only service process/runtime measurements that meet the data-minimization contract; no collectors that inspect other applications. Runtime collectors must respect future platform boundaries and omit unsupported measurements.

### Collect backend telemetry through the private transport

Add a narrowly scoped telemetry operation to the existing gateway-to-backend transport, retaining peer checks and socket permissions. It reads only the backend registry; do not call capability discovery or diagnostic operations, accept user-selected collectors, forward scraper credentials, or reuse the administrative interface.

Bound the response before decoding and reject malformed, duplicate, or unexpected metric families. Gateway telemetry remains available when this operation fails. The backend-up gauge reports telemetry reachability only; generation mismatch must continue to block diagnostics under existing rules without blocking service telemetry.

### Bound scrape work independently

Start with one concurrent scrape per gateway and one backend telemetry collection, a five-second total scrape deadline, a two-second backend deadline, and a 1 MiB exposition ceiling. These are implementation constants, documented and tested; do not add tuning knobs without evidence. Preserve the existing transport header limits and apply write deadlines to slow readers.

Acquire admission before authentication, retain it until underlying work ends, and cancel the backend request on client cancellation. Gather into bounded output before writing success headers. Do not cache stale backend samples, launch a goroutine per rejected request, or hold coordinator locks during collection.

Measure instrumentation overhead against the unchanged request path and measure MCP progress during a scrape storm. Dedicated admission limits shared CPU contention but cannot eliminate it, so benchmark evidence is required before claiming negligible impact.

## Risks / Trade-offs

- **Anonymous exposure.** Explicit opt-in makes service operational metadata public on every configured listener. Document this scope beside the option; no inspected content is exported.
- **Role regression.** Adding a name to hierarchical authorization could grant unintended tools. Test every role alone and in combinations, including direct calls to remembered tools.
- **Partial scrape semantics.** HTTP 200 means gateway exposition succeeded, not backend health. Document alerting on `hostlens_backend_up` separately from the scraper's target-up metric.
- **Dependency and runtime overhead.** Verify the chosen client and bound collector output. Compare request throughput and allocations with instrumentation enabled and disabled.

## Migration Plan

Update configuration examples and token-role help with implementation. Existing tokens keep their permissions; administrators create or update a dedicated scraper token to include metrics. Document an authenticated scrape example and the explicit anonymous alternative in the existing operator guide.

Rollback to an older binary requires removing the new configuration keys and removing metrics from token role sets using the newer CLI first. Do not weaken unknown-role or unknown-field validation for downgrade convenience.

## Dependency and implementation choices

Selected `github.com/prometheus/client_golang v1.24.1`, the official July 24, 2026 bugfix release. Its Go 1.25 minimum fits the existing Go 1.27 toolchain. Sources: [official release](https://github.com/prometheus/client_golang/releases/tag/v1.24.1), [module manifest](https://github.com/prometheus/client_golang/blob/v1.24.1/go.mod), [package registry](https://pkg.go.dev/github.com/prometheus/client_golang@v1.24.1/prometheus), and [security model](https://prometheus.io/docs/operating/security/). The security guidance motivates protected access and independent bounded admission; no default HTTP handler or global registry is exposed.

Direct schema/exposition imports use the client's required `client_model v0.6.2` and `common v0.70.1`. The selected graph requires compatible `x/oauth2 v0.36.0` and `x/sync v0.21.0`; other existing direct dependencies remain unchanged. Manifests and checksums move together. The repository container vulnerability scan reported no vulnerabilities after resolution. Packaging includes the resulting dependency notices.

A narrow runtime collector reads fixed Go runtime samples and Linux self `getrusage` values on demand. Unsupported samples are omitted. Backend telemetry uses size-limited structured metric families over the existing peer-checked socket; the gateway validates family/type/label/bucket vocabularies and duplicate series before exposition. Public output uses the maintained Prometheus serializer. The catalog and component counter ownership are documented in the existing operator guide.
