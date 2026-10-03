# Rewrite peer-review record

Review scope is the staged rewrite against `main`, including the active change, packaged units, migration, and acceptance scripts. Each round uses the named peer-review panel body. Findings are checked against the implementation before fixes; later rounds see the fixes from earlier rounds. No reviewer edits the tree.

## Round 1: Security and adversarial

- **High, verified:** The repair IPC accepted a token ID that the gateway identity could read from token metadata. A gateway process could claim another client's repair role without that client's bearer secret. The gateway now forwards only the current request's bearer credential over protected IPC, and the repair worker verifies it afresh. A token ID without the secret is refused in a container test.
- **Medium, verified:** Docker's pre-filter truncation flag disclosed a count-derived signal from denied containers. Inventories now report a constant 500-row source limit and make no completeness claim. The typed result snapshot and authorization test were updated.
- **Validation:** Race tests, private nested Docker acceptance, disposable systemd repair, archive verification, formatting, lint, and strict OpenSpec validation were rerun after the fixes. Native arm64 runtime remains a CI gate.

## Round 2: Go idiom and maintainability

- **Medium, verified:** The observer and repair worker repeated the same Unix peer credential extraction. Both now use one small Linux IPC helper; their operation-specific authorization remains separate.
- **Medium, verified:** The whole Cobra CLI and gateway setup were Linux tagged, so adding macOS or Windows adapters would require another CLI rewrite. Command registration and HTTP gateway setup are now portable; platform files provide only native identity, signals, observation adapters, and worker entry points. Cross builds cover the complete CLI on macOS arm64 and Windows amd64.
- **Idiomatic review:** No additional findings.
- **Validation:** Linux build, macOS and Windows cross builds, race tests, formatting, lint, strict OpenSpec validation, archive verification, and disposable systemd acceptance were rerun. Native runtime checks for macOS and Windows are outside this release.

## Round 3: Performance

- **Finding:** None. The reviewer checked the gateway admission bound, fresh token verification, request-scoped collection, Docker response cap, fixed inventory limit, and benchmark comparison. No material regression or unbounded path was verified.
- **Evidence:** The same bounded `tools/list` shape measured 214–231 µs and 605 allocations after the fixes, versus 227–248 µs and 798–800 allocations in the pre-rewrite local baseline. The latency ranges overlap; the authentication setup also differs. Final published-0.5.0 and rewrite measurements appear in `docs/v2/VALIDATION.md`.

## Round 4: Contract and product intent

- **High, verified:** Migration accepted an invalid legacy mode when a token-store path was explicit and silently ignored unknown legacy settings. Legacy decoding now rejects unknown fields and extra documents, validates the supported system mode and privilege, and refuses a legacy remediation flag before candidate publication. Rejection cases assert that no candidate directory is created.
- **Medium, verified:** A restart could complete and then fail its status read, leaving the client with only an error and no indication that mutation occurred. Repair results now expose invocation, completion, an optional post-action state, and a stable issue code. A worker test covers the missing post-action observation.
- **Contract clarification:** Active profiles are installation-wide grants for each role, not per-token scopes. The profile specification and operator guide state that explicitly.
- **Validation:** Disposable container race tests, cross builds, schema snapshot comparison, and strict source formatting passed after the repair and migration changes. Additional final gates remain scheduled after all review rounds.

## Round 5: Security and adversarial closure

- **High, verified:** A Docker or systemd restart response can be lost after the mutation request reaches its target. Results now omit nullable `invoked` and `completed` fields when their values are unknown, with a stable `restart_outcome_unknown` issue. A disposable fake Docker daemon test receives the POST and drops the response.
- **Medium, verified:** Configuration and migration sources were checked by path and then reopened, allowing substitution under a writable ancestor. Shared protected-file opening now checks every parent, refuses symlinks and writable untrusted directories, and validates the opened descriptor. Tests reject symlinks and writable parents.
- **Medium, verified:** Migration reported direct legacy profile names but omitted included profiles. Bounded traversal now validates the active include graph and reports all reached names; a nested profile fixture covers the report. Explicitly configured token stores must exist.
- **Validation:** Focused race tests and the full disposable container suite passed, including native Linux and portable cross builds, schema comparison, and source formatting. Packaging gates remain scheduled after the remaining review rounds.

## Round 6: Delivery and operator documentation

- **CI and delivery review:** No verified implementation finding. The reviewer inspected Make/Just parity, archive hooks, the private Docker target, and the native arm64 CI job; `actionlint` and staged diff whitespace checks passed. GitHub pipeline status was unavailable before publication of this staged rewrite.
- **Medium, verified:** Installation instructions tried to check a multi-asset checksum file after downloading only one archive. They now require every listed asset before strict checksum verification.
- **Medium, verified:** The local release checklist omitted `make systemd-image` before `make test-systemd`; the required preparation step is now explicit.
- **Medium, verified:** The operator guide said restart tools disappear when the repair worker stops, while discovery checks configuration. It now states that a stopped worker causes calls to fail.
- **Validation:** Documentation was formatted and checked against the implementation and task runner. Final end-to-end gates remain scheduled.

## Round 7: Authorization and specification closure

- **High, verified by both reviewers:** `container_status` authorized a supplied short Docker ID but the Docker API resolved and returned the full ID. A denial on that full ID could be bypassed with a prefix. The gateway, observer, and fixed Docker transport now require the full 64-character ID; a role/profile test exercises conflicting wildcard grant and full-ID denial through MCP.
- **Spec review:** All twelve added requirement sections were inspected; no other verified divergence was found.
- **Validation:** Focused disposable race tests for MCP authorization, Docker, and the observer passed. The generated contract snapshot and operator table now specify a full ID.

## Round 8: Final adversarial, maintainability, and intent review

- **Adversarial and maintainability:** No additional verified code finding after tracing authorization, process boundaries, error paths, migration, packaging, and the staged diff.
- **Medium, verified:** The design promised a 0.5.0-to-version-2 MCP tool map, but the upgrade guide only linked to the new catalog. The guide now maps available replacements, names every removed old tool, and states that payload compatibility is not preserved.
- **Validation:** The final disposable race/vet/build/contract suite, private nested Docker acceptance, both candidate archives and checksum verification, disposable systemd install/repair/read-only/rollback, vulnerability scan, formatting, lint, and strict specification checks passed. Native arm64 runtime and GitHub CI remain pending until the branch pipeline runs.
