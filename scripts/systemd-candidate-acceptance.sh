#!/bin/sh
set -eu

archive=${1:?archive path required}
version=${2:?version required}
mkdir -p /opt/hostlens-release
tar -xzf "$archive" -C /opt/hostlens-release
cd /opt/hostlens-release
./hostlens version | grep -Fx "$version"
./hostlens --help | grep -q 'migrate'
./hostlens uninstall --help | grep -q -- '--apply'
./hostlens token create --help | grep -q -- '--roles'
printf '%s\n' 'HOSTLENS_STAGE: preview and apply fresh installation'
./hostlens install --source . --start >/tmp/install-preview.json
test ! -e /usr/local/bin/hostlens
if getent passwd hostlens-gateway >/dev/null; then
  echo 'preview created the gateway identity' >&2
  exit 1
fi
plan=$(python3 -c 'import json; print(json.load(open("/tmp/install-preview.json"))["plan_digest"])')
cp profiles/host.yaml /tmp/host-profile.original
printf '%s\n' '# changed after verification' >>profiles/host.yaml
if ./hostlens install --source . --start --apply --plan "$plan" >/tmp/install-unexpected.json 2>/tmp/install-refusal.txt; then
  echo 'changed release member was installed' >&2
  exit 1
fi
cp /tmp/host-profile.original profiles/host.yaml
mkdir -m 0750 /etc/hostlens
if ./hostlens install --source . --start --apply --plan "$plan" >/tmp/install-unexpected.json 2>/tmp/install-refusal.txt; then
  echo 'changed installation plan was applied' >&2
  exit 1
fi
rmdir /etc/hostlens
./hostlens install --source . --start --apply --plan "$plan" >/tmp/install-result.json
/usr/local/bin/hostlens config validate
test -s /etc/hostlens/unit-provenance.json
systemctl is-active --quiet hostlens-gateway.service
systemctl is-enabled --quiet hostlens-gateway.service
wait_gateway() {
  attempt=0
  until [ "$(curl -sS --max-time 1 -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/mcp 2>/dev/null)" = 401 ]; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 30 ]; then
      echo 'gateway did not become ready' >&2
      exit 1
    fi
    sleep 0.2
  done
}
wait_gateway
restart_gateway() {
  systemctl reset-failed hostlens-gateway.service
  systemctl restart hostlens-gateway.service
}

observe=$(/usr/local/bin/hostlens token create --name acceptance --roles observe --expires never | python3 -c 'import json,sys; print(json.load(sys.stdin)["secret"])')
token_hash=$(sha256sum /etc/hostlens/secrets/tokens.json | cut -d' ' -f1)
if /usr/local/bin/hostlens token create --name missing-expiry --roles observe >/tmp/token-unexpected.json 2>/tmp/token-refusal.txt; then
  echo 'token without explicit expiry was created' >&2
  exit 1
fi
test "$token_hash" = "$(sha256sum /etc/hostlens/secrets/tokens.json | cut -d' ' -f1)"
if runuser -u hostlens-observer -- cat /etc/hostlens/secrets/tokens.json >/dev/null 2>&1; then
  echo 'observer can read token hashes' >&2
  exit 1
fi
request() {
  curl -fsS --max-time 10 -X POST http://127.0.0.1:8080/mcp \
    -H "Authorization: Bearer $1" -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    --data "$2"
}
request "$observe" '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | grep -q 'host_status'
request "$observe" '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"host_status","arguments":{}}}' | grep -q 'linux'
sed -i 's/active_profiles: \[host\]/active_profiles: [host, services-readonly, service-logs]/' /etc/hostlens/config.yaml
/usr/local/bin/hostlens config validate
/usr/local/bin/hostlens policy explain observe service_status hostlens-gateway.service | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["allowed"] and d["scope"]=="profile"'
restart_gateway
wait_gateway
request "$observe" '{"jsonrpc":"2.0","id":6,"method":"tools/list","params":{}}' | grep -q 'list_services'
printf '%s\n' 'HOSTLENS_STAGE: bounded service inventory'
request "$observe" '{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"list_services","arguments":{}}}' | python3 -c 'import json,sys; body=sys.stdin.read(); d=json.loads(next((line[6:] for line in body.splitlines() if line.startswith("data: ")), body)); assert "result" in d, "service inventory returned an RPC error"; assert not d["result"].get("isError"), "service inventory reported a tool error"; assert "hostlens-gateway.service" in json.dumps(d["result"]), "permitted gateway service missing"'
printf '%s\n' 'HOSTLENS_STAGE: service detail'
request "$observe" '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"service_status","arguments":{"name":"hostlens-gateway.service"}}}' | grep -q 'hostlens-gateway.service'
printf '%s\n' 'HOSTLENS_STAGE: real journal evidence with disposable group grant'
usermod -a -G systemd-journal hostlens-gateway
restart_gateway
wait_gateway
systemd-run --wait --unit=hostlens-journal-fixture /bin/sh -c 'echo HOSTLENS_JOURNAL_FIXTURE' >/dev/null
request "$observe" '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"service_logs","arguments":{"name":"hostlens-journal-fixture.service","limit":10}}}' | grep -q 'HOSTLENS_JOURNAL_FIXTURE'

