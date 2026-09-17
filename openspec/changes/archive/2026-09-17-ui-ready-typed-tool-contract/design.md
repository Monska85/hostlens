# Design: ui-ready-typed-tool-contract

## Context

See proposal.md, Why. Current state that shapes the approach:

- `internal/contract/contract.go` holds the exhaustive registry (`ToolDefinition` with a `Schema` kind enum), one shared `Args` union used for every tool and for IPC, and `Result{Data map[string]any}`.
- `internal/gateway/schema.go` hand-builds `inputSchema` per `Schema` kind and passes it to `mcp.AddTool`; the SDK infers `outputSchema` from `contract.Result`, which yields an untyped `data` object for all 27 tools.
- `internal/gateway/transport.go` already pre-parses each `tools/call` payload for role and read-only denial before the SDK handler runs, and `gateway.go:327` registers tools in a loop over the registry.
- `internal/backend/backend.go:242` decodes `contract.Request` and calls `Collector.Collect(ctx, tool, args)`; `Result.Bounded` enforces the response ceiling on the marshaled envelope plus text mirror.
- `internal/platform/linux/*.go` fills `r.Data["..."]` at about one hundred sites; `audit_linux.go` `validAuditArgs` re-validates cross-tool argument misuse; `dockerobs/projection.go` converts typed observer structs into `map[string]any`.
- `github.com/google/jsonschema-go` v0.4.3 is already in the module graph as the SDK's schema library.
- The gateway server already reports `name: hostlens` and the release version in `initialize`; the CLI already emits JSON for `install` plans and `token create`. Nothing else is needed for client detection or provisioning.

## Goals / Non-Goals

**Goals:**

- One Go definition per tool is the single source for: argument schema, result schema, gateway validation, backend decoding, collector types, and the published snapshot.
- Wire compatibility for MCP clients: identical tool names, argument names, and `data` member names.
- Net code reduction: remove the schema table, the argument union, the enum, the cross-tool guards, the map projections, and the post-hoc key-presence checks.

**Non-Goals:**

- No new tools, no REST endpoints, no CORS headers, no macOS or Windows collectors.
- No change to roles, policy, effect registry, admission, ceilings, audit records, or IPC transport and peer verification.
- No new distribution channel: the snapshot travels in the existing archives and as one extra asset on the existing GitHub release, nothing else.

## Decisions

**D1. Registry entries carry Go types; schemas are derived once at init.**
`contract.ToolDefinition` gains `In` and `Out` (`reflect.Type`) plus `InputSchema` and `OutputSchema` (`*jsonschema.Schema`), populated by a generic constructor `Define[In, Out any](ToolDefinition) ToolDefinition` that calls `jsonschema.For` for both types. `Schema`, its constants, and the enum branch in `ValidateRegistry` are deleted; `ValidateRegistry` instead requires non-nil schemas and rejects duplicate descriptions. Alternative rejected: keep `mcp.AddTool[In, Out]` generics per tool. That needs 27 static instantiations in the gateway, duplicating the registry, and forbids the loop-driven registration that role and capability filtering rely on.

**D2. Registration uses the SDK's low-level `Server.AddTool` with explicit schemas; the gateway validates both directions itself.**
`gateway.go` registers `&mcp.Tool{Name, Description, InputSchema, OutputSchema}` with a raw handler. Argument validation runs in the existing pre-parse step in `transport.go` (where denial already happens) using the resolved input schema, so `invalid_arguments` is returned before token re-verification cost and before IPC. Result validation runs in the handler after `Coordinator.Call` using the resolved output schema; a mismatch becomes `contract.Failure("response_shape")` with `isError`. Resolve each schema once at init (`Schema.Resolve`), not per request. Verify in the test container whether go-sdk v1.7.0's low-level `AddTool` also validates against `InputSchema`; if it does, keep the gateway's own check anyway because it must run before IPC and audit, and make the error text consistent.

**D3. Argument types are small shared structs, reused across tools with the same shape.**
`NoArgs`, `PageArgs{Offset, Limit}`, `PathArgs{Path}`, `UnitArgs{Unit}`, `PIDArgs{PID}`, `LogsArgs{Path, Unit, Format, RawTail, Since, Until, Priority *int, Limit}`, `ContainerArgs{Container}`, `DockerLogsArgs{Container, Since, Until, Limit}`. Constraints move to `jsonschema` struct tags (`minLength=1`, `minimum=0`, `minimum=1,maximum=4194304`, `minimum=0,maximum=7`, descriptions for `since`, `until`, `format`). One post-processing function in `contract` applies what tags cannot express: `additionalProperties: false` on every input schema and the `oneOf` path-or-unit rule on `LogsArgs`. Confirm the exact tag grammar of jsonschema-go v0.4.3 in the container before writing tags; if a constraint is not expressible by tag, add it in that single post-processing function, never in a second table.

