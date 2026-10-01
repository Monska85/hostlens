# Tasks

## 1. Baseline and deletion map

- [x] 1.1 Record production Go lines, package sizes, allocations, and representative request benchmarks in `review.md`; verify commands and fixtures can reproduce them in disposable containers.
- [x] 1.2 Map duplicate logic, state ownership, hot paths, and trust boundaries across every production package; record a concrete deletion map in `review.md` and verify each proposed removal against current specs and tests.

## 2. Gateway, backend, and contracts

- [x] 2.1 Simplify gateway admission, authorization, validation, and response flow; verify MCP schema snapshot, role and effect denials, cancellation, and representative request benchmarks.
- [x] 2.2 Simplify backend preparation, activation, and IPC without retained evidence; verify reload, generation mismatch, bound, and failure-path tests.
- [x] 2.3 Remove justified policy and contract duplication while preserving one typed tool definition and fail-closed effects; verify policy and contract tests and portable builds.
- [x] 2.4 Update affected MCP, security, and operator documentation; verify published member names and examples against the executable behavior.

## 3. Collectors and observer

- [x] 3.1 Simplify generic Linux collection, source access, pagination, and budgets; verify bounded failure, denial, cancellation, and evidence-gap scenarios in disposable containers.
- [x] 3.2 Simplify Docker observer decoding, correlation, and tool dispatch without broadening its GET-only authority; verify truncation, alias, race, no-mutation, and live-engine behavior where available.
- [x] 3.2a Reuse the image projection for inventory and disk usage, including the dangling fact for untagged images; verify the disk-usage scenario in a disposable container.
- [x] 3.3 Update affected diagnostic and Docker documentation; verify examples and OpenSpec requirements remain accurate.

## 4. Operations and future boundaries

- [x] 4.1 Simplify configuration, token, CLI, and lifecycle transaction code; verify trusted reads, rollback, install, upgrade, and removal in disposable container cases.
- [x] 4.1a Bound installation manifest decoding before lifecycle mutation; verify oversized and trailing state rejection in a disposable container.
- [x] 4.2 Keep native Linux operations out of portable packages and retain a distinct future remediation authority; verify macOS and Windows cross-builds and read-only transition tests.
- [x] 4.2a Reserve a distinct remediation role in the registry while keeping v1 token issuance read-only; verify mismatched effect/role definitions are rejected.
- [x] 4.3 Update `AGENTS.md`, operator procedures, and changelog where behavior or workflow changed; verify links and documented commands.

## 5. Full-product review cycles

- [x] 5.1 Review the complete changed product for correctness, security, performance, line reduction, idiomatic Go, and compatibility; record findings and dispositions in `review.md`.
- [x] 5.2 Repeat full implementation and review as needed, up to eight rounds total; verify every in-scope finding is fixed or explicitly reported as an unmet gate.
- [x] 5.3 Compare final production lines and workloads against baseline and `origin/main`; report the missed stretch target and verify no material security or performance regression before claiming success.

## 6. Delivery validation

- [x] 6.1 Run the complete disposable-container race, vet, build, coverage, format, lint, and vulnerability checks; record exact outcomes and container limits.
- [x] 6.2 Build coherent amd64 and arm64 archives, verify checksums and shipped links, and pass the full disposable acceptance matrix on the exact final archives.
- [x] 6.2a Bound release manifest decoding and verify malformed, trailing, and oversized archive rejection in disposable containers.
- [x] 6.3 Validate current and archived OpenSpec artifacts and inspect the complete staged diff before the single local commit.
