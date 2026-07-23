#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-payment-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13381}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26431}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18152}"
DATABASE="mochat_payment_collection"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-payment.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-payment-collection-jwt-secret}"
WEBHOOK_SECRET="${MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET:-payment-collection-webhook-secret-2026}"
WEBHOOK_SEQ=0

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
trap 'echo "SaaS payment collection smoke failed at line $LINENO" >&2' ERR
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
  if [ "$status" != "$expected" ]; then
    echo "POST $path returned $status, expected $expected" >&2
    cat "$output" >&2 || true
    return 1
  fi
}

webhook_post() {
  local body="$1"
  local output="$2"
  local expected="${3:-200}"
  local override_signature="${4:-}"
  local timestamp body_file signature status
  WEBHOOK_SEQ=$((WEBHOOK_SEQ + 1))
  body_file="$WORK_DIR/webhook-$WEBHOOK_SEQ.json"
  printf '%s' "$body" >"$body_file"
  timestamp="$(date +%s)"
  signature="$(python3 - "$WEBHOOK_SECRET" "$timestamp" "$body_file" <<'PY'
import hashlib
import hmac
import pathlib
import sys

secret, timestamp, path = sys.argv[1:]
body = pathlib.Path(path).read_bytes()
message = timestamp.encode() + b"." + body
print("v1=" + hmac.new(secret.encode(), message, hashlib.sha256).hexdigest())
PY
)"
  if [ -n "$override_signature" ]; then
    signature="$override_signature"
  fi
  status="$(curl -sS -o "$output" -w '%{http_code}' \
    -H "Content-Type: application/json" \
    -H "X-Mochat-Go-Payment-Timestamp: $timestamp" \
    -H "X-Mochat-Go-Payment-Signature: $signature" \
    --data-binary "@$body_file" "http://$GO_ADDR/webhooks/saas/payment")"
  if [ "$status" != "$expected" ]; then
    echo "payment webhook returned $status, expected $expected" >&2
    cat "$output" >&2 || true
    return 1
  fi
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
    MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK=1 \
    MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET="$WEBHOOK_SECRET" \
    MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS=300 \
    MOCHAT_GO_ENABLE_SAAS_PAYMENT_DUNNING_CRON="$enable_cron" \
    MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_RUN_ON_START="$run_on_start" \
    MOCHAT_GO_SAAS_PAYMENT_DUNNING_LIMIT=100 \
    MOCHAT_GO_SAAS_PAYMENT_DUNNING_RETRY_DELAY_SECONDS=3600 \
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

test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SHOW TABLES LIKE 'mochat_go_saas_payment_orders'" | tr -d '\r')" = "mochat_go_saas_payment_orders"
test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SHOW TABLES LIKE 'mochat_go_saas_payment_webhook_events'" | tr -d '\r')" = "mochat_go_saas_payment_webhook_events"
test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SHOW TABLES LIKE 'mochat_go_saas_payment_refunds'" | tr -d '\r')" = "mochat_go_saas_payment_refunds"

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
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND INDEX_NAME = 'uni_mochat_go_saas_payment_orders_idempotency'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_payment_webhook_events' AND INDEX_NAME = 'uni_mochat_go_saas_payment_webhook_event'")" = "2"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,支付平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
901,支付测试租户,13800000901,secret901,支付管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "2"

start_go 0 0
PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_TOKEN="$(login_token 13800000901 secret901 "$WORK_DIR/tenant-auth.json")"

TENANT_FORBIDDEN="$(curl -sS -o "$WORK_DIR/tenant-forbidden.json" -w '%{http_code}' -H "Authorization: Bearer $TENANT_TOKEN" "http://$GO_ADDR/dashboard/saasAdmin/paymentOrders")"
test "$TENANT_FORBIDDEN" = "403"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":901,"status":"past_due","expectedVersion":0,"idempotencyKey":"payment-smoke-past-due","reason":"等待支付恢复"}' \
  "$WORK_DIR/past-due.json"
PAST_DUE_STATUS="$(curl -sS -o "$WORK_DIR/past-due-auth.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000901","password":"secret901"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$PAST_DUE_STATUS" = "403"

