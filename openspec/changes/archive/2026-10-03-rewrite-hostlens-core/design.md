# Design

## Context

See [proposal.md](proposal.md). The existing tree has 12,884 production Go lines and 14,850 Go test lines in `cmd` and `internal`. It separates an HTTP gateway, diagnostic backend, and Docker observer, but also owns substantial MCP schema, Docker transport, procfs parsing, policy, and lifecycle code. Published 0.5.0 installations and clients exist, so this rewrite must make its breaking contract and migration explicit.

## Goals / Non-Goals

**Goals:** Produce a smaller implementation of the five new capability specifications; keep observations request scoped and failures honest; separate repair authority; ship working Linux amd64 and arm64 artifacts; make native platform implementations replaceable without Linux behavior in shared code.

**Non-Goals:** Preserve every 0.5.0 MCP tool or field, expose arbitrary commands or Docker methods, implement macOS or Windows collectors now, or claim a speedup from line-count reduction alone.

## Decisions

1. **Typed MCP catalog.** Use the official Go MCP SDK's generic `AddTool` registration for input and output schema inference and validation. Register each tool once with its effect and profile capability. Keep a small dispatch gate in front of each handler and use it for both discovery and execution. Remove parallel schema builders and gateway/backend validation copies. The alternative is extending the current registry, which preserves duplicate protocol machinery.
2. **Native adapters.** Define narrow `HostStatus`, `Services`, and `Containers` interfaces around observations and targeted restarts. Linux uses gopsutil for host facts, go-systemd for service state and restart, and Moby's Go client and API types for Docker. Runtime wiring chooses Linux implementations; absent macOS or Windows implementations fail clearly at startup. Do not create generic filesystem, command, or Docker-path interfaces. The alternative is a single platform package with build-tag branches through shared policy and MCP code.
3. **Separate observation and repair authority.** The public gateway authenticates calls and handles generic host observations without mutation authority. A dedicated Docker observer holds access to the mixed Docker API but exposes only named read operations. A repair worker has a distinct identity and access to the native mutation APIs. The worker independently checks the current token role and activated target grant and re-reads target identity and state before applying a named restart. Read-only activation drains or cancels admitted repair work before success. The alternative of putting restarts in the gateway or Docker observer creates a mutation path across the read boundary.
4. **Profiles as decisions, not source parsers.** A profile activates named tool grants and target selectors. Denials win over grants, and read and repair effects are distinct. Profiles cannot supply shell commands, file contents, or arbitrary API paths. Use existing YAML parsing only for this small configuration. The alternative of preserving broad file and audit policy retains much of the present product and code surface.
5. **Bounds at every external edge.** Limit HTTP request and response bytes, work admission, deadlines, Docker list sizes, and serialized IPC frames before decode. A named-only Docker observation interface sits in front of the mixed read-write API. Keep platform helpers' results local to the admitted request; do not retain evidence or treat timeout as completed work. Use standard `os.Root` for filesystem operations where path confinement is required. Libraries replace protocol and system parsing, not the product's bounds or authority checks.
6. **Migration and delivery.** Keep the 0.5.0 tag and baseline commit as immutable references. Build and test from the isolated rewrite worktree. Provide a migration command that reads old configuration and credentials, reports unmappable profile grants, and writes a separate versioned candidate without touching services. Packaging supplies one binary, profile examples, systemd units, manifest, checksums, and current docs. Operators preserve the old binary and state until candidate validation and manually switch services. Container acceptance exercises a failed candidate and restoration of known-good state. All lifecycle and live Docker fixture tests use disposable containers only.
7. **Review timing and Git.** Complete the full rewrite and deterministic gates first. Then run eight peer-review panel rounds, selecting security, performance, Go idiom, maintainability, specification, adversarial, docs, and CI experts according to the changed surface. Verify and fix findings, rerun affected gates, and create one final conventional commit. No intermediate commits or host service changes.

## Risks / Trade-offs

- Library calls may reopen paths or decode unbounded data. Check the selected API and place limits at the transport or request boundary; reject an API that cannot meet the bound.
- A Docker socket grants broad daemon authority to its holder. Keep it out of the gateway and host observation adapter, expose only named operations from the separate Docker observer, and document the remaining observer compromise risk.
- A smaller tool catalog breaks clients. Publish a mapping from old to new tools and an explicit migration procedure; do not silently return fabricated compatibility fields.
- Manual installation places more responsibility on the operator. Document protected file modes, identity setup, activation order, and rollback, and exercise candidate failure and restoration in a disposable systemd container.
- Latest stable packages may change during the rewrite. Recheck official tags, maintenance, security notes, and Go compatibility before the final module tidy and container build.

## Migration Plan

Build the new candidate alongside the old release, validate old configuration and credentials with the migration command, and test install and rollback in disposable containers. Package and verify both Linux architectures. On installed hosts, require administrator review of the generated candidate and action grants before manually replacing services. Restore the preserved 0.5.0 binary and state together if activation fails. Do not publish or claim production readiness until all required gates and review rounds complete.
