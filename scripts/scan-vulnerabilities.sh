#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "${0}")/.." && pwd)
mod_cache=${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}
image=${HOSTLENS_TEST_IMAGE:-hostlens-checks:local}
# shellcheck source=scripts/compiler-cache.sh
. "${repo}/scripts/compiler-cache.sh"
set --
if [ -n "${compiler_cache}" ]; then
  set -- --mount "type=bind,src=${compiler_cache},dst=/tmp/go-build"
fi
# Network is required only to read the Go vulnerability database.
exec docker run --rm --pull=never "${@}" --read-only --tmpfs /tmp:exec,size=2g \
  --cap-drop=ALL --security-opt=no-new-privileges \
  --pids-limit=512 --memory=4g --cpus=2 \
  --mount "type=bind,src=${repo},dst=/source,readonly" \
  --mount "type=bind,src=${mod_cache},dst=/modules,readonly" \
  -w /source -e GOMODCACHE=/modules \
  -e GOCACHE=/tmp/go-build -e GOPATH=/tmp/go \
  -e GOPROXY=off -e GOTOOLCHAIN=local \
  "${image}" govulncheck ./...
