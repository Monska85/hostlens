# Repository audit

Scope: the entire repository, including existing defects, with security and performance as primary constraints. The user authorized three local review/improvement rounds and up to three further correction rounds only for hosted CI failures. Reviewers stayed read-only during review; implementation was separately assigned.

## Round 1

Nine perspectives ran in three groups: security/adversarial; performance/idiomatic/maintainability; specification/intent/documentation/CI. The coordinator traced every retained finding. Duplicate reports are combined below; severity describes the initial problem.

| ID  | Severity | Verified issue                                                            | Disposition                                                                                                                     |
| --- | -------- | ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| H01 | High     | Secret masks defeat pathname inode comparisons for hard-link aliases      | Reject multiply linked diagnostic files; original exploit reproduced and fixed denial demonstrated in a mount-capable container |
| H02 | High     | Service names can share a UID                                             | Parse nonzero UIDs, reject collisions, and recheck created identities before writing units; focused regressions pass            |
| H03 | High     | Authentication and MCP setup bypass concurrency admission                 | Add gateway admission before token reads and request deadlines; focused race tests pass, locking challenged in round 2          |
| H04 | High     | Upgrade stop failure bypasses rollback                                    | Stop gateway before backend and establish bounded recovery before either stop; injected second-stop regression passes           |
| H05 | High     | Rebuilds retain removed or accidental staged files                        | Fresh managed staging; stale-file and failed-build preservation regressions pass                                                |
| H06 | High     | Matrix verifies archives but executes unverified expanded files/helpers   | Bind expanded contents/modes and helper hashes to the candidate; tampering regressions pass                                     |
| H07 | Medium   | Upgrade readiness probes default endpoints                                | Load trusted installed configuration; nondefault-port systemd acceptance added                                                  |
| H08 | Medium   | Token mutation can write a store larger than its read ceiling             | Reject oversized serialized candidates before replacement; preservation regression passes                                       |
| H09 | Medium   | Journal field decode failures invent empty values                         | Validate selected native types and aggregate explicit failures; parser regressions pass                                         |
| H10 | Medium   | Configuration and directory limits occur after full allocation            | Bounded regular-file reads and directory batches; FIFO, oversized-input, and directory regressions pass                         |
| H11 | Medium   | Recursive explanation enumerates whole directories before checking limits | Bounded nofollow traversal and bounded issue output; focused CLI tests pass                                                     |
| H12 | Medium   | Raw tail rejects split UTF-8 and invents an empty record                  | Trim incomplete leading records before decoding, check seek, preserve empty collections; boundary regressions pass              |
| H13 | Medium   | Status repeatedly hashes immutable policy                                 | Cache fingerprint in each snapshot; generation tests and scaling benchmark pass                                                 |
| H14 | Medium   | Python verifier permits sizes rejected by runtime extraction              | Streaming verifier with matching member and aggregate ceilings; overflow regressions pass                                       |
| H15 | Medium   | Tar EOF hides a damaged gzip trailer                                      | Drain bounded gzip streams in both verifiers; trailer regressions pass                                                          |
| H16 | Medium   | CI omits specification and Markdown checks                                | Locked tools and common local/CI gates added; local formatting/specification gates pass                                         |
| H17 | Medium   | Linux policy/default semantics live in shared logic                       | Explicit Linux compiler/default/validation entry points with private semantics; portable cross-builds pass                      |
| H18 | Medium   | Product guide points to an outdated archived authority                    | Current OpenSpec is authoritative; archives retain rationale                                                                    |
| H19 | Medium   | Validation/release docs contradict hosted evidence                        | Replace historical narrative with current procedures and scoped evidence                                                        |
| H20 | Medium   | Security fixtures ignore creation failures                                | Check fixture mutations; preserve distinct safety tests                                                                         |
| H21 | Low      | Portable token roles/types are Linux-only                                 | Inject verifier into gateway and separate portable authorization; Windows/macOS shared-package cross-builds pass                |
| H22 | Low      | Unused contract.Client interface                                          | Remove unconsumed interface                                                                                                     |
| H23 | Low      | Packaging cache argument and Main-module branch are dead                  | Remove unused argument and unreachable branch                                                                                   |
| H24 | Low      | Duplicate partial/coverage test rows                                      | Remove identical row; retain failure/partial-result coverage                                                                    |
| H25 | Low      | Archive operator guide links outside archive                              | Identify repository and archive example locations without a broken relative link                                                |

## Preserved behavior and limits

