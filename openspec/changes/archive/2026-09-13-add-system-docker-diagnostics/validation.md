# Validation record — add-system-docker-diagnostics

Implementation record for the change. Every claim below was executed; unavailable checks are named.

## Prerequisites

- `enforce-stateless-mcp-read-only` archived; its hosted CI run (4cefc3aa) was green before this change began.
- Engine API compatibility verified against official Docker version-history documentation before implementation. Verified floor 1.41 (one-shot stats, `CgroupVersion` in daemon info), tested ceiling 1.51 (legacy `/system/df` response fields and the `shared-size` images query stable in that range). Newer daemons negotiate down to the tested ceiling; daemons whose minimum API exceeds 1.51 are refused explicitly.
- No new Go dependency: the observer uses the standard library HTTP client over the Unix socket. `go.mod` is unchanged.

## Representative engines exercised

| Engine                     | API reported | API negotiated | Storage driver | cgroup | Architecture | Log driver |
| -------------------------- | ------------ | -------------- | -------------- | ------ | ------------ | ---------- |
| Docker 26.1.5, Debian 13   | 1.45         | 1.45           | overlay2       | 2      | amd64        | json-file  |
| Docker 29.7.2, Arch         | 1.55         | 1.51           | overlay2       | 2      | amd64        | json-file  |
| Docker 29.8.0, Arch        | 1.56         | 1.51           | overlayfs      | 2      | amd64        | json-file  |

Two real findings from live engines were fixed and covered by regression tests:

- Negotiation wrongly refused daemons whose `MinAPIVersion` (for example 1.40 on Docker 29.x) is below the client floor; the daemon can still serve versions between its own minimum and current API. Negotiation now targets `max(min(daemon, tested ceiling), client floor, daemon minimum)`.
- The all-container list timestamp is Unix seconds, not RFC3339; the projection decoded it correctly after the fix, and network attachment names fall back to the map key when the daemon omits the value field.

## Executed validation

- **Recording daemon:** the full nine-tool registry and all observer operations ran against a fake Unix-socket engine that records every request; every upstream request was a versioned GET from the fixed table, log observations requested no follow mode, and the exact request sequence was asserted.
- **Secret-bearing exclusion:** container list, inspect, stats, and log projections were checked against unique markers for environment values, commands, labels, health output, log-driver options, and unknown future daemon fields; none crossed the observer IPC or appeared in MCP projections.
- **Race-safe identity:** name reuse and ambiguous-prefix fixtures return `identity_changed` and `ambiguous_selector` instead of evidence; re-verification re-reads the live inventory.
- **Unused analysis:** stopped-container references prevent unused classification, dangling stays distinct from unused, racing reference fingerprints suppress `currently_unused` and report `non_atomic_observation`, and creation time never becomes `unused_since` or `unused_duration`.
- **Stateless evidence:** every Docker tool re-reads the daemon on each call; failure and cancellation paths release observations and leave no markers in component state, metrics, or audit logs.
- **Disposable container suite:** the full race/vet/build/Python suite passed inside the disposable checks container, including the new `dockerobs`, `observerapp`, lifecycle, gateway, and CLI packages and portable Windows/macOS cross-builds of the shared contracts and projections.
- **Coverage:** container coverage run completed; internal statement coverage 74.6% for this revision (coverage scope unchanged from prior evidence).
- **Vulnerabilities:** `govulncheck` over runtime modules: no vulnerabilities found at scan time.
- **Packaging:** GoReleaser archives contain `hostlens`, `hostlens-diagnostics`, `hostlens-docker-observer`, the three example profiles, and license notices; checksum manifests verify strictly; archive verification ran in a disposable container for both architectures.
- **Live read-only acceptance:** the complete tool surface ran against all three real engines through the real observer IPC without any Docker mutation (before/after engine snapshots; unrelated external engine churn was distinguished from fixture state). Re-run after the review fixes on the local engine.
- **Disposable fixtures on representative engines:** the fixture lifecycle on the Debian and Arch engines proved running references, stopped-container references, unreferenced classification after removal, honest size omissions, and state restoration; fixture cleanup was verified by snapshot comparison. Re-run after the review fixes on the Debian engine.
- **Native systemd lifecycle on representative Debian and Arch installations:** fresh install provisions no observer without enablement; reconciliation plans do not mutate before `--apply`; apply provisions the identity, binary, units, and socket activation; the observer service stays dormant without an activation request; Docker tools report explicit unavailability while the engine is absent or stopped; stop/start cycles keep the socket unit active (`TriggerLimitIntervalSec=0`) and the next request re-activates the observer without another reconciliation; reload rejects topology changes; disablement removes owned units, socket, and binary while preserving the dormant identity; uninstall removes owned resources and identities; Docker containers, images, volumes, networks, and daemon configuration were snapshot-verified unchanged across install, reconcile, upgrade, stop/start, disablement, and uninstall on both installations.
- **Upgrade from a release without the observer:** the previous release's upgrade does not know the observer binary, so reconciliation restores it from the extracted release source (`--source`) and records ownership; documented in operator guidance.

