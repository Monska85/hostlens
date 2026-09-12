#!/bin/sh
# Sourced by container launchers after selecting image and repository.
# Only an explicitly prepared, private directory may receive compiler outputs.
compiler_cache=
if [ -n "${HOSTLENS_BUILD_CACHE:-}" ]; then
  if [ -d "${HOSTLENS_BUILD_CACHE}" ] && [ -w "${HOSTLENS_BUILD_CACHE}" ]; then
    cache_root=$(CDPATH='' cd -- "${HOSTLENS_BUILD_CACHE}" && pwd -P)
    if ! python3 -c 'import os, stat, sys; s = os.stat(sys.argv[1]); sys.exit(s.st_uid != os.getuid() or stat.S_IMODE(s.st_mode) != 0o700)' "${cache_root}"; then
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
    # Hosted cache restoration changes ownership to the runner. Containers drop
    # DAC override and may use remapped UIDs, so restored entries need write
    # access too. The mode-0700 parent keeps this storage private on the host.
    # Leave container-owned entries alone: local users cannot chmod those.
    find "${compiler_cache}" -user "$(id -u)" -type d -exec chmod a+rwx {} +
    find "${compiler_cache}" -user "$(id -u)" -type f -exec chmod a+rw {} +
    printf '%s\n' 'Compiler cache: enabled (validation image scoped)'
  else
    printf '%s\n' 'Compiler cache unavailable; using disposable compilation.' >&2
  fi
fi
