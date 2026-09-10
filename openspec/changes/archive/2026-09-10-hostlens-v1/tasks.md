# HostLens v1 implementation tasks

The completed local implementation and container checks are recorded in [validation evidence](../../../../docs/v1/VALIDATION.md). Checked tasks have implementation and focused validation evidence. The selected review and final reconciliation gates are complete; the specifications are synced and this local change is archived.

Publication is explicitly deferred outside this session and is recorded separately from active implementation checkboxes. Each section maps to the similarly named capability specification.

## 1. Project and platform foundation

- [x] 1.1 Select the Go module path with the repository owner and pin Go, official MCP SDK, YAML, and glob dependencies; verify documented kernel and build prerequisites and a reproducible dependency resolution.
- [x] 1.2 Create gateway/CLI and diagnostic executable boundaries with shared typed contracts; verify both build for Linux amd64 and arm64 and no privileged collector is registered by the gateway.
- [x] 1.3 Define platform adapters for collectors, paths, IPC, reload triggers, identities, and service lifecycle; verify Linux-specific imports remain behind platform boundaries.
- [x] 1.4 Implement interface-based runtime capability discovery; verify Debian, Ubuntu, Arch, unknown-version, missing-systemd, and missing-package-manager fixtures without OS-version gates.

## 2. Typed access policy

- [x] 2.1 Implement strict YAML configuration and typed profile parsing; verify source-located errors for unknown keys, malformed patterns, unsupported active selectors, and remediation enablement.
- [x] 2.2 Resolve mode-specific and additional local profile directories; verify unique-name enforcement, safe references, inactive profiles, missing dependencies, cycle diagnostics, and unsafe profile ownership or writable parent directories.
- [x] 2.3 Compile deduplicated allow/deny rules while retaining every source and inclusion chain; verify deny-wins and order-independent decisions across direct rules and dependencies.
- [x] 2.4 Implement exact, child, and recursive path matching plus journal-unit globs; verify no accidental recursion from a bare directory and no path semantics reused as journal expressions.
- [x] 2.5 Implement handle-bound safe file access and mandatory exclusions for configured secret sources; verify symlink escapes, concurrent replacements, alternate tool paths, and special-file rejection.
- [x] 2.6 Ship reviewed base, Nginx, and inactive allow-all profiles; verify fresh installations activate no broad access and main-file exceptions override profile grants.

## 3. IPC and policy generations

- [x] 3.1 Implement access-controlled Unix socket operations and peer validation; verify unrelated local identities cannot call diagnostics and backend failures produce explicit unavailable responses.
- [x] 3.2 Enforce backend source policy and shared resource budgets independently of gateway tool discovery; verify direct malformed IPC cannot select arbitrary commands or policies.
- [x] 3.3 Implement coordinated immutable configuration generations and SIGHUP reload; verify successful activation, failed validation, backend interruption, in-flight reads, and generation mismatch handling.
- [x] 3.4 Verify isolated backend restart re-synchronization, full-restart disk loading, and atomic status snapshots; reject restart-only changes during reload; verify changed listeners, certificates, identities, and privileges leave the previous complete generation active.
- [x] 3.5 Implement local instance status and canonical effective-policy fingerprints; verify comment/order stability, semantic changes, evaluator-version changes, and exclusion of credentials.
- [x] 3.6 Implement policy explain for files and directories with optional bounded recursion; verify provenance, inactive matches, unresolved paths, mandatory denials, and MATCH/DIFFERENT/UNKNOWN reporting without claiming full configuration equality.

## 4. Tokens and roles

- [x] 4.1 Implement protected atomic token storage, random secrets, hashes, public IDs, names, expiry, roles, and status; verify one-time secret output and absence of plaintext secrets in persisted state.
- [x] 4.2 Implement locally authorized token create/list/update/revoke/rotate commands; verify listing filters, role replacement, explicit overlap expiration, and refusal of MCP-only administrative access.
- [x] 4.3 Implement bearer verification and fixed additive health/inspect/diagnostics roles; verify every tool-role pair including get_os_info and denied-path requests under diagnostics.
- [x] 4.4 Apply token changes on each request including existing sessions; verify revocation, role removal, expiration, identity mixing, and concurrent token-store updates without service restart.

## 5. Diagnostic collectors and tools

- [x] 5.1 Define compact response schemas with observations, provenance, times, issues, and explicit truncation; verify unavailable values are omitted and actual zeros are preserved.
- [x] 5.2 Implement shared OS and inventory collectors; verify consistent family, distribution/product, version omission, kernel, architecture, host identity, and hardware facts.
- [x] 5.3 Implement documented basic collector grants with explicit-deny precedence and sanitized status output; implement systemd service listing/status and dpkg/pacman package inventory; verify native states, pagination, absent services, and command argument validation.
- [x] 5.4 Implement policy-enforced configuration reads; verify size rejection, denied targets, encoding issues, and no silent partial configuration output.
- [x] 5.5 Implement bounded file-log queries with explicit parsing and coverage semantics; verify rotation, missing timestamps, time filtering, byte/entry limits, source-supported severity filters, and no invented event times or severity labels.
- [x] 5.6 Implement selected-unit journal queries with native priority filtering; verify allowed and denied units, unscoped-query rejection, unavailable journals, source boundaries, and retention gaps.
- [x] 5.7 Implement backend deadlines, cancellation, concurrency admission, pagination, and response ceilings; verify shared limits across clients and explicit overload/truncation outcomes.
- [x] 5.8 Register the eight diagnostic MCP tools and capability/role-aware discovery; verify direct calls cannot bypass authorization and no remediation or shell tool is available, host identity is accurate, and independently authenticated clients share the endpoint safely.

