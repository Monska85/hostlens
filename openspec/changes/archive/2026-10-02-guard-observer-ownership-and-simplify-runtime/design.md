# Design

## Context

See [proposal.md](proposal.md). Reconciliation currently creates manifest entries before checking whether an observer path was already present. The observer is the only HostLens process with Docker socket authority, so ownership checks must precede identity and unit changes. Configuration validation is shared by startup and reload; Linux path rules remain in the Linux adapter.

## Goals / Non-Goals

**Goals:** Preserve unowned observer resources; reject conflicts before mutation; reduce memory used by hash-only file checks; keep measured runtime work within existing ceilings.

**Non-Goals:** Change Docker policy meanings, add a remediation process, implement macOS or Windows collectors, or alter published tool schemas.

## Decisions

1. Reconciliation checks the manifest record and filesystem type for each observer-owned path before entering the mutating path. An absent record allows creation only when the path is absent. A removed record with a newly present path is also a conflict. The existing identity checks remain in place. This is simpler than attempting to infer ownership from filenames, file contents, or service definitions, which cannot establish who created a resource.
2. A recorded binary with a checksum is hashed before any observer mutation. Unexpected content is preserved for administrator review. Unit content drift remains repairable when the unit has an owned record, because the generated text is available and reconciliation already repairs those units.
3. An apply that needs a missing observer binary checks for a regular source binary before creating identities. Dry run may still show the plan and required source. This keeps the review path available while preventing a partial apply caused by a known missing input.
4. Linux validation compares the diagnostic and administrative socket paths independently of Docker configuration. Docker observer path checks continue to apply when that path is configured.
5. Hash-only checks reuse the existing streamed checksum helper. Docker inventories evaluate their collection grant once per response and continue checking each item's stable identity and names for denials. Keep this change only if the same bounded benchmark improves and policy-denial tests remain unchanged.

## Risks / Trade-offs

- Existing installations with unrecorded observer files now require administrator inspection and removal or ownership recovery before reconciliation. The error names the conflicting resource; it never deletes it.
- Filesystem state can change between preflight and mutation. Recheck each path immediately before writing or enabling its service; preserve a detected conflict.
- More preflight reads add a small cost to reconciliation. They avoid adopting executable content and hold no diagnostic evidence.

## Migration Plan

No automatic migration is required. If an unrecorded observer resource exists, the administrator resolves the conflict and reruns the plan. Existing recorded installations continue to reconcile normally.
