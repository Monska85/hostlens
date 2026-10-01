# Design

## Context

HostLens has 12,669 production Go lines under `internal/`. The largest files coordinate Docker diagnostics, general Linux collection, and lifecycle changes. The current MCP contract, Linux privilege model, stateless evidence rule, and OpenSpec requirements are the compatibility baseline.

## Goals / Non-Goals

**Goals:** Reduce production code and measured request cost while keeping every current security and compatibility guarantee. Make existing portable packages remain buildable for macOS and Windows. Keep future remediation behind a separate authority boundary.

**Non-Goals:** Add macOS or Windows runtime support, add a remediation tool or credential, change published MCP member names, replace the official MCP SDK, or weaken a limit to meet a line target.

## Decisions

- Compare a clean baseline and each candidate with the same Go version, container image, fixture, and workload. Record production lines separately from tests and documentation. Use allocations and latency distributions for request paths; use source inspection for trust-boundary changes. A shorter function is not evidence of faster or safer behavior.
- Refactor by complete behavior slice, keeping old and new behavior covered by the same contract tests before deleting the old path. A single wholesale rewrite was considered but would make failures difficult to localize in a public product.
- Keep the typed tool registry as the sole source of schemas and effects. Simplify dispatch and result flow around it; do not add hand-written schemas or generic mutation operations.
- Share only platform-independent semantics. Linux path traversal, peer credentials, systemd, capabilities, and collectors remain native. Portable packages continue to cross-build for macOS and Windows without pretending those systems are supported at runtime.
- Preserve a distinct future remediation process, identity, credential, IPC contract, role, and policy. Diagnostic processes stay observation-only. The effect gate must continue to reject remediation-class calls and read-only activation must be able to drain or cancel any future admitted mutations before succeeding.
- Reserve the remediation role in the typed registry without admitting it to v1 token issuance. A future remediation tool definition must pair its effect with that distinct role; diagnostic roles cannot be reused for it.
- Reuse bounded observation and policy primitives where they replace real duplication. Do not cache diagnostic evidence or create abstraction layers solely to reduce line count.
- Measure production Go lines against both the 12,669-line pre-refactor baseline and `origin/main`. A 15% reduction was the initial stretch target, but it is not a release gate: explicit typed schemas and the added trust checks must remain even when they cost lines. Report the exact result and the missed target. Reject a reduction that drops a security check, hides a coverage gap, or causes a material workload regression.

## Risks / Trade-offs

- Cross-cutting refactoring can change error classification or optional JSON members. Snapshot, scenario, and archive acceptance must catch drift before delivery.
- Shared helpers can erase source-specific security checks. Each helper needs explicit ownership, limits, and failure semantics, with direct boundary tests.
- Microbenchmarks can reward an isolated path while end-to-end behavior slows. Compare representative MCP calls and large bounded inventories in the same disposable environment.
- Future platform and remediation seams can add code without current value. Preserve narrow contracts already required by the specifications; do not implement unused native adapters or mutation authority.

## Migration Plan

Keep the public contract and on-disk formats stable. Build the gateway, backend, observer, and CLI as one coherent archive. Validate the exact archives in disposable platform and lifecycle cases. Roll back by restoring the previous coherent release archive; no data migration is planned.
