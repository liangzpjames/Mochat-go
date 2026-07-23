#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-payment-order-create-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13419}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26469}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18193}"
DATABASE="mochat_saas_payment_order_create_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-payment-order-create-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-payment-order-create-approval-jwt-secret}"
WEBHOOK_SECRET="${MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET:-payment-order-create-approval-webhook-secret-2026}"

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
trap 'echo "SaaS payment order create approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

wait_service_healthy() {
  local service="$1" deadline=$((SECONDS + ${MOCHAT_SERVICE_HEALTH_TIMEOUT_SECONDS:-360}))
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

request_payment_order_approval() {
  local payload="$1" key="$2" output="$3" request_body
  request_body="$(jq -cn --argjson payload "$payload" --arg key "$key" \
    '{actionType:"payment.order.create",payload:$payload,reason:"合同、租户、套餐和收款金额已复核",idempotencyKey:$key}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$output"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认合同与租户\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认套餐与金额\"}" \
    "$WORK_DIR/$prefix-vote-two.json"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$prefix-vote-two.json" >/dev/null
}

webhook_post() {
  local body="$1" output="$2" timestamp body_file signature status
  body_file="$WORK_DIR/webhook-body.json"
  printf '%s' "$body" >"$body_file"
  timestamp="$(date +%s)"
  signature="$(python3 - "$WEBHOOK_SECRET" "$timestamp" "$body_file" <<'PY'
import hashlib, hmac, pathlib, sys
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
  test "$status" = "200" || { echo "payment webhook returned $status" >&2; cat "$output" >&2; return 1; }
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
grep -q $'0090_saas_payment_order_create_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.order.create' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND COLUMN_NAME IN ('package_version', 'package_limits_json')")" = "2"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,收款治理平台,13800000065,secret065,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
994,收款审批租户,13800000994,secret994,租户管理员,SaaS租户超级管理员,growth,成长版,2,88,1888,50,5,20,20,20,20,20,20,20,20,20,20,20,20,20,768,20,20,20,20,20,2,1000,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000066', password, '收款申请人', 0, '财务部', '收款', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000065' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000067', password, '收款复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000065' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000068', password, '收款复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000065' LIMIT 1;
SQL

REQUESTER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000066'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000067'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000068'")"
FINANCE_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_finance'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($REQUESTER_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($REQUESTER_ID, $FINANCE_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());
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
  MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK=1 \
  MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET="$WEBHOOK_SECRET" \
  MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS=300 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000065 secret065 "$WORK_DIR/platform-auth.json")"
REQUESTER_TOKEN="$(login_token 13800000066 secret065 "$WORK_DIR/requester-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000067 secret065 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000068 secret065 "$WORK_DIR/approver-two-auth.json")"

api_get "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "payment.order.create")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.finance.manage" and .targetType == "payment_order" and .governanceLocked == true)' "$WORK_DIR/policies.json" >/dev/null

SUCCESS_PAYLOAD='{"tenantId":994,"provider":"gateway","providerOrderNo":"GW-APPROVED-994","idempotencyKey":"payment-order-success-0090","packageCode":"growth","billingCycle":"yearly","serviceExpiresAt":"2037-12-30 00:00:00","amountCents":288000,"currency":"CNY","checkoutUrl":"https://pay.example.test/approved-994","checkoutExpiresAt":"2037-01-01 00:30:00","maxDunningAttempts":3,"remark":"0090双人审批收款"}'
api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/paymentOrder" "$SUCCESS_PAYLOAD" "$WORK_DIR/direct-create-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "payment.order.create" and .data.requiredApprovals == 2' "$WORK_DIR/direct-create-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_orders WHERE tenant_id = 994")" = "0"

PACKAGE_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_packages WHERE code = 'growth' AND deleted_at IS NULL")"
PACKAGE_MAX_USERS="$(mysql_scalar "SELECT max_users FROM mochat_go_saas_packages WHERE code = 'growth' AND deleted_at IS NULL")"
request_payment_order_approval "$SUCCESS_PAYLOAD" "payment-order-approval-success-0090" "$WORK_DIR/success-request.json"
SUCCESS_APPROVAL_ID="$(json_value "$WORK_DIR/success-request.json" data.approval.id)"
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-request.json" data.approval.version)"
SUCCESS_ORDER_NO="$(json_value "$WORK_DIR/success-request.json" data.approval.request.create.orderNo)"
jq -e --argjson packageVersion "$PACKAGE_VERSION" --argjson maxUsers "$PACKAGE_MAX_USERS" '.data.approval.actionType == "payment.order.create" and .data.approval.riskLevel == "critical" and .data.approval.targetType == "payment_order" and (.data.approval.targetId | startswith("APR-PAY-")) and .data.approval.targetId == .data.approval.request.create.orderNo and .data.approval.requiredApprovals == 2 and .data.approval.request.create.expectedTenantStatus == 1 and .data.approval.request.create.expectedPackageVersion == $packageVersion and .data.approval.request.create.amountCents == 288000 and .data.approval.request.tenant.tenantId == 994 and .data.approval.request.tenant.tenantStatus == 1 and .data.approval.request.package.code == "growth" and .data.approval.request.package.version == $packageVersion and .data.approval.request.package.limits.maxUsers == $maxUsers' "$WORK_DIR/success-request.json" >/dev/null
request_payment_order_approval "$SUCCESS_PAYLOAD" "payment-order-approval-success-0090" "$WORK_DIR/success-idempotent.json"
jq -e --argjson approvalId "$SUCCESS_APPROVAL_ID" --arg orderNo "$SUCCESS_ORDER_NO" '.data.idempotent == true and .data.approval.id == $approvalId and .data.approval.targetId == $orderNo' "$WORK_DIR/success-idempotent.json" >/dev/null

