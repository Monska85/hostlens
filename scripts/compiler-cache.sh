#!/bin/sh
# Sourced by container launchers after selecting image and repository.
# Only an explicitly prepared, private directory may receive compiler outputs.
compiler_cache=
if [ -n "${HOSTLENS_BUILD_CACHE:-}" ]; then
  if [ -d "${HOSTLENS_BUILD_CACHE}" ] && [ -w "${HOSTLENS_BUILD_CACHE}" ]; then
    cache_root=$(CDPATH='' cd -- "${HOSTLENS_BUILD_CACHE}" && pwd -P)
    if [ "$(stat -c '%u:%a' "${cache_root}")" != "$(id -u):700" ]; then
      printf '%s\n' 'Compiler cache parent must be caller-owned with mode 0700; using disposable compilation.' >&2
      return 0
    fi
    # Image identity includes architecture, Go, C compiler and validation tools.
    cache_image=$(docker image inspect --format '{{.Id}}' "${image:?Expected validation image}")
    case "${cache_image}" in sha256:*) ;; *) exit 1 ;; esac
    compiler_cache=${cache_root}/${cache_image#sha256:}
    if [ -L "${compiler_cache}" ]; then
      printf '%s\n' 'Compiler cache must not be a symlink.' >&2
      exit 1
    fi
    mkdir -p "${compiler_cache}"
    # This dedicated non-secret leaf supports Docker user-namespace remapping.
    chmod 1777 "${compiler_cache}"
    printf '%s\n' 'Compiler cache: enabled (validation image scoped)'
  else
    printf '%s\n' 'Compiler cache unavailable; using disposable compilation.' >&2
  fi
fi
