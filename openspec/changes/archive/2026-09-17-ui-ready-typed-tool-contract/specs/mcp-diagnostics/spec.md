## MODIFIED Requirements

### Requirement: Initial tool surface

HostLens SHALL expose get_os_info, get_inventory, get_health_snapshot, list_services, get_service_status, query_logs, list_packages, and read_config through a single MCP endpoint. It SHALL additionally expose diagnostics-role audit tools list_processes, get_process_info, get_network_info, list_accounts, get_storage_info, get_update_info, get_security_info, inspect_service and inspect_path under explicit audit policy, and the health-role audit tool get_hostlens_info under the same explicit audit policy. Tool discovery SHALL reflect client roles and runtime capability availability. No generic shell or unrestricted read_file tool SHALL be exposed. No tool, discovery response, or server-information response SHALL be served to an unauthenticated request.

#### Scenario: Authorized discovery

- **WHEN** a health-only client lists tools
- **THEN** only its available health tools are listed

#### Scenario: Direct forbidden call

- **WHEN** a client calls an unlisted or unauthorized tool by name
- **THEN** the call is rejected independently of discovery

#### Scenario: Health client reads server facts

- **WHEN** a health-only client with an active `hostlens` audit grant lists tools and calls get_hostlens_info
- **THEN** get_hostlens_info is listed and returns the server facts, while every other audit tool stays unlisted and rejected

#### Scenario: Anonymous server information

- **WHEN** a request without a valid bearer token asks for tool discovery, server information, or any tool
- **THEN** the gateway answers with authentication failure and discloses neither version nor tool names

### Requirement: Tool-specific argument contracts

Each MCP tool SHALL advertise only arguments that affect its behavior and reject unrelated arguments before collection. Source-reading tools SHALL require their documented source. Query logs SHALL require exactly one path or unit; list operations SHALL expose pagination without unrelated source filters. The advertised argument schema and the argument validation applied to direct calls SHALL derive from the same single tool definition, so that no argument is accepted that discovery does not advertise and no advertised argument is rejected when it satisfies its published constraints. Argument rejection SHALL return an MCP error with a structured `invalid_arguments` issue and SHALL occur before any backend, collector, or external integration is contacted.

#### Scenario: Observation without arguments

- **WHEN** a client discovers OS, inventory, or health tools
- **THEN** their schemas accept an empty object and reject file, journal, and pagination arguments

#### Scenario: Ambiguous log source

- **WHEN** a log request specifies both a path and a unit
- **THEN** schema validation rejects the request before collector execution

#### Scenario: Missing configuration source

- **WHEN** read_config lacks a nonempty path
- **THEN** the request fails validation before collector execution

#### Scenario: Unknown argument

- **WHEN** a client calls any tool with an argument name absent from that tool's advertised schema
- **THEN** the call fails with `isError` and an `invalid_arguments` issue, and no collection work is admitted

#### Scenario: Out-of-range argument

- **WHEN** a client supplies an argument outside the constraints published in the advertised schema, such as a zero PID or a negative offset
- **THEN** the call fails with `invalid_arguments` before collector execution, and the same request with an in-range value is accepted

## ADDED Requirements

### Requirement: Typed result contracts

Every MCP tool SHALL advertise an `outputSchema` in discovery that describes its complete structured result: the common envelope (`observed_at`, optional `host`, `source`, `issues`, `truncated`, `next_offset`, `error`) and a tool-specific `data` member whose properties are enumerated with their types. The result schema and the result validation applied before a response leaves the gateway SHALL derive from the same single tool definition that provides the argument schema. A structured result that does not conform to the advertised schema SHALL NOT be returned as an observation; the gateway SHALL return an MCP error with a `response_shape` issue and no `data`. Optional observations SHALL be omitted from both the result and the schema's required list rather than represented by placeholders. Each tool's discovery description SHALL state what that tool observes; tools SHALL NOT share a generic description.

#### Scenario: Discovery includes result schema

- **WHEN** an authorized client lists tools
- **THEN** every listed tool carries an `outputSchema` whose `data` member enumerates that tool's properties, and no two tools share the same description text

#### Scenario: Conforming result

- **WHEN** a collector returns an observation whose members match the tool's typed contract
- **THEN** the MCP response `structuredContent` validates against the advertised `outputSchema` and the `content` text mirror contains the same JSON

#### Scenario: Nonconforming result

- **WHEN** the backend returns a `data` member that violates the advertised `outputSchema`
- **THEN** the gateway sets `isError`, returns a `response_shape` issue, omits `data`, and records the outcome in the payload-free audit record

#### Scenario: Omitted optional observation

- **WHEN** an optional measurement is unavailable on the inspected host
- **THEN** its member is absent from `data`, the result still validates, and the schema lists that member as optional rather than required

#### Scenario: Failure envelope

- **WHEN** collection fails before producing any observation
- **THEN** the failure result carries `error`, `observed_at`, and `issues` without `data`, and still validates against the tool's `outputSchema`
