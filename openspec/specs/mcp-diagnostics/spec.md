# Mcp diagnostics

## Purpose

Define the observable HostLens v1 mcp diagnostics behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## Requirements

### Requirement: Initial tool surface

HostLens SHALL expose get_os_info, get_inventory, get_health_snapshot, list_services, get_service_status, query_logs, list_packages, and read_config through a single MCP endpoint. Tool discovery SHALL reflect client roles and runtime capability availability. No generic shell or unrestricted read_file tool SHALL be exposed.

#### Scenario: Authorized discovery

- **WHEN** a health-only client lists tools
- **THEN** only its available health tools are listed

#### Scenario: Direct forbidden call

- **WHEN** a client calls an unlisted or unauthorized tool by name
- **THEN** the call is rejected independently of discovery

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

Each MCP tool SHALL advertise only arguments that affect its behavior and reject unrelated arguments before collection. Source-reading tools SHALL require their documented source. Query logs SHALL require exactly one path or unit; list operations SHALL expose pagination without unrelated source filters.

#### Scenario: Observation without arguments

- **WHEN** a client discovers OS, inventory, or health tools
- **THEN** their schemas accept an empty object and reject file, journal, and pagination arguments

#### Scenario: Ambiguous log source

- **WHEN** a log request specifies both a path and a unit
- **THEN** schema validation rejects the request before collector execution

#### Scenario: Missing configuration source

- **WHEN** read_config lacks a nonempty path
- **THEN** the request fails validation before collector execution
