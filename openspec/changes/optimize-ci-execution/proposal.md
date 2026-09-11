## Why

CI repeats compiler work and test-image preparation, and superseded runs continue consuming runners. Reduce elapsed time and runner usage while preserving the existing validation coverage, isolation and exact-candidate release contract.

## What Changes

- **Cancel obsolete CI:** Superseding branch/PR updates cancel their earlier ordinary CI runs without cancelling unrelated runs or release validation.
- **Reuse preparation:** Cache validation/systemd image layers and container Go compilation with explicit compatibility and trust boundaries.
- **Measure before tuning:** Record baseline, cold-cache and warm-cache timings; distinguish elapsed time, queue delay and total runner minutes. Tune matrix parallelism and remove redundant setup only where measurements justify it.
- **Preserve behavior:** Keep all maintained acceptance cases, native hosted ARM64, container isolation, readable logs, equivalent Make/Just commands and publication of the tested archives.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation`: Add safe cache reuse, scoped supersession and evidence-based CI performance acceptance.

## Impact

Primary areas are `.github/workflows/ci.yml`, its interaction with `release.yml`, explicit image preparation, and the container test/scanner launchers. Documentation and focused tooling regressions will describe cache controls and cancellation behavior. Use maintained GitHub/Docker actions and Go tooling; do not add a custom scheduler or cache framework.

GoReleaser, archive staging, the upgrade manifest and product behavior remain unchanged. This entry authorizes planning only; implementation and publication are separate work.
