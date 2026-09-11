## MODIFIED Requirements

### Requirement: Initial tool surface

HostLens SHALL expose get_os_info, get_inventory, get_health_snapshot, list_services, get_service_status, query_logs, list_packages, and read_config through a single MCP endpoint. It SHALL additionally expose diagnostics-role audit tools list_processes, get_process_info, get_network_info, list_accounts, get_storage_info, get_update_info, get_security_info, get_hostlens_info, inspect_service and inspect_path under explicit audit policy. Tool discovery SHALL reflect client roles and runtime capability availability. No generic shell or unrestricted read_file tool SHALL be exposed.

#### Scenario: Authorized discovery

- **WHEN** a health-only client lists tools
- **THEN** only its available health tools are listed

#### Scenario: Direct forbidden call

- **WHEN** a client calls an unlisted or unauthorized tool by name
- **THEN** the call is rejected independently of discovery
