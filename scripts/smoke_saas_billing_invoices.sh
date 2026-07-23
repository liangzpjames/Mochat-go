#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-billing-invoice-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13383}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26433}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18155}"
DATABASE="mochat_billing_invoices"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-billing-invoices.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-billing-invoice-jwt-secret}"
WEBHOOK_SECRET="${MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET:-billing-invoice-webhook-secret-2026}"
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
trap 'echo "SaaS billing invoice smoke failed at line $LINENO" >&2; tail -120 "$GO_LOG" >&2 || true' ERR
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

api_get() {
  local token="$1"
  local path="$2"
  local output="$3"
  local expected="${4:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  if [ "$status" != "$expected" ]; then
    echo "GET $path returned $status, expected $expected" >&2
    cat "$output" >&2 || true
    return 1
  fi
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

concurrent_invoice_post() {
  local token="$1"
  local body="$2"
  local output="$3"
  local status_file="$4"
  curl -sS -o "$output" -w '%{http_code}' \
    -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
    -d "$body" "http://$GO_ADDR/dashboard/saasAdmin/invoice" >"$status_file"
}

webhook_post() {
  local body="$1"
  local output="$2"
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
  status="$(curl -sS -o "$output" -w '%{http_code}' \
    -H "Content-Type: application/json" \
    -H "X-Mochat-Go-Payment-Timestamp: $timestamp" \
    -H "X-Mochat-Go-Payment-Signature: $signature" \
    --data-binary "@$body_file" "http://$GO_ADDR/webhooks/saas/payment")"
  if [ "$status" != "200" ]; then
    echo "payment webhook returned $status" >&2
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
    MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL=1 \
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
grep -q $'0042_saas_billing_invoices\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0043_saas_payment_settlements\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0045_saas_admin_rbac\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0046_saas_admin_approvals\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0047_saas_admin_approval_governance\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0049_saas_service_accounts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_billing_profiles'")" = "mochat_go_saas_billing_profiles"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_invoice_documents'")" = "mochat_go_saas_invoice_documents"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,发票平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
921,发票租户甲,13800000921,secret921,租户甲管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
922,发票租户乙,13800000922,secret922,租户乙管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "3"

start_go
PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_A_TOKEN="$(login_token 13800000921 secret921 "$WORK_DIR/tenant-a-auth.json")"
TENANT_B_TOKEN="$(login_token 13800000922 secret922 "$WORK_DIR/tenant-b-auth.json")"

api_get "$TENANT_A_TOKEN" "/dashboard/saasAdmin/invoiceDocuments" "$WORK_DIR/tenant-admin-forbidden.json" 403
api_get "$TENANT_A_TOKEN" "/dashboard/saasBilling/summary?tenantId=922" "$WORK_DIR/tenant-a-empty-summary.json"
api_get "$TENANT_B_TOKEN" "/dashboard/saasBilling/summary?tenantId=921" "$WORK_DIR/tenant-b-empty-summary.json"

PROFILE_CREATE='{"tenantId":922,"invoiceType":"normal","invoiceTitle":"发票租户甲有限公司","taxIdentifier":"91310000TEST000921","email":"billing921@example.com","phone":"021-88889999","recipientName":"财务部","expectedVersion":0,"remark":"租户自助维护"}'
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoiceProfile" "$PROFILE_CREATE" "$WORK_DIR/profile-create.json"
test "$(mysql_scalar "SELECT CONCAT(tenant_id, ':', version) FROM mochat_go_saas_billing_profiles")" = "921:1"
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoiceProfile" \
  '{"tenantId":922,"invoiceType":"normal","invoiceTitle":"错误版本","taxIdentifier":"91310000TEST000921","email":"billing921@example.com","expectedVersion":99}' \
  "$WORK_DIR/profile-stale.json" 409
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoiceProfile" \
  '{"tenantId":922,"invoiceType":"special","invoiceTitle":"发票租户甲有限公司","taxIdentifier":"91310000TEST000921","email":"billing921@example.com","phone":"021-88889999","registeredAddress":"上海市测试路 921 号","bankName":"测试银行上海分行","bankAccount":"6222000000000921","recipientName":"财务部","expectedVersion":1,"remark":"升级专票资料"}' \
  "$WORK_DIR/profile-update.json"
test "$(mysql_scalar "SELECT CONCAT(tenant_id, ':', invoice_type, ':', version) FROM mochat_go_saas_billing_profiles")" = "921:special:2"
api_get "$TENANT_B_TOKEN" "/dashboard/saasBilling/invoiceProfile?tenantId=921" "$WORK_DIR/tenant-b-profile.json"
grep -q '"exists":false' "$WORK_DIR/tenant-b-profile.json"

ORDER_BODY='{"orderNo":"PAY-INVOICE-921","tenantId":921,"provider":"gateway","providerOrderNo":"GW-PAY-INVOICE-921","idempotencyKey":"invoice-smoke-order-921","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-10 00:00:00","amountCents":100000,"currency":"CNY","remark":"发票主链路验收"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$ORDER_BODY" "$WORK_DIR/order.json"
PAYMENT_EVENT='{"provider":"gateway","eventId":"evt-pay-invoice-921","eventType":"payment.succeeded","orderNo":"PAY-INVOICE-921","providerOrderNo":"GW-PAY-INVOICE-921","amountCents":100000,"currency":"CNY","paidAt":"2026-07-10 09:00:00","occurredAt":"2026-07-10 09:00:00"}'
webhook_post "$PAYMENT_EVENT" "$WORK_DIR/payment-succeeded.json"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-921'")" = "paid"

TEMP_INVOICE='{"tenantId":922,"documentNo":"INV-TEMP-921","orderNo":"PAY-INVOICE-921","amountCents":10000,"currency":"CNY","provider":"manual","idempotencyKey":"invoice-temp-921","remark":"租户取消释放预占"}'
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoice" "$TEMP_INVOICE" "$WORK_DIR/temp-invoice.json"
test "$(mysql_scalar "SELECT invoice_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-921'")" = "10000"
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoiceCancel" \
  '{"tenantId":922,"documentNo":"INV-TEMP-921","expectedVersion":1,"reason":"租户撤回测试"}' \
  "$WORK_DIR/temp-invoice-cancel.json"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-TEMP-921'")" = "canceled:2"
test "$(mysql_scalar "SELECT invoice_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-921'")" = "0"

MAIN_INVOICE='{"tenantId":922,"documentNo":"INV-MAIN-921","orderNo":"PAY-INVOICE-921","amountCents":100000,"currency":"CNY","provider":"manual","idempotencyKey":"invoice-main-921","remark":"全额蓝票"}'
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoice" "$MAIN_INVOICE" "$WORK_DIR/main-invoice.json"
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoice" "$MAIN_INVOICE" "$WORK_DIR/main-invoice-replay.json"
grep -q '"idempotent":true' "$WORK_DIR/main-invoice-replay.json"
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoice" \
  '{"documentNo":"INV-MAIN-CONFLICT-921","orderNo":"PAY-INVOICE-921","amountCents":99999,"currency":"CNY","idempotencyKey":"invoice-main-921"}' \
  "$WORK_DIR/main-invoice-conflict.json" 409
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoice" \
  '{"documentNo":"INV-OVER-921","orderNo":"PAY-INVOICE-921","amountCents":1,"currency":"CNY","idempotencyKey":"invoice-over-921"}' \
  "$WORK_DIR/main-invoice-over.json" 409
api_post "$TENANT_B_TOKEN" "/dashboard/saasBilling/invoiceCancel" \
  '{"documentNo":"INV-MAIN-921","expectedVersion":1,"reason":"跨租户撤回应不可见"}' \
  "$WORK_DIR/cross-tenant-cancel.json" 404
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" \
  '{"refundNo":"REF-BLOCKED-BY-INVOICE-921","orderNo":"PAY-INVOICE-921","idempotencyKey":"refund-blocked-invoice-921","amountCents":1,"currency":"CNY","reason":"蓝票预占退款冲突","entitlementAction":"keep"}' \
  "$WORK_DIR/refund-blocked-by-pending-invoice.json" 409

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-MAIN-921","expectedVersion":1,"status":"processing","provider":"manual","remark":"财务审核通过"}' \
  "$WORK_DIR/main-processing.json"
api_post "$TENANT_A_TOKEN" "/dashboard/saasBilling/invoiceCancel" \
  '{"documentNo":"INV-MAIN-921","expectedVersion":2,"reason":"处理中撤回应被拦截"}' \
  "$WORK_DIR/main-tenant-cancel-blocked.json" 409
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-MAIN-921","expectedVersion":2,"status":"issued","provider":"manual","providerDocumentNo":"FP-MAIN-921","documentUrl":"https://invoice.example.test/FP-MAIN-921.pdf","issuedAt":"2026-07-10 10:00:00","remark":"蓝票已开具"}' \
  "$WORK_DIR/main-issued.json"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-921'")" = "0:100000"

REFUND_BODY='{"refundNo":"REF-INVOICE-921","orderNo":"PAY-INVOICE-921","providerRefundNo":"GW-REF-INVOICE-921","idempotencyKey":"refund-invoice-921","amountCents":30000,"currency":"CNY","reason":"开票后部分退款","entitlementAction":"keep"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" "$REFUND_BODY" "$WORK_DIR/refund-create.json"
REFUND_EVENT='{"provider":"gateway","eventId":"evt-refund-invoice-921","eventType":"refund.succeeded","orderNo":"PAY-INVOICE-921","refundNo":"REF-INVOICE-921","providerRefundNo":"GW-REF-INVOICE-921","amountCents":30000,"currency":"CNY","refundedAt":"2026-07-10 10:30:00","occurredAt":"2026-07-10 10:30:00"}'
webhook_post "$REFUND_EVENT" "$WORK_DIR/refund-succeeded.json"
test "$(mysql_scalar "SELECT CONCAT(refunded_amount_cents, ':', invoiced_amount_cents - credited_amount_cents) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-921'")" = "30000:100000"

CREDIT_BODY='{"documentNo":"CRN-MAIN-921","tenantId":921,"orderNo":"PAY-INVOICE-921","originalDocumentNo":"INV-MAIN-921","amountCents":30000,"currency":"CNY","provider":"manual","idempotencyKey":"credit-main-921","remark":"部分退款红冲"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/creditNote" "$CREDIT_BODY" "$WORK_DIR/credit-create.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/creditNote" "$CREDIT_BODY" "$WORK_DIR/credit-replay.json"
grep -q '"idempotent":true' "$WORK_DIR/credit-replay.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/creditNote" \
  '{"documentNo":"CRN-OVER-921","tenantId":921,"orderNo":"PAY-INVOICE-921","originalDocumentNo":"INV-MAIN-921","amountCents":1,"currency":"CNY","idempotencyKey":"credit-over-921"}' \
  "$WORK_DIR/credit-over.json" 409
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"CRN-MAIN-921","expectedVersion":1,"status":"issued","provider":"manual","providerDocumentNo":"FP-CRN-921","documentUrl":"https://invoice.example.test/FP-CRN-921.pdf","issuedAt":"2026-07-10 11:00:00","remark":"红票已开具"}' \
  "$WORK_DIR/credit-issued.json"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents, ':', credit_pending_amount_cents, ':', credited_amount_cents) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-921'")" = "0:100000:0:30000"

