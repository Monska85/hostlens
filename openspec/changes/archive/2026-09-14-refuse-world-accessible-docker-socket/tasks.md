# Tasks: refuse-world-accessible-docker-socket

## 1. Implementation

- [x] 1.1 Require `mode&0o007 == 0` in `validateEngineSocket` and update the refusal message. Verify: `go build ./... && go vet ./internal/observerapp/`.

## 2. Tests

- [x] 2.1 Extend socket-validation tests: `0644` refuses, `0666` refuses with the world-access message, `0660` passes. Verify: focused `go test ./internal/observerapp/ -race`.

## 3. Validation

- [x] 3.1 Container race suite. Verify: `scripts/test-container.sh`, `openspec validate refuse-world-accessible-docker-socket`, `make build`.