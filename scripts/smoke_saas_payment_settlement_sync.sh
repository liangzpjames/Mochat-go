#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-payment-settlement-sync-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13385}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26435}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18157}"
BRIDGE_ADDR="${MOCHAT_PAYMENT_SETTLEMENT_BRIDGE_ADDR:-127.0.0.1:18158}"
DATABASE="mochat_payment_settlement_sync"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-payment-settlement-sync.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
BRIDGE_LOG="$WORK_DIR/bridge.log"
BRIDGE_MODE="$WORK_DIR/bridge-mode"
GO_PID=""
BRIDGE_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-payment-settlement-sync-jwt-secret}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

stop_processes() {
  for pid in "$GO_PID" "$BRIDGE_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  GO_PID=""
  BRIDGE_PID=""
}

cleanup() {
  stop_processes
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "SaaS payment settlement sync smoke failed at line $LINENO" >&2; tail -160 "$GO_LOG" >&2 || true; tail -80 "$BRIDGE_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

wait_service_healthy() {
  local service="$1"
  local deadline=$((SECONDS + ${MOCHAT_SERVICE_HEALTH_TIMEOUT_SECONDS:-360}))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    if [ "$status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

wait_sql() {
  local query="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ "$(mysql_scalar "$query" 2>/dev/null || true)" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "SQL condition timed out: $query expected $expected" >&2
  return 1
}

login_token() {
  local phone="$1"
  local password="$2"
  local output="$3"
  curl -sS -f -H "Content-Type: application/json" -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  python3 - "$output" <<'PY'
import json, pathlib, sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
print(payload["data"]["token"])
PY
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

start_bridge() {
  printf 'normal' >"$BRIDGE_MODE"
  python3 -u - "${BRIDGE_ADDR%:*}" "${BRIDGE_ADDR##*:}" "$BRIDGE_MODE" >"$BRIDGE_LOG" 2>&1 <<'PY' &
import json
import pathlib
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

host, port, mode_path = sys.argv[1], int(sys.argv[2]), pathlib.Path(sys.argv[3])

def batch(number):
    return {
        "provider": "gateway",
        "providerSettlementNo": f"GW-AUTO-{number}",
        "periodStart": "2026-07-10 00:00:00",
        "periodEnd": "2026-07-11 00:00:00",
        "currency": "CNY",
        "remark": f"Bridge 自动结算 {number}",
        "entries": [{
            "lineNo": 1,
            "providerTransactionNo": f"GW-AUTO-TXN-{number}",
            "transactionType": "payment",
            "orderNo": f"PAY-AUTO-{number}",
            "providerOrderNo": f"GW-ORDER-AUTO-{number}",
            "amountCents": 10000 + number,
            "feeCents": -60,
            "netAmountCents": 9940 + number,
            "occurredAt": "2026-07-10 08:00:00"
        }]
    }

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query, keep_blank_values=True)
        if parsed.path != "/v1/payment-settlements":
            self.send_error(404)
            return
        if self.headers.get("Authorization") != "Bearer bridge-token":
            self.send_error(401)
            return
        if query.get("provider", [""])[0] != "gateway" or query.get("limit", [""])[0] != "100":
            self.send_error(400)
            return
        cursor = query.get("cursor", [""])[0]
        mode = mode_path.read_text(encoding="utf-8").strip()
        if mode == "fail":
            payload = b'{"error":"bridge unavailable"}'
            self.send_response(503)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            return
        if mode == "slow":
            time.sleep(4)
        batches, next_cursor = [], cursor
        if cursor == "":
            batches, next_cursor = [batch(1)], "cursor-1"
        elif cursor == "cursor-1" and mode == "replay":
            batches, next_cursor = [batch(1)], "cursor-2"
        elif cursor == "cursor-2" and mode == "next":
            batches, next_cursor = [batch(2)], "cursor-3"
        response = json.dumps({"provider": "gateway", "nextCursor": next_cursor, "hasMore": False, "batches": batches}, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(response)))
        self.end_headers()
        self.wfile.write(response)

    def log_message(self, fmt, *args):
        print(fmt % args, flush=True)

ThreadingHTTPServer((host, port), Handler).serve_forever()
PY
  BRIDGE_PID="$!"
  wait_url "http://$BRIDGE_ADDR/not-found" 404
}

start_go() {
  : >"$GO_LOG"
  env -u GOROOT \
    MOCHAT_GO_STANDALONE=1 \
    MOCHAT_GO_ADDR="$GO_ADDR" \
    MOCHAT_MYSQL_DSN="$DSN" \
    MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
    MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
    MOCHAT_GO_MIGRATE_AUTH=1 \
    MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
    MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON=1 \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_RUN_ON_START=1 \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL="http://$BRIDGE_ADDR" \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TOKEN="bridge-token" \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS="gateway" \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT=100 \
    MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS=10 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"
assert_port_free "${BRIDGE_ADDR##*:}"

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
grep -q $'0045_saas_admin_rbac\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0046_saas_admin_approvals\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0047_saas_admin_approval_governance\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0049_saas_service_accounts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_sync_states'")" = "mochat_go_saas_payment_settlement_sync_states"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_sync_runs'")" = "mochat_go_saas_payment_settlement_sync_runs"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,结算同步平台,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
941,结算同步租户,13800000941,secret941,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

start_bridge
start_go
wait_sql "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_sync_runs WHERE provider = 'gateway' AND source = 'cron' AND status = 'succeeded'" "1"
wait_sql "SELECT \`cursor\` FROM mochat_go_saas_payment_settlement_sync_states WHERE provider = 'gateway'" "cursor-1"
wait_sql "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-payment-settlement-sync' AND status = 'succeeded'" "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_batches WHERE provider_settlement_no = 'GW-AUTO-1'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE alert_json LIKE '%payment_settlement_issue%'")" = "1"

PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_TOKEN="$(login_token 13800000941 secret941 "$WORK_DIR/tenant-auth.json")"
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/paymentSettlementSyncRuns" "$WORK_DIR/tenant-get-forbidden.json" 403
api_post "$TENANT_TOKEN" "/dashboard/saasAdmin/paymentSettlementSync" '{"provider":"gateway","dryRun":true}' "$WORK_DIR/tenant-post-forbidden.json" 403

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementSyncRuns?provider=gateway&limit=20" "$WORK_DIR/runs-initial.json"
python3 - "$WORK_DIR/runs-initial.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["enabled"] is True and data["configuredProviders"] == ["gateway"], data
assert data["states"][0]["cursor"] == "cursor-1" and data["states"][0]["activeRunId"] == 0, data
assert data["runs"][0]["status"] == "succeeded" and data["runs"][0]["importedBatchCount"] == 1, data
PY

printf 'replay' >"$BRIDGE_MODE"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementSync" '{"provider":"gateway","dryRun":false}' "$WORK_DIR/replay.json"
python3 - "$WORK_DIR/replay.json" <<'PY'
import json, pathlib, sys
run = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["run"]
assert run["status"] == "succeeded" and run["importedBatchCount"] == 0 and run["idempotentBatchCount"] == 1, run
assert run["cursorAfter"] == "cursor-2", run
PY
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_batches")" = "1"

printf 'next' >"$BRIDGE_MODE"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementSync" '{"provider":"gateway","dryRun":true}' "$WORK_DIR/preview.json"
grep -q '"status":"previewed"' "$WORK_DIR/preview.json"
test "$(mysql_scalar "SELECT \`cursor\` FROM mochat_go_saas_payment_settlement_sync_states WHERE provider = 'gateway'")" = "cursor-2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_batches")" = "1"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementSync" '{"provider":"gateway","dryRun":false}' "$WORK_DIR/next.json"
grep -q '"cursorAfter":"cursor-3"' "$WORK_DIR/next.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_batches")" = "2"

printf 'slow' >"$BRIDGE_MODE"
curl -sS -o "$WORK_DIR/slow-first.json" -H "Authorization: Bearer $PLATFORM_TOKEN" -H "Content-Type: application/json" -d '{"provider":"gateway","dryRun":false}' "http://$GO_ADDR/dashboard/saasAdmin/paymentSettlementSync" &
SLOW_PID="$!"
wait_sql "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_sync_states WHERE provider = 'gateway' AND active_run_id IS NOT NULL" "1"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementSync" '{"provider":"gateway","dryRun":false}' "$WORK_DIR/concurrent.json" 409
wait "$SLOW_PID"
wait_sql "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_sync_states WHERE provider = 'gateway' AND active_run_id IS NULL" "1"

printf 'fail' >"$BRIDGE_MODE"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementSync" '{"provider":"gateway","dryRun":false}' "$WORK_DIR/failure.json" 500
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_settlement_sync_runs ORDER BY id DESC LIMIT 1")" = "failed"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE alert_json LIKE '%payment_settlement_sync_failed%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'payment.settlement.sync'")" -ge "5"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/admin-page.html"
grep -q 'id="paymentSettlementSyncProvider"' "$WORK_DIR/admin-page.html"
grep -q 'id="paymentSettlementSyncStates"' "$WORK_DIR/admin-page.html"
grep -q 'id="paymentSettlementSyncRuns"' "$WORK_DIR/admin-page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
for route in \
  'GET /dashboard/saasAdmin/paymentSettlementSyncRuns' \
  'POST /dashboard/saasAdmin/paymentSettlementSync' \
  'PUT /dashboard/saasAdmin/paymentSettlementSync'; do
  grep -q "$route" "$WORK_DIR/routes.json"
done
grep -q 'go cron enabled: SaaS payment settlement sync' "$GO_LOG"
grep -q 'SaaS payment settlement sync finished: provider=gateway' "$GO_LOG"

echo "SaaS payment settlement sync smoke passed"
