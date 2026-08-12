#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-tenant-domains-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13407}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26457}"
DNS_PORT="${MOCHAT_DNS_PORT:-15357}"
BRIDGE_PORT="${MOCHAT_DOMAIN_BRIDGE_PORT:-18357}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18181}"
DATABASE="mochat_saas_tenant_domains"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-tenant-domains.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
DNS_LOG="$WORK_DIR/dns.log"
DNS_VALUE_FILE="$WORK_DIR/dns-value.txt"
BRIDGE_LOG="$WORK_DIR/bridge.log"
BRIDGE_MODE_FILE="$WORK_DIR/bridge-mode.txt"
GO_PID=""
DNS_PID=""
BRIDGE_PID=""
STACK_STARTED=0
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-tenant-domain-jwt-secret}"
IDENTITY_KEY="${MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY:-6262626262626262626262626262626262626262626262626262626262626262}"
BRIDGE_TOKEN="${MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN:-domain-delivery-bridge-token}"
CALLBACK_SECRET="${MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET:-0123456789abcdef0123456789abcdef}"
DOMAIN_ONE="login.customer.test"
DOMAIN_TWO="app.customer.test"
DOMAIN_DELETE="delete.customer.test"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$DNS_PID" ] && kill -0 "$DNS_PID" 2>/dev/null; then
    kill "$DNS_PID" 2>/dev/null || true
    wait "$DNS_PID" 2>/dev/null || true
  fi
  if [ -n "$BRIDGE_PID" ] && kill -0 "$BRIDGE_PID" 2>/dev/null; then
    kill "$BRIDGE_PID" 2>/dev/null || true
    wait "$BRIDGE_PID" 2>/dev/null || true
  fi
  if [ "$STACK_STARTED" = "1" ] && [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK_DIR"
}
trap 'echo "SaaS tenant domains smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true; tail -80 "$DNS_LOG" >&2 || true; tail -80 "$BRIDGE_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_tcp_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "TCP port $port is already in use" >&2
    exit 1
  fi
}

assert_udp_port_free() {
  local port="$1"
  if lsof -nP -iUDP:"$port" >/dev/null 2>&1; then
    echo "UDP port $port is already in use" >&2
    exit 1
  fi
}

wait_service_healthy() {
  local service="$1" deadline=$((SECONDS + 180))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    [ "$status" = "healthy" ] && return 0
    sleep 2
  done
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  return 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    [ "$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)" = "$expected" ] && return 0
    sleep 1
  done
  return 1
}

