## MODIFIED Requirements

### Requirement: TLS and explicit plaintext exposure

Native TLS SHALL be opt-in through certificate-chain and private-key paths. Non-loopback plaintext SHALL require allow_insecure_http true. Failure to load enabled TLS SHALL fail startup without HTTP fallback. Bearer authentication SHALL remain required for MCP behind proxies and on loopback. Metrics SHALL also require bearer authentication unless metrics.allow_anonymous is explicitly true; this exception SHALL apply only to the metrics endpoint. Certificate replacement SHALL require restart in v1. The Authorization header SHALL carry exactly one value whose auth-scheme matches `bearer` case-insensitively per RFC 6750, and the credential after the first space SHALL be verified against the token store regardless of scheme spelling.

#### Scenario: Default

- **WHEN** no networking overrides are configured
- **THEN** only loopback HTTP is served and bearer authentication is required

#### Scenario: Unsafe bind

- **WHEN** non-loopback HTTP is configured without explicit opt-in
- **THEN** startup fails

#### Scenario: Invalid key

- **WHEN** TLS is enabled with an unreadable or mismatched key
- **THEN** startup fails without plaintext fallback

#### Scenario: RFC 6750 scheme spellings

- **WHEN** a client sends `bearer <secret>`, `BEARER <secret>`, or `Bearer <secret>`
- **THEN** authentication proceeds identically for all three spellings

#### Scenario: Malformed credentials unchanged

- **WHEN** a request omits the header, sends multiple Authorization values, sends a non-bearer scheme, or sends an unverifiable credential
- **THEN** the response status codes and error classes match the pre-existing behavior