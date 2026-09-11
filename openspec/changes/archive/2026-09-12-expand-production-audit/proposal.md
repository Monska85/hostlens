## Why

The initial diagnostic tools cannot provide enough evidence for an agent to investigate an unfamiliar Linux service without SSH. Add general OS audit primitives and explicit coverage reporting so application knowledge stays with the agent.

## What Changes

- Add bounded process, network, account, storage, maintenance, security-control, file-metadata and HostLens deployment observations.
- Add explicit audit-domain policy grants. Preserve existing file/journal denial precedence, token roles, secret protection and OS privileges.
- Extend service evidence with selected runtime and hardening properties without command arguments or environment values.
- Demonstrate PostgreSQL, Apache, nginx and MySQL investigation using generic tools and approved configuration/log sources, without application-specific code or database access.
- Document unsupported, inaccessible and stale evidence. Do not claim a complete security audit from partial observations.

## Capabilities

### New Capabilities

- `production-audit`: generic, scoped OS and HostLens audit evidence and application-independent investigation.

### Modified Capabilities

- `access-policy`: explicit audit-domain grants with global denial precedence.
- `mcp-diagnostics`: additional tool schemas, role requirements and richer service evidence.

## Impact

Portable contracts, Linux collectors, source policy, MCP schemas, tests and operator guidance. Existing restricted mode and standard-mode CAP_DAC_READ_SEARCH remain unchanged. No privileged helper, arbitrary command execution, SQL execution, application-specific integration, background collection or automatic remediation. macOS and Windows remain future native implementations.
