# Final validation gaps

The latest tab 4 review identified two defects and missing retained regressions. All are addressed without restructuring.

- **Checksum coverage:** Archive verification and publication compare the sorted standard SHA-256 output for both expected archives with the complete supplied list. Missing, extra and conflicting entries fail. Retained tests exercise the actual verification commands and publication block with a mock uploader.
- **Status deadlines:** Each probe runs under coreutils timeout with its remaining budget. Zero remaining time never disables timeout, and a late successful probe is rejected. A retained stalled-process regression exercises the production polling function with a shortened budget.
- **HTTP readiness:** The existing context deadline is now injectable for tests. Retained tests cover the expected authentication response, unexpected responses, redirects and a stalled HTTP handler.

Validation passed in disposable containers: race tests, vet, Linux/portable builds, archive verification and all six acceptance cases. Thirteen Python regressions passed across the full run and the focused rerun after adding the stalled-process test. Internal statement coverage remains 76.7%. Formatting, lint and specification checks passed.

The release command was tested with a mock uploader. No commit, push, hosted CI run or publication was performed. Local ARM64 acceptance used emulation; no VM was used.