wait_mysql_value() {
  local query="$1" expected="$2" deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    [ "$(mysql_scalar "$query" 2>/dev/null || true)" = "$expected" ] && return 0
    sleep 1
  done
  echo "timed out waiting for MySQL value: $query => $expected" >&2
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

json_value() {
  local file="$1" expression="$2"
  python3 - "$file" "$expression" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split('.'):
    value = value[int(part)] if isinstance(value, list) else value[part]
if isinstance(value, bool):
    print(str(value).lower())
elif isinstance(value, (dict, list)):
    print(json.dumps(value, ensure_ascii=False, separators=(",", ":")))
else:
    print(value)
PY
}

login_token() {
  local phone="$1" password="$2" output="$3"
  curl -sS -f -H 'Content-Type: application/json' -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  json_value "$output" data.token
}

host_request() {
  local method="$1" host="$2" path="$3" body="$4" output="$5" expected="$6"
  local args=(-sS -o "$output" -w '%{http_code}' -X "$method" -H "Host: $host")
  if [ -n "$body" ]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  local status
  status="$(curl "${args[@]}" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "$method $host$path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_write() {
  local method="$1" token="$2" path="$3" body="$4" output="$5" expected="${6:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "$method $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

cat >"$WORK_DIR/dns_server.py" <<'PY'
import pathlib
import socket
import struct
import sys

port = int(sys.argv[1])
value_file = pathlib.Path(sys.argv[2])
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind(("127.0.0.1", port))
while True:
    packet, address = sock.recvfrom(4096)
    if len(packet) < 17:
        continue
    offset = 12
    labels = []
    while offset < len(packet) and packet[offset]:
        size = packet[offset]
        offset += 1
        labels.append(packet[offset:offset + size].decode("ascii", "ignore"))
        offset += size
    offset += 1
    if offset + 4 > len(packet):
        continue
    question = packet[12:offset + 4]
    qtype, _ = struct.unpack("!HH", packet[offset:offset + 4])
    value = value_file.read_text(encoding="utf-8").strip() if value_file.exists() else ""
    answer_count = 1 if qtype == 16 and value and len(value.encode("utf-8")) <= 255 else 0
    header = packet[:2] + struct.pack("!HHHHH", 0x8180, 1, answer_count, 0, 0)
    response = header + question
    if answer_count:
        encoded = value.encode("utf-8")
        rdata = bytes([len(encoded)]) + encoded
        response += b"\xc0\x0c" + struct.pack("!HHIH", 16, 1, 30, len(rdata)) + rdata
    sock.sendto(response, address)
PY

cat >"$WORK_DIR/domain_bridge.py" <<'PY'
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

port = int(sys.argv[1])
mode_file = pathlib.Path(sys.argv[2])
log_file = pathlib.Path(sys.argv[3])
token = sys.argv[4]

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        with log_file.open("ab") as handle:
            handle.write(json.dumps({
                "path": self.path,
                "authorization": self.headers.get("Authorization", ""),
                "eventId": self.headers.get("X-Mochat-Go-Event-ID", ""),
                "body": body.decode("utf-8", "replace"),
            }, ensure_ascii=False).encode("utf-8") + b"\n")
        if self.path != "/v1/domain-deliveries" or self.headers.get("Authorization") != "Bearer " + token:
            self.send_response(401)
            self.end_headers()
            return
        payload = json.loads(body)
        mode = mode_file.read_text(encoding="utf-8").strip() if mode_file.exists() else "ready"
        if mode == "fail_once":
            mode_file.write_text("ready", encoding="utf-8")
            self.send_response(500)
            self.end_headers()
            self.wfile.write(b"sensitive-upstream-body-must-not-persist")
            return
        action = payload.get("action", "")
        if mode == "accepted_once":
            mode_file.write_text("ready", encoding="utf-8")
            response = {"status": "accepted", "provider": "fake-ingress", "requestId": "async-" + payload["jobNo"], "routingStatus": "provisioning", "certificateStatus": "provisioning"}
        elif action == "disable":
            response = {"status": "accepted", "provider": "fake-ingress", "requestId": "disable-" + payload["jobNo"], "routingStatus": "disabled", "certificateStatus": "disabled"}
        elif action == "delete":
            response = {"status": "accepted", "provider": "fake-ingress", "requestId": "delete-" + payload["jobNo"], "routingStatus": "deleted", "certificateStatus": "deleted"}
        else:
            response = {"status": "ready", "provider": "fake-ingress", "requestId": "ready-" + payload["jobNo"], "routingStatus": "ready", "certificateStatus": "active", "certificateId": "cert-ref-" + str(payload["domain"]["id"]), "certificateNotBefore": "2026-07-01T00:00:00Z", "certificateExpiresAt": "2037-07-01T00:00:00Z"}
        encoded = json.dumps(response, separators=(",", ":")).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, *_):
        return

ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
PY

assert_tcp_port_free "$MYSQL_PORT"
assert_tcp_port_free "$REDIS_PORT"
assert_tcp_port_free "${GO_ADDR##*:}"
assert_udp_port_free "$DNS_PORT"
assert_tcp_port_free "$BRIDGE_PORT"

STACK_STARTED=1
compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis
mysql_root <<SQL
DROP DATABASE IF EXISTS $DATABASE;
CREATE DATABASE $DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON $DATABASE.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "98"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_tenant_domains'")" = "mochat_go_saas_tenant_domains"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_tenant_domain_delivery_jobs'")" = "mochat_go_saas_tenant_domain_delivery_jobs"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.domains.read'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.domains.manage'")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,域名治理平台,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
991,域名业务租户,13800000991,secret991,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000002', password, '平台运营', 0, '运营部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000003', password, '平台审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
SQL
OPERATIONS_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000002' ORDER BY id DESC LIMIT 1")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000003' ORDER BY id DESC LIMIT 1")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at) VALUES ($OPERATIONS_ID, 1, 1, NOW(), NOW()), ($AUDITOR_ID, 1, 1, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at) VALUES ($OPERATIONS_ID, $OPERATIONS_ROLE_ID, 1, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 1, NOW());
SQL

: >"$DNS_VALUE_FILE"
python3 "$WORK_DIR/dns_server.py" "$DNS_PORT" "$DNS_VALUE_FILE" >"$DNS_LOG" 2>&1 &
DNS_PID="$!"
sleep 1
kill -0 "$DNS_PID"

printf '%s' ready >"$BRIDGE_MODE_FILE"
python3 "$WORK_DIR/domain_bridge.py" "$BRIDGE_PORT" "$BRIDGE_MODE_FILE" "$BRIDGE_LOG" "$BRIDGE_TOKEN" &
BRIDGE_PID="$!"
sleep 1
kill -0 "$BRIDGE_PID"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY=1 \
  MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY" \
  MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID=tenant-domain-smoke \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER="127.0.0.1:$DNS_PORT" \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS=2 \
  MOCHAT_GO_ENABLE_SAAS_TENANT_DOMAIN_DELIVERY_CRON=1 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_INTERVAL_SECONDS=1 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_RUN_ON_START=1 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL="http://127.0.0.1:$BRIDGE_PORT" \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN="$BRIDGE_TOKEN" \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TIMEOUT_SECONDS=2 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL="http://$GO_ADDR/webhooks/saas/domain-delivery" \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET="$CALLBACK_SECRET" \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_TOLERANCE_SECONDS=300 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LIMIT=20 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LEASE_SECONDS=10 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_RETRY_DELAY_SECONDS=1 \
  MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_WAIT_SECONDS=10 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-login.json")"
OPERATIONS_TOKEN="$(login_token 13800000002 secret001 "$WORK_DIR/operations-login.json")"
AUDITOR_TOKEN="$(login_token 13800000003 secret001 "$WORK_DIR/auditor-login.json")"
TENANT_TOKEN="$(login_token 13800000991 secret991 "$WORK_DIR/tenant-login.json")"

api_get "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomains?status=all' "$WORK_DIR/operations-read.json"
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/tenantDomains?status=all' "$WORK_DIR/auditor-read.json"
api_get "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomainDeliveryJobs?status=all' "$WORK_DIR/operations-delivery-read.json"
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/tenantDomainDeliveryJobs?status=all' "$WORK_DIR/auditor-delivery-read.json"
api_write POST "$AUDITOR_TOKEN" '/dashboard/saasAdmin/tenantDomain' '{}' "$WORK_DIR/auditor-write.json" 403
api_write POST "$AUDITOR_TOKEN" '/dashboard/saasAdmin/tenantDomainDelivery' '{}' "$WORK_DIR/auditor-delivery-write.json" 403
api_get "$TENANT_TOKEN" '/dashboard/saasAdmin/tenantDomains?status=all' "$WORK_DIR/tenant-read.json" 403
api_get "$TENANT_TOKEN" '/dashboard/saasAdmin/tenantDomainDeliveryJobs?status=all' "$WORK_DIR/tenant-delivery-read.json" 403

BRANDING_BODY='{"tenantId":991,"status":"active","productName":"域名客户云","productShortName":"客户云","productSubtitle":"自定义域名客户运营","logoUrl":"/img/logo-no-word.c30823d0.png","faviconUrl":"/favicon.ico","loginBackgroundUrl":"/img/background.e06f03d5.png","primaryColor":"#1D4ED8","accentColor":"#1E40AF","supportUrl":"/support","docsUrl":"/docs","footerText":"域名客户云","expectedVersion":0}'
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/brandingProfile' "$BRANDING_BODY" "$WORK_DIR/branding.json"

api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"create\",\"tenantId\":991,\"hostname\":\"$DOMAIN_ONE\"}" "$WORK_DIR/domain-one-create.json" 201
DOMAIN_ONE_ID="$(json_value "$WORK_DIR/domain-one-create.json" data.domain.id)"
DOMAIN_ONE_TOKEN="$(json_value "$WORK_DIR/domain-one-create.json" data.domain.verificationToken)"
test "$(json_value "$WORK_DIR/domain-one-create.json" data.domain.isPrimary)" = "false"
test "$(json_value "$WORK_DIR/domain-one-create.json" data.domain.verificationRecordName)" = "_mochat.$DOMAIN_ONE"
test "$(json_value "$WORK_DIR/domain-one-create.json" data.domain.verificationRecordValue)" = "mochat-domain-verification=$DOMAIN_ONE_TOKEN"

host_request GET "$DOMAIN_ONE" '/security/login' '' "$WORK_DIR/domain-one-pending.html" 421
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"verify\",\"id\":$DOMAIN_ONE_ID,\"expectedVersion\":1}" "$WORK_DIR/domain-one-verify-failed.json" 422
test "$(json_value "$WORK_DIR/domain-one-verify-failed.json" data.domain.version)" = "2"
test -n "$(json_value "$WORK_DIR/domain-one-verify-failed.json" data.domain.verificationError)"

printf '%s' "mochat-domain-verification=$DOMAIN_ONE_TOKEN" >"$DNS_VALUE_FILE"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"verify\",\"id\":$DOMAIN_ONE_ID,\"expectedVersion\":2}" "$WORK_DIR/domain-one-verified.json"
test "$(json_value "$WORK_DIR/domain-one-verified.json" data.domain.status)" = "active"
test "$(json_value "$WORK_DIR/domain-one-verified.json" data.domain.isPrimary)" = "true"

host_request GET "$DOMAIN_ONE" '/security/login' '' "$WORK_DIR/domain-one-login.html" 200
grep -q '域名客户云' "$WORK_DIR/domain-one-login.html"
host_request POST "$DOMAIN_ONE" '/dashboard/user/auth' '{"phone":"13800000991","password":"secret991"}' "$WORK_DIR/domain-one-tenant-auth.json" 200
test -n "$(json_value "$WORK_DIR/domain-one-tenant-auth.json" data.token)"
test "$(mysql_scalar "SELECT tenant_id FROM mochat_go_saas_identity_sessions WHERE user_id = (SELECT id FROM mc_user WHERE phone = '13800000991' LIMIT 1) ORDER BY id DESC LIMIT 1")" = "991"
host_request POST "$DOMAIN_ONE" '/dashboard/user/auth' '{"phone":"13800000001","password":"secret001"}' "$WORK_DIR/domain-one-platform-auth.json" 401

api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"create\",\"tenantId\":1,\"hostname\":\"$DOMAIN_ONE\"}" "$WORK_DIR/domain-conflict.json" 409
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"create\",\"tenantId\":991,\"hostname\":\"$DOMAIN_TWO\"}" "$WORK_DIR/domain-two-create.json" 201
DOMAIN_TWO_ID="$(json_value "$WORK_DIR/domain-two-create.json" data.domain.id)"
DOMAIN_TWO_TOKEN="$(json_value "$WORK_DIR/domain-two-create.json" data.domain.verificationToken)"
printf '%s' "mochat-domain-verification=$DOMAIN_TWO_TOKEN" >"$DNS_VALUE_FILE"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"verify\",\"id\":$DOMAIN_TWO_ID,\"expectedVersion\":1}" "$WORK_DIR/domain-two-verified.json"
test "$(json_value "$WORK_DIR/domain-two-verified.json" data.domain.isPrimary)" = "false"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"set_primary\",\"id\":$DOMAIN_TWO_ID,\"expectedVersion\":2}" "$WORK_DIR/domain-two-primary.json"
test "$(json_value "$WORK_DIR/domain-two-primary.json" data.domain.isPrimary)" = "true"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domains WHERE tenant_id = 991 AND is_primary = 1 AND deleted_at IS NULL")" = "1"

api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"disable\",\"id\":$DOMAIN_TWO_ID,\"expectedVersion\":3}" "$WORK_DIR/domain-two-disabled.json"
host_request GET "$DOMAIN_TWO" '/security/login' '' "$WORK_DIR/domain-two-disabled.html" 421
test "$(mysql_scalar "SELECT hostname FROM mochat_go_saas_tenant_domains WHERE tenant_id = 991 AND is_primary = 1 AND deleted_at IS NULL")" = "$DOMAIN_ONE"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"enable\",\"id\":$DOMAIN_TWO_ID,\"expectedVersion\":4}" "$WORK_DIR/domain-two-enabled.json"
host_request GET "$DOMAIN_TWO" '/security/login' '' "$WORK_DIR/domain-two-enabled.html" 200

api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"rotate_token\",\"id\":$DOMAIN_TWO_ID,\"expectedVersion\":5}" "$WORK_DIR/domain-two-rotated.json"
ROTATED_TOKEN="$(json_value "$WORK_DIR/domain-two-rotated.json" data.domain.verificationToken)"
test "$ROTATED_TOKEN" != "$DOMAIN_TWO_TOKEN"
host_request GET "$DOMAIN_TWO" '/security/login' '' "$WORK_DIR/domain-two-rotated.html" 421
printf '%s' "mochat-domain-verification=$ROTATED_TOKEN" >"$DNS_VALUE_FILE"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"verify\",\"id\":$DOMAIN_TWO_ID,\"expectedVersion\":6}" "$WORK_DIR/domain-two-reverified.json"
host_request POST "$DOMAIN_TWO" '/dashboard/user/auth' '{"phone":"13800000991","password":"secret991"}' "$WORK_DIR/domain-two-tenant-auth.json" 200

api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"create\",\"tenantId\":991,\"hostname\":\"$DOMAIN_DELETE\"}" "$WORK_DIR/domain-delete-create.json" 201
DOMAIN_DELETE_ID="$(json_value "$WORK_DIR/domain-delete-create.json" data.domain.id)"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"delete\",\"id\":$DOMAIN_DELETE_ID,\"expectedVersion\":1}" "$WORK_DIR/domain-delete.json"
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomain' "{\"action\":\"create\",\"tenantId\":1,\"hostname\":\"$DOMAIN_DELETE\"}" "$WORK_DIR/domain-rebind.json" 201

wait_mysql_value "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE active_domain_id IS NOT NULL" "0"
test "$(mysql_scalar "SELECT delivery_status FROM mochat_go_saas_tenant_domain_deliveries WHERE domain_id = $DOMAIN_ONE_ID")" = "ready"
test "$(mysql_scalar "SELECT delivery_status FROM mochat_go_saas_tenant_domain_deliveries WHERE domain_id = $DOMAIN_TWO_ID")" = "ready"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE status = 'succeeded'")" -ge "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE status IN ('succeeded', 'canceled')")" -ge "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE status = 'failed'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE status = 'canceled' AND last_error <> 'superseded by a newer domain lifecycle action'")" = "0"

printf '%s' accepted_once >"$BRIDGE_MODE_FILE"
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomainDelivery' "{\"action\":\"refresh\",\"domainId\":$DOMAIN_ONE_ID}" "$WORK_DIR/delivery-async-create.json" 201
ASYNC_JOB_ID="$(json_value "$WORK_DIR/delivery-async-create.json" data.job.id)"
ASYNC_JOB_NO="$(json_value "$WORK_DIR/delivery-async-create.json" data.job.jobNo)"
wait_mysql_value "SELECT status FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE id = $ASYNC_JOB_ID" "waiting"

CALLBACK_TIMESTAMP="$(date +%s)"
CALLBACK_OCCURRED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
cat >"$WORK_DIR/domain-callback.json" <<JSON
{"eventId":"domain-event-1","eventType":"delivery.status","jobNo":"$ASYNC_JOB_NO","domainId":$DOMAIN_ONE_ID,"hostname":"$DOMAIN_ONE","status":"ready","provider":"fake-ingress","requestId":"async-$ASYNC_JOB_NO","routingStatus":"ready","certificateStatus":"active","certificateId":"cert-ref-async-$DOMAIN_ONE_ID","certificateNotBefore":"2026-07-01T00:00:00Z","certificateExpiresAt":"2037-07-01T00:00:00Z","occurredAt":"$CALLBACK_OCCURRED_AT"}
JSON
CALLBACK_SIGNATURE="$(python3 - "$CALLBACK_SECRET" "$CALLBACK_TIMESTAMP" "$WORK_DIR/domain-callback.json" <<'PY'
import hashlib, hmac, pathlib, sys
secret, timestamp, path = sys.argv[1], sys.argv[2], pathlib.Path(sys.argv[3])
print(hmac.new(secret.encode(), timestamp.encode() + b"\n" + path.read_bytes(), hashlib.sha256).hexdigest())
PY
)"
status="$(curl -sS -o "$WORK_DIR/domain-callback-bad-signature.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "X-Mochat-Go-Domain-Timestamp: $CALLBACK_TIMESTAMP" -H 'X-Mochat-Go-Event-ID: domain-event-1' -H 'X-Mochat-Go-Domain-Signature: v1=bad' --data-binary "@$WORK_DIR/domain-callback.json" "http://$GO_ADDR/webhooks/saas/domain-delivery")"
test "$status" = "401"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_events WHERE event_id = 'domain-event-1'")" = "0"

