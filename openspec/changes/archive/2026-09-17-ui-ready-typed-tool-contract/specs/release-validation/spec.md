## ADDED Requirements

### Requirement: Tool contract snapshot

The repository SHALL carry a machine-readable snapshot of MCP tool discovery for a client holding every diagnostic role with every runtime capability available: for each tool its name, description, `inputSchema`, and `outputSchema`, ordered by tool name. The test suite SHALL derive the same document from the live tool registry and SHALL fail when the snapshot differs. Regeneration SHALL be an explicit, documented developer action, never an implicit side effect of a normal test run. The snapshot SHALL be published in the versioned documentation, SHALL be included unchanged in every release archive, and SHALL be attached to the release as a standalone asset covered by the release checksum manifest, so that a client can pin a HostLens release and generate types without a running server. Archive acceptance SHALL verify that the shipped snapshot equals the discovery output of the executed candidate.

#### Scenario: Contract drift

- **WHEN** a tool's arguments, result members, or description change without regenerating the snapshot
- **THEN** the container test suite fails and names the differing tool

#### Scenario: Deliberate regeneration

- **WHEN** a developer runs the documented regeneration command after an intended contract change
- **THEN** the snapshot is rewritten from the live registry, the test passes, and the diff is reviewable in the change

#### Scenario: Snapshot matches live discovery

- **WHEN** a diagnostics client with all capabilities calls `tools/list` on a release build
- **THEN** each returned tool's name, description, `inputSchema`, and `outputSchema` equal the snapshot entry for that release

#### Scenario: Shipped snapshot

- **WHEN** a release archive is extracted and the release assets are listed
- **THEN** the archive contains `tools.json`, the release carries a standalone snapshot asset, both are covered by `checksums.txt`, and the amd64 and arm64 copies are byte-identical

#### Scenario: Shipped snapshot diverges

- **WHEN** archive acceptance finds that `tools.json` from the candidate archive differs from the executed candidate's `tools/list`
- **THEN** acceptance fails and release delivery cannot run
