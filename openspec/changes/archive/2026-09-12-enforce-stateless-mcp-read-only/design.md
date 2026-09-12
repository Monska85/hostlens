## Context

HostLens currently registers its diagnostic tools directly in the gateway and forwards accepted calls over a diagnostic-only backend contract. Configuration already rejects remediation in v1, while local administrative commands use a separate local interface. The current on-demand contract prohibits metric history and inspected-log retention, but it does not define a complete diagnostic evidence boundary or a reusable tool-effect gate for future remediation and container runtime integrations.

This change crosses configuration loading and fingerprinting, coordinated reload, MCP registration and dispatch, IPC contracts, audit output, tests, documentation, and agent guidance. See `proposal.md` for motivation and the delta specs for observable requirements.

## Goals / Non-Goals

**Goals:**

- Make read-only behavior a default-on, global MCP invariant enforced from one effect registry.
- Keep diagnostic evidence request-scoped while preserving the control-plane state required to operate and audit the service.
- Make local administration independent from the MCP gate.
- Establish authority boundaries that later remediation and mixed read-write infrastructure APIs cannot bypass.
- Make both invariants testable at registration, discovery, dispatch, backend, persistence, reload, and release-validation boundaries.

**Non-Goals:**

- Add remediation tools, roles, policies, credentials, or processes in v1.
- Add Docker or another container runtime collector in this change.
- Remove configuration, credentials, installation ownership state, service metrics, or payload-free operational audit records.
- Promise that a source system performs no internal accounting when its read API is called.
- Scrub transient response bytes from process memory beyond releasing HostLens references when the admitted collection worker exits.

## Decisions

### Default-on configuration with eligibility semantics

Add `MCP.ReadOnly bool` to the effective configuration, mapped from `mcp.read_only`. Defaults establish `true` before strict YAML decoding, so omission is safe and explicit `false` remains distinguishable through the resulting value. Include the value in the effective-policy fingerprint and local status.

`false` removes only the global read-only ceiling. It does not grant a role, change policy, prove runtime capability, or enable remediation. V1 continues to reject `remediation.enabled: true`, so either flag value produces the same diagnostic-only public surface today.

The alternative was to couple the setting to `remediation.enabled`. That makes one switch carry two meanings and prevents the gate from remaining a final safety ceiling after remediation exists.

### One exhaustive effect registry

Replace parallel tool-name lists with one registry entry per MCP tool. Each entry contains its name, effect, role or permission requirement, schema, description, and capability identity. Derive MCP registration, discovery checks, and direct-call admission from these entries.

Use a closed effect type with only `diagnostic` and `remediation`. Registry construction rejects an empty or unknown effect, duplicate names, and handlers outside the registry. Tests compare registered tools with the registry and exercise every entry under both read-only values.

The alternative was to hide remediation tools only during discovery. MCP clients can retain names and call them directly, so discovery filtering alone cannot enforce the boundary.

### Reject before crossing an authority boundary

The gateway evaluates the read-only gate before invoking backend capability discovery or execution. Backend IPC remains an allowlisted diagnostic protocol in v1 and also rejects unknown operations. The gateway identity has no direct host-read or mutation authority.

Future remediation uses a separate process or service identity, a distinct IPC operation family, dedicated credentials, a remediation-specific role, and explicit policy grants. The diagnostic backend never receives those credentials or a generic forwarding operation. An integration such as a system-wide container runtime, whose native endpoint combines read and write methods, needs a fixed observer broker or equivalent constrained boundary that exposes only named read operations to diagnostics.

The alternative was to give a diagnostic process a broad external API credential and rely on call-site discipline. That cannot provide a defensible read-only architecture after a collector defect or gateway compromise.

### Atomic transition into read-only mode

Track remediation admission separately from diagnostic admission when a remediation process is eventually introduced. Preparing a generation that changes `mcp.read_only` to `true` first closes remediation admission. Activation succeeds only after admitted remediation finishes or confirms cancellation within the existing bounded reload operation. If that cannot be established, reload fails and atomically restores the previous generation's admission state.

