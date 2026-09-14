# Design: align-audit-link-comment

## Context

`auditLink` reads link targets via `readlinkat`; for executables it re-checks the resolved target against file policy with builtins default-allowed. See proposal.md — Why.

## Goals / Non-Goals

- Goals: make the comment state the real policy semantics so operators read the truth.
- Non-Goals: no behavior change; `builtin=false` tightening deferred (would alter stock collection behavior).

## Decisions

- **Correct the comment, keep `builtin=true`** (plan-selected): the built-in sources are protected observation sources whose executable paths belong to the OS, not the inspected application; explicit policy denials still apply.

## Risks / Trade-offs

- [A future reader expects the comment to promise target-policy approval] → The corrected comment states the default-allow semantics and the exposure (path string only).

## Migration Plan

None.

## Open Questions

None.