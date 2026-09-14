#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "${0}")/.." && pwd)
version=$(python3 -B "${repo}/tools/release/prepare.py" version)
image=${2:?Expected privilege mode and selected image}
if ! docker image inspect "${image}" >/dev/null 2>&1; then
  printf 'Systemd test image missing: %s. Build it first: docker build -t %s -f packaging/tests/Dockerfile .\n' \
    "${image}" "${image}" >&2
  exit 1
fi

helpers=${HOSTLENS_HELPERS:?Run make test-matrix or just test-matrix to prepare disposable helpers}
archives=${HOSTLENS_ARCHIVES:-${repo}/dist/archives}
arch=$("${repo}/scripts/container-arch.sh")
mode=${1:-restricted}
case "${mode}" in
  standard | restricted)
    ;;
  *)
    printf '%s\n' 'expected standard or restricted' >&2
    exit 2
    ;;
esac

name="hostlens-systemd-test-$(python3 -c 'import uuid; print(uuid.uuid4().hex)')"
cleanup() {
  docker stop --time 5 "${name}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

set --
if [ "${mode}" = standard ]; then
  set -- --cap-add=DAC_READ_SEARCH
fi
printf 'Systemd: image=%s, mode=%s, architecture=%s (native)\n' "${image}" "${mode}" "${arch}"
printf '%s\n' 'HOSTLENS_STAGE: start systemd container'
docker run --detach --rm --pull=never \
  --name "${name}" --platform "linux/${arch}" \
  --network=none \
  --cgroupns=private \
  --tmpfs /run \
  --tmpfs /run/lock \
  --tmpfs /tmp \
  --pids-limit=512 \
  --memory=1g \
  --cpus=2 \
  --cap-add=SYS_ADMIN \
  --cap-add=SYS_PTRACE \
  "${@}" \
  --security-opt=apparmor=unconfined \
  -e SYSTEMD_LOG_TARGET=console \
  -e "HOSTLENS_VERSION=${version}" \
  "${image}" /bin/sh -ec '
    mount -o remount,rw /sys/fs/cgroup
    exec /sbin/init
  ' >/dev/null

attempt=0
until docker exec "${name}" systemctl is-system-running --quiet; do
  attempt=$((attempt + 1))
  if [ "${attempt}" -ge 30 ]; then
    docker logs "${name}"
    exit 1
  fi
  sleep 1
done

printf '%s\n' 'HOSTLENS_STAGE: copy and verify candidate'
docker cp "${helpers}/archive" "${name}:/opt/archive"
docker cp "${helpers}/containment" "${name}:/opt/hostlens-containment"
docker cp "${helpers}/smoke" "${name}:/opt/hostlens-smoke"
docker cp "${archives}/hostlens-${version}-linux-${arch}.tar.gz" "${name}:/opt/hostlens-${version}-linux-${arch}.tar.gz"
docker cp "${archives}/checksums.txt" "${name}:/opt/checksums.txt"
docker exec -w /opt "${name}" sha256sum --check --strict --ignore-missing checksums.txt
docker exec "${name}" /opt/archive -archive "/opt/hostlens-${version}-linux-${arch}.tar.gz" \
  -dest /opt/hostlens-release -arch "${arch}" -version "${version}"
docker exec "${name}" mv "/opt/hostlens-${version}-linux-${arch}.tar.gz" /opt/candidate.tar.gz
docker cp "${repo}/scripts/systemd-acceptance.sh" "${name}:/opt/systemd-acceptance.sh"
if ! docker exec "${name}" /bin/sh /opt/systemd-acceptance.sh "${mode}" "${arch}"; then
  docker exec "${name}" systemctl show --property=Result --property=ActiveState --property=SubState hostlens-diagnostics.service hostlens-gateway.service
  docker exec "${name}" journalctl --no-pager \
    -u hostlens-gateway.service -u hostlens-diagnostics.service -n 50
  exit 1
fi
