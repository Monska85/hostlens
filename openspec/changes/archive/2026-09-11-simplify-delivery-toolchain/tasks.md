## 1. Ground the delivery model

- [x] 1.1 Compare official Go/GoReleaser behavior and pinned upstream project configurations; record sources and retained guarantees in the review report.
- [x] 1.2 Complete the steered panel review, triage actual findings, and verify closure within five rounds without a findings quota.

## 2. Simplify the workflow

- [x] 2.1 Separate local build and distribution packaging in equivalent Make/Just commands; verify a Go-only build and the existing archive/matrix contract.
- [x] 2.2 Use GoReleaser for archive/checksum construction and preserve exact-candidate draft delivery; remove superseded scripts/tools, validate configuration and archive safety without publishing.
- [x] 2.3 Replace the unrelated test image and host tool mounts with a pinned container-owned toolchain; validate image build, race/coverage tests, scanner execution, and cleanup.

## 3. Deliver evidence

- [x] 3.1 Reconcile docs/specs and validate formatting, links, and current/archive requirements.
- [x] 3.2 Run affected container suites, archive verification, and the shared matrix; report platform and publication limits accurately.
- [x] 3.3 Deliver a concise report explaining the resulting commands, retained tools, removed dependencies, evidence, and limits. Leave all stage changes uncommitted and unpushed.
