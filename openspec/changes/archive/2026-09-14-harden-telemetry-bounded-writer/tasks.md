# Tasks: harden-telemetry-bounded-writer

## 1. Implementation

- [x] 1.1 Replace the embedded `bytes.Buffer` in `internal/telemetry/telemetry.go` `limitedBuffer` with a plain bounded writer (`buf []byte`, `exceeded bool`) that refuses writes past `MaxBytes` with the existing `telemetry size limit` error; keep `Encode`/`Wire` call semantics unchanged. Verify: `go build ./... && go vet ./...`.

## 2. Regression tests

- [x] 2.1 Add tests proving the ceiling holds under `io.Copy` (no promoted `ReadFrom` dispatch), direct overflow refusal with `exceeded` set, exact-fit acceptance, and the `exceeded` path through `Encode`/`Wire`. Verify: focused `go test ./internal/telemetry/ -race -run 'LimitedBuffer|MaximumCatalog'`.

## 3. Validation

- [x] 3.1 Run the container race suite and coverage; confirm coverage stays at or above the 85.8% baseline or explain any delta. Verify: `scripts/test-container.sh`, `make coverage`, `make build`.