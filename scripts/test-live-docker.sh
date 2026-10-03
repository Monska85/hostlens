#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
# shellcheck source=scripts/live-docker-images.conf
. "$repo/scripts/live-docker-images.conf"
checks=${HOSTLENS_TEST_IMAGE:-hostlens-checks:local}
mod_cache=${HOSTLENS_MOD_CACHE:-$(go env GOMODCACHE)}
outer_host=${DOCKER_HOST:-$(docker context inspect --format '{{(index .Endpoints "docker").Host}}')}
case "$outer_host" in
  unix:///*) ;;
  *)
    printf 'Live Docker acceptance requires a local Unix Docker endpoint, got: %s\n' "$outer_host" >&2
    exit 1
    ;;
esac
# Pin every outer Docker command to the checked local endpoint, even when a
# separate DOCKER_CONTEXT was set in the caller's environment.
DOCKER_HOST=$outer_host
export DOCKER_HOST
unset DOCKER_CONTEXT

for image in "$dind" "$fixture" "$checks"; do
  if ! docker image inspect "$image" >/dev/null 2>&1; then
    printf 'Required image is unavailable: %s. Run scripts/prepare-live-docker.sh and make test-image.\n' "$image" >&2
    exit 1
  fi
done
if [ "$(docker image inspect --format '{{.Id}}' "$fixture")" != "$(docker image inspect --format '{{.Id}}' "$fixture_tag")" ]; then
  printf 'Pinned fixture tag is unavailable or differs from %s. Run scripts/prepare-live-docker.sh.\n' "$fixture" >&2
  exit 1
fi
if [ ! -d "$mod_cache/cache/download" ]; then
  printf 'Module cache missing: %s. Run make deps first.\n' "$mod_cache" >&2
  exit 1
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/hostlens-live-docker.XXXXXX")
chmod 711 "$work"
mkdir "$work/socket"
touch "$work/isolated-runner"
engine="hostlens-dind-$(basename "$work")"
cleanup() {
  if [ -s "$work/checks-container" ]; then
    docker rm -f "$(cat "$work/checks-container")" >/dev/null 2>&1 || true
  fi
  docker rm -f "$engine" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker save -o "$work/fixture.tar" "$fixture_tag"
chmod 644 "$work/fixture.tar"
printf '%s\n' 'HOSTLENS_STAGE: start private nested Docker engine'
docker run --rm -d --pull=never --privileged --network=none \
  --name "$engine" \
  --mount "type=bind,src=$work/socket,dst=/dind" \
  --tmpfs /var/lib/docker:exec,size=2g \
  --tmpfs /run:exec,size=64m \
  --tmpfs /tmp:exec,size=64m \
  --entrypoint dockerd "$dind" \
  --host=unix:///dind/docker.sock --storage-driver=vfs \
  --iptables=false --bridge=none --ip-forward=false --ip-masq=false --group=root >/dev/null

ready=0
attempt=0
while [ "$attempt" -lt 60 ]; do
  if docker exec -e DOCKER_HOST=unix:///dind/docker.sock "$engine" docker info >/dev/null 2>&1; then
    ready=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  docker logs "$engine" >&2
  printf '%s\n' 'Private nested Docker engine did not become ready.' >&2
  exit 1
fi
docker cp "$engine:/usr/local/bin/docker" "$work/docker"
chmod 755 "$work/docker"

printf '%s\n' 'HOSTLENS_STAGE: load pinned fixture and run live Docker observer acceptance'
# The private daemon socket is root-owned; checks retain no Linux capabilities.
timeout --signal=TERM --kill-after=5s 240s docker run --rm --pull=never \
  --cidfile "$work/checks-container" --user 0:0 --network=none --read-only \
  --tmpfs /tmp:exec,size=4g --pids-limit=512 --memory=4g --cpus=2 \
  --cap-drop=ALL --security-opt=no-new-privileges \
  --mount "type=bind,src=$repo,dst=/source,readonly" \
  --mount "type=bind,src=$mod_cache,dst=/modules,readonly" \
  --mount "type=bind,src=$work/socket,dst=/dind" \
  --mount "type=bind,src=$work/fixture.tar,dst=/fixtures/busybox.tar,readonly" \
  --mount "type=bind,src=$work/docker,dst=/usr/local/bin/docker,readonly" \
  --mount "type=bind,src=$work/isolated-runner,dst=/run/hostlens-isolated-docker-test,readonly" \
  -e GOMODCACHE=/modules -e GOCACHE=/tmp/go-build -e GOPATH=/tmp/go \
  -e GOTOOLCHAIN=local -e GOPROXY=off \
  -e DOCKER_HOST=unix:///dind/docker.sock \
  -e HOSTLENS_LIVE_DOCKER_SOCKET=/dind/docker.sock \
  -e HOSTLENS_LIVE_DOCKER_FIXTURES=1 \
  "$checks" /bin/sh -eu -c \
  'cd /source; docker load -i /fixtures/busybox.tar; go test -count=1 -timeout=180s -run "^TestPrivateLiveDocker$" ./internal/containers'
printf '%s\n' 'PASS: private live Docker observer acceptance'
