## 1. Prerequisite and compatibility

- [x] 1.1 Complete, validate, archive, commit, push, and obtain green CI for `enforce-stateless-mcp-read-only`; verify its effect registry, default-on MCP gate, stateless evidence tests, and `AGENTS.md` invariants are present before changing the Docker surface.
- [x] 1.2 Verify the selected Docker Engine API endpoints and compatibility range against current official Docker documentation and representative Debian and Arch system-wide engines; record the tested daemon, API, architecture, storage-driver, cgroup, and log-driver matrix in the validation artifact.
- [x] 1.3 Confirm the minimal standard-library Engine API client remains the safer implementation; if a dependency is required, verify its registry identity, maintenance, release date, Go compatibility, license, and security guidance before updating `go.mod` and `go.sum`, then verify module and license checks pass.

## 2. Configuration and policy

- [x] 2.1 Add disabled-by-default Docker configuration with local daemon and observer Unix sockets, an expected existing socket-owning access group, strict validation, IPC collision checks, restart-only topology comparison, and explicit desired-state reporting; verify omission, standard and nonstandard groups, valid enablement, disabled, enabled-waiting, available, unavailable, remote and rootless rejection, unsafe paths, collisions, and reload preservation in container tests.
- [x] 2.2 Add canonical Docker policy resources for daemon, lists, items, stats, logs, networks, and disk usage with allow and deny provenance; verify denial precedence, list filtering, role isolation, stable-ID matching, name reuse, ambiguous prefixes, and policy fingerprint changes.
- [x] 2.3 Extend example profiles and policy explanation for Docker resources without enabling access by default; verify inactive grants do not affect decisions and explanations identify each matching allow, deny, source, and resolved stable identity.

## 3. Docker observer boundary

- [x] 3.1 Add the separately built `hostlens-docker-observer` executable and versioned, platform-neutral typed IPC contract; keep Linux Docker transport, peer authentication, paths, identities, and supervisor details in Linux-specific composition, and verify only the diagnostic service identity can connect while unknown operation types, versions, oversized requests, and malformed requests fail before Docker access.
- [x] 3.2 Implement the private Unix-socket Engine client with fixed GET endpoints, query keys, response ceilings, streaming decode, deadline, and cancellation; verify a recording daemon rejects every non-GET method and any unexpected path, query, header, body, follow mode, or registry credential.
- [x] 3.3 Implement runtime version negotiation and per-operation capability discovery; verify compatible newer engines work, incompatible APIs fail explicitly, optional endpoint loss affects only related tools, and no Docker CLI fallback executes.
- [x] 3.4 Project daemon responses into maintained non-secret contracts; verify environment, commands, arguments, unrestricted labels, raw health output, proxy values, registry data, secrets, configs, plugin data, and unknown future fields never cross observer IPC.
- [x] 3.5 Apply observer-specific concurrency and work bounds for lists, disk usage, stats, and logs; verify stalls, oversized streams, malformed JSON, cancellation, and repeated abandoned calls cannot accumulate workers, memory, descriptors, or queued work.

## 4. Docker diagnostic tools

- [x] 4.1 Register all nine Docker tools as diagnostic effects behind role, policy, capability, and MCP read-only gates; verify discovery and direct-call tests cover enabled, disabled, unavailable, denied, and both read-only flag values without backend invocation on rejection.
- [x] 4.2 Implement daemon, container list, container detail, and one-shot stats projections; verify running, stopped, unhealthy, restarting, renamed, removed, and name-reused fixtures return live bounded evidence with honest unavailable fields.
- [x] 4.3 Implement image, volume, network, and disk-usage projections with shared-layer and driver semantics; verify deduplication, current reference counts, missing size data, reclaimable estimates, filtered resources, truncation, and non-additive physical totals.
- [x] 4.4 Implement current-unused and cleanup-candidate analysis from all-container references with before-and-after consistency checks; verify stopped-container references prevent unused classification, dangling remains distinct, races suppress certainty, and creation time never becomes unused duration.
- [x] 4.5 Implement bounded Docker log retrieval with stable identity, time and byte windows, stream decoding, driver capability, rotation scope, and untrusted-data handling; verify TTY and multiplexed logs, unsupported drivers, truncation, malicious content, cancellation, and name reuse.
- [x] 4.6 Audit success, partial failure, timeout, and cancellation paths for retained Docker evidence; verify unique markers never remain in component state, HostLens-managed files, metrics, or audit logs and a later request always re-reads the daemon.

