#!/bin/sh
set -eu

expiry=$(date -u -d '+1 day' '+%Y-%m-%dT%H:%M:%SZ')

umask 077
mode=${1}
arch=${2}

wait_for() {
  label=${1}
  shift
  deadline=$(($(date +%s) + 30))
  while [ "$(date +%s)" -lt "${deadline}" ]; do
    remaining=$((deadline - $(date +%s)))
    [ "${remaining}" -gt 0 ] || break
    if timeout --signal=KILL "${remaining}" "${@}" >/dev/null 2>&1 && [ "$(date +%s)" -lt "${deadline}" ]; then
      return
    fi
    sleep 0.1
  done
  printf 'Timed out waiting for %s\n' "${label}" >&2
  exit 1
}
ready() {
  wait_for 'diagnostics socket' test -S /run/hostlens/diagnostics.sock
  wait_for 'gateway administrative readiness' hostlens status --system
  /opt/hostlens-smoke --ready
}

assert_fails() {
  if "${@}"; then
    printf 'Expected command to fail: %s\n' "${*}" >&2
    exit 1
  fi
}

printf '%s\n' 'HOSTLENS_STAGE: check interrupted install cleanup'
# Interrupt before unit creation, preserving identities that predate the plan.
groupadd --system hostlens-diagnostics
useradd --system --no-create-home --gid hostlens-diagnostics \
  --home-dir /nonexistent --shell /usr/sbin/nologin hostlens-diagnostics
cp -a /opt/hostlens-release /opt/interrupted-release
rm /opt/interrupted-release/profiles/nginx.yaml
if /opt/hostlens-release/hostlens install \
  --source /opt/interrupted-release --privilege "${mode}" --apply >/tmp/interrupted-plan.json 2>/tmp/interrupted.err; then
  printf '%s\n' 'interrupted fixture unexpectedly installed' >&2
  exit 1
fi
test ! -e /etc/systemd/system/hostlens-gateway.service
printf 'preserve me\n' >/etc/hostlens/external.conf
if /opt/hostlens-release/hostlens uninstall --apply >/tmp/interrupted-cleanup.json 2>/tmp/interrupted-cleanup.err; then
  printf '%s\n' 'external cleanup conflict ignored' >&2
  exit 1
fi
getent passwd hostlens-diagnostics
getent group hostlens-diagnostics
test -f /etc/hostlens/external.conf
rm /etc/hostlens/external.conf
/opt/hostlens-release/hostlens uninstall --apply >/tmp/interrupted-retry.json
test ! -e /var/lib/hostlens/install.json
getent passwd hostlens-diagnostics
getent group hostlens-diagnostics
userdel hostlens-diagnostics
if getent group hostlens-diagnostics >/dev/null; then
  groupdel hostlens-diagnostics
fi
assert_fails getent passwd hostlens-gateway
printf '%s\n' 'systemd interrupted-install cleanup and identity preservation: passed'
printf '%s\n' 'HOSTLENS_STAGE: preserve existing private groups'
# A newly created user must not let userdel remove a pre-existing private group.
groupadd --system hostlens-gateway
if /opt/hostlens-release/hostlens install \
  --source /opt/interrupted-release --privilege "${mode}" --apply >/tmp/group-only-plan.json 2>/tmp/group-only-install.err; then
  printf '%s\n' 'interrupted group-only fixture unexpectedly installed' >&2
  exit 1
fi
if /opt/hostlens-release/hostlens uninstall --apply >/tmp/group-only-cleanup.json 2>/tmp/group-only-cleanup.err; then
  printf '%s\n' 'pre-existing private group was not protected' >&2
  exit 1
fi
getent passwd hostlens-gateway
getent group hostlens-gateway
# Explicit fixture administration resolves the preserved mixed identity state.
userdel hostlens-gateway
if getent group hostlens-gateway >/dev/null; then
  groupdel hostlens-gateway