FAIL_ORDER_BODY='{"orderNo":"PAY-FAIL-901","tenantId":901,"provider":"gateway","providerOrderNo":"GW-FAIL-901","idempotencyKey":"payment-smoke-fail-901","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-01 00:00:00","amountCents":128000,"currency":"CNY","checkoutUrl":"https://pay.example.test/PAY-FAIL-901","checkoutExpiresAt":"2037-12-31 23:59:59","maxDunningAttempts":2,"remark":"失败催缴验收"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$FAIL_ORDER_BODY" "$WORK_DIR/fail-order.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$FAIL_ORDER_BODY" "$WORK_DIR/fail-order-replay.json"
grep -q '"idempotent":true' "$WORK_DIR/fail-order-replay.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")" = "1"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" \
  '{"orderNo":"PAY-FAIL-CONFLICT","tenantId":901,"provider":"gateway","providerOrderNo":"GW-FAIL-CONFLICT","idempotencyKey":"payment-smoke-fail-901","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-01 00:00:00","amountCents":129000,"currency":"CNY"}' \
  "$WORK_DIR/fail-order-conflict.json" 409
grep -q 'idempotency key already used' "$WORK_DIR/fail-order-conflict.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" \
  '{"orderNo":"PAY-TIMESTAMP-RANGE","tenantId":901,"provider":"gateway","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2039-01-01 00:00:00","amountCents":100,"currency":"CNY"}' \
  "$WORK_DIR/timestamp-range.json" 400
grep -q 'MySQL 5.7 TIMESTAMP' "$WORK_DIR/timestamp-range.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-TIMESTAMP-RANGE'")" = "0"

FAIL_EVENT='{"provider":"gateway","eventId":"evt-fail-901","eventType":"payment.failed","orderNo":"PAY-FAIL-901","providerOrderNo":"GW-FAIL-901","occurredAt":"2026-07-10 12:00:00","failureCode":"insufficient_funds","failureMessage":"余额不足","metadata":{"source":"smoke"}}'
webhook_post "$FAIL_EVENT" "$WORK_DIR/fail-webhook.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")" = "failed"
test "$(mysql_scalar "SELECT attempt_count FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")" = "1"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 901")" = "past_due"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 901")" = "0"

webhook_post "$FAIL_EVENT" "$WORK_DIR/fail-webhook-replay.json"
grep -q '"duplicate":true' "$WORK_DIR/fail-webhook-replay.json"
test "$(mysql_scalar "SELECT attempt_count FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")" = "1"
test "$(mysql_scalar "SELECT attempts FROM mochat_go_saas_payment_webhook_events WHERE provider = 'gateway' AND event_id = 'evt-fail-901'")" = "1"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentDunning" \
  '{"limit":100,"dryRun":true,"retryDelaySeconds":3600,"notificationMaxAttempts":5}' \
  "$WORK_DIR/dunning-preview.json"
grep -q '"matchedCount":1' "$WORK_DIR/dunning-preview.json"
grep -q '"enqueuedCount":0' "$WORK_DIR/dunning-preview.json"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentDunning" \
  '{"limit":100,"dryRun":false,"retryDelaySeconds":3600,"notificationMaxAttempts":5}' \
  "$WORK_DIR/dunning-apply.json"
grep -q '"enqueuedCount":1' "$WORK_DIR/dunning-apply.json"
test "$(mysql_scalar "SELECT dunning_attempts FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE tenant_id = 901 AND alert_json LIKE '%payment_failed_reminder%' AND alert_json LIKE '%PAY-FAIL-901%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'payment.order.dunning' AND target_id = 'PAY-FAIL-901'")" = "1"

FAIL_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")"
STALE_VERSION=$((FAIL_VERSION - 1))
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrderCancel" \
  "{\"orderNo\":\"PAY-FAIL-901\",\"expectedVersion\":$STALE_VERSION,\"reason\":\"并发冲突\"}" \
  "$WORK_DIR/cancel-conflict.json" 409
grep -q 'version conflict' "$WORK_DIR/cancel-conflict.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrderCancel" \
  "{\"orderNo\":\"PAY-FAIL-901\",\"expectedVersion\":$FAIL_VERSION,\"reason\":\"终止失败订单\"}" \
  "$WORK_DIR/cancel-success.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-FAIL-901'")" = "canceled"

