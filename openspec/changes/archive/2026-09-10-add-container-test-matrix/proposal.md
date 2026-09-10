## Why

Local and GitHub acceptance tests currently run platform and lifecycle cases sequentially. A shared matrix will expose individual failures in CI and let developers run one target quickly or all targets with bounded parallelism.

## What Changes

- **Shared cases.** Define platform and lifecycle targets once for local commands and GitHub's job matrix.
- **Focused checks.** Accept an optional target in independent Make and Just commands; reject unknown names before starting containers.
- **Parallel execution.** Bound local concurrency, preserve separate case logs, and fail the overall run when any case fails.
- **Artifact reuse.** Build candidates once before CI matrix jobs; local checks reuse existing verified artifacts and reject stale build inputs.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation`: Selectable shared matrix coverage and aggregate release gating.

## Impact

Changes test scripts, task runners, archive build metadata, GitHub CI, and developer documentation. Product behavior and archive upgrade format remain unchanged. Tests continue to use disposable containers; no VM is provisioned.
