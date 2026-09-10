# HostLens v1 design

## Context

See [the proposal](proposal.md) for motivation and scope. The initial implementation candidate follows these requirements. [Validation evidence](../../../../docs/v1/VALIDATION.md) records executed checks and their limits.

The first deployment contains four to six Linux hosts reachable from the administrator's laptop. Each host provides a named MCP connection to clients such as Codex or a separately scheduled agent application. Clients may send selected diagnostic results to an external model provider; HostLens does not call that provider.

## Goals / Non-Goals

**Goals:** Keep the network endpoint unprivileged, enforce source policy at the reader, and make installation practical without requiring per-file ACL maintenance in standard mode. Preserve interfaces that allow future platforms without treating Linux capabilities as portable.

**Non-goals:** An AI runtime, scheduler, centralized log store, incident notification service, remediation implementation, or a universal sandbox. V1 does not promise that a compromised read-capable backend cannot disclose information.

## Decisions

### Go and two executables

Use Go and the official Go MCP SDK. Produce `hostlens` for gateway and administrative CLI operations, and `hostlens-diagnostics` for the local backend. Prefer dependencies compatible with pure-Go builds to simplify amd64 and arm64 archives; verify resulting linkage and document native dependencies rather than assuming static portability.

Keep token management and network authentication out of the diagnostic executable. Keep privileged collectors out of the gateway. Both share versioned request and response types, validation primitives, and policy definitions.

Python and TypeScript would add runtime deployment concerns across hosts. A single privileged MCP process would enlarge the trusted network surface. The selected split supports later addition of a separate remediation executable without changing the host's MCP address.

### Platform boundaries

Separate OS collectors, path resolution, IPC, identity provisioning, service management, and administration triggers from protocol and policy logic. Linux uses Unix sockets and systemd; future macOS can use Unix sockets and launchd, and Windows can use named pipes and service identities.

Windows path matching must account for drive paths, UNC paths, case behavior, and reparse points. Do not reuse Linux string normalization as a security policy for Windows. Future Windows event selectors have channel, provider, and event-ID semantics; they are not renamed journal units.

Detect interfaces at runtime. Parse OS identity from established sources, use Linux resource interfaces, and select dpkg or pacman collectors when available. A missing interface yields unavailable capability or a collection issue, not fabricated values or a distribution-version rejection.

### Runtime request boundary

The gateway receives Streamable HTTP requests at `/mcp`. It authenticates every request, checks current token roles, and exposes only permitted available tools. A cached tool name never bypasses authorization.

The gateway sends a typed operation, request identifier, and policy generation to the diagnostic backend through `/run/hostlens/diagnostics.sock`. Socket permissions and peer checks restrict callers. The backend accepts no general command, arbitrary executable, or caller-supplied policy override.

Basic observation collectors have documented built-in grants for narrowly selected facts; explicit source denials still win. Those grants do not permit raw file retrieval, and service status must strip embedded journal bodies, environment secrets, and configuration contents.

The backend independently applies its active source policy and resource budgets. It owns the final decision before opening a file or querying a journal unit. Backend disconnection yields an explicit unavailable result.

Keep a separate restricted local administrative control surface for token mutations, active-policy status, and reload coordination. MCP bearer access cannot invoke these operations. An operator's OS identity, not a model-generated identity claim, authorizes local administration.

### Source access and privilege modes

Both modes use dedicated non-login `hostlens-gateway` and `hostlens-diagnostics` Linux users. The installer manages creation and records whether an identity was pre-existing. The gateway needs read access to token verification state and its configured TLS key; diagnostics must not receive either through normal IPC.

Standard mode grants `CAP_DAC_READ_SEARCH` only to the diagnostic service through systemd. This bypasses ordinary read/search checks without granting a general write-permission bypass. It is broad read privilege, not a path-scoped capability, and does not override all mandatory access controls.

Restricted mode grants no added capability. Administrators can grant targeted read access or reader-group membership; permission failures stay visible. Manually changed ACLs and rotation rules remain administrator-owned unless explicitly managed and recorded by HostLens.