SUCCESS_ORDER_BODY='{"orderNo":"PAY-SUCCESS-901","tenantId":901,"provider":"gateway","providerOrderNo":"GW-SUCCESS-901","idempotencyKey":"payment-smoke-success-901","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-10 00:00:00","amountCents":256000,"currency":"CNY","checkoutUrl":"https://pay.example.test/PAY-SUCCESS-901","checkoutExpiresAt":"2038-01-09 23:59:59","maxDunningAttempts":3,"remark":"成功回款验收"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$SUCCESS_ORDER_BODY" "$WORK_DIR/success-order.json"

SUCCESS_EVENT='{"provider":"gateway","eventId":"evt-success-901","eventType":"payment.succeeded","orderNo":"PAY-SUCCESS-901","providerOrderNo":"GW-SUCCESS-901","amountCents":256000,"currency":"CNY","paidAt":"2026-07-10 13:00:00","occurredAt":"2026-07-10 13:00:00","metadata":{"source":"smoke"}}'
webhook_post "$SUCCESS_EVENT" "$WORK_DIR/invalid-signature.json" 401 'v1=invalid'
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_webhook_events WHERE event_id = 'evt-success-901'")" = "0"

webhook_post "$SUCCESS_EVENT" "$WORK_DIR/success-webhook.json"
python3 - "$WORK_DIR/success-webhook.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["order"]["status"] == "paid", data
assert data["billingEventId"] > 0, data
assert data["event"]["status"] == "processed", data
PY
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-SUCCESS-901'")" = "paid"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 901 AND event_type = 'renewal' AND external_order_no = 'PAY-SUCCESS-901' AND payment_method = 'gateway'")" = "1"
test "$(mysql_scalar "SELECT package_code FROM mochat_go_saas_tenant_packages WHERE tenant_id = 901 AND deleted_at IS NULL")" = "starter"
test "$(mysql_scalar "SELECT DATE(expires_at) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 901 AND deleted_at IS NULL")" = "2038-01-10"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 901")" = "active"
test "$(mysql_scalar "SELECT IF(latest_billing_event_id = (SELECT billing_event_id FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-SUCCESS-901'), 1, 0) FROM mochat_go_saas_subscriptions WHERE tenant_id = 901")" = "1"
test "$(mysql_scalar "SELECT billing_cycle FROM mochat_go_saas_subscriptions WHERE tenant_id = 901")" = "yearly"
RECOVERED_TOKEN="$(login_token 13800000901 secret901 "$WORK_DIR/recovered-auth.json")"
test -n "$RECOVERED_TOKEN"

webhook_post "$SUCCESS_EVENT" "$WORK_DIR/success-webhook-replay.json"
grep -q '"duplicate":true' "$WORK_DIR/success-webhook-replay.json"
SUCCESS_EVENT_CHANGED='{"provider":"gateway","eventId":"evt-success-901","eventType":"payment.succeeded","orderNo":"PAY-SUCCESS-901","providerOrderNo":"GW-SUCCESS-901","amountCents":256001,"currency":"CNY","paidAt":"2026-07-10 13:00:00","occurredAt":"2026-07-10 13:00:00"}'
webhook_post "$SUCCESS_EVENT_CHANGED" "$WORK_DIR/success-webhook-conflict.json" 409
grep -q 'payload digest mismatch' "$WORK_DIR/success-webhook-conflict.json"

LATE_FAIL_EVENT='{"provider":"gateway","eventId":"evt-late-fail-901","eventType":"payment.failed","orderNo":"PAY-SUCCESS-901","providerOrderNo":"GW-SUCCESS-901","occurredAt":"2026-07-10 13:05:00","failureCode":"late_failure","failureMessage":"延迟失败事件"}'
webhook_post "$LATE_FAIL_EVENT" "$WORK_DIR/late-fail-webhook.json"
grep -q '"ignored":true' "$WORK_DIR/late-fail-webhook.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-SUCCESS-901'")" = "paid"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 901 AND external_order_no = 'PAY-SUCCESS-901'")" = "1"

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/paymentOrders?limit=20" >"$WORK_DIR/orders.json"
python3 - "$WORK_DIR/orders.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
summary = data["summary"]
assert summary["orderCount"] == 2, summary
assert summary["paidCount"] == 1, summary
assert summary["canceledCount"] == 1, summary
assert summary["paidAmountCents"] == 256000, summary
assert summary["providerCount"] == 1, summary
orders = {item["orderNo"]: item for item in data["orders"]}
assert orders["PAY-SUCCESS-901"]["billingEventId"] > 0, orders
assert orders["PAY-FAIL-901"]["dunningAttempts"] == 1, orders
PY

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/paymentOrders?status=paid&keyword=SUCCESS&limit=20" >"$WORK_DIR/paid-orders.json"
grep -q 'PAY-SUCCESS-901' "$WORK_DIR/paid-orders.json"
if grep -q 'PAY-FAIL-901' "$WORK_DIR/paid-orders.json"; then
  exit 1