approve_twice "$SUCCESS_APPROVAL_ID" "$SUCCESS_APPROVAL_VERSION" "success"
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$SUCCESS_APPROVAL_ID,\"expectedVersion\":$SUCCESS_APPROVAL_VERSION}" "$WORK_DIR/success-execute.json"
SUCCESS_OPERATION_ID="$(json_value "$WORK_DIR/success-execute.json" data.result.operationId)"
jq -e --arg orderNo "$SUCCESS_ORDER_NO" --argjson packageVersion "$PACKAGE_VERSION" --argjson maxUsers "$PACKAGE_MAX_USERS" '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.order.orderNo == $orderNo and .data.result.order.status == "pending" and .data.result.order.packageVersion == $packageVersion and .data.result.order.packageLimits.maxUsers == $maxUsers and .data.result.order.amountCents == 288000 and .data.result.operationId > 0' "$WORK_DIR/success-execute.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_orders WHERE order_no = '$SUCCESS_ORDER_NO' AND tenant_id = 994 AND package_version = $PACKAGE_VERSION AND JSON_UNQUOTE(JSON_EXTRACT(package_limits_json, '$.maxUsers')) = '$PACKAGE_MAX_USERS' AND amount_cents = 288000")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $SUCCESS_OPERATION_ID AND tenant_id = 994 AND action = 'payment.order.create' AND target_type = 'payment_order' AND target_id = '$SUCCESS_ORDER_NO'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $SUCCESS_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $SUCCESS_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

mysql_scalar "UPDATE mochat_go_saas_packages SET max_users = max_users + 77, version = version + 1, updated_at = NOW() WHERE code = 'growth' AND deleted_at IS NULL" >/dev/null
CURRENT_PACKAGE_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_packages WHERE code = 'growth' AND deleted_at IS NULL")"
test "$CURRENT_PACKAGE_VERSION" = "$((PACKAGE_VERSION + 1))"
SUCCESS_EVENT="$(jq -cn --arg orderNo "$SUCCESS_ORDER_NO" '{provider:"gateway",eventId:"evt-approved-994",eventType:"payment.succeeded",orderNo:$orderNo,providerOrderNo:"GW-APPROVED-994",amountCents:288000,currency:"CNY",paidAt:"2026-07-18 13:00:00",occurredAt:"2026-07-18 13:00:00",metadata:{source:"approval-smoke"}}')"
webhook_post "$SUCCESS_EVENT" "$WORK_DIR/success-webhook.json"
jq -e '.data.order.status == "paid" and .data.billingEventId > 0 and .data.event.status == "processed"' "$WORK_DIR/success-webhook.json" >/dev/null
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(limits_json, '$.maxUsers')) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 994 AND deleted_at IS NULL")" = "$PACKAGE_MAX_USERS"
test "$(mysql_scalar "SELECT package_version FROM mochat_go_saas_payment_orders WHERE order_no = '$SUCCESS_ORDER_NO'")" = "$PACKAGE_VERSION"

DRIFT_PAYLOAD='{"tenantId":994,"provider":"gateway","providerOrderNo":"GW-DRIFT-994","idempotencyKey":"payment-order-drift-0090","packageCode":"growth","billingCycle":"yearly","serviceExpiresAt":"2037-12-31 00:00:00","amountCents":388000,"currency":"CNY","checkoutExpiresAt":"2037-01-01 00:30:00","maxDunningAttempts":3,"remark":"0090套餐漂移拒绝"}'
request_payment_order_approval "$DRIFT_PAYLOAD" "payment-order-approval-drift-0090" "$WORK_DIR/drift-request.json"
DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/drift-request.json" data.approval.id)"
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-request.json" data.approval.version)"
DRIFT_ORDER_NO="$(json_value "$WORK_DIR/drift-request.json" data.approval.request.create.orderNo)"
jq -e --argjson packageVersion "$CURRENT_PACKAGE_VERSION" '.data.approval.request.create.expectedPackageVersion == $packageVersion and .data.approval.request.package.version == $packageVersion' "$WORK_DIR/drift-request.json" >/dev/null
approve_twice "$DRIFT_APPROVAL_ID" "$DRIFT_APPROVAL_VERSION" "drift"
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_packages SET version = version + 1, updated_at = NOW() WHERE code = 'growth' AND deleted_at IS NULL" >/dev/null
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DRIFT_APPROVAL_ID,\"expectedVersion\":$DRIFT_APPROVAL_VERSION}" "$WORK_DIR/drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("package version changed after approval request"))' "$WORK_DIR/drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_orders WHERE order_no = '$DRIFT_ORDER_NO'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $DRIFT_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND effect_applied_at IS NULL AND last_error LIKE '%package version changed after approval request%'")" = "1"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/page" "$WORK_DIR/page.html"
grep -q '提交收款审批' "$WORK_DIR/page.html"
grep -q "approvalActionRequired('payment.order.create', 0)" "$WORK_DIR/page.html"
grep -q "'payment.order.create'," "$WORK_DIR/page.html"

echo "SaaS payment order create approval smoke passed"
