# Network transport

## Purpose

Define the observable HostLens v1 network transport behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: One endpoint and multiple binds

The gateway SHALL expose one Streamable HTTP MCP endpoint per listener, with a list of IP bind addresses sharing port, TLS, and authentication settings. Defaults SHALL be [127.0.0.1] and port 8080. IPv4 and IPv6 SHALL use explicit separate family behavior. Duplicate or overlapping binds SHALL fail validation.

#### Scenario: Dual stack

- **WHEN** 0.0.0.0 and :: are configured with TLS
- **THEN** separate IPv4 and IPv6 listeners use the same configured port and policy

#### Scenario: Partial startup failure

- **WHEN** one address cannot bind after another succeeds
- **THEN** startup closes opened listeners and fails

#### Scenario: Overlap

- **WHEN** 0.0.0.0 and a specific IPv4 address share the port
- **THEN** validation rejects the overlap

### Requirement: TLS and explicit plaintext exposure

Native TLS SHALL be opt-in through certificate-chain and private-key paths. Non-loopback plaintext SHALL require allow_insecure_http true. Failure to load enabled TLS SHALL fail startup without HTTP fallback. Bearer authentication SHALL remain required for MCP behind proxies and on loopback. Metrics SHALL also require bearer authentication unless metrics.allow_anonymous is explicitly true; this exception SHALL apply only to the metrics endpoint. Certificate replacement SHALL require restart in v1.

#### Scenario: Default

- **WHEN** no networking overrides are configured
- **THEN** only loopback HTTP is served and bearer authentication is required

#### Scenario: Unsafe bind

- **WHEN** non-loopback HTTP is configured without explicit opt-in
- **THEN** startup fails

#### Scenario: Invalid key

- **WHEN** TLS is enabled with an unreadable or mismatched key
- **THEN** startup fails without plaintext fallback

### Requirement: Proxy trust and auditing

No proxies SHALL be trusted by default. Configured proxy IPs or CIDRs SHALL govern interpretation of the configured client-IP header, initially X-Forwarded-For. The immediate peer MUST be trusted before parsing the chain from right to left and choosing the first untrusted address. Invalid or indeterminate chains SHALL fall back to the peer. Peer and resolved client IP SHALL both be retained.

#### Scenario: Forged header

- **WHEN** an untrusted peer supplies X-Forwarded-For
- **THEN** the supplied client identity is ignored

#### Scenario: Trusted chain

- **WHEN** a trusted immediate proxy appends a valid forwarding chain
- **THEN** the resolved client is the first untrusted address from the right

#### Scenario: No authorization by address

- **WHEN** a trusted proxy forwards an MCP request or an authentication-required metrics request without a valid bearer token
- **THEN** authentication still fails

#### Scenario: Anonymous scraping does not weaken MCP

- **WHEN** metrics.allow_anonymous is true and a client sends requests without credentials
- **THEN** the enabled metrics endpoint permits scraping but the MCP endpoint still rejects unauthenticated access, including through trusted proxies

### Requirement: Transport hardening

HTTP parsing, request sizes, idle connections, and origin handling SHALL be bounded and conform to the selected MCP transport specification. The service SHALL document safe proxy header forwarding and the need to protect every non-loopback bearer-token hop.

#### Scenario: Oversized request

- **WHEN** a client exceeds the configured HTTP request ceiling
- **THEN** the server rejects it without unbounded buffering

#### Scenario: Proxy deployment

- **WHEN** TLS terminates on another machine
- **THEN** installation guidance identifies that the backend hop also needs protected transport or explicit risk acceptance

### Requirement: Bounded gateway processing

The gateway SHALL bound active MCP HTTP handlers before reading token state or constructing protocol handlers. Excess requests SHALL receive HTTP 503 without waiting for an admission slot. Accepted requests SHALL have a bounded context lifetime, including backend capability discovery.

#### Scenario: Authentication flood

- **WHEN** concurrent requests reach the configured operation ceiling
- **THEN** further requests receive an overload response without additional token-store reads or backend calls

#### Scenario: Backend stalls during discovery

- **WHEN** capability discovery does not complete before the request deadline
- **THEN** the request terminates and releases its gateway admission slot
