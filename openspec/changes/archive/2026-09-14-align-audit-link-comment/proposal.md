# Proposal: align-audit-link-comment

## Why

The `auditLink` comment (`internal/platform/linux/audit_process_linux.go`) promises "executable paths require target policy approval", but the exe-target check calls `Policy.Allowed(..., true)` with builtins default-allowed. The comment misdescribes the boundary an operator would rely on.

## What Changes

- Correct the comment: executable target paths are default-allowed unless denied by policy (the check still denies explicit policy denials); only the path string is exposed, never link contents.
- No code change. `builtin=true` stays: tightening to `builtin=false` would change collection behavior for stock installations and is out of proportion for 0.2.x (recorded decision).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None — comment-only; `skip_specs: true`.

## Impact

- `internal/platform/linux/audit_process_linux.go` comment text only.