## Peer review round 1 — findings and dispositions

Six reviewers (security, idiomatic, maintainability, performance, spec, adversarial) reviewed the implementation. Every high and medium finding was verified in code (three also at runtime) and fixed:

- **Policy before observer access (High).** The dispatcher now enforces each tool's class grant before any observer contact: collection forms evaluate allow and deny directly, item classes require an active allow without a class-wide denial, discovery skips the capability probe when no Docker class is granted, and discovery respects collection-form denials. Ungranted and denied direct calls are covered by a mutation-probe test asserting zero observer requests.
- **Container policy in disk usage (High).** `get_docker_disk_usage` now filters container rows and stopped-container identifiers through the container decision, matching the image and volume filtering already present.
- **Deny rules on practical identity forms (High).** Image lists evaluate denial through every tag and digest; network and volume lists evaluate the current name form alongside the stable ID.
- **Inspect port bindings (Critical).** The Engine API serializes `HostPort` as a JSON string; the projection now parses it, and the fake daemon fixtures match the real API shape.
- **Response-budget pagination (High).** Docker list pages derive their default and maximum page size from the response ceiling, so a full default observation is never discarded wholesale; explicit over-budget limits clamp with a next offset.
- **Negotiation race (High).** Engine API negotiation serializes under a mutex, failed probes stay retriable, and the observation budget bounds negotiation as well; concurrent cold-start is covered under `-race`.
- **Reconciliation disclosure (High).** The reconcile report now enumerates every identity, unit, socket, and file change plus the root-equivalent disclosure text in both dry-run and apply results.
- **Observer unit sandbox (Medium).** The observer service adopts the repository baseline: system-call filter, namespace, personality, realtime, architecture, and network restrictions.
- **Driver-gap classification (Medium).** The observer reports unsupported log drivers through a structured issue code instead of error-text matching.
- **Log byte clamp (Medium).** Inspection budgets above the 8 MiB observer log ceiling clamp in the collector; a response ending exactly at the ceiling no longer reports truncation.
- **Invalid page bounds (Medium).** Docker lists mirror the other collectors: over-ceiling bounds set the error result instead of a success-shaped empty page.
- **Network unused scope (Medium).** The undeclared `currently_unused` classification left the network contract; networks report current references only.
- **Acceptance stage expectation (High).** The systemd stage asserted an unachievable direct-call outcome for an undiscoverable tool; it now asserts discovery omission through the smoke helper's `list-tools` mode, and the validation record was corrected.
- **Configuration validation coverage (High).** Docker configuration table tests now reject remote transports, unsafe paths, IPC collisions, invalid group names, enabled-without-required-fields, and user-mode enablement.
- **Observer adversarial coverage (Medium).** Peer-UID rejection, every socket-validation refusal, oversized and malformed daemon bodies, stalled-daemon budgets, repeated abandoned calls, exact-ceiling honesty, and hostile IPC bodies failing before engine access are covered; concurrent cold-start runs under the race detector.

Dead code (unused helpers, a dead decode callback, duplicate decode tails) was removed; the remaining consolidation candidates are tracked as follow-ups.

## Peer review round 2 — findings and dispositions

Round 2 verified every round-1 fix and found three verified issues, all fixed and covered:

- **Discovery preset leak (High).** The collector's capability preset admitted every registry tool name before the Docker decision, so disabled, ungranted, and observer-unreachable installations advertised all nine Docker tools in MCP discovery while direct calls still failed closed. The preset now skips Docker tools entirely; the real collector's `Capabilities` (not a stub) asserts omission in all three states while non-Docker discovery stays intact.
- **Daemon socket shadowing (Medium).** Configuration validation now rejects `observer_socket == daemon_socket`, so reconciliation can never bind the Docker control path; covered in the collision table test.
- **Acceptance stage ordering (High).** The systemd Docker stage assumed a reload that the restart-only rule must reject and a disablement without removing the configured intent first. The stage now restarts the services to adopt the enablement, asserts discovery omission, removes the section before the removal apply, and the upgrade stage clears the recorded restart history. Both systemd matrix cases (standard and restricted) pass end-to-end with the Docker stage, including upgrade, rollback-adjacent restarts, and uninstall.
- **Upgrade artifact coherence (High, found by execution).** Upgrades from a disabled integration now restore the observer binary from the archive unconditionally, adopt or update its ownership record, restart the observer only when it was active, and the rollback removes the replaced observer when it was absent before. A rollback manifest-consistency regression introduced by an intermediate save was caught by the existing rollback test and removed.
- Dead code removed: unused image/volume/network resolvers, the undeclared network `currently_unused` field, and stray helper binaries are now ignored by version control.

