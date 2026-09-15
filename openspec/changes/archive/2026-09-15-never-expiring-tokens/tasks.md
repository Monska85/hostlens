# Tasks: never-expiring-tokens

## 1. Implementation

- [x] 1.1 Token package: accept the zero-expiry sentinel in `Create`, bypass only the date check in `Verify`, and treat a zero old-expiry as infinite in `Rotate`'s overlap checks. Verify: `go build ./... && go vet ./internal/token/`.
- [x] 1.2 CLI: accept the literal `never` in the `--expires` parser, update help text, and render `expires` as `"never"` for zero-expiry records in `create`, `list`, and `rotate` outputs. Verify: `go build ./... && go vet ./internal/cli/`.

## 2. Tests

- [x] 2.1 Token package: create-never succeeds, verify-never passes and revoked-never fails, list does not auto-expire the sentinel, rotate-never imposes the finite overlap, rotate replacement may be never, and a store with the sentinel read by expiring-only logic fails closed. Verify: focused `go test ./internal/token/ -race`.
- [x] 2.2 CLI: `--expires never` parses, empty/invalid values still fail, and outputs render `never`. Verify: focused `go test ./internal/cli/ -race`.

## 3. Validation

- [x] 3.1 Container race suite, strict OpenSpec validation, docs updated in the same change. Verify: `scripts/test-container.sh`, `openspec validate never-expiring-tokens`, `make build`.