Use a read-only filesystem view, protected secret paths, device restrictions, bounded syscalls where compatible, and network restrictions for the diagnostic process. Required isolation of configured token state and TLS keys must succeed before standard mode starts, including secret paths outside default directories. Ordinary owner permissions alone cannot hide the token store from `CAP_DAC_READ_SEARCH`; the standard-mode service must provide an additional inaccessible view of credential paths. Unix sockets and other side-effect interfaces also need restriction, since read-only mounts do not disable them.

Do not add a root shell fallback when containment blocks a collector. Collectors needing commands must use fixed executables and validated argument arrays without a shell. Validate the systemd sandbox, credential isolation, and journald access in disposable containers. Record unsupported containment checks explicitly. A VM is allowed only when strictly necessary and after explicit user confirmation before provisioning, downloading, or starting it; never test privileged lifecycle on the administrator host.

### Typed profiles and matching

Profiles contain `profiles`, `allow`, and `deny`; the latter two contain `files` and `journal`. Main configuration supports the same rules plus configured profile directories. See [configuration examples](examples.md).

Profile names resolve only within configured local directories. Preserve directory order for a possible future explicit precedence feature, but reject duplicate names in v1. Reject cycles, missing referenced profiles, malformed patterns, unsafe references, and unsupported active categories.

Resolve the inclusion graph once per configuration generation. Deduplicate patterns for matching while retaining every origin for explanation. An active denial from any source wins over every allow; mandatory secret exclusions cannot be disabled. Unreferenced installed profiles remain inactive.

Use path-aware glob semantics. `*` and `?` do not cross separators; `**` enables recursion. Journal globs match unit names and never accept arbitrary journal expressions. The optional allow-all profile covers file paths only unless an administrator separately enables broad journal access.

Filesystem checks must bind policy evaluation to the opened object. Avoid check-then-open string validation, resolve symlinks safely, and validate both requested and resolved paths. Use safe descriptor-relative operations supported by the declared kernel baseline; if equivalent safety is unavailable, fail the affected operation rather than fall back to unsafe traversal.

Hard links and bind mounts can give an object multiple names. V1 must protect known secret objects through service containment and source restrictions, and document that a path policy is not a full information-flow policy. General readers reject devices, sockets, FIFOs, and unrestricted pseudo-filesystem access; resource collectors expose only defined observations.

### Reload and administrative explanation

Treat each resolved policy and its limits as an immutable generation. A Linux `SIGHUP` reaches the gateway coordinator, which validates the complete candidate and asks the backend to prepare it before activation. Serialize activation with admission of new diagnostic operations; mismatched generations are rejected rather than executed permissively.

If preparation or activation fails, keep the previous generation consistently active or temporarily reject new work until consistency is restored. Already admitted operations may finish under their original generation. Token changes are separately atomic and apply on the next request. After an isolated backend restart, the gateway restores its validated active generation before admission; if that cannot be done safely, operations remain unavailable. A complete service restart validates and loads disk configuration. Administrative status reads fingerprint and generation from one consistent snapshot.

Listeners, TLS material, identities, and privilege mode require restart. A reload candidate containing restart-only differences is rejected as a whole with restart guidance. This avoids partially applying a file that administrators expect to be active in full.

Compute the policy fingerprint from canonical effective access policy, mandatory exclusions, and evaluator version. Exclude comments, source ordering with no semantic effect, and secrets. This fingerprint proves policy equivalence only; it is not a fingerprint of certificates, listener settings, or every configuration field.

`hostlens policy explain PATH` evaluates disk configuration and compares against an explicitly identified local instance. Always print MATCH, DIFFERENT, or UNKNOWN, with the disk configuration path and live instance identity when available. Show matched rules and inclusion provenance, and distinguish inactive matches. A missing path can receive a lexical policy analysis but must be marked unresolved; it cannot be reported as a successful safe read.

Directory explanation is not a blanket decision for descendants. Optional recursive inspection evaluates existing entries with limits and reports truncation. Source policy and OS permissions remain separate checks even under MATCH.