The full disposable-container suite, coverage, and live-engine re-validation ran after these fixes (see executed validation above).

## Peer review round 3 — findings and dispositions

Round 3 verified the round-2 fixes as complete and found four verified issues, all fixed and covered:

- **Drift-repair disclosure (High).** The reconcile plan and apply report now enumerate drift repairs: externally deleted identities (recreate), missing binaries (restore), and content-drifted units or binaries (rewrite/restore). A hash-drifted observer binary is no longer silently adopted; reconciliation refuses it until the operator restores the verified release with `--source`, mirroring the uninstall refusal.
- **Socket path move (Medium).** An enabled reconciliation now restarts the socket unit when its content changed, so a moved IPC path rebinds; superseded socket resources are removed by both enable and disable paths and disclosed in the plan.
- **Upgrade readiness probe (Medium).** The observer service's activation readiness is verified against its own systemd unit state instead of a false-positive diagnostics `/status` probe.
- **Rollback removal coverage (Medium).** A lifecycle test now proves the rollback removes a previously-absent replaced observer binary, marks the record removed durably, and a subsequent successful upgrade adopts it with ownership.

Both systemd matrix cases re-passed end-to-end after these fixes; the complete local suite (360 tests), formatting, lint, and strict OpenSpec validation pass.

## Peer review round 4 — findings and dispositions

Round 4 verified three of the four round-3 fixes and found two verified reporting and convergence issues, both fixed and covered:

- **Socket-path move convergence (Medium).** The enabled reconciliation now restarts the socket unit when its content changed (the earlier edit had silently failed to land; verified by probe), removes superseded socket resources on both paths, and records the configured socket state. A reconciliation test proves the move end-to-end: unit rewrite, restart command, superseded record removal.
- **Report coherence (Medium).** The disablement plan and applied report now state that dormant identity records are preserved instead of claiming removals apply never performs, and the socket state record carries an explicit state so a provisioned, unchanged system's plan no longer lists a phantom create. Report-coherence assertions extend the disablement and socket-move tests.

Both systemd matrix cases re-passed end-to-end; the complete local suite (361 tests), formatting, lint, and strict OpenSpec validation pass.

## Peer review round 5 — findings and dispositions

Round 5 verified the round-4 fixes at runtime and found two remaining issues, both fixed and covered:

- **Socket record state transitions (Medium).** A socket record left in the removed state after a disable cycle was re-disclosed as a pending create forever, and a completed path move reported creates for the superseded path instead of the new one. The enabled reconciliation now re-adopts the live-path record regardless of prior state, the superseded-path removal case precedes the stale-state create case in the plan, and tests prove the disable-and-re-enable cycle plus a completed move converge to an empty plan with the record adopted.
- **Recording fixture race (Low).** The concurrent cold-start test fixture appended to its request transcript without synchronization; the fixture is now mutex-guarded and the race detector passes, keeping the recorded evidence honest.

After these fixes the complete local suite (396 tests, race-enabled for the observer package), both systemd matrix cases, formatting, lint, and strict OpenSpec validation pass. No further findings were reported by the final verification round. The final revision re-ran the complete disposable-container race/vet/build suite and the live-engine acceptance (read-only registry plus disposable fixtures) on the Debian engine; all passed.

## Limits and deferred evidence

- The validation environments provide one CPU and 2 GiB RAM, so the full systemd container matrix cases (`systemd-restricted`, `systemd-standard`) run locally with relaxed container CPU/memory limits and in hosted CI at the pinned limits; local evidence otherwise covers the disposable checks container, live engines, and native systemd installations.
- Native acceptance used rootful Docker with the standard socket and group; nonstandard socket and group paths are covered by configuration, observer validation, and unit tests, not by a live nonstandard engine.
- Shared-kernel namespace limits still apply to disposable fixtures; container and HostLens namespace distinctions are recorded rather than removed.
- Rootless, remote, Docker Desktop, Swarm administration, Podman, and Kubernetes remain unsupported and unclaimed; the portable cross-builds are structural evidence only.
- Hosted CI for the exact pushed revision is the final acceptance gate; local evidence does not assert future workflow success.