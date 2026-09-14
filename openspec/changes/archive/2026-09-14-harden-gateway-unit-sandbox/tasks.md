# Tasks: harden-gateway-unit-sandbox

## 1. Implementation

- [x] 1.1 Add the syscall baseline and protected clock/hostname/kernel-logs/private-IPC directives to `GatewayUnit`, and the common directives plus `ProtectProc=invisible`/`ProcSubset=pid` to `ObserverUnit` and `DiagnosticsUnit` (diagnostics without the proc pair). Decide `MemoryDenyWriteExecute` empirically. Verify: `go build ./... && go vet ./internal/lifecycle/`, unit-generation tests pass.

## 2. Acceptance and docs

- [x] 2.1 Assert the sandbox directives on installed gateway, diagnostics, and observer units in `scripts/systemd-acceptance.sh`; update the `docs/v1/OPERATIONS.md` hardening paragraph. Verify: `shellcheck --shell=sh scripts/systemd-acceptance.sh`.

## 3. Validation

- [x] 3.1 Run the systemd acceptance suite in containers for both privilege modes; confirm services start and all probes pass with the new units. Verify: `make systemd-image && make test-systemd-restricted && make test-systemd-standard`, `openspec validate harden-gateway-unit-sandbox`, `make build`.