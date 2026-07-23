#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-payment-settlement-reopen-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13421}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26471}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18195}"
DATABASE="mochat_saas_payment_settlement_reopen_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-payment-settlement-reopen-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-payment-settlement-reopen-approval-jwt-secret}"

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
trap 'echo "SaaS payment settlement reopen approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    return 1
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
  tail -120 "$GO_LOG" >&2 || true
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

request_reopen_approval() {
  local batch_no="$1" version="$2" key="$3" output="$4" payload body
  payload="$(jq -cn --arg batchNo "$batch_no" --argjson version "$version" '{batchNo:$batchNo,expectedVersion:$version,action:"reopen",reason:"渠道补发结算明细，需重新核对"}')"
  body="$(jq -cn --argjson payload "$payload" --arg key "$key" '{actionType:"payment.settlement.reopen",payload:$payload,reason:"财务与内控确认重开范围",idempotencyKey:$key}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认重开原因和账务范围\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认关账审计和重开风险\"}" \
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
grep -q $'0092_saas_payment_settlement_reopen_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.settlement.reopen' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals >= 2")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,结算重开治理平台,13800000075,secret075,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000076', password, '结算重开申请人', 0, '财务部', '结算', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000075' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000077', password, '结算重开复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000075' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000078', password, '结算重开复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000075' LIMIT 1;

INSERT INTO mochat_go_saas_payment_settlement_batches
  (batch_no, provider, provider_settlement_no, period_start, period_end, currency, status, source_sha256,
   entry_count, payment_count, refund_count, matched_count, issue_count, open_issue_count,
   resolved_issue_count, ignored_issue_count, total_amount_cents, total_fee_cents, total_net_cents,
   difference_amount_cents, imported_by_user_id, imported_by_tenant_id, imported_at, reconciled_at,
   closed_by_user_id, closed_at, close_reason, version, remark, created_at, updated_at)
VALUES
  ('SET-REOPEN-SUCCESS-92', 'gateway', 'GW-REOPEN-SUCCESS-92', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'closed', REPEAT('a', 64),
   2, 1, 1, 2, 0, 0, 0, 0, 88000, -800, 87200, 0, 1, 1, '2026-07-18 00:01:00', '2026-07-18 00:02:00', 1, '2026-07-18 00:03:00', '0091 双人复核关账', 2, '0092 成功重开', '2026-07-18 00:01:00', '2026-07-18 00:03:00'),
  ('SET-REOPEN-NOT-CLOSED-92', 'gateway', 'GW-REOPEN-NOT-CLOSED-92', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'reconciled', REPEAT('b', 64),
   1, 1, 0, 1, 0, 0, 0, 0, 9900, -100, 9800, 0, 1, 1, NOW(), NOW(), 0, NULL, '', 1, '0092 非关闭态', NOW(), NOW()),
  ('SET-REOPEN-INCOMPLETE-92', 'gateway', 'GW-REOPEN-INCOMPLETE-92', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'closed', REPEAT('c', 64),
   1, 1, 0, 1, 0, 0, 0, 0, 12800, -80, 12720, 0, 1, 1, NOW(), NOW(), 1, NOW(), '', 2, '0092 关账审计缺失', NOW(), NOW()),
  ('SET-REOPEN-DRIFT-92', 'gateway', 'GW-REOPEN-DRIFT-92', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'closed', REPEAT('d', 64),
   1, 1, 0, 1, 0, 0, 0, 0, 128800, -800, 128000, 0, 1, 1, '2026-07-18 00:01:00', '2026-07-18 00:02:00', 1, '2026-07-18 00:03:00', '0091 完成关账', 2, '0092 漂移拒绝', '2026-07-18 00:01:00', '2026-07-18 00:03:00');
SQL

REQUESTER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000076'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000077'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000078'")"
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
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000075 secret075 "$WORK_DIR/platform-auth.json")"
REQUESTER_TOKEN="$(login_token 13800000076 secret075 "$WORK_DIR/requester-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000077 secret075 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000078 secret075 "$WORK_DIR/approver-two-auth.json")"

api_get "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "payment.settlement.reopen")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.finance.manage" and .targetType == "payment_settlement_batch" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true)' "$WORK_DIR/policies.json" >/dev/null

POLICY_VERSION="$(jq -r '.data.policies[] | select(.actionType == "payment.settlement.reopen") | .version' "$WORK_DIR/policies.json")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalRequest" \
  "{\"actionType\":\"approval.policy.update\",\"payload\":{\"actionType\":\"payment.settlement.reopen\",\"enabled\":false,\"amountThresholdCents\":0,\"requiredApprovals\":1,\"slaMinutes\":240,\"reminderMinutes\":60,\"expiryHours\":24,\"expectedVersion\":$POLICY_VERSION},\"reason\":\"尝试弱化重开门禁\",\"idempotencyKey\":\"settlement-reopen-policy-weaken-0092\"}" \
  "$WORK_DIR/policy-weaken-blocked.json" 400
jq -e '.code == 400 and (.msg | contains("严重风险门禁"))' "$WORK_DIR/policy-weaken-blocked.json" >/dev/null

