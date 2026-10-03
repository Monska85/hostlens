# HostLens

[![CI](https://github.com/Monska85/hostlens/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/Monska85/hostlens/actions/workflows/ci.yml)
[![Release](https://github.com/Monska85/hostlens/actions/workflows/release.yml/badge.svg)](https://github.com/Monska85/hostlens/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

HostLens is a small MCP server for live Linux host, systemd service, and local Docker status. It starts in read-only mode. An administrator can enable two narrowly scoped repairs: restart an exact systemd service or a Docker container identified by its full ID.

The gateway authenticates bearer tokens and enforces active profile grants for both tool discovery and calls. A separate Docker observer exposes only named reads from the mixed Docker API. A separate repair worker checks role, profile, and live target again before a restart. There is no generic shell, filesystem read, log, or Docker API tool. Observations are request scoped and report unavailable sources explicitly.

Linux amd64 and arm64 are supported now. The shared CLI, gateway, authorization, and MCP packages build on macOS and Windows; those platforms need native status, identity, and repair adapters before they can run HostLens.

## Start

Build a development candidate with `make package`, or use the 0.6.0 release archive. Its configuration schema is version 2, separate from the product release number. Download every asset listed in `checksums.txt` before checking it, then follow the [installation guide](docs/current/INSTALL.md) to create the dedicated service identities, install the single binary and units, and issue an observe token. Existing 0.5.0 installations require the [upgrade guide](UPGRADING.md); the MCP catalog and configuration change.

```sh
hostlens --help
hostlens config validate --config /etc/hostlens/config.yaml
hostlens token create --name first-client --roles observe --expires never
```

The default endpoint is `http://127.0.0.1:8080/mcp` with bearer authentication. The [operations guide](docs/current/OPERATIONS.md) lists all seven tools, profile rules, limits, and the steps for enabling repair. [Generated tool schemas](docs/current/tools.json) are checked against the SDK registry during validation. `hostlens uninstall` previews safe removal; `hostlens uninstall --apply` stops the packaged services and removes verified HostLens files while retaining administrator data and service identities.

## Develop

Use the repository's `Makefile` or lowercase `justfile`; the common targets call the same scripts or tools independently. Tests, systemd acceptance, and private Docker fixtures run in disposable containers. No validation target installs HostLens on the development host or changes its systemd units.

```sh
make deps
make test-image
make test
make package
make verify-archives
```

See [validation](docs/current/VALIDATION.md), [release engineering](docs/RELEASING.md), and the [current OpenSpec requirements](openspec/specs). The tag workflow publishes a final release only after its checks pass.

## License

[Apache-2.0](LICENSE). [NOTICE](NOTICE) and the release archives record attribution and dependency licenses.
