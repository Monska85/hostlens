# Proposal: harden-gateway-unit-sandbox

## Why

The gateway is the network-facing, most-exposed HostLens process but its systemd unit (`internal/lifecycle/units.go` `GatewayUnit`) lacks the sandbox baseline every other unit already has: no `SystemCallFilter`, `SystemCallArchitectures`, `LockPersonality`, `RestrictRealtime`, or `RestrictNamespaces`, and no protected clock/hostname/kernel-logs or private IPC. A compromised gateway can currently call the full default syscall surface with realtime scheduling and namespace creation available.

## What Changes

- `GatewayUnit` gains the diagnostics/observer syscall baseline (`SystemCallFilter=@system-service openat2`, `SystemCallFilter=~@mount @reboot @swap @raw-io`, `SystemCallArchitectures=native`, `LockPersonality`, `RestrictRealtime`, `RestrictNamespaces`) while keeping TCP/TLS address families.
- All three service units gain `ProtectClock=yes`, `ProtectHostname=yes`, `ProtectKernelLogs=yes`, `PrivateIPC=yes`.
- Gateway and observer additionally gain `ProtectProc=invisible` and `ProcSubset=pid`. The diagnostic backend is excluded: its audit evidence reads system-wide `/proc` files (`/proc/net/*`, `/proc/partitions`, `/proc/self/mountinfo`) and other users' process entries, and hiding them would silently degrade collection.
- `MemoryDenyWriteExecute` is evaluated empirically in the acceptance container and only kept if the services start and pass acceptance; otherwise the exclusion is recorded with its reason.
- `scripts/systemd-acceptance.sh` asserts the directives on the installed units.
- `docs/v1/OPERATIONS.md` hardening paragraph updated.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `installation-lifecycle` — add a generated-unit sandbox baseline requirement naming the directives per unit, with the diagnostics `/proc` exclusion scenario.
- `platform-runtime` — the socket-activated observer requirement now names the observer sandbox directives. Applied together with the `refuse-world-accessible-docker-socket` change, whose delta carried the merged observer requirement text (sandbox baseline plus world-access validation); this change therefore ships no separate `platform-runtime` delta.

## Impact

- `internal/lifecycle/units.go`, `scripts/systemd-acceptance.sh`, `docs/v1/OPERATIONS.md`.
- Existing installations adopt the new units on upgrade/reinstall; unit files change, service behavior is otherwise identical.