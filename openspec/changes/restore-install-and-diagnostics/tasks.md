# Tasks

## 1. Lifecycle

- [x] 1.1 Implement verified archive inspection and read-only install/upgrade plans; verify digest, conflict, identity, and changed-plan cases in a disposable container.
- [x] 1.2 Implement protected fresh install and opt-in startup with rollback; verify staged failures and read-only gateway activation in a disposable systemd container.
- [x] 1.3 Implement installed upgrade with state preservation and rollback; verify successful and failed activation in a disposable systemd container.
- [x] 1.4 Update installation and upgrade guides with actual commands and verify archive packaging and guide snippets in disposable containers.

## 2. Diagnostic evidence

- [x] 2.1 Extend typed host health and native service detail and add filtered bounded service discovery; verify partial failures and denied-resource hiding in disposable-container tests.
- [x] 2.2 Add separately granted, bounded service journal observations using the native journal reader; verify unit isolation, unavailable access, and byte/time ceilings in disposable-container tests.
- [x] 2.3 Extend Docker container health and add separately granted stats and log observations through the fixed observer; verify grants, stable IDs, output bounds, and private nested-engine behavior.
- [x] 2.4 Regenerate the typed MCP snapshot and update operator guidance; verify snapshot drift and examples with the repository checks.

## 3. Authority and administration

- [x] 3.1 Add payload-free repair audit events for denied, failed, unknown, and completed operations; verify that tests find no secret or diagnostic payload in events.
- [x] 3.2 Require explicit token expiry for create and rotate; verify omission leaves the token store untouched and explicit finite/never values work.
- [x] 3.3 Add local policy explanation using compiled grant and denial rules; verify effective decisions and no diagnostic collection.
- [x] 3.4 Update security and operations guides for audit, expiry, and policy explanation; verify the documented CLI commands.

## 4. Release integration

- [x] 4.1 Run format, lint, race, vet, portable builds, package verification, private Docker, and disposable systemd acceptance; fix failures and record unavailable gates truthfully.
- [x] 4.2 Validate and sync OpenSpec deltas, update release notes and version examples, and verify documentation links and the final code-line report.
- [x] 4.3 After all implementation tasks finish, run six full-scope review/fix rounds, triage and resolve findings, then rerun affected gates; verify zero unresolved findings.
- [ ] 4.4 If the final review has zero unresolved findings, amend the one release commit, verify main CI, and replace the 0.6.0 tag and release under the authorized one-time protection change; verify published checksums and restore protections.
