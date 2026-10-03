# Tasks

## 1. Baseline and dependencies

- [x] 1.1 Inventory live commands, release members, installed state, tool contracts, and old configuration; verify the inventory against 0.5.0 archives and source.
- [x] 1.2 Record production, test, documentation, and tooling line counts and container benchmark results; verify repeatable bounded workloads.
- [x] 1.3 Verify official latest stable Go, MCP, system, Docker, systemd, CLI, packaging, and development releases for identity, security, maintenance, and platform compatibility; pin modules and checksums and verify tidy and portable builds.
- [x] 1.4 Run Go dead-code reachability, call-site, build-tag, and coverage analysis; record candidates and verify each removal against public behavior.

## 2. Read-only product

- [x] 2.1 Implement a small typed MCP registry with SDK-derived schemas and one fail-closed discovery and dispatch gate; verify malformed, denied, and allowed calls in container tests.
- [x] 2.2 Implement bearer token creation, verification, revocation, and role separation with bounded durable state; verify expiry, rotation, and secret containment in container tests.
- [x] 2.3 Implement activated profiles with per-tool and resource grants and denial precedence; verify inactive, conflicting, unknown, and malformed cases in container tests and document the new format.
- [x] 2.4 Implement bounded Linux amd64/arm64 host and service adapters through maintained libraries; verify live status, invalid targets, canceled calls, and unavailable interfaces in container tests and document coverage limits.
- [x] 2.5 Implement Docker engine and container observations through the maintained client behind named operations; verify bounded responses, profile filtering, and no mutating diagnostic call using the private nested Engine and document coverage limits.

## 3. Controlled repair

- [x] 3.1 Implement separate repair identity, local protocol, role and profile checks, read-only activation drain, and live revalidation; verify denial and identity boundaries in disposable containers.
- [x] 3.2 Implement named systemd service restart with expected identity and post-action observation; verify success, changed target, refusal, and failure inside disposable systemd containers and document its effect.
- [x] 3.3 Implement named Docker container restart with expected stable ID and post-action observation; verify success, changed target, refusal, and failure against a private nested Engine and document its effect.

## 4. Delivery and migration

- [x] 4.1 Implement a checked 0.5.0 configuration and token candidate, report unmapped profiles and installed-state actions, verify convertible and rejected cases without touching host systemd, and document operator-controlled activation and rollback.
- [x] 4.2 Build and verify Linux amd64 and arm64 archives with current CLI help, checksums, licenses, and shipped documentation; verify fresh installation, targeted repair, read-only activation, and configuration rollback in a disposable systemd container.
- [x] 4.3 Remove superseded production packages, dead tests, scripts, dependencies, and old active specs; update README, AGENTS.md, operations, release documentation, and changelog; verify links, commands, Go reachability, and no stale release claims.

## 5. Integration and review

- [x] 5.1 Run format, lint, race, vet, vulnerability, build, package, private Docker, disposable systemd, and strict OpenSpec checks; configure native arm64 CI as a release gate and distinguish its pending evidence from a pass.
- [x] 5.2 Compare the same bounded container benchmarks and line-count categories before and after, investigate regressions, and record measured limits.
- [x] 5.3 After complete implementation, run eight peer-review panel rounds with experts chosen for the changed surface; verify and fix findings, rerun affected checks, and record each round.
- [x] 5.4 Inspect final diff and staged files, then make one conventional commit containing the full rewrite and verify the branch has only one new commit.
