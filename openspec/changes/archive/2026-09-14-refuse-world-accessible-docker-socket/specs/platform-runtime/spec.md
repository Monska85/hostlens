## MODIFIED Requirements

### Requirement: Linux socket-activated observer

The Linux system installation SHALL expose the observer through a HostLens-owned systemd socket unit and SHALL start the observer service only when the diagnostic backend requests Docker evidence. The service SHALL use `SupplementaryGroups=` for the validated Docker socket group and SHALL declare `Requisite=docker.service`, `After=docker.service`, and `PartOf=docker.service`. It SHALL NOT use `ConditionPathIsSocket=`. The observer SHALL validate the configured Docker socket type, locality, root ownership, access group, group access, and absence of world access before connecting. The observer unit SHALL declare the repository sandbox baseline: `SystemCallArchitectures=native`, the `@system-service openat2` allow-list with its `@mount @reboot @swap @raw-io` deny-list, `LockPersonality=yes`, `RestrictRealtime=yes`, `RestrictNamespaces=yes`, `ProtectClock=yes`, `ProtectHostname=yes`, `ProtectKernelLogs=yes`, `PrivateIPC=yes`, `ProtectProc=invisible`, and `ProcSubset=pid`.

#### Scenario: Docker request while active

- **WHEN** the observer is dormant, Docker is active, and the diagnostic backend connects to the HostLens observer socket
- **THEN** systemd starts the observer with process-scoped Docker group authority and the observer validates the Docker socket before issuing a GET observation

#### Scenario: World-accessible socket refused

- **WHEN** the configured Docker socket grants read or write access to other users (for example mode 0666 or 0644)
- **THEN** the observer refuses the connection with a precondition error before any daemon query

#### Scenario: Group-restricted socket accepted

- **WHEN** the configured Docker socket is a root-owned Unix socket in the configured access group with owner and group access only (mode 0660)
- **THEN** observation proceeds

#### Scenario: Docker request while stopped

- **WHEN** Docker is stopped and a diagnostic request reaches the observer socket
- **THEN** the observer does not become available, HostLens reports Docker unavailable, and no HostLens unit starts Docker

#### Scenario: Docker stops after activation

- **WHEN** Docker stops or restarts while the observer is active
- **THEN** systemd stops the observer through the declared dependency and other HostLens processes remain operational

#### Scenario: Observer sandbox baseline

- **WHEN** the observer activates to serve a Docker evidence request
- **THEN** observation completes under the declared sandbox directives without denials