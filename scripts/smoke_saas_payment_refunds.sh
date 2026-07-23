#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-refund-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13382}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26432}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18153}"
DATABASE="mochat_payment_refunds"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-refunds.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-payment-refund-jwt-secret}"
WEBHOOK_SECRET="${MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET:-payment-refund-webhook-secret-2026}"
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
trap 'echo "SaaS payment refund smoke failed at line $LINENO" >&2' ERR
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

concurrent_refund_post() {
  local token="$1"
  local body="$2"
  local output="$3"
  local status_file="$4"
  curl -sS -o "$output" -w '%{http_code}' \
    -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
    -d "$body" "http://$GO_ADDR/dashboard/saasAdmin/paymentRefund" >"$status_file"
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
print("v1=" + hmac.new(secret.encode(), timestamp.encode() + b"." + body, hashlib.sha256).hexdigest())
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

test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SHOW TABLES LIKE 'mochat_go_saas_payment_refunds'" | tr -d '\r')" = "mochat_go_saas_payment_refunds"
test "$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND COLUMN_NAME IN ('refund_pending_amount_cents', 'refunded_amount_cents', 'latest_refund_id')" | tr -d '\r')" = "3"

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
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_payment_refunds' AND INDEX_NAME = 'uni_mochat_go_saas_payment_refunds_idempotency'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND INDEX_NAME = 'idx_mochat_go_saas_payment_orders_refund'")" = "4"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,退款平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
911,退款测试租户,13800000911,secret911,退款租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "2"

start_go
PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_TOKEN="$(login_token 13800000911 secret911 "$WORK_DIR/tenant-auth.json")"

TENANT_FORBIDDEN="$(curl -sS -o "$WORK_DIR/tenant-forbidden.json" -w '%{http_code}' -H "Authorization: Bearer $TENANT_TOKEN" "http://$GO_ADDR/dashboard/saasAdmin/paymentRefunds")"
test "$TENANT_FORBIDDEN" = "403"

ORDER_BODY='{"orderNo":"PAY-REFUND-911","tenantId":911,"provider":"gateway","providerOrderNo":"GW-PAY-REFUND-911","idempotencyKey":"refund-smoke-order-911","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-10 00:00:00","amountCents":100000,"currency":"CNY","remark":"退款主链路验收"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$ORDER_BODY" "$WORK_DIR/order.json"
PAYMENT_EVENT='{"provider":"gateway","eventId":"evt-pay-refund-911","eventType":"payment.succeeded","orderNo":"PAY-REFUND-911","providerOrderNo":"GW-PAY-REFUND-911","amountCents":100000,"currency":"CNY","paidAt":"2026-07-10 09:00:00","occurredAt":"2026-07-10 09:00:00"}'
webhook_post "$PAYMENT_EVENT" "$WORK_DIR/payment-invalid-signature.json" 401 'v1=invalid'
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_webhook_events WHERE event_id = 'evt-pay-refund-911'")" = "0"
webhook_post "$PAYMENT_EVENT" "$WORK_DIR/payment-succeeded.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-REFUND-911'")" = "paid"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 911")" = "active"

