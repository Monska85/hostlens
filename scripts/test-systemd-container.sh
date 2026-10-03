#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
image=${HOSTLENS_SYSTEMD_IMAGE:-hostlens-systemd-probe:local}
version=$(python3 -B "$repo/tools/release/prepare.py" version)
archive_dir=${HOSTLENS_ARCHIVES:-$repo/dist/archives}
arch=$("$repo/scripts/container-arch.sh")
case "$arch" in amd64 | arm64) ;; *)
  echo "unsupported Docker architecture: $arch" >&2
  exit 1
  ;;
esac
if ! docker image inspect "$image" >/dev/null 2>&1; then
  echo "systemd test image missing; run make systemd-image" >&2
  exit 1
fi
archive="$archive_dir/hostlens-$version-linux-$arch.tar.gz"
test -f "$archive"
name="hostlens-systemd-$(python3 -c 'import uuid; print(uuid.uuid4().hex)')"
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf 'HOSTLENS_STAGE: disposable systemd container (%s)\n' "$arch"
docker run --detach --rm --pull=never --name "$name" --platform "linux/$arch" \
  --network=none --cgroupns=private --tmpfs /run --tmpfs /run/lock --tmpfs /tmp \
  --pids-limit=512 --memory=1g --cpus=2 --cap-add=SYS_ADMIN \
  --security-opt=apparmor=unconfined -e SYSTEMD_LOG_TARGET=console \
  "$image" /bin/sh -ec 'mount -o remount,rw /sys/fs/cgroup; exec /sbin/init' >/dev/null
attempt=0
until docker exec "$name" systemctl list-units --no-pager >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$name" >&2
    exit 1
  fi
  sleep 1
done
docker cp "$archive" "$name:/opt/candidate.tar.gz"
docker cp "$repo/scripts/systemd-candidate-acceptance.sh" "$name:/opt/acceptance.sh"
if ! docker exec "$name" /bin/sh /opt/acceptance.sh /opt/candidate.tar.gz "$version"; then
  docker exec "$name" journalctl --no-pager -u hostlens-gateway.service -u hostlens-repair.service -n 60 || true
  exit 1
fi