fi
/opt/hostlens-release/hostlens uninstall --apply >/tmp/group-only-retry.json
test ! -e /var/lib/hostlens/install.json
printf '%s\n' 'systemd pre-existing private-group protection: passed'

printf '%s\n' 'HOSTLENS_STAGE: install and start services'
/opt/hostlens-release/hostlens install --source /opt/hostlens-release --privilege "${mode}" --apply >/tmp/install-plan.json
assert_fails systemctl is-active --quiet hostlens-gateway.service
assert_fails systemctl is-active --quiet hostlens-diagnostics.service
hostlens token create --system --name acceptance --roles diagnostics --expires "${expiry}" >/tmp/token.json
printf 'fixture: visible\n' >/opt/hostlens-fixture.conf
chmod 0644 /opt/hostlens-fixture.conf
cat >/etc/hostlens/profiles/acceptance.yaml <<'YAML'
allow:
  files: [/opt/hostlens-fixture.conf]
  journal: [hostlens-fixture.service]
YAML
chmod 0644 /etc/hostlens/profiles/acceptance.yaml
sed -i 's/profiles: \[\]/profiles: [acceptance]/' /etc/hostlens/config.yaml
systemd-analyze verify /etc/systemd/system/hostlens-gateway.service /etc/systemd/system/hostlens-diagnostics.service
systemctl start hostlens-diagnostics.service hostlens-gateway.service
ready
systemctl is-active --quiet hostlens-diagnostics.service hostlens-gateway.service
printf '%s\n' 'HOSTLENS_STAGE: reject unauthorized IPC peers despite socket access'
# Grant only the socket group so connect succeeds and SO_PEERCRED must reject.
for socket in /run/hostlens/diagnostics.sock /run/hostlens/admin.sock; do
  setpriv --reuid=nobody --regid=hostlens-gateway --clear-groups \
    /opt/hostlens-containment ipc "${socket}" deny
done
# Rejections must leave both listeners usable by their authorized identities.
setpriv --reuid=hostlens-gateway --regid=hostlens-gateway --clear-groups \
  /opt/hostlens-containment ipc /run/hostlens/diagnostics.sock allow
/opt/hostlens-containment ipc /run/hostlens/admin.sock allow
hostlens status --system >/tmp/status.json
/opt/hostlens-smoke /tmp/token.json get_os_info "${arch}"
/opt/hostlens-smoke /tmp/token.json read_config visible
/opt/hostlens-smoke /tmp/token.json get_inventory "${arch}"
/opt/hostlens-smoke /tmp/token.json list_packages systemd
printf '%s\n' 'HOSTLENS_STAGE: verify journal access'
systemd-run --wait --unit=hostlens-fixture /bin/echo HOSTLENS_LOG_FIXTURE
journalctl --sync
if [ "${mode}" = restricted ]; then
  /opt/hostlens-smoke /tmp/token.json query_logs collection_failed --expect-error
  usermod -aG systemd-journal hostlens-diagnostics
  systemctl restart hostlens-diagnostics.service
  ready
fi
/opt/hostlens-smoke /tmp/token.json query_logs HOSTLENS_LOG_FIXTURE
printf '%s\n' 'HOSTLENS_STAGE: verify privilege and secret containment'
# Probe the service mount view and observed capabilities independently of policy.
printf 'positive control\n' >/opt/hostlens-root-private.conf
chmod 0600 /opt/hostlens-root-private.conf
pid=$(systemctl show --property=MainPID --value hostlens-diagnostics.service)
if [ "${mode}" = standard ]; then
  nsenter --target "${pid}" --mount -- setpriv --reuid=hostlens-diagnostics --regid=hostlens-gateway \
    --init-groups \
    --inh-caps=+dac_read_search \
    --ambient-caps=+dac_read_search /opt/hostlens-containment "${mode}" /etc/hostlens/secrets/tokens.json
