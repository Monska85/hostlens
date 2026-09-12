## 1. Configuration contract

- [x] 1.1 Add the default-on `mcp.read_only` field to configuration defaults and strict YAML decoding, and verify unit tests cover omission, explicit true, explicit false, and unknown MCP fields.
- [x] 1.2 Include the active read-only value in effective-policy fingerprints and local administrative status, and verify equal and differing configurations produce the expected fingerprints and status output without secrets.
- [x] 1.3 Make `mcp.read_only` reloadable through coordinated generations, and verify reload tests cover both transitions, failed candidate preservation, concurrent diagnostic reads, and the bounded no-active-remediation condition with a mutation-sentinel test double.

## 2. MCP effect gate

- [x] 2.1 Define a closed diagnostic or remediation effect type and one exhaustive registry for every MCP tool, and verify construction rejects unknown effects, missing effects, duplicate names, and handlers outside the registry.
- [x] 2.2 Derive MCP registration and role, policy, and capability discovery from the registry, and verify every current role sees the same authorized diagnostic surface with omitted, true, and false read-only settings.
- [x] 2.3 Enforce the same registry and read-only decision on direct calls before backend access, and verify remembered remediation names, unclassified names, unauthorized names, and unavailable capabilities cannot invoke a backend or external integration.
- [x] 2.4 Keep backend IPC restricted to named diagnostic operations in v1, and verify unknown or mutation-class operations fail before collector execution.

## 3. Stateless evidence and authority boundaries

- [x] 3.1 Audit gateway, backend, collectors, pagination, sampling, cancellation, and failure paths for retained diagnostic evidence, remove service-owned references when collection workers exit, and verify repeated mutable-fixture requests always observe live sources.
- [x] 3.2 Centralize payload-free audit fields and aggregate metric labels, and verify unique source, argument, result, partial-failure, and cancellation markers do not appear in HostLens-managed files, logs, metrics, or retained component state.
- [x] 3.3 Encode the diagnostic IPC and integration boundary so the gateway and diagnostic backend have no generic mutation contract or credential, and verify a mutation-sentinel observer rejects non-read operations without changing fixture state.
- [x] 3.4 Add a reusable contract test that future diagnostic integrations, including mixed read-write infrastructure APIs, must satisfy to prove named observation-only operations and per-request live collection.

## 4. Administrator and operator behavior

- [x] 4.1 Verify local installation, upgrade, uninstall, token administration, configuration validation, reload, status, and telemetry remain available with MCP read-only mode active and remain absent from MCP discovery and dispatch.
- [x] 4.2 Update example configuration and operator documentation with the default, MCP-only scope, false-as-eligibility semantics, stateless evidence boundary, allowed control-plane state, and older-binary rollback instruction, then verify every documented command and configuration snippet against the implementation.
- [x] 4.3 Add concise stateless diagnostics, MCP effect gate, separate mutation authority, and constrained infrastructure observer invariants to `AGENTS.md`; verify `CLAUDE.md` remains a relative symlink to it and document formatting passes.

## 5. Acceptance validation

- [x] 5.1 Run focused configuration, gateway, backend, audit, metrics, reload, and CLI tests in the established disposable test container and verify success, failure, timeout, cancellation, and direct-call bypass cases pass.
- [x] 5.2 Run equivalent Make and Just formatting, lint, test, race, vulnerability, coverage, and combined checks through the repository's established container workflow, and verify both entry points report the same required results.
- [x] 5.3 Run the shared Debian, Ubuntu, and Arch acceptance matrix for native supported architectures available in CI, record container kernel and namespace limits, and verify no untested boundary is reported as passed.
- [x] 5.4 Run strict current and archived OpenSpec validation and verify the active change, repository specs, archived changes, and documentation are valid before marking implementation tasks complete.
