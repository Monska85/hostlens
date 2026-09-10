## Context

The tab 4 review targets 2,382 lines across packaging, scripts, and tools. Keep product safety and meaningful acceptance coverage, while deleting the reusable candidate bookkeeping and local scheduling framework.

## Goals / Non-Goals

Tests must execute the selected archive bytes in disposable environments. They must not claim those bytes represent current source unless packaging was explicitly run. Keep GoReleaser OSS, Linux privilege tests, and future platform boundaries. Do not introduce another framework or paid tooling.

## Decisions

- Use one serial local dispatcher with inherited console output. GitHub Actions retains native matrix concurrency, timeouts, and web logs. Launchers own container cleanup; the dispatcher forwards cancellation to the active process group.
- Compile small static acceptance helpers into a temporary directory for each invocation using the selected checkout and prepared Go cache. This replaces persistent helpers and their checksum protocol. CI prepares Go and module dependencies for these fixtures.
- Move the existing Go archive reader into a portable source file within lifecycle. Accept an explicit expected architecture for offline inspection; upgrade always supplies its native architecture. One small helper verifies and extracts release archives through that reader, with packaging assertions for licenses, modes, and expected version.
- Run platform and systemd tests from fresh extracted archives. Keep staging only as a packaging implementation detail. Remove Python tar parsing, source fingerprints, expanded-tree comparison, helper hashes, and the redundant dependencies.json inventory.
- Keep a single root GoReleaser packaging configuration. Standard gh draft upload replaces the publishing-only configuration, preserving tag eligibility, exact filenames, checksum verification, prerelease handling, and full CI gating.
- Delete the empty shipped web-common dependency. Existing administrator profiles remain untouched by upgrade; general include support remains supported.

## Risks / Trade-offs

Full local matrix runs become serial. Test invocation now needs Go and prepared modules to build disposable helpers; this cost buys a smaller trust and maintenance surface. CI can reuse its Go cache. Container output streams directly to the console; GitHub retains normal job logs, without a custom artifact-log subsystem.

## Migration Plan

Replace local concurrency and stale-source promises with explicit archive selection in specs and docs. Compare line counts against both the incoming tree and committed baseline, including new files. Validate corruption, cancellation, profile preservation, both architectures, and both privilege modes in containers. Leave changes uncommitted and unpushed.
