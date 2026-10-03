# Proposal

## Why

The released CLI makes common administration commands hard to use: `status` can report only a bare missing-file error for a system installation, top-level help does not explain commands, flag help shows misleading one-dash forms and unrelated options, and uninstall prints an opaque ACL warning. These failures hide the required action and undermine trust in the installation.

## What Changes

- Show grouped command help and accurate, command-specific options using the documented `--flag` spelling.
- Make `hostlens help COMMAND` and `COMMAND --help` resolve the same command guidance without loading configuration or changing state.
- Select the system configuration by default for root invoking `status` when no configuration mode or path is specified. Keep non-root administration protected and report missing configuration or socket with an actionable path and command.
- Explain plainly which journals and manually granted permissions uninstall leaves for the administrator.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `configuration-lifecycle`: Define discoverable CLI help and actionable local status failures without changing administration authority.
- `installation-lifecycle`: Define understandable uninstall ownership guidance for resources outside the installation manifest.

## Impact

The Linux CLI, its integration tests, operator documentation, and two existing specifications change. MCP tool contracts, service credentials, and installation ownership rules do not change.
