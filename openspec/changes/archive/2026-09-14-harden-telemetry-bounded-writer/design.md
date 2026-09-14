# Design: harden-telemetry-bounded-writer

## Context

`internal/telemetry`'s `limitedBuffer` is `struct{ bytes.Buffer }` with a `Write` override that enforces `MaxBytes` (1 MiB). Embedding promotes `bytes.Buffer`'s full API, including `ReadFrom`, so the type satisfies `io.ReaderFrom` and `io.Copy` dispatches to it, bypassing the override entirely. The audit reproduced this empirically. The privileged collector's buffer (`internal/platform/linux/collect_linux.go:31-51`) is already the correct plain-struct form; `docs/v1/VALIDATION.md` claims that defect class was eliminated.

## Goals / Non-Goals

- Goals: make `limitedBuffer` incapable of unbounded writes regardless of caller; preserve `Encode`/`Wire` behavior, error text, and the exported surface.
- Non-Goals: no changes to `MaxBytes`, the exposition format, `DecodeBackend`, or the collector's own buffer; no new exported API.

## Decisions

- **Mirror the collector's plain struct** (`buf []byte`, `exceeded bool`) rather than removing `ReadFrom` via a stub: the collector shape is proven, reviewed in prior work, and keeps one canonical pattern for bounded writers in the codebase. A `ReadFrom` stub returning an error would also work but adds a method whose only purpose is to fail, inviting confusion.
- **Refuse the write (return `0, err`) instead of clamping** — telemetry `Write` semantics differ from the collector's clamping semantics, and `Encode`/`Wire` must keep returning their existing `telemetry size limit` error so call-site behavior (non-success exposition) stays identical. `exceeded` is set before the error return for diagnostics and test assertions.
- **Regression guard as a runtime `io.Copy` test** rather than a build-tagged negative compile check: Go has no supported way to assert "must not compile" inside a normal test file, while `io.Copy` exercises exactly the promoted-method dispatch that caused the defect. If `bytes.Buffer` embedding returns, `io.Copy` succeeds beyond the ceiling and the test fails.

## Risks / Trade-offs

- [A future writer relies on clamping behavior like the collector's] → This buffer is package-private and documented as refusing overflow; `Encode`/`Wire` propagate the error, which callers already treat as failure.
- [`bytes()` slice escapes/aliases after growth] → Buffer is single-use per encode call, owned by `Encode`/`Wire`, and never written after return.

## Migration Plan

Single-package change; no data migration. Rollback is a plain revert.

## Open Questions

None.