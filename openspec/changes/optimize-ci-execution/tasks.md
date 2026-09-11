## 1. Baseline and compatibility

- [ ] 1.1 Record comparable baseline job/step timings, queue delays and total execution minutes with run links, commit IDs, runner labels and sample counts; inventory all current checks and matrix cases in the measurement report.
- [ ] 1.2 Verify the chosen official action versions, cache compatibility inputs and ref/event trust rules against current documentation; record the cache namespaces and release/PR access matrix in the design.

## 2. Scoped cancellation

- [ ] 2.1 Add ordinary-CI supersession groups and release/reusable-workflow isolation; pass workflow lint and verify same-branch supersession, unrelated-branch independence and release isolation through controlled hosted runs when publication is authorized.
- [ ] 2.2 Verify an interrupted acceptance case terminates its disposable container and leaves a cancelled/failed gate, with a focused regression for any changed launcher behavior.

## 3. Reusable preparation

- [ ] 3.1 Add standard Buildx layer caching for explicit validation and systemd image preparation; verify cold and warm builds produce runnable images, and local preparation still works without GitHub credentials.
- [ ] 3.2 Add optional, scoped compiler-cache mounts for container tests and vulnerability scanning; verify cache hits, disabled/missing-cache fallback, incompatible-toolchain separation and unchanged source/module mount restrictions.
- [ ] 3.3 Verify cache trust isolation and forced test execution: a deliberately failing disposable fixture must fail after a warm-cache run; untrusted cache writers must not populate trusted release restore namespaces. Keep freshly built helpers scoped to the selected checkout.

## 4. Measured tuning and acceptance

- [ ] 4.1 Compare equivalent cold and warm candidate runs with baseline evidence, separating elapsed time from execution minutes; record sample counts, cache/image identities, variance and limitations without inventing unavailable timings.
- [ ] 4.2 Evaluate matrix queue time and the current parallelism cap; retain or adjust the limit and remove only measured redundant setup, documenting observed benefit and verifying unchanged case selection and failure aggregation.
- [ ] 4.3 Run the established formatting/lint, race/vet/coverage, archive and full container matrix checks on the final implementation; verify native hosted ARM64, both privilege modes, application cases and exact tested-archive handoff remain intact. Do not publish a release merely to test the workflow.

## 5. Documentation and completion

- [ ] 5.1 Document cache controls, local fallback and measured results in the existing developer/validation guides; verify Make and Just remain equivalent, Markdown is formatted and OpenSpec validation passes.
- [ ] 5.2 Review the final workflow for unnecessary machinery, security regressions and unsupported performance claims; record findings and verified dispositions, then synchronize and archive the completed change only after acceptance evidence exists.
