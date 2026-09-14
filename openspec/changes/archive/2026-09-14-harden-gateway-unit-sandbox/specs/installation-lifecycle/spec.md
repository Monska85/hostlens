## ADDED Requirements

### Requirement: Generated unit sandbox baseline

Generated systemd service units SHALL deny syscall surface beyond the repository baseline: every service SHALL declare `SystemCallArchitectures=native`, a `@system-service openat2` allow-list with a `@mount @reboot @swap @raw-io` deny-list, `LockPersonality=yes`, `RestrictRealtime=yes`, and `RestrictNamespaces=yes`. The gateway SHALL keep `AF_INET`, `AF_INET6`, and `AF_UNIX` address families; the diagnostics backend and observer SHALL keep `AF_UNIX` only. Every service SHALL declare `ProtectClock=yes`, `ProtectHostname=yes`, `ProtectKernelLogs=yes`, and `PrivateIPC=yes`. The gateway and observer SHALL additionally declare `ProtectProc=invisible` and `ProcSubset=pid`; the diagnostics backend SHALL NOT declare them because its audit evidence requires system-wide `/proc` files and other users' process entries.

#### Scenario: Gateway starts under the filter set

- **WHEN** the gateway service starts with the sandbox baseline applied
- **THEN** TLS and MCP traffic, token verification, and reloads work with no sandbox denials in service logs

#### Scenario: Diagnostics keep system-wide proc evidence

- **WHEN** the diagnostics backend audits network, storage, or process evidence while its unit omits `ProtectProc` and `ProcSubset`
- **THEN** system-wide `/proc` files and other users' process entries remain observable instead of being silently hidden

#### Scenario: Acceptance asserts the baseline

- **WHEN** the systemd acceptance suite inspects the installed units
- **THEN** every declared sandbox directive is present on each unit it is specified for