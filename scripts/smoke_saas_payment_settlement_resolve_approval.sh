#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-payment-settlement-resolve-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13422}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26472}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18196}"
DATABASE="mochat_saas_payment_settlement_resolve_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-payment-settlement-resolve-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-payment-settlement-resolve-approval-jwt-secret}"

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
trap 'echo "SaaS payment settlement resolve approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
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

request_resolve_approval() {
  local entry_id="$1" version="$2" handling_status="$3" key="$4" output="$5" payload body
  payload="$(jq -cn --argjson entryId "$entry_id" --argjson version "$version" --arg status "$handling_status" '{entryId:$entryId,expectedVersion:$version,handlingStatus:$status,reason:("0093 财务处理 " + $status)}')"
  body="$(jq -cn --argjson payload "$payload" --arg key "$key" '{actionType:"payment.settlement.resolve",payload:$payload,reason:"财务与内控复核结算差异",idempotencyKey:$key}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认渠道流水与内部账本\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认差异处理结论\"}" \
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
grep -q $'0093_saas_payment_settlement_resolve_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.settlement.resolve' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 240 AND reminder_minutes = 60 AND expiry_hours = 24")" = "1"

printf '%s\n' \
  'tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode' \
  '1,结算差异治理平台,13800000079,secret079,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing' \
  >"$WORK_DIR/tenants.csv"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000080', password, '结算差异申请人', 0, '财务部', '结算', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000079' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000081', password, '结算差异复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000079' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000082', password, '结算差异复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000079' LIMIT 1;

INSERT INTO mochat_go_saas_payment_settlement_batches
  (id, batch_no, provider, provider_settlement_no, period_start, period_end, currency, status, source_sha256,
   entry_count, payment_count, refund_count, matched_count, issue_count, open_issue_count, resolved_issue_count,
   ignored_issue_count, total_amount_cents, total_fee_cents, total_net_cents, difference_amount_cents,
   imported_by_user_id, imported_by_tenant_id, imported_at, reconciled_at, version, remark, created_at, updated_at)
VALUES
  (9301, 'SET-RESOLVE-SUCCESS-93', 'gateway', 'GW-RESOLVE-SUCCESS-93', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'reconciled', REPEAT('a', 64),
   1, 1, 0, 0, 1, 1, 0, 0, 88000, -800, 87200, 200, 1, 1, '2026-07-18 00:01:00', '2026-07-18 00:02:00', 1, '0093 成功处理', '2026-07-18 00:01:00', '2026-07-18 00:02:00'),
  (9302, 'SET-RESOLVE-MATCHED-93', 'gateway', 'GW-RESOLVE-MATCHED-93', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'reconciled', REPEAT('b', 64),
   1, 1, 0, 1, 0, 0, 0, 0, 9900, -100, 9800, 0, 1, 1, NOW(), NOW(), 1, '0093 已匹配', NOW(), NOW()),
  (9303, 'SET-RESOLVE-CLOSED-93', 'gateway', 'GW-RESOLVE-CLOSED-93', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'closed', REPEAT('c', 64),
   1, 1, 0, 0, 1, 1, 0, 0, 12800, -80, 12720, 100, 1, 1, NOW(), NOW(), 2, '0093 已关账', NOW(), NOW()),
  (9304, 'SET-RESOLVE-DRIFT-93', 'gateway', 'GW-RESOLVE-DRIFT-93', '2026-07-17 00:00:00', '2026-07-18 00:00:00', 'CNY', 'reconciled', REPEAT('d', 64),
   1, 1, 0, 0, 1, 1, 0, 0, 128800, -800, 128000, 800, 1, 1, '2026-07-18 00:01:00', '2026-07-18 00:02:00', 1, '0093 漂移拒绝', '2026-07-18 00:01:00', '2026-07-18 00:02:00');

INSERT INTO mochat_go_saas_payment_settlement_entries
  (id, batch_id, line_no, provider, provider_transaction_no, transaction_type, order_no, provider_order_no,
   amount_cents, fee_cents, net_amount_cents, currency, occurred_at, expected_amount_cents, expected_currency,
   expected_status, reconciliation_status, difference_amount_cents, issue_codes_json, issue_message,
   handling_status, version, raw_json, created_at, updated_at)
VALUES
  (9311, 9301, 1, 'gateway', 'GW-TXN-RESOLVE-9311', 'payment', 'PAY-9311', 'GW-PAY-9311',
   88000, -800, 87200, 'CNY', '2026-07-18 00:00:30', 87800, 'CNY', 'paid', 'amount_mismatch', 200,
   JSON_ARRAY('amount_mismatch'), '结算金额与内部订单不一致', 'open', 1, JSON_OBJECT('source', 'gateway', 'line', 1), '2026-07-18 00:01:00', '2026-07-18 00:02:00'),
  (9312, 9302, 1, 'gateway', 'GW-TXN-MATCHED-9312', 'payment', 'PAY-9312', 'GW-PAY-9312',
   9900, -100, 9800, 'CNY', NOW(), 9900, 'CNY', 'paid', 'matched', 0, JSON_ARRAY(), '', 'none', 1, JSON_OBJECT('source', 'gateway', 'line', 1), NOW(), NOW()),
  (9313, 9303, 1, 'gateway', 'GW-TXN-CLOSED-9313', 'payment', 'PAY-9313', 'GW-PAY-9313',
   12800, -80, 12720, 'CNY', NOW(), 12700, 'CNY', 'paid', 'amount_mismatch', 100,
   JSON_ARRAY('amount_mismatch'), '已关账批次差异', 'open', 1, JSON_OBJECT('source', 'gateway', 'line', 1), NOW(), NOW()),
  (9314, 9304, 1, 'gateway', 'GW-TXN-DRIFT-9314', 'payment', 'PAY-9314', 'GW-PAY-9314',
   128800, -800, 128000, 'CNY', '2026-07-18 00:00:30', 128000, 'CNY', 'paid', 'amount_mismatch', 800,
   JSON_ARRAY('amount_mismatch'), '待验证冻结快照', 'open', 1, JSON_OBJECT('source', 'gateway', 'line', 1), '2026-07-18 00:01:00', '2026-07-18 00:02:00');
SQL

REQUESTER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000080'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000081'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000082'")"
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

PLATFORM_TOKEN="$(login_token 13800000079 secret079 "$WORK_DIR/platform-auth.json")"
REQUESTER_TOKEN="$(login_token 13800000080 secret079 "$WORK_DIR/requester-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000081 secret079 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000082 secret079 "$WORK_DIR/approver-two-auth.json")"

