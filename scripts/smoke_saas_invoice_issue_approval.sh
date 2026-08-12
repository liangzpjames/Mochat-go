#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-invoice-issue-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13418}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26468}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18192}"
DATABASE="mochat_saas_invoice_issue_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-invoice-issue-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-invoice-issue-approval-jwt-secret}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "SaaS invoice issue approval smoke failed at line $LINENO" >&2; [ ! -f "$WORK_DIR/success-request.json" ] || { echo "approval response:" >&2; cat "$WORK_DIR/success-request.json" >&2; }; tail -180 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
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
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)" = "$expected" ] && return 0
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

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

json_value() {
  local file="$1" expression="$2"
  python3 - "$file" "$expression" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split("."):
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
  api_post "" "/dashboard/user/auth" "{\"phone\":\"$phone\",\"password\":\"$password\"}" "$output"
  json_value "$output" data.token
}

request_issue_approval() {
  local document_no="$1" provider_document_no="$2" key="$3" output="$4"
  local payload request_body
  payload="$(jq -cn --arg documentNo "$document_no" --arg providerDocumentNo "$provider_document_no" \
    '{documentNo:$documentNo,expectedVersion:3,status:"issued",provider:"manual",providerDocumentNo:$providerDocumentNo,documentUrl:("https://invoice.example.test/" + $providerDocumentNo + ".pdf"),remark:"正式开具"}')"
  request_body="$(jq -cn --argjson payload "$payload" --arg key "$key" \
    '{actionType:"billing.invoice.issue",payload:$payload,reason:"票面、回款和金额台账已复核",idempotencyKey:$key}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$output"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认票面与订单\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认金额台账\"}" \
    "$WORK_DIR/$prefix-vote-two.json"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$prefix-vote-two.json" >/dev/null
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
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'billing.invoice.issue' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,开票治理平台,13800000061,secret061,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
993,开票审批租户,13800000993,secret993,租户管理员,SaaS租户超级管理员,growth,成长版,2,100,1000,50,5,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000062', password, '开票申请人', 0, '财务部', '开票', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000061' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000063', password, '开票复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000061' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000064', password, '开票复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000061' LIMIT 1;
SQL

REQUESTER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000062'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000063'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000064'")"
FINANCE_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_finance'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($REQUESTER_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($REQUESTER_ID, $FINANCE_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());

INSERT INTO mochat_go_saas_payment_orders
  (order_no, tenant_id, provider, provider_order_no, status, package_code, package_name, billing_cycle,
   amount_cents, refund_pending_amount_cents, refunded_amount_cents, invoice_pending_amount_cents,
   invoiced_amount_cents, credit_pending_amount_cents, credited_amount_cents, currency, paid_at,
   version, created_by_user_id, created_by_tenant_id, remark, created_at, updated_at)
VALUES
  ('PAY-ISSUE-SUCCESS', 993, 'gateway', 'GW-ISSUE-SUCCESS', 'paid', 'growth', '成长版', 'yearly',
   100000, 0, 0, 88000, 0, 0, 0, 'CNY', NOW(), 8, $REQUESTER_ID, 1, '成功开具订单', NOW(), NOW()),
  ('PAY-ISSUE-DOC-DRIFT', 993, 'gateway', 'GW-ISSUE-DOC-DRIFT', 'paid', 'growth', '成长版', 'yearly',
   100000, 0, 0, 66000, 0, 0, 0, 'CNY', NOW(), 8, $REQUESTER_ID, 1, '单据漂移订单', NOW(), NOW()),
  ('PAY-ISSUE-ORDER-DRIFT', 993, 'gateway', 'GW-ISSUE-ORDER-DRIFT', 'paid', 'growth', '成长版', 'yearly',
   100000, 0, 0, 55000, 0, 0, 0, 'CNY', NOW(), 8, $REQUESTER_ID, 1, '订单漂移订单', NOW(), NOW()),
  ('PAY-ISSUE-PROCESSING', 993, 'gateway', 'GW-ISSUE-PROCESSING', 'paid', 'growth', '成长版', 'yearly',
   100000, 0, 0, 44000, 0, 0, 0, 'CNY', NOW(), 8, $REQUESTER_ID, 1, '直接处理中订单', NOW(), NOW());

INSERT INTO mochat_go_saas_invoice_documents
  (document_no, tenant_id, payment_order_id, order_no, kind, status, amount_cents, currency,
   invoice_type, invoice_title, tax_identifier, email, provider, requested_by_user_id,
   requested_by_tenant_id, processed_by_user_id, processed_by_tenant_id, requested_at,
   processing_at, version, remark, created_at, updated_at)
SELECT 'INV-ISSUE-SUCCESS', 993, id, order_no, 'invoice', 'processing', 88000, 'CNY',
       'special', '开票审批租户有限公司', '91310000ISSUE00993', 'finance@example.test', 'manual',
       $REQUESTER_ID, 1, $REQUESTER_ID, 1, NOW(), NOW(), 3, '等待正式开具', NOW(), NOW()
FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-SUCCESS'
UNION ALL
SELECT 'INV-ISSUE-DOC-DRIFT', 993, id, order_no, 'invoice', 'processing', 66000, 'CNY',
       'special', '开票审批租户有限公司', '91310000ISSUE00993', 'finance@example.test', 'manual',
       $REQUESTER_ID, 1, $REQUESTER_ID, 1, NOW(), NOW(), 3, '等待单据漂移验收', NOW(), NOW()
FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-DOC-DRIFT'
UNION ALL
SELECT 'INV-ISSUE-ORDER-DRIFT', 993, id, order_no, 'invoice', 'processing', 55000, 'CNY',
       'special', '开票审批租户有限公司', '91310000ISSUE00993', 'finance@example.test', 'manual',
       $REQUESTER_ID, 1, $REQUESTER_ID, 1, NOW(), NOW(), 3, '等待订单漂移验收', NOW(), NOW()
FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-ORDER-DRIFT'
UNION ALL
SELECT 'INV-ISSUE-PROCESSING', 993, id, order_no, 'invoice', 'requested', 44000, 'CNY',
       'special', '开票审批租户有限公司', '91310000ISSUE00993', 'finance@example.test', 'manual',
       $REQUESTER_ID, 1, 0, 0, NOW(), NULL, 1, '等待进入处理中', NOW(), NOW()
FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-PROCESSING';
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$JWT_SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000061 secret061 "$WORK_DIR/platform-auth.json")"
REQUESTER_TOKEN="$(login_token 13800000062 secret061 "$WORK_DIR/requester-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000063 secret061 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000064 secret061 "$WORK_DIR/approver-two-auth.json")"