api_get "$TENANT_A_TOKEN" "/dashboard/saasBilling/summary?tenantId=922" "$WORK_DIR/tenant-a-summary.json"
api_get "$TENANT_B_TOKEN" "/dashboard/saasBilling/summary?tenantId=921" "$WORK_DIR/tenant-b-summary.json"
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrders?tenantId=921&limit=20" "$WORK_DIR/admin-orders.json"
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceDocuments?tenantId=921&limit=100" "$WORK_DIR/admin-invoices.json"
python3 - "$WORK_DIR/tenant-a-summary.json" "$WORK_DIR/tenant-b-summary.json" "$WORK_DIR/admin-orders.json" "$WORK_DIR/admin-invoices.json" <<'PY'
import json
import pathlib
import sys

tenant_a, tenant_b, admin_orders, admin_invoices = [
    json.loads(pathlib.Path(path).read_text(encoding="utf-8"))["data"] for path in sys.argv[1:]
]
assert tenant_a["tenantId"] == 921, tenant_a
assert tenant_a["profile"]["tenantId"] == 921 and tenant_a["profile"]["version"] == 2, tenant_a
order_summary = tenant_a["paymentOrders"]["summary"]
assert order_summary["paidAmountCents"] == 100000, order_summary
assert order_summary["refundedAmountCents"] == 30000, order_summary
assert order_summary["netPaidAmountCents"] == 70000, order_summary
assert order_summary["invoicedAmountCents"] == 100000, order_summary
assert order_summary["creditedAmountCents"] == 30000, order_summary
assert order_summary["netInvoicedCents"] == 70000, order_summary
assert order_summary["invoiceAvailableCents"] == 0, order_summary
assert order_summary["creditNoteDueCents"] == 0, order_summary
invoice_summary = tenant_a["invoices"]["summary"]
assert invoice_summary["documentCount"] == 3, invoice_summary
assert invoice_summary["invoiceCount"] == 2 and invoice_summary["creditNoteCount"] == 1, invoice_summary
assert invoice_summary["issuedInvoiceAmountCents"] == 100000, invoice_summary
assert invoice_summary["issuedCreditAmountCents"] == 30000, invoice_summary
assert invoice_summary["netIssuedAmountCents"] == 70000, invoice_summary
assert tenant_b["tenantId"] == 922, tenant_b
assert tenant_b["paymentOrders"]["summary"]["orderCount"] == 0, tenant_b
assert tenant_b["invoices"]["summary"]["documentCount"] == 0, tenant_b
admin_order = admin_orders["orders"][0]
assert admin_order["amountCents"] - admin_order["refundedAmountCents"] == admin_order["netInvoicedCents"] == 70000, admin_order
assert admin_order["invoiceStatus"] == "invoiced", admin_order
documents = {item["documentNo"]: item for item in admin_invoices["documents"]}
assert documents["INV-TEMP-921"]["status"] == "canceled", documents
assert documents["INV-MAIN-921"]["status"] == "issued", documents
assert documents["CRN-MAIN-921"]["status"] == "issued", documents
PY

