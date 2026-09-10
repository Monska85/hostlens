# Hardening tasks

## 1. Runtime correctness and boundaries

- [x] 1.1 Reject hard-linked diagnostic inputs and bound trusted configuration reads; verify denied aliases, oversized files, and directory ceilings in containers.
- [x] 1.2 Bound gateway admission and request lifetime, cache snapshot fingerprints, and remove unused interfaces; verify overload, cancellation, and reload behavior with race tests.
- [x] 1.3 Bound recursive policy explanation and correct raw-tail parsing; verify entry limits, empty logs, and split UTF-8 records.
- [x] 1.4 Separate portable authorization from Linux persistence and document native policy boundaries; verify supported builds and portable-package cross-builds.
- [x] 1.5 Fix verified lifecycle and token persistence failures; verify identity separation, rollback, actual configured readiness, and storage ceilings.
- [x] 1.6 Check security-test fixture creation and remove only duplicate assertions; run the affected tests in containers.

## 2. Build and documentation

- [x] 2.1 Build fresh archive staging and bind matrix inputs to verified artifacts; verify stale-file and executable/helper substitution regressions.
- [x] 2.2 Align archive verification and extraction limits and validate gzip completion; verify malformed and oversized archive rejection.
- [x] 2.3 Add matching Make/Just documentation checks and CI specification/format gates; verify runner dispatch, workflow lint, and checks.
- [x] 2.4 Reconcile README, product, operator, release, and validation docs; add concise writing rules to AGENTS.md and remove generated configuration commentary. Verify links, formatting, and agreement with current requirements.
- [x] 2.5 Normalize local and CI tool setup, formatting, linting, and checks across Go, Python, shell, documentation, and configuration; verify locked tools, matching Make/Just commands, and failure propagation.

## 3. Review and acceptance

- [x] 3.1 Triage every round-one panel finding with evidence and a disposition in the audit record.
- [x] 3.2 Complete the second independent review round, implement verified findings, and rerun affected checks.
- [x] 3.3 Complete the third independent review round, resolve verified findings within the authorized limit, and report any remaining limitations.
- [x] 3.4 Pass container race/vet/build and full distribution/systemd matrix acceptance, vulnerability scanning, static checks, and strict specifications; record actual results.

## Publication handoff

After all implementation tasks pass, inspect the final diff, amend the initial commit once, and push main with an explicit expected remote SHA lease. Verify hosted CI for that exact commit; use at most three CI correction rounds if required. Record hosted outcomes in the final session report, since successful CI alone does not authorize another documentation-only amendment.
