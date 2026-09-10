## 1. Shared execution

- [x] 1.1 Define and validate the shared matrix; test target selection, invalid inputs, and GitHub JSON output in a container.
- [x] 1.2 Implement bounded execution, separate logs, failure aggregation, and cancellation; verify with controlled process fixtures and actual container cases.
- [x] 1.3 Record build-input fingerprints and reject stale or missing candidates; verify changed-source and valid-build scenarios.

## 2. Developer and CI integration

- [x] 2.1 Add equivalent independent Make and Just recipes; verify focused dispatch and list output from both runners.
- [x] 2.2 Split GitHub checks, build, matrix, and aggregate gate while preserving release artifact reuse; validate workflow syntax and hosted results.
- [x] 2.3 Update current specifications and documentation, run local matrix coverage, and record verified results and limits.
