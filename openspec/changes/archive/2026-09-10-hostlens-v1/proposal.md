# HostLens v1 proposal

## Why

Administrators need to let MCP clients inspect a small Linux fleet without handing agents unrestricted shell access or centralizing server logs. HostLens provides a reusable per-host diagnostic interface for interactive reasoning and externally scheduled checks, with explicit data-access boundaries.

## What Changes

- **Diagnostics:** Add eight bounded, fact-based tools through one named MCP connection per host.
- **Separation:** Use an unprivileged Go MCP gateway and a separately supervised diagnostic backend with enforced local policy.
- **Access control:** Add bearer-token management, additive roles, YAML profiles, global denials, and policy explanations.
- **Operation:** Support explicit listeners, optional native TLS, trusted proxies, atomic policy reloads, and structured auditing.
- **Lifecycle:** Distribute Linux amd64 and arm64 archives with standard and restricted installation modes, tracked uninstall, and recoverable upgrades.
- **Portability:** Isolate platform collectors, path handling, IPC, identities, and service management for future systems.

## Capabilities

### New Capabilities

- **`platform-runtime`:** Go component boundaries, local IPC, Linux capability detection, and future platform support.
- **`mcp-diagnostics`:** Tool contracts, honest observations, bounded responses, and on-demand execution.
- **`health-assessment`:** Current host measurements, configurable checks, and incomplete-health reporting.
- **`access-policy`:** Typed YAML profiles, includes, file and journal rules, precedence, and safe path access.
- **`configuration-lifecycle`:** Validation, reload, effective-policy fingerprints, and policy explanation.
- **`token-authorization`:** Bearer-token lifecycle, roles, local administration, and per-request enforcement.
- **`network-transport`:** Multiple listeners, TLS, plaintext opt-in, and trusted-proxy handling.
- **`operational-audit`:** Structured logging with correlated requests and sensitive-data exclusion.
- **`installation-lifecycle`:** Archives, accounts, two privilege modes, install manifest, uninstall, and upgrades.
- **`release-validation`:** Representative platform tests, security checks, and release acceptance.

### Modified Capabilities

None. This change introduced the initial implementation and specifications.

## Impact

The implementation introduces Go executables and Linux systemd integration. The standard diagnostic mode grants broad OS read privilege to the backend and must disclose its confidentiality tradeoff; the restricted mode relies on existing permissions and explicit administrator grants.

The selected module is `github.com/Monska85/hostlens`. Implementation and local validation are authorized; repository visibility, project license, and publication remain deferred. This session creates no commits, remote, or published release.

## Non-goals

V1 excludes remediation execution, macOS and Windows collectors, OAuth, background scheduling, metric-history storage, automatic updates, certificate issuance, and native distribution packages. It does not claim that host health proves application health or that a read-capable backend is immune to compromise.