Diagnostic reads keep the current generation semantics and may finish under the generation that admitted them. Because v1 contains no remediation execution path, the initial implementation proves this transition with a mutation-sentinel test double and keeps production behavior simple.

The alternative was to activate the configuration as soon as new admission was blocked. That would let status report read-only mode while an older write was still changing the host.

### Request-scoped evidence ownership

Collectors read live permitted sources for every call. Results, parsed records, derived assessments, inventory pages, cleanup candidates, and any future plan belong to the active collection worker and lose all service-owned references when that worker exits. Client timeout and cancellation signal the worker but cannot synchronously erase a goroutine blocked in an uninterruptible OS read; such a worker remains admission-bounded, visible as active work, and unable to expose its partial result to another request. Pagination offsets describe a new bounded observation and do not create server-side snapshots.

HostLens does retain the control plane required to operate safely:

- **configuration state.** Active validated configuration, policy definitions, fingerprints, and generations.
- **security state.** Credential records and authorization metadata.
- **lifecycle state.** Installation ownership records and retained rollback artifacts.
- **coordination state.** Bounded admissions, deadlines, cancellation, and active request bookkeeping.
- **operational state.** Aggregate in-memory service counters and payload-free audit metadata captured by the service manager.

None of this state can contain or reconstruct inspected source content, returned observations, resource inventories, cleanup candidates, or remediation plans. The alternative was to describe HostLens as having no state at all, which would conflict with authentication, safe lifecycle rollback, atomic reload, and accountable operations.

### Audit metadata remains payload-free

Keep the existing structured audit path and service-manager retention. Audit fields stay limited to request correlation, authenticated identity when known, tool name, generation, duration, outcome, and bounded aggregate counts. Service metrics remain aggregate and use no source or resource labels.

Tests inject unique markers into arguments, sources, partial results, failures, and cancellation paths. They then inspect logs, metrics, service-owned files, and retained component state. This tests the evidence boundary without pretending that Go runtime memory is synchronously erased.

### Agent instructions preserve the invariants

Add concise rules to `AGENTS.md` during implementation. They will require live request-scoped diagnostics, the central MCP effect gate, separate mutation authority, and constrained infrastructure observers. They will also state that container runtimes are infrastructure evidence sources rather than application-specific production collectors.

## Risks / Trade-offs

- **Boolean default ambiguity.** Go boolean zero values can accidentally turn omission into `false`. Initialize defaults before strict decoding and add omission tests for every supported config-loading path.
- **Registry drift.** A second registration path could bypass classification. Keep registration constructors private to the gateway package and test equality between the runtime surface and the registry.
- **Reload deadlock.** Waiting for remediation while holding generation locks could block completion. Close admission and wait outside shared state locks under the existing reload deadline.
- **Read APIs with side effects.** Some source systems update access metadata or internal counters on reads. Document the guarantee as HostLens issuing only approved observation operations, and isolate mixed APIs behind a constrained observer.
- **Sensitive audit expansion.** Future fields could turn logs or metrics into evidence stores. Centralize allowed audit fields, prohibit dynamic source-derived labels, and retain unique-marker tests.
- **False confidence before remediation exists.** V1 can prove classification and non-invocation but cannot validate a production remediation service. Require the same invariant suite to cover that service before any remediation capability ships.

## Migration Plan

1. Add the configuration field with a default of `true`, then update examples, status, and policy fingerprinting.
2. Introduce the exhaustive effect registry and route existing diagnostic registration, discovery, and dispatch through it.
3. Add admission hooks for atomic read-only activation and validate them with a mutation-sentinel test double while v1 has no remediation backend.
4. Audit component fields, logs, metrics, and files for diagnostic retention. Remove any retained evidence and add marker-based regression coverage.
5. Update operator documentation and `AGENTS.md`, then run focused tests, repository checks, strict OpenSpec validation, and disposable-container acceptance.

Existing configurations migrate safely because omission resolves to `mcp.read_only: true`, matching the current diagnostic-only surface. Rollback removes the new field from configuration examples before using an older binary; unknown-field validation would otherwise reject it. A code rollback restores the previous diagnostic-only implementation and does not expose remediation.