api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/paymentSettlementTransition" \
  '{"batchNo":"SET-REOPEN-SUCCESS-92","expectedVersion":2,"action":"reopen","reason":"不应直接重开"}' \
  "$WORK_DIR/direct-reopen-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "payment.settlement.reopen" and .data.requiredApprovals == 2' "$WORK_DIR/direct-reopen-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-REOPEN-SUCCESS-92'")" = "closed:2"

for batch_no in SET-REOPEN-NOT-CLOSED-92 SET-REOPEN-INCOMPLETE-92; do
  version="$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = '$batch_no'")"
  body="$(jq -cn --arg batchNo "$batch_no" --argjson version "$version" '{actionType:"payment.settlement.reopen",payload:{batchNo:$batchNo,expectedVersion:$version,action:"reopen",reason:"尝试重开"},reason:"尝试重开",idempotencyKey:("invalid-"+$batchNo)}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$body" "$WORK_DIR/$batch_no-blocked.json" 409
done
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE target_id IN ('SET-REOPEN-NOT-CLOSED-92','SET-REOPEN-INCOMPLETE-92')")" = "0"

request_reopen_approval SET-REOPEN-SUCCESS-92 2 settlement-reopen-success-0092 "$WORK_DIR/success-request.json"
SUCCESS_APPROVAL_ID="$(json_value "$WORK_DIR/success-request.json" data.approval.id)"
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-request.json" data.approval.version)"
jq -e '.data.approval.actionType == "payment.settlement.reopen" and .data.approval.riskLevel == "critical" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == "SET-REOPEN-SUCCESS-92" and .data.approval.request.schemaVersion == 2 and .data.approval.request.transition.action == "reopen" and .data.approval.request.transition.expectedVersion == 2 and .data.approval.request.batch.status == "closed" and .data.approval.request.batch.closedByUserId == 1 and .data.approval.request.batch.closedAt == "2026-07-18 00:03:00" and .data.approval.request.batch.closeReason == "0091 双人复核关账" and .data.approval.request.batch.totalAmountCents == 88000 and .data.approval.request.batch.totalFeeCents == -800 and .data.approval.request.batch.totalNetCents == 87200 and .data.approval.request.batch.version == 2' "$WORK_DIR/success-request.json" >/dev/null
request_reopen_approval SET-REOPEN-SUCCESS-92 2 settlement-reopen-success-0092 "$WORK_DIR/success-idempotent.json"
jq -e --argjson approvalId "$SUCCESS_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/success-idempotent.json" >/dev/null

approve_twice "$SUCCESS_APPROVAL_ID" "$SUCCESS_APPROVAL_VERSION" success
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-vote-two.json" data.approval.version)"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$SUCCESS_APPROVAL_ID,\"expectedVersion\":$SUCCESS_APPROVAL_VERSION}" "$WORK_DIR/success-execute.json"
SUCCESS_OPERATION_ID="$(json_value "$WORK_DIR/success-execute.json" data.result.operationId)"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.batch.status == "reconciled" and .data.result.batch.version == 3 and .data.result.batch.closedByUserId == 0 and .data.result.batch.closedAt == "" and .data.result.batch.closeReason == "" and .data.result.previousStatus == "closed" and .data.result.operationId > 0' "$WORK_DIR/success-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version, ':', closed_by_user_id, ':', IFNULL(closed_at, ''), ':', close_reason) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-REOPEN-SUCCESS-92'")" = "reconciled:3:0::"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $SUCCESS_OPERATION_ID AND action = 'payment.settlement.transition' AND target_type = 'payment_settlement_batch' AND target_id = 'SET-REOPEN-SUCCESS-92'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $SUCCESS_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $SUCCESS_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

request_reopen_approval SET-REOPEN-DRIFT-92 2 settlement-reopen-drift-0092 "$WORK_DIR/drift-request.json"
DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/drift-request.json" data.approval.id)"
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-request.json" data.approval.version)"
approve_twice "$DRIFT_APPROVAL_ID" "$DRIFT_APPROVAL_VERSION" drift
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_payment_settlement_batches SET close_reason = '审批外篡改关账原因' WHERE batch_no = 'SET-REOPEN-DRIFT-92'" >/dev/null
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-REOPEN-DRIFT-92'")" = "2"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DRIFT_APPROVAL_ID,\"expectedVersion\":$DRIFT_APPROVAL_VERSION}" "$WORK_DIR/drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("账务、关账审计或状态已变化"))' "$WORK_DIR/drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-REOPEN-DRIFT-92'")" = "closed:2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $DRIFT_APPROVAL_ID AND status = 'approved' AND effect_applied_at IS NULL AND effect_operation_id = 0 AND last_error LIKE '%关账审计或状态已变化%'")" = "1"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/page" "$WORK_DIR/page.html"
grep -q "'payment.settlement.reopen'" "$WORK_DIR/page.html"
grep -q 'data-transition="reopen"' "$WORK_DIR/page.html"

echo "SaaS payment settlement reopen approval smoke passed"