**D4. `Result.Data` becomes `any`; collectors assign typed payload structs.**
The envelope keeps its fields and JSON names. Collectors set `r.Data = contract.OSInfo{...}`; JSON marshaling over IPC is unchanged; the gateway decodes `data` into `any` and validates it against the tool's output schema. The output schema is the envelope schema with the `data` property replaced by `jsonschema.For[Out]`; `data` stays optional so failure envelopes validate. Alternative rejected: a generic `Result[T]` envelope, which forces generic plumbing through backend, audit, and `Bounded` for no wire benefit.

**D5. Payload types live where their semantics live.**
Host, audit, and log payloads live in `contract` (portable, no Linux imports): `OSInfo`, `Inventory`, `HealthSnapshot{Severity, Coverage..., Checks map[string]HealthCheck}`, `ServicePage`, `ServiceStatus{Id, LoadState, ActiveState, SubState, UnitFileState, MainPID, Result}`, `PackagePage`, `LogPage` (entries or lines plus parser, ordering, window, rotation and coverage members), `ConfigFile`, `ProcessPage`, `ProcessInfo`, `NetworkInfo`, `AccountPage`, `StorageInfo`, `UpdateInfo`, `SecurityInfo`, `HostlensInfo`, `ServiceInspection`, `PathInspection`. Docker payloads live in `dockerobs`, replacing the map builders in `projection.go` with typed structs and constructors that keep today's JSON member names (`containers`, `running`, `negotiated_api`, and so on); `contract` imports `dockerobs` for registry `Out` types. Neither package imports the other today, so no cycle. Every JSON name equals the key emitted today; parity is proven by task 1.2 and the snapshot.

**D6. Optionality is expressed by Go, not by placeholders.**
Always-present members are plain fields and become `required`; members that may be unavailable are pointers or `omitempty` and are absent from `required`. Dynamic-key collections that are genuinely keyed by observed names (health checks, audit domain grants) are `map[string]T` and appear as `additionalProperties` of type `T`. Fields typed `any` are forbidden in payloads because they infer to an empty schema.

**D7. Backend decodes per tool with unknown-field rejection.**
`contract.Request.Args` becomes `json.RawMessage`. The backend looks up the definition, allocates `reflect.New(def.In)`, decodes with `DisallowUnknownFields`, and on failure returns `Failure("invalid_arguments")` without admitting a worker. `Collector.Collect(ctx, tool string, args any)` receives the decoded value; collectors type-assert. This makes `validAuditArgs` and every `a.X != ""` cross-tool guard redundant; they are deleted. Bounds that depend on configuration (page ceiling) stay in the collector because the schema cannot know the administrator's limit.

**D8. Pagination is generic.**
`page[T any](r *Result, items []T, a PageArgs, maxPage int) PageResult[T]`-style helper replaces the three `[]map[string]any` variants (`page`, `pagedProjects`, Docker inventories). Page payload types embed `Items []T` and `SnapshotConsistent bool` with today's JSON names.

**D9. Snapshot is a golden file owned by a test and shipped as a release artifact.**
`docs/v1/tools.json` is produced by a gateway test that builds a server for a diagnostics identity with all capabilities true and `read_only` true, calls `tools/list` through the HTTP handler, sorts by name, and compares to the file. `HOSTLENS_UPDATE_SNAPSHOT=1` rewrites it. Using the real handler, not the registry directly, proves discovery equals the snapshot. `docs/RELEASING.md` documents the regeneration command.

Distribution: `tools/release/prepare.py` copies the file unchanged into `dist/linux-<arch>/tools.json`; the GoReleaser `*.json` glob already archives it next to `release.json`, so `.goreleaser.yaml` needs no edit. The release workflow uploads `dist/linux-amd64/tools.json` once more as a standalone asset named `hostlens-<version>-tools.json` and includes it in `checksums.txt`, so `hostlens-ui` downloads one small verified file per pinned version instead of a full archive. The snapshot is architecture-independent by construction (the registry has no per-architecture branch); archive acceptance asserts the amd64 and arm64 copies are byte-identical. The file carries no version field: the release version identifies it, and adding one would make every release a snapshot diff.

**D10. Server facts for every authenticated client; nothing for anonymous ones.**
`get_hostlens_info` keeps its `hostlens` audit-domain policy grant but its `RequiredRole` drops from `diagnostics` to `health`, a one-field registry edit plus tests. A desktop client with a least-privilege token can then read version, mode, privilege, limits, and policy fingerprint after `initialize`. Alternative rejected: an unauthenticated server-information tool or HTTP response. Reasons: exact-version disclosure to any network peer is fingerprinting that maps hosts to CVEs; every anonymous feature moves JSON-RPC parsing, dispatch, and marshaling in front of authentication, where today only a header check and `401` run; `network-transport` requires bearer authentication on loopback and behind proxies, with `metrics.allow_anonymous` as the single explicit opt-in exception; loopback is not trust, since any local process could query. Local detection needs no network: the desktop client runs `hostlens version` and probes `/mcp` for `401`. Remote hosts are added with a token anyway, and `initialize` already returns the release version.