cat >/etc/systemd/system/hostlens-test.service <<'UNIT'
[Unit]
Description=Disposable HostLens repair target
[Service]
ExecStart=/bin/sleep 600
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl start hostlens-test.service
sed -i 's/read_only: true/read_only: false/; s/active_profiles: \[host, services-readonly, service-logs\]/active_profiles: [host, services-readonly, service-logs, repair-example]/' /etc/hostlens/config.yaml
sed -i 's/example.service/hostlens-test.service/' /etc/hostlens/profiles/repair-example.yaml
cat >>/etc/hostlens/profiles/repair-example.yaml <<'PROFILE'
  - effect: repair
    tool: restart_service
    resource: hostlens-missing.service
PROFILE
/usr/local/bin/hostlens config validate
systemctl start hostlens-repair.service
restart_gateway
wait_gateway
repair=$(/usr/local/bin/hostlens token create --name repair-test --roles repair --expires never | python3 -c 'import json,sys; print(json.load(sys.stdin)["secret"])')
request "$repair" '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"restart_service","arguments":{"name":"hostlens-test.service"}}}' | grep -q 'hostlens-test.service'
systemctl is-active --quiet hostlens-test.service
request "$repair" '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"restart_service","arguments":{"name":"hostlens-missing.service"}}}' | grep -q '"isError":true'
attempt=0
until journalctl --no-pager -o cat -u hostlens-repair.service --since '-2min' | grep -q '"decision":"allowed"'; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 10 ]; then
    echo 'repair audit event missing' >&2
    exit 1
  fi
  sleep 0.2
done
if journalctl --no-pager -o cat -u hostlens-repair.service --since '-2min' | grep -Fq "$repair"; then
  echo 'bearer secret appeared in repair journal' >&2
  exit 1
fi
cp /etc/hostlens/config.yaml /tmp/repair-config.yaml
sed -i 's/read_only: false/read_only: true/; s|repair_socket: /run/hostlens-repair/repair.sock|repair_socket: ""|' /etc/hostlens/config.yaml
if timeout 5 runuser -u hostlens-gateway -- /usr/local/bin/hostlens serve --config /etc/hostlens/config.yaml >/tmp/readonly-unexpected.txt 2>&1; then
  echo 'read-only gateway activated while repair worker remained active' >&2
  exit 1
fi
grep -q 'repair worker is active' /tmp/readonly-unexpected.txt
cp /tmp/repair-config.yaml /etc/hostlens/config.yaml
chown root:hostlens-shared /etc/hostlens/config.yaml
chmod 0640 /etc/hostlens/config.yaml
systemctl stop hostlens-repair.service
sed -i 's/read_only: false/read_only: true/' /etc/hostlens/config.yaml
/usr/local/bin/hostlens config validate
restart_gateway
wait_gateway
request "$repair" '{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}' >/tmp/readonly-tools.json
if grep -q 'restart_service' /tmp/readonly-tools.json; then
  echo 'repair tool remained visible after read-only activation' >&2
  exit 1
fi

# A failed candidate activation leaves a restorable known-good configuration.
cp /etc/hostlens/config.yaml /opt/known-good.yaml
sed -i 's/version: 2/version: 999/' /etc/hostlens/config.yaml
if /usr/local/bin/hostlens config validate >/dev/null 2>&1; then
  echo 'invalid candidate was accepted' >&2
  exit 1
fi
cp /opt/known-good.yaml /etc/hostlens/config.yaml
chown root:hostlens-shared /etc/hostlens/config.yaml
chmod 0640 /etc/hostlens/config.yaml
restart_gateway
systemctl is-active --quiet hostlens-gateway.service
wait_gateway

config_before=$(sha256sum /etc/hostlens/config.yaml | cut -d' ' -f1)
tokens_before=$(sha256sum /etc/hostlens/secrets/tokens.json | cut -d' ' -f1)
printf '%s\n' 'HOSTLENS_STAGE: preserve state during installed upgrade'
./hostlens upgrade --source . >/tmp/upgrade-preview.json
upgrade_plan=$(python3 -c 'import json; print(json.load(open("/tmp/upgrade-preview.json"))["plan_digest"])')
cp /etc/systemd/system/hostlens-observer.service /tmp/observer-unit.original
printf '%s\n' '# modified by administrator' >>/etc/systemd/system/hostlens-observer.service
if ./hostlens upgrade --source . --apply --plan "$upgrade_plan" >/tmp/upgrade-unexpected.json 2>/tmp/upgrade-refusal.txt; then
  echo 'modified packaged unit was overwritten' >&2
  exit 1