api_get "$TENANT_A_TOKEN" "/dashboard/saasBilling/paymentOrders?tenantId=922&limit=20" "$WORK_DIR/tenant-a-orders-scope.json"
grep -q '"tenantId":921' "$WORK_DIR/tenant-a-orders-scope.json"
if grep -q '"tenantId":922' "$WORK_DIR/tenant-a-orders-scope.json"; then
  echo "tenant A response leaked tenant B" >&2
  exit 1
fi

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceProfile" \
  '{"tenantId":922,"invoiceType":"normal","invoiceTitle":"发票租户乙有限公司","taxIdentifier":"91310000TEST000922","email":"billing922@example.com","expectedVersion":0,"remark":"平台代维护"}' \
  "$WORK_DIR/tenant-b-profile-create.json"
SECOND_ORDER='{"orderNo":"PAY-INVOICE-922","tenantId":922,"provider":"gateway","providerOrderNo":"GW-PAY-INVOICE-922","idempotencyKey":"invoice-smoke-order-922","packageCode":"starter","billingCycle":"yearly","serviceExpiresAt":"2038-01-10 00:00:00","amountCents":20000,"currency":"CNY","remark":"失败释放验收"}'
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$SECOND_ORDER" "$WORK_DIR/tenant-b-order.json"
SECOND_PAYMENT_EVENT='{"provider":"gateway","eventId":"evt-pay-invoice-922","eventType":"payment.succeeded","orderNo":"PAY-INVOICE-922","providerOrderNo":"GW-PAY-INVOICE-922","amountCents":20000,"currency":"CNY","paidAt":"2026-07-10 12:00:00","occurredAt":"2026-07-10 12:00:00"}'
webhook_post "$SECOND_PAYMENT_EVENT" "$WORK_DIR/tenant-b-payment-succeeded.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoice" \
  '{"documentNo":"INV-FAILED-922","tenantId":922,"orderNo":"PAY-INVOICE-922","amountCents":12000,"currency":"CNY","provider":"manual","idempotencyKey":"invoice-failed-922","remark":"平台代开发票"}' \
  "$WORK_DIR/tenant-b-invoice-create.json"
