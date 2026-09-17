# Mcp diagnostics

## Purpose

Define the observable HostLens v1 mcp diagnostics behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

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

### Requirement: Observed data only

Tool results SHALL contain observed data only. Unavailable optional fields SHALL be omitted without null, empty-string, zero, or unknown placeholders. Real observed zero values SHALL remain valid. Collection failures SHALL produce concise structured issues and preserve partial results. Source, host identity, and observation times SHALL be supplied where needed to interpret evidence.

#### Scenario: Rolling release

- **WHEN** Arch exposes no meaningful distribution version
- **THEN** get_os_info omits distribution version while retaining observed kernel and architecture fields

#### Scenario: Permission failure

- **WHEN** a required collector cannot read its source
- **THEN** the result contains an issue and does not imply successful empty output

#### Scenario: Alternative measurement

- **WHEN** a platform offers a differently defined metric
- **THEN** the metric uses its accurate name and source instead of a fabricated equivalent

### Requirement: Inventory and operating system information

get_os_info SHALL report observed OS family, distribution or product, version and codename when present, kernel version, and normalized architecture. get_inventory SHALL include consistent OS information plus available host and hardware information. These tools SHALL use consistent observations and naming.

#### Scenario: Focused OS query

- **WHEN** a client requests get_os_info
- **THEN** the response excludes unrelated package inventories and logs

#### Scenario: Consistent inventory

- **WHEN** both tools inspect the same unchanged host
- **THEN** overlapping facts agree

### Requirement: Services and packages

list_services and get_service_status SHALL retain native service states. list_packages SHALL identify installed package names and versions with their source. Service and package lists SHALL support bounded pagination without claiming snapshot consistency across mutations.

#### Scenario: Large inventory

- **WHEN** installed packages exceed a response page
- **THEN** a bounded page and continuation information are returned

#### Scenario: Unknown service

- **WHEN** the requested service does not exist
- **THEN** the result distinguishes absence from collection failure

### Requirement: Bounded file and journal queries

query_logs SHALL accept an explicit approved file or journal unit source and bounded time, count, and size filters. read_config SHALL read only policy-permitted regular files. Journal access SHALL enforce selected unit scope. Contents SHALL be treated as untrusted data, never executable instructions.

#### Scenario: File denied

- **WHEN** a denied configuration file is requested through query_logs
- **THEN** the backend rejects it using the same file policy

#### Scenario: Untrusted message

- **WHEN** a log entry contains instructions to reveal credentials
- **THEN** HostLens returns it only as bounded source data and never executes it

#### Scenario: Oversized configuration

- **WHEN** a file exceeds the configured read ceiling
- **THEN** the tool reports a size-limit issue without presenting a silently partial configuration

### Requirement: Configurable operational ceilings

The backend SHALL enforce configurable defaults of 10 seconds per call, four concurrent operations per host, 64 KiB per configuration read, 200 log entries, a default last-15-minutes log window, and 128 KiB per serialized tool response. Requested values SHALL NOT exceed administrator ceilings. Log-window default and maximum SHALL be distinct configurable concepts. Truncation, timeout, cancellation, and overload SHALL be explicit.

#### Scenario: Excessive request

- **WHEN** a client requests more than the configured ceiling
- **THEN** the request is rejected or explicitly bounded, never silently exceeds the ceiling

#### Scenario: Truncated logs

- **WHEN** matching logs exceed the result limit
- **THEN** the response identifies truncation and the returned window

#### Scenario: Concurrent clients

- **WHEN** multiple clients exceed the shared backend operation budget
- **THEN** excess work receives a bounded overload response rather than unbounded queuing

### Requirement: On-demand execution

V1 SHALL collect diagnostics only on request, using a configurable sampling interval where needed. It SHALL NOT store metric history, retain copies of inspected logs, schedule checks, call a model provider, or send incident notifications.

#### Scenario: Historical question

- **WHEN** a client asks for an interval no longer covered by source retention
- **THEN** available evidence and coverage gaps are reported without claiming no incident

#### Scenario: Idle host

- **WHEN** no diagnostic request is active
- **THEN** HostLens performs no periodic diagnostic collection

### Requirement: Honest file-log time filtering

File-log queries SHALL declare the supported parser and its timestamp, timezone, multiline, and ordering semantics. An unknown format, ambiguous timezone, or missing event timestamps SHALL yield an unsupported or unresolved time-window issue. File modification times SHALL NOT substitute for event timestamps. An explicitly requested bounded raw tail SHALL be labeled as lacking verified event-time coverage. Rotation scope and truncation SHALL be explicit.

#### Scenario: Unknown timestamps

- **WHEN** a time-filtered query targets a file whose event times cannot be parsed reliably
- **THEN** the tool reports unresolved window coverage rather than claiming the requested interval was checked

#### Scenario: Raw tail alternative

- **WHEN** a client explicitly requests a bounded tail of an allowed unstructured log
- **THEN** the result reports its file and ordering scope without inventing timestamps or complete historical coverage

### Requirement: Source-supported log severity

query_logs SHALL support severity filtering when the source provides a reliable native or explicitly parsed severity. The response SHALL preserve the source meaning or document a supported mapping. A severity request for an unsupported format SHALL produce an explicit unsupported-filter issue rather than infer severity from arbitrary message text.

