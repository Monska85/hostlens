# Proposal: harden-token-store-directory

## Why

`Store.read()` in `internal/token/token_linux.go` validates the token file (type, permissions, owner, size, `O_NOFOLLOW`) but never its parent directory, while `Store.change()` validates the directory. A local attacker with write access to the containing directory can swap in a crafted `tokens.json` between the directory check (only on admin writes) and a verification read, injecting arbitrary roles. The same pre-existing-misconfiguration class is already denied for hard-link aliasing; directory-level swap must be denied too.

## What Changes

- Factor a shared directory-trust helper used by both `read()` and `change()`: the parent directory must be a real directory (via `Lstat`, no symlink), owned by root or the effective user, and lack group/other write bits.
- `read()` now fails with a precondition error when the parent directory is untrusted, so bearer verification and admin listing refuse tokens from an attacker-writable directory.
- `change()` gains the owner check (it previously checked only type and write bits), matching the file-level owner policy.
- Table-driven tests covering trusted directories, group-write, other-write, non-root owner, symlinked directory, and root-owned directories for euid 0.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `token-authorization` — add a storage-protection requirement naming the precondition checks applied on both read and write paths, with failure scenarios.

## Impact

- `internal/token/token_linux.go` (`read`, `change`, new shared helper).
- `internal/token/token_linux_test.go` (new directory cases).
- `openspec/specs/token-authorization/spec.md` (via delta, applied at archive time).
- Behavior note: deployments whose token directory is group/other-writable or owned by an untrusted user now fail closed with a precondition error instead of accepting credentials from a swappable directory.