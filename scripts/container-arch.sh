#!/bin/sh
set -eu

platform=$(docker info --format '{{.OSType}}/{{.Architecture}}')
case "${platform}" in
  linux/amd64 | linux/x86_64)
    printf '%s\n' amd64
    ;;
  linux/arm64 | linux/aarch64)
    printf '%s\n' arm64
    ;;
  *)
    printf 'Unsupported Docker engine platform: %s. A Linux amd64 or arm64 engine is required.\n' "${platform}" >&2
    exit 1
    ;;
esac
