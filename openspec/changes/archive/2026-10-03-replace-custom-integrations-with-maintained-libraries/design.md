# Design

## Context

See [proposal.md](proposal.md). The Docker observer is a separate process that exposes only named observations. Its upstream access currently uses fixed GET requests, validates the Unix socket before each connection, and limits response bytes. Linux collectors read policy-approved files with per-request budgets. The published MCP and metrics payloads are stable contracts.

## Goals / Non-Goals

**Goals:** Remove maintained protocol and parser code where upstream packages can preserve the same observable behavior; use compatible latest stable versions; retain explicit failure and truncation signals.

**Non-Goals:** Broaden Docker authority, add new platform collectors, change metrics or MCP member names, or substitute a general-purpose token store for the current durable credential rules.

## Decisions

1. Use Moby's maintained API types inside the Docker observer for version, engine, inventory, and mount fields. Keep the fixed GET transport because it enforces socket ownership, an endpoint allowlist, and response byte ceilings before decoding. The full Moby client exposes mutation methods and eagerly decodes complete responses; introducing it here would add authority and allocation surface without removing those required guards. Keep narrow local types where the API type loses optional-field presence or does not represent the negotiated legacy response.
2. Retain the bounded log-frame decoder. Moby's `stdcopy.StdCopy` allocates from the daemon-controlled frame length before reading the payload and rejects unknown stream IDs. HostLens instead caps frame sizes and counts skipped frames; replacing it would weaken those published behaviors.
3. Keep policy-owned `/proc` parsing. Prometheus procfs exposes file-opening methods, but its relevant parse functions are unexported. Calling it after a policy check would reopen the source outside HostLens's bounded descriptor, creating a second read and a race.
4. Use Go's `os.Root` for archive extraction, manifest reads, and checksum verification, while retaining explicit member validation. Keep fixed systemd unit templates and bounded systemctl/journalctl commands: go-systemd's journal binding requires cgo, and its D-Bus API exposes write operations across the diagnostic boundary.
5. Keep existing Prometheus runtime metric names and token persistence. Their proposed library alternatives would change public metrics or weaken exclusive creation and durability. Version and dependency updates are independent of adopting those alternatives.

## Risks / Trade-offs

- Moby's larger types can decode fields HostLens does not publish → keep response byte ceilings and release request-scoped objects after projection; compare fixture output and benchmark relevant inventory paths.
- Upstream typed fields may differ from current narrow daemon types → compare all published fields and missing-value behavior with observer fixtures and acceptance cases.
- Dependency upgrades may alter protocol or serialization behavior → run race, contract, portable-build, systemd, and private Docker acceptance in disposable containers.

## Migration Plan

Update module and toolchain pins, refactor one integration at a time, regenerate checksums, and build the same release archives. No persistent data migration is needed. A failed release candidate can be rolled back through the existing archive upgrade mechanism.
