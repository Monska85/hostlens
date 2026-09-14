# Design: harden-gateway-unit-sandbox

## Context

`GatewayUnit` (`internal/lifecycle/units.go:11-37`) carries the filesystem/privilege baseline but lacks the syscall and lock directives that `DiagnosticsUnit` and `ObserverUnit` already declare. The gateway serves TCP/TLS, so the `RestrictAddressFamilies` set must differ from the backend/observer `AF_UNIX`-only form. See proposal.md — Why.

## Goals / Non-Goals

- Goals: identical syscall-ceiling semantics across all three units; additional protected-clock/hostname/kernel-logs and private-IPC isolation; proc visibility only where evidence collection does not depend on it.
- Non-Goals: no changes to identity provisioning, capabilities, or the socket-activation dependency model; no new systemd features beyond unit directives.

## Decisions

- **Mirror the diagnostics filter set on the gateway**: `@system-service` includes `@network-io`, so TCP/TLS serving needs no exceptions; `openat2` stays allow-listed for uniformity with the other units.
- **Exclude the diagnostics backend from `ProtectProc`/`ProcSubset`**: `ProcSubset=pid` hides `/proc/net/*`, `/proc/partitions`, `/proc/self/mountinfo`, and `ProtectProc=invisible` hides other users' processes — both are primary audit evidence sources (`audit_network_linux.go`, `audit_storage_linux.go`, process audit). Hiding evidence would convert denials into silent partial collection, which the spec forbids. The gateway and observer read no `/proc` files (peer credentials come from `SO_PEERCRED`, a syscall), so they take the stricter form.
- **`MemoryDenyWriteExecute` decided empirically**: Go with CGO disabled is expected to work, but the decision is made in the disposable acceptance container; if any service fails, the directive is dropped and the reason recorded in the ledger and design notes.
- **Assertions by unit-file inspection** in `systemd-acceptance.sh` (directive grep) rather than `systemd-analyze security` scoring: exact, deterministic, and independent of systemd version scoring drift.

## Risks / Trade-offs

- [`@system-service` misses a syscall the gateway legitimately uses] → Acceptance exercises TLS, MCP calls, reloads, and metrics under the filter set; a missing syscall fails visibly in the container, not in production.
- [Upgraded installations replace unit files] → The installer already regenerates units on install/upgrade with a plan and ownership records; admins see the diff in the plan.
- [Container kernel may lack some namespace protections] → Acceptance reports container kernel and namespace limits honestly; directive presence is asserted from unit files, service behavior from real starts.

## Migration Plan

Units regenerate on next install/upgrade/reconcile; no data migration. Rollback is a plain revert.

## Open Questions

None — the `MemoryDenyWriteExecute` decision is deliberately delegated to the acceptance run.