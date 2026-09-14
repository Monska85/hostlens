# Tasks: accept-case-insensitive-bearer

## 1. Implementation

- [x] 1.1 Replace prefix matching with `strings.Cut` + `strings.EqualFold` in `internal/gateway/transport.go` and `internal/gateway/metrics.go`. Verify: `go build ./... && go vet ./internal/gateway/`.

## 2. Tests

- [x] 2.1 Cover `bearer`, `BEARER`, and `Bearer` spellings for the MCP transport and the metrics endpoint, plus unchanged rejection of missing/multiple/non-bearer/unverifiable credentials. Verify: focused `go test ./internal/gateway/ -race`.

## 3. Validation

- [x] 3.1 Container race suite and gateway HTTP tests. Verify: `scripts/test-container.sh`, `openspec validate accept-case-insensitive-bearer`, `make build`.