#### Scenario: Journal priority

- **WHEN** a client requests a priority filter on an allowed journal unit
- **THEN** the query filters using the journal's recorded priority within all other configured bounds

#### Scenario: Unstructured file

- **WHEN** a severity filter is requested for a file with no supported severity parser
- **THEN** the tool reports the unsupported filter without inventing severity labels

### Requirement: MCP execution failure status

Fatal tool execution failures SHALL set MCP `isError` while retaining structured issues. Successful partial observations and historical coverage warnings SHALL NOT become fatal solely because they contain issues.

#### Scenario: Denied or unavailable execution

- **WHEN** policy denial, source read failure, timeout, overload, or backend failure prevents tool execution
- **THEN** the MCP response sets `isError` and retains the corresponding structured failure

#### Scenario: Useful partial observation

- **WHEN** a tool returns valid observations with incomplete coverage
- **THEN** the result preserves those observations and issues without setting `isError` solely for incomplete coverage

### Requirement: Bounded parser issue accumulation

JSONL parsing SHALL aggregate repeated parse failures into bounded summaries with counts and honor cancellation between records. Parsing and inspection gaps SHALL remain explicit without unbounded per-record issues.

#### Scenario: Malformed input at the inspection ceiling

- **WHEN** a file at the default inspection ceiling contains only malformed JSONL records
- **THEN** the result reports the failure count and unresolved coverage using bounded issue summaries

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

### Requirement: Fail-closed MCP tool effect classification

Every MCP tool SHALL have exactly one centrally defined effect classification: `diagnostic` or `remediation`. Tool discovery and direct dispatch SHALL use the same classification. An unclassified tool SHALL be unavailable and SHALL NOT reach a collector, backend, external integration, or mutation authority. In MCP read-only mode, only diagnostic tools SHALL be discoverable or executable. When read-only mode is disabled, role authorization, policy, capability availability, and remediation enablement SHALL still apply.

#### Scenario: Read-only discovery

- **WHEN** an authorized client lists tools while `mcp.read_only` is `true`
- **THEN** the response contains only available diagnostic tools permitted by its roles and policy

#### Scenario: Remembered remediation call

- **WHEN** a client directly calls a known remediation tool while `mcp.read_only` is `true`
- **THEN** the gateway rejects the call before contacting any backend or external integration

#### Scenario: Unclassified tool

- **WHEN** a tool is registered without an effect classification
- **THEN** startup or registration fails and the tool cannot be discovered or dispatched

#### Scenario: Read-only disabled

- **WHEN** `mcp.read_only` is `false`
- **THEN** a remediation tool remains unavailable unless every separate role, policy, capability, and remediation enablement check permits it

### Requirement: Stateless diagnostic evidence

Each diagnostic request SHALL fetch or calculate evidence from live permitted sources for that request. HostLens SHALL NOT retain diagnostic observations, inspected payloads, metric histories, host inventories, container inventories, cleanup candidates, or remediation plans for reuse after collection work completes. Request-scoped sampling and pagination state MAY remain in memory only while the admitted collection worker is active and SHALL be released when that worker exits. A client timeout or cancellation SHALL cancel the worker and make any still-active, bounded worker visible through operational admission metrics; an uninterruptible OS read MAY retain transient in-flight memory until it returns, but that evidence SHALL NOT be exposed to another request or copied into control-plane state. HostLens SHALL report source-retention gaps instead of substituting a retained HostLens copy.

#### Scenario: Repeated observation

- **WHEN** a source changes between two diagnostic requests
- **THEN** the second result reflects a new source observation rather than a cached result from the first request

#### Scenario: Request completes

- **WHEN** a diagnostic request returns or fails and its admitted collection worker exits
- **THEN** HostLens retains no inspected content, derived inventory, cleanup candidate, or diagnostic result for a later request

#### Scenario: Historical evidence unavailable

- **WHEN** the live source no longer retains evidence for a requested interval
- **THEN** HostLens reports the coverage gap and does not answer from a private history

#### Scenario: Cancelled sampling

- **WHEN** a diagnostic request is cancelled during sampling
- **THEN** cancellation is delivered within its bound, unfinished work remains admission-bounded and visible until it exits, and its request-scoped observations are not retained for reuse

### Requirement: Docker MCP integration

Available Docker tools SHALL participate in the same MCP effect registry, role checks, policy checks, capability discovery, argument validation, deadlines, concurrency limits, response ceilings, failure status, and payload-free audit path as other diagnostic tools. All Docker tools SHALL have the diagnostic effect. A failure or stall in Docker discovery or collection SHALL remain isolated from non-Docker tool discovery and execution.

#### Scenario: Read-only mode

- **WHEN** Docker diagnostics are enabled and `mcp.read_only` is `true`
- **THEN** authorized Docker diagnostic tools remain available because every exposed operation is read-only

#### Scenario: Docker observer stalls

- **WHEN** Docker capability discovery exceeds its deadline
- **THEN** Docker tools report unavailable without blocking non-Docker discovery, configuration activation, or unrelated requests

#### Scenario: Oversized Docker response

- **WHEN** a Docker inventory exceeds the configured response or pagination ceiling
- **THEN** HostLens returns a bounded page or explicit truncation without silently dropping policy or coverage information