fi
cp /tmp/observer-unit.original /etc/systemd/system/hostlens-observer.service
./hostlens upgrade --source . --apply --plan "$upgrade_plan" >/tmp/upgrade-result.json
systemctl is-active --quiet hostlens-gateway.service
test "$config_before" = "$(sha256sum /etc/hostlens/config.yaml | cut -d' ' -f1)"
test "$tokens_before" = "$(sha256sum /etc/hostlens/secrets/tokens.json | cut -d' ' -f1)"
if ls -d /etc/hostlens/.rollback-* >/dev/null 2>&1; then
  echo 'successful upgrade left a rollback copy' >&2
  exit 1
fi
wait_gateway

printf '%s\n' 'HOSTLENS_STAGE: upgrade across a packaged unit revision'
cp -a /opt/hostlens-release /opt/hostlens-unit-revision
printf '%s\n' '# next release unit revision' >>/opt/hostlens-unit-revision/systemd/hostlens-observer.service
python3 -c 'import hashlib,json; p="/opt/hostlens-unit-revision/release.json"; d=json.load(open(p)); d["checksums"]["systemd/hostlens-observer.service"]=hashlib.sha256(open("/opt/hostlens-unit-revision/systemd/hostlens-observer.service","rb").read()).hexdigest(); open(p,"w").write(json.dumps(d))'
revision_plan=$(/opt/hostlens-unit-revision/hostlens upgrade --source /opt/hostlens-unit-revision | python3 -c 'import json,sys; print(json.load(sys.stdin)["plan_digest"])')
/opt/hostlens-unit-revision/hostlens upgrade --source /opt/hostlens-unit-revision --apply --plan "$revision_plan" >/tmp/revision-result.json
grep -q 'next release unit revision' /etc/systemd/system/hostlens-observer.service
restore_plan=$(./hostlens upgrade --source . | python3 -c 'import json,sys; print(json.load(sys.stdin)["plan_digest"])')
./hostlens upgrade --source . --apply --plan "$restore_plan" >/tmp/restore-result.json
cmp systemd/hostlens-observer.service /etc/systemd/system/hostlens-observer.service
wait_gateway

binary_before=$(sha256sum /usr/local/bin/hostlens | cut -d' ' -f1)
provenance_before=$(sha256sum /etc/hostlens/unit-provenance.json | cut -d' ' -f1)
printf '%s\n' 'HOSTLENS_STAGE: restore installed bytes after failed activation'
cp -a /opt/hostlens-release /opt/hostlens-failure
printf '\000' >>/opt/hostlens-failure/hostlens
python3 -c 'import hashlib,json; p="/opt/hostlens-failure/release.json"; d=json.load(open(p)); d["checksums"]["hostlens"]=hashlib.sha256(open("/opt/hostlens-failure/hostlens","rb").read()).hexdigest(); open(p,"w").write(json.dumps(d))'
mkdir -p /opt/fake-bin
cat >/opt/fake-bin/systemctl <<'WRAPPER'
#!/bin/sh
if [ "$1" = start ] && [ "${2:-}" = hostlens-gateway.service ] && [ -e /tmp/fail-next-gateway-start ]; then
  rm /tmp/fail-next-gateway-start
  exit 1
fi
if [ "$1" = start ] && [ "${2:-}" = hostlens-gateway.service ] && [ -e /tmp/stop-next-gateway-start ]; then
  rm /tmp/stop-next-gateway-start
  /usr/bin/systemctl start hostlens-gateway.service || exit $?
  (sleep 0.2; /usr/bin/systemctl stop hostlens-gateway.service) >/dev/null 2>&1 &
  exit 0
fi
exec /usr/bin/systemctl "$@"
WRAPPER
chmod 0755 /opt/fake-bin/systemctl
systemctl reset-failed hostlens-gateway.service
failure_plan=$(PATH="/opt/fake-bin:$PATH" ./hostlens upgrade --source /opt/hostlens-failure | python3 -c 'import json,sys; print(json.load(sys.stdin)["plan_digest"])')
: >/tmp/fail-next-gateway-start
if PATH="/opt/fake-bin:$PATH" ./hostlens upgrade --source /opt/hostlens-failure --apply --plan "$failure_plan" >/tmp/upgrade-unexpected.json 2>/tmp/upgrade-failure.txt; then
  echo 'failed gateway activation was reported as successful' >&2
  exit 1
