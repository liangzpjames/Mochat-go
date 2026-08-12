#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-payment-settlement-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13384}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26434}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18156}"
DATABASE="mochat_payment_settlements"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-payment-settlements.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-payment-settlement-jwt-secret}"
WEBHOOK_SECRET="${MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET:-payment-settlement-webhook-secret-2026}"
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
trap 'echo "SaaS payment settlement smoke failed at line $LINENO" >&2; tail -160 "$GO_LOG" >&2 || true' ERR
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
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
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
print(payload["data"]["token"])
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

api_post_file() {
  local token="$1"
  local path="$2"
  local content_type="$3"
  local input="$4"
  local output="$5"
  local expected="${6:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' \
    -H "Authorization: Bearer $token" -H "Content-Type: $content_type" \
    --data-binary "@$input" "http://$GO_ADDR$path")"
  if [ "$status" != "$expected" ]; then
    echo "POST $path returned $status, expected $expected" >&2
    cat "$output" >&2 || true
    return 1
  fi
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
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK=1 \
    MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET="$WEBHOOK_SECRET" \
    MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS=300 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
}

create_order() {
  local order_no="$1"
  local provider_no="$2"
  local amount="$3"
  local output="$4"
  api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentOrder" \
    "{\"orderNo\":\"$order_no\",\"tenantId\":931,\"provider\":\"gateway\",\"providerOrderNo\":\"$provider_no\",\"idempotencyKey\":\"settlement-$order_no\",\"packageCode\":\"starter\",\"billingCycle\":\"yearly\",\"serviceExpiresAt\":\"2037-12-31 00:00:00\",\"amountCents\":$amount,\"currency\":\"CNY\",\"remark\":\"结算对账验收\"}" \
    "$output"
}

pay_order() {
  local order_no="$1"
  local provider_no="$2"
  local amount="$3"
  local event_id="$4"
  local output="$5"
  webhook_post "{\"provider\":\"gateway\",\"eventId\":\"$event_id\",\"eventType\":\"payment.succeeded\",\"orderNo\":\"$order_no\",\"providerOrderNo\":\"$provider_no\",\"amountCents\":$amount,\"currency\":\"CNY\",\"paidAt\":\"2026-07-10 08:00:00\",\"occurredAt\":\"2026-07-10 08:00:00\"}" "$output"
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
grep -q $'0043_saas_payment_settlements\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0045_saas_admin_rbac\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0046_saas_admin_approvals\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0047_saas_admin_approval_governance\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0049_saas_service_accounts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_batches'")" = "mochat_go_saas_payment_settlement_batches"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_entries'")" = "mochat_go_saas_payment_settlement_entries"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,结算平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
931,结算业务租户,13800000931,secret931,结算租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "2"

start_go
PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_TOKEN="$(login_token 13800000931 secret931 "$WORK_DIR/tenant-auth.json")"

api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/paymentSettlementBatches" "$WORK_DIR/tenant-forbidden.json" 403

create_order PAY-SET-A GW-ORDER-A 10000 "$WORK_DIR/order-a.json"
pay_order PAY-SET-A GW-ORDER-A 10000 evt-set-a "$WORK_DIR/pay-a.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentRefund" \
  '{"refundNo":"REF-SET-A","orderNo":"PAY-SET-A","providerRefundNo":"GW-REFUND-A","idempotencyKey":"settlement-refund-a","amountCents":3000,"currency":"CNY","reason":"结算退款验收","entitlementAction":"keep"}' \
  "$WORK_DIR/refund-a.json"
webhook_post '{"provider":"gateway","eventId":"evt-ref-set-a","eventType":"refund.succeeded","orderNo":"PAY-SET-A","providerOrderNo":"GW-ORDER-A","refundNo":"REF-SET-A","providerRefundNo":"GW-REFUND-A","amountCents":3000,"currency":"CNY","refundedAt":"2026-07-10 09:00:00","occurredAt":"2026-07-10 09:00:00"}' "$WORK_DIR/refund-a-paid.json"

create_order PAY-SET-B GW-ORDER-B 2000 "$WORK_DIR/order-b.json"
pay_order PAY-SET-B GW-ORDER-B 2000 evt-set-b "$WORK_DIR/pay-b.json"
create_order PAY-SET-C GW-ORDER-C 5000 "$WORK_DIR/order-c.json"
create_order PAY-SET-D GW-ORDER-D 7000 "$WORK_DIR/order-d.json"
pay_order PAY-SET-D GW-ORDER-D 7000 evt-set-d "$WORK_DIR/pay-d.json"
create_order PAY-SET-E GW-ORDER-E 6000 "$WORK_DIR/order-e.json"
pay_order PAY-SET-E GW-ORDER-E 6000 evt-set-e "$WORK_DIR/pay-e.json"
create_order PAY-SET-F GW-ORDER-F 6100 "$WORK_DIR/order-f.json"
pay_order PAY-SET-F GW-ORDER-F 6100 evt-set-f "$WORK_DIR/pay-f.json"
create_order PAY-SET-G GW-ORDER-G 8800 "$WORK_DIR/order-g.json"
pay_order PAY-SET-G GW-ORDER-G 8800 evt-set-g "$WORK_DIR/pay-g.json"

cat >"$WORK_DIR/settlement.json" <<'JSON'
{"batchNo":"SET-MAIN-931","provider":"gateway","providerSettlementNo":"GW-SET-MAIN-931","periodStart":"2026-07-10 00:00:00","periodEnd":"2026-07-11 00:00:00","currency":"CNY","remark":"渠道日结主批次","entries":[{"lineNo":1,"providerTransactionNo":"GW-SET-TXN-A","transactionType":"payment","orderNo":"PAY-SET-A","providerOrderNo":"GW-ORDER-A","amountCents":10000,"feeCents":-60,"netAmountCents":9940,"occurredAt":"2026-07-10 08:00:00"},{"lineNo":2,"providerTransactionNo":"GW-SET-REF-A","transactionType":"refund","refundNo":"REF-SET-A","providerRefundNo":"GW-REFUND-A","amountCents":-3000,"feeCents":0,"netAmountCents":-3000,"occurredAt":"2026-07-10 09:00:00"},{"lineNo":3,"providerTransactionNo":"GW-SET-MISSING","transactionType":"payment","orderNo":"PAY-MISSING","amountCents":500,"feeCents":0,"netAmountCents":500},{"lineNo":4,"providerTransactionNo":"GW-SET-AMOUNT","transactionType":"payment","orderNo":"PAY-SET-B","providerOrderNo":"GW-ORDER-B","amountCents":1999,"feeCents":0,"netAmountCents":1999},{"lineNo":5,"providerTransactionNo":"GW-SET-STATUS","transactionType":"payment","orderNo":"PAY-SET-C","providerOrderNo":"GW-ORDER-C","amountCents":5000,"feeCents":0,"netAmountCents":5000},{"lineNo":6,"providerTransactionNo":"GW-SET-CURRENCY","transactionType":"payment","orderNo":"PAY-SET-D","providerOrderNo":"GW-ORDER-D","amountCents":7000,"feeCents":0,"netAmountCents":7000,"currency":"USD"},{"lineNo":7,"providerTransactionNo":"GW-SET-IDENTIFIER","transactionType":"payment","orderNo":"PAY-SET-E","providerOrderNo":"GW-ORDER-F","amountCents":6000,"feeCents":0,"netAmountCents":6000}]}
JSON

api_post_file "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementImport" "application/json" "$WORK_DIR/settlement.json" "$WORK_DIR/import.json"
python3 - "$WORK_DIR/import.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
batch = data["batch"]
summary = data["entries"]["summary"]
assert data["idempotent"] is False, data
assert batch["batchNo"] == "SET-MAIN-931" and batch["version"] == 1, batch
assert batch["entryCount"] == 7 and batch["matchedCount"] == 2, batch
assert batch["issueCount"] == 5 and batch["openIssueCount"] == 5, batch
assert batch["totalAmountCents"] == 27499 and batch["totalFeeCents"] == -60 and batch["totalNetCents"] == 27439, batch
assert batch["differenceAmountCents"] == 499, batch
assert summary["missingInternalCount"] == 1 and summary["amountMismatchCount"] == 1, summary
assert summary["statusMismatchCount"] == 1 and summary["currencyMismatchCount"] == 1, summary
assert summary["identifierConflictCount"] == 1, summary
PY

api_post_file "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementImport" "application/json" "$WORK_DIR/settlement.json" "$WORK_DIR/import-replay.json"
grep -q '"idempotent":true' "$WORK_DIR/import-replay.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_batches")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries")" = "7"

python3 - "$WORK_DIR/settlement.json" "$WORK_DIR/settlement-conflict.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
payload["remark"] = "同渠道批次不同内容"
pathlib.Path(sys.argv[2]).write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
PY
api_post_file "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementImport" "application/json" "$WORK_DIR/settlement-conflict.json" "$WORK_DIR/import-conflict.json" 409

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementBatches?provider=gateway&status=reconciled&keyword=MAIN&limit=20" "$WORK_DIR/batches.json"
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementEntries?batchNo=SET-MAIN-931&handlingStatus=open&limit=20" "$WORK_DIR/open-entries.json"
python3 - "$WORK_DIR/batches.json" "$WORK_DIR/open-entries.json" <<'PY'
import json
import pathlib
import sys

batches, entries = [json.loads(pathlib.Path(path).read_text(encoding="utf-8"))["data"] for path in sys.argv[1:]]
assert batches["summary"]["batchCount"] == 1 and batches["batches"][0]["batchNo"] == "SET-MAIN-931", batches
assert entries["summary"]["entryCount"] == 5 and entries["summary"]["openIssueCount"] == 5, entries
PY

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementTransition" \
  '{"batchNo":"SET-MAIN-931","expectedVersion":1,"action":"close","reason":"不应允许带未处理差异关闭"}' \
  "$WORK_DIR/close-blocked.json" 409

while IFS=$'\t' read -r entry_id handling_status; do
  version="$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_entries WHERE id = $entry_id")"
  api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementResolve" \
    "{\"entryId\":$entry_id,\"expectedVersion\":$version,\"handlingStatus\":\"$handling_status\",\"reason\":\"财务人工核验 $entry_id\"}" \
    "$WORK_DIR/resolve-$entry_id.json"
done < <(mysql_scalar "SELECT id, CASE reconciliation_status WHEN 'missing_internal' THEN 'ignored' WHEN 'currency_mismatch' THEN 'ignored' ELSE 'resolved' END FROM mochat_go_saas_payment_settlement_entries WHERE batch_id = 1 AND reconciliation_status <> 'matched' ORDER BY id")

test "$(mysql_scalar "SELECT open_issue_count FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-MAIN-931'")" = "0"
BATCH_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-MAIN-931'")"
test "$BATCH_VERSION" = "6"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementTransition" \
  "{\"batchNo\":\"SET-MAIN-931\",\"expectedVersion\":$BATCH_VERSION,\"action\":\"close\",\"reason\":\"首轮差异已处理\"}" \
  "$WORK_DIR/close.json"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-MAIN-931'")" = "closed:7"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementReconcile" \
  '{"batchNo":"SET-MAIN-931","expectedVersion":7,"dryRun":false}' "$WORK_DIR/reconcile-closed.json" 409
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementTransition" \
  '{"batchNo":"SET-MAIN-931","expectedVersion":7,"action":"reopen","reason":"渠道补发状态后重核"}' "$WORK_DIR/reopen.json"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-MAIN-931'")" = "reconciled:8"

pay_order PAY-SET-C GW-ORDER-C 5000 evt-set-c "$WORK_DIR/pay-c.json"
STATUS_ENTRY_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_payment_settlement_entries WHERE provider_transaction_no = 'GW-SET-STATUS'")"
test "$(mysql_scalar "SELECT CONCAT(reconciliation_status, ':', handling_status) FROM mochat_go_saas_payment_settlement_entries WHERE id = $STATUS_ENTRY_ID")" = "status_mismatch:resolved"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementReconcile" \
  '{"batchNo":"SET-MAIN-931","expectedVersion":8,"dryRun":true}' "$WORK_DIR/reconcile-preview.json"
python3 - "$WORK_DIR/reconcile-preview.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["dryRun"] is True and data["batch"]["matchedCount"] == 3, data
entry = next(item for item in data["entries"]["entries"] if item["providerTransactionNo"] == "GW-SET-STATUS")
assert entry["reconciliationStatus"] == "matched" and entry["handlingStatus"] == "none", entry
PY
test "$(mysql_scalar "SELECT CONCAT(reconciliation_status, ':', handling_status) FROM mochat_go_saas_payment_settlement_entries WHERE id = $STATUS_ENTRY_ID")" = "status_mismatch:resolved"
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-MAIN-931'")" = "8"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementReconcile" \
  '{"batchNo":"SET-MAIN-931","expectedVersion":8,"dryRun":false}' "$WORK_DIR/reconcile-apply.json"
test "$(mysql_scalar "SELECT CONCAT(reconciliation_status, ':', handling_status) FROM mochat_go_saas_payment_settlement_entries WHERE id = $STATUS_ENTRY_ID")" = "matched:none"
test "$(mysql_scalar "SELECT CONCAT(matched_count, ':', issue_count, ':', open_issue_count, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-MAIN-931'")" = "3:4:0:9"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementTransition" \
  '{"batchNo":"SET-MAIN-931","expectedVersion":9,"action":"close","reason":"补状态后复核完成"}' "$WORK_DIR/reclose.json"

cat >"$WORK_DIR/settlement.csv" <<'CSV'
batchNo,provider,providerSettlementNo,periodStart,periodEnd,currency,lineNo,providerTransactionNo,transactionType,orderNo,providerOrderNo,refundNo,providerRefundNo,amountCents,feeCents,netAmountCents,occurredAt,remark
SET-CSV-931,gateway,GW-SET-CSV-931,2026-07-11 00:00:00,2026-07-12 00:00:00,CNY,1,GW-SET-TXN-G,payment,PAY-SET-G,GW-ORDER-G,,,8800,-40,8760,2026-07-11 08:00:00,CSV 实际导入
CSV
api_post_file "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementImport" "text/csv" "$WORK_DIR/settlement.csv" "$WORK_DIR/import-csv.json"
python3 - "$WORK_DIR/import-csv.json" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["batch"]["batchNo"] == "SET-CSV-931", data
assert data["batch"]["matchedCount"] == 1 and data["batch"]["openIssueCount"] == 0, data
assert data["batch"]["totalNetCents"] == 8760, data
PY

cat >"$WORK_DIR/duplicate-transaction.json" <<'JSON'
{"batchNo":"SET-DUP-931","provider":"gateway","providerSettlementNo":"GW-SET-DUP-931","currency":"CNY","entries":[{"lineNo":1,"providerTransactionNo":"GW-SET-TXN-A","transactionType":"payment","orderNo":"PAY-SET-A","providerOrderNo":"GW-ORDER-A","amountCents":10000,"feeCents":0,"netAmountCents":10000},{"lineNo":2,"providerTransactionNo":"GW-SET-DUP-NEW","transactionType":"payment","orderNo":"PAY-SET-G","providerOrderNo":"GW-ORDER-G","amountCents":8800,"feeCents":0,"netAmountCents":8800}]}
JSON
api_post_file "$PLATFORM_TOKEN" "/dashboard/saasAdmin/paymentSettlementImport" "application/json" "$WORK_DIR/duplicate-transaction.json" "$WORK_DIR/duplicate-transaction-response.json" 409
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-DUP-931'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries WHERE provider_transaction_no = 'GW-SET-DUP-NEW'")" = "0"

curl -sS -f -D "$WORK_DIR/batches-csv.headers" -o "$WORK_DIR/batches.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=paymentSettlementBatches&provider=gateway&limit=1000"
curl -sS -f -D "$WORK_DIR/entries-csv.headers" -o "$WORK_DIR/entries.csv" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=paymentSettlementEntries&batchNo=SET-MAIN-931&limit=1000"
grep -qi 'filename="mochat-saas-payment-settlement-batches-' "$WORK_DIR/batches-csv.headers"
grep -qi 'filename="mochat-saas-payment-settlement-entries-' "$WORK_DIR/entries-csv.headers"
grep -q 'SET-MAIN-931' "$WORK_DIR/batches.csv"
grep -q 'GW-SET-STATUS' "$WORK_DIR/entries.csv"

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action IN ('payment.settlement.import', 'payment.settlement.resolve', 'payment.settlement.reconcile', 'payment.settlement.transition')")" -ge "10"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/admin-page.html"
grep -q 'id="paymentSettlementBatches"' "$WORK_DIR/admin-page.html"
grep -q 'id="paymentSettlementEntries"' "$WORK_DIR/admin-page.html"
grep -q 'id="importPaymentSettlement"' "$WORK_DIR/admin-page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
for route in \
  'GET /dashboard/saasAdmin/paymentSettlementBatches' \
  'GET /dashboard/saasAdmin/paymentSettlementEntries' \
  'POST /dashboard/saasAdmin/paymentSettlementImport' \
  'POST /dashboard/saasAdmin/paymentSettlementReconcile' \
  'POST /dashboard/saasAdmin/paymentSettlementResolve' \
  'POST /dashboard/saasAdmin/paymentSettlementTransition'; do
  grep -q "$route" "$WORK_DIR/routes.json"
done
grep -q 'go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentSettlementImport' "$GO_LOG"

echo "SaaS payment settlement smoke passed"
