# Install and upgrade HostLens

HostLens runs on Linux amd64 and arm64. The archive contains the executable, configuration profiles, systemd units, MCP contract, checksummed member manifest, licenses, and these guides. The archive does not change a host by itself.

Commands that edit `/etc/hostlens`, manage systemd, create tokens, or apply removal require root. Upgrade preview also requires root because it checks protected installed files. The examples below use bare `hostlens`, `sed`, and `systemctl` for those actions; run those blocks in a root shell (`sudo -i`), or prefix each privileged command with `sudo`. Run archive download and fresh installation preview as your normal user.

## Fresh system installation

Run these commands on the target Linux host. The installer uses the host's `groupadd`, `useradd`, and `systemctl` tools. Service inventory collection also needs util-linux `prlimit` at `/usr/bin/prlimit` to bound the systemctl child; without it, those inventory requests report the source as unavailable. Run commands from the extracted release directory.

1. Download `checksums.txt`, both architecture archives, and `hostlens-0.6.0-tools.json` from the [0.6.0 release](https://github.com/Monska85/hostlens/releases/tag/v0.6.0). All files named in `checksums.txt` must be present for verification. On x86-64 select amd64; on AArch64 select arm64. From the download directory, run:

   ```sh
   sha256sum --check --strict checksums.txt
   mkdir hostlens-0.6.0
   tar -xzf hostlens-0.6.0-linux-amd64.tar.gz -C hostlens-0.6.0
   cd hostlens-0.6.0
   ./hostlens version
   ```

   Replace `amd64` with `arm64` on an AArch64 host. `sha256sum` verifies the downloaded archive against the release checksums; the installer verifies each extracted member against `release.json` and rejects the wrong architecture.

2. Preview installation. Use `--start` in both commands if you want the read-only gateway started after validation. Copy the `plan_digest` from the JSON preview into the apply command. Preview changes no files, accounts, or services; apply requires root and refuses a changed plan.

   ```sh
   ./hostlens install --source . --start
   sudo ./hostlens install --source . --start --apply --plan PLAN_DIGEST
   ```

   The installer creates separate gateway and observer identities and token and IPC groups, then installs protected files. Existing administrator configuration, profiles, and tokens are retained. It records the packaged unit checksums in root-owned `/etc/hostlens/unit-provenance.json` so a later upgrade can distinguish an unchanged previous unit from an administrator edit. It installs the three unit files but starts only the read-only gateway when `--start` is present. A retained configuration with `read_only: false` blocks `--start`; review it before starting repair separately. Check `systemctl is-active hostlens-gateway.service` before issuing a token. An installation without `--start` leaves all services stopped.

3. Create a client token as root. Choose a finite RFC3339 expiry or write `--expires never` explicitly. The secret appears only once; save it in the MCP client. The default endpoint is `http://127.0.0.1:8080/mcp`.

   ```sh
   sudo /usr/local/bin/hostlens token create --name first-client --roles observe --expires "$(date -u -d '+30 days' '+%Y-%m-%dT%H:%M:%SZ')"
   ```

Follow the [first request](OPERATIONS.md#first-request) to check discovery and call a tool.

### Add Docker observation

The default installation does not connect to Docker. The packaged observer unit requires a local socket owned by root and group `docker`, with no access for other users. Check the socket before proceeding; the unit grants that group only to the observer process. The gateway reaches Docker through named observation operations on its private socket.

```sh
stat -Lc '%U:%G %a' /var/run/docker.sock
sed -i \
  -e 's|^docker_socket: ""$|docker_socket: /var/run/docker.sock|' \
  -e 's/^active_profiles: \[host\]$/active_profiles: [host, docker-readonly]/' \
  /etc/hostlens/config.yaml
hostlens config validate
systemctl daemon-reload
systemctl enable --now hostlens-observer.service
systemctl restart hostlens-gateway.service
```

If the Docker socket uses another group, the packaged observer unit cannot access it as written. Review the unit and uninstall behavior before adapting it: uninstall refuses modified packaged units.

## Enable targeted repair

Repair is opt-in. Replace the placeholder targets in `/etc/hostlens/profiles/repair-example.yaml` with exact local `.service` names or full 64-character container IDs. In `/etc/hostlens/config.yaml`, set `read_only: false` and add `repair-example` to `active_profiles`. Keep any observation profiles you still need. Validate before starting the worker. The worker rechecks the token, grant, and live target. It accepts only the two named restart operations.

```sh
hostlens config validate
systemctl daemon-reload
systemctl enable --now hostlens-repair.service
systemctl restart hostlens-gateway.service
hostlens token create --name repair-client --roles repair --expires "$(date -u -d '+30 days' '+%Y-%m-%dT%H:%M:%SZ')"
```

A repair token can also carry `observe` if the client needs diagnostic calls. Active profiles apply to every token with the same role; see [profile and role rules](OPERATIONS.md#profiles-and-roles) before granting repair.

To return to read-only mode, stop and disable the worker first. Wait for it to exit. Then set `read_only: true` and remove `repair-example` from `active_profiles` in `/etc/hostlens/config.yaml`; keep any observation profiles. Validate and restart the gateway. Stopping the worker drains or cancels admitted work before it exits. A read-only gateway refuses to start while the installed repair unit is active or its configured socket remains; wait for the worker to exit and remove the socket. If a crash leaves a stale socket, inspect the worker and remove that stale socket before retrying.

```sh
systemctl disable --now hostlens-repair.service
hostlens config validate
systemctl restart hostlens-gateway.service
```

## Upgrade an existing installation

For an installed 0.6.0 system, verify and extract the new release as in step 1, then preview and apply from the extracted directory:

```sh
sudo ./hostlens upgrade --source .
sudo ./hostlens upgrade --source . --apply --plan PLAN_DIGEST
```

Upgrade preserves configuration, profiles, credentials, and dedicated identities. It resumes only units active before replacement. If validation or startup fails, it attempts to restore the previous executable and units and restart those units; any failed restore is reported with the protected rollback copy's path. Modified packaged unit files block the upgrade for administrator review. On an older 0.6.0 installation without unit provenance, the candidate must have identical unit files; apply the reissued 0.6.0 once to establish provenance before moving to a future release that changes units. For the 0.5.0 to 0.6.0 configuration transition, follow [UPGRADING.md](../../UPGRADING.md).

## Remove a system installation

Preview removal with `hostlens uninstall`. The JSON output lists verified HostLens unit files and the installed binary, plus everything retained. The preview does not require root or change the host. Apply as root with `hostlens uninstall --apply`. Progress goes to stderr and the result is JSON on stdout. Run `./hostlens uninstall --apply` from the extracted archive if a previous attempt already removed `/usr/local/bin/hostlens`; repeating it is safe.

```sh
hostlens uninstall
hostlens uninstall --apply
```

The command stops and disables the packaged HostLens units, removes their unit files, reloads systemd, then removes the verified binary. A reload failure restores the unit files and leaves the binary for retry. It does not traverse backup, FUSE, or other mounted filesystems. An altered unit, untrusted file owner or mode, or unrecognized binary blocks removal before a service is stopped. Restore the packaged unit or resolve the conflict, then retry.

Configuration, profiles, tokens, dedicated users and groups, shared journals, and manually granted permissions remain. Keep a protected copy of `/etc/hostlens` until credentials and profiles are no longer needed. Remove dedicated identities only after separately checking that no process or file on any relevant filesystem still uses their numeric IDs. This includes secondary and remote mounts; HostLens cannot prove that from a bounded uninstall. Remove administrator-granted permissions separately when no longer needed. The command never deletes these resources automatically.