## 6. Health assessment

- [x] 6.1 Implement current memory, swap, filesystem space/inodes, failed-service, load-average, CPU-availability, and sampled utilization observations; verify units, sources, measurement windows, and partial failures.
- [x] 6.2 Define documented configurable health thresholds, required checks, exclusions, and sampling defaults; verify invalid thresholds are rejected and configured thresholds appear with findings.
- [x] 6.3 Implement severity and completeness as separate outputs; verify mixed warnings and failed required checks never yield unqualified healthy results.
- [x] 6.4 Verify on-demand behavior and namespace scope in integration tests; demonstrate no background collection, metric store, log-copy retention, model invocation, or historical incident claims.

## 7. Transport and audit

- [x] 7.1 Implement atomic multi-listener startup with explicit IPv4/IPv6 families; verify loopback defaults, wildcard overlap validation, unavailable addresses, and listener cleanup on failure.
- [x] 7.2 Implement optional native TLS and non-loopback plaintext opt-in; verify certificate/key failures, no downgrade, bearer enforcement behind a proxy, and restart-only certificate replacement.
- [x] 7.3 Implement bounded HTTP handling and MCP origin protections; verify malformed requests, oversized bodies, idle timeouts, and selected SDK protocol behavior.
- [x] 7.4 Implement trusted proxy CIDRs and right-to-left X-Forwarded-For evaluation; verify spoofed headers, invalid chains, IPv6, peer fallback, and no IP-derived authorization.
- [x] 7.5 Implement structured correlated audit records and configurable verbosity/success logging; verify required failure events, peer/client IPs, and exclusion of tokens, hashes, keys, and returned bodies.

## 8. Installation and lifecycle

- [x] 8.1 Define archive layout, installation paths, and a durable ownership manifest; verify installation plans, conflict detection, crash recovery, later-created state tracking, and preservation of unexpected files.
- [x] 8.2 Implement local administrator installation with dedicated non-login accounts and systemd units; verify no passwords are requested and services remain stopped by default.
- [x] 8.3 Implement standard-mode process capability and service containment; verify in a disposable container that gateway secrets remain inaccessible to the backend and ordinary log rotation needs no ACL changes; record any unverified service containment, and obtain explicit user confirmation before provisioning, downloading, or starting a strictly necessary VM.
- [x] 8.4 Implement restricted mode without additional capabilities; verify explicit permission failures and document targeted ACL/group/rotation arrangements.
- [x] 8.5 Implement manifest-driven uninstall; verify removal of created users/groups/services/state, preservation of pre-existing and external resources, conflict reports, and shared-journal preservation.
- [x] 8.6 Implement staged upgrades with compatibility validation and retained binaries; verify active-service restoration, failed-activation rollback, and refusal before irreversible migrations.
- [x] 8.7 Implement bundled-profile change review during upgrade; verify no silent activation, broadening, or overwriting of administrator-edited policy.

## 9. Release acceptance and operator documentation

- [x] 9.1 Establish disposable container execution for all tests, including systemd and lifecycle where supported; publish exact environments, architecture execution versus emulation, and kernel/namespace limitations. If a VM is strictly necessary, obtain explicit user confirmation before provisioning, downloading, or starting it; never test privileged lifecycle on the administrator host.
- [x] 9.2 Run cross-component security scenarios for path escapes, alternate sources, token changes, proxy spoofing, reload failure, quotas, and credential containment; verify every required scenario passes before release claims.
- [x] 9.3 Produce amd64 and arm64 archives with verification metadata and dependency/license inventory; verify clean-machine install, upgrade, rollback, and uninstall in both privilege modes.
- [x] 9.4 Write operator instructions for roles, tokens, profiles, explain/reload, TLS/proxy deployment, audit retention, and privilege tradeoffs; verify every documented command against the implemented CLI.
- [x] 9.6 Run OpenSpec validation and reconcile each requirement with implementation evidence; archive the change only after implementation and required validation are complete.

## Final gate reconciliation

- **9.2:** All ten accepted findings are fixed. Spec, performance, and docs closure reviews passed. The coordinator verified the exact security and adversarial patches and container regressions after those roles exhausted their two-run budgets following platform content-filter interruptions.
- **9.5:** Publication is deferred by the user. Owner/module selection is complete; visibility, license, final release version, publication, commits, pushes, and remote creation remain outside this session.
- **9.6:** Strict OpenSpec validation, the requirement evidence map, and final review reconciliation passed. The completed local change is synced to the main specifications and archived.

## Review fix round 1 evidence

Accepted source-boundary, policy traversal, MCP failure, JSONL parsing, shared snapshot, lifecycle recovery, and reproduction-prerequisite fixes are implemented. The disposable race suite, vet, module verification, static builds, platform smoke, and both real-systemd modes pass; [validation evidence](../../../../docs/v1/VALIDATION.md) records the added regressions and conservative identity cleanup limit.

Tasks 9.2 and 9.6 await coordinator review reconciliation. This fix round does not authorize archival or publication.

## Deferred publication

9.5 Confirm GitHub owner, visibility, license, version, and publication authorization before publishing; verify repository metadata and release artifacts match the reviewed scope.

Publication is outside the authorized local implementation scope. This obligation remains required before a future publication.
