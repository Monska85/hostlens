## 1. Reduce the implementation

- [x] 1.1 Replace scheduler and its tests with a small serial dispatcher and focused selection/failure/cancellation regressions.
- [x] 1.2 Share the Go archive reader between upgrades and artifact acceptance; remove Python parsing, fingerprints, expanded-tree checks, and helper hashes.
- [x] 1.3 Build disposable helpers and run acceptance from extracted archives; preserve native CI, bounded containers, and actual lifecycle coverage.
- [x] 1.4 Consolidate GoReleaser configuration and use standard draft upload with exact-artifact and tag gates.
- [x] 1.5 Remove the empty shipped profile and unused dependency inventory while preserving existing profiles and notices.

## 2. Verify and report

- [x] 2.1 Reconcile specifications and contributor/operator instructions, then validate formatting, links, and specification consistency.
- [x] 2.2 Run container regressions, packaging verification, full acceptance, static checks, and safe publication-command fixtures.
- [x] 2.3 Triage every tab 4 recommendation and report net line changes across implementation, tests, configuration, and dependencies. Do not commit or push.