fi

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/paymentWebhookEvents?provider=gateway&limit=20" >"$WORK_DIR/webhook-events.json"
python3 - "$WORK_DIR/webhook-events.json" <<'PY'
import json
import pathlib
import sys

events = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["events"]
assert len(events) == 3, events
by_id = {item["eventId"]: item for item in events}
assert by_id["evt-fail-901"]["status"] == "processed", by_id
assert by_id["evt-success-901"]["status"] == "processed", by_id
assert by_id["evt-late-fail-901"]["status"] == "ignored", by_id
PY

curl -sS -f -D "$WORK_DIR/payment-csv.headers" -o "$WORK_DIR/payment-orders.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=paymentOrders&limit=1000"
grep -qi 'filename="mochat-saas-payment-orders-' "$WORK_DIR/payment-csv.headers"
python3 - "$WORK_DIR/payment-orders.csv" <<'PY'
import csv
import pathlib
import sys

rows = list(csv.DictReader(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8-sig").splitlines()))
assert {row["orderNo"] for row in rows} == {"PAY-FAIL-901", "PAY-SUCCESS-901"}, rows
assert next(row for row in rows if row["orderNo"] == "PAY-SUCCESS-901")["status"] == "paid"
PY

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'aria-label="支付收款"' "$WORK_DIR/page.html"
grep -q 'id="paymentOrders"' "$WORK_DIR/page.html"
grep -q 'id="paymentWebhookEvents"' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/paymentOrder'" "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/paymentOrders' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/paymentOrder' "$WORK_DIR/routes.json"
grep -q 'POST /webhooks/saas/payment' "$WORK_DIR/routes.json"

CRON_ORDER_BODY='{"orderNo":"PAY-CRON-901","tenantId":901,"provider":"gateway","providerOrderNo":"GW-CRON-901","idempotencyKey":"payment-smoke-cron-901","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-15 00:00:00","amountCents":64000,"currency":"CNY","maxDunningAttempts":2,"remark":"催缴定时任务验收"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$CRON_ORDER_BODY" "$WORK_DIR/cron-order.json"
CRON_FAIL_EVENT='{"provider":"gateway","eventId":"evt-cron-fail-901","eventType":"payment.failed","orderNo":"PAY-CRON-901","providerOrderNo":"GW-CRON-901","occurredAt":"2026-07-10 14:00:00","failureCode":"timeout","failureMessage":"支付超时"}'
webhook_post "$CRON_FAIL_EVENT" "$WORK_DIR/cron-fail-webhook.json"
test "$(mysql_scalar "SELECT dunning_attempts FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-CRON-901'")" = "0"

stop_go
start_go 1 1
wait_mysql_value "SELECT dunning_attempts FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-CRON-901'" "1"
wait_mysql_value "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE tenant_id = 901 AND alert_json LIKE '%PAY-CRON-901%'" "1"
grep -q 'go cron enabled: SaaS payment dunning interval=1h0m0s run_on_start=true limit=100 retry_delay=1h0m0s' "$GO_LOG"
grep -q 'SaaS payment dunning cron finished: matched=1 enqueued=1 exhausted=0 skipped=0 failed=0' "$GO_LOG"
wait_mysql_value "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-payment-dunning' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"

echo "SaaS payment collection smoke passed"
