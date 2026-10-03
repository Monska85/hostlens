# Design

## Context

See [proposal](proposal.md). The 0.6.0 gateway already has one typed tool registry, a separate Docker observer, and a separate repair worker. Installation currently relies on root shell instructions; the binary only previews or applies removal. The release archive has a per-member checksum manifest and fixed systemd unit names.

## Goals / Non-Goals

**Goals:** Keep the typed MCP and effect gate, use existing maintained Go SDKs and native system tools, make lifecycle changes previewable and recoverable, and give clients evidence useful before a targeted restart.

**Non-Goals:** Restore the 0.5.0 implementation, add a generic shell or Docker API proxy, retain diagnostic history, or implement macOS and Windows adapters now.

## Decisions

1. **Linux lifecycle around release assets.** Add a Linux lifecycle package beside the bounded uninstaller. Read `release.json` and verify required members before a plan. Use the standard Go filesystem APIs for trusted file creation, `os/user` for lookup, and fixed `groupadd`, `useradd`, and `systemctl` operations for identities and activation. Preview never runs these commands. Reject unknown existing targets and preserve configuration, tokens, and profiles on upgrade. Store protected checksums of installed unit files so a future release may change them without treating pristine old files as administrator modifications. On legacy 0.6.0 installations without this marker, require unit bytes to match the candidate and establish provenance on successful upgrade. Record rollback bytes and unit activation state before replacing packaged files. This reuses the operating system's account and service managers without copying the old installer. The alternative of shell scripts in documentation preserves the current error-prone manual path.
2. **Typed evidence from maintained libraries.** Extend the existing gopsutil host adapter, go-systemd service adapter, and Moby Docker client. The journal is read through the installed `journalctl` binary with fixed arguments, one exact unit, bounded `--since`, and a bounded output reader; this avoids a cgo dependency on libsystemd that would complicate both architecture builds. Docker stats and logs use named Moby APIs through the observer's fixed transport allow-list and bounded decoders. Do not accept caller-supplied native paths or API methods. The alternative of parsing `/proc`, journald files, or Docker HTTP bodies directly would rebuild removed machinery.
3. **One authorization decision per new tool.** Add each observation to the existing typed registry and profile effect gate. Status, stats, and logs have distinct grants. Filter service and container inventories before serialization; a denied item's count or truncation signal cannot leak through aggregate fields. Preserve request-scoped collection and explicit issues.
4. **Repair audit at the mutation boundary.** Emit a structured, payload-free event from the repair worker after peer, token, role, and profile checks and after native invocation. Include an opaque request ID, token ID when verification succeeds, tool, target, decision, and outcome. Never log the bearer secret or observed payload. Logging failure remains visible on stderr and is not represented as a successful audit.
5. **Explicit credential lifetime and policy explanation.** Cobra requires `--expires` on create and rotate; `never` remains supported only when written explicitly. A local `policy explain` command uses the compiled profile policy and reports matching grants and denials without making an MCP call. It has no mutation privilege.
6. **Portable boundaries.** Shared tool payloads and effect rules stay OS-neutral. Linux-only lifecycle, journal, and systemd implementation lives behind native build-tagged packages. Unsupported operating systems keep explicit unavailable adapters. Future native installers implement the same preview/apply contract without inheriting Linux paths or service semantics.

## Risks / Trade-offs

- **Privileged installation mistakes** → Reject symlinks, untrusted owners and modes, digest mismatches, changed plans, and unexpected existing resources before mutation; keep rollback copies and test failure stages in disposable systemd containers.
- **Sensitive diagnostic output** → Separate grants for logs and stats; cap time, rows, and bytes; omit command, environment, labels, and health-check output; treat returned text as data.
- **External tool availability** → If `journalctl` or systemd is unavailable, return an explicit gap rather than a fabricated empty result.
- **Release tag replacement** → Only after full implementation, six review/fix rounds, and a zero-open-finding final result, amend the single commit and replace 0.6.0 under the user's one-time authorization. Verify both public archives and warn consumers that checksums changed.

## Migration Plan

Keep the current 0.5.0 migration candidate command separate. For an installed 0.6.0 host, preview upgrade using the extracted new archive, preserve existing protected data, apply the new binary and units, validate, then reactivate only services that were active. Restore the previous binary and units if validation or activation fails. Fresh installations start only the read-only gateway when explicitly requested. All acceptance changes occur inside disposable containers, including native arm64 verification in CI.
