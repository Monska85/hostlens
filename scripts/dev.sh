#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "${0}")/.." && pwd)
cd "${repo}"

require_tools() {
  for tool in "${@}"; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
      printf 'Missing development tool: %s. Install documented base prerequisites, then run make deps or just deps.\n' "${tool}" >&2
      exit 1
    fi
  done
}

case "${1:-help}" in
  fmt | fmt-check)
    require_tools gofmt .tools/python/bin/ruff .tools/bin/shfmt
    ;;
  lint)
    require_tools .tools/python/bin/ruff shellcheck .tools/bin/actionlint
    ;;
esac

case "${1:-help}" in
  deps)
    for tool in go python3 shellcheck; do
      if ! command -v "${tool}" >/dev/null 2>&1; then
        printf 'Missing prerequisite: %s. Install it before running deps.\n' "${tool}" >&2
        exit 1
      fi
    done
    python3 -c 'import sys; sys.exit("Python 3.11 or newer is required" if sys.version_info < (3, 11) else 0)'
    GOMODCACHE=${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}
    export GOMODCACHE
    export GOTOOLCHAIN=local
    go mod download
    if ! python3 -m venv .tools/python; then
      printf '%s\n' 'Python venv support is required (python3-venv on Debian/Ubuntu).' >&2
      exit 1
    fi
    .tools/python/bin/python -m pip install --disable-pip-version-check \
      --require-hashes --only-binary=:all: -r tools/dev/requirements.txt
    mkdir -p .tools/bin
    GOBIN="${repo}/.tools/bin" go -C tools/dev install github.com/rhysd/actionlint/cmd/actionlint mvdan.cc/sh/v3/cmd/shfmt
    printf '%s\n' 'Dependencies ready. Prepare Docker images explicitly before test or test-matrix.'
    ;;
  fmt)
    gofmt -w cmd internal tools
    .tools/python/bin/ruff check --select I --fix tools
    .tools/python/bin/ruff format tools
    .tools/bin/shfmt -w -ln posix -i 2 -ci scripts/*.sh
    ;;
  fmt-check)
    files=$(gofmt -l cmd internal tools)
    if [ -n "${files}" ]; then
      printf '%s\n' "${files}"
      exit 1
    fi
    .tools/python/bin/ruff format --check tools
    .tools/bin/shfmt -d -ln posix -i 2 -ci scripts/*.sh
    ;;
  lint)
    .tools/python/bin/ruff check tools
    shellcheck --shell=sh scripts/*.sh
    .tools/bin/actionlint
    ;;
  help)
    printf '%s\n' \
      'deps                       Install locked development tools and Go modules' \
      'fmt / fmt-check            Format or check Go, Python and POSIX shell' \
      'lint                       Check Python, shell and GitHub workflows' \
      'test                       Container race tests, Go vet and cross-builds' \
      'coverage                   Container checks with Go text and HTML reports' \
      'check                      Run fmt-check, lint and test' \
      'build                      Compile Linux binaries using Go only' \
      'package / verify-archives  Build or verify installable Linux archives' \
      'release-check              Validate GoReleaser configurations' \
      'test-image                 Build the container validation toolchain' \
      'scan-vulnerabilities       Scan Go dependencies in a disposable container' \
      'test-targets               List container matrix targets' \
      'test-matrix                Run all cases or one target (Make TARGET= / Just argument)' \
      'test-platforms             Run the distribution and architecture cases' \
      'systemd-image              Build the disposable lifecycle image' \
      'test-systemd-restricted    Run restricted lifecycle acceptance' \
      'test-systemd-standard      Run standard lifecycle acceptance'
    ;;
  *)
    printf 'Unknown development command: %s\n' "${1}" >&2
    exit 2
    ;;
esac
