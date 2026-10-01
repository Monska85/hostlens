# Proposal

## Why

The previous refactor improved isolated workloads but left live Docker behavior unverified in a disposable engine and repeated avoidable work in resource correlation. Trusted lifecycle inputs and local identity files also deserve tighter bounds before a production-readiness claim.

## What Changes

- Run existing live Docker observation and fixture scenarios against a disposable Docker daemon, with explicit failure when that acceptance target cannot run and no access to host Docker workloads.
- Treat Docker's unlimited PID sentinel as an unavailable finite limit while preserving the rest of a valid stats observation.
- Avoid building unused Docker reference indexes for resource-specific list calls and compare bounded workloads before and after the change.
- Reject unknown installation and release manifest fields within their existing byte limits, and bound service identity file reads.
- Hash TLS restart inputs without reading entire files into memory.
- Reject architecture-mismatched systemd matrix cases instead of reporting amd64 execution as arm64 evidence.
- Reconcile validation claims, contributor instructions, and the changelog with measured evidence and remaining limits.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation`: require disposable live Docker acceptance and strict release manifest decoding.
- `installation-lifecycle`: require strict installation manifest decoding before mutation.
- `docker-diagnostics`: preserve stats when the daemon reports an unlimited PID limit.

## Impact

Docker collector, lifecycle decoding, Linux identity lookup, gateway restart checks, test tooling, OpenSpec, and validation documentation. MCP member names, Docker observation authority, supported Linux platforms, and release archive format stay unchanged.
