# Proposal

## Why

The 0.6.0 rewrite simplified HostLens but removed installation automation and much of the evidence needed to diagnose a host before using its new repair tools. Restore those capabilities on the current typed, separated architecture so a fresh installation and a repair decision are safe and practical.

## What Changes

- Add a previewable Linux install and upgrade command for the packaged binary, identities, protected configuration, and systemd units. Keep administrator review, rollback, and the existing bounded uninstall contract.
- Add bounded, policy-controlled service discovery, service and container evidence, host health, and Docker health observations. Keep raw commands and unrestricted API access unavailable.
- Add a local policy explanation command and payload-free audit events for repair decisions and outcomes.
- Require an explicit token expiry choice, including an explicit `never` option.
- Keep Linux amd64 and arm64 functional and the shared MCP, authorization, and lifecycle contracts ready for later macOS and Windows adapters.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `installation-lifecycle`: previewable fresh installation and safe in-place upgrade.
- `host-status`: richer health, service discovery, and bounded native service evidence.
- `docker-status`: bounded container health, stats, and logs.
- `profile-authorization`: explicit token lifetime and local policy explanation.
- `controlled-remediation`: payload-free audit events for repair decisions and outcomes.
- `delivery-migration`: release and acceptance coverage for the restored lifecycle and diagnostic contract.

## Impact

The CLI, typed MCP catalog, platform adapters, observer and repair IPC, Linux lifecycle code, profiles, release archives, documentation, and container acceptance tests change. Existing 0.6.0 clients can continue to use the seven current tools; new optional fields and tools extend the catalog. A published replacement of the 0.6.0 tag is authorized only after the full repair and six review/fix rounds end with no unresolved findings.