### Token storage and roles

Use high-entropy opaque bearer secrets, public IDs, and server-side metadata. Store only hashes, names, roles, expiration, and lifecycle state. A protected local store with atomic updates avoids embedding roles in immutable tokens. Use constant-time secret verification and never print secrets after creation.

The CLI supports `token create`, `token list`, `token update`, `token revoke`, and `token rotate`. Syntax is proposed until CLI implementation. Creation requires an explicit expiry; rotation requires an explicit overlap deadline. Revocation preserves audit metadata and immediately blocks later requests, including established MCP sessions.

Roles are fixed in v1: health includes OS information, inventory, and snapshot; inspect adds service and package inspection; diagnostics adds logs and configuration reads. One shared source policy applies per host. Client IP, proxy headers, tool annotations, and conversational approval are never substitutes for authentication or authorization.

### Measurements, ceilings, and data fidelity

Use shared collectors for overlapping OS and inventory fields. Omit unavailable optional fields entirely, including meaningless rolling-release version fields. Report collection failures as structured issues while retaining valid partial observations. Do not confuse a real zero with an absent value.

Load averages and CPU utilization remain separate observations. Sampling consumes the tool deadline and reports its interval. Health status and completeness are independent; missing required checks prevent unqualified healthy output. Thresholds, required checks, and filesystem exclusions are configurable.

Enforce the agreed default ceilings in the backend: ten seconds, four concurrent operations, 64 KiB configuration reads, 200 log entries, last fifteen minutes as the default log window, and 128 KiB serialized responses. Introduce explicit administrator ceilings for query window and transport sizes; exact additional defaults must be documented and tested during implementation. Clients can narrow requests but not enlarge ceilings.

Service and package results paginate. Log truncation is explicit. Time-filtered file logs require a declared parser with timestamp, timezone, multiline, and ordering semantics; unparseable event times yield unresolved coverage, never file-mtime substitutions. An explicitly requested raw tail can return bounded lines labeled without verified event-time coverage. Configuration reads that exceed the limit return a size issue without an apparently complete fragment. Cursor handling must not bypass authorization or source scope.

### Networking and operational records

Use a list of IP bind addresses with a shared port and transport policy. Start all listeners successfully or close all of them. Explicit IPv4-only and IPv6-only socket behavior avoids platform-dependent wildcard overlap.

Default to loopback HTTP on port 8080. Non-loopback plaintext requires `allow_insecure_http: true`; direct HTTPS requires configured certificate and key. Reverse proxies forward bearer credentials and protect the backend hop where it crosses a network. Invalid TLS never falls back to plaintext.

Trusted proxies are empty by default. For a trusted immediate peer, parse X-Forwarded-For from right to left, skipping trusted hops and using the first untrusted address; invalid or indeterminate chains fall back to the peer. Keep peer and resolved IP in audit metadata and do not use them to grant roles.

Emit structured stdout/stderr records with correlated IDs, tool metadata, duration, and outcomes. Never record returned content, bearer secrets, token hashes, or private keys. The system journal retains its own history; uninstall cannot erase shared journal records safely.

## Risks / Trade-offs

- **Broad read privilege:** Standard mode reduces administrator work but enlarges the impact of backend compromise. Keep the reader small, enforce policy locally, isolate secret paths, and offer restricted mode.
- **Sensitive allowed content:** A permitted application configuration or log may contain secrets. Profile exclusions and selective collectors reduce exposure; redaction cannot guarantee discovery of every secret.
- **Untrusted diagnostic text:** Logs can contain instructions aimed at an agent. Treat bodies as data, expose no arbitrary execution, and document the client-side reasoning boundary.
- **Reload races:** Two processes can disagree on active policy. Couple admission to a validated generation and fail closed during inconsistency.
- **Future portability:** macOS privacy controls and Windows access semantics need their own designs. Interface separation reduces rework but does not prove cross-platform security.
- **Missing history:** A current snapshot and retained host logs cannot prove that an earlier incident did not occur. Report coverage honestly.

