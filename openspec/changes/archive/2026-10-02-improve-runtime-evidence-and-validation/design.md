# Design

## Context

The current live Docker tests are opt-in and can target an arbitrary socket. The standard checks container has no daemon or Docker CLI. Docker list collection builds image, volume, and network reference maps for each resource-specific request. Lifecycle manifests already have byte ceilings but accept unknown fields.

## Goals / Non-Goals

**Goals:** Prove the Docker observation path against an isolated engine, reduce unnecessary reference indexing, and fail closed on undeclared lifecycle state.

**Non-Goals:** Add MCP tools, support Docker mutation, use the host daemon as a fixture, change JSON member names, or claim production load and native macOS/Windows behavior.

## Decisions

- Run the official Docker 29.8.0 DinD image pinned by digest in a disposable privileged container with no network, temporary daemon storage, and a private Unix socket. Load the official BusyBox 1.37.0 musl fixture image from a pinned outer image into the nested daemon before tests. The test process runs in the established checks image with source and module cache read-only. This avoids touching host Docker workloads. Running against the host daemon was rejected because the fixture test mutates resources.
- Make the live tests require a private runner marker and fail, rather than skip, when explicitly invoked. Keep ordinary race/coverage runs offline and unprivileged. A missing kernel namespace feature or unavailable image fails the live gate with context.
- Construct only the reference indexes required by each Docker list tool. Disk accounting retains image and volume indexes. Compare the same bounded benchmark fixtures with the current implementation and after the edit; retain direct policy and race tests.
- Decode Docker's PID limit as an unsigned value and omit the finite-limit field when the daemon returns an unrepresentable unlimited sentinel. Keep the public JSON member and finite values unchanged.
- Use the existing bounded strict JSON decoder for installation and release manifests. This preserves on-disk schema 1 while refusing input the code cannot interpret. Add failure-path tests before mutation or candidate acceptance.
- Bound local account-file reads used for service identity resolution. Do not cache identity evidence or change UID/GID semantics.
- Stream TLS file bytes into the existing restart fingerprint hash so restart detection retains the same value without a whole-file allocation.
- Pass the selected matrix architecture into the systemd runner and reject a mismatch with the connected Docker engine before container creation. Native arm64 systemd acceptance remains a hosted arm64 runner gate.

## Risks / Trade-offs

- Privileged DinD shares the host kernel. The private socket and storage limit fixture effects to the nested engine, but this is not VM isolation or production-scale evidence.
- Nested containers may fail under some runner kernels or cgroup policies. The explicit acceptance target reports failure instead of claiming a pass; hosted CI should run it only on a compatible runner.
- A new strict unknown-field check can reject hand-edited or future-format manifests. Schema 1 has no extension contract, and a clear rejection is safer than partial interpretation.

## Migration Plan

Existing generated manifests remain valid. No state migration or MCP client change is required. Rollback uses the previous coherent release archive.
