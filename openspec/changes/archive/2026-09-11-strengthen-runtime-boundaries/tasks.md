# Audit implementation tasks

## 1. Verified runtime corrections

- [x] 1.1 Preserve service-health observations independently of pagination and partial coverage; verify large service lists and mixed failure/denial fixtures in containers.
- [x] 1.2 Keep native capability collection outside state locks and bound stalled discovery; verify admission, activation, cancellation, and bounded work with container race tests.
- [x] 1.3 Validate adopted service groups before mutation and after creation; verify zero/colliding IDs and unrelated explicit members are rejected without installing files.
- [x] 1.4 Apply other verified security, idiomatic, performance, and consistency findings; record each disposition and its focused verification in review.md.

## 2. Independent review rounds

- [x] 2.1 Complete round one triage and fixes; retain proof and focused results.
- [x] 2.2 Complete round two triage and fixes; retain proof and focused results.
- [x] 2.3 Complete round three triage and fixes; retain proof and focused results.
- [x] 2.4 Complete round four triage and fixes; retain proof and focused results.
- [x] 2.5 Complete round five closure and fixes; record unresolved limitations without a sixth local review.

## 3. Integration and documentation

- [x] 3.1 Reconcile affected docs and current specifications; verify formatting, links, and strict current/archive validation.
- [x] 3.2 Pass the combined container race/vet/build, static/tooling checks, vulnerability scan, archive verification, and full six-case matrix; record actual evidence.

## 4. Fresh-eyes follow-up cycle

After the first cycle, start independent reviewers with fresh contexts and no preferred review angle. Up to five further review/improvement rounds are authorized. A no-finding result is valid; investigate suspicious behavior without inventing issues. Record actual rounds, fixes, and validation in the audit record before the same single final publication handoff.

- [x] 4.1 Complete the fresh-context follow-up cycle and resolve verified findings; record round outcomes and final checks.

## Publication handoff

After implementation and validation, inspect the final diff, amend the current initial commit once, and push main with an exact remote SHA lease. Verify hosted CI for that commit; at most three CI correction rounds may amend and push again. Record hosted results in the final session report without an extra documentation-only amendment.