Backend timeout retains admission until underlying work ends. Releasing it early would allow unlimited abandoned I/O. Add cancellation checks between operations and document stalled-filesystem recovery; kernel-blocked I/O is an operating-system limit, not a passed cancellation guarantee.

Fixed executable paths and argument arrays already prevent a general command-injection surface. Existing tests for symlinks, special files, cached-tool authorization, source denials, reload generations, and conservative cleanup enforce different behaviors and remain useful. Broad standard-mode reads are intentional; they are not complete containment of a compromised backend.

## Toolchain scope addition

The user requested consistent developer tooling during implementation. Retain Go for runtime/native helpers, Python for standard-library build/matrix operations, and shell for container dispatch. Unify setup, formatting, lint, and check entry points with locally installed locked tools. A wholesale language rewrite would add code and break the matrix's ability to run without Go.

## Round 2

The panel rechecked fixes and challenged remaining failure paths. All twelve findings below were corrected and passed focused container tests or the applicable static checks.

| ID  | Severity | Verified issue                                                   | Disposition                                                                                                                                          |
| --- | -------- | ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| H26 | High     | Slow backend/reload work holds the admission mutex               | Separate short snapshot/admission locking from operation coordination; fail promptly during reload; slow-call and discovery deadline race tests pass |
| H27 | High     | Failed service-state queries are treated as inactive             | Require a recognized ActiveState before any upgrade mutation; query-error regression passes                                                          |
| H28 | Medium   | Uninstall removes its manifest before retained-directory failure | Preserve retry state for unexpected files and restore it on final removal races; regressions pass                                                    |
| H29 | Medium   | Token-store FIFO replacement blocks opening                      | Use nonblocking open and reject non-regular objects; FIFO regression passes                                                                          |
| H30 | Medium   | Token parser accepts trailing JSON                               | Require EOF after the single store document; trailing-data regression passes                                                                         |
| H31 | Medium   | JSONL missing/null messages become fabricated empty text         | Require a present string while preserving explicit empty messages; parsing regressions pass                                                          |
| H32 | Medium   | Raw-tail reads extend past the observed file size                | Use a fixed observed window; append/shrink regressions pass                                                                                          |
| H33 | Medium   | Discovery reports collectors whose commands cannot execute       | Require native markers and the same executable resolution used by collection; fixture regressions pass                                               |
| H34 | Medium   | Restrictive umask makes expanded staging differ from archives    | Normalize staged modes to archive modes; restrictive-umask regression passes                                                                         |
| H35 | Medium   | Missing development tools produce unclear shell errors           | Check command-specific prerequisites and provide setup guidance; dispatch regression passes                                                          |
| H36 | Medium   | Base toolchain documentation omits Python/venv and ShellCheck    | State prerequisites in one linked toolchain section                                                                                                  |
| H37 | Low      | Fixed token expiry eventually breaks acceptance                  | Generate fixture expiry from each disposable container's current clock                                                                               |

Two proposed documentation defects were rejected after tracing the actual paths: generated system configuration includes the TLS fields used by the acceptance fixture, and the established formatter includes hidden OpenSpec metadata.

## Round 3

The final panel retained four findings. No fourth local review round is authorized.

| ID  | Severity | Verified issue                                                  | Disposition                                                                                                                             |
| --- | -------- | --------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| H38 | High     | Reloaded operation limits disagree with startup HTTP deadlines  | Align network deadlines with admitted request limits; idle timeout becomes restart-only; focused container regressions pass             |
| H39 | Medium   | Dependency metadata splits cache paths containing spaces        | Preserve the directory field, exclude the main module at the producer, and reject malformed records; focused container regressions pass |
| H40 | Low      | Explain and backend CLI silently ignore surplus arguments       | Reject extra positional arguments; focused container regressions pass                                                                   |
| H41 | Medium   | Per-file limits leave aggregate configuration loading unbounded | Cap total input at 8 MiB and total profile definitions/directories at 1,024; focused container regressions pass                         |

## Validation

All 41 verified findings are resolved: nine High, 25 Medium, and seven Low. The final combined check passed formatting, lint, strict specifications, Go race/vet/cross-builds, and all 15 Python utility regressions. Both rebuilt archives passed verification and all six distribution/systemd matrix cases. A fresh runtime vulnerability scan found no vulnerabilities; GoReleaser configuration validation passed.

The source tree contains no unresolved panel finding. Native macOS/Windows runtime, remediation, native arm64 systemd execution, sustained capacity measurements, and signed release delivery remain outside this audit's implemented or established scope. Hosted evidence is reported for the exact pushed commit in the final session report; no release has been published.
