# Install and upgrade HostLens

HostLens runs on Linux amd64 and arm64. The archive contains one `hostlens` executable, example configuration and profiles, three systemd units, the MCP tool contract, a release manifest, licenses, and these guides. The archive does not change a host by itself.

## Fresh system installation

1. Download `checksums.txt` and every asset named in it, then run `sha256sum --check --strict checksums.txt`. Extract the archive for your architecture and inspect `release.json` before copying files.
2. Create separate `hostlens-shared` and `hostlens-tokens` system groups, plus separate `hostlens-gateway` and `hostlens-observer` system users. Make `hostlens-shared` their primary group. The gateway unit receives `hostlens-tokens` as a supplementary group; the observer never receives it. The repair unit runs as root with a separate process and socket. Grant `hostlens-observer` membership in the local Docker access group only when Docker observation is enabled. Keep the gateway out of that group.
3. Install the executable at `/usr/local/bin/hostlens`, configuration at `/etc/hostlens/config.yaml`, and the example profiles under `/etc/hostlens/profiles`. Create `/etc/hostlens/secrets` owned by root and `hostlens-tokens`, mode `0750`; the token file is root owned, group `hostlens-tokens`, mode `0640`. Make the configuration and profiles owned by root and group `hostlens-shared`, with no group write permission.
4. Replace the numeric IDs in `config.yaml` with `id -u hostlens-gateway`, `id -u hostlens-observer`, `getent group hostlens-shared` for `shared_gid`, and `getent group hostlens-tokens` for `token_gid`. Keep `repair_uid: 0`. Run `hostlens config validate` before starting services. Leave `read_only: true` and `active_profiles: [host]` for the first start.
5. Install `systemd/hostlens-gateway.service` under `/etc/systemd/system/`, run `systemctl daemon-reload`, then `systemctl enable --now hostlens-gateway.service` so it starts again after reboot. Run `hostlens token create --name CLIENT --roles observe --expires never` as root. Store the returned secret in the client; it is shown once. The gateway listens on `127.0.0.1:8080` by default and requires `Authorization: Bearer SECRET` for MCP requests.

The example does not enable Docker. To observe Docker, set `docker_socket: /var/run/docker.sock`, activate `docker-readonly` in `active_profiles`, install and enable `hostlens-observer.service` with `systemctl enable --now`, and restart the gateway. The observer's systemd identity alone receives Docker group access. The gateway reaches only the observer's named, read-only IPC operations. The local Docker socket must be root owned and inaccessible to other users.

## Enable targeted repair

Repair is opt-in. Set `read_only: false`, add a profile with exact `restart_service` unit names or full `restart_container` IDs, and issue a separate token with the `repair` role. Install and start `hostlens-repair.service`, then restart the gateway. The worker independently rechecks the current token, profile grant, and live target before a restart. It accepts only the two named restart operations. A repair token can also carry `observe` if the client needs diagnostic calls.

To return to read-only mode, stop `hostlens-repair.service` and wait until it has exited, set `read_only: true`, validate the configuration, and restart the gateway. The stop drains or cancels admitted work before the worker exits. Keep the repair unit disabled when it is not needed.

## Upgrade an existing installation

Follow the [0.5.0 to 0.6.0 upgrade guide](../../UPGRADING.md).

## Remove a system installation

Preview removal with `hostlens uninstall`. The JSON output lists verified HostLens unit files and the installed binary, plus everything retained. The preview does not require root or change the host. Apply as root with `hostlens uninstall --apply`. Progress goes to stderr and the result is JSON on stdout. Run the command from the extracted archive if a previous attempt already removed `/usr/local/bin/hostlens`; repeating it is safe.

The command stops and disables the packaged HostLens units, removes their unit files, reloads systemd, then removes the verified binary. A reload failure restores the unit files and leaves the binary for retry. It does not traverse backup, FUSE, or other mounted filesystems. An altered unit, untrusted file owner or mode, or unrecognized binary blocks removal before a service is stopped. Restore the packaged unit or resolve the conflict, then retry.

Configuration, profiles, tokens, dedicated users and groups, shared journals, and manually granted permissions remain. Keep a protected copy of `/etc/hostlens` until credentials and profiles are no longer needed. Remove dedicated identities only after separately checking that no process or file on any relevant filesystem still uses their numeric IDs. This includes secondary and remote mounts; HostLens cannot prove that from a bounded uninstall. Remove administrator-granted permissions separately when no longer needed. The command never deletes these resources automatically.