CONCURRENT_A='{"refundNo":"REF-CONCURRENT-A-911","orderNo":"PAY-REFUND-911","idempotencyKey":"refund-concurrent-a-911","amountCents":70000,"currency":"CNY","reason":"并发退款 A","entitlementAction":"keep"}'
CONCURRENT_B='{"refundNo":"REF-CONCURRENT-B-911","orderNo":"PAY-REFUND-911","idempotencyKey":"refund-concurrent-b-911","amountCents":70000,"currency":"CNY","reason":"并发退款 B","entitlementAction":"keep"}'
concurrent_refund_post "$PLATFORM_TOKEN" "$CONCURRENT_A" "$WORK_DIR/concurrent-a.json" "$WORK_DIR/concurrent-a.status" &
PID_A="$!"
concurrent_refund_post "$PLATFORM_TOKEN" "$CONCURRENT_B" "$WORK_DIR/concurrent-b.json" "$WORK_DIR/concurrent-b.status" &
PID_B="$!"
wait "$PID_A"
wait "$PID_B"
test "$(sort "$WORK_DIR/concurrent-a.status" "$WORK_DIR/concurrent-b.status" | tr '\n' ' ')" = "200 409 "
test "$(mysql_scalar "SELECT refund_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-REFUND-911'")" = "70000"
WINNER_REFUND="$(mysql_scalar "SELECT refund_no FROM mochat_go_saas_payment_refunds WHERE refund_no IN ('REF-CONCURRENT-A-911', 'REF-CONCURRENT-B-911') LIMIT 1")"
WINNER_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_refunds WHERE refund_no = '$WINNER_REFUND'")"
STALE_VERSION=$((WINNER_VERSION + 1))
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefundCancel" \
  "{\"refundNo\":\"$WINNER_REFUND\",\"expectedVersion\":$STALE_VERSION,\"reason\":\"并发版本冲突\"}" \
  "$WORK_DIR/concurrent-cancel-stale.json" 409
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefundCancel" \
  "{\"refundNo\":\"$WINNER_REFUND\",\"expectedVersion\":$WINNER_VERSION,\"reason\":\"释放并发预占\"}" \
  "$WORK_DIR/concurrent-cancel.json"
test "$(mysql_scalar "SELECT refund_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-REFUND-911'")" = "0"

MISMATCH_BODY='{"refundNo":"REF-MISMATCH-911","orderNo":"PAY-REFUND-911","idempotencyKey":"refund-mismatch-911","amountCents":10000,"currency":"CNY","reason":"错误金额关联验收","entitlementAction":"keep"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" "$MISMATCH_BODY" "$WORK_DIR/mismatch-create.json"
MISMATCH_EVENT='{"provider":"gateway","eventId":"evt-refund-mismatch-911","eventType":"refund.succeeded","orderNo":"PAY-REFUND-911","refundNo":"REF-MISMATCH-911","providerRefundNo":"GW-REF-MISMATCH-911","amountCents":9999,"currency":"CNY","refundedAt":"2026-07-10 09:30:00"}'
webhook_post "$MISMATCH_EVENT" "$WORK_DIR/mismatch-webhook.json" 422
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_webhook_events WHERE event_id = 'evt-refund-mismatch-911'")" = "failed"
test "$(mysql_scalar "SELECT IF(refund_id = (SELECT id FROM mochat_go_saas_payment_refunds WHERE refund_no = 'REF-MISMATCH-911'), 1, 0) FROM mochat_go_saas_payment_webhook_events WHERE event_id = 'evt-refund-mismatch-911'")" = "1"
MISMATCH_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_refunds WHERE refund_no = 'REF-MISMATCH-911'")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefundCancel" \
  "{\"refundNo\":\"REF-MISMATCH-911\",\"expectedVersion\":$MISMATCH_VERSION,\"reason\":\"修正错误回调后取消\"}" \
  "$WORK_DIR/mismatch-cancel.json"

FAILED_BODY='{"refundNo":"REF-FAILED-911","orderNo":"PAY-REFUND-911","idempotencyKey":"refund-failed-911","amountCents":10000,"currency":"CNY","reason":"退款失败验收","entitlementAction":"keep"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" "$FAILED_BODY" "$WORK_DIR/failed-create.json"
FAILED_EVENT='{"provider":"gateway","eventId":"evt-refund-failed-911","eventType":"refund.failed","orderNo":"PAY-REFUND-911","refundNo":"REF-FAILED-911","providerRefundNo":"GW-REF-FAILED-911","occurredAt":"2026-07-10 10:00:00","failureCode":"provider_rejected","failureMessage":"退款被支付渠道拒绝"}'
webhook_post "$FAILED_EVENT" "$WORK_DIR/failed-webhook.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_refunds WHERE refund_no = 'REF-FAILED-911'")" = "failed"
test "$(mysql_scalar "SELECT refund_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-REFUND-911'")" = "0"

PARTIAL_BODY='{"refundNo":"REF-PARTIAL-911","orderNo":"PAY-REFUND-911","providerRefundNo":"GW-REF-PARTIAL-911","idempotencyKey":"refund-partial-911","amountCents":30000,"currency":"CNY","reason":"部分退款验收","entitlementAction":"keep","metadata":{"source":"smoke"}}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" "$PARTIAL_BODY" "$WORK_DIR/partial-create.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" "$PARTIAL_BODY" "$WORK_DIR/partial-replay.json"
grep -q '"idempotent":true' "$WORK_DIR/partial-replay.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" \
  '{"refundNo":"REF-PARTIAL-CONFLICT-911","orderNo":"PAY-REFUND-911","idempotencyKey":"refund-partial-911","amountCents":30001,"currency":"CNY","reason":"幂等冲突","entitlementAction":"keep"}' \
  "$WORK_DIR/partial-conflict.json" 409
PROCESSING_EVENT='{"provider":"gateway","eventId":"evt-refund-processing-911","eventType":"refund.processing","orderNo":"PAY-REFUND-911","refundNo":"REF-PARTIAL-911","providerRefundNo":"GW-REF-PARTIAL-911","occurredAt":"2026-07-10 10:20:00"}'
webhook_post "$PROCESSING_EVENT" "$WORK_DIR/partial-processing.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_refunds WHERE refund_no = 'REF-PARTIAL-911'")" = "processing"
PARTIAL_EVENT='{"provider":"gateway","eventId":"evt-refund-partial-911","eventType":"refund.succeeded","orderNo":"PAY-REFUND-911","providerOrderNo":"GW-PAY-REFUND-911","refundNo":"REF-PARTIAL-911","providerRefundNo":"GW-REF-PARTIAL-911","amountCents":30000,"currency":"CNY","refundedAt":"2026-07-10 10:30:00","occurredAt":"2026-07-10 10:30:00"}'
webhook_post "$PARTIAL_EVENT" "$WORK_DIR/partial-succeeded.json"
webhook_post "$PARTIAL_EVENT" "$WORK_DIR/partial-replay-webhook.json"
grep -q '"duplicate":true' "$WORK_DIR/partial-replay-webhook.json"
PARTIAL_CHANGED='{"provider":"gateway","eventId":"evt-refund-partial-911","eventType":"refund.succeeded","orderNo":"PAY-REFUND-911","refundNo":"REF-PARTIAL-911","providerRefundNo":"GW-REF-PARTIAL-911","amountCents":30001,"currency":"CNY","refundedAt":"2026-07-10 10:30:00"}'
webhook_post "$PARTIAL_CHANGED" "$WORK_DIR/partial-digest-conflict.json" 409
grep -q 'payload digest mismatch' "$WORK_DIR/partial-digest-conflict.json"
test "$(mysql_scalar "SELECT refunded_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-REFUND-911'")" = "30000"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE event_type = 'refund' AND external_order_no = 'REF-PARTIAL-911'")" = "1"

