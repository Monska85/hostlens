# Implementation and validation

## 1. Fresh code-reduction cycle

- [x] 1.1 Remove verified redundant wrappers/placeholders and validate archive construction, release configuration, and container utility tests.
- [x] 1.2 Review the resulting reduction with independent contexts; resolve verified findings within five rounds and record an early no-finding closure when justified.

## 2. Fresh QA cycle

- [x] 2.1 Measure coverage and independently compare existing unit/integration/end-to-end tests with product contracts; record meaningful gaps and duplicate assertions.
- [x] 2.2 Implement equivalent Make/Just coverage commands with local reports and CI retention; verify failure propagation, artifact freshness, and container cleanup.
- [x] 2.3 Address verified test gaps and consolidate true duplication; verify retained scenarios and added edge/security cases in containers, extending end-to-end tests only where needed.
- [x] 2.4 Review QA changes independently within five rounds; close early on no findings and record actual coverage scope and results.

## 3. Final verification

- [x] 3.1 Reconcile documentation and requirements; pass static checks, external specification validation, and link checks.
- [x] 3.2 Pass combined container checks, coverage reporting, archive verification, vulnerability scan, release configuration, and the six-case matrix on the final candidate.

## Publication handoff

Publish the validated code-reduction checkpoint by amending the single main commit and pushing with an exact SHA lease. Verify hosted CI before resuming QA. After QA and final validation, amend and push again with an exact SHA lease. Verify the exact commit's hosted CI. At most three actual CI correction rounds may amend and push again. Report hosted results in the session without an extra documentation-only amendment.
