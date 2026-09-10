## Context

The current packaging code uses Go builds plus Python's standard tar/gzip and hashing libraries. Its extra responsibilities are the upgrade manifest, dependency license collection, deterministic metadata, and exact-candidate validation. GoReleaser disables its own builds/checksums and only publishes.

## Goals / Non-Goals

Make ordinary builds require only Go. Keep one local/CI packaging path and exact tested-byte delivery. Keep test isolation and meaningful native-platform boundaries. Native OS packages, product containers, platform implementation, and automatic host rollout remain out of scope.

## Decisions

- Use `go build` directly for local binaries and expose existing archive construction as `package`. Keep Make and Just independent and equivalent.
- Retain tar.gz bundles because installation needs two binaries, profiles, configuration examples, and an integrity manifest. Replacing these with a bare executable would change the installation contract.
- Keep GoReleaser. Let it construct archives and checksum companions using its OSS features; use standard Go compilation for the two executables. Remove bespoke tar/gzip/checksum construction and trivial shell forwarding scripts.
- Preserve the HostLens upgrade manifest and dependency license notices through a focused preparation adapter. A disposable OSS proof confirms global before hooks run sequentially after output cleanup. Use those hooks for Go builds and metadata preparation, then meta archives. This avoids concurrent build-hook ordering and Pro-only archive hooks.
- Publish the exact tested archives through the existing draft-only GoReleaser release path. Delivery must not rebuild. Keep package preparation and publishing responsibilities explicit.
- Use one standard `checksums.txt` for both archives. Keep the internal manifest schema and per-member hashes unchanged. Put distributable output in `dist/archives`; keep expanded matrix inputs outside GoReleaser's cleaned output directory. Remove bundled Go module files and pre-rendered systemd units, which installation does not consume. Keep operator guidance and dependency identities.
- Use a project test image derived from pinned official Go/Debian images, with Python, race compilation prerequisites, and the scanner built from existing locked development modules. Test launchers retain offline runtime-module access and resource/isolation limits.

## Risks / Trade-offs

The test image needs explicit preparation and reviewed base updates. Docker bind mounts still require source/module paths visible to the engine. macOS/Windows product runtimes remain future work. Local workflow/command tests do not establish an actual release upload or hosted CI success for these uncommitted changes.

## Migration Plan

Update documented `build` archive consumers to `package`; retain ordinary Go commands for quick builds. Keep GoReleaser configuration validation in local commands and CI. Remove superseded custom code and update all callers. Validate locally and deliver a concise source-grounded report with uncommitted changes. Do not publish this stage.
