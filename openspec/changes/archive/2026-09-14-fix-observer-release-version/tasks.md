# Tasks: fix-observer-release-version

## 1. Implementation

- [x] 1.1 Delete `observerapp.Version`; print `contract.Version` in the `version` subcommand; import `internal/contract`. Verify: `go build ./... && go vet ./internal/observerapp/`.

## 2. Acceptance assertions

- [x] 2.1 Add `hostlens-docker-observer` to the `scripts/platform-smoke.sh` version loop and assert the installed observer version in `scripts/systemd-acceptance.sh`. Verify: `shellcheck --shell=sh scripts/platform-smoke.sh scripts/systemd-acceptance.sh`.

## 3. Validation

- [x] 3.1 Container race suite plus a real `make package` confirming the observer binary inside archives reports the candidate version. Verify: `scripts/test-container.sh`, `make package && make verify-archives`, `openspec validate fix-observer-release-version`, `make build`.