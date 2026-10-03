# Proposal

## Why

The current public HostLens implementation carries a large custom protocol, collection, policy, and delivery surface. A smaller product can expose useful host and Docker status and narrowly authorized repairs through maintained Go libraries while preserving clear failure and security boundaries.

## What Changes

- **BREAKING:** Replace the existing MCP tool catalog with a small typed catalog for host status, services, Docker status, and containers. Remove generic audit, configuration reads, logs, package enumeration, bespoke metrics, and unused compatibility machinery.
- **BREAKING:** Replace configuration and profiles with explicit tool and resource grants. An active denial overrides every grant, and write grants are separate from diagnostic grants.
- Add an opt-in remediation process with its own identity and fixed contract. The first allowed changes are restarting an explicitly granted systemd unit or Docker container after live-state revalidation. No generic command or API-forwarding tool exists.
- Rewrite the implementation around the official MCP SDK, maintained system and Docker libraries, and small platform interfaces. Linux amd64 and arm64 work now; platform-specific collection and repair implementations can later be added for macOS and Windows.
- Replace obsolete code, tests, scripts, and documentation. Provide a checked, separate migration candidate for existing 0.5.0 configuration and credentials, with operator-controlled activation and rollback.
- Use the latest stable compatible releases of selected libraries and tools, verified from official sources at implementation time.

## Capabilities

### New Capabilities

- `host-status`: Bounded live system and service observations with explicit gaps and platform boundaries.
- `docker-status`: Bounded local Docker observations through a fixed resource surface.
- `profile-authorization`: Per-tool, per-resource read and write authorization through activated profiles and independently managed tokens.
- `controlled-remediation`: Opt-in, separately authorized and live-revalidated service and container restarts.
- `delivery-migration`: Disposable-container validation, release artifacts, and explicit upgrade migration from the published product.

### Modified Capabilities

The existing v1 capability specifications are superseded by the new capabilities above. Their former requirements and scenarios will be removed from the active specification set when the replacement implementation and migration are verified. Archived changes retain historical context.

## Impact

The Go implementation, MCP contract, profiles, configuration, credentials, systemd units, release archives, CI, operator guidance, and OpenSpec active specifications change. Existing 0.5.0 clients require a tool-contract migration. Installed hosts require an explicit checked migration and manual service switch rather than silent adoption.
