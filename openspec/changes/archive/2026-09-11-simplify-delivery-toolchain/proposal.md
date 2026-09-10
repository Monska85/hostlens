## Why

Developers currently build full distribution bundles when they request a build. Custom scripts construct archives while GoReleaser only publishes existing files, and tests use an unrelated linter image while mounting a second Go toolchain. Reduce those costs without weakening the release and upgrade contract.

## What Changes

- **BREAKING developer command:** `build` produces local binaries through standard Go tooling; `package` produces the existing installable amd64/arm64 archives.
- Keep GoReleaser for packaging and verified, draft-only delivery. Replace custom archive/checksum construction and redundant command wrappers with standard tools.
- Build an explicit test image with the selected Go toolchain, Python, race-test compiler support, and locked vulnerability scanner. Remove host Linux toolchain/scanner mounts.
- Retain standard tar.gz delivery and only the project-specific manifest, license, safety, and acceptance code that standard tools cannot replace. Remove unused generated assets. Document retained responsibilities.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `installation-lifecycle`: generate service definitions during installation rather than shipping unused rendered copies.
- `release-validation`: distinguish ordinary builds from distributable bundles and make the container own executable validation tools.

## Impact

Task runners, CI, test-image preparation, container launchers, and development documentation change. Runtime features and upgrade integrity and runtime behavior remain unchanged. No commit, push, or publication is authorized for this stage. Review may close before five rounds when no findings remain.
