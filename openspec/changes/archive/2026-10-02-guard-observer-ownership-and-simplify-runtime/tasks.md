# Tasks

## 1. Observer ownership

- [x] 1.1 Reject unrecorded, removed-then-replaced, or wrong-type observer paths before plan or apply; verify disposable lifecycle regressions preserve files and make no service or identity calls.
- [x] 1.2 Require a usable release source before applying a missing binary and recheck paths before mutation; verify older-installation recovery and interrupted-state tests.
- [x] 1.3 Update operator guidance for observer conflicts and verify the documented recovery steps match the CLI behavior.

## 2. Local IPC separation

- [x] 2.1 Reject diagnostic/admin socket collisions independent of Docker configuration; verify disabled-Docker and valid distinct-path cases in container tests.
- [x] 2.2 Update the configuration contract and operator notes, then validate the delta spec strictly.

## 3. Runtime reduction

- [x] 3.1 Replace hash-only whole-file reads with streamed checks in lifecycle paths; verify checksum, drift, uninstall, and recovery cases in disposable containers.
- [x] 3.2 Measure Docker list-policy work with the repository benchmark before and after any simplification; keep a change only if denial behavior and measured cost improve.
- [x] 3.3 Record user-visible changes in the changelog and update validation evidence with measured results and limits; verify Markdown formatting and links.

## 4. Integration and review

- [x] 4.1 Run the repository's race, vet, Linux and portable build, lint, archive, private Docker, and feasible platform acceptance checks; record passed, failed, and unavailable cases separately.
- [x] 4.2 Review the complete diff against ownership, policy, API, documentation, and performance contracts; resolve findings and validate change and main specs before archiving.
