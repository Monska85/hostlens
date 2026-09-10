# Hardening design

## Context

See [proposal](proposal.md). The implementation already separates gateway and diagnostics and retains bounded backend slots until work actually stops. Preserve those boundaries and the current v1 exclusions.

## Goals / Non-Goals

Fix demonstrated defects with small changes and scenario regressions. Do not replace the MCP SDK, introduce a cross-platform framework, or remove tests that enforce distinct security and failure behavior.

## Decisions

- **Secret aliases.** Reject diagnostic regular files with multiple hard links. Secret bind masks obscure original inode identity, making pathname-based alias comparisons insufficient. A separate privileged inode inventory would add avoidable state and synchronization.
- **Admission.** Bound concurrent gateway handlers before token-store reads and MCP construction, return overload promptly, and apply a request context deadline. Keep backend admission independent.
- **Immutable policy.** Cache fingerprints at snapshot construction. Preserve provenance for explanations and avoid caching authorization across token changes.
- **Bounded inputs.** Read configuration with a byte ceiling and enumerate directory entries in bounded batches. Stop traversal on count, time, or cancellation without claiming complete coverage.
- **Platform separation.** Move portable credential and authorization logic out of Linux-only files; keep filesystem and locking behavior in OS adapters. Verify portable packages cross-compile without claiming runtime support.
- **Artifacts.** Build in fresh managed staging directories. Verify expanded executables and helper hashes before matrix execution; align archive verification with runtime extraction limits and gzip integrity.
- **Documentation.** README explains purpose and entry points; operator and release guides own procedures; current OpenSpec owns requirements. Keep historical evidence clearly historical and record this audit separately.

## Risks / Trade-offs

Hard-linked configuration and log files become unsupported diagnostic inputs. Document the rejection rather than weakening mandatory secret protection.

Kernel-blocked filesystem I/O cannot be cancelled safely from a Go context. Retain occupied slots to bound unfinished work, check cancellation between operations, and document filesystem exclusions and recovery. Containers cannot prove native arm64, macOS, Windows, or arbitrary host-kernel behavior.

## Migration Plan

No token or configuration schema migration is intended. Rebuild and run local acceptance before one amendment and leased push of the existing initial commit. Hosted failures permit only the authorized CI correction rounds. Do not publish releases or change visibility.