api_get "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "billing.invoice.issue")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.finance.manage" and .targetType == "invoice_document" and .governanceLocked == true)' "$WORK_DIR/policies.json" >/dev/null

api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-ISSUE-SUCCESS","expectedVersion":3,"status":"issued","provider":"manual","providerDocumentNo":"FP-DIRECT-BLOCKED"}' \
  "$WORK_DIR/direct-issued-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "billing.invoice.issue" and .data.requiredApprovals == 2' "$WORK_DIR/direct-issued-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-ISSUE-SUCCESS'")" = "processing:3"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents, ':', version) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-SUCCESS'")" = "88000:0:8"

api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/invoiceTransition" \
  '{"documentNo":"INV-ISSUE-PROCESSING","expectedVersion":1,"status":"processing","provider":"manual","remark":"财务已受理"}' \
  "$WORK_DIR/direct-processing.json"
jq -e '.data.previousStatus == "requested" and .data.document.status == "processing" and .data.document.version == 2' "$WORK_DIR/direct-processing.json" >/dev/null

request_issue_approval "INV-ISSUE-SUCCESS" "FP-ISSUE-SUCCESS" "invoice-issue-success-0089" "$WORK_DIR/success-request.json"
SUCCESS_APPROVAL_ID="$(json_value "$WORK_DIR/success-request.json" data.approval.id)"
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-request.json" data.approval.version)"
jq -e '.data.approval.actionType == "billing.invoice.issue" and .data.approval.riskLevel == "critical" and .data.approval.targetType == "invoice_document" and .data.approval.requiredApprovals == 2 and .data.approval.request.document.documentNo == "INV-ISSUE-SUCCESS" and .data.approval.request.document.status == "processing" and .data.approval.request.document.version == 3 and .data.approval.request.document.amountCents == 88000 and .data.approval.request.order.orderNo == "PAY-ISSUE-SUCCESS" and .data.approval.request.order.status == "paid" and .data.approval.request.order.version == 8 and .data.approval.request.order.invoicePendingCents == 88000 and .data.approval.request.transition.Status == "issued"' "$WORK_DIR/success-request.json" >/dev/null
request_issue_approval "INV-ISSUE-SUCCESS" "FP-ISSUE-SUCCESS" "invoice-issue-success-0089" "$WORK_DIR/success-idempotent.json"
jq -e --argjson approvalId "$SUCCESS_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/success-idempotent.json" >/dev/null