test "$(mysql_scalar "SELECT invoice_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-922'")" = "12000"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-FAILED-922","expectedVersion":1,"status":"failed","provider":"manual","failureCode":"profile_rejected","failureMessage":"开票渠道拒绝测试资料","remark":"失败后释放预占"}' \
  "$WORK_DIR/tenant-b-invoice-failed.json"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-922'")" = "0:0"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-FAILED-922'")" = "failed:2"
api_get "$TENANT_B_TOKEN" "/dashboard/saasBilling/summary?tenantId=921" "$WORK_DIR/tenant-b-own-summary.json"
python3 - "$WORK_DIR/tenant-b-own-summary.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["tenantId"] == 922, data
assert data["paymentOrders"]["summary"]["orderCount"] == 1, data
assert data["paymentOrders"]["summary"]["invoiceAvailableCents"] == 20000, data
assert data["invoices"]["summary"]["documentCount"] == 1, data
assert data["invoices"]["documents"][0]["documentNo"] == "INV-FAILED-922", data
PY

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoice" \
  '{"documentNo":"INV-DUPLICATE-PROVIDER-922","tenantId":922,"orderNo":"PAY-INVOICE-922","amountCents":5000,"currency":"CNY","provider":"manual","idempotencyKey":"invoice-duplicate-provider-922"}' \
  "$WORK_DIR/duplicate-provider-create.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-DUPLICATE-PROVIDER-922","expectedVersion":1,"status":"issued","provider":"manual","providerDocumentNo":"FP-MAIN-921","documentUrl":"https://invoice.example.test/duplicate.pdf"}' \
  "$WORK_DIR/duplicate-provider-conflict.json" 409