api_get "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "payment.settlement.resolve")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.finance.manage" and .targetType == "payment_settlement_entry" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true)' "$WORK_DIR/policies.json" >/dev/null

POLICY_VERSION="$(jq -r '.data.policies[] | select(.actionType == "payment.settlement.resolve") | .version' "$WORK_DIR/policies.json")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalRequest" \
  "{\"actionType\":\"approval.policy.update\",\"payload\":{\"actionType\":\"payment.settlement.resolve\",\"enabled\":false,\"amountThresholdCents\":1,\"requiredApprovals\":1,\"slaMinutes\":240,\"reminderMinutes\":60,\"expiryHours\":24,\"expectedVersion\":$POLICY_VERSION},\"reason\":\"尝试弱化差异处理门禁\",\"idempotencyKey\":\"settlement-resolve-policy-weaken-0093\"}" \
  "$WORK_DIR/policy-weaken-blocked.json" 400
jq -e '.code == 400' "$WORK_DIR/policy-weaken-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(enabled, ':', amount_threshold_cents, ':', required_approvals, ':', version) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.settlement.resolve'")" = "1:0:2:$POLICY_VERSION"

api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/paymentSettlementResolve" \
  '{"entryId":9311,"expectedVersion":1,"handlingStatus":"resolved","reason":"不应直接处理"}' \
  "$WORK_DIR/direct-resolve-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "payment.settlement.resolve" and .data.requiredApprovals == 2' "$WORK_DIR/direct-resolve-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(handling_status, ':', version) FROM mochat_go_saas_payment_settlement_entries WHERE id = 9311")" = "open:1"

for invalid in '9311:open' '9312:resolved' '9313:resolved'; do
  entry_id="${invalid%%:*}"
  status="${invalid##*:}"
  body="$(jq -cn --argjson entryId "$entry_id" --arg status "$status" '{actionType:"payment.settlement.resolve",payload:{entryId:$entryId,expectedVersion:1,handlingStatus:$status,reason:"非法处理状态"},reason:"非法处理状态",idempotencyKey:("invalid-"+($entryId|tostring)+"-"+$status)}')"
  api_post "$REQUESTER_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$body" "$WORK_DIR/invalid-$entry_id-$status.json" 409
done
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE target_id IN ('9311','9312','9313')")" = "0"

request_resolve_approval 9311 1 resolved settlement-resolve-success-0093 "$WORK_DIR/success-request.json"
SUCCESS_APPROVAL_ID="$(json_value "$WORK_DIR/success-request.json" data.approval.id)"
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-request.json" data.approval.version)"
jq -e '.data.approval.actionType == "payment.settlement.resolve" and .data.approval.riskLevel == "critical" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == "9311" and .data.approval.request.schemaVersion == 1 and .data.approval.request.resolve.handlingStatus == "resolved" and .data.approval.request.resolve.expectedVersion == 1 and .data.approval.request.entry.id == 9311 and .data.approval.request.entry.handlingStatus == "open" and .data.approval.request.entry.issueCodesJson == "[\"amount_mismatch\"]" and .data.approval.request.entry.rawJson != "" and .data.approval.request.batch.id == 9301 and .data.approval.request.batch.openIssueCount == 1 and .data.approval.request.batch.totalNetCents == 87200 and .data.approval.request.batch.version == 1' "$WORK_DIR/success-request.json" >/dev/null
request_resolve_approval 9311 1 resolved settlement-resolve-success-0093 "$WORK_DIR/success-idempotent.json"
jq -e --argjson approvalId "$SUCCESS_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/success-idempotent.json" >/dev/null

