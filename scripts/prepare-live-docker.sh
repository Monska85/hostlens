#!/bin/sh
set -eu

repo=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
# shellcheck source=scripts/live-docker-images.conf
. "$repo/scripts/live-docker-images.conf"

docker pull "$dind"
docker pull "$fixture"
docker tag "$fixture" "$fixture_tag"
printf '%s\n' 'PASS: pinned private Docker acceptance images ready'
