## Context

HostLens currently has a gateway and a diagnostic backend. Neither can access Docker. A rootful Docker Unix socket is a control interface, and Docker documents that membership in its socket-owning group grants root-level privileges. Mounting that socket read-only changes filesystem access to the socket entry but does not limit HTTP methods sent through an established connection.

The Docker Engine API provides live daemon, container, image, volume, network, stats, log, and disk-usage observations. It reports current references and some creation or lifecycle timestamps, but it does not provide a reliable general timestamp for when an image or volume became unused. Shared image layers also prevent naive size summation.

Implementation starts only after `enforce-stateless-mcp-read-only` is implemented and archived. Its effect registry, stateless evidence boundary, and separated mutation authority are prerequisites rather than duplicated infrastructure.

## Goals / Non-Goals

**Goals:**

- Treat the local system-wide Docker Engine as a generic host infrastructure source.
- Give MCP clients useful daemon, workload, resource, log, and unused-storage evidence under explicit policy.
- Keep Docker control authority out of the gateway and diagnostic backend.
- Ensure every daemon request is fixed, bounded, observable in tests, and read-only.
- Preserve honest time, size, reference, consistency, and namespace semantics.
- Install and remove the optional observer without changing Docker or its resources.
- Establish portable observation and lifecycle contracts that future native platforms can implement without inheriting Linux path, identity, IPC, or supervisor assumptions.

**Non-Goals:**

- Support rootless or remote Docker, Docker Desktop, Windows containers, Podman, direct containerd, Kubernetes, or Swarm administration.
- Implement or claim native Windows or macOS runtime support in this change.
- Add container exec, attach, file copy, event streaming, image registry, build, plugin, secret, config, or mutation operations.
- Inspect application-specific data inside containers.
- Persist Docker events, stats, logs, inventories, cleanup candidates, or last-use history.
- Infer unused duration when the live daemon has no last-reference timestamp.
- Protect against replacement of the trusted, installed observer binary by a host root administrator.

## Decisions

### Use a separate privileged observer

Add `hostlens-docker-observer` as a separate optional executable and independently supervised process. It alone opens the configured system-wide Docker Unix socket. A dedicated non-login identity, distinct from the gateway and diagnostic identities, receives access through process-scoped service credentials. Its systemd service uses the socket owner's existing group through `SupplementaryGroups=` without adding persistent account membership. The installer treats this authority as root-equivalent, records the generated service configuration, and displays it before mutation.

The observer exposes a separate local Unix socket readable only by the diagnostic backend. Its protocol accepts a closed set of typed observation requests. The observer constructs the upstream endpoint, query, and headers; callers cannot supply an HTTP method, URL, header, body, Docker context, or registry credential. Responses are decoded into HostLens-owned bounded structures before IPC, so raw Docker objects and newly added upstream fields do not cross the boundary automatically.

The observer is part of the trusted computing base because the Docker daemon does not provide a general read-only permission for its local control socket. The guarantee is enforced by the installed observer code, its typed IPC surface, the absence of mutation implementations, gateway and backend isolation, and recorded-request acceptance tests. Running the diagnostic backend in the Docker group or mounting the Docker socket into it was rejected because either would turn a parser defect into unrestricted Docker control.

### Preserve native platform boundaries

Keep the versioned observation operations, bounded request and response structures, capability states, evidence-gap semantics, and MCP projections independent of the local transport. Put the Linux Docker connector, Unix peer authentication, systemd unit construction, user and group discovery, filesystem paths, and lifecycle commands behind Linux-specific composition. Do not place Unix paths, numeric Unix identities, systemd unit names, or Linux-only error details in shared observation contracts.

Future Windows support must provide Windows services with separate service identities, ACL-protected named pipes, native credential storage, and a Docker transport appropriate to the supported Engine mode. Future macOS support must provide `launchd` supervision, native credential storage, and a proven XPC or socket boundary. Docker Desktop modes have user-session, virtualization, and lifecycle boundaries that differ from the system-wide Linux engine in this change.

Do not add unused Windows or macOS adapters during the Linux implementation. The portable contracts form the compatibility seam. Each native implementation requires its own OpenSpec change, threat analysis, packaging, lifecycle tests, and native validation proving that the gateway and diagnostic backend cannot access the Docker control interface.

### Use a minimal Engine API client

Use Go's HTTP client over an absolute Unix socket and implement only selected GET observations from the Engine API. Do not invoke the Docker CLI and do not include a generic request helper in packages reachable from IPC dispatch. Keep the upstream endpoint table private to the observer package and associate every entry with its fixed GET method, path constructor, allowed query keys, maximum response size, decoder, and projection.

