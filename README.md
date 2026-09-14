# HostLens

HostLens gives agents controlled access to host diagnostics through the Model Context Protocol (MCP). It returns observed system information and explicit collection failures, with bearer authentication, fixed roles, and administrator-defined source policies.

**Scope:** Linux amd64 and arm64, with systemd installation. macOS, Windows, and separately authorized remediation are future work. V1 exposes no shell execution or remediation tools.

## What it does

- **Inspect:** OS identity, hardware inventory, resource health, services, and installed packages.
- **Audit:** Explicitly authorized process, network, account, storage, security, metadata and deployment evidence through generic tools.
- **Observe Docker:** Opt-in, read-only evidence from the local system-wide Docker Engine through an isolated observer with a typed GET-only contract.
- **Read approved sources:** Bounded UTF-8 configuration files, JSONL logs, raw tails, and selected journal units.
- **Control access:** Expiring tokens, role checks on each call, profile includes, and global denial precedence.
- **Operate locally:** Explicit installation, policy reload, token administration, recoverable upgrades, and tracked removal.

The gateway and diagnostic backend run as separate processes and identities. The gateway authenticates clients; the backend enforces source policy and collection limits. Neither component calls model providers or schedules background monitoring.

## Start here

Download release archives from the [GitHub Releases page](https://github.com/Monska85/hostlens/releases). Read the [installation guide](docs/v1/INSTALL.md) to install and configure a host, [operator instructions](docs/v1/OPERATIONS.md) for day-two reference, and [validation evidence](docs/v1/VALIDATION.md) for tested behavior and remaining limits. The MCP endpoint defaults to `http://127.0.0.1:8080/mcp`; bearer authentication remains required on loopback.

Choose diagnostic privilege deliberately:

- **Restricted:** Uses ordinary OS permissions and reports inaccessible sources.
- **Standard:** Gives only the backend `CAP_DAC_READ_SEARCH`. This enables broad host reads and requires the documented secret masks and service hardening.

Profiles constrain supported diagnostic requests, not a compromised privileged process. Allowed application files may contain secrets. Protect every network hop carrying bearer credentials, and treat returned log/configuration content as untrusted data.

## Develop and validate

Prepare the [base toolchain](docs/RELEASING.md#toolchain). Setup installs the repository's locked development tools locally.

```sh
make deps
make test-image
make check
make build
make package
make verify-archives
```

`make build` compiles to `bin/` using Go only. `make package` uses GoReleaser to create installable archives in `dist/archives/`. Acceptance verifies and executes those archive bytes; package again after changing source.

Prepare container images before running tests. See [local validation and matrix prerequisites](docs/RELEASING.md#local-validation) for the full workflow, cache overrides, systemd cases, and arm64 emulation. All executable tests run in disposable containers.

Use `make coverage` or `just coverage` for an inspectable HTML report; see [coverage scope](docs/RELEASING.md#coverage).

`make help` and `just --list` list equivalent commands; neither runner depends on the other.

## Architecture and requirements

[Product and architecture](docs/v1/SPEC.md) maps the implementation and future platform boundaries. [Current OpenSpec specifications](openspec/specs) are authoritative; [archived changes](openspec/changes/archive) preserve design rationale. Discover unfinished work with the system CLI: `openspec list`.

Changes must preserve separate authentication, source policy, and OS privilege boundaries. New platform support must implement native collectors, safe filesystem handling, credentials, IPC, and lifecycle adapters without changing Linux rule meanings.

## License

[Apache-2.0](LICENSE), copyright 2026 Monska85 and HostLens contributors. [NOTICE](NOTICE) records attribution. Executable archives include project and dependency license notices.