status="$(curl -sS -o "$WORK_DIR/domain-callback-valid.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "X-Mochat-Go-Domain-Timestamp: $CALLBACK_TIMESTAMP" -H 'X-Mochat-Go-Event-ID: domain-event-1' -H "X-Mochat-Go-Domain-Signature: v1=$CALLBACK_SIGNATURE" --data-binary "@$WORK_DIR/domain-callback.json" "http://$GO_ADDR/webhooks/saas/domain-delivery")"
test "$status" = "200"
test "$(json_value "$WORK_DIR/domain-callback-valid.json" data.duplicate)" = "false"
wait_mysql_value "SELECT status FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE id = $ASYNC_JOB_ID" "succeeded"
test "$(mysql_scalar "SELECT certificate_id FROM mochat_go_saas_tenant_domain_deliveries WHERE domain_id = $DOMAIN_ONE_ID")" = "cert-ref-async-$DOMAIN_ONE_ID"

status="$(curl -sS -o "$WORK_DIR/domain-callback-replay.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "X-Mochat-Go-Domain-Timestamp: $CALLBACK_TIMESTAMP" -H 'X-Mochat-Go-Event-ID: domain-event-1' -H "X-Mochat-Go-Domain-Signature: v1=$CALLBACK_SIGNATURE" --data-binary "@$WORK_DIR/domain-callback.json" "http://$GO_ADDR/webhooks/saas/domain-delivery")"
test "$status" = "200"
test "$(json_value "$WORK_DIR/domain-callback-replay.json" data.duplicate)" = "true"

