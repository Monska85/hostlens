# Proposal

## Why

HostLens still maintains Docker Engine protocol handling and several Linux data parsers that upstream Go packages provide. This increases the code and compatibility surface of a public diagnostic product.

## What Changes

- Update the Go toolchain, direct and indirect Go modules, and release tooling to compatible latest stable releases verified against official sources.
- Replace suitable Docker Engine response types with Moby's maintained API types while preserving the fixed observation-only request interface, socket checks, limits, and error contract.
- Evaluate Linux procfs and systemd libraries against source policy, bounded reads, cgo-free builds, and result semantics; adopt only compatible APIs.
- Use Go's rooted filesystem API for release archive extraction while retaining archive member, ownership, and checksum checks.
- Keep bespoke authorization, token durability, lifecycle ownership, diagnostic statelessness, and published MCP and metrics contracts where the proposed libraries do not implement the same behavior.

## Capabilities

### New Capabilities

None. This change preserves the current capabilities and external contracts.

### Modified Capabilities

None. The change refactors implementations and tooling without changing requirements.

## Impact

The Docker observer, Linux collectors, lifecycle internals, module manifests, Go and GoReleaser pins, container tooling, and validation documentation are affected. Linux amd64 and arm64 remain the supported installed targets; portable packages must still build for macOS and Windows. No new diagnostic or mutation authority is introduced.