LATE_FAILED_EVENT='{"provider":"gateway","eventId":"evt-refund-late-failed-911","eventType":"refund.failed","orderNo":"PAY-REFUND-911","refundNo":"REF-PARTIAL-911","providerRefundNo":"GW-REF-PARTIAL-911","occurredAt":"2026-07-10 10:40:00","failureCode":"late_failure","failureMessage":"延迟失败回调"}'
webhook_post "$LATE_FAILED_EVENT" "$WORK_DIR/late-failed.json"
grep -q '"ignored":true' "$WORK_DIR/late-failed.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_refunds WHERE refund_no = 'REF-PARTIAL-911'")" = "succeeded"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" \
  '{"refundNo":"REF-OVER-911","orderNo":"PAY-REFUND-911","idempotencyKey":"refund-over-911","amountCents":70001,"currency":"CNY","reason":"超额退款拦截","entitlementAction":"keep"}' \
  "$WORK_DIR/over-refund.json" 409

FULL_BODY='{"refundNo":"REF-FULL-REMAINDER-911","orderNo":"PAY-REFUND-911","providerRefundNo":"GW-REF-FULL-911","idempotencyKey":"refund-full-911","amountCents":70000,"currency":"CNY","reason":"全额退款并取消订阅","entitlementAction":"cancel"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" "$FULL_BODY" "$WORK_DIR/full-create.json"
FULL_EVENT='{"provider":"gateway","eventId":"evt-refund-full-911","eventType":"refund.succeeded","orderNo":"PAY-REFUND-911","providerOrderNo":"GW-PAY-REFUND-911","refundNo":"REF-FULL-REMAINDER-911","providerRefundNo":"GW-REF-FULL-911","amountCents":70000,"currency":"CNY","refundedAt":"2026-07-10 11:00:00","occurredAt":"2026-07-10 11:00:00"}'
webhook_post "$FULL_EVENT" "$WORK_DIR/full-succeeded.json"
test "$(mysql_scalar "SELECT CONCAT(refund_pending_amount_cents, ':', refunded_amount_cents) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-REFUND-911'")" = "0:100000"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 911")" = "canceled"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscription_events WHERE tenant_id = 911 AND event_type = 'refund' AND source = 'payment_refund'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = 911 AND action = 'payment.refund.succeeded'")" = "2"
test "$(mysql_scalar "SELECT SUM(CASE WHEN event_type = 'refund' THEN -amount_cents ELSE amount_cents END) FROM mochat_go_saas_billing_events WHERE tenant_id = 911")" = "0"

