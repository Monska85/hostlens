# Independent Just entry points. Keep this filename lowercase: justfile.
set shell := ["/bin/sh", "-eu", "-c"]

# List available recipes.
help:
    @scripts/dev.sh help

# Prepare local tools and the selected Go module cache.
deps:
    scripts/dev.sh deps

# Compile Linux binaries for the selected architecture using Go only.
build:
    GOMODCACHE="${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}" CGO_ENABLED=0 GOOS=linux go build -trimpath -o bin/ ./cmd/...

# Build installable Linux amd64 and arm64 archives.
package:
    GOMODCACHE="${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}" goreleaser release --snapshot --clean

# Prepare the container-owned validation toolchain.
test-image:
    docker build --load -t "${HOSTLENS_TEST_IMAGE:-hostlens-checks:local}" -f packaging/tests/Dockerfile.checks .

# Run the Go race suite, vet and builds in a disposable container.
test:
    scripts/test-container.sh

# Run container checks and write Go coverage reports to coverage/.
coverage:
    scripts/test-container.sh coverage

# Format Go, Python and POSIX shell.
fmt:
    scripts/dev.sh fmt

# Check formatting without modifications.
fmt-check:
    scripts/dev.sh fmt-check

# Lint Python, shell and workflows. Go vet runs with container tests.
lint:
    scripts/dev.sh lint

# Check POSIX shell scripts only.
lint-shell:
    shellcheck --shell=sh scripts/*.sh

# Run distribution and emulated arm64 smoke tests; requires package.
test-platforms:
    python3 -B tools/test-matrix/main.py --kind platform

# Build the local systemd acceptance image.
systemd-image:
    docker build -t "${HOSTLENS_SYSTEMD_IMAGE:-hostlens-systemd-probe:local}" -f packaging/tests/Dockerfile .

# Run restricted-mode lifecycle tests; requires package and systemd-image.
test-systemd-restricted:
    python3 -B tools/test-matrix/main.py systemd-restricted

# Run standard-mode lifecycle tests; requires package and systemd-image.
test-systemd-standard:
    python3 -B tools/test-matrix/main.py systemd-standard

# Run formatting, lint and container tests.
check: fmt-check lint test

# Verify candidate versions, members and checksums; requires package.
verify-archives:
    scripts/test-container.sh archives

# Scan Go dependencies in a disposable container; requires test-image.
scan-vulnerabilities:
    scripts/scan-vulnerabilities.sh

# List shared container targets, including the native quick-check alias.
test-targets:
    python3 -B tools/test-matrix/main.py --list

# Run all container cases, or one named target; requires a current build.
test-matrix target='':
    python3 -B tools/test-matrix/main.py {{quote(target)}}

# Validate packaging and exact-candidate publishing configurations.
release-check:
    goreleaser check
