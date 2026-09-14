# Design: unify-fallback-tool-admission

## Context

Per-call admission uses `toolAdmitted(definition, known, readOnly)`; the backend-loss fallback checked only role membership. See proposal.md — Why.

## Goals / Non-Goals

- Goals: one admission predicate everywhere; the gate provable without depending on the current registry contents.
- Non-Goals: no behavior change for today's registry; no new tool classes.

## Decisions

- **Reuse `toolAdmitted` directly** rather than a parallel check: drift between the two paths was the defect; sharing the function prevents it.
- **Prove with an unknown tool** in the fallback path plus the existing direct `toolAdmitted` remediation/unknown assertions: the registry cannot currently express a non-diagnostic known tool, so the unknown-tool case exercises the same `known` leg and the envelope must not appear.

## Risks / Trade-offs

- [Unknown tools now fall through to the SDK handler during backend loss instead of receiving a failure envelope] → Matches the normal admission path's denial shape for non-admitted calls.

## Migration Plan

None. Rollback is a plain revert.

## Open Questions

None.