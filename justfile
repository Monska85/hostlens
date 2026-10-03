set shell := ["/bin/sh", "-eu", "-c"]

help:
    @scripts/dev.sh help

deps:
    scripts/dev.sh deps

build:
    GOMODCACHE="${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}" CGO_ENABLED=0 GOOS=linux go build -trimpath -o bin/hostlens ./cmd/hostlens

package:
    GOMODCACHE="${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}" goreleaser release --snapshot --clean

test-image:
    docker build --load -t "${HOSTLENS_TEST_IMAGE:-hostlens-checks:local}" -f packaging/tests/Dockerfile.checks .

test:
    scripts/test-container.sh

coverage:
    scripts/test-container.sh coverage

benchmark:
    scripts/test-container.sh benchmark

fmt:
    scripts/dev.sh fmt

fmt-check:
    scripts/dev.sh fmt-check

lint:
    scripts/dev.sh lint

check: fmt-check lint test

verify-archives:
    scripts/test-container.sh archives

release-check:
    goreleaser check

systemd-image:
    docker build -t "${HOSTLENS_SYSTEMD_IMAGE:-hostlens-systemd-probe:local}" -f packaging/tests/Dockerfile .

test-systemd:
    scripts/test-systemd-container.sh

prepare-live-docker:
    scripts/prepare-live-docker.sh

test-live-docker:
    scripts/test-live-docker.sh

scan-vulnerabilities:
    scripts/scan-vulnerabilities.sh
