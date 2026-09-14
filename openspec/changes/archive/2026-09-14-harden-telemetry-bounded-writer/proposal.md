# Proposal: harden-telemetry-bounded-writer

## Why

`internal/telemetry`'s `limitedBuffer` embeds `bytes.Buffer`, so `ReadFrom` is promoted: any `io.Copy`-style writer bypasses the 1 MiB telemetry ceiling. This is the same defect class fixed in `internal/platform/linux/collect_linux.go` (the `Unreleased` bounded-capture fix) and contradicts the `service-metrics` byte-ceiling guarantee. Current callers (`Encode`, `Wire`) are safe, but the type silently permits unbounded writes for any future caller.

## What Changes

- Replace the `bytes.Buffer`-embedded `limitedBuffer` with a plain bounded writer (`buf []byte`, `exceeded bool`) that refuses writes beyond `MaxBytes` and keeps the existing `telemetry size limit` error.
- Keep `Encode`/`Wire` semantics and error messages unchanged.
- Add regression tests: an `io.Copy` runtime guard proving the buffer is not an `io.ReaderFrom` (the promoted-method bypass), direct overflow/exact-fit cases, and `exceeded` coverage through `Encode`/`Wire`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None — no spec-level requirement changes. The `service-metrics` byte-ceiling requirement already mandates finite exposition ceilings; this change removes an implementation-level bypass so the code cannot violate it. `skip_specs: true` is set in `.openspec.yaml` per the CLI contract for zero-delta changes.

## Impact

- `internal/telemetry/telemetry.go` (`limitedBuffer`, `Encode`, `Wire`).
- `internal/telemetry/telemetry_test.go` (new regression tests).
- No exported API changes; no wire-format changes; no dependency changes.