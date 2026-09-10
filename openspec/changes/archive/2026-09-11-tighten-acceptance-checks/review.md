# Follow-up review resolution

All seven findings from the reread of Herdr tab 4 are addressed. No commit or push was performed.

| Finding                          | Resolution                                                                                                                                                                              |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Systemd image mismatch           | Dispatcher passes the selected image explicitly; one environment override is applied there. Regression covers both paths.                                                               |
| Inconsistent checksum parser     | Removed the Go parser. Coreutils validates lists in containers; focused cases expose only their selected archive to the checker. Conflicting entries fail.                              |
| Duplicate verification container | Archive verification now uses the existing test-container entry point. Removed the dispatcher's archive-only mode.                                                                      |
| Circular dispatch                | Platform launcher requires one explicit image/architecture case. Updated prerequisite regression.                                                                                       |
| Weak smoke assertions            | Validate structured OS, architecture, package versions, fixture content, journal messages and issue codes. Failure output excludes inspected payloads.                                  |
| Fixed startup sleeps             | Bounded socket/admin/HTTP readiness checks replace delays. Journal fixture waits for completion and synchronization. Unexpected HTTP outcomes and security assertions fail immediately. |
| Duplicated defaults              | Example configuration now specifies only version, system mode and standard privilege; runtime defaults provide other settings.                                                          |

## Evidence

Go race tests, vet, portable builds, eleven Python regressions and structured smoke regressions passed in disposable containers. Internal statement coverage remains 76.7%. Packaging and both archive verifications passed. All six acceptance cases passed again after the final HTTP readiness integration.

A disposable checksum fixture verified conflicting-entry rejection and focused selection. A separate disposable readiness fixture checked HTTP 401 success and immediate rejection of HTTP 500 and redirects. Static checks and current/archived specification validation passed. The final read-only review reported no concrete remaining findings.

No native ARM64 hosted execution or publication is claimed for this uncommitted tree. ARM64 acceptance used emulation; containers share the host kernel.

## Maintenance accounting

Across all repository code, tests and configuration, including new files and excluding documentation/OpenSpec history: committed HEAD has 11,240 lines; this tree has 11,089. Net reduction is **151 lines**.

This pass adds **85 lines** relative to the previous pass's 11,004. Removing orchestration and duplicated defaults did not fully offset the structured assertions, readiness checks and regression tests. These additions verify behavior the previous substring checks and sleeps did not establish.