approve_twice "$SUCCESS_APPROVAL_ID" "$SUCCESS_APPROVAL_VERSION" "success"
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$SUCCESS_APPROVAL_ID,\"expectedVersion\":$SUCCESS_APPROVAL_VERSION}" "$WORK_DIR/success-execute.json"
SUCCESS_OPERATION_ID="$(json_value "$WORK_DIR/success-execute.json" data.result.operationId)"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.previousStatus == "processing" and .data.result.document.status == "issued" and .data.result.document.version == 4 and .data.result.document.providerDocumentNo == "FP-ISSUE-SUCCESS" and .data.result.order.invoicePendingCents == 0 and .data.result.order.invoicedAmountCents == 88000 and .data.result.order.version == 9 and .data.result.operationId > 0' "$WORK_DIR/success-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version, ':', provider_document_no, ':', operation_id) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-ISSUE-SUCCESS'")" = "issued:4:FP-ISSUE-SUCCESS:$SUCCESS_OPERATION_ID"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents, ':', version) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-SUCCESS'")" = "0:88000:9"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $SUCCESS_OPERATION_ID AND tenant_id = 993 AND action = 'billing.invoice.transition' AND target_type = 'invoice_document' AND target_id = 'INV-ISSUE-SUCCESS'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $SUCCESS_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $SUCCESS_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

request_issue_approval "INV-ISSUE-DOC-DRIFT" "FP-ISSUE-DOC-DRIFT" "invoice-issue-document-drift-0089" "$WORK_DIR/document-drift-request.json"
DOCUMENT_DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/document-drift-request.json" data.approval.id)"
DOCUMENT_DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/document-drift-request.json" data.approval.version)"
approve_twice "$DOCUMENT_DRIFT_APPROVAL_ID" "$DOCUMENT_DRIFT_APPROVAL_VERSION" "document-drift"
DOCUMENT_DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/document-drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_invoice_documents SET version = version + 1, remark = '审批后单据漂移', updated_at = NOW() WHERE document_no = 'INV-ISSUE-DOC-DRIFT'" >/dev/null
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DOCUMENT_DRIFT_APPROVAL_ID,\"expectedVersion\":$DOCUMENT_DRIFT_APPROVAL_VERSION}" "$WORK_DIR/document-drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("发票单据已变化"))' "$WORK_DIR/document-drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-ISSUE-DOC-DRIFT'")" = "processing:4"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents, ':', version) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-DOC-DRIFT'")" = "66000:0:8"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $DOCUMENT_DRIFT_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND effect_applied_at IS NULL AND last_error LIKE '%发票单据已变化%'")" = "1"

request_issue_approval "INV-ISSUE-ORDER-DRIFT" "FP-ISSUE-ORDER-DRIFT" "invoice-issue-order-drift-0089" "$WORK_DIR/order-drift-request.json"
ORDER_DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/order-drift-request.json" data.approval.id)"
ORDER_DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/order-drift-request.json" data.approval.version)"
approve_twice "$ORDER_DRIFT_APPROVAL_ID" "$ORDER_DRIFT_APPROVAL_VERSION" "order-drift"
ORDER_DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/order-drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_payment_orders SET version = version + 1, remark = '审批后订单漂移', updated_at = NOW() WHERE order_no = 'PAY-ISSUE-ORDER-DRIFT'" >/dev/null
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$ORDER_DRIFT_APPROVAL_ID,\"expectedVersion\":$ORDER_DRIFT_APPROVAL_VERSION}" "$WORK_DIR/order-drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("支付订单或开票金额台账已变化"))' "$WORK_DIR/order-drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_invoice_documents WHERE document_no = 'INV-ISSUE-ORDER-DRIFT'")" = "processing:3"
test "$(mysql_scalar "SELECT CONCAT(invoice_pending_amount_cents, ':', invoiced_amount_cents, ':', version) FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-ISSUE-ORDER-DRIFT'")" = "55000:0:9"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $ORDER_DRIFT_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND effect_applied_at IS NULL AND last_error LIKE '%支付订单或开票金额台账已变化%'")" = "1"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/page" "$WORK_DIR/page.html"
grep -q '提交开具审批' "$WORK_DIR/page.html"
grep -q "approvalActionRequired('billing.invoice.issue', 0)" "$WORK_DIR/page.html"
grep -q 'await requestHighRiskApproval' "$WORK_DIR/page.html"
grep -q "'billing.invoice.issue'," "$WORK_DIR/page.html"
grep -q '开具审批已提交，等待两人复核' "$WORK_DIR/page.html"

echo "SaaS invoice issue approval smoke passed"
