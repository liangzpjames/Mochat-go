#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-subscription-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13380}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26430}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18151}"
DATABASE="mochat_subscription_lifecycle"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-subscription.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-subscription-lifecycle-secret}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

stop_go() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  GO_PID=""
}

cleanup() {
  stop_go
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
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
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  tail -120 "$GO_LOG" >&2 || true
  exit 1
}

wait_mysql_value() {
  local sql="$1"
  local expected="$2"
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ "$(mysql_scalar "$sql")" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for SQL value: $sql => $expected" >&2
  exit 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" | tr -d '\r'
}

login_token() {
  local phone="$1"
  local password="$2"
  local output="$3"
  curl -sS -f -H "Content-Type: application/json" \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$output"
  python3 - "$output" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
token = payload.get("data", {}).get("token", "")
assert token, payload
print(token)
PY
}

api_post() {
  local token="$1"
  local path="$2"
  local body="$3"
  local output="$4"
  local expected="${5:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' \
    -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
    -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected"
}

start_go() {
  local enable_cron="${1:-0}"
  local run_on_start="${2:-0}"
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
    MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON="$enable_cron" \
    MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_RUN_ON_START="$run_on_start" \
    MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_LIMIT=500 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SHOW TABLES LIKE 'mochat_go_saas_subscriptions'" | tr -d '\r')" = "mochat_go_saas_subscriptions"
test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SHOW TABLES LIKE 'mochat_go_saas_subscription_events'" | tr -d '\r')" = "mochat_go_saas_subscription_events"

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
grep -q $'0039_saas_subscription_lifecycle\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0040_saas_payment_collection\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0041_saas_payment_refunds\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0042_saas_billing_invoices\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0043_saas_payment_settlements\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0045_saas_admin_rbac\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0046_saas_admin_approvals\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0047_saas_admin_approval_governance\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0049_saas_service_accounts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_subscriptions' AND INDEX_NAME = 'idx_mochat_go_saas_subscriptions_status_period'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_subscription_events' AND INDEX_NAME = 'uni_mochat_go_saas_subscription_events_idempotency'")" = "2"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,订阅平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
801,试用租户,13800000801,secret801,试用管理员,SaaS租户超级管理员,growth,增长版,2,10,1000,50,3,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
802,宽限租户,13800000802,secret802,宽限管理员,SaaS租户超级管理员,growth,增长版,2,10,1000,50,3,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
803,欠费租户,13800000803,secret803,欠费管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
804,启停租户,13800000804,secret804,启停管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "5"

start_go 0 0
PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":1,"status":"active","expectedVersion":0,"idempotencyKey":"smoke-platform-init","reason":"平台订阅初始化"}' \
  "$WORK_DIR/platform-init.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":801,"status":"trialing","trialEndsAt":"2037-02-01 00:00:00","expectedVersion":0,"idempotencyKey":"smoke-trial","reason":"开通试用"}' \
  "$WORK_DIR/trial.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":802,"status":"grace","currentPeriodEndsAt":"2020-01-01 00:00:00","graceEndsAt":"2037-02-01 00:00:00","expectedVersion":0,"idempotencyKey":"smoke-grace","reason":"等待回款"}' \
  "$WORK_DIR/grace.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":803,"status":"past_due","expectedVersion":0,"idempotencyKey":"smoke-past-due","reason":"宽限期结束"}' \
  "$WORK_DIR/past-due.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":804,"status":"active","expectedVersion":0,"idempotencyKey":"smoke-active","reason":"正式订阅"}' \
  "$WORK_DIR/active.json"

TRIAL_TOKEN="$(login_token 13800000801 secret801 "$WORK_DIR/trial-auth.json")"
GRACE_TOKEN="$(login_token 13800000802 secret802 "$WORK_DIR/grace-auth.json")"
PAST_DUE_STATUS="$(curl -sS -o "$WORK_DIR/past-due-auth.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000803","password":"secret803"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$PAST_DUE_STATUS" = "403"
grep -q '租户订阅已欠费' "$WORK_DIR/past-due-auth.json"

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/subscriptions?limit=20" >"$WORK_DIR/subscriptions.json"
python3 - "$WORK_DIR/subscriptions.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
summary = data["summary"]
assert summary["subscriptionCount"] == 5, summary
assert summary["trialingCount"] == 1, summary
assert summary["activeCount"] == 2, summary
assert summary["graceCount"] == 1, summary
assert summary["pastDueCount"] == 1, summary
assert summary["accessAllowedCount"] == 4, summary
assert summary["accessBlockedCount"] == 1, summary
items = {item["tenantId"]: item for item in data["subscriptions"]}
assert items[801]["effectiveStatus"] == "trialing", items[801]
assert items[802]["effectiveStatus"] == "grace", items[802]
assert items[803]["effectiveStatus"] == "past_due", items[803]
PY

TENANT_FORBIDDEN="$(curl -sS -o "$WORK_DIR/tenant-forbidden.json" -w '%{http_code}' -H "Authorization: Bearer $TRIAL_TOKEN" "http://$GO_ADDR/dashboard/saasAdmin/subscriptions")"
test "$TENANT_FORBIDDEN" = "403"

GRACE_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_subscriptions WHERE tenant_id = 802")"
CONFLICT_VERSION=$((GRACE_VERSION - 1))
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  "{\"tenantId\":802,\"status\":\"grace\",\"graceEndsAt\":\"2037-02-01 00:00:00\",\"expectedVersion\":$CONFLICT_VERSION,\"reason\":\"并发冲突\"}" \
  "$WORK_DIR/version-conflict.json" 400
grep -q 'version conflict' "$WORK_DIR/version-conflict.json"

ACTIVE_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_subscriptions WHERE tenant_id = 804")"
IDEMPOTENT_BODY="{\"tenantId\":804,\"status\":\"active\",\"expectedVersion\":$ACTIVE_VERSION,\"idempotencyKey\":\"smoke-idempotent-804\",\"reason\":\"幂等状态确认\"}"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" "$IDEMPOTENT_BODY" "$WORK_DIR/idempotent-first.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" "$IDEMPOTENT_BODY" "$WORK_DIR/idempotent-second.json"
grep -q '"idempotent":true' "$WORK_DIR/idempotent-second.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscription_events WHERE tenant_id = 804 AND idempotency_key = 'smoke-idempotent-804'")" = "1"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/tenantStatus" \
  '{"tenantId":804,"status":2,"remark":"风控暂停"}' "$WORK_DIR/tenant-disable.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 804")" = "suspended"
