# Proposal: ui-ready-typed-tool-contract

## Why

A desktop client (`hostlens-ui`) will consume the existing MCP endpoint directly instead of a dedicated REST API. Today discovery advertises a precise `inputSchema` but an `outputSchema` whose `data` member is an untyped object, because every collector fills `map[string]any` with string-literal keys at roughly one hundred call sites. A client has no contract for what a tool returns, a server rename breaks it silently, and the same argument constraints are spelled three times (hand-written schema table, shared `Args` union, per-collector guards). One typed definition per tool must drive the argument schema, the result schema, IPC decoding, collector code, and a published snapshot, so that the client generates its types from the server and drift fails a test instead of a user.

## What Changes

- Every MCP tool is defined once with a typed argument struct and a typed result payload struct. Input and output JSON Schemas are derived from those types at startup; `tools/list` advertises both, plus a tool-specific description instead of the shared placeholder sentence used by 18 tools today.
- Argument validation happens once from the derived schema: the gateway rejects unknown or malformed arguments before contacting the backend, and the backend independently refuses arguments that do not decode into the tool's typed struct. The shared `Args` union, the `Schema` kind enum, the hand-written schema builder, and per-collector cross-tool argument guards are removed.
- Every tool result is validated against its advertised `outputSchema` before it leaves the gateway. A result that does not conform is returned as an MCP error with a structured `response_shape` issue; no partially conforming observation is passed off as valid.
- Collectors build typed payload structs instead of `map[string]any`; Docker projections become typed constructors instead of map builders. The JSON produced on the wire keeps today's member names, so existing clients see no change in `data` content.
- The repository carries `docs/v1/tools.json`, the exact discovery output for a fully capable diagnostics client. A test regenerates it on request and fails on drift. Release archives ship the same file as `tools.json` and it is also attached as a standalone release asset covered by `checksums.txt`, so a client can pin a HostLens release and generate types offline without a running server.
- `get_hostlens_info` becomes available to the `health` role (still behind the `hostlens` audit-domain grant), so any authenticated client, including a least-privilege desktop client, can read server version, mode, privilege, limits, and policy fingerprint after connecting. No unauthenticated server-information tool or endpoint is added; anonymous requests keep receiving only `401`.
- Operator documentation gains a client-integration section: discovery, envelope fields, `isError` semantics, pagination, and how to expose a gateway to a remote desktop client (TLS, token, no browser origin involved).

Not breaking for MCP clients: tool names, argument names, and `data` member names are unchanged. The gateway-to-backend IPC request shape changes; both executables already ship and upgrade together.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mcp-diagnostics`: the tool-surface requirement lists `get_hostlens_info` at the `health` role; the argument-contract requirement now covers result contracts too (advertised `outputSchema`, validation before return, `response_shape` failure, specific descriptions) and states that both schemas derive from one definition.
- `platform-runtime`: backend IPC carries per-operation typed arguments and rejects requests that do not decode into the operation's contract.
- `production-audit`: the audit-authority requirement now names `get_hostlens_info` as the one audit tool available at the `health` role; the audit-domain grant requirement is unchanged.
- `release-validation`: the repository carries a machine-readable tool contract snapshot that CI compares against live discovery; release archives and release assets ship that snapshot, and archive acceptance proves the shipped copy equals the executed candidate's discovery.

## Impact

- `internal/contract`: typed argument and payload types, generic tool definition, schema derivation, removal of `Schema`, `Args`, and the map-typed `Result.Data`. `github.com/google/jsonschema-go` moves from indirect to direct dependency; no new module enters the graph.
- `internal/gateway`: registration with both schemas, input validation before IPC, output validation after IPC, deletion of `schema.go`.
- `internal/backend`: per-tool argument decoding with unknown-field rejection; `Collector` interface takes the decoded typed arguments.
- `internal/platform/linux`, `internal/dockerobs`: every collector and projection produces typed payloads; `validAuditArgs` and map projections are deleted; generic pagination helper.
- `tools/smoke`: adapts to the typed result envelope.
- `docs/v1/tools.json` (new), `docs/v1/OPERATIONS.md`, `docs/v1/INSTALL.md` (extracted file list), `docs/v1/SPEC.md`, `docs/v1/VALIDATION.md`, `docs/RELEASING.md`, `CHANGELOG.md`.
- Release packaging: `tools/release/prepare.py` copies the snapshot into each architecture directory (the existing `*.json` archive glob picks it up), `tools/archive/main.go` expects it, the release workflow attaches it as a standalone asset listed in `checksums.txt`, and `AGENTS.md` gains the standing rule that every tool is defined once from typed Go structs.
- Tests: 27 test files reference the removed types; a golden-snapshot test, an output-validation failure test, and unknown-argument rejection tests are added.