else
  nsenter --target "${pid}" --mount -- setpriv --reuid=hostlens-diagnostics --regid=hostlens-gateway \
    --init-groups /opt/hostlens-containment "${mode}" /etc/hostlens/secrets/tokens.json
fi
if [ "${mode}" = standard ]; then
  mkdir -p /opt/custom-secret /etc/systemd/system/hostlens-diagnostics.service.d
  printf 'custom key fixture\n' >/opt/custom-secret/key.pem
  chmod 0600 /opt/custom-secret/key.pem
  rm /opt/hostlens-fixture.conf
  ln /opt/custom-secret/key.pem /opt/hostlens-fixture.conf
  sed -i 's|key_file: ""|key_file: /opt/custom-secret/key.pem|' /etc/hostlens/config.yaml
  printf '[Service]\nRestart=no\n' >/etc/systemd/system/hostlens-diagnostics.service.d/acceptance.conf
  systemctl daemon-reload
  systemctl restart hostlens-diagnostics.service
  wait_for 'diagnostics rejection' systemctl is-failed --quiet hostlens-diagnostics.service
  printf 'InaccessiblePaths=/opt/custom-secret/key.pem\n' >>/etc/systemd/system/hostlens-diagnostics.service.d/acceptance.conf
  systemctl daemon-reload
  systemctl start hostlens-diagnostics.service
  ready
  systemctl is-active --quiet hostlens-diagnostics.service
  systemctl restart hostlens-gateway.service
  ready
  /opt/hostlens-smoke /tmp/token.json read_config source_denied_or_unavailable --expect-error
  pid=$(systemctl show --property=MainPID --value hostlens-diagnostics.service)
  nsenter --target "${pid}" --mount -- setpriv --reuid=hostlens-diagnostics --regid=hostlens-gateway \
    --init-groups \
    --inh-caps=+dac_read_search --ambient-caps=+dac_read_search /opt/hostlens-containment "${mode}" /opt/custom-secret/key.pem
  sed -i 's|key_file: /opt/custom-secret/key.pem|key_file: ""|' /etc/hostlens/config.yaml
  rm /etc/systemd/system/hostlens-diagnostics.service.d/acceptance.conf
  rmdir /etc/systemd/system/hostlens-diagnostics.service.d
  systemctl daemon-reload
  systemctl restart hostlens-diagnostics.service
  rm /opt/hostlens-fixture.conf
  printf 'fixture: visible\n' >/opt/hostlens-fixture.conf
  chmod 0644 /opt/hostlens-fixture.conf
fi
printf '%s\n' 'HOSTLENS_STAGE: upgrade on configured endpoint'
# Upgrade readiness must follow the installed configuration, not defaults.
sed -i 's/port: 8080/port: 18080/' /etc/hostlens/config.yaml
systemctl restart hostlens-diagnostics.service hostlens-gateway.service
export HOSTLENS_SMOKE_ENDPOINT=http://127.0.0.1:18080/mcp
hostlens upgrade --archive /opt/candidate.tar.gz --apply
systemctl is-active --quiet hostlens-diagnostics.service hostlens-gateway.service
/opt/hostlens-smoke /tmp/token.json read_config visible
printf '%s\n' 'HOSTLENS_STAGE: remove tracked resources'
# Administrator-created profile is unexpected content and must be reported/preserved.
if hostlens uninstall --apply >/tmp/uninstall.json 2>/tmp/uninstall.err; then
  printf '%s\n' 'uninstall did not report external profile' >&2
  exit 1
fi
test -f /etc/hostlens/profiles/acceptance.yaml
rm /etc/hostlens/profiles/acceptance.yaml
/opt/hostlens-release/hostlens uninstall --apply >/tmp/uninstall-retry.json
test ! -e /var/lib/hostlens/install.json
assert_fails getent passwd hostlens-gateway
assert_fails getent passwd hostlens-diagnostics
printf 'systemd lifecycle %s: passed\n' "${mode}"