OLD_TOKEN_STATUS="$(curl -sS -o "$WORK_DIR/old-token-blocked.json" -w '%{http_code}' -H "Authorization: Bearer $TENANT_TOKEN" "http://$GO_ADDR/dashboard/user/loginShow")"
test "$OLD_TOKEN_STATUS" = "401"
LOGIN_BLOCKED_STATUS="$(curl -sS -o "$WORK_DIR/login-blocked.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000911","password":"secret911"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$LOGIN_BLOCKED_STATUS" = "403"

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/paymentRefunds?tenantId=911&limit=100" >"$WORK_DIR/refunds.json"
python3 - "$WORK_DIR/refunds.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
summary = data["summary"]
assert summary["refundCount"] == 5, summary
assert summary["succeededCount"] == 2, summary
assert summary["failedCount"] == 1, summary
assert summary["canceledCount"] == 2, summary
assert summary["pendingAmountCents"] == 0, summary
assert summary["succeededAmountCents"] == 100000, summary
refunds = {item["refundNo"]: item for item in data["refunds"]}
assert refunds["REF-PARTIAL-911"]["billingEventId"] > 0, refunds
assert refunds["REF-FULL-REMAINDER-911"]["entitlementAction"] == "cancel", refunds
assert refunds["REF-FULL-REMAINDER-911"]["subscriptionEventId"] > 0, refunds
PY

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/paymentOrders?tenantId=911&limit=20" >"$WORK_DIR/orders.json"
python3 - "$WORK_DIR/orders.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
summary = data["summary"]
assert summary["paidAmountCents"] == 100000, summary
assert summary["refundedAmountCents"] == 100000, summary
assert summary["refundPendingCents"] == 0, summary
assert summary["netPaidAmountCents"] == 0, summary
order = data["orders"][0]
assert order["refundStatus"] == "full", order
assert order["refundableAmountCents"] == 0, order
PY

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/businessMetrics?tenantLimit=100&billingLimit=100" >"$WORK_DIR/business-metrics.json"
python3 - "$WORK_DIR/business-metrics.json" <<'PY'
import json
import pathlib
import sys

summary = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["summary"]
assert summary["recentRenewalCount"] == 1, summary
assert summary["recentRefundCount"] == 2, summary
assert summary["recentGrossAmountCents"] == 100000, summary
assert summary["recentRefundAmountCents"] == 100000, summary
assert summary["recentBillingAmountCents"] == 0, summary
PY

curl -sS -f -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/businessTrends?months=1&billingLimit=100&taskLimit=100" >"$WORK_DIR/business-trends.json"
python3 - "$WORK_DIR/business-trends.json" <<'PY'
import json
import pathlib
import sys

summary = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["summary"]
assert summary["billingEventCount"] == 3, summary
assert summary["renewalCount"] == 1, summary
assert summary["refundCount"] == 2, summary
assert summary["grossAmountCents"] == 100000, summary
assert summary["refundAmountCents"] == 100000, summary
assert summary["billingAmountCents"] == 0, summary
PY

curl -sS -f -D "$WORK_DIR/refund-csv.headers" -o "$WORK_DIR/refunds.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=paymentRefunds&tenantId=911&limit=1000"
grep -qi 'filename="mochat-saas-payment-refunds-' "$WORK_DIR/refund-csv.headers"
python3 - "$WORK_DIR/refunds.csv" <<'PY'
import csv
import pathlib
import sys

rows = list(csv.DictReader(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8-sig").splitlines()))
assert len(rows) == 5, rows
assert next(row for row in rows if row["refundNo"] == "REF-FULL-REMAINDER-911")["status"] == "succeeded"
PY

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'id="paymentRefunds"' "$WORK_DIR/page.html"
grep -q 'id="createPaymentRefund"' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/paymentRefund'" "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/paymentRefunds' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/paymentRefund' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/paymentRefundCancel' "$WORK_DIR/routes.json"

echo "SaaS payment refund smoke passed"
