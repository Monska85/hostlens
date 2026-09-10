# Reduction and QA review

## Code reduction

The scope is the whole repository, including existing code. Reviewers used fresh contexts and the nine sf-peer-review role bodies. Initial grouped reviews were supplementary, not a substitute for the full panel.

| Role            | Result                                                                   |
| --------------- | ------------------------------------------------------------------------ |
| Security        | Unreachable policy guard; targeted verification found no remaining issue |
| Adversarial     | No findings                                                              |
| Idiomatic       | Unreachable cryptographic read error branches                            |
| Performance     | No findings                                                              |
| Maintainability | Duplicate of the policy guard finding                                    |
| Specification   | No findings                                                              |
| Intent          | No findings                                                              |
| Documentation   | No findings                                                              |
| CI/CD           | No findings; hosted execution requires the publication checkpoint        |

Verified reductions, all low severity:

- **Command wrappers.** Removed two forwarding shell scripts. Existing Make/Just release commands and the archive builder call the underlying tools directly.
- **Placeholders.** Removed `.gitkeep` files from populated specification and archive directories.
- **Policy guard.** Removed an unreachable rule-count check. The insertion limit, traversal limit, and depth limit remain enforced.
- **Random generation.** Removed unreachable error branches from two `crypto/rand.Read` calls. The required Go toolchain guarantees a complete read or process termination; the cryptographic source is unchanged.

Caller tracing and focused container race tests for policy, tokens, and diagnostics passed. Formatting, lint, both release-check entry points, and Python release/matrix tests passed. The final checkpoint passed archive construction and verification, combined container race/vet/build checks, all six container matrix cases, and strict specification validation. The reduction cycle closes after the initial panel and targeted fix verification; no further findings remain in that scope.

## QA review

The reduction checkpoint `a001b7c0f5f6b23ba3fe74cca7fe6437b99c4f4e` passed all nine jobs in [CI run 34566345999](https://github.com/Monska85/hostlens/actions/runs/34566345999). QA resumed afterward. The full panel used fresh role contexts; two earlier generic QA reviews were supplementary.

| Role            | Findings and disposition                                                                        |
| --------------- | ----------------------------------------------------------------------------------------------- |
| Security        | Role-removal false positive and absent negative peer-credential acceptance; fixed               |
| Adversarial     | Deadline-growth false positive and missing rotation-overlap assertion; fixed                    |
| Idiomatic       | Role-removal duplicate; revised rotation test needed an exact persisted-expiry assertion; fixed |
| Performance     | No findings                                                                                     |
| Maintainability | Rotation-expiry duplicate; fixed and verified                                                   |
| Specification   | Missing operational-audit assertions; fixed                                                     |
| Documentation   | No independent findings; coverage usage/scope reconciled as planned work                        |
| Intent          | No findings                                                                                     |
| CI/CD           | No findings; reviewed reporting, failure, cancellation, artifact retention, and gate behavior   |

### Verified QA findings

All findings below have high confidence. They concern test evidence, not newly demonstrated runtime vulnerabilities.

| Severity | Gap                                                                  | Resolution                                                                                                                                                 |
| -------- | -------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Medium   | A prior source denial masked ineffective token-role updates          | Restore and prove source access, require exact JSON-RPC authorization denial after downgrade, and retain permitted health access                           |
| Medium   | Rotation checked only expiry after a short sleep                     | Verify both credentials during overlap, assert the stored overlap deadline, then expire the old record deterministically                                   |
| Medium   | Deadline growth accepted any HTTP 200 response                       | Decode the complete MCP result and require observed data without protocol or execution errors                                                              |
| Medium   | Audit correlation and confidentiality had no behavioral assertions   | Exercise gateway/backend log emission, match request IDs and identity/outcome, and reject credential, path, payload, or injected-record leakage            |
| Medium   | HTTP origin and body ceilings lacked direct regression coverage      | Check permitted/rejected origins, declared/streamed oversized bodies, no collection after rejection, and released admission                                |
| Medium   | Privileged Unix peer rejection was untested                          | Require successful filesystem connection by an unrelated group-sharing UID, prompt rejection before HTTP, and authorized positive controls on both sockets |
| Low      | Provenance insertion ceiling lacked boundary assertions              | Accept exactly 10,000 configured rules and reject the next rule distinctly from traversal limits                                                           |
| Low      | Scaled IPC fixture had insufficient scheduling margin under coverage | Preserve scaled deadline relationships with a wider body-read margin; the failure was reproduced during the combined container run                         |

### Verification and corrections

A disposable mutation probe established the original role-update false positive: replacing the CLI update with a no-op still passed the old end-to-end assertion. The new assertion checks the specific authorization outcome.

The first revised rotation test manually expired the old record but did not prove that rotation stored its requested deadline. Idiomatic and maintainability reviewers caught this regression; the test now checks the exact stored expiry before advancing it.

An initial streamed-body assertion incorrectly forbade capability discovery. The SDK may discover capabilities before reading an unknown-length body; the corrected test forbids diagnostic collection and requires rejection. This was a refuted test assumption, not a runtime defect.

A Go-only coverage probe demonstrated that excluding command packages from the instrumented child build emitted no counters. Including command packages enables emission; `covdata` filters the merged report to internal packages. No coverage service or dependency was added. Go's [integration coverage documentation](https://go.dev/doc/build-cover) describes the native counter and reporting workflow.

Coverage launcher fixtures exercise success, test failure, missing reports, missing cache, and cancellation. They check failure codes, stale-report removal, temporary cleanup, and container removal. Tests execute in the existing disposable container suite. A fresh adversarial verification of the resulting QA changes returned no findings; the cycle closes after initial review and targeted verification rather than consuming five rounds unnecessarily.

### Coverage scope

The initial baseline was 67.7% of internal statements without subprocess counters or consistent cross-package execution. The merged instrumented run measured 76.6%. This difference includes instrumentation scope changes, not only new assertions. Python and platform/systemd acceptance remain outside the Go percentage. Native macOS/Windows execution and sustained production load remain unverified.

Final local validation passed: container race tests, vet, Linux builds, portable macOS/Windows compilation boundaries, six release tests, ten matrix tests, two developer-command tests, merged Go coverage, static checks, GoReleaser configuration, archive verification, and all six matrix cases. Both Make and Just coverage entry points were exercised. The final vulnerability scan found no vulnerabilities after one transient DNS failure. All 142 local Markdown targets resolved and strict current specifications passed.

Observed merged profile entries include gateway CLI serving at 80.0% and diagnostic Main at 64.9%, confirming subprocess counters reached the report. This does not measure separate systemd-helper execution. The final amendment's hosted result is reported in the session; no extra documentation-only amendment is required.