sed 's/cert-ref-async-/cert-ref-conflict-/' "$WORK_DIR/domain-callback.json" >"$WORK_DIR/domain-callback-conflict.json"
CONFLICT_SIGNATURE="$(python3 - "$CALLBACK_SECRET" "$CALLBACK_TIMESTAMP" "$WORK_DIR/domain-callback-conflict.json" <<'PY'
import hashlib, hmac, pathlib, sys
secret, timestamp, path = sys.argv[1], sys.argv[2], pathlib.Path(sys.argv[3])
print(hmac.new(secret.encode(), timestamp.encode() + b"\n" + path.read_bytes(), hashlib.sha256).hexdigest())
PY
)"
status="$(curl -sS -o "$WORK_DIR/domain-callback-conflict-response.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "X-Mochat-Go-Domain-Timestamp: $CALLBACK_TIMESTAMP" -H 'X-Mochat-Go-Event-ID: domain-event-1' -H "X-Mochat-Go-Domain-Signature: v1=$CONFLICT_SIGNATURE" --data-binary "@$WORK_DIR/domain-callback-conflict.json" "http://$GO_ADDR/webhooks/saas/domain-delivery")"
test "$status" = "409"

printf '%s' fail_once >"$BRIDGE_MODE_FILE"
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomainDelivery' "{\"action\":\"refresh\",\"domainId\":$DOMAIN_ONE_ID}" "$WORK_DIR/delivery-retry-create.json" 201
RETRY_JOB_ID="$(json_value "$WORK_DIR/delivery-retry-create.json" data.job.id)"
wait_mysql_value "SELECT status FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE id = $RETRY_JOB_ID" "succeeded"
test "$(mysql_scalar "SELECT attempts FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE id = $RETRY_JOB_ID")" -ge "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_jobs WHERE last_error LIKE '%sensitive-upstream-body-must-not-persist%'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME LIKE 'mochat_go_saas_tenant_domain_delivery%' AND (COLUMN_NAME LIKE '%private%' OR COLUMN_NAME LIKE '%pem%')")" = "0"
grep -q "Bearer $BRIDGE_TOKEN" "$BRIDGE_LOG"
grep -q '/webhooks/saas/domain-delivery' "$BRIDGE_LOG"
if grep -Eqi 'privateKey|certificatePem' "$BRIDGE_LOG"; then
  echo "domain delivery bridge payload leaked certificate key material" >&2
  exit 1
