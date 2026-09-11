#!/bin/sh
set -eu

# Execute acceptance tests in a disposable filesystem with isolated namespaces.
repo=$(CDPATH='' cd -- "$(dirname -- "${0}")/.." && pwd)
image=${HOSTLENS_TEST_IMAGE:-hostlens-checks:local}
mode=${1:-test}
case "${mode}" in test | coverage | archives) ;; *)
  printf 'Usage: %s [test|coverage|archives]\n' "$0" >&2
  exit 2
  ;;
esac
if [ "${mode}" = coverage ]; then
  mkdir -p "${repo}/coverage"
  rm -f "${repo}/coverage/coverage.out" "${repo}/coverage/index.html" "${repo}/coverage/functions.txt"
fi
mod_cache=${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}
if [ ! -d "${mod_cache}/cache/download" ]; then
  printf 'Module cache missing: %s. Run GOMODCACHE="%s" go mod download first.\n' \
    "${mod_cache}" "${mod_cache}" >&2
  exit 1
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/hostlens-check.XXXXXX")
cleanup() {
  if [ -s "${work}/container" ]; then
    docker rm -f "$(cat "${work}/container")" >/dev/null 2>&1 || true
  fi
  rm -rf "${work}"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
set --
if [ "${mode}" = coverage ]; then
  reports=${work}/reports
  mkdir "${reports}"
  # Only this disposable, non-secret report directory admits remapped Docker UIDs.
  chmod 1777 "${reports}"
  set -- --mount "type=bind,src=${reports},dst=/coverage"
elif [ "${mode}" = archives ]; then
  archives=${HOSTLENS_ARCHIVES:-${repo}/dist/archives}
  version=$(python3 -B "${repo}/tools/release/prepare.py" version)
  set -- --mount "type=bind,src=${archives},dst=/archives,readonly" -e "HOSTLENS_VERSION=${version}"
fi

# shellcheck source=scripts/compiler-cache.sh
. "${repo}/scripts/compiler-cache.sh"
if [ -n "${compiler_cache}" ]; then
  set -- "${@}" --mount "type=bind,src=${compiler_cache},dst=/tmp/go-build"
fi

docker run --rm --pull=never --cidfile "${work}/container" "${@}" \
  --network=none \
  --read-only \
  --tmpfs /tmp:exec,size=4g \
  --pids-limit=512 \
  --memory=4g \
  --cpus=2 \
  --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --mount "type=bind,src=${repo},dst=/source,readonly" \
  --mount "type=bind,src=${mod_cache},dst=/modules,readonly" \
  -e GOMODCACHE=/modules \
  -e GOCACHE=/tmp/go-build \
  -e GOPATH=/tmp/go \
  -e GOTOOLCHAIN=local \
  -e GOPROXY=off \
  "${image}" /bin/sh /source/scripts/container-acceptance.sh "${mode}" &
wait "$!"
if [ "${mode}" = coverage ]; then
  for file in coverage.out index.html functions.txt; do
    test -s "${reports}/${file}"
  done
  cp "${reports}/coverage.out" "${reports}/index.html" "${reports}/functions.txt" "${repo}/coverage/"
  printf 'Coverage reports: %s/coverage/index.html (HTML), coverage.out, functions.txt\n' "${repo}"
fi