## 5. Installation and operator documentation

- [x] 5.1 Always package `hostlens-docker-observer` in the same versioned checksum manifest as the gateway and backend. Extend installer and reconciliation plans, ownership records, systemd units, hardening, readiness, upgrade, rollback, and uninstall for the optional topology; verify Docker-disabled installs grant no authority and Docker-enabled plans disclose root-equivalent observer authority before mutation.
- [x] 5.2 Implement dry-run `hostlens reconcile --system` and transactional `hostlens reconcile --system --apply` using existing ownership, conflict, rollback, and interruption protections; verify enablement, disablement, retry, pre-existing resources, and uncertain ownership without requiring reinstall.
- [x] 5.3 Add the HostLens-owned observer socket unit and service with a non-login identity, process-scoped `SupplementaryGroups=`, `Requisite=docker.service`, `After=docker.service`, and `PartOf=docker.service`; verify on-demand activation and shutdown, absence of persistent group membership, absence of `ConditionPathIsSocket=`, and that no HostLens action starts or restarts Docker.
- [x] 5.4 Add lifecycle regressions for Docker installed before HostLens, Docker installed after enabled-waiting reconciliation, Docker stopped, restarted, and removed, nonstandard socket or group, pre-existing identities and access, conflicts, interruption, retry, upgrade, rollback, disablement, and uninstall; verify only confirmed HostLens-owned observer resources are changed or removed.
- [x] 5.5 Snapshot Docker configuration, service state, containers, images, volumes, and networks around every lifecycle operation; verify Docker is never installed, reconfigured, restarted, stopped, or pruned and all pre-existing Docker resources remain unchanged.
- [x] 5.6 Update operator documentation with reconciliation, enablement, policy examples, tool contracts, sensitive-field exclusions, runtime scope, root-equivalent observer warning, live-data semantics, unused-time limitation, troubleshooting, late installation, disablement, uninstall, and downgrade steps; verify commands and examples against packaged binaries.

## 6. Acceptance validation

- [x] 6.1 Run focused observer, policy, gateway, backend, schema, audit, metrics, configuration, and lifecycle tests through the established disposable container workflow; verify race, vet, coverage, formatting, lint, vulnerability, packaging, and strict OpenSpec checks pass.
- [x] 6.2 Run every Docker MCP tool and adversarial bypass case against the recording daemon; verify the complete upstream transcript matches the read allowlist and before-and-after Docker fixture state is identical.
- [x] 6.3 Run the shared acceptance matrix with disposable Docker resources on representative Debian and Arch environments; verify live changes, health, stats, logs, storage accounting, daemon interruption, and namespace limitations are recorded without unsupported production claims.
- [x] 6.4 Exercise native systemd socket activation, dependency direction, socket permissions, installation, Docker-before-HostLens and Docker-after-HostLens flows, disablement, upgrade, rollback, and uninstall on representative Debian and Arch installations; verify pre-existing resources and persistent account memberships are preserved, remove test credentials and fixtures, and leave each installation running the validated latest build.
- [x] 6.5 Cross-build shared observer contracts and MCP projections for the established Windows and macOS targets; verify shared structures contain no Linux transport or lifecycle fields and report this only as structural evidence, not native runtime support.
- [x] 6.6 Re-run equivalent Make and Just full validation and strict current and archived OpenSpec validation on the exact candidate revision; verify every required stage passes and the validation artifact identifies any remaining environmental limitation.