fi

api_get "$PLATFORM_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/access-profile.json"
python3 - "$WORK_DIR/access-profile.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
codes = {item["code"] for item in data["permissions"]}
assert len(codes) == 32, data
assert {"platform.domains.read", "platform.domains.manage"} <= codes, data
PY

api_get "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomains?tenantId=991&status=all' "$WORK_DIR/domains-final.json"
api_get "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/tenantDomainDeliveryJobs?tenantId=991&status=all&limit=100' "$WORK_DIR/delivery-jobs-final.json"
test "$(json_value "$WORK_DIR/delivery-jobs-final.json" data.returnedCount)" -ge "4"
api_get "$PLATFORM_TOKEN" '/dashboard/saasAdmin/systemHealth' "$WORK_DIR/system-health.json"
jq -e 'any(.data.checks[]; .code == "domain_delivery_queue" and .status == "healthy") and any(.data.checks[]; .code == "domain_certificate_lifecycle" and .status == "healthy") and any(.data.checks[]; .code == "domain_delivery_configuration" and .status == "healthy")' "$WORK_DIR/system-health.json" >/dev/null
test "$(json_value "$WORK_DIR/domains-final.json" data.verification.recordPrefix)" = "_mochat."
test "$(json_value "$WORK_DIR/domains-final.json" data.verification.valuePrefix)" = "mochat-domain-verification="
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action LIKE 'saas.admin.tenant_domain.%'")" -ge "12"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.tenant_domain.verify_failed'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action LIKE 'saas.admin.tenant_domain_delivery.%'")" -ge "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_domain_delivery_events WHERE event_id = 'domain-event-1' AND result = 'applied' AND CHAR_LENGTH(payload_sha256) = 64")" = "1"

