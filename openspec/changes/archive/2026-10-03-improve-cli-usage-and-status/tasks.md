# Tasks

## 1. Command help

- [x] 1.1 Render grouped top-level and command-specific help from accepted options; verify help works without configuration and shows only double-hyphen spellings for supported flags in container CLI tests.
- [x] 1.2 Route `help COMMAND` and `COMMAND --help` to equivalent output and reject irrelevant options before loading; verify both valid and invalid paths in container CLI tests.
- [x] 1.3 Document help discovery and per-command flags in the operator guide; verify the examples against the built CLI.

## 2. Status diagnostics

- [x] 2.1 Default root `status` to the system installation without changing explicit mode or path selections; verify system, user, and explicit-override cases in container CLI tests.
- [x] 2.2 Give missing configuration and socket errors the selected path and safe next action without weakening identity checks; verify the failure paths in container CLI tests.
- [x] 2.3 Document `status` defaults and root requirements in the operator guide; verify the examples against the built CLI.

## 3. Uninstall notice

- [x] 3.1 Replace the opaque uninstall warning with plain ownership and cleanup guidance; verify the preview text in a disposable lifecycle test.
- [x] 3.2 Align the uninstall guide with the CLI notice; verify the documentation describes the existing ownership boundary.

## 4. Integration validation

- [x] 4.1 Run the repository's disposable-container race, build, formatting, lint, and relevant acceptance checks; record actual results and limits in validation evidence.
- [x] 4.2 Validate current and archived OpenSpec artifacts, reconcile main specs when appropriate, and check off tasks only after their required behavior passes.
