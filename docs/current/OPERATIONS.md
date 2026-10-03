# Operate HostLens

`hostlens --help` lists commands and their flags. `hostlens config validate` checks the version 2 configuration and all active profiles. `hostlens status` collects a fresh local host observation and prints JSON. Token commands and `hostlens policy explain` require root. Token creation and rotation require an explicit `--expires` value: an RFC3339 time or `never`. Rotation gives the old token a finite retirement time through `--overlap-until`. Restart the gateway after changing configuration; it does not silently adopt a changed policy.

## First request

`hostlens status` works locally without a configuration file or bearer token. To use the MCP server, start the gateway and create an observe token as described in the [installation guide](INSTALL.md#fresh-system-installation). Point a local MCP client at `http://127.0.0.1:8080/mcp` and set its authorization header to `Bearer SECRET`.

From Bash on the same host, enter the token without echoing it or placing it in shell history. The first request lists tools allowed by the token and active profiles; the second calls `host_status`.

```bash
hostlens status
read -r -s -p 'Observe token: ' HOSTLENS_TOKEN
printf '\n'
curl -fsS --max-time 10 -X POST http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $HOSTLENS_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
curl -fsS --max-time 10 -X POST http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $HOSTLENS_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"host_status","arguments":{}}}'
unset HOSTLENS_TOKEN
```

The default listener accepts only local clients. Use a protected tunnel for a remote client, or configure a non-loopback listener with a TLS certificate and key. The gateway rejects non-loopback listeners without both TLS paths.

## MCP catalog

| Tool                     | Target                     | Effect                                                                  |
| ------------------------ | -------------------------- | ----------------------------------------------------------------------- |
| `host_status`            | `host`                     | Observe host resources and permitted failed services.                   |
| `list_services`          | Each returned service      | List up to 500 loaded services after profile filtering.                 |
| `service_status`         | Exact `.service` name      | Observe one systemd unit, its result, and restart count.                |
| `service_logs`           | Exact `.service` name      | Read up to 100 journal entries from the past 15 minutes.                |
| `docker_status`          | `engine`                   | Observe Docker Engine version and platform.                             |
| `list_docker_containers` | Each returned container ID | List up to 500 current containers after profile filtering.              |
| `container_status`       | Full 64-character ID       | Observe container state, health, restart count, and OOM status.         |
| `container_stats`        | Full 64-character ID       | Observe current CPU, memory, and PID counters.                          |
| `container_logs`         | Full 64-character ID       | Read up to 100 lines from the past 15 minutes with a 64 KiB text limit. |
| `restart_service`        | Exact `.service` name      | Restart a loaded systemd unit.                                          |
| `restart_container`      | Full stable container ID   | Restart one container.                                                  |

The SDK-derived [tool schema snapshot](tools.json) records exact inputs and outputs. A missing observation appears as an issue or an error; HostLens does not invent a value. Every request takes a new observation. The catalog has no arbitrary command, file read, process dump, or raw Docker API forwarding tool. Logs require a separate grant and can contain application secrets; grant them only where needed.

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

The packaged `services-readonly` and `docker-readonly` profiles enable discovery and status. `service-logs` and `docker-evidence` add log or stats access separately. Edit resources to exact service names or full container IDs when a client needs narrower visibility. To see why a rule applies, run `sudo /usr/local/bin/hostlens policy explain observe service_status nginx.service` or substitute another effect, tool, and resource. This command reads the active policy and collects no diagnostic evidence.

## Failure and limits

The gateway admits at most 16 concurrent HTTP requests, the Docker observer 8, and the repair worker 1. Requests have deadlines and bounded bodies. Docker responses are capped before decoding. Service inventories stream systemctl output and return at most 500 permitted rows; failed-service summaries return at most 50 permitted names. The systemctl child runs through util-linux `prlimit` with a 128 MiB address-space limit and a short deadline. When more permitted rows exist, the result reports `result_limit`. If the child cannot complete, the source is unavailable. Results do not claim completeness. Container inventories scan at most 500 source rows per request. The limit does not reveal how many denied resources exist. Each observation is request scoped. There is no retained diagnostic snapshot.

Systemd status requires a working system bus. Service logs also require `/usr/bin/journalctl` and operating-system permission for the gateway account to read the system journal. A profile grant does not add that permission; the default installation may return `access_limited`. Giving the gateway journal access is a separate administrator decision because the account could then read more logs outside HostLens. Docker status requires the private observer and a local root-owned socket. Container engines, systemd, and host kernel namespaces determine what is actually visible inside a container. A timeout or unavailable source is an error or explicit issue, never a healthy result. The gateway includes failed service names in `host_status` only when the same service is permitted by `service_status`. Failed-service summaries and inventories report fixed limits; service result limits apply after policy filtering, while Docker scans at most 500 containers before filtering. Query a known permitted unit or container by its exact name or full ID. Log results report `truncated` when a count or byte ceiling removes entries.

Restart results report `invoked` and `completed` separately. Either field is omitted when its value is unknown. If the restart completed but the follow-up read failed, the state field is absent and `issue` is `post_observation_unavailable`; inspect the target before retrying. A lost mutation response yields `restart_outcome_unknown`; inspect the target before retrying.

The repair worker writes one JSON audit event before invocation and one after it to its systemd journal. Events contain time, opaque request ID, token ID when verified, authorized tool and target, decision, and outcome. Denied requests omit attacker supplied tool and target. Audit events contain no bearer secret or diagnostic payload. If audit recording fails before invocation, the repair is refused; if it fails afterward, treat the outcome as unknown and inspect the target.

## Release procedure

Follow the repository's release engineering guide. A dated changelog section and examples are committed on the default branch before the tag. The tag workflow reruns validation, builds the candidate, and publishes the final release after its checks pass.