SUSPENDED_STATUS="$(curl -sS -o "$WORK_DIR/suspended-auth.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000804","password":"secret804"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$SUSPENDED_STATUS" = "403"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/tenantStatus" \
  '{"tenantId":804,"status":1,"remark":"风控解除"}' "$WORK_DIR/tenant-enable.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 804")" = "active"
ACTIVE_TOKEN="$(login_token 13800000804 secret804 "$WORK_DIR/active-auth.json")"
test -n "$ACTIVE_TOKEN"

mysql_root "$DATABASE" <<'SQL'
UPDATE mochat_go_saas_subscriptions
SET status = 'active', current_period_ends_at = DATE_SUB(NOW(), INTERVAL 1 DAY),
    grace_ends_at = DATE_ADD(NOW(), INTERVAL 1 DAY), version = version + 1,
    state_reason = 'smoke stale active row', updated_at = NOW()
WHERE tenant_id = 802;
SQL
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionReconcile" \
  '{"tenantId":802,"limit":10,"dryRun":true}' "$WORK_DIR/reconcile-preview.json"
grep -q '"reconciliationDue":1' "$WORK_DIR/reconcile-preview.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionReconcile" \
  '{"tenantId":802,"limit":10,"dryRun":false}' "$WORK_DIR/reconcile-apply.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 802")" = "grace"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/tenantRenewal" \
  '{"tenantId":803,"packageCode":"starter","expiresAt":"2038-01-01 00:00:00","amountCents":120000,"currency":"CNY","paidAt":"2026-07-10 12:00:00","paymentMethod":"offline","externalOrderNo":"SUB-SMOKE-803","remark":"欠费续费恢复"}' \
  "$WORK_DIR/renewal.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 803")" = "active"
test "$(mysql_scalar "SELECT IF(latest_billing_event_id > 0, 1, 0) FROM mochat_go_saas_subscriptions WHERE tenant_id = 803")" = "1"
RENEWED_TOKEN="$(login_token 13800000803 secret803 "$WORK_DIR/renewed-auth.json")"
test -n "$RENEWED_TOKEN"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":1,"status":"suspended","reason":"不应允许"}' "$WORK_DIR/platform-protected.json" 400
grep -q 'platform tenant subscription cannot be blocked' "$WORK_DIR/platform-protected.json"

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/subscriptionEvents?tenantId=803&limit=20" >"$WORK_DIR/events.json"
python3 - "$WORK_DIR/events.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
events = data["events"]
assert any(item["eventType"] == "renewed" and item["source"] == "billing" for item in events), events
assert any(item["toStatus"] == "past_due" for item in events), events
PY

curl -sS -f -D "$WORK_DIR/subscriptions-csv.headers" -o "$WORK_DIR/subscriptions.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=subscriptions&limit=1000"
grep -qi 'filename="mochat-saas-subscriptions-' "$WORK_DIR/subscriptions-csv.headers"
python3 - "$WORK_DIR/subscriptions.csv" <<'PY'
import csv
import pathlib
import sys

rows = list(csv.DictReader(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8-sig").splitlines()))
assert {row["租户ID"] for row in rows} == {"1", "801", "802", "803", "804"}, rows
assert next(row for row in rows if row["租户ID"] == "802")["有效状态"] == "grace"
PY

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'id="subscriptionSummary"' "$WORK_DIR/page.html"
grep -q 'id="subscriptionTransitionStatus"' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/subscriptionTransition'" "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/subscriptions' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/subscriptionReconcile' "$WORK_DIR/routes.json"

mysql_root "$DATABASE" <<'SQL'
UPDATE mochat_go_saas_subscriptions
SET status = 'trialing', trial_ends_at = DATE_SUB(NOW(), INTERVAL 1 DAY),
    grace_ends_at = DATE_ADD(NOW(), INTERVAL 1 DAY), version = version + 1,
    state_reason = 'smoke cron stale trial', updated_at = NOW()
WHERE tenant_id = 801;
SQL
stop_go
start_go 1 1
wait_mysql_value "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 801" "grace"
grep -q 'go cron enabled: SaaS subscription reconcile interval=1h0m0s run_on_start=true limit=500' "$GO_LOG"
grep -q 'SaaS subscription reconcile cron finished:' "$GO_LOG"
wait_mysql_value "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-subscription-reconcile' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"

echo "SaaS subscription lifecycle smoke passed"
