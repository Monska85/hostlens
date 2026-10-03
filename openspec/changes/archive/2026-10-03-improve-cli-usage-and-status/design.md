# Design

## Context

The CLI used Go `flag.FlagSet` for administration commands. A single set registered token, policy, and common options for every command, so its generated help advertised irrelevant one-dash options. Status defaulted to a user configuration path even when root manages a system installation. The uninstall notice did not explain the operator action.

## Goals / Non-Goals

**Goals:** Make help match accepted command options, select the expected status configuration for root, and make missing resources and uninstall guidance actionable.

**Non-Goals:** Change socket permissions, allow non-root access to system status, alter lifecycle ownership, or change MCP behavior.

## Decisions

- Use Cobra's command tree, parser, and help. Register only relevant flags on each command, so help derives from accepted options and shows double-hyphen spelling. Disable the unused completion command.
- Let Cobra resolve `help COMMAND` and `COMMAND --help` before configuration loading and service contact. Unknown commands and options fail before loading.
- Default only `status` to system mode when the effective UID is root and no mode or path was supplied. `reload` remains explicit because it mutates service state. A non-root missing user configuration gets a command hint without reading system configuration.
- Wrap missing configuration and socket errors with the selected path and a safe service-check action. Preserve underlying error classification and existing identity checks.
- Replace the uninstall notice with a direct statement about retained journal entries and manual permissions. The actual removal plan and ownership records remain authoritative.

## Risks / Trade-offs

- Root users with a user-mode instance must pass `--config` or explicitly choose their mode for `status`. The help explains this default.
- The new CLI dependency adds code to the binary. Its maintained parser and generated help replace the repository's command dispatch and custom help rendering; command-specific behavior remains in HostLens.

## Migration Plan

No configuration or data migration is required. Existing `--system` and `--config` calls continue to select the same installation.
