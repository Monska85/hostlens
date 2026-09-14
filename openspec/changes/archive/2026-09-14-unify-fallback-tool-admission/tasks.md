# Tasks: unify-fallback-tool-admission

## 1. Implementation

- [x] 1.1 Include `toolAdmitted(definition, known, readOnly)` in the backend-unavailable fallback condition, reusing the per-call predicate. Verify: `go build ./... && go vet ./internal/gateway/`.

## 2. Tests

- [x] 2.1 Add the backend-loss fallback test: admitted tool receives `backend_unavailable`; unknown tool never receives the failure envelope. Verify: focused `go test ./internal/gateway/ -race -run BackendLoss`.

## 3. Validation

- [x] 3.1 Container race suite and strict OpenSpec validation. Verify: `scripts/test-container.sh`, `openspec validate unify-fallback-tool-admission`, `make build`.