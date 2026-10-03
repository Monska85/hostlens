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

groupadd --system hostlens-shared
groupadd --system hostlens-tokens
useradd --system --no-create-home --gid hostlens-shared --shell /usr/sbin/nologin hostlens-gateway
useradd --system --no-create-home --gid hostlens-shared --shell /usr/sbin/nologin hostlens-observer
install -m 0755 hostlens /usr/local/bin/hostlens
install -d -m 0750 -o root -g hostlens-shared /etc/hostlens /etc/hostlens/profiles
install -d -m 0750 -o root -g hostlens-tokens /etc/hostlens/secrets
install -m 0640 -o root -g hostlens-shared config.example.yaml /etc/hostlens/config.yaml
install -m 0640 -o root -g hostlens-shared profiles/host.yaml /etc/hostlens/profiles/host.yaml
install -m 0640 -o root -g hostlens-shared profiles/repair-example.yaml /etc/hostlens/profiles/repair-example.yaml
sed -i "s/gateway_uid: 1001/gateway_uid: $(id -u hostlens-gateway)/; s/observer_uid: 1002/observer_uid: $(id -u hostlens-observer)/; s/shared_gid: 1001/shared_gid: $(getent group hostlens-shared | cut -d: -f3)/; s/token_gid: 1002/token_gid: $(getent group hostlens-tokens | cut -d: -f3)/" /etc/hostlens/config.yaml
/usr/local/bin/hostlens config validate
install -m 0644 systemd/hostlens-gateway.service /etc/systemd/system/hostlens-gateway.service
install -m 0644 systemd/hostlens-repair.service /etc/systemd/system/hostlens-repair.service
systemctl daemon-reload
systemctl enable --now hostlens-gateway.service
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

observe=$(/usr/local/bin/hostlens token create --name acceptance --roles observe --expires never | python3 -c 'import json,sys; print(json.load(sys.stdin)["secret"])')
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
sed -i 's/read_only: true/read_only: false/; s/active_profiles: \[host\]/active_profiles: [host, repair-example]/' /etc/hostlens/config.yaml
sed -i 's/example.service/hostlens-test.service/' /etc/hostlens/profiles/repair-example.yaml
cat >>/etc/hostlens/profiles/repair-example.yaml <<'PROFILE'
  - effect: repair
    tool: restart_service
    resource: hostlens-missing.service
PROFILE
/usr/local/bin/hostlens config validate
systemctl start hostlens-repair.service
systemctl restart hostlens-gateway.service
wait_gateway
repair=$(/usr/local/bin/hostlens token create --name repair-test --roles repair --expires never | python3 -c 'import json,sys; print(json.load(sys.stdin)["secret"])')
request "$repair" '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"restart_service","arguments":{"name":"hostlens-test.service"}}}' | grep -q 'hostlens-test.service'
systemctl is-active --quiet hostlens-test.service
request "$repair" '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"restart_service","arguments":{"name":"hostlens-missing.service"}}}' | grep -q '"isError":true'
systemctl stop hostlens-repair.service
sed -i 's/read_only: false/read_only: true/' /etc/hostlens/config.yaml
/usr/local/bin/hostlens config validate
systemctl restart hostlens-gateway.service
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
systemctl restart hostlens-gateway.service
systemctl is-active --quiet hostlens-gateway.service
wait_gateway

mkdir -p /backup
if mount -t tmpfs tmpfs /backup; then
  printf '%s\n' 'HOSTLENS_STAGE: secondary tmpfs mount available for uninstall test'
else
  printf '%s\n' 'UNAVAILABLE: secondary mount denied by container; testing a large directory instead' >&2
fi
mkdir -p /backup/large-tree
for i in $(seq 1 1000); do : >"/backup/large-tree/file-$i"; done
/usr/local/bin/hostlens uninstall >/tmp/uninstall-preview.json
python3 -c 'import json; d=json.load(open("/tmp/uninstall-preview.json")); assert d["action"]=="preview"; assert len(d["remove"])==3; assert len(d["retain"])==4'
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
python3 -c 'import json; d=json.load(open("/tmp/uninstall-result.json")); assert d["action"]=="removed"; assert len(d["remove"])==3'
test ! -e /usr/local/bin/hostlens
test ! -e /etc/systemd/system/hostlens-gateway.service
test -f /etc/hostlens/config.yaml
getent passwd hostlens-gateway >/dev/null
/opt/hostlens-release/hostlens uninstall --apply >/tmp/uninstall-retry.json
python3 -c 'import json; d=json.load(open("/tmp/uninstall-retry.json")); assert d["action"]=="removed"; assert d["remove"]==[]'
printf '%s\n' 'PASS: disposable systemd install, repair, read-only activation, rollback, and bounded uninstall'
