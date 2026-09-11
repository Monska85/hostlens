## Context

The current eight tools expose basic identity, health, services, packages and approved source content. Policy categories are files and journal. Diagnostics runs without extra privileges or with CAP_DAC_READ_SEARCH; its sandbox restricts sockets and devices. See proposal.md for the expansion goal.

## Goals / Non-Goals

Provide general evidence that lets an agent investigate an unfamiliar service. Preserve native semantics, least privilege and bounded on-demand work. Application-specific conclusions remain the agent's responsibility. No SQL, generic execution, privilege escalation, application plugins, automatic updates or restore exercises.

## Decisions

- Add explicit `audit` rules containing known domain names: `processes`, `network`, `accounts`, `storage`, `updates`, `security`, `services`, `hostlens`, `paths`. Exact names and `*` are accepted; unknown names fail validation. Existing broad file or journal grants do not imply audit grants. Diagnostics role is required for all new audit tools. Existing tools keep their role meanings.
- Expose `list_processes`, `get_process_info`, `get_network_info`, `list_accounts`, `get_storage_info`, `get_update_info`, `get_security_info`, `get_hostlens_info`, `inspect_service`, and `inspect_path`. Lists use existing offset/limit ceilings. PID, unit and path selectors have strict schemas; no command or SQL selector exists.
- Use procfs/sysfs and standard native read interfaces. Every file-backed source continues to honor explicit file denials. Fixed command collectors check their source boundaries, use fixed arguments and controlled environments, and never interpolate shell text. Prefer native sources over adding runtime dependencies.
- Process observations exclude environment and command arguments, which frequently contain secrets. Return numeric identities, resource counters and executable identity when permitted. PID reuse, namespace scope and races must remain visible limitations.
- Network evidence covers interfaces, addresses, routes and TCP listeners/UDP endpoints with native scope. No active scanning. Firewall evidence is unavailable when the existing capability or socket sandbox cannot read it; configured files do not substitute for active rules.
- Accounts expose local account/group metadata without password hashes. Account state, sudo and effective SSH coverage is partial when permissions or safe evaluation prevent collection. Configuration text alone does not prove Match-dependent SSH behavior or directory-service identity.
- Storage exposes block-device and mount observations. Device health and LVM/RAID details unavailable under the selected sandbox remain explicit gaps. Maintenance queries use only local metadata, never refresh repositories or invoke upgrades; results include freshness/scope limitations.
- Service details use an allowlist of systemd runtime, dependency, identity and hardening properties. Do not return environment, command arguments or unrestricted unit properties. Selected file metadata uses verified handles and the same explicit file policy, including mandatory exclusions; directory metadata does not imply recursive listing or content access.
- HostLens self-inspection returns selected non-secret effective configuration and backend runtime facts. It does not read the token store or expose key contents, policy source contents or token hashes.
- Every new tool returns explicit coverage and structured issues for policy denial, insufficient privileges, missing interfaces, malformed sources and limits. Lack of a collector must never become an empty successful security finding. A capability is not described as a complete production audit.

## Risks / Trade-offs

- Incomplete OS authority: preserve restrictions and report actionable gaps, rather than add capabilities or SSH fallback.
- Filesystem and process races: use existing safe handles, bounded reads and identity checks; identify snapshots as non-atomic.
- Sensitive evidence: select fields, require domain plus source authorization, and never claim arbitrary approved application content is automatically secret-free.
- Native command variability: test supported output forms, retain explicit unsupported/parse failures, and avoid distribution-version gates.

## Migration Plan

Existing configurations remain valid and do not activate new audit domains. Administrators explicitly grant selected domains and application file/journal sources. Validate in disposable containers with isolated application fixtures. Audit each fixture through generic MCP calls, then remove temporary grants and fixtures. Commit only after validation and panel triage; verify hosted CI afterward.
