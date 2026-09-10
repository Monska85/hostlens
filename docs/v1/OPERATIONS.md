# Operating HostLens

HostLens is a local implementation candidate for Linux amd64 and arm64. It exposes eight diagnostic tools through bearer-authenticated Streamable HTTP at `/mcp`. It does not execute remediation, schedule monitoring, retain inspected content, or call model providers. See [validation evidence](VALIDATION.md) before deployment; no release has been published.

## Prerequisites

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

Review `/etc/hostlens/config.yaml` and installed profiles before starting. Fresh installations include no active file or journal grants. Bundled `nginx` and `allow-all` profiles remain inactive until explicitly referenced. Source denials override every profile and every built-in observation grant.

`config.example.yaml` in the archive (repository: `packaging/config.yaml`) contains the shipped defaults. Additional fields are `profile_dirs`, `token_store`, `socket`, `admin_socket`, `gateway_user`, `diagnostics_user`, and `server.allowed_origins`. System identities are fixed in v1. YAML rejects unknown fields and active remediation settings. System-service policy files and their parents must be root-controlled and not writable by unrelated identities.

## Tokens and roles

Run token commands as root for system instances, or as the owning user for user instances. Every token requires a name, roles, and an explicit RFC3339 expiry. Creation and rotation print the secret once; protect that output and never paste it into logs or issue reports.

```sh
hostlens token create --system --name laptop --roles diagnostics --expires 2030-01-01T00:00:00Z
hostlens token list --system
hostlens token list --system --all
hostlens token update --system --id PUBLIC_ID --roles health
hostlens token rotate --system --id PUBLIC_ID --expires 2030-01-01T00:00:00Z --overlap-until 2029-01-01T00:00:00Z
hostlens token revoke --system --id PUBLIC_ID
```

Choose expiry and overlap dates appropriate to the operation; overlap cannot extend the old token's existing expiry. Listing never returns hashes or secrets. Updates replace the role set. Revocation, expiry, and role changes apply to the next request, including clients that remember previously available tools.

| Role          | Tools                                                                    |
| ------------- | ------------------------------------------------------------------------ |
| `health`      | `get_os_info`, `get_inventory`, `get_health_snapshot`                    |
| `inspect`     | Health tools plus `list_services`, `get_service_status`, `list_packages` |
| `diagnostics` | Inspect tools plus `read_config`, `query_logs`                           |

Use `Authorization: Bearer SECRET` in the MCP client. Multiple independently authenticated clients share one host policy. The gateway uses stateless SDK transport: no retained session identity can preserve revoked authority. MCP tokens never authorize local administrative commands.

Start the services after reviewing policy and creating credentials:

```sh
systemctl start hostlens-diagnostics.service hostlens-gateway.service
hostlens status --system
```

## Policy, explanation, and reload

Patterns use absolute Linux paths. A literal matches only that path, `*` does not cross separators, and `**` enables recursion. Journal patterns match concrete systemd unit names and never accept arbitrary journal expressions. Only included profiles activate; duplicate names across configured directories and include cycles fail validation.

```sh
hostlens config validate --system
hostlens policy explain /etc/nginx/nginx.conf --system
hostlens policy explain /etc/nginx --system --recursive
hostlens reload --system
systemctl kill --kill-whom=main --signal=HUP hostlens-gateway.service
```

Explanation evaluates disk policy and prints `MATCH`, `DIFFERENT`, or `UNKNOWN` against an identified local instance. It shows matching origins, inclusion chains, inactive rules, and resolution status. A match proves policy equivalence only; it does not prove OS readability, full configuration equality, or blanket descendant access.

Reload validates the complete candidate in both processes. Listener, TLS material, identity, credential-location, privilege, and connection idle-timeout changes require restart; a mixed reload fails without partially applying the file. If a backend restart cannot restore the gateway's active validated generation from disk, calls remain unavailable until configuration consistency is restored. Status reports fingerprint and generation together.

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

Response ceilings include the SDK's text and structured representations. Gateway admission bounds HTTP work before authentication; excess requests receive HTTP 503. Timeout and cancellation return explicit issues. A blocked OS read retains its backend admission slot until the operation finishes, preventing unbounded abandoned work.

Capability discovery and backend configuration preparation each permit one underlying operation at a time. A stalled operation keeps that capacity occupied after cancellation; further preparation fails promptly and does not replace the active configuration.

