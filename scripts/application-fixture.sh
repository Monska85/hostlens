#!/bin/sh
# Sourced only by disposable archive acceptance. Applications bind to isolated loopback.
application=${application:?application fixture required}
application_failure() {
  application_status=${1}
  printf 'Application fixture %s failed (exit %s): bounded startup diagnostics\n' "${application}" "${application_status}" >&2
  for application_log in /tmp/application-init.log /tmp/application.log; do
    if [ -f "${application_log}" ]; then
      printf '%s\n' "${application_log}" >&2
      tail -c 4096 "${application_log}" | tail -n 20 >&2 || true
    fi
  done
  exit "${application_status}"
}
case "${application}" in
  postgres)
    initdb -D /tmp/application-data --auth=trust >/tmp/application-init.log 2>&1 || application_failure "${?}"
    cat >/tmp/application.conf <<'CONFIG'
data_directory = '/tmp/application-data'
hba_file = '/tmp/application-data/pg_hba.conf'
ident_file = '/tmp/application-data/pg_ident.conf'
listen_addresses = '127.0.0.1'
port = 15432
unix_socket_directories = '/tmp'
CONFIG
    postgres -c config_file=/tmp/application.conf >/tmp/application.log 2>&1 &
    application_pid=${!}
    application_process=postgres
    application_port=15432
    ;;
  nginx)
    cat >/tmp/application.conf <<'CONFIG'
pid /tmp/application.pid;
error_log /tmp/application.log notice;
events { worker_connections 32; }
http {
  access_log off;
  client_body_temp_path /tmp/nginx-body;
  proxy_temp_path /tmp/nginx-proxy;
  fastcgi_temp_path /tmp/nginx-fastcgi;
  uwsgi_temp_path /tmp/nginx-uwsgi;
  scgi_temp_path /tmp/nginx-scgi;
  server { listen 127.0.0.1:18081; location / { return 200 'fixture'; } }
}
CONFIG
    nginx -c /tmp/application.conf -g 'daemon off;' >>/tmp/application.log 2>&1 &
    application_pid=${!}
    application_process=nginx
    application_port=18081
    ;;
  apache)
    cat >/tmp/application.conf <<'CONFIG'
ServerRoot "/usr/local/apache2"
Listen 127.0.0.1:18082
LoadModule mpm_event_module modules/mod_mpm_event.so
LoadModule authz_core_module modules/mod_authz_core.so
LoadModule unixd_module modules/mod_unixd.so
ServerName localhost
PidFile /tmp/application.pid
ErrorLog /tmp/application.log
LogLevel notice
DocumentRoot "/usr/local/apache2/htdocs"
<Directory "/usr/local/apache2/htdocs">
Require all granted
</Directory>
CONFIG
    httpd -f /tmp/application.conf -DFOREGROUND >>/tmp/application.log 2>&1 &
    application_pid=${!}
    application_process=httpd
    application_port=18082
    ;;
  mysql)
    mkdir /tmp/application-data
    mysqld --no-defaults --initialize-insecure --datadir=/tmp/application-data >/tmp/application-init.log 2>&1 || application_failure "${?}"
    cat >/tmp/application.conf <<'CONFIG'
[mysqld]
datadir=/tmp/application-data
socket=/tmp/application.sock
pid-file=/tmp/application.pid
bind-address=127.0.0.1
port=13306
mysqlx=0
log-error=/tmp/application.log
CONFIG
    mysqld --defaults-file=/tmp/application.conf >>/tmp/application.log 2>&1 &
    application_pid=${!}
    application_process=mysqld
    application_port=13306
    ;;
  *)
    printf 'Unsupported application fixture: %s\n' "${application}" >&2
    exit 2
    ;;
esac
export application_pid
/helpers/smoke --tcp-ready "${application_port}" || application_failure "${?}"
printf '{"process":"%s","port":%s,"config":"/tmp/application.conf","log":"/tmp/application.log"}\n' \
  "${application_process}" "${application_port}" >/tmp/application-fixture.json