The fixed initial allowlist contains observation forms of these endpoints:

- **daemon.** Version and system information.
- **containers.** All-container list, stable-ID inspect, one-shot stats, and bounded logs.
- **images.** Image list and disk-usage information needed for references and shared-size semantics.
- **volumes.** Volume list and disk-usage information when supported by the driver and daemon.
- **networks.** Network list with selected non-secret fields.
- **disk usage.** System disk-usage summary and detailed accounting under the shared deadline.

Version negotiation first reads the daemon version and selects the highest Engine API version that is both supported by HostLens and the daemon. Implementation records the verified compatibility floor after checking official API documentation and representative engines. A missing optional endpoint or field removes only the affected observation. An incompatible base API removes Docker capability without falling back to CLI output parsing.

The official Docker Go client remains an alternative only if dependency verification shows a smaller and safer result than the fixed client. Any such decision must preserve the private endpoint allowlist and cannot expose the SDK client through collector or IPC interfaces.

### Keep Docker policy resource-based

Extend policy rules with a `docker` resource class. Rules use explicit resource forms for daemon information, collection listings, disk usage, and item operations. Container stats and logs have separate resources from container metadata. Collection authorization happens before observer access; item filtering happens again after stable identities are observed.

For a name selector, the backend resolves the name to a full immutable ID, evaluates allow and deny rules against both forms, performs the typed request by full ID, and verifies the returned ID. Any deny match wins. A list response filters denied items before deriving visible counts, and reports incomplete policy coverage so a client cannot interpret a filtered list as the complete engine.

The alternative was one broad `audit: docker` permission. That would make granting inventory silently grant logs and every resource detail, which is too wide for homeserver policies.

### Expose diagnostic projections instead of raw inspect data

Each tool returns a maintained schema with observed fields only. The projections exclude environment variables, command and arguments, unrestricted labels, registry authentication, secrets, configs, raw health output, daemon proxy values, and opaque plugin data. Container mounts include type and destination plus policy-safe source metadata only when needed; they do not read mounted content.

`query_docker_logs` uses stable container identity, bounded time and byte windows, no follow mode, and the daemon's log retrieval interface. The decoder handles Docker's multiplexed stream and TTY behavior without treating log bytes as protocol. Unsupported drivers and uncertain rotation coverage produce evidence gaps.

`get_docker_container_stats` requests a one-shot sample. HostLens reports daemon-supplied counters with their exact scope and calculates rates only when the required pair of values and interval belong to that response. It retains no earlier sample.

### Derive unused state from a single bounded live observation

For cleanup analysis, fetch all containers including stopped containers and correlate their stable image and volume references with the bounded image and volume inventories. A resource is `currently_unused` only when the completed observation contains no container reference. A dangling image remains a separate daemon fact because an untagged image can still be referenced.

Return creation time when available, but never map it to `unused_since`. If Docker has no trustworthy last-reference timestamp, omit last-use and duration fields and include `last_use_unavailable`. A cleanup candidate contains the observation time, current reference basis, non-atomic coverage, and reclaimable estimate if supplied. It is evidence, not an executable or retained plan.

Inventory generation can race with Docker changes. Read the container inventory before and after expensive resource correlation and compare a bounded identity/reference fingerprint. Retry once within the request deadline when it changes. If stability is not established, return useful observations with an explicit non-atomic issue and do not mark uncertain items as cleanup candidates.

### Apply shared bounds before and during decoding

Docker tools use the existing MCP request deadline, concurrency admission, output ceiling, cancellation, and pagination contracts. The observer adds per-endpoint input byte and item ceilings. It uses streaming JSON decoding where practical and closes the daemon response on cancellation or overflow.

Docker list endpoints do not provide a common stable pagination snapshot. Sort a bounded live result by full stable identity, apply the requested offset and limit, and state that the next page is a new observation. Reject offsets beyond the configured inspection ceiling. Disk usage and stats share Docker-specific admission so expensive requests cannot starve all diagnostic work.

### Make topology opt-in and reconcilable

Add a disabled-by-default Docker configuration with the daemon Unix socket and observer IPC socket. Only absolute local Unix paths are accepted. Enabling, disabling, or changing either socket changes service topology and authority, so it requires a full HostLens restart. Docker policy and existing operation limits remain reloadable.

Treat configured intent and current runtime availability as separate states: `disabled`, `enabled-waiting`, `available`, and `unavailable`. The system installer or `hostlens reconcile --system` calculates and displays the complete plan without mutation. `hostlens reconcile --system --apply` transactionally creates or removes the observer identity, units, socket directory, and process-scoped access using the lifecycle ownership, conflict, rollback, and interruption protections. It does not install Docker, edit daemon configuration, change the daemon socket, start or restart Docker, or create a Docker group.

