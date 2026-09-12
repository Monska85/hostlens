# Product and architecture

HostLens exposes controlled, on-demand host diagnostics to independently authorized MCP clients. [Current OpenSpec requirements](../../openspec/specs) define the contract; this page explains the product and maps its implementation. Archived changes retain historical decisions, not current completion status.

## Product boundary

V1 supports Linux amd64 and arm64, using capability detection rather than distribution-version gates. Debian, Ubuntu, and Arch are representative environments. Installation uses systemd; missing collection interfaces produce explicit limitations.

Clients own scheduling, notifications, model calls, and retained monitoring history. HostLens performs no remediation, arbitrary shell execution, background monitoring, or automatic updates. macOS, Windows, OAuth, native packages, and separately authorized remediation remain future capabilities.

## Components and trust

| Component   | Responsibility                                                                    | Boundary                                                                    |
| ----------- | --------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| Gateway     | Streamable HTTP, bearer verification, roles, MCP dispatch, audit, service metrics | Unprivileged service identity; access to token state and TLS key            |
| Diagnostics | Source authorization, collection, assessment, budgets                             | Separate identity; optional standard-mode read capability                   |
| Local CLI   | Tokens, policy explanation, reload/status, installation and upgrades              | Local administrator or owning-user authority; never granted by an MCP token |

Gateway and backend communicate through structured local IPC with peer-identity checks. Each call carries a generation; the backend rejects inconsistent generations. Source policies are immutable snapshots, and status reports their cached fingerprint and generation together.

Reload validates both components before activating new policy, assessment settings, and limits. Listener, TLS, credential-location, identity, and privilege changes require restart. In-flight calls may finish under their original generation. An unresolved restart mismatch fails closed.

Standard mode grants only `CAP_DAC_READ_SEARCH` to diagnostics through systemd. Restricted mode grants no additional capability. Both enforce policy, isolate configured gateway secrets, and constrain writes, devices, and networking; neither profiles nor read-only mounts fully contain a compromised broad-read process.

## Tool contract

| Minimum role  | Tools                                                                             |
| ------------- | --------------------------------------------------------------------------------- |
| `health`      | `get_os_info`, `get_inventory`, `get_health_snapshot`                             |
| `inspect`     | Health tools plus `list_services`, `get_service_status`, `list_packages`          |
| `diagnostics` | Inspect tools plus `read_config`, `query_logs` and explicitly granted audit tools |

The independent `metrics` role authorizes only service scrapes. Diagnostic roles do not inherit it. See [service metrics](OPERATIONS.md#service-metrics) for the catalog and access controls.

Token expiry, revocation, and role changes apply to subsequent requests and are checked again at tool execution. Discovery is not authorization. Tokens cannot administer the host or change policy.

Return observed values only. Missing optional measurements are omitted; failed collection produces structured issues alongside valid partial observations. Fatal execution failures set MCP `isError`. Health severity and coverage completeness are independent, so incomplete coverage cannot establish health.

Service and package lists paginate without claiming snapshot consistency. File configuration reads return complete bounded UTF-8 content or fail. Log queries select one source and report parser, time window, ordering, truncation, and retention limitations. Raw tails establish no event-time coverage. Log and configuration content is always untrusted data.

## Source policy and budgets

Profiles combine typed file, journal and audit-domain rules with transitive includes. Only referenced profiles activate. Every active denial and mandatory exclusion overrides allows; unlisted sources are denied except narrowly defined built-in observations.

General reads check requested and descriptor-resolved paths, reject symlinks and special files, and cannot retrieve pseudo-filesystems. Built-in observations permit ordinary OS symlinks within their root but reject denied targets. Multiply linked regular files are rejected so masked secrets cannot escape protection through hard-link aliases.

Gateway admission precedes token reads and MCP setup. Backend admission separately bounds unfinished collection work. Size, entry, time, and concurrency limits apply without silently returning fabricated observations. Kernel-blocked I/O retains its slot until it ends; see [recovery guidance](OPERATIONS.md#diagnostic-formats-and-limits).

## Generic audit evidence

Audit tools expose application-independent process, network, account, storage, maintenance, security-control, service, path and HostLens evidence. Diagnostics role and an explicit audit-domain grant are both required; ordinary file/journal grants do not activate audit domains. Existing source denials and mandatory protected objects remain effective.

Collectors retain the existing OS privilege model and do not execute application commands or SQL. Native filesystem reads share an aggregate per-call inspection budget, including failed oversized reads. Bounded PID enumeration precedes page-specific process collection. Process arguments and environments are omitted; selected service properties exclude secret-bearing execution settings.

See [audit tools and evidence limits](OPERATIONS.md#generic-server-and-application-audits) for the operator contract. The agent supplies application interpretation; returned coverage never certifies overall production readiness.

## Platform extension points

Portable protocol contracts, backend coordination, token records, and role evaluation remain independent of Linux process APIs. The gateway receives a token verifier; Linux supplies file-backed verification and locking. Collectors implement the shared collection interface, while the executable composition wires native IPC and lifecycle behavior.

Linux file/journal semantics and defaults are explicitly Linux-specific. A future platform must supply its own path rules, safe handles, credential persistence, service identities, and collection sources. Windows reparse points, drive/UNC paths, named pipes, and event logs must not inherit Linux assumptions. macOS similarly needs native filesystem, launchd, and privilege handling.

Do not add unused abstractions to anticipate these implementations. Share behavior only where its meaning is platform-independent. Future remediation requires a separately authorized process and operation contract; diagnostic roles must not acquire write authority implicitly.

## Requirement map

| Concern                                 | Authoritative specification                                                     |
| --------------------------------------- | ------------------------------------------------------------------------------- |
| Platforms and process separation        | [Platform runtime](../../openspec/specs/platform-runtime/spec.md)               |
| Tool arguments and information fidelity | [MCP diagnostics](../../openspec/specs/mcp-diagnostics/spec.md)                 |
| Health severity and coverage            | [Health assessment](../../openspec/specs/health-assessment/spec.md)             |
| Profiles and safe source access         | [Access policy](../../openspec/specs/access-policy/spec.md)                     |
| Reload and policy explanation           | [Configuration lifecycle](../../openspec/specs/configuration-lifecycle/spec.md) |
| Credentials and roles                   | [Token authorization](../../openspec/specs/token-authorization/spec.md)         |
| HTTP, TLS, proxies, and admission       | [Network transport](../../openspec/specs/network-transport/spec.md)             |
| Service scrape access and bounds        | [Service metrics](../../openspec/specs/service-metrics/spec.md)                 |
| Operational metadata                    | [Operational audit](../../openspec/specs/operational-audit/spec.md)             |
| Install, upgrade, and removal           | [Installation lifecycle](../../openspec/specs/installation-lifecycle/spec.md)   |
| Tests, artifacts, and releases          | [Release validation](../../openspec/specs/release-validation/spec.md)           |

Use [operator instructions](OPERATIONS.md) for deployment, [release instructions](../RELEASING.md) for contributor validation, and [validation evidence](VALIDATION.md) for results and limitations.
