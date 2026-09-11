## 1. Authority and contracts

- [x] 1.1 Add audit-domain policy, diagnostics-role enforcement and strict MCP selectors; verify grants, denials, reload fingerprints and malformed calls in container tests.
- [x] 1.2 Implement shared audit dispatch and coverage/error semantics; verify unavailable, denied and partial observations remain distinct.

## 2. Generic collectors

- [x] 2.1 Implement process lists/details and network evidence with bounded native parsing; test malformed sources, process races, namespace limits and omitted secrets.
- [x] 2.2 Implement local accounts, storage, maintenance and security-control evidence; test denied sources, missing interfaces, stale metadata and malformed native data.
- [x] 2.3 Implement selected service details, policy-bound path metadata and sanitized HostLens self-inspection; test protected aliases, unrelated selectors and secret exclusion.

## 3. Acceptance and documentation

- [x] 3.1 Document capability grants, evidence scope and generic investigation workflow; validate specifications and formatting.
- [x] 3.2 Run race, static, build, archive and container acceptance checks using the established commands; fix failures.
- [x] 3.3 Validate PostgreSQL, Apache, nginx and MySQL investigations through MCP in disposable containers; assert positive and negative policy controls and remove temporary fixtures.

## 4. Review and delivery

- [x] 4.1 Run the selected sf-peer-review panel, triage every finding and verify fixes, with at most five review/fix rounds; retain evidence and valid no-finding outcomes.
- [x] 4.2 Synchronize completed specifications and validation records and prepare the verified change for publication.

Publication is the final delivery gate after this implementation checklist: commit and push to main, then confirm hosted CI; allow at most three CI fix rounds. Hosted success is not asserted by completing implementation tasks.
