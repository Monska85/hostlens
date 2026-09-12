## MODIFIED Requirements

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
