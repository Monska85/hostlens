# HostLens configuration examples

These YAML examples describe the implemented v1 interface. Paths and names are illustrative; installation validates and protects actual configured locations. See [the complete default configuration](../../../../packaging/config.yaml) and [operator instructions](../../../../docs/v1/OPERATIONS.md).

## Main configuration

```yaml
server:
  bind:
    - "127.0.0.1"
    - "::1"
  port: 8080
  tls:
    enabled: false
  allow_insecure_http: false
  trusted_proxies: []
  client_ip_header: "X-Forwarded-For"

profile_dirs:
  - "/etc/hostlens/profiles"
  - "/etc/hostlens/custom-profiles"

profiles:
  - nginx

allow:
  files:
    - "/etc/myapp/public-settings.yaml"
  journal: []

deny:
  files:
    - "/etc/nginx/private/**"
    - "/etc/myapp/credentials.yaml"
  journal:
    - "sensitive-worker.service"

limits:
  tool_timeout: "10s"
  max_concurrent_operations: 4
  max_config_bytes: 65536
  max_log_entries: 200
  default_log_window: "15m"
  max_response_bytes: 131072

logging:
  level: "info"
  audit_successful_calls: true
```

Additional configurable ceilings, health-check settings, and administrative state paths will receive concrete schema definitions during implementation. Token records are separate from this file. Role membership belongs to token metadata, and source rules apply to all tokens on the host.

## Nginx profile

Save this as `nginx.yaml` in a configured directory. The included profile must exist even if it contains no rules.

```yaml
profiles:
  - web-common
allow:
  files:
    - "/etc/nginx/**"
    - "/var/log/nginx/*.log"
  journal:
    - "nginx.service"
deny:
  files:
    - "/etc/nginx/private/**"
  journal: []
```

An initial `web-common.yaml` can be an explicit empty base:

```yaml
profiles: []
allow:
  files: []
  journal: []
deny:
  files: []
  journal: []
```

## Optional broad file access

Ship `allow-all.yaml` without referencing it by default. This example broadens file access only; it does not enable all journal units, special-file reads, or unrestricted execution.

```yaml
profiles: []
allow:
  files:
    - "/**"
  journal: []
deny:
  files: []
  journal: []
```

Global denials, mandatory internal exclusions, OS restrictions, and operation limits still apply.

## Direct HTTPS

Replace the main `server` block with this block to expose both address families using native TLS:

```yaml
server:
  bind:
    - "0.0.0.0"
    - "::"
  port: 8443
  tls:
    enabled: true
    cert_file: "/etc/hostlens/tls/fullchain.pem"
    key_file: "/etc/hostlens/tls/private-key.pem"
  allow_insecure_http: false
  trusted_proxies: []
  client_ip_header: "X-Forwarded-For"
```

## Same-host reverse proxy

Keep the loopback HTTP listener and configure exact proxy addresses as trusted only when that proxy sanitizes or appends forwarding headers correctly:

```yaml
trusted_proxies:
  - "127.0.0.1/32"
  - "::1/128"
```

The proxy must forward the bearer Authorization header. A proxy on another machine requires a protected backend hop, or an explicit non-loopback plaintext opt-in with its risk understood.

## Future Windows profile

This is a future schema example, not supported Linux v1 configuration. `MyApp` and event ID `9001` are illustrative. Active use of `windows_events` must fail validation until supported.

```yaml
profiles:
  - windows-basic
allow:
  files:
    - "C:/ProgramData/MyApp/config/**"
    - "C:/ProgramData/MyApp/logs/*.log"
  windows_events:
    - channel: Application
      providers:
        - MyApp
deny:
  files:
    - "C:/ProgramData/MyApp/config/secrets/**"
  windows_events:
    - channel: Application
      providers:
        - MyApp
      event_ids:
        - 9001
```

Fields within an event selector match together; omitted event IDs mean all IDs for its selected channel and providers. Platform-specific selectors must retain their native meaning.