Reconciliation enables a HostLens-owned systemd socket unit while leaving the observer service dormant. The observer service declares `Requisite=docker.service`, `After=docker.service`, and `PartOf=docker.service`. A Docker diagnostic request can activate the observer only while Docker is active, and HostLens never pulls Docker into the transaction or controls its lifecycle. The observer validates the Docker socket type, locality, ownership, group, and permissions before every connection. Do not use `ConditionPathIsSocket=` because the supported Debian systemd baseline predates that directive.

When Docker is absent, an enabled and reconciled configuration remains `enabled-waiting` and non-Docker HostLens services operate normally. If Docker is installed and started later with the configured socket and group, its package refreshes the systemd manager and the next Docker diagnostic request activates the observer without reinstalling or reconciling HostLens again. A stopped or removed daemon stops the observer through the declared dependency, makes Docker diagnostics unavailable without changing configured intent, and leaves other tools operational. A nonstandard socket or group produces actionable unavailable status; the administrator updates configuration and reruns reconciliation.

Disabling the integration through reconciliation stops and disables the observer service and socket unit. The dormant account then has no Docker group authority because no persistent membership exists. Reconciliation never grants access to the gateway or diagnostic backend.

Upgrade and rollback replace all HostLens executables from one versioned distribution and preserve configured intent. They restart the observer only when provisioned and never control Docker. Uninstall stops the observer first, removes only recorded HostLens-owned service configuration, identities, and files, and preserves every Docker resource and pre-existing system object. Uncertain ownership blocks deletion and appears in the lifecycle report.

### Validate with a recorder and real engines

A fake Unix-socket Engine API records exact requests and returns controlled races, oversized bodies, malformed JSON, multiplexed logs, missing fields, and delayed responses. The full Docker tool registry runs against it. Any upstream request outside the GET-only observation allowlist fails the test.

Real-engine acceptance uses disposable resources with unique labels and names, then compares Docker state before and after every tool and failure path. It covers Debian and Arch system-wide engines, records daemon/API/storage/cgroup details, and distinguishes nested or shared-kernel evidence. Lifecycle acceptance uses the established disposable systemd workflow when possible and representative native systemd installations only for behavior a container cannot establish.

## Risks / Trade-offs

- **Observer authority.** The observer's daemon socket access is root-equivalent. Keep the process small, separately identified, hardened, unavailable by default, and reachable only through typed local IPC.
- **Engine API drift.** New fields or endpoints can change exposure. Decode only maintained fields, negotiate a tested API range, and treat unknown data as ignored rather than automatically returned.
- **Expensive disk accounting.** Docker disk usage can be slow on large stores. Give it separate bounded admission, cancellation, byte limits, and explicit timeout results.
- **Incomplete unused analysis.** Concurrent resource changes can invalidate correlation. Use before-and-after reference fingerprints and suppress cleanup-candidate claims when stability is not established.
- **Secret-bearing metadata.** Labels, environment, commands, health output, logs, and mount sources can contain secrets. Omit unrestricted metadata, separate log authority, apply source policy, and run unique-marker tests.
- **Service-group differences.** Docker socket ownership varies by installation. Validate the actual socket and group without changing them, disclose the planned access, and fail safely when dedicated access cannot be established.
- **Deferred platform guarantees.** Windows service and named-pipe isolation appear compatible with the observer model, while per-user Docker Desktop installations have different authority boundaries. Claim support only after a native implementation proves equivalent separation and lifecycle safety.
- **Nested test limits.** Docker-in-Docker and containers share host kernel behavior. Record that scope and use representative native systemd installations only for unresolved systemd, socket permission, and uninstall checks.

## Migration Plan

1. Complete and archive `enforce-stateless-mcp-read-only`.
2. Verify Docker API compatibility and any dependency against official documentation and representative Debian and Arch engines.
3. Add disabled configuration and policy syntax without changing the default installation surface.
4. Implement and harden the observer, typed IPC, projections, and fake recording daemon before registering MCP tools.
5. Add Docker collectors and tool schemas behind capability, role, policy, and effect gates.
6. Extend installation, reconciliation, socket activation, upgrade, rollback, disablement, and uninstall ownership tracking for the optional observer, including Docker installed before and after HostLens.
7. Run focused tests, disposable-container validation, and native-host gap validation before archive and publication.

Rollback first disables Docker diagnostics and performs a coordinated HostLens service restart, then uses the previous HostLens build. An older binary will reject the new Docker configuration as unknown, so the operator must remove that section before downgrade. Rollback never changes Docker daemon configuration or resources.