approve_twice "$SUCCESS_APPROVAL_ID" "$SUCCESS_APPROVAL_VERSION" success
SUCCESS_APPROVAL_VERSION="$(json_value "$WORK_DIR/success-vote-two.json" data.approval.version)"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$SUCCESS_APPROVAL_ID,\"expectedVersion\":$SUCCESS_APPROVAL_VERSION}" "$WORK_DIR/success-execute.json"
SUCCESS_OPERATION_ID="$(json_value "$WORK_DIR/success-execute.json" data.result.operationId)"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.entry.handlingStatus == "resolved" and .data.result.entry.version == 2 and .data.result.batch.openIssueCount == 0 and .data.result.batch.resolvedIssueCount == 1 and .data.result.batch.version == 2 and .data.result.operationId > 0' "$WORK_DIR/success-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(handling_status, ':', version, ':', handled_by_user_id, ':', handled_by_tenant_id) FROM mochat_go_saas_payment_settlement_entries WHERE id = 9311")" = "resolved:2:$APPROVER1_ID:1"
test "$(mysql_scalar "SELECT CONCAT(open_issue_count, ':', resolved_issue_count, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE id = 9301")" = "0:1:2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $SUCCESS_OPERATION_ID AND action = 'payment.settlement.resolve' AND target_type = 'payment_settlement_entry' AND target_id = '9311'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $SUCCESS_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $SUCCESS_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

request_resolve_approval 9311 2 open settlement-resolve-reopen-0093 "$WORK_DIR/reopen-request.json"
REOPEN_APPROVAL_ID="$(json_value "$WORK_DIR/reopen-request.json" data.approval.id)"
REOPEN_APPROVAL_VERSION="$(json_value "$WORK_DIR/reopen-request.json" data.approval.version)"
approve_twice "$REOPEN_APPROVAL_ID" "$REOPEN_APPROVAL_VERSION" reopen
REOPEN_APPROVAL_VERSION="$(json_value "$WORK_DIR/reopen-vote-two.json" data.approval.version)"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$REOPEN_APPROVAL_ID,\"expectedVersion\":$REOPEN_APPROVAL_VERSION}" "$WORK_DIR/reopen-execute.json"
jq -e '.data.approval.status == "executed" and .data.result.entry.handlingStatus == "open" and .data.result.entry.version == 3 and .data.result.batch.openIssueCount == 1 and .data.result.batch.resolvedIssueCount == 0 and .data.result.batch.version == 3' "$WORK_DIR/reopen-execute.json" >/dev/null

request_resolve_approval 9314 1 ignored settlement-resolve-drift-0093 "$WORK_DIR/drift-request.json"
DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/drift-request.json" data.approval.id)"
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-request.json" data.approval.version)"
approve_twice "$DRIFT_APPROVAL_ID" "$DRIFT_APPROVAL_VERSION" drift
DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_payment_settlement_entries SET issue_message = '审批外篡改差异说明' WHERE id = 9314" >/dev/null
mysql_scalar "UPDATE mochat_go_saas_payment_settlement_batches SET remark = '审批外篡改批次备注' WHERE id = 9304" >/dev/null
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_entries WHERE id = 9314")" = "1"
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_payment_settlement_batches WHERE id = 9304")" = "1"
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DRIFT_APPROVAL_ID,\"expectedVersion\":$DRIFT_APPROVAL_VERSION}" "$WORK_DIR/drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("明细、所属批次账务或处理审计已变化"))' "$WORK_DIR/drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(handling_status, ':', version) FROM mochat_go_saas_payment_settlement_entries WHERE id = 9314")" = "open:1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $DRIFT_APPROVAL_ID AND status = 'approved' AND effect_applied_at IS NULL AND effect_operation_id = 0 AND last_error LIKE '%所属批次账务或处理审计已变化%'")" = "1"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/page" "$WORK_DIR/page.html"
grep -q "approvalActionRequired('payment.settlement.resolve', 0)" "$WORK_DIR/page.html"
grep -q '解决提审' "$WORK_DIR/page.html"
grep -q '忽略提审' "$WORK_DIR/page.html"
grep -q '重开提审' "$WORK_DIR/page.html"

echo "SaaS payment settlement resolve approval smoke passed"
