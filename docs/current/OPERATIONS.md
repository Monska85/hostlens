# Operate HostLens

`hostlens --help` lists commands and their flags. `hostlens config validate` checks the version 2 configuration and all active profiles. `hostlens status` collects a fresh local host observation and prints JSON. `hostlens token create`, `token list`, `token revoke`, and `token rotate` require root and use the configured token store. Rotation issues a new secret and gives the old token an explicit finite retirement time through `--overlap-until`. Restart a service after changing configuration; the gateway does not silently adopt a changed policy.

## MCP catalog

| Tool                     | Target                     | Effect                                                                             |
| ------------------------ | -------------------------- | ---------------------------------------------------------------------------------- |
| `host_status`            | `host`                     | Observe host identity, uptime, CPU count, load, memory, and root filesystem usage. |
| `service_status`         | Exact `.service` name      | Observe a systemd unit.                                                            |
| `docker_status`          | `engine`                   | Observe Docker Engine version and platform.                                        |
| `list_docker_containers` | Each returned container ID | List current containers after profile filtering.                                   |
| `container_status`       | Full 64-character ID       | Observe one container.                                                             |
| `restart_service`        | Exact `.service` name      | Restart a loaded systemd unit.                                                     |
| `restart_container`      | Full stable container ID   | Restart one container.                                                             |

The SDK-derived [tool schema snapshot](tools.json) records exact inputs and outputs. A missing observation appears as an issue or an error; HostLens does not invent a value. Every request takes a new observation. The catalog intentionally omits arbitrary commands, file reads, logs, process dumps, and raw Docker API forwarding.

## Profiles and roles

A profile is a YAML file named `NAME.yaml` under `profile_dir`. Activate its name in `active_profiles`. `includes` activates another named file; a cycle or unknown rule prevents startup. Each `allow` or `deny` rule names an `effect`, `tool`, and `resource`. Resources use Go `path.Match` patterns. Any matching denial overrides all grants. An inactive profile grants nothing. Use exact resource names for repair.

```yaml
allow:
  - effect: observe
    tool: service_status
    resource: nginx.service
  - effect: repair
    tool: restart_service
    resource: nginx.service
```

A bearer token also needs the matching `observe` or `repair` role. `read_only: true` hides and denies repair tools even if a profile and token allow them. Docker tools remain unavailable until an observer socket is configured. Restart tools require a configured repair socket; calls fail if the repair worker is stopped. A direct tool call receives the same authorization gate as discovery.

Active profiles apply to the whole installation. Two tokens with the same role receive the same profile grants; profiles do not isolate individual clients. Issue the `repair` role only to clients allowed to use every active repair grant.

## Failure and limits

The gateway admits at most 16 concurrent HTTP requests, the Docker observer 8, and the repair worker 1. Requests have deadlines and bounded bodies. Docker responses are capped before decoding. Container inventories inspect at most 500 source rows per request and report this fixed limit; they do not claim completeness. The limit does not reveal how many denied containers exist. Each observation is request scoped. There is no retained diagnostic snapshot.

Systemd status requires a working system bus. Docker status requires the private observer and a local root-owned socket. Container engines, systemd, and host kernel namespaces determine what is actually visible inside a container. A timeout or unavailable source is an error or explicit issue, not a healthy result. Use service logs from systemd to investigate process startup; avoid placing bearer tokens or observed payloads in logs.

Restart results report `invoked` and `completed` separately. Either field is omitted when its value is unknown. If the restart completed but the follow-up read failed, the state field is absent and `issue` is `post_observation_unavailable`; inspect the target before retrying. A lost mutation response yields `restart_outcome_unknown`; inspect the target before retrying.

## Release procedure

Follow the repository's release engineering guide. A dated changelog section and examples are committed on the default branch before the tag. The tag workflow reruns validation, builds the candidate, and publishes the final release after all gates pass.
