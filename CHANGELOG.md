# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## 0.1.0 - 2026-09-13

First public release: bearer-authenticated MCP diagnostics for Linux hosts with opt-in, isolated Docker observer diagnostics.

### Added

- Bearer-authenticated MCP gateway over Streamable HTTP with TLS, fixed roles (`health`, `inspect`, `diagnostics`, `metrics`), expiring tokens, and read-only enforcement of the diagnostic effect.
- Diagnostic tools for OS identity, inventory, resource health, services, and packages, plus bounded reads of approved configuration files, JSONL logs, raw tails, and journal units.
- Generic audit tools for processes, network, accounts, storage, updates, security, service units, path metadata, and HostLens runtime facts, each behind explicit policy grants.
- Source policy with profile includes, global denial precedence, provenance-tracked policy explanation, and protected built-in observation sources.
- Service metrics endpoint with a fixed, bounded Prometheus catalog and payload-free operational audit records.
- Stateless read-only MCP enforcement: exhaustive effect registry, default-on read-only gate, and cancellation-safe evidence release on every request.
- Opt-in Docker diagnostics: isolated `hostlens-docker-observer` with a typed GET-only observation contract, nine policy-controlled MCP tools, honest unused-resource analysis, and no retained Docker evidence.
- `hostlens reconcile --system` for Docker observer enablement and disablement with disclosed, ownership-tracked topology changes and socket activation under systemd.
- Installation lifecycle with durable ownership records, recoverable upgrade and rollback, conflict refusal, and Docker-preserving uninstall.
- Release pipeline with reproducible archives, per-member upgrade manifests, checksum verification, container acceptance matrix, and draft delivery to GitHub releases.
- Documentation: installation guide, operator reference, product specification, and validation evidence.