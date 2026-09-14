# Design: version-coherent-shipped-docs

## Context

`prepare.py` copies four Markdown docs into each archive staging tree verbatim. Repository docs keep development-state placeholders; shipped copies must match the archive. See proposal.md — Why.

## Goals / Non-Goals

- Goals: shipped docs always name the archive they ride in; historical prose untouched.
- Non-Goals: no templating engine, no regex over prose; only exact archive-name placeholder forms are substituted.

## Decisions

- **Exact-byte placeholder pairs** (`hostlens-0.1.0-dev-`, `hostlens-0.1.0-linux-`) instead of a generic regex: substitution cannot corrupt historical prose, code blocks, or unrelated numbers, and every substitution is reviewable.
- **Fix the repository's stale `0.2.0` literal** rather than adding a placeholder for it: the repo-side doc should describe the development state; the build-time substitution then handles all future versions.

## Risks / Trade-offs

- [A future doc literal uses a new form the substitution misses] → The release test asserts candidate-version coherence on shipped copies, so a miss fails the suite rather than shipping silently.

## Migration Plan

None. Rollback is a plain revert.

## Open Questions

None.