**D11. Descriptions are specific and unique.**
Each definition gets a one-sentence description naming what is observed and its notable exclusion or limit. Uniqueness is enforced in `ValidateRegistry` so the placeholder cannot return.

## Risks / Trade-offs

- [Silent member rename during the map-to-struct rewrite] → Task 1.2 captures every emitted key before deletion; the snapshot's `outputSchema.properties` must contain exactly those names per tool; any difference is a defect, not a documentation update.
- [Output validation turns a collector bug into a visible tool error] → Intended: matches "never fabricate" and "no silent partial success". Tests cover the failure path; the audit outcome records `response_shape`.
- [jsonschema-go tag grammar or SDK validation behavior differs from expectation] → Task 0.2 verifies both in the container before any code is written; D3 confines any gap to one post-processing function.
- [Double validation cost per call] → Schemas are resolved once at init; validating a bounded 128 KiB result is negligible against live collection. The existing gateway benchmark test must not regress beyond noise.
- [Coverage drop while deleting tests tied to removed types] → Coverage gate stays at the current 85.9 percent statement coverage; new tests target the new validation paths.
- [Public schema becomes a compatibility commitment] → Accepted; the snapshot makes every change explicit and reviewable, and the release version already identifies the contract.
- [Shipped snapshot diverges from the executed binary] → Archive acceptance calls `tools/list` on the installed candidate and compares it byte-for-byte (after canonical JSON ordering) with the `tools.json` extracted from the same archive; the standalone asset is checksummed with the archives.

## Migration Plan

Gateway, backend, and observer ship and upgrade together already; the IPC request shape change needs no compatibility shim. Rollback is a plain binary rollback through the existing upgrade manifest. MCP clients need no change. `hostlens-ui` pins a release and generates types from `docs/v1/tools.json` at that tag.

## Parity Report (task 7.2)

A scripted comparison (`parity` check during the task) walked all 27 tools in
`docs/v1/tools.json`, comparing `outputSchema.properties.data` member names,
required/optional classification, and one- and two-level element shapes against
`keys-before.txt`.

Defects found by the check and fixed in tasks 4–6:

- `get_os_info` / `get_inventory.os` `kernel` was an optional member; the legacy
  collector always emitted it on success, so it is now required.
- `get_docker_disk_usage` `reclaimable` was an optional pointer; the legacy
  accounting always emitted the analysis, so it is now a required value member
  (failure-path seeds carry the zero-value analysis).

Accepted differences (success-path wire unchanged; the schema models the union
of success and failure envelopes more honestly than the success-only capture):

- `get_process_info.process` and `inspect_service.service` are optional: every
  success emits them, and policy/selector/identity failure paths omit them
  exactly as the legacy maps did.
- `get_service_status` members (`Id` … `Result`) are optional: the legacy
  collector dropped empty systemd property values at runtime, so presence was
  always conditional; the oracle capture happened to observe all seven non-empty.
- `process.identity_rechecked` is required in the schema while the oracle marks
  it conditional: the legacy code set it in both branches whenever `process`
  was emitted, so the oracle's `?` was an over-conservative capture.

Known wire deltas recorded for release notes:

- The ten audit payloads now always declare `coverage_complete` (embedded
  `AuditCoverage`); legacy emitted it only on failure paths.
- Failure paths of audit and Docker tools that return the in-flight result emit
  schema-valid zero payloads instead of the legacy empty object; fresh
  `contract.Failure` results keep `data` absent, which the envelope accepts.

## Implementation Record

- jsonschema-go v0.4.3: struct tags carry only `description`; `omitempty` and
  pointers make members optional; structs get `additionalProperties:false`;
  `Schema.MarshalJSON` keeps `properties:{}`; every payload type keeps at least
  one field so an object schema is produced.
- The generic SDK `AddTool` path owns the whole result shape, so the design uses
  low-level `Server.AddTool` with explicitly resolved schemas to keep the
  envelope (`content` mirror + `structuredContent` + `isError`).
- Gateway benchmark (disposable container, task 0.1 baseline commit versus the
  typed registry): 369–421 µs/op → 252–265 µs/op, ~535 kB/op → ~415 kB/op,
  2795 → ~800 allocs/op. The typed registry is faster and allocates less.
- Audit collectors fill pre-allocated typed payloads in place and return an
  evidence flag; `collectAudit` turns evidence-free issue results into errors.
- Docker `pagedProjects` is generic over typed row slices and stores
  `*dockerobs.Page[T]`; empty inventories still serialize as `items: []`.

## Open Questions

None that change specs, approach, or tasks. Exact struct-tag spellings and whether the low-level SDK registration validates input are verified in task 0.2 and recorded in the task notes.