## Migration Plan

This is a greenfield installation. An explicit administrator command previews installation and chooses standard or restricted mode. It durably records intended and completed changes for crash recovery, tracks later token state and rollback artifacts, installs services without starting by default, and leaves profile activation and networking review visible.

Upgrades validate compatibility before replacement, preserve administrator state, retain previous binaries, and restart only services previously running. Stage changes to bundled profiles for review before activation. Stop before incompatible state migrations; rollback is promised only when state remains reversible.

Uninstall previews and removes owned services, files, identities, and recorded permission changes. Preserve pre-existing identities, external directories, unexpected files within managed directories, unrelated data, and shared journal records. Report unresolved cleanup instead of claiming total removal.

## Retained future direction

The following decisions preserve the discussion's later-release direction without adding v1 implementation tasks.

### Remediation boundary

Future remediation uses a separately installable executable, OS identity, and narrow operation privileges behind the same host MCP gateway. A server-side YAML switch plus restart enables the backend; disabled means the backend is absent or stopped, tools are undiscoverable, and remembered tool calls are rejected. The gateway itself does not acquire the backend's OS privileges.

Server enablement and client authorization are separate. A monitor keeps diagnostic-only authority after a host enables remediation. A diagnostic credential can never enable remediation or edit its policy. Each remediation operation validates supported targets and parameters in its backend; no unrestricted root shell is implied.

When an operation requires approval, an independent mechanism must bind approval to the host, operation, exact parameters, and expiry. The model cannot approve itself. This does not mandate approval for every future operation; it preserves the distinction between policy-authorized automation and operations requiring human approval.

### OAuth and fleet growth

Keep identity verification separate from role/source authorization so OAuth-issued access tokens can be introduced without rewriting tool policy. Browser login, machine credentials, and mTLS were discussed as options; only pre-issued bearer tokens are selected for v1.

Direct per-host connections serve the initial fleet. A future fleet gateway can select hosts behind a shared connection while preserving their interfaces and identities. Ansible can later distribute configuration or installation assets but is not a runtime dependency. A dedicated always-on agent application owns periodic checks and notification reliability.

### Inventory context and external reasoning

The original brainstorming use case also benefits from administrator-maintained host purpose, dependencies, maintenance constraints, procedures, and past incidents. The discussion did not settle a v1 authoring or storage interface for those notes; record them as later enrichment rather than fabricate them as discovered facts.

Any future operator notes must remain distinguishable from observed inventory and retain their source and freshness. External agent applications control what returned excerpts reach a model and how conversation history is retained. HostLens can bound and restrict data access but cannot erase results already sent to another application.

### Naming and installation portability

HostLens is the accepted product name. Keep tool names and permission identifiers functional so a branding change does not require a protocol rename. A later rename may still require Go module/import changes, executable and package migration, service-name aliases, and configuration-path compatibility for installed hosts.

macOS daemon users and Windows virtual service accounts are future alternatives to Linux system users; none require pretending Linux capability semantics are portable. Installation and uninstall adapters must track platform-specific identities and preserve the same owned-resource cleanup contract.

### Explicit future profile precedence

Configured directory order is retained, but v1 rejects duplicate profile names. A future precedence setting can select the winning definition explicitly. It must not turn directory order into an override of the global deny-wins rule.

## Remaining publication decisions

The owner selected `github.com/Monska85/hostlens`. Repository visibility, project license, release signing, publication, and archival remain deferred. Runtime, dependencies, defaults, safe path semantics, parser behavior, and local archive verification are selected below.

## Implementation decisions (2026-09-10)

The owner selected `github.com/Monska85/hostlens`. Local builds pin Go 1.27.0, the official MCP SDK v1.7.0 (2026-07-27), YAML organization parser v3.0.5 (2026-07-26), doublestar v4.10.0 (2026-01-25), and Go x/sys v0.48.0 (2026-08-31). Module metadata was verified through the official Go proxy; the SDK requires Go 1.25 and x/sys requires Go 1.26. All fit the selected runtime. Dependency checksums and upstream license inventory accompany local archives; no publication license is selected.

