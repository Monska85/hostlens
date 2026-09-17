# Installing HostLens

This guide walks an administrator through installing HostLens on a Linux target host, starting the services, issuing a first token, and optionally enabling Docker diagnostics. Each step runs on the target host unless noted. For reference material on tools, policy, and limits, read [Operating HostLens](OPERATIONS.md).

## What installs

The installer places three executables in `/usr/local/bin`, configuration and profiles under `/etc/hostlens`, and the installation manifest in `/var/lib/hostlens/install.json`:

- **`hostlens`** is the MCP gateway. It terminates HTTP, verifies bearer tokens, and dispatches tool calls.
- **`hostlens-diagnostics`** is the diagnostic backend. It enforces source policy and collects host evidence.
- **`hostlens-docker-observer`** is the isolated Docker observer. Installation places the binary, but it holds no Docker authority until you enable and reconcile the integration.

Two system identities, `hostlens-gateway` and `hostlens-diagnostics`, are created with no login shell. The installer never adds a persistent Docker group membership.

## Prerequisites

- Linux amd64 or arm64 with systemd and a kernel that provides `openat2` (5.6 or newer).
- Root access on the target host.
- A release archive `hostlens-<version>-linux-<arch>.tar.gz` with its `checksums.txt`, downloaded from the [GitHub Releases page](https://github.com/Monska85/hostlens/releases). The checksum manifest verifies integrity only; verify the build's origin with the release provenance attestations (`gh attestation verify`, see below) and any publisher signatures offered through your distribution channel.
- Rootful Docker Engine installed and running, only if you plan to enable Docker diagnostics. Rootless Docker, remote engines, Docker Desktop, Swarm administration, Podman, and Kubernetes remain unsupported. Verify the daemon socket:

```sh
ls -l /var/run/docker.sock
docker version --format '{{.Server.APIVersion}}'
```

The API version must be 1.41 or newer; HostLens negotiates down from newer versions to its tested ceiling of 1.51.

## Verify and extract the archive

Run this on a trusted workstation or the target host with the archive and `checksums.txt` in the working directory:

```sh
sha256sum --ignore-missing -c checksums.txt
mkdir -p /opt/hostlens-release
tar -xzf hostlens-0.1.0-linux-amd64.tar.gz -C /opt/hostlens-release
ls /opt/hostlens-release
```

The extracted set contains the `hostlens`, `hostlens-diagnostics`, and `hostlens-docker-observer` executables, `release.json` (the per-member upgrade manifest), `tools.json` (the pinned MCP tool contract snapshot for UI clients), `config.example.yaml`, `nginx.yaml`, `allow-all.yaml`, and `docker-readonly.yaml` under `profiles/`, `OPERATIONS.md`, `INSTALL.md`, `VALIDATION.md`, `LICENSE`, `NOTICE.txt`, and the upstream `licenses/` directory.

Every release asset also carries a GitHub build provenance attestation recorded by the release workflow once the repository is public; while it is private, the workflow reports attestations as explicitly unavailable and delivery remains checksum-verified. From a repository checkout you can verify that the exact archive bytes were built by this repository's workflow:

```sh
gh attestation verify hostlens-<version>-linux-<arch>.tar.gz -R Monska85/hostlens
```

## Plan and install

Print the plan first. The plan lists every directory, identity, file, and service unit it will create, and refuses pre-existing conflicts:

```sh
/opt/hostlens-release/hostlens install --source /opt/hostlens-release --privilege standard
```

Review the plan, then apply. `--start` also starts both services:

```sh
/opt/hostlens-release/hostlens install --source /opt/hostlens-release --privilege standard --apply --start
```

Choose the privilege mode deliberately:

- **standard** grants only `CAP_DAC_READ_SEARCH` to the backend and enables broad host reads. The service hardening, secret masks, and InaccessiblePaths units are generated for you.
- **restricted** grants no additional capability and reports inaccessible sources honestly.

Verify the services:

```sh
systemctl is-active hostlens-diagnostics.service hostlens-gateway.service
/usr/local/bin/hostlens status --system
```

## Issue a first token

Roles are fixed: `health`, `inspect`, `diagnostics` (each includes the previous), and `metrics` (independent). Issue a diagnostics token and keep the secret from the output. Prefer finite expiries; for credentials that must outlive rotation cycles, `--expires never` creates a non-expiring token that stays valid until revoked or rotated:

```sh
/usr/local/bin/hostlens token create --system \
  --name first-client --roles diagnostics \
  --expires "$(date -u -d '+30 days' +%Y-%m-%dT%H:%M:%SZ)"

/usr/local/bin/hostlens token create --system \
  --name service-account --roles health --expires never
```

The MCP endpoint defaults to `http://127.0.0.1:8080/mcp`. Bearer authentication is required on loopback. Call a first tool to confirm the full stack:

```sh
curl -sS -X POST http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $SECRET" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "MCP-Protocol-Version: 2025-06-18" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}'
```

## Configure sources

Edit `/etc/hostlens/config.yaml`. Profile files live in `/etc/hostlens/profiles/`; activate one by listing it under `profiles: []`. Example:

```yaml
profiles: [web]
```

```sh
cat >/etc/hostlens/profiles/web.yaml <<'YAML'
profiles: []
allow:
  files: [/etc/nginx/nginx.conf, /var/log/nginx/*.log]
  journal: [nginx.service]
deny:
  files: [/etc/nginx/private/**]
  journal: []
YAML
chmod 0644 /etc/hostlens/profiles/web.yaml
/usr/local/bin/hostlens config validate --system
hostlens reload --system
```

Policy changes reload without a restart. `hostlens policy explain --system PATH` explains every matching rule for a path; `docker:container/web` style targets explain Docker grants.

## Enable Docker diagnostics

Docker diagnostics are disabled by default and carry no authority until you enable and reconcile them. The observer's access to the rootful Docker socket is root-equivalent; the plan discloses this before any mutation.

Append or edit the docker section in `/etc/hostlens/config.yaml`:

```yaml
docker:
  enabled: true
  daemon_socket: /var/run/docker.sock
  observer_socket: /run/hostlens/docker-observer.sock
  group: docker
```

`group` must be the Unix group that owns the Docker socket on this host (usually `docker`); change it for nonstandard installations. Validate and preview the plan without mutation:

```sh
hostlens config validate --system
hostlens reconcile --system
```

Review the `disclosure` field and the `changed` list in the plan output, then apply and restart the services so the running processes adopt the new topology:

```sh
hostlens reconcile --system --apply
systemctl restart hostlens-diagnostics.service hostlens-gateway.service
```

Verify the observer topology:

```sh
systemctl status hostlens-docker-observer.socket
hostlens status --system
```

Grant Docker resources through a profile. The bundled `docker-readonly.yaml` shows the grant forms; copy the lines you want into an active profile:

```yaml
profiles: [docker-readonly]
```

```sh
hostlens reload --system
```

Then confirm availability through MCP with a diagnostics token:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": { "name": "get_docker_info", "arguments": {} }
}
```

A result with `"available": true` and the negotiated Engine API confirms the integration. Docker tools absent from discovery mean the observer is unreachable, Docker is stopped, or no `docker` grant is active. Direct calls to unavailable Docker tools fail closed with explicit issue codes before the observer is contacted.

## Reload and restart

`mcp.read_only`, profiles, and policy changes reload with `hostlens reload --system`. Listener, TLS material, identity, credential location, privilege, Docker observer settings, and the connection idle timeout require a restart of both services; a reload over such a change fails without applying anything.

## Upgrade

```sh
hostlens upgrade --archive hostlens-0.1.0-linux-amd64.tar.gz --apply
```

The upgrade verifies every archive member, retains the previous executables, preserves configuration, tokens, and active profiles, and restarts only previously active services. Changed bundled profiles are saved as `.candidate` files for review. Upgrading from a release that predates the Docker observer does not install the new binary (the running release performs the upgrade). After enabling the section, restore the binary from the extracted release once:

```sh
hostlens reconcile --system --apply --source /opt/hostlens-release
```

## Disable Docker diagnostics and uninstall

Remove the `docker:` section from `/etc/hostlens/config.yaml`, restart both services, and reconcile:

```sh
hostlens reload --system
systemctl restart hostlens-diagnostics.service hostlens-gateway.service
hostlens reconcile --system --apply
```

Disablement stops the observer, removes its owned units, socket, and binary, and leaves the dormant `hostlens-observer` account without Docker group authority. Full removal:

```sh
hostlens uninstall --apply
```

Uninstall removes tracked resources and created identities, preserves pre-existing Docker resources and unrelated files, and reports conflicts for administrator review. Downgrading to a build without the `docker:` section requires removing the section first: older binaries reject it as unknown.

## Troubleshooting

Structured service logs carry every outcome:

```sh
journalctl -u hostlens-gateway.service -u hostlens-diagnostics.service
```

- **`docker_disabled`:** the section is missing or `enabled` is false.
- **`docker_unavailable`:** Docker is stopped or absent, the observer is not provisioned, or the socket is missing. Check `systemctl status hostlens-docker-observer.socket`, the configured `daemon_socket` and `group`, and rerun reconciliation after any change.
- **`unsupported_engine`:** the daemon reports rootless, Docker Desktop, or another unsupported mode.
- **`driver_gap`:** the container's log driver cannot serve records; this never establishes that no logs exist.
- **`non_atomic_observation`:** Docker changed during accounting; reclaimable estimates stay advisory.

See [Operating HostLens](OPERATIONS.md) for tool contracts, policy reference, limits, and the full troubleshooting guidance.
