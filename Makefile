# Independent GNU Make entry points. Each target invokes the responsible tool or script.
SHELL := /bin/sh
.SHELLFLAGS := -eu -c
.DEFAULT_GOAL := help

.PHONY: release-check package test-image coverage lint verify-archives scan-vulnerabilities help check deps build test fmt fmt-check lint-shell test-platforms systemd-image test-systemd-restricted test-systemd-standard

help:
	@scripts/dev.sh help

# Prepare local tools and the selected Go module cache.
deps:
	scripts/dev.sh deps

# Compile Linux binaries for the selected architecture using Go only.
build:
	GOMODCACHE="$${HOSTLENS_MOD_CACHE:-$$(go env GOMODCACHE)}" CGO_ENABLED=0 GOOS=linux go build -trimpath -o bin/ ./cmd/...

# Build installable Linux amd64 and arm64 archives.
package:
	GOMODCACHE="$${HOSTLENS_MOD_CACHE:-$$(go env GOMODCACHE)}" goreleaser release --snapshot --clean

# Prepare the container-owned validation toolchain.
test-image:
	docker build --load -t "$${HOSTLENS_TEST_IMAGE:-hostlens-checks:local}" -f packaging/tests/Dockerfile.checks .

# Run the Go race suite, vet and builds in a disposable container.
test:
	scripts/test-container.sh

# Run container checks and write Go coverage reports to coverage/.
coverage:
	scripts/test-container.sh coverage

# Format Go, Python and POSIX shell.
fmt:
	scripts/dev.sh fmt

fmt-check:
	scripts/dev.sh fmt-check

lint:
	scripts/dev.sh lint

lint-shell:
	shellcheck --shell=sh scripts/*.sh

# Run distribution and emulated arm64 smoke tests; requires package.
test-platforms:
	python3 -B tools/test-matrix/main.py --kind platform

# Build the local systemd acceptance image.
systemd-image:
	docker build -t "$${HOSTLENS_SYSTEMD_IMAGE:-hostlens-systemd-probe:local}" -f packaging/tests/Dockerfile .

# Run restricted-mode lifecycle tests; requires package and systemd-image.
test-systemd-restricted:
	python3 -B tools/test-matrix/main.py systemd-restricted

# Run standard-mode lifecycle tests; requires package and systemd-image.
test-systemd-standard:
	python3 -B tools/test-matrix/main.py systemd-standard

# Independent checks may run concurrently with make -j.
check: fmt-check lint test

# Verify candidate versions, members and checksums; requires package.
verify-archives:
	scripts/test-container.sh archives

# Scan Go dependencies in a disposable container; requires test-image.
scan-vulnerabilities:
	scripts/scan-vulnerabilities.sh

# List shared container targets, including the native quick-check alias.
.PHONY: test-targets test-matrix
# Export through the environment rather than interpolating target text into shell code.
export HOSTLENS_MATRIX_TARGET = $(TARGET)
test-targets:
	python3 -B tools/test-matrix/main.py --list

# Run all container cases, or one named target; requires a current build.
test-matrix:
	python3 -B tools/test-matrix/main.py "$${HOSTLENS_MATRIX_TARGET}"

# Validate packaging and exact-candidate publishing configurations.
release-check:
	goreleaser check