curl -sS "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/admin-page.html"
grep -q 'id="tenantDomainCenter"' "$WORK_DIR/admin-page.html"
grep -q 'id="tenantDomainActionDialog"' "$WORK_DIR/admin-page.html"
grep -q 'id="tenantDomainDeliveryJobs"' "$WORK_DIR/admin-page.html"
grep -q '/dashboard/saasAdmin/tenantDomainDeliveryJobs' "$WORK_DIR/admin-page.html"
curl -sS "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q '/dashboard/saasAdmin/tenantDomains' "$WORK_DIR/routes.json"
grep -q '/dashboard/saasAdmin/tenantDomain' "$WORK_DIR/routes.json"
grep -q '/dashboard/saasAdmin/tenantDomainDeliveryJobs' "$WORK_DIR/routes.json"
grep -q '/dashboard/saasAdmin/tenantDomainDelivery' "$WORK_DIR/routes.json"
grep -q '/webhooks/saas/domain-delivery' "$WORK_DIR/routes.json"
if grep -Eqi 'Deadlock found|Lock wait timeout exceeded' "$GO_LOG"; then
  echo "tenant domain delivery left an unhandled MySQL transaction conflict" >&2
  exit 1
fi

echo "SaaS tenant domains and route/TLS delivery smoke passed"