fi
test "$binary_before" = "$(sha256sum /usr/local/bin/hostlens | cut -d' ' -f1)"
test "$provenance_before" = "$(sha256sum /etc/hostlens/unit-provenance.json | cut -d' ' -f1)"
if ls -d /etc/hostlens/.rollback-* >/dev/null 2>&1; then
  echo 'restored upgrade left a rollback copy' >&2
  exit 1
fi
systemctl is-active --quiet hostlens-gateway.service
wait_gateway
printf '%s\n' 'HOSTLENS_STAGE: asynchronous activation failure rolls back'
systemctl reset-failed hostlens-gateway.service
failure_plan=$(PATH="/opt/fake-bin:$PATH" ./hostlens upgrade --source /opt/hostlens-failure | python3 -c 'import json,sys; print(json.load(sys.stdin)["plan_digest"])')
: >/tmp/stop-next-gateway-start
if PATH="/opt/fake-bin:$PATH" ./hostlens upgrade --source /opt/hostlens-failure --apply --plan "$failure_plan" >/tmp/upgrade-unexpected.json 2>/tmp/upgrade-failure.txt; then
  echo 'asynchronous gateway failure was reported as success' >&2
  exit 1
fi
test "$binary_before" = "$(sha256sum /usr/local/bin/hostlens | cut -d' ' -f1)"
systemctl is-active --quiet hostlens-gateway.service
wait_gateway

mkdir -p /backup
printf '%s\n' 'HOSTLENS_STAGE: bounded uninstall and retained administrator state'
if mount -t tmpfs tmpfs /backup; then
  printf '%s\n' 'HOSTLENS_STAGE: secondary tmpfs mount available for uninstall test'
else
  printf '%s\n' 'UNAVAILABLE: secondary mount denied by container; testing a large directory instead' >&2
fi
mkdir -p /backup/large-tree
for i in $(seq 1 1000); do : >"/backup/large-tree/file-$i"; done
/usr/local/bin/hostlens uninstall >/tmp/uninstall-preview.json
python3 -c 'import json; d=json.load(open("/tmp/uninstall-preview.json")); assert d["action"]=="preview"; assert len(d["remove"])==4; assert len(d["retain"])==4'
test -f /etc/systemd/system/hostlens-gateway.service
cp /etc/systemd/system/hostlens-repair.service /tmp/hostlens-repair-original.service
printf '%s\n' '# administrator change' >>/etc/systemd/system/hostlens-repair.service
if /usr/local/bin/hostlens uninstall --apply >/tmp/uninstall-unexpected.json 2>/tmp/uninstall-refusal.txt; then
  echo 'modified unit was removed' >&2
  exit 1
fi
grep -q 'refusing modified unit' /tmp/uninstall-refusal.txt
systemctl is-active --quiet hostlens-gateway.service
cp /tmp/hostlens-repair-original.service /etc/systemd/system/hostlens-repair.service
timeout 45 /usr/local/bin/hostlens uninstall --apply >/tmp/uninstall-result.json
python3 -c 'import json; d=json.load(open("/tmp/uninstall-result.json")); assert d["action"]=="removed"; assert len(d["remove"])==4'
test ! -e /usr/local/bin/hostlens
test ! -e /etc/systemd/system/hostlens-gateway.service
test -f /etc/hostlens/config.yaml
getent passwd hostlens-gateway >/dev/null
/opt/hostlens-release/hostlens uninstall --apply >/tmp/uninstall-retry.json
python3 -c 'import json; d=json.load(open("/tmp/uninstall-retry.json")); assert d["action"]=="removed"; assert d["remove"]==[]'
printf '%s\n' 'HOSTLENS_STAGE: reinstall after retained state'
sed -i 's/read_only: true/read_only: false/' /etc/hostlens/config.yaml
if ./hostlens install --source . --start >/tmp/reinstall-unexpected.json 2>/tmp/reinstall-refusal.txt; then
  echo 'reinstall preview accepted repair-capable gateway startup' >&2
  exit 1
fi
sed -i 's/read_only: false/read_only: true/' /etc/hostlens/config.yaml
config_before=$(sha256sum /etc/hostlens/config.yaml | cut -d' ' -f1)
reinstall_plan=$(./hostlens install --source . | python3 -c 'import json,sys; print(json.load(sys.stdin)["plan_digest"])')
./hostlens install --source . --apply --plan "$reinstall_plan" >/tmp/reinstall-result.json
test -x /usr/local/bin/hostlens
test "$config_before" = "$(sha256sum /etc/hostlens/config.yaml | cut -d' ' -f1)"
test "$tokens_before" = "$(sha256sum /etc/hostlens/secrets/tokens.json | cut -d' ' -f1)"
if systemctl is-active --quiet hostlens-gateway.service; then
  echo 'reinstall started the gateway without --start' >&2
  exit 1
fi
printf '%s\n' 'PASS: disposable systemd install, repair, read-only activation, upgrade rollback, bounded uninstall, and reinstall'
