# Runtime boundary decisions

## Context

The prior audit established container acceptance and closed its recorded findings. This cycle checks remaining behavior, including health's reuse of paginated service output, native work under state locks, and group adoption that currently checks existence only.

## Goals / Non-Goals

Preserve observable facts and fast admission under failure. Keep changes specific to verified findings; do not introduce future-platform frameworks or remove tests that protect distinct behavior.

## Decisions

- Separate bounded service collection from public pagination. Health can consume the same collected observations without launching repeated commands or losing failures after a page boundary.
- Copy immutable state under a short lock, then perform native discovery outside it. Bound discovery workers independently so client cancellation does not create unlimited kernel-stalled work.
- Parse group records alongside passwd records. Reject numeric identity conflicts and unrelated explicit group members before granting state access. Preserve pre-existing accounts and groups on rejection.
- Retain pinned Go/Python/shell development tools and shared Make/Just dispatch. OpenSpec is an external CLI; Markdown formatting belongs to the agent harness. Remove repository npm manifests, Prettier integration, and local copies of harness tools. Agents validate project specifications with the system CLI; CI runs project checks without OpenSpec or Node.js.

## Risks / Trade-offs

- Kernel-stalled filesystem work cannot be killed safely by context cancellation. Keep its bounded admission occupied until it returns and report the limitation.
- Stricter group adoption may reject an existing administrator arrangement. Report the identity conflict without silently changing group membership.
- Performance changes must retain denial precedence, provenance, and observed failure reporting. Use behavior regressions and focused benchmarks instead of timing assumptions.

## Migration Plan

Apply through the existing explicit upgrade/restart workflow. No configuration migration or runtime dependency upgrade is planned. Keep all changes on main and amend once after local validation; use the user's separate CI correction allowance only for hosted failures.
