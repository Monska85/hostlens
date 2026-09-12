## Why

Operators need to measure HostLens availability, request latency, failures, and resource use without granting diagnostic access. A scrape endpoint provides service telemetry through existing monitoring infrastructure.

## What Changes

- **Endpoint.** Add `GET /metrics` to the existing HTTP listeners, enabled by default and configurable off.
- **Authorization.** Require a bearer token with the independent `metrics` role by default. This role grants no MCP tool, administrative, or remediation permissions; other roles do not inherit it.
- **Anonymous access.** Allow an explicit `metrics.allow_anonymous: true` exception for scraping only. Preserve MCP authentication and transport safeguards.
- **Telemetry.** Expose bounded service counters, latency, concurrency, and gateway/backend runtime metrics. Report backend collection failures without fabricated observations.

## Capabilities

### New Capabilities

- **`service-metrics`.** Endpoint configuration, scrape behavior, telemetry scope, and resource bounds.

### Modified Capabilities

- **`token-authorization`.** Add the independent `metrics` role to existing token administration and request authorization.
- **`network-transport`.** Scope the optional anonymous exception to metrics while retaining MCP authentication behind proxies and on loopback.

## Impact

Changes affect configuration defaults, gateway routing, token validation, backend telemetry transport, and service instrumentation. Implementation must update operator documentation and container tests and verify any metrics dependency before adoption. No new listener, OS privileges, application adapter, background host polling, or monitoring deployment is required.