Linux 5.6 or later is required for `openat2`. General reads reject symlinks conservatively with `RESOLVE_NO_SYMLINKS | RESOLVE_NO_MAGICLINKS`; descriptor-bound regular-file checks precede reads, and pseudo-filesystem roots are excluded. No unsafe kernel fallback exists. Narrow observation collectors may follow established OS identity links after checking both paths. Trust checks reject symlinked policy sources and untrusted writable ancestors.

Additional default ceilings are a 24-hour maximum log window, 1 MiB log inspection, 64 KiB HTTP request, 200 inventory entries per page, 30-second HTTP idle timeout, and 1,000 entries / two seconds for recursive explanation. Health uses a one-second CPU sample and 80/95 percent warning/critical thresholds for utilization and resource usage; normalized one-minute load uses 1/2 thresholds. Required checks are memory, swap, filesystem, services, load, and CPU. All these settings are configurable and validated.

File logs support explicit `jsonl` records with RFC3339 `timestamp`, string `message`, and optional journal-compatible integer `priority` (0 through 7), or explicit `raw` tail mode. Each physical line is one record; multiline messages must use JSON escaping. File order is preserved, rotation is limited to the selected opened file, and coverage remains incomplete when parsing, inspection ceilings, or retention cannot establish the requested interval. Raw tails carry no verified time coverage.

The complete configuration is represented by the typed Go configuration and the shipped example. System mode fixes administrator trust to root; user mode fixes it to the invoking UID. Restart-only comparisons include listener/TLS settings, credential paths, runtime paths, mode, and identities, including certificate/key content digests.

Local archive verification uses SHA-256 manifests over every member and strict archive path/type validation. A checksum proves agreement with the supplied manifest, not publisher identity; administrators obtain both through a trusted channel. Signing and publication remain separate decisions. All tests default to disposable containers. Record which service containment checks were demonstrated and which remain unavailable; container success alone does not prove host-kernel isolation. A strictly necessary VM requires explicit user confirmation before provisioning, downloading, or starting it.

The diagnostic systemd unit explicitly disables `RestrictSUIDSGID` because systemd blocks `openat2` under that setting: seccomp cannot inspect the indirect mode argument. The gateway retains the setting. The reader retains `NoNewPrivileges`, its read-only filesystem view, inaccessible credential paths, fixed capability set, namespace restrictions, device isolation, network restrictions, and syscall filtering. A diagnostic identity cannot create a root-owned setuid executable; it has no ownership-changing capability. This compatibility choice preserves the required safe-open primitive instead of introducing a fallback. See the [systemd execution reference](https://manpages.debian.org/unstable/systemd/systemd.exec.5.en.html#RestrictSUIDSGID=).

Fresh installation refuses pre-existing managed directories as conflicts, preserving their ownership and permissions rather than adopting them. Tracked uninstall checks remaining user and group ownership before deleting created identities.

### Review closure invariants

Configured mandatory source paths are literal names, including glob metacharacters. Only the internal pseudo-filesystem exclusions are glob patterns. Match caching and fingerprints include this distinction, and object protection checks every mandatory literal source.

Built-in file observations use descriptor-relative `openat2` resolution within the selected filesystem root, permit ordinary OS and sysfs symlinks within that root, and reject magic links and non-regular objects. Policy checks cover the requested name and the opened descriptor's resolved name before reading bytes.

Profile compilation rejects candidates exceeding 10,000 graph visits or 10,000 expanded provenance rules, independently of include depth. Log parsing aggregates repeated failures into bounded issue summaries with counts and checks cancellation between records. MCP execution failures set `isError`; useful partial observations and coverage warnings remain successful tool results.

Installation records identity existence before any mutation. An unvisited planned identity is never owned; an interrupted identity creation with uncertain completion is preserved for administrator inspection. Cleanup verifies service absence before skipping missing units and preserves genuine stop failures. Because `userdel` can implicitly remove a same-name private group, cleanup retains and reports a created user when that group is pre-existing or uncertain.
