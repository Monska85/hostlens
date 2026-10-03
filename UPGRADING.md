# Upgrade HostLens from 0.5.0 to 0.6.0

The MCP catalog and configuration format change. Do not replace the running 0.5.0 binaries in place. First extract the 0.6.0 archive into a separate directory and preserve the old binaries, units, configuration, profiles, and token store. Run the new archive's `./hostlens migrate`; the installed 0.5.0 binary does not have this command.

As root, prepare a candidate with the actual service account IDs:

```sh
./hostlens migrate --from /etc/hostlens/config.yaml --output /root/hostlens-candidate --gateway-uid GATEWAY_UID --observer-uid OBSERVER_UID --repair-uid 0 --shared-gid SHARED_GID --token-gid TOKEN_GID
```

The command reads the old configuration, active profile graph, and token store, then writes a configuration schema version 2 candidate and a migration report in a new private directory. The report lists included profiles as well as directly activated profiles. It preserves active bearer secrets through a marked legacy hash format and maps old diagnostic roles only to `observe`; metrics-only tokens stay revoked. It does **not** map old file, journal, audit, Docker, or profile grants to new tools. Review the report, create exact new grants, change `token_store` and `profile_dir` from candidate paths to their final protected locations, and install the migrated token file with the `hostlens-tokens` group and mode `0640` before activation. A configuration with multiple listeners or unsupported source data is rejected without publishing a candidate. Existing clients must change their tool calls to the [new catalog](docs/current/OPERATIONS.md).

### MCP client tool changes

| 0.5.0 tool                                                                | New tool                 | Scope change                                                                                      |
| ------------------------------------------------------------------------- | ------------------------ | ------------------------------------------------------------------------------------------------- |
| `get_docker_info`                                                         | `docker_status`          | Engine version and platform only.                                                                 |
| `get_docker_container`                                                    | `container_status`       | State metadata only; supply the full 64-character ID.                                             |
| `list_docker_containers`                                                  | `list_docker_containers` | Current bounded inventory, filtered by active profiles.                                           |
| `get_service_status`, `inspect_service`                                   | `service_status`         | One exact systemd unit and its current state only.                                                |
| `get_health_snapshot`, `get_inventory`, `get_os_info`, `get_storage_info` | `host_status`            | Host identity and basic resource status only; no equivalent health assessment or broad inventory. |

The other 0.5.0 MCP tools have no 0.6.0 equivalent: `get_docker_container_stats`, `get_docker_disk_usage`, `get_hostlens_info`, `get_network_info`, `get_process_info`, `get_security_info`, `get_update_info`, `inspect_path`, `list_accounts`, `list_docker_images`, `list_docker_networks`, `list_docker_volumes`, `list_packages`, `list_processes`, `list_services`, `query_docker_logs`, `query_logs`, and `read_config`. The new `restart_service` and `restart_container` tools require explicit repair authority and have no 0.5.0 counterpart. Clients must update both tool names and argument/result handling; the mapping does not preserve old payloads.

After review, stop and disable the old gateway, diagnostics, and Docker observer units, then wait for them to exit. Remove only the old HostLens unit files and observer socket after verifying their paths and contents against the saved installation. Do not run the old `uninstall --apply` as an upgrade step: its ownership scan can stall on large mounted trees, and it can remove the installed binary before finishing. Preserve old state and service identities for rollback.

Install the new binary, configuration, profiles, and units, validate the candidate, then start the new gateway and optional observer. Start repair only after explicitly enabling it. Check MCP discovery and allowed calls with an observe token before removing the rollback copy. If activation fails, stop the new units and restore the preserved 0.5.0 binary, state, and units together. Do not feed the schema version 2 token store or configuration to a 0.5.0 process.

After 0.6.0 is active and rollback is no longer needed, use `hostlens uninstall` only when removing the new installation. It does not delete preserved configuration or old service identities. Follow the [removal guide](docs/current/INSTALL.md#remove-a-system-installation) for their separate review.

The repository tests installation and lifecycle behavior only inside disposable containers. Operators should rehearse this breaking migration on a clone before production activation.
