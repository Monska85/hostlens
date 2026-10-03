# installation-lifecycle Specification

## Purpose

Allow administrators to remove a system installation safely and predictably while preserving data and identities that HostLens cannot prove are unused.

## Requirements

### Requirement: Bounded system uninstall

On Linux, `uninstall` SHALL preview the exact known systemd units and binary it would remove. `uninstall --apply` SHALL require root, verify every existing target and its containing directory is root-owned and not writable by group or others, verify unit bytes or executable identity, then disable and stop verified units. It SHALL remove unit files, reload systemd, then remove the executable. If removal or reload fails, it SHALL restore removed unit files for a verifiable retry. It SHALL emit named progress stages on stderr and a machine-readable result on stdout. Missing targets SHALL be harmless on retry. An unexpected target SHALL block mutation.

#### Scenario: Large secondary mount

- **WHEN** an administrator applies uninstall while a large backup or user mount exists
- **THEN** the command completes without traversing either mount

#### Scenario: Replaced unit

- **WHEN** a known unit path contains unexpected content or is a symlink
- **THEN** uninstall fails before stopping or removing any service

#### Scenario: Retry

- **WHEN** the administrator repeats uninstall after its files have been removed
- **THEN** it succeeds without scanning the filesystem or reporting a false failure

### Requirement: Clear retained resources

Uninstall SHALL retain configuration, token store, profiles, dedicated identities and groups, shared journals, and administrator-granted permissions. The preview and result SHALL name these categories and explain the reason identities remain. It SHALL not imply those resources were removed or proven unused.

#### Scenario: Preview

- **WHEN** an operator runs `uninstall` without `--apply`
- **THEN** it reports planned removals and retained resources without requiring root or changing state
