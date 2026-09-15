# Operating HostLens

HostLens is a local implementation candidate for Linux amd64 and arm64. It exposes diagnostic tools through bearer-authenticated Streamable HTTP at `/mcp` and service telemetry at `/metrics`. It does not execute remediation, schedule monitoring, retain inspected content, or call model providers. See [validation evidence](VALIDATION.md) before deployment. Release archives are prepared and validated as drafts by the release workflow and published on the [GitHub Releases page](https://github.com/Monska85/hostlens/releases) after maintainer approval; verification procedures live in the repository's `docs/RELEASING.md`, and `SECURITY.md` records the support policy.

## Prerequisites

For a step-by-step first installation on a new host, follow the [installation guide](INSTALL.md). This reference assumes the services are installed.

Linux amd64 or arm64, systemd for managed installation, and kernel 5.6 or later with `openat2` are required. Archives contain static Go binaries and dependency notices. Build and test procedures live in the repository's `docs/RELEASING.md`.

## Install and configure

Obtain an archive and its release `checksums.txt` file through a trusted channel. A checksum detects corruption or disagreement with a manifest; it does not authenticate the publisher. HostLens uses Apache-2.0. Each archive includes LICENSE, HostLens attribution in NOTICE.txt, and dependency notices in licenses/. Local development archives are unsigned.

Verify, extract into a new directory, and inspect the plan as the local administrator:

```sh
sha256sum --ignore-missing -c checksums.txt
mkdir hostlens-release
tar -xzf hostlens-0.1.0-dev-linux-amd64.tar.gz -C hostlens-release
hostlens-release/hostlens install --source hostlens-release --privilege restricted
hostlens-release/hostlens install --source hostlens-release --privilege restricted --apply
```

The installer creates separate non-login `hostlens-gateway` and `hostlens-diagnostics` identities and installs systemd units. Services remain stopped unless `--start` is explicit. Existing managed directories, configuration, or executable conflicts stop installation without adopting their permissions. The root-owned manifest at `/var/lib/hostlens/install.json` records intended and completed mutations, reserved token/socket state, and retained upgrade files.

Standard mode uses `--privilege standard`. It gives only the diagnostic service `CAP_DAC_READ_SEARCH`, which bypasses ordinary file read/search permissions and carries broad confidentiality risk. It does not grant arbitrary writes, shell access through MCP, or immunity to mandatory access controls. Restricted mode adds no capability; inaccessible observations remain explicit issues.

Review `/etc/hostlens/config.yaml` and installed profiles before starting. Fresh installations include no active file or journal grants. Bundled `nginx`, `allow-all`, and `docker-readonly` profiles remain inactive until explicitly referenced. Installation also places the dormant `hostlens-docker-observer` executable without any Docker authority; reconciliation provisions its identity and units only after you enable the integration. Source denials override every profile and every built-in observation grant.

`config.example.yaml` in the archive (repository: `packaging/config.yaml`) is a minimal configuration; omitted fields use built-in defaults. Additional fields include `profile_dirs`, `token_store`, `socket`, `admin_socket`, `gateway_user`, `diagnostics_user`, and `server.allowed_origins`. System identities are fixed in v1. YAML rejects unknown fields and active remediation settings. System-service policy files and their parents must be root-controlled and not writable by unrelated identities.

`mcp.read_only` defaults to `true` and is the global ceiling for MCP tool discovery and execution. Setting it to `false` only makes future remediation tools eligible for their separate role, policy, capability, and authority checks. It grants nothing by itself, and v1 still exposes diagnostics only. Local installation, lifecycle, token, status, reload, and telemetry administration do not pass through the MCP gate.

Diagnostics collect live evidence for each request. HostLens does not retain results, inspected content, inventories, history, cleanup candidates, or plans after the admitted collection worker exits. Client timeout or cancellation signals the worker; uninterruptible OS reads remain admission-bounded and visible until they return. HostLens retains only validated configuration and policy, credential metadata, installation ownership and rollback files, bounded request coordination, aggregate service metrics, and payload-free audit metadata. Pagination starts a new live observation rather than reading a server-side snapshot.

## Tokens and roles

Run token commands as root for system instances, or as the owning user for user instances. Every token requires a name, roles, and an explicit RFC3339 expiry or the `never` sentinel. Creation and rotation print the secret once; protect that output and never paste it into logs or issue reports.

```sh
hostlens token create --system --name laptop --roles diagnostics --expires 2030-01-01T00:00:00Z
hostlens token create --system --name service --roles health --expires never
hostlens token list --system
hostlens token list --system --all
hostlens token update --system --id PUBLIC_ID --roles health
hostlens token rotate --system --id PUBLIC_ID --expires 2030-01-01T00:00:00Z --overlap-until 2029-01-01T00:00:00Z
hostlens token revoke --system --id PUBLIC_ID
```

Choose expiry and overlap dates appropriate to the operation; overlap cannot extend the old token's existing expiry. A token created with `--expires never` stays valid until revoked or rotated: rotation imposes the overlap deadline on the old token as its retirement date, and the replacement may itself be non-expiring. Revocation is the instant kill for any token, including non-expiring ones. Older binaries that predate the sentinel treat such tokens as already expired and reject them. `token create`, `token list`, and `token rotate` display a non-expiring token's expiry as `never`. Listing never returns hashes or secrets. Updates replace the role set. Revocation, expiry, and role changes apply to the next request, including clients that remember previously available tools.

| Role          | Tools                                                                    |
| ------------- | ------------------------------------------------------------------------ |
| `health`      | `get_os_info`, `get_inventory`, `get_health_snapshot`                    |
| `inspect`     | Health tools plus `list_services`, `get_service_status`, `list_packages` |
| `diagnostics` | Inspect tools plus `read_config`, `query_logs`                           |
| `metrics`     | Scrape `/metrics` only; no diagnostic or administrative authority        |

Use `Authorization: Bearer SECRET` in the MCP client. Multiple independently authenticated clients share one host policy. The gateway uses stateless SDK transport: no retained session identity can preserve revoked authority. MCP tokens never authorize local administrative commands.

Start the services after reviewing policy and creating credentials:

```sh
systemctl start hostlens-diagnostics.service hostlens-gateway.service
hostlens status --system
```

## Service metrics

`GET /metrics` shares every configured HTTP listener and its TLS settings. Existing configurations default to a protected endpoint:

```yaml
metrics:
  enabled: true
  allow_anonymous: false
```

Both settings support `hostlens reload --system`. A failed reload preserves active settings, and already admitted scrapes may finish with their original access snapshot. Set `enabled: false` to return 404 and stop recording new request/tool observations. Re-enabling resumes existing counters; it does not reset them.

Create a dedicated token, or update an existing token while explicitly preserving its other roles:

```sh
hostlens token create --system --name prometheus --roles metrics --expires 2030-01-01T00:00:00Z
hostlens token update --system --id PUBLIC_ID --roles health,metrics
```

Role updates preserve the bearer secret. No diagnostic role includes metrics, and metrics alone grants no MCP tools or local administration. Expiry, revocation, rotation overlap and role removal take effect on subsequent protected scrapes. Store the one-time secret in a protected scraper credential file. For example, with a mode-0600 curl configuration containing `header = "Authorization: Bearer SECRET"`:

```sh
curl --config /secure/hostlens-scrape.conf --fail http://127.0.0.1:8080/metrics
```

Alternatively, explicitly set `allow_anonymous: true` and reload. This exposes service metadata to anyone who can reach **any configured listener**, including through a proxy. Anonymous scraping ignores supplied credentials. MCP continues to require bearer authentication on loopback and behind trusted proxies. Use TLS for remote bearer traffic.

Disabled routes return 404 before authentication; enabled methods other than GET return 405 with `Allow: GET`. Protected GET requests return 401 for missing/invalid/expired/revoked credentials and 403 for valid tokens without metrics. Each gateway admits one scrape before token reads; excess requests return 503 without queuing. MCP uses separate admission. Total collection and writing have a five-second deadline, backend collection has two seconds, and output is limited to 1 MiB. A stalled operation retains its slot until underlying work ends. Gateway collection/encoding failures return non-success, never a successful truncated exposition.

HTTP 200 establishes available gateway telemetry. Alert separately on `hostlens_backend_up`: 0 means backend telemetry failed, timed out or was malformed; backend samples are omitted, never cached or replaced by zero. A value of 1 establishes telemetry reachability only. Diagnostics still enforce generation synchronization and source policy.

### Metric catalog

All names below are fixed. Component metrics have `component="gateway"` or `component="backend"`; `hostlens_backend_up` has no labels. Counters reset with their owning process. Runtime values describe HostLens itself, without host/application polling or diagnostic collection. No labels include credential identities, addresses, hostnames, paths, application names, process IDs, commands, raw errors or inspected content.

| Metric                                   | Type and unit           | Additional labels and meaning                                                                                                                             |
| ---------------------------------------- | ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `hostlens_http_requests_total`           | Counter, requests       | Gateway only: `route` = `mcp` or `metrics`; `status` = `2xx`, `3xx`, `4xx`, `5xx`. Completed route requests, including rejections.                        |
| `hostlens_http_request_duration_seconds` | Histogram, seconds      | Gateway only: `route`; elapsed handler time, including authentication and response writing.                                                               |
| `hostlens_http_requests_active`          | Gauge, requests         | Gateway only: `route`; admitted unfinished work, including stalled scrape authentication/collection.                                                      |
| `hostlens_http_rejections_total`         | Counter, rejections     | Gateway only: `route`, `reason` = `authentication`, `authorization`, `overload`. Also includes MCP tool authorization failures inside HTTP 200 responses. |
| `hostlens_tool_calls_total`              | Counter, calls          | `tool`, `outcome`; completed calls observed at each component boundary.                                                                                   |
| `hostlens_tool_duration_seconds`         | Histogram, seconds      | `tool`; duration until the component returns its call result.                                                                                             |
| `hostlens_tool_work_active`              | Gauge, operations       | Unfinished component work. Backend retains stalled native collection until it ends.                                                                       |
| `hostlens_collection_gaps_total`         | Counter, affected calls | `tool`, `reason`; each gap category counted at most once per call.                                                                                        |
| `hostlens_go_heap_objects_bytes`         | Gauge, bytes            | Memory occupied by live or unswept Go heap objects.                                                                                                       |
| `hostlens_go_memory_bytes`               | Gauge, bytes            | Total memory mapped by the Go runtime.                                                                                                                    |
| `hostlens_go_goroutines`                 | Gauge, goroutines       | Current Go goroutines.                                                                                                                                    |
| `hostlens_go_gc_cycles_total`            | Counter, cycles         | Completed Go garbage collections.                                                                                                                         |
| `hostlens_process_start_time_seconds`    | Gauge, Unix seconds     | Process telemetry initialization time, near process startup.                                                                                              |
| `hostlens_process_cpu_seconds_total`     | Counter, CPU seconds    | Linux process user plus system CPU time.                                                                                                                  |
| `hostlens_process_max_resident_bytes`    | Gauge, bytes            | Linux process lifetime peak resident memory, not current RSS.                                                                                             |
| `hostlens_backend_up`                    | Gauge, 0 or 1           | Backend telemetry reachability for this scrape.                                                                                                           |

Histograms use upper bounds `0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60` seconds, plus `+Inf`, `_sum` and `_count`. Tool labels are the fixed [diagnostic tool names](SPEC.md#tool-contract) and audit names documented below; arbitrary names map to `unknown`. Outcomes are `success`, `partial`, `unavailable`, `error`, `timeout`, `cancelled`, `overload`, `denied`. Gap reasons group result issue codes into `unavailable`, `denied`, `limit`, `invalid`, `coverage`, or `other`; truncation contributes `limit`.

Gateway and backend call counters are separate boundary observations: do not sum both components to count user calls. Transport/authentication rejection does not invent a tool execution. Backend rejection can occur before collection, and a gateway call may fail before reaching the backend. Tool timeout counters describe the returned result; active backend work may remain nonzero afterward. HTTP counters include only `/mcp` and enabled `/metrics`; a scrape's completed HTTP outcome becomes visible in a later scrape.

The fixed catalog permits at most 1,745 exposed series across both components, including histogram series and all tool/outcome/gap combinations. Unused counter/vector combinations are absent until observed. Unsupported Go measurements and non-Linux native process measurements are omitted. No generic Prometheus default/global collectors, build labels or application exporters are registered.

Before downgrading to a binary without metrics support, use the current CLI to remove `metrics` from all token role sets. For metrics-only tokens, revoke them first, then update their inactive role metadata to `health`; updating roles does not reactivate a revoked token. After cleaning token metadata, remove the `metrics` configuration section, and then downgrade both services together. Older unknown-role/unknown-field checks remain strict.

## Policy, explanation, and reload

Patterns use absolute Linux paths. A literal matches only that path, `*` does not cross separators, and `**` enables recursion. Journal patterns match concrete systemd unit names and never accept arbitrary journal expressions. Only included profiles activate; duplicate names across configured directories and include cycles fail validation.

```sh
hostlens config validate --system
hostlens policy explain /etc/nginx/nginx.conf --system
hostlens policy explain /etc/nginx --system --recursive
hostlens reload --system
systemctl kill --kill-whom=main --signal=HUP hostlens-gateway.service
```

Explanation evaluates disk policy plus the active MCP read-only setting and prints `MATCH`, `DIFFERENT`, or `UNKNOWN` against an identified local instance. It shows matching origins, inclusion chains, inactive rules, and resolution status. A match proves effective policy equivalence only; it does not prove OS readability, full configuration equality, or blanket descendant access.

Reload validates the complete candidate in both processes. `mcp.read_only` is reloadable. A transition to read-only mode closes future remediation admission before activation; diagnostics already admitted under their generation can finish. Listener, TLS material, identity, credential-location, privilege, Docker observer settings, and connection idle-timeout changes require restart; a mixed reload fails without partially applying the file. If a backend restart cannot restore the gateway's active validated generation from disk, calls remain unavailable until configuration consistency is restored. Status reports the read-only value, effective fingerprint, and generation together.

Before downgrading to a binary that predates `mcp.read_only`, remove the `mcp` section from configuration with the current binary, validate the result, and downgrade both services together. Older binaries reject the unknown field rather than silently ignoring the safety setting.

This hardening update changes the Linux policy evaluator fingerprint to reflect hard-link rejection. Upgrade and restart both services together; old and new evaluators must not share an active generation.

General reads reject symlinks, multiply linked regular files, special files, `/proc`, `/sys`, and `/dev`, even with broad grants. Configured credential paths and policy files are literal mandatory exclusions through every reader, even when their filenames contain glob characters. Built-in observations permit ordinary symlinks within their filesystem root, reject multiply linked files, and validate the opened target before reading. Avoid creating aliases or copies of secrets in allowed paths: path policy is not an information-flow guarantee. Allowed application files and logs can themselves contain sensitive data.

Trusted configuration loading accepts at most 1 MiB per document and 8 MiB across the main configuration and all profiles. It also caps configured profile directories, entries per directory, and total profile definitions at 1,024 each. Oversized reloads preserve the active generation.

## Diagnostic formats and limits

All tools return observation times and explicit issues. Fatal execution failures set MCP `isError`; partial observations and historical coverage warnings remain successful results. Repeated JSONL parsing failures are aggregated with counts, and cancellation stops parsing between records. Optional unavailable facts are omitted; observed zero values remain valid. Host identity comes from the target, not the client's connection label. Namespace-visible measurements do not establish physical-host or application health.

`list_services` and `list_packages` accept `offset` and `limit`. Continuation uses `next_offset`; underlying mutations can change subsequent pages. Service status uses selected native properties and excludes environment, configuration bodies, and journal excerpts.

Use each tool's advertised argument schema. OS, inventory, and health take no arguments; configuration reads require `path`, service status requires `unit`, and log queries require exactly one source. Unrelated arguments fail validation. Health evaluates collected services before pagination and retains observed failures when coverage is incomplete.

`read_config` accepts `path` and returns a complete UTF-8 regular file or an issue. It never presents an oversized or invalid-encoding fragment as complete configuration.

`query_logs` selects exactly one `path` or `unit`. File time queries require `format: jsonl`: each physical line contains an RFC3339 `timestamp` and string `message`, with optional integer `priority` from 0 through 7. JSON escaping represents multiline messages. `since` and `until` are inclusive RFC3339 boundaries; `priority` retains journal meaning and selects priorities at or above that severity (numerically less than or equal to the value).

Explicit `raw_tail: true` returns a bounded physical tail without verified event timestamps; it cannot be combined with time or severity filters. Rotation scope is the selected opened file only. Journal queries retain the selected unit and native timestamps/priorities. Every log result reports incomplete historical coverage when rotation, retention, permissions, parsing, or ceilings prevent establishing the requested interval. Messages are untrusted data, never instructions to HostLens.

| Setting                                            | Default               |
| -------------------------------------------------- | --------------------- |
| Tool deadline / host concurrency                   | 10 seconds / 4        |
| Configuration bytes / serialized MCP tool response | 65,536 / 131,072      |
| Log entries / inspected bytes                      | 200 / 1,048,576       |
| Default / maximum log window                       | 15 minutes / 24 hours |
| HTTP request bytes / idle timeout                  | 65,536 / 30 seconds   |
| Inventory page / explanation count                 | 200 / 1,000           |
| Explanation deadline / CPU sample                  | 2 seconds / 1 second  |

Docker observation ceilings are fixed in the observer contract, not configurable: 5,000 items per container/image/volume inventory, 1,000 networks, 16 MiB per list or disk-usage response, 256 KiB per daemon info, container inspect, or stats response, 8 MiB and 10,000 records per log response, 64 KiB per IPC request, and 4 concurrent observations with one dedicated disk-usage slot. The verified Engine API range is 1.41 through 1.51; newer compatible daemons negotiate down to the tested ceiling.

Response ceilings include the SDK's text and structured representations. Gateway admission bounds HTTP work before authentication; excess requests receive HTTP 503. Timeout and cancellation return explicit issues. A blocked OS read retains its backend admission slot and only its transient request-scoped evidence until the worker finishes; it cannot supply a later request. This prevents unbounded abandoned work without claiming synchronous memory erasure for uninterruptible kernel I/O.

Capability discovery and backend configuration preparation each permit one underlying operation at a time. A stalled operation keeps that capacity occupied after cancellation; further preparation fails promptly and does not replace the active configuration.

Exclude unreliable network/FUSE mounts with `health.exclude_filesystems` and avoid granting reads to stalled filesystems. If all slots remain occupied after client timeouts, restore the failing filesystem first, then restart the backend if needed. A process restart cannot repair kernel I/O that remains uninterruptible.

Health reports severity separately from completeness. Capacity checks skip autofs and binfmt_misc control filesystems; mounted storage remains eligible. Bind mounts and tmpfs reflect the service mount namespace, so several paths can report the same underlying capacity. Required checks default to memory, swap, filesystem, services, load, and CPU. Usage warning/critical thresholds are 80/95 percent; normalized one-minute load thresholds are 1/2. Filesystem exclusions, required checks, sampling, and thresholds are configurable. Failed-service findings warn at one and become critical at two. CPU affinity and aggregate procfs utilization have different scopes, and load does not prove CPU saturation.

## TLS, proxy trust, and containment

The default listener is `127.0.0.1:8080`. `server.bind` accepts an explicit IPv4/IPv6 list; same-family overlapping binds fail. All listeners must start successfully. Native TLS uses `server.tls.enabled`, `cert_file`, and `key_file`; invalid material never falls back to HTTP.

Non-loopback plaintext requires `allow_insecure_http: true`. Protect every network hop carrying bearer credentials, including the backend hop from a remote TLS proxy. Proxies forward `Authorization`; they do not replace token verification.

Configure each server as a separately named MCP connection with its own endpoint and token. Host selection and comparisons belong to the client; connecting several servers does not create a shared policy or grant authority to change them.

No proxies are trusted by default. `trusted_proxies` accepts IPs/CIDRs; only a trusted immediate peer enables right-to-left `X-Forwarded-For` resolution. Invalid or entirely trusted chains fall back to the peer. Origin checks and the SDK's localhost protection remain active. Add exact trusted browser origins using `allowed_origins` when required.

If changing token or TLS-key locations, update the diagnostic unit's `InaccessiblePaths` for those configured sources before restarting. Standard mode refuses startup unless a verified systemd inaccessible mount hides each configured secret. Mount verification handles masked ancestors and checks the inaccessible object identity. Restart TLS to activate replacement certificate or key contents. After correcting repeated startup failures, run `systemctl reset-failed hostlens-gateway.service` before starting it again if systemd reports that start requests repeated too quickly.

The diagnostic unit sets `RestrictSUIDSGID=no` because systemd otherwise disables required `openat2`; the gateway keeps that control. Every service unit applies the same sandbox baseline: a `@system-service openat2` syscall allow-list with mount, reboot, swap, and raw-I/O denials, native architecture pinning, personality, realtime, and namespace locks, and protected clock, hostname, and kernel logs plus a private IPC namespace. The gateway and observer additionally hide other users' processes and non-PID proc files (`ProtectProc=invisible`, `ProcSubset=pid`); the diagnostic reader omits them because audit evidence reads system-wide proc files. The reader retains `NoNewPrivileges`, read-only filesystem protection, fixed capabilities, protected credential mounts, namespace/device/network restrictions, and syscall filtering. Profiles and read-only mounts do not provide complete protection against a compromised backend.

In restricted mode, administrators may grant targeted ACLs or reader-group access. For journal access, membership in `systemd-journal` gives the process broader journal visibility than individual HostLens unit rules expose. Such manual grants are administrator-owned; account for file replacement during log rotation and document cleanup separately.

## Audit, upgrade, and removal

Structured JSON logs go to the service manager. Authentication failures, denied calls, reloads, backend failures, and tool outcomes include correlation metadata. Logging level and successful-call auditing are configurable. Records never include bearer secrets, token hashes, TLS keys, returned configuration, or inspected log bodies.

```sh
journalctl -u hostlens-gateway.service -u hostlens-diagnostics.service
hostlens upgrade --archive hostlens-0.1.0-dev-linux-amd64.tar.gz
hostlens upgrade --archive hostlens-0.1.0-dev-linux-amd64.tar.gz --apply
hostlens uninstall
hostlens uninstall --apply
```

Upgrade verifies every archive member and compatibility before replacement, retains old executables, and restarts only previously active services. Activation checks actual IPC and gateway readiness. Rollback resets failed services before restarting restored binaries, so a crashing candidate cannot leave recovery blocked by its start-rate limit. Reversible failures restore the previous executables. Changed bundled profiles are saved as `.candidate` files for administrator review; active policy is preserved.

A partial lifecycle operation leaves its manifest for inspection and cleanup. Uncreated units do not block cleanup. Pre-existing and unvisited identities are preserved; an identity whose creation was interrupted before ownership confirmation requires administrator inspection. Cleanup also retains a created user when removing it could implicitly delete a pre-existing same-name private group; resolve that mixed identity state explicitly before retrying. Uninstall removes tracked resources and created identities, preserves unexpected files, and reports incomplete cleanup. After resolving a reported external-file conflict, retry with an executable retained from the original extracted archive if installed binaries were already removed. Shared journals remain under normal system retention. HostLens does not undo untracked ACL or rotation changes by guessing their previous state.

## Built-in observation sources

Built-in grants apply only to selected facts, never raw file tools. Explicit denials can disable these source locations:

- **Identity:** `/etc/os-release` and its resolved source, `/proc/sys/kernel/osrelease`, `/proc/sys/kernel/hostname`, and `/etc/machine-id`.
- **Hardware:** `/sys/class/dmi/id/product_name` and `/sys/class/dmi/id/sys_vendor`, when available.
- **Resources:** `/proc/meminfo`, `/proc/loadavg`, `/proc/stat`, and `/proc/self/mounts`, plus `statfs` for each included mount. CPU availability uses scheduler affinity.
- **Services:** `/run/systemd/system` identifies the native service observation interface. Status uses fixed `systemctl show` properties. Explicit unit denials also restrict service observations.
- **Packages:** `/var/lib/dpkg/status` or `/var/lib/pacman/local` identifies the package database behind the fixed native query.

Configuration input is capped at 1 MiB per file, profile directories at 1,024 entries, include depth at 64, and resolved provenance at 10,000 rules, with a separate 10,000-visit include traversal ceiling. These parser safety bounds are separate from configurable diagnostic limits.

### Ownership checks during uninstall

Uninstall preserves accounts and groups when ownership scanning fails, including when root cannot traverse desktop FUSE mounts such as GVFS or document portals. An ownership-check failure does not prove that unrelated owned files exist. Resolve the inaccessible mount or end the affected user session, then retry uninstall. Do not bypass the check by deleting the account manually on a production host.

## Docker diagnostics

Docker diagnostics are opt-in and disabled by default. They expose bounded, read-only evidence from the local rootful system-wide Docker Engine on Linux through an isolated `hostlens-docker-observer` process. Rootless Docker, remote engines, Docker Desktop, Swarm administration, Kubernetes, Podman, and other container runtimes remain unsupported.

**Authority warning:** the Docker Unix socket grants root-equivalent control to the process that opens it, and Docker provides no read-only socket permission. Access to that socket therefore sits in a separately built, separately supervised observer identity with a typed, GET-only observation contract. The gateway and diagnostic backend never receive the Docker socket, Docker group membership, or a generic Docker API pass-through. Every observation request is a fixed GET; the daemon records the full transcript in acceptance tests.

### Enable and reconcile

```sh
# 1. Append the docker section to /etc/hostlens/config.yaml.
# 2. Validate and plan without mutation:
hostlens config validate --system
hostlens reconcile --system
# 3. Review the plan. It discloses root-equivalent observer authority,
#    the socket, the group, and every identity, unit, and file change.
# 4. Apply:
hostlens reconcile --system --apply
# 5. Restart the services so the running configuration activates:
systemctl restart hostlens-diagnostics.service hostlens-gateway.service
```

Reconciliation creates a dedicated non-login identity (`hostlens-observer`), adopts the observer binary placed by installation, writes the observer service and socket units, and enables the socket unit. Upgrading from a release that predates the observer needs `--source` with the extracted release (see below). `install`, `upgrade`, `uninstall`, and `reconcile` each enforce a two-minute plan-and-mutate deadline; long-running mutations abort with an error and the plan records completed steps for a safe retry. Docker group authority is process-scoped: the service declares `SupplementaryGroups=` for the configured socket group and no persistent account membership is ever added. The service declares `Requisite=docker.service`, `After=docker.service`, and `PartOf=docker.service`, so a diagnostic request can activate the observer only while Docker is active, and no HostLens action starts, stops, restarts, or reconfigures Docker.

Runtime states are honest and separate:

- **disabled:** no Docker authority or tools.
- **enabled-waiting:** reconciliation provisioned the topology but the daemon socket is absent or the engine is stopped. Installing and starting Docker later lets the next diagnostic request activate the observer without another reconciliation.
- **available:** the observer reached a compatible engine (Engine API floor 1.41; newer compatible daemons negotiate the tested maximum).
- **unavailable:** the observer cannot access the socket, the daemon stopped, the engine is incompatible, or the daemon reports an unsupported mode such as rootless or Docker Desktop.

A stopped or removed daemon stops the observer through the declared dependency without clearing configured intent. A nonstandard socket or group produces an actionable unavailable status: update `daemon_socket` or `group` in configuration and rerun reconciliation.

### Policy grants

Docker resources use the `docker` policy category. Collection forms grant one inventory or aggregate observation each; item forms scope stable identities and current names. Denial wins across stable IDs, current names, and list membership, and a denied container also excludes that container's stats and logs.

```yaml
allow:
  docker: [daemon, containers, disk_usage]
  # Item forms: container/<name-or-id-or-glob>, stats/*, logs/*,
  # image/*, volume/*, network/*
deny:
  docker: [container/db, logs/db]
```

`hostlens policy explain --system docker:container/web` explains every matching rule with its provenance and resolved decision. Inactive example grants in `docker-readonly.yaml` never affect decisions until copied into an active profile.

### Tool contracts and exclusions

| Tool                         | Evidence                                                                                        |
| ---------------------------- | ----------------------------------------------------------------------------------------------- |
| `get_docker_info`            | Engine version, negotiated API scope, storage/cgroup/log drivers, counts, capability state      |
| `list_docker_containers`     | Bounded live inventory: identity, lifecycle, health, ports, mounts, references                  |
| `get_docker_container`       | One permitted container: state, restart count, limits, mounts, log driver, no health output     |
| `get_docker_container_stats` | One point-in-time sample with daemon-supplied counters and their exact scope                    |
| `list_docker_images`         | Deduplicated identities, tags, digests, shared-layer sizes, current container references        |
| `list_docker_volumes`        | Names, drivers, current references, sizes only when the daemon supplies them                    |
| `list_docker_networks`       | Identity, driver, scope, selected non-secret configuration, current references                  |
| `get_docker_disk_usage`      | Daemon disk totals, per-resource sizes, advisory reclaimable estimates from the unused analysis |
| `query_docker_logs`          | Bounded stdout/stderr records with explicit truncation, rotation scope, and driver gaps         |

Lists accept `offset` and `limit`, sort by full stable identity, and state that the next page is a new observation. Container tools require one `container` selector; name selectors resolve to the full immutable identity, policy is evaluated against both forms, and the resolved identity is re-verified before evidence is returned, so name reuse releases nothing for an unauthorized replacement.

Docker issue `reason` strings and messages may contain Docker daemon-supplied error text; treat them as untrusted data for downstream consumers and never as HostLens vouching.

Never exposed: environment values, commands and arguments, unrestricted labels, registry authentication, secrets, configs, plugin data, proxy values, raw health-check output, raw inspect objects, mounted content, or any Docker mutation (exec, attach, copy, events, build, prune, pull, push, tag, lifecycle control). A generic Docker request tool does not exist; every request is fixed in the observer.

### Live-data and unused-resource semantics

Every result is calculated from live daemon responses within the request. Nothing is retained between requests: no inventories, log records, stats history, cleanup candidates, or last-use estimates. A second request always re-reads the daemon.

Unused classification uses only the completed observation: an image or volume is `currently_unused` only when no container references it, including stopped containers. Dangling remains a separate daemon fact. Creation time is reported when supplied but never mapped to `unused_since` or `unused_duration`: Docker supplies no trustworthy last-reference time, so last-use evidence is reported unavailable. Cleanup candidates are advisory evidence with an observation-stability check; racing inventory changes suppress certainty and are reported as `non_atomic_observation`.

### Troubleshooting

- `docker_disabled`: the configuration section is missing or `enabled` is false.
- `docker_unavailable`: the observer is not provisioned, Docker is stopped or absent, the socket is missing, or peer validation refused access. Check `systemctl status hostlens-docker-observer.socket`, the configured `daemon_socket` and `group`, and rerun reconciliation after any change.
- `unsupported_engine`: the daemon reports rootless, Docker Desktop, or another unsupported mode; HostLens keeps Docker tools unavailable.
- `driver_gap`: the container's log driver cannot serve records through the daemon interface; this does not establish that no logs or incidents exist.
- `non_atomic_observation`: Docker changed during inventory correlation; unused claims are suppressed and results stay advisory.

### Disable, uninstall, and downgrade

`hostlens reconcile --system --apply` with the section disabled or removed stops and disables the observer, removes its owned units, socket, and binary, and leaves the dormant account without Docker group authority. Uninstall removes all owned observer resources while preserving every Docker resource, container, image, volume, network, and daemon configuration. Upgrading from a release that predates the observer does not install the new binary (the running release performs the upgrade), so run `hostlens reconcile --system --apply --source /path/to/extracted-release` once after enabling the section; reconciliation restores the binary and records ownership. Downgrading to a build without the `docker` section requires removing the section first: an older binary rejects it as unknown. Upgrade and rollback never restart Docker.

## Generic server and application audits

Audit tools require a diagnostics token and explicit `audit` grants. Existing configurations activate none. Profiles can grant selected domains or `*`; every active denial wins. File and journal grants remain independent, and explicit source denials still apply to built-in observations.

```yaml
allow:
  audit:
    [
      processes,
      network,
      accounts,
      storage,
      updates,
      security,
      services,
      hostlens,
      paths,
    ]
  files: [/etc/example/app.conf, /var/log/example/app.log]
  journal: [example.service]
```

Choose actual application paths and units before activating a profile. Approved configuration and log contents can contain credentials; HostLens does not claim automatic secret redaction for arbitrary application text. Process arguments, environments and unrestricted service properties are not exposed by audit collectors.

| Tool                                 | Evidence                                                                                                                         | Audit domain |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| `list_processes`, `get_process_info` | Process identity, native CPU ticks, resident pages, selected credentials, executable path and accessible socket inode references | `processes`  |
| `get_network_info`                   | Interfaces/counters, addresses, routes, TCP listeners and UDP endpoints                                                          | `network`    |
| `list_accounts`                      | Local accounts and group membership, without password fields                                                                     | `accounts`   |
| `get_storage_info`                   | Block devices, mount identity/options and software RAID activation                                                               | `storage`    |
| `get_update_info`                    | Local repository metadata age and available expiry dates                                                                         | `updates`    |
| `get_security_info`                  | Selected kernel controls and visible active security modules                                                                     | `security`   |
| `inspect_service`                    | Selected effective systemd runtime, dependency and hardening properties                                                          | `services`   |
| `inspect_path`                       | Allowed regular-file or directory metadata, without content or recursive traversal                                               | `paths`      |
| `get_hostlens_info`                  | Selected effective backend configuration and runtime identity, excluding credentials                                             | `hostlens`   |

`get_process_info` requires a positive `pid`; `inspect_service` requires a concrete `unit`; `inspect_path` requires an absolute clean `path` plus file-source permission. Process/account lists accept `offset` and `limit`. Other audit tools accept no arguments. Process offsets index the enumerated PID list: a page can be empty if its processes disappear or become inaccessible. Continue when `next_offset` exists; pages are not atomic snapshots.

A generic investigation starts with service/process identity, correlates listening sockets and storage, examines privileges and isolation, then reads explicitly approved configuration and logs. The agent interprets the application evidence. No PostgreSQL, MySQL, Apache or nginx integration is needed; HostLens never executes SQL or arbitrary commands.

### Evidence limits

`coverage_complete` describes collector coverage, not a security verdict. Preserve valid partial evidence and report missing observations. `policy_denied`, `permission_denied`, unavailable-source, malformed-source and limit issues identify different causes. Neither an empty list nor an inaccessible source establishes absence of a security problem.

- **OS authority:** Existing restricted mode and standard-mode CAP_DAC_READ_SEARCH are unchanged. Firewall inspection, process descriptor access and device-health interfaces may be unavailable; no additional privileged helper is installed.
- **Network:** Observations describe the collector network namespace. IPv4 address/interface association, policy-routing rules, additional IPv4 tables and external reachability are not established. Correlate socket inode references only when process visibility permits.
- **Accounts and configuration:** Local accounts do not establish directory-service users, password lock/expiry, effective sudo access or context-dependent SSH Match behavior. Selected configuration reads can support investigation but are not an evaluated authorization result.
- **Maintenance:** Cached repository dates do not prove a successful refresh, signature verification, pending upgrade candidates, reboot requirements or a current vulnerability assessment. Native package-manager configuration is not executed to bypass source policy.
- **Storage and recovery:** Mount aliases are not separate disks. LVM topology, RAID redundancy/recovery, physical health behind virtualization and backup restorability require additional evidence. Application logs may record backups; a recorded success is not a verified restore.

Service dependency identities and unit-file locations remain subject to their source denials. Path metadata rejects symlinks, special files and hard-linked regular files, including protected aliases. Mode bits alone do not establish effective access through parent directories, ACLs or security modules.