Exclude unreliable network/FUSE mounts with `health.exclude_filesystems` and avoid granting reads to stalled filesystems. If all slots remain occupied after client timeouts, restore the failing filesystem first, then restart the backend if needed. A process restart cannot repair kernel I/O that remains uninterruptible.

Health reports severity separately from completeness. Required checks default to memory, swap, filesystem, services, load, and CPU. Usage warning/critical thresholds are 80/95 percent; normalized one-minute load thresholds are 1/2. Filesystem exclusions, required checks, sampling, and thresholds are configurable. Failed-service findings warn at one and become critical at two. CPU affinity and aggregate procfs utilization have different scopes, and load does not prove CPU saturation.

## TLS, proxy trust, and containment

The default listener is `127.0.0.1:8080`. `server.bind` accepts an explicit IPv4/IPv6 list; same-family overlapping binds fail. All listeners must start successfully. Native TLS uses `server.tls.enabled`, `cert_file`, and `key_file`; invalid material never falls back to HTTP.

Non-loopback plaintext requires `allow_insecure_http: true`. Protect every network hop carrying bearer credentials, including the backend hop from a remote TLS proxy. Proxies forward `Authorization`; they do not replace token verification.

No proxies are trusted by default. `trusted_proxies` accepts IPs/CIDRs; only a trusted immediate peer enables right-to-left `X-Forwarded-For` resolution. Invalid or entirely trusted chains fall back to the peer. Origin checks and the SDK's localhost protection remain active. Add exact trusted browser origins using `allowed_origins` when required.

If changing token or TLS-key locations, update the diagnostic unit's `InaccessiblePaths` for those configured sources before restarting. Standard mode refuses startup unless a verified systemd inaccessible mount hides each configured secret. Mount verification handles masked ancestors and checks the inaccessible object identity. Restart TLS to activate replacement certificate or key contents.

The diagnostic unit sets `RestrictSUIDSGID=no` because systemd otherwise disables required `openat2`; the gateway keeps that control. The reader retains `NoNewPrivileges`, read-only filesystem protection, fixed capabilities, protected credential mounts, namespace/device/network restrictions, and syscall filtering. Profiles and read-only mounts do not provide complete protection against a compromised backend.

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

Upgrade verifies every archive member and compatibility before replacement, retains old executables, and restarts only previously active services. Activation checks actual IPC and gateway readiness. Reversible failures restore the previous executables. Changed bundled profiles are saved as `.candidate` files for administrator review; active policy is preserved.

A partial lifecycle operation leaves its manifest for inspection and cleanup. Uncreated units do not block cleanup. Pre-existing and unvisited identities are preserved; an identity whose creation was interrupted before ownership confirmation requires administrator inspection. Cleanup also retains a created user when removing it could implicitly delete a pre-existing same-name private group; resolve that mixed identity state explicitly before retrying. Uninstall removes tracked resources and created identities, preserves unexpected files, and reports incomplete cleanup. After resolving a reported external-file conflict, retry with an executable retained from the original extracted archive if installed binaries were already removed. Shared journals remain under normal system retention. HostLens does not undo untracked ACL or rotation changes by guessing their previous state.

## Built-in observation sources

Built-in grants apply only to selected facts, never raw file tools. Explicit denials can disable these source locations:

- **Identity:** `/etc/os-release` and its resolved source, `/proc/sys/kernel/osrelease`, `/proc/sys/kernel/hostname`, and `/etc/machine-id`.
- **Hardware:** `/sys/class/dmi/id/product_name` and `/sys/class/dmi/id/sys_vendor`, when available.
- **Resources:** `/proc/meminfo`, `/proc/loadavg`, `/proc/stat`, and `/proc/self/mounts`, plus `statfs` for each included mount. CPU availability uses scheduler affinity.
- **Services:** `/run/systemd/system` identifies the native service observation interface. Status uses fixed `systemctl show` properties. Explicit unit denials also restrict service observations.
- **Packages:** `/var/lib/dpkg/status` or `/var/lib/pacman/local` identifies the package database behind the fixed native query.

Configuration input is capped at 1 MiB per file, profile directories at 1,024 entries, include depth at 64, and resolved provenance at 10,000 rules, with a separate 10,000-visit include traversal ceiling. These parser safety bounds are separate from configurable diagnostic limits.
