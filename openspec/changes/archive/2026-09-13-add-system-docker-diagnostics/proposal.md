## Why

Docker is a core host infrastructure surface on many homeservers, but HostLens cannot currently report its runtime health, resource use, or unused storage. The integration must provide useful live evidence without granting the MCP gateway or diagnostic backend Docker's root-equivalent control authority.

## What Changes

- Add first-class, policy-controlled diagnostics for the local system-wide Docker Engine on Linux.
- Expose bounded tools for daemon information, containers, container details and resource snapshots, images, volumes, networks, disk usage, and container logs.
- Report current reference state and reclaimable space for unused resources. Report creation and last-known runtime timestamps only when Docker supplies them, and never mislabel creation age as time since last use.
- Use an opt-in, systemd-socket-activated observer with a fixed GET-only observation contract. The gateway and diagnostic backend never receive the Docker socket, Docker group membership, or a generic Docker API pass-through.
- Fetch every result from the live daemon per request and retain no Docker inventory, log payload, usage history, cleanup candidate, or remediation plan.
- Keep cleanup and every other Docker mutation out of scope. Future remediation must use the separate authority and live revalidation contract established by `enforce-stateless-mcp-read-only`.
- Support the rootful system-wide daemon first. Defer rootless Docker, remote daemons, Swarm administration, Kubernetes, Podman, and other container runtimes.
- Keep the typed observation contract, lifecycle states, and MCP schemas independent of Linux transport and service-manager details. Future Windows and macOS integrations must provide native identity, IPC, credential, supervisor, and runtime adapters and pass native isolation validation before support is claimed.
- Reconcile, install, upgrade, disable, and uninstall the observer without starting or restarting Docker, changing Docker daemon configuration, or deleting pre-existing Docker resources.

This change SHALL be implemented after `enforce-stateless-mcp-read-only` is implemented and archived, so the shared MCP effect gate and stateless evidence boundary exist before Docker tools are registered.

## Capabilities

### New Capabilities

- `docker-diagnostics`: Define the Docker tool surface, live evidence model, honest unused-resource analysis, bounded results, and read-only behavior.

### Modified Capabilities

- `access-policy`: Add explicit Docker resource grants and denials with deny precedence and safe resource identity matching.
- `configuration-lifecycle`: Define validated Docker observer configuration, restart-only topology changes, and dry-run-first reconciliation.
- `installation-lifecycle`: Add safe observer lifecycle management that preserves all Docker configuration, data, and resources.
- `mcp-diagnostics`: Add the Docker diagnostic tools to role-filtered, capability-aware MCP discovery and dispatch.
- `platform-runtime`: Add the isolated system-wide Docker observer, define portable observer boundaries and lifecycle states, and declare supported and deferred runtime modes.
- `release-validation`: Require read-only, stateless, compatibility, lifecycle, and failure-path acceptance on representative Docker environments.

## Impact

The change adds one optional local service and IPC protocol, Docker-specific policy resources, MCP schemas, collectors, installer-managed unit state, fixtures, and acceptance coverage. It may add a Docker Engine client dependency only after its identity, maintenance, API compatibility, and security guidance are verified. Existing installations remain diagnostic-only and do not gain Docker access until a local administrator explicitly enables and grants it.
