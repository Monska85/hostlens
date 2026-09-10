# Tab 4 review resolution

All addressable recommendations were implemented. GoReleaser remains the packager. No commit, push, or release was performed.

## Changes

| Review item                         | Resolution                                                                                                                                                                                   |
| ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Custom local scheduler              | Serial dispatcher with named selection, visible output, aggregate failure and cancellation cleanup. GitHub Actions owns hosted parallelism.                                                  |
| Duplicate candidate representations | Acceptance extracts and executes selected archives. Disposable helpers replace source fingerprints, helper hashes and expanded-tree comparisons.                                             |
| Independent archive parsers         | Acceptance uses the runtime Go reader, with small checks for release materials, version and archive checksum. Corruption, unsafe headers and resource boundaries retain regression coverage. |
| Packaging orchestration             | One root GoReleaser OSS configuration; two explicit Go cross-build hooks and a metadata adapter. Standard gh uploads the tested files to a draft after CI succeeds.                          |
| Empty profile and inventory         | Removed shipped web-common and dependencies.json. Tests verify that administrator-created profiles survive upgrades. Dependency licenses and notices remain.                                 |
| Useful acceptance assets            | Kept real platform/systemd tests, protocol and containment probes, bounded container launchers, image definitions and development locks. No folder relocation presented as removal.          |
| Overspecified requirements          | Replaced scheduler and fingerprint requirements with selected-archive identity, observable results and cleanup guarantees. Updated contributor and agent instructions.                       |

GoReleaser OSS has no aggregate post-build hook before archiving; ordinary per-build hooks cannot safely generate a manifest covering both binaries. Retaining explicit builds avoids adding synchronization code or a Pro dependency. See [build hooks](https://goreleaser.com/customization/builds/hooks/) and [archive configuration](https://goreleaser.com/customization/package/archives/). Draft upload uses documented [gh release create](https://cli.github.com/manual/gh_release_create) flags.

## Measured result

Counts include comments, blank lines, new files and deleted files, compared with committed HEAD. Code moved into the shared reader is included.

| Maintained material                                             |   HEAD |  Final |   Change |
| --------------------------------------------------------------- | -----: | -----: | -------: |
| Scripts                                                         |    691 |    626 |      -65 |
| Tool implementation                                             |    756 |    587 |     -169 |
| Tool tests, including new Go tests                              |    652 |    483 |     -169 |
| Packaging, workflows, task runners and tool configuration/locks |    637 |    722 |      +85 |
| Runtime/CLI implementation and tests                            |        |        |      +82 |
| All repository code, tests and configuration                    | 11,240 | 11,004 | **-236** |

Documentation and OpenSpec history are excluded from the code totals and increase separately. The report does not claim that total repository text shrank. No dependency was added.

## Verification

- **Container regression suite:** Go race tests, vet, Linux builds, portable package cross-builds and all ten Python tests passed. Combined internal statement coverage: 76.7%.
- **Release acceptance:** GoReleaser configuration/package checks and shared-reader verification passed for both archives. All six platform/systemd cases passed; ARM64 was emulated locally. The systemd context correction passed a further restricted-mode case.
- **Failure paths:** Unsafe-header assertions passed after strengthening their expected-error checks. Cancellation and partial-failure regressions passed. Disposable publication fixtures verified stable/prerelease asset selection, checksum rejection before gh and propagation of upload failure.
- **Static and dependency checks:** Formatting, Ruff, ShellCheck, actionlint, specification validation and dependency vulnerability scanning passed.
- **Independent adversarial pass:** Found one missing systemd image/mode/architecture context line, now fixed. No further concrete finding in the reviewed extraction, cleanup or CI gates.

This working tree has local evidence only. Hosted native ARM64 and release workflow execution remain unverified for these uncommitted changes. Containers share the host kernel; no VM was used.

## New workflow

Use Go alone for compilation: make build or just build. Use make package or just package for installable archives. With documented prerequisites prepared, test-matrix accepts those existing archives; package first to test current source. Full local acceptance runs sequentially, while CI runs the shared cases in parallel and gates delivery of the same archive bytes.
