# Tasks: align-audit-link-comment

## 1. Implementation

- [x] 1.1 Correct the `auditLink` comment to state that executable target paths are default-allowed unless denied by policy and that only the path string is exposed. Verify: `go build ./...`, `gofmt -l internal` empty.

## 2. Validation

- [x] 2.1 Confirm no behavior change and strict OpenSpec validation. Verify: `scripts/test-container.sh`, `openspec validate align-audit-link-comment`.