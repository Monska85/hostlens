# Delivery review

GoReleaser stays. Standard Go compiles the programs; GoReleaser creates distribution archives and checksums, then publishes the exact validated files through the existing draft-release workflow. This stage is uncommitted and unpushed.

## Resulting workflow

| Command                                     | Result                                                               |
| ------------------------------------------- | -------------------------------------------------------------------- |
| `make build` / `just build`                 | Linux binaries in `bin/`, requiring Go only                          |
| `make package` / `just package`             | GoReleaser snapshot archives and `checksums.txt` in `dist/archives/` |
| `make test-image` / `just test-image`       | Explicitly prepared container validation toolchain                   |
| `make coverage` / `just coverage`           | Container regressions, console total, local HTML and Go profiles     |
| `make test-matrix` / `just test-matrix`     | Existing candidate checked across the shared six cases               |
| `make release-check` / `just release-check` | Packaging and publishing configuration validation                    |

Version-tag CI builds once, validates that candidate, and gives only the publishing job write permission. Publication does not rebuild or roll out hosts.

## Findings and remediation

| Severity | Verified issue                                                             | Resolution                                                                                      |
| -------- | -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| Medium   | Ordinary builds performed full distribution work                           | Direct Go build; distinct package command                                                       |
| Medium   | Custom tar/gzip/checksum implementation duplicated GoReleaser capabilities | GoReleaser OSS meta archives and standard checksum list                                         |
| Medium   | Tests used an unrelated linter image plus host Linux tools                 | Pinned Go/Debian validation image owns Go, Python, race prerequisites, and scanner              |
| Low      | Three shell files only forwarded to Python                                 | Deleted wrappers; direct calls from runners and CI                                              |
| Low      | Generated service-unit copies and module files had no installer consumer   | Removed unit renderer and those archive assets; installer continues generating configured units |
| Medium   | Initial change omitted the affected installation requirement               | Added explicit installer-generated service-definition requirement and scenario                  |
| Low      | Version values could be interpreted as CLI options                         | Added argument separators and invalid-option regression cases                                   |

Deleted `scripts/build-archives.sh`, `scripts/release-version.sh`, `scripts/verify-archives.sh`, `scripts/test-matrix.sh`, `tools/package-archives/main.py`, and `tools/render-units/main.go`. The replacement metadata adapter does not implement an archive format or checksum-file writer.

The steered panel used all nine role bodies: security, adversarial, idiomatic, maintainability, performance, specification, intent, documentation, and CI/CD. Discovery and OSS proofs informed implementation; the corrected-candidate review found the missing installation requirement. That finding was verified and reconciled. Other final role reviews returned no findings, without a finding quota.

Final verification also moved the success message after checksum-list validation; a corrupt checksum file now fails without printing a premature success line. The existing corruption regression verifies this behavior.

## Why the remaining code exists

- **Release metadata and verification:** HostLens upgrades require an architecture/version manifest with per-member hashes. Dependency notices, extraction bounds, complete gzip integrity, and matrix identity checks are project requirements beyond ordinary Go compilation or GoReleaser archive creation.
- **Container scripts and matrix runner:** These implement disposable lifecycle tests, native/emulated selection, bounded concurrency, cancellation, and useful console stages. Removing them would remove acceptance coverage or duplicate it in CI and task runners.
- **Go smoke and containment helpers:** These test MCP behavior and Linux credentials against the actual binaries. They are test fixtures, not an alternative compiler or packager.
- **Development manifests and dispatch:** They lock Ruff, actionlint, shfmt, and the scanner independently of runtime modules; ShellCheck remains a documented base prerequisite. Make and Just remain equivalent and independent.

No new language, runtime dependency, native package manager, product container, or VM workflow was added.

## Grounding

[Go build/install](https://go.dev/doc/tutorial/compile-install) produces executables. [GoReleaser archives](https://goreleaser.com/customization/package/archives/) and [checksums](https://goreleaser.com/customization/package/checksum/) cover distribution packaging. An OSS 2.18.1 container proof verified sequential global hooks, nested file paths, regular-file-only archives, modes, version injection, and standard `sha256sum` compatibility. No Pro-only archive hooks or assumed parallel build order are used.

Upstream practice varies: [Caddy uses GoReleaser with project preparation](https://github.com/caddyserver/caddy/blob/425a3381fdf65c826dd991c92e78eaf6ef24941e/.goreleaser.yml), [node_exporter uses promu for build/tarball metadata](https://github.com/prometheus/node_exporter/blob/17ddd77c59ba27e1508e9f7894b1e55b44d6aed3/.promu.yml), and [restic wraps Go compilation](https://github.com/restic/restic/blob/ba802d42b7294c98b62c16d1157ea3e80820c019/build.go). These examples support using standard tools with justified project adapters; they do not establish one universal delivery framework.

## Verification and limits

Real GoReleaser archives passed manifest and expanded-tree verification. Make and Just produced identical archive checksums on repeated builds with the same source and toolchain. All six container matrix cases passed, including both systemd privilege modes. Full container race tests, vet, portable package builds, Python regressions, and coverage passed; internal Go statement coverage remains 76.6%. Formatting, workflow lint, both GoReleaser configurations, and the container vulnerability scan passed.

The validation image decreased from 913,774,552 to 579,060,878 bytes on this amd64 engine, about 36.6%. Its ARM64 execution and a whole-image OS vulnerability scan were not performed. Local ARM64 acceptance used explicit emulation; native ARM64 evidence belongs to the previously published QA baseline.

[QA baseline CI](https://github.com/Monska85/hostlens/actions/runs/34567760567) passed all nine jobs at `212488a7d7211452b206df496219d9349a1299d8`. These uncommitted delivery changes have local evidence only. No release upload, publisher authentication signature, or host rollout is claimed.
