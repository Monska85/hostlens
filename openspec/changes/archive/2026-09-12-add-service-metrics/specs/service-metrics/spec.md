## Purpose

Expose bounded HostLens service telemetry for monitoring while keeping metrics access separate from diagnostic and administrative authority.

## ADDED Requirements

### Requirement: Configurable scrape endpoint

HostLens SHALL expose `GET /metrics` on its existing HTTP listeners using their bind and TLS settings. Configuration SHALL default to `metrics.enabled: true` and `metrics.allow_anonymous: false`, including when the metrics section is omitted. Disabling metrics SHALL return HTTP 404 without collecting telemetry. Unsupported methods on an enabled endpoint SHALL return HTTP 405 and advertise GET.

Both settings SHALL use the existing validated atomic reload lifecycle. New requests SHALL use the activated settings; failed reloads SHALL retain the previous settings. Already admitted scrapes MAY finish under their original configuration snapshot.

#### Scenario: Existing configuration

- **WHEN** HostLens starts with a valid configuration that omits metrics settings
- **THEN** the metrics endpoint is enabled and requires an authorized bearer token

#### Scenario: Disabled endpoint

- **WHEN** metrics.enabled is false, regardless of allow_anonymous
- **THEN** requests to /metrics return HTTP 404 and do not initiate telemetry collection

#### Scenario: Unsupported method

- **WHEN** a client sends POST to an enabled /metrics endpoint
- **THEN** it receives HTTP 405 with Allow: GET and no telemetry

#### Scenario: Atomic access change

- **WHEN** a successful reload changes allow_anonymous from true to false
- **THEN** newly admitted scrapes require a metrics token, and a later invalid reload does not restore anonymous access

### Requirement: Scrape authentication

Unless anonymous scraping is explicitly enabled, each scrape SHALL verify a current bearer token through the existing token store and require its metrics role before collecting telemetry. Missing, malformed, expired, or revoked credentials SHALL return HTTP 401; valid tokens without the role SHALL return HTTP 403. Anonymous mode SHALL permit scraping without interpreting bearer credentials and SHALL NOT change authorization on other routes. Neither mode SHALL expose token material in responses or telemetry.

#### Scenario: Missing credential

- **WHEN** a client scrapes without a bearer token under default settings
- **THEN** it receives HTTP 401 and no telemetry is collected

#### Scenario: Authorized scraper

- **WHEN** a valid unexpired token has the metrics role
- **THEN** the enabled endpoint returns a scrape without granting any additional permissions

#### Scenario: Public scrape

- **WHEN** allow_anonymous is explicitly true
- **THEN** GET /metrics does not require a token, including when an invalid Authorization header is supplied

### Requirement: Service telemetry and partial availability

Successful scrapes SHALL return Prometheus-compatible metrics with the corresponding content type. Telemetry SHALL cover service request counts and latency, active work, authorization failures, overloads, tool outcomes, and collection gaps, plus available runtime and resource metrics for HostLens gateway and backend processes. It SHALL distinguish the two components and document metric names, types, units, labels, histogram boundaries, and reset behavior.

A failed, timed-out, or malformed backend telemetry response SHALL leave available gateway telemetry scrapeable with HTTP 200, set `hostlens_backend_up` to 0, and omit unavailable backend observations. Successful backend telemetry collection SHALL set this gauge to 1; it SHALL NOT claim diagnostic capability or generation readiness. Unsupported process measurements SHALL be omitted and documented rather than fabricated. Counters SHALL reflect observations within their owning process lifetime and SHALL NOT silently persist across restarts.

#### Scenario: Normal scrape

- **WHEN** gateway and backend telemetry are available
- **THEN** the response parses as Prometheus metrics, distinguishes the components, and reports hostlens_backend_up as 1

#### Scenario: Backend unavailable

- **WHEN** the backend cannot supply a valid telemetry response within the scrape deadline
- **THEN** the scrape contains gateway telemetry and hostlens_backend_up 0 without stale backend samples or substitute zero values

#### Scenario: Tool evidence gap

- **WHEN** a diagnostic result reports partial or unavailable evidence
- **THEN** bounded outcome and gap metrics distinguish that result from complete collection without including inspected data

#### Scenario: Component restart

- **WHEN** the backend restarts while the gateway remains running
- **THEN** backend counters restart for the new process while gateway counters retain their lifetime values

### Requirement: Bounded collection and data minimization

Scraping SHALL measure HostLens itself without executing diagnostic tools, scanning inspected application data, launching subprocesses, or adding periodic host polling. It SHALL NOT require additional OS privileges. Labels SHALL use finite documented vocabularies; arbitrary tool names SHALL map to a bounded unknown value. Credentials, token identities, client addresses, hostnames, filesystem paths, application names, process IDs, command lines, inspected content, and raw errors SHALL NOT become labels or metric values.

Scrape admission SHALL be bounded before token-store reads and backend calls, separately from MCP admission. Excess scrapes SHALL return HTTP 503 without queuing. Collection and response writes SHALL have finite deadlines and byte ceilings; cancellation SHALL not leak workers or permit unbounded background collection. Oversized or failed gateway exposition SHALL return a non-success response rather than a successful truncated scrape. Instrumentation SHALL not perform network or filesystem I/O on the request hot path beyond work already required by the operation.

#### Scenario: Scrape storm

- **WHEN** scraping exceeds the admission ceiling
- **THEN** excess requests fail promptly with HTTP 503 without new token reads or backend work, and scrapes do not consume MCP admission slots

#### Scenario: Untrusted label input

- **WHEN** requests contain many distinct unknown tool names or sensitive arguments
- **THEN** neither the series count nor metric contents incorporate those values

#### Scenario: Cancellation and output overflow

- **WHEN** a scraper disconnects, collection stalls, or exposition exceeds its byte ceiling
- **THEN** bounded work terminates or retains its admission until it actually ends, and no successful truncated exposition is returned
