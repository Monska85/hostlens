#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "${0}")/.." && pwd)
if [ "${#}" -ne 2 ] && [ "${#}" -ne 3 ]; then
  printf '%s\n' 'Expected image, binary architecture and optional application fixture.' >&2
  exit 2
fi
engine_arch=$("${repo}/scripts/container-arch.sh")
qemu=${HOSTLENS_QEMU_AARCH64:-/usr/bin/qemu-aarch64-static}
if [ "${engine_arch}" = amd64 ] && [ "${2}" = arm64 ]; then
  if [ ! -f "${qemu}" ] || [ ! -x "${qemu}" ]; then
    printf 'Missing qemu-aarch64-static interpreter: %s. Set HOSTLENS_QEMU_AARCH64. No VM is used.\n' "${qemu}" >&2
    exit 1
  fi
fi
image=${1}
arch=${2}
application=${3:-}
fixture_user=0
memory=512m
tmp_size=256m
if [ -n "${application}" ]; then
  memory=2g
  tmp_size=1536m
  case "${application}" in
    postgres) fixture_user=postgres ;;
    nginx) fixture_user=nginx ;;
    apache) fixture_user=daemon ;;
    mysql) fixture_user=mysql ;;
    *)
      printf '%s\n' 'Unknown application fixture' >&2
      exit 2
      ;;
  esac
fi
case "${arch}" in
  amd64 | arm64) ;;
  *) exit 2 ;;
esac
helpers=${HOSTLENS_HELPERS:?Run make test-matrix or just test-matrix to prepare disposable helpers}
archives=${HOSTLENS_ARCHIVES:-${repo}/dist/archives}
version=$(python3 -B "${repo}/tools/release/prepare.py" version)
container_arch=${arch}
set --
if [ "${arch}" = arm64 ] && [ "${engine_arch}" = amd64 ]; then
  container_arch=amd64
  set -- --mount "type=bind,src=${qemu},dst=/qemu-aarch64-static,readonly"
fi
execution=native
if [ "${arch}" != "${engine_arch}" ]; then
  execution=emulated
fi
printf 'Platform: %s, binary: %s, Docker engine: %s, execution: %s\n' "${image}" "${arch}" "${engine_arch}" "${execution}"
name="hostlens-platform-test-$(python3 -c 'import uuid; print(uuid.uuid4().hex)')"
cleanup() {
  docker rm --force "${name}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
docker run --rm --pull=never --name "${name}" --platform "linux/${container_arch}" \
  --network=none --read-only --tmpfs "/tmp:exec,size=${tmp_size}" \
  --memory="${memory}" --cpus=1 --pids-limit=256 --user "${fixture_user}" --entrypoint /bin/sh \
  --cap-drop=ALL --security-opt=no-new-privileges \
  -e "HOSTLENS_VERSION=${version}" \
  -e "HOSTLENS_CONTAINER_ARCH=${container_arch}" \
  -e "HOSTLENS_QEMU_PRESERVE_ARGV0=${HOSTLENS_QEMU_PRESERVE_ARGV0:-0}" \
  --mount "type=bind,src=${archives},dst=/archives,readonly" \
  --mount "type=bind,src=${helpers},dst=/helpers,readonly" \
  --mount "type=bind,src=${repo}/scripts/platform-smoke.sh,dst=/smoke.sh,readonly" \
  --mount "type=bind,src=${repo}/scripts/application-fixture.sh,dst=/application-fixture.sh,readonly" \
  "${@}" "${image}" /smoke.sh "${arch}" "${application}"