grep -q 'invoice provider document number already exists' "$WORK_DIR/duplicate-provider-conflict.json"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-DUPLICATE-PROVIDER-922'")" = "requested:1"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-922'")" = "5000:0"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-DUPLICATE-PROVIDER-922","expectedVersion":1,"status":"canceled","provider":"manual","remark":"重复渠道单号后释放预占"}' \
  "$WORK_DIR/duplicate-provider-cancel.json"
test "$(mysql_scalar "SELECT invoice_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-922'")" = "0"

CONCURRENT_INVOICE_A='{"documentNo":"INV-CONCURRENT-A-922","tenantId":922,"orderNo":"PAY-INVOICE-922","amountCents":15000,"currency":"CNY","provider":"manual","idempotencyKey":"invoice-concurrent-a-922"}'
CONCURRENT_INVOICE_B='{"documentNo":"INV-CONCURRENT-B-922","tenantId":922,"orderNo":"PAY-INVOICE-922","amountCents":15000,"currency":"CNY","provider":"manual","idempotencyKey":"invoice-concurrent-b-922"}'
concurrent_invoice_post "$PLATFORM_TOKEN" "$CONCURRENT_INVOICE_A" "$WORK_DIR/concurrent-invoice-a.json" "$WORK_DIR/concurrent-invoice-a.status" &
PID_A="$!"
concurrent_invoice_post "$PLATFORM_TOKEN" "$CONCURRENT_INVOICE_B" "$WORK_DIR/concurrent-invoice-b.json" "$WORK_DIR/concurrent-invoice-b.status" &
PID_B="$!"
wait "$PID_A"
wait "$PID_B"
test "$(sort "$WORK_DIR/concurrent-invoice-a.status" "$WORK_DIR/concurrent-invoice-b.status" | tr '\n' ' ')" = "200 409 "
test "$(mysql_scalar "SELECT invoice_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-922'")" = "15000"
WINNER_INVOICE="$(mysql_scalar "SELECT document_no FROM mochat_go_saas_invoice_documents WHERE document_no IN ('INV-CONCURRENT-A-922', 'INV-CONCURRENT-B-922') AND status = 'requested' LIMIT 1")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  "{\"documentNo\":\"$WINNER_INVOICE\",\"expectedVersion\":1,\"status\":\"canceled\",\"provider\":\"manual\",\"remark\":\"释放并发预占\"}" \
  "$WORK_DIR/concurrent-invoice-cancel.json"
test "$(mysql_scalar "SELECT invoice_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-INVOICE-922'")" = "0"

curl -sS -f -D "$WORK_DIR/invoice-csv.headers" -o "$WORK_DIR/invoices.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=invoiceDocuments&tenantId=921&limit=1000"
grep -qi 'filename="mochat-saas-invoice-documents-' "$WORK_DIR/invoice-csv.headers"
python3 - "$WORK_DIR/invoices.csv" <<'PY'
import csv
import pathlib
import sys

rows = list(csv.DictReader(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8-sig").splitlines()))
assert len(rows) == 3, rows
assert {row["kind"] for row in rows} == {"invoice", "credit_note"}, rows
PY

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/admin-page.html"
curl -sS -f "http://$GO_ADDR/dashboard/saasBilling/page" >"$WORK_DIR/tenant-page.html"
grep -q 'id="invoiceDocuments"' "$WORK_DIR/admin-page.html"
grep -q 'id="createInvoiceDocument"' "$WORK_DIR/admin-page.html"
grep -q 'id="billingSummary"' "$WORK_DIR/tenant-page.html"
grep -q 'id="requestInvoice"' "$WORK_DIR/tenant-page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
for route in \
  'GET /dashboard/saasBilling/summary' \
  'POST /dashboard/saasBilling/invoice' \
  'POST /dashboard/saasBilling/invoiceCancel' \
  'GET /dashboard/saasAdmin/invoiceDocuments' \
  'POST /dashboard/saasAdmin/creditNote' \
  'POST /dashboard/saasAdmin/invoiceTransition'; do
  grep -q "$route" "$WORK_DIR/routes.json"
done

grep -q 'go SaaS billing route enabled: GET /dashboard/saasBilling/summary' "$GO_LOG"
grep -q 'go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/creditNote' "$GO_LOG"

echo "SaaS billing invoice smoke passed"
