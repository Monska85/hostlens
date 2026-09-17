#!/bin/sh
set -eu

expiry=$(date -u -d '+1 day' '+%Y-%m-%dT%H:%M:%SZ')

arch=${1}
application=${2:-}
run_binary() (
  if [ "${arch}" != "${HOSTLENS_CONTAINER_ARCH}" ]; then
    if [ "${HOSTLENS_QEMU_PRESERVE_ARGV0:-0}" = 1 ]; then
      # The binfmt image expects an explicit guest argv[0] after the executable.
      set -- /qemu-aarch64-static "${1}" "${@}"
    else
      set -- /qemu-aarch64-static "${@}"
    fi
  fi
  if [ -n "${probe_timeout:-}" ]; then
    set -- timeout --signal=KILL "${probe_timeout}" "${@}"
  fi
  exec "${@}"
)
printf '%s\n' 'HOSTLENS_STAGE: verify and extract release archive'
mkdir /tmp/candidate
ln -s "/archives/hostlens-${HOSTLENS_VERSION}-linux-${arch}.tar.gz" /tmp/candidate/
(cd /tmp/candidate && sha256sum --check --strict --ignore-missing /archives/checksums.txt)
/helpers/archive -archive "/archives/hostlens-${HOSTLENS_VERSION}-linux-${arch}.tar.gz" \
  -dest /tmp/release -arch "${arch}" -version "${HOSTLENS_VERSION}"
printf '%s\n' 'HOSTLENS_STAGE: prepare platform fixture'
mkdir -m 0700 /tmp/hostlens
cp "/tmp/release/config.example.yaml" /tmp/hostlens/config.yaml
sed -i 's/mode: system/mode: user/;s/privilege: standard/privilege: restricted/' /tmp/hostlens/config.yaml
cat >>/tmp/hostlens/config.yaml <<'YAML'
token_store: /tmp/hostlens/tokens.json
socket: /tmp/hostlens/diagnostics.sock
admin_socket: /tmp/hostlens/admin.sock
profile_dirs: []
YAML
if [ -n "${application}" ]; then
  printf '%s\n' 'denied fixture content' >/tmp/application-denied.conf
  cat >>/tmp/hostlens/config.yaml <<'YAML'
allow:
  audit: ['*']
  files: [/tmp/application.conf, /tmp/application.log, /tmp/application-denied.conf]
deny:
  files: [/tmp/application-denied.conf]
YAML
fi
cat /etc/os-release
printf '%s\n' 'HOSTLENS_STAGE: verify versions and configuration'
for binary in hostlens hostlens-diagnostics hostlens-docker-observer; do
  actual=$(run_binary "/tmp/release/${binary}" version)
  if [ "${actual}" != "${HOSTLENS_VERSION}" ]; then
    printf 'Version mismatch for %s: %s instead of %s\n' "${binary}" "${actual}" "${HOSTLENS_VERSION}" >&2
    exit 1
  fi
done
run_binary "/tmp/release/hostlens" config validate \
  --config /tmp/hostlens/config.yaml
printf '%s\n' 'HOSTLENS_STAGE: start authenticated services'
run_binary "/tmp/release/hostlens" token create \
  --config /tmp/hostlens/config.yaml --name platform --roles diagnostics --expires "${expiry}" >/tmp/hostlens/token.json
run_binary "/tmp/release/hostlens-diagnostics" serve \
  --config /tmp/hostlens/config.yaml >/tmp/hostlens/backend.log 2>&1 &
backend=${!}
run_binary "/tmp/release/hostlens" serve \
  --config /tmp/hostlens/config.yaml >/tmp/hostlens/gateway.log 2>&1 &
gateway=${!}
cleanup() {
  kill "${gateway}" "${backend}" "${application_pid:-}" 2>/dev/null || true
  wait "${gateway}" "${backend}" "${application_pid:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM
deadline=$(($(date +%s) + 30))
while [ "$(date +%s)" -lt "${deadline}" ]; do
  probe_timeout=$((deadline - $(date +%s)))
  [ "${probe_timeout}" -gt 0 ] || break
  if run_binary /tmp/release/hostlens status --config /tmp/hostlens/config.yaml >/dev/null 2>&1 && [ "$(date +%s)" -lt "${deadline}" ]; then
    break
  fi
  sleep 0.1
done
unset probe_timeout
if [ "$(date +%s)" -ge "${deadline}" ]; then
  printf '%s\n' 'Timed out waiting for gateway administrative readiness' >&2
  exit 1
fi
/helpers/smoke --ready
printf '%s\n' 'HOSTLENS_STAGE: inspect system inventory and packages'
"/helpers/smoke" /tmp/hostlens/token.json get_os_info "${arch}"
"/helpers/smoke" /tmp/hostlens/token.json get_inventory "${arch}"
# Discovery of the installed candidate must match the contract snapshot
# shipped inside the very archive under test, byte for byte.
"/helpers/smoke" --list /tmp/hostlens/token.json /tmp/release/tools.json
"/helpers/smoke" --reject /tmp/hostlens/token.json get_os_info
printf '%s\n' 'HOSTLENS_STAGE: one typed call per role'
run_binary "/tmp/release/hostlens" token create \
  --config /tmp/hostlens/config.yaml --name health --roles health --expires "${expiry}" >/tmp/hostlens/health.json
"/helpers/smoke" /tmp/hostlens/health.json get_os_info "${arch}"
if [ -z "${application}" ]; then
  "/helpers/smoke" /tmp/hostlens/token.json list_packages nonempty
  run_binary "/tmp/release/hostlens" token create \
    --config /tmp/hostlens/config.yaml --name inspect --roles inspect --expires "${expiry}" >/tmp/hostlens/inspect.json
  "/helpers/smoke" /tmp/hostlens/inspect.json list_packages nonempty
fi
printf '%s\n' 'HOSTLENS_STAGE: verify backend status'
run_binary "/tmp/release/hostlens" status \
  --config /tmp/hostlens/config.yaml
if [ -n "${application}" ]; then
  printf 'HOSTLENS_STAGE: start %s and investigate through generic MCP tools\n' "${application}"
  # The mounted fixture is linted independently.
  # shellcheck source=/dev/null
  . /application-fixture.sh
  /helpers/smoke --audit /tmp/hostlens/token.json /tmp/application-fixture.json
fi
printf 'platform smoke %s: passed\n' "${arch}"
