# Tasks: harden-token-store-directory

## 1. Implementation

- [x] 1.1 Add the shared `verifyDirTrusted` helper (is-directory via `Lstat`, owner root-or-euid, no group/other write bits) and call it from `read()` before opening the store file and from `change()` replacing its inline checks. Verify: `go build ./... && go vet ./internal/token/`.

## 2. Tests

- [x] 2.1 Add table-driven directory cases to `internal/token`: trusted dir passes; group-write dir fails; other-write dir fails; non-root owner fails; symlinked dir fails; root-owned dir passes. Verify: focused `go test ./internal/token/ -race -run Dir`.

## 3. Validation

- [x] 3.1 Container race suite and focused token tests in container; confirm the token-authorization delta validates. Verify: `scripts/test-container.sh`, `openspec validate harden-token-store-directory`, `make build`.