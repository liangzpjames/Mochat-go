#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-admin-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13388}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26438}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18162}"
DATABASE="mochat_saas_admin_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-admin-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-admin-approval-jwt-secret}"

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
trap 'echo "SaaS admin approval smoke failed at line $LINENO" >&2; tail -220 "$GO_LOG" >&2 || true' ERR
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
    if [ "$status" = "healthy" ]; then return 0; fi
    sleep 2
  done
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then return 0; fi
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

login_token() {
  local phone="$1" password="$2" output="$3"
  curl -sS -f -H "Content-Type: application/json" -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

request_approval() {
  local token="$1" action="$2" payload="$3" reason="$4" key="$5" output="$6"
  local body
  body="$(jq -cn --arg actionType "$action" --arg reason "$reason" --arg idempotencyKey "$key" --argjson payload "$payload" '{actionType:$actionType,payload:$payload,reason:$reason,idempotencyKey:$idempotencyKey}')"
  api_post "$token" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

approve() {
  local token="$1" id="$2" version="$3" output="$4"
  api_post "$token" "/dashboard/saasAdmin/approvalDecision" "{\"approvalId\":$id,\"expectedVersion\":$version,\"decision\":\"approve\",\"reason\":\"独立复核通过\"}" "$output"
}

execute_approval() {
  local token="$1" id="$2" version="$3" output="$4" expected="${5:-200}"
  api_post "$token" "/dashboard/saasAdmin/approvalExecute" "{\"approvalId\":$id,\"expectedVersion\":$version}" "$output" "$expected"
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
grep -q '0046_saas_admin_approvals' "$WORK_DIR/migrate.out"
grep -q '0047_saas_admin_approval_governance' "$WORK_DIR/migrate.out"
grep -q '0049_saas_service_accounts' "$WORK_DIR/migrate.out"
grep -q '0064_saas_audit_anchor_remote_immutability' "$WORK_DIR/migrate.out"
grep -q '0067_wecom_credential_encryption' "$WORK_DIR/migrate.out"
grep -q '0068_wechat_open_credential_encryption' "$WORK_DIR/migrate.out"
grep -q '0074_saas_compliance_legal_hold_release_guard' "$WORK_DIR/migrate.out"
grep -q '0076_saas_identity_policy_change_guard' "$WORK_DIR/migrate.out"
grep -q '0077_saas_tenant_disable_approval_guard' "$WORK_DIR/migrate.out"
grep -q '0079_saas_service_account_key_revoke_guard' "$WORK_DIR/migrate.out"
grep -q '0080_saas_service_account_update_guard' "$WORK_DIR/migrate.out"
grep -q '0081_saas_service_account_key_rotate_guard' "$WORK_DIR/migrate.out"
grep -q '0082_saas_service_account_create_guard' "$WORK_DIR/migrate.out"
grep -q '0083_saas_identity_mfa_reset_guard' "$WORK_DIR/migrate.out"
grep -q '0084_saas_package_definition_guard' "$WORK_DIR/migrate.out"
grep -q '0085_saas_tenant_package_assignment_guard' "$WORK_DIR/migrate.out"
grep -q '0089_saas_invoice_issue_approval_guard' "$WORK_DIR/migrate.out"
grep -q '0096_saas_tenant_enable_approval_guard' "$WORK_DIR/migrate.out"
grep -q '0098_scrm_lead_foundation' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_roles WHERE is_system = 1")" = "6"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = '$DATABASE' AND TABLE_NAME = 'mochat_go_saas_admin_approvals' AND COLUMN_NAME IN ('effect_applied_at', 'effect_operation_id')")" = "2"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,审批平台,13800000011,secret011,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
961,审批业务租户,13800000961,secret961,业务租户管理员,SaaS租户超级管理员,growth,增长版,2,20,5000,200,5,50,30,30,30,30,30,30,30,30,30,30,30,30,1024,50,50,50,50,50,5,5000,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000012', password, '审批平台财务', 0, '财务部', '财务', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000013', password, '审批平台运营', 0, '运营部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000014', password, '审批复核人', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000015', password, '审批权限管理员', 0, '安全部', '权限治理', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000016', password, '待授权平台成员', 0, '运营部', '成员', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000017', password, '审批复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;

INSERT INTO mochat_go_saas_admin_roles (code, name, description, status, is_system, version, created_by, updated_by, created_at, updated_at)
VALUES ('approval_security', '审批权限治理', '用于发起权限变更审批', 1, 0, 1, 0, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_role_permissions (role_id, permission_code, created_at)
SELECT id, 'platform.overview.read', NOW() FROM mochat_go_saas_admin_roles WHERE code = 'approval_security'
UNION ALL SELECT id, 'platform.audit.read', NOW() FROM mochat_go_saas_admin_roles WHERE code = 'approval_security'
UNION ALL SELECT id, 'platform.access.manage', NOW() FROM mochat_go_saas_admin_roles WHERE code = 'approval_security'
UNION ALL SELECT id, 'platform.approvals.read', NOW() FROM mochat_go_saas_admin_roles WHERE code = 'approval_security';
SQL

FINANCE_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000012'")"
OPERATIONS_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000013'")"
APPROVER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000014'")"
SECURITY_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000015'")"
TARGET_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000016'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000017'")"
FINANCE_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_finance'")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
SECURITY_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'approval_security'")"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($FINANCE_ID, 1, 0, NOW(), NOW()), ($OPERATIONS_ID, 1, 0, NOW(), NOW()), ($APPROVER_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW()), ($SECURITY_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($FINANCE_ID, $FINANCE_ROLE_ID, 0, NOW()), ($OPERATIONS_ID, $OPERATIONS_ROLE_ID, 0, NOW()),
       ($APPROVER_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW()), ($SECURITY_ID, $SECURITY_ROLE_ID, 0, NOW());
INSERT INTO mochat_go_saas_payment_orders
  (order_no, tenant_id, provider, provider_order_no, status, package_code, package_name, billing_cycle, service_expires_at,
   amount_cents, currency, paid_at, version, created_by_user_id, created_by_tenant_id, remark, created_at, updated_at)
VALUES ('PAY-APPROVAL-1', 961, 'gateway', 'GW-APPROVAL-1', 'paid', 'growth', '增长版', 'yearly', '2037-01-01 00:00:00',
        10000, 'CNY', NOW(), 1, $FINANCE_ID, 1, '审批退款订单', NOW(), NOW());
INSERT INTO mochat_go_saas_payment_settlement_batches
  (batch_no, provider, provider_settlement_no, currency, status, source_sha256, entry_count, matched_count, issue_count,
   open_issue_count, imported_by_user_id, imported_by_tenant_id, reconciled_at, version, remark, created_at, updated_at)
VALUES ('SET-APPROVAL-1', 'gateway', 'GW-SET-APPROVAL-1', 'CNY', 'reconciled', REPEAT('a', 64), 1, 1, 0,
        0, $FINANCE_ID, 1, NOW(), 1, '审批关账批次', NOW(), NOW());
INSERT INTO mochat_go_saas_subscriptions
  (tenant_id, package_code, package_name, status, billing_cycle, current_period_starts_at,
   current_period_ends_at, grace_ends_at, version, state_reason, metadata_json, created_at, updated_at, deleted_at)
SELECT tenant_id, package_code, package_name, 'active', 'custom', NOW(), '2037-01-01 00:00:00',
       '2037-01-08 00:00:00', 1, '0096 approval smoke initial active subscription',
       JSON_OBJECT('source', '0096_approval_smoke'), NOW(), NOW(), NULL
FROM mochat_go_saas_tenant_packages
WHERE tenant_id = 961 AND deleted_at IS NULL
LIMIT 1;
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

SUPER_TOKEN="$(login_token 13800000011 secret011 "$WORK_DIR/super-auth.json")"
FINANCE_TOKEN="$(login_token 13800000012 secret011 "$WORK_DIR/finance-auth.json")"
OPERATIONS_TOKEN="$(login_token 13800000013 secret011 "$WORK_DIR/operations-auth.json")"
APPROVER_TOKEN="$(login_token 13800000014 secret011 "$WORK_DIR/approver-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000017 secret011 "$WORK_DIR/approver2-auth.json")"
SECURITY_TOKEN="$(login_token 13800000015 secret011 "$WORK_DIR/security-auth.json")"
TARGET_TOKEN="$(login_token 13800000016 secret011 "$WORK_DIR/target-auth.json")"
TENANT_TOKEN="$(login_token 13800000961 secret961 "$WORK_DIR/tenant-auth.json")"

api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and all(.data.policies[] | select(.riskLevel == "critical"); .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true and .enabled == true and .amountThresholdCents == 0 and .requiredApprovals >= 2) and any(.data.policies[]; .actionType == "tenant.provision" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.renewal" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.subscription.transition" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "package.upsert" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.package.update" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.domain.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.domain.command" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.order.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.refund.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "billing.invoice.issue" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "identity.mfa.reset" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.update" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.key.rotate" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.key.revoke" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.settlement.close" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.settlement.reopen" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true and .requiredApprovals == 2)' "$WORK_DIR/policies.json" >/dev/null
jq -e 'any(.data.policies[]; .actionType == "tenant.enable" and .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.tenants.manage" and .governanceLocked == true)' "$WORK_DIR/policies.json" >/dev/null
api_get "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvals?status=all&limit=10" "$WORK_DIR/empty-approvals.json"
api_get "$TARGET_TOKEN" "/dashboard/saasAdmin/approvals?status=all&limit=10" "$WORK_DIR/target-unassigned.json" 403
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/approvals?status=all&limit=10" "$WORK_DIR/tenant-forbidden.json" 403

api_post "$FINANCE_TOKEN" "/dashboard/saasAdmin/paymentRefund" '{"orderNo":"PAY-APPROVAL-1","amountCents":2500,"currency":"CNY","reason":"直接退款","entitlementAction":"keep"}' "$WORK_DIR/direct-refund.json" 428
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/tenantStatus" '{"tenantId":961,"status":2,"remark":"直接停用"}' "$WORK_DIR/direct-disable.json" 428
api_post "$FINANCE_TOKEN" "/dashboard/saasAdmin/paymentSettlementTransition" '{"batchNo":"SET-APPROVAL-1","expectedVersion":1,"action":"close","reason":"直接关账"}' "$WORK_DIR/direct-close.json" 428
api_post "$SUPER_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$TARGET_ID,\"roleIds\":[$FINANCE_ROLE_ID],\"expectedVersion\":0}" "$WORK_DIR/direct-access.json" 428

REFUND_PAYLOAD='{"orderNo":"PAY-APPROVAL-1","amountCents":2500,"currency":"CNY","reason":"客户退款","entitlementAction":"keep","remark":"工单 APR-1"}'
request_approval "$FINANCE_TOKEN" "payment.refund.create" "$REFUND_PAYLOAD" "复核客户退款凭证" "approval-refund-1" "$WORK_DIR/refund-request.json"
REFUND_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/refund-request.json")"
jq -e '.data.approval.status == "pending" and .data.approval.requiredApprovals == 2 and .data.approval.approvalCount == 0' "$WORK_DIR/refund-request.json" >/dev/null
request_approval "$FINANCE_TOKEN" "payment.refund.create" "$REFUND_PAYLOAD" "复核客户退款凭证" "approval-refund-1" "$WORK_DIR/refund-replay.json"
jq -e --argjson id "$REFUND_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $id' "$WORK_DIR/refund-replay.json" >/dev/null
api_post "$FINANCE_TOKEN" "/dashboard/saasAdmin/approvalDecision" "{\"approvalId\":$REFUND_APPROVAL_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"自己批准\"}" "$WORK_DIR/requester-review-forbidden.json" 403
approve "$APPROVER_TOKEN" "$REFUND_APPROVAL_ID" 1 "$WORK_DIR/refund-approved.json"
jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1 and .data.approval.version == 2' "$WORK_DIR/refund-approved.json" >/dev/null
approve "$APPROVER2_TOKEN" "$REFUND_APPROVAL_ID" 2 "$WORK_DIR/refund-approved-two.json"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2 and .data.approval.version == 3' "$WORK_DIR/refund-approved-two.json" >/dev/null
execute_approval "$FINANCE_TOKEN" "$REFUND_APPROVAL_ID" 3 "$WORK_DIR/requester-execute-forbidden.json" 403
execute_approval "$APPROVER_TOKEN" "$REFUND_APPROVAL_ID" 3 "$WORK_DIR/refund-executed.json"
jq -e '.data.approval.status == "executed" and .data.approval.version == 5' "$WORK_DIR/refund-executed.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_refunds WHERE order_no = 'PAY-APPROVAL-1' AND status = 'requested' AND amount_cents = 2500")" = "1"
test "$(mysql_scalar "SELECT refund_pending_amount_cents FROM mochat_go_saas_payment_orders WHERE order_no = 'PAY-APPROVAL-1'")" = "2500"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $REFUND_APPROVAL_ID AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
REFUND_OPERATION_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'payment.refund.request' AND target_id = (SELECT target_id FROM mochat_go_saas_admin_approvals WHERE id = $REFUND_APPROVAL_ID)")"
mysql_root "$DATABASE" <<SQL
UPDATE mochat_go_saas_admin_approvals
SET status = 'executing', execution_started_at = DATE_SUB(NOW(), INTERVAL 16 MINUTE), executed_at = NULL
WHERE id = $REFUND_APPROVAL_ID AND status = 'executed' AND version = 5;
SQL
execute_approval "$APPROVER_TOKEN" "$REFUND_APPROVAL_ID" 5 "$WORK_DIR/refund-recovered.json"
jq -e '.data.approval.status == "executed" and .data.approval.version == 7 and .data.approval.executionAttempts == 2 and .data.result.recovered == true' "$WORK_DIR/refund-recovered.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_refunds WHERE order_no = 'PAY-APPROVAL-1' AND status = 'requested' AND amount_cents = 2500")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'payment.refund.request' AND target_id = (SELECT target_id FROM mochat_go_saas_admin_approvals WHERE id = $REFUND_APPROVAL_ID)")" = "$REFUND_OPERATION_COUNT"
execute_approval "$APPROVER_TOKEN" "$REFUND_APPROVAL_ID" 7 "$WORK_DIR/refund-replay-execute.json" 409
api_get "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvalEvents?approvalId=$REFUND_APPROVAL_ID&limit=20" "$WORK_DIR/refund-events.json"
jq -e '(.data.events | map(.eventType)) == ["requested","decision_approve","decision_approve","execution_started","execution_succeeded","execution_recovered","execution_succeeded"]' "$WORK_DIR/refund-events.json" >/dev/null

request_approval "$OPERATIONS_TOKEN" "tenant.disable" '{"tenantId":961,"status":2,"remark":"欠费停用"}' "复核欠费停用" "approval-tenant-disable-1" "$WORK_DIR/tenant-request.json"
TENANT_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/tenant-request.json")"
approve "$APPROVER_TOKEN" "$TENANT_APPROVAL_ID" 1 "$WORK_DIR/tenant-approved.json"
jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1 and .data.approval.requiredApprovals == 2' "$WORK_DIR/tenant-approved.json" >/dev/null
approve "$APPROVER2_TOKEN" "$TENANT_APPROVAL_ID" 2 "$WORK_DIR/tenant-approved-two.json"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/tenant-approved-two.json" >/dev/null
execute_approval "$APPROVER_TOKEN" "$TENANT_APPROVAL_ID" 3 "$WORK_DIR/tenant-executed.json"
test "$(mysql_scalar "SELECT status FROM mc_tenant WHERE id = 961")" = "2"
status="$(curl -sS -o "$WORK_DIR/disabled-login.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000961","password":"secret961"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$status" = "403"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/tenantStatus" '{"tenantId":961,"status":1,"remark":"直接启用"}' "$WORK_DIR/direct-enable.json" 428
jq -e '.data.actionType == "tenant.enable" and .data.requiredApprovals == 2' "$WORK_DIR/direct-enable.json" >/dev/null

SUBSCRIPTION_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_subscriptions WHERE tenant_id = 961 AND deleted_at IS NULL")"
SUBSCRIPTION_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_subscriptions WHERE tenant_id = 961 AND deleted_at IS NULL")"
test -n "$SUBSCRIPTION_ID"
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE id = $SUBSCRIPTION_ID")" = "suspended"
request_approval "$OPERATIONS_TOKEN" "tenant.enable" '{"tenantId":961,"status":1,"remark":"恢复租户服务"}' "复核租户恢复条件" "approval-tenant-enable-1" "$WORK_DIR/tenant-enable-request.json"
TENANT_ENABLE_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/tenant-enable-request.json")"
jq -e --argjson subscriptionId "$SUBSCRIPTION_ID" --argjson subscriptionVersion "$SUBSCRIPTION_VERSION" '.data.approval.actionType == "tenant.enable" and .data.approval.requiredApprovals == 2 and .data.approval.request.schemaVersion == 1 and .data.approval.request.update.tenantId == 961 and .data.approval.request.update.status == 1 and .data.approval.request.update.expectedStatus == 2 and .data.approval.request.snapshot.tenantStatus == 2 and .data.approval.request.snapshot.subscriptionPresent == true and .data.approval.request.snapshot.subscriptionId == $subscriptionId and .data.approval.request.snapshot.subscriptionStatus == "suspended" and .data.approval.request.snapshot.subscriptionVersion == $subscriptionVersion' "$WORK_DIR/tenant-enable-request.json" >/dev/null
approve "$APPROVER_TOKEN" "$TENANT_ENABLE_APPROVAL_ID" 1 "$WORK_DIR/tenant-enable-approved.json"
approve "$APPROVER2_TOKEN" "$TENANT_ENABLE_APPROVAL_ID" 2 "$WORK_DIR/tenant-enable-approved-two.json"
mysql_root "$DATABASE" -e "UPDATE mc_tenant SET status = 1, updated_at = NOW() WHERE id = 961"
execute_approval "$APPROVER_TOKEN" "$TENANT_ENABLE_APPROVAL_ID" 3 "$WORK_DIR/tenant-enable-drift.json" 409
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE id = $SUBSCRIPTION_ID")" = "suspended"
mysql_root "$DATABASE" -e "UPDATE mc_tenant SET status = 2, updated_at = NOW() WHERE id = 961"
execute_approval "$APPROVER_TOKEN" "$TENANT_ENABLE_APPROVAL_ID" 5 "$WORK_DIR/tenant-enable-executed.json"
jq -e '.data.approval.status == "executed" and .data.approval.version == 7 and .data.result.previousStatus == 2 and .data.result.status == 1' "$WORK_DIR/tenant-enable-executed.json" >/dev/null
test "$(mysql_scalar "SELECT status FROM mc_tenant WHERE id = 961")" = "1"
test "$(mysql_scalar "SELECT status <> 'suspended' FROM mochat_go_saas_subscriptions WHERE id = $SUBSCRIPTION_ID")" = "1"
status="$(curl -sS -o "$WORK_DIR/enabled-login.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000961","password":"secret961"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$status" = "200"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $TENANT_ENABLE_APPROVAL_ID AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"

request_approval "$FINANCE_TOKEN" "payment.settlement.close" '{"batchNo":"SET-APPROVAL-1","expectedVersion":1,"action":"close","reason":"对账完成"}' "复核渠道结算关账" "approval-settlement-close-1" "$WORK_DIR/settlement-request.json"
SETTLEMENT_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/settlement-request.json")"
approve "$APPROVER_TOKEN" "$SETTLEMENT_APPROVAL_ID" 1 "$WORK_DIR/settlement-approved.json"
approve "$APPROVER2_TOKEN" "$SETTLEMENT_APPROVAL_ID" 2 "$WORK_DIR/settlement-approved-two.json"
execute_approval "$APPROVER_TOKEN" "$SETTLEMENT_APPROVAL_ID" 3 "$WORK_DIR/settlement-executed.json"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-APPROVAL-1'")" = "closed:2"

ROLE_PAYLOAD='{"code":"approved_readonly","name":"审批创建只读岗","description":"经审批创建","status":1,"permissions":["platform.overview.read","platform.tenants.read","platform.approvals.read"]}'
request_approval "$SECURITY_TOKEN" "access.role.save" "$ROLE_PAYLOAD" "创建只读岗位" "approval-role-create-1" "$WORK_DIR/role-request.json"
ROLE_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/role-request.json")"
approve "$APPROVER_TOKEN" "$ROLE_APPROVAL_ID" 1 "$WORK_DIR/role-approved.json"
approve "$APPROVER2_TOKEN" "$ROLE_APPROVAL_ID" 2 "$WORK_DIR/role-approved-two.json"
execute_approval "$APPROVER_TOKEN" "$ROLE_APPROVAL_ID" 3 "$WORK_DIR/role-executed.json"
APPROVED_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'approved_readonly'")"
test -n "$APPROVED_ROLE_ID"

request_approval "$SECURITY_TOKEN" "access.assignment.save" "{\"userId\":$TARGET_ID,\"roleIds\":[$APPROVED_ROLE_ID],\"expectedVersion\":0}" "给平台成员分配只读岗位" "approval-assignment-1" "$WORK_DIR/assignment-request.json"
ASSIGNMENT_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/assignment-request.json")"
approve "$APPROVER_TOKEN" "$ASSIGNMENT_APPROVAL_ID" 1 "$WORK_DIR/assignment-approved.json"
approve "$APPROVER2_TOKEN" "$ASSIGNMENT_APPROVAL_ID" 2 "$WORK_DIR/assignment-approved-two.json"
execute_approval "$APPROVER_TOKEN" "$ASSIGNMENT_APPROVAL_ID" 3 "$WORK_DIR/assignment-executed.json"
api_get "$TARGET_TOKEN" "/dashboard/saasAdmin/accessProfile" "$WORK_DIR/target-profile.json"
jq -e '.data.profile.permissions | index("platform.tenants.read") != null' "$WORK_DIR/target-profile.json" >/dev/null
request_approval "$SECURITY_TOKEN" "access.assignment.save" "{\"userId\":$SECURITY_ID,\"roleIds\":[$SECURITY_ROLE_ID],\"expectedVersion\":1}" "尝试给自己授权" "approval-self-assignment" "$WORK_DIR/self-assignment.json" 2>/dev/null || true
test "$(jq -r '.code' "$WORK_DIR/self-assignment.json")" = "400"

request_approval "$OPERATIONS_TOKEN" "tenant.disable" '{"tenantId":961,"status":2,"remark":"用于驳回测试"}' "用于驳回测试" "approval-reject-1" "$WORK_DIR/reject-request.json"
REJECT_ID="$(jq -r '.data.approval.id' "$WORK_DIR/reject-request.json")"
api_post "$APPROVER_TOKEN" "/dashboard/saasAdmin/approvalDecision" "{\"approvalId\":$REJECT_ID,\"expectedVersion\":1,\"decision\":\"reject\",\"reason\":\"不应再次停用\"}" "$WORK_DIR/rejected.json"
test "$(jq -r '.data.approval.status' "$WORK_DIR/rejected.json")" = "rejected"

request_approval "$FINANCE_TOKEN" "payment.refund.create" '{"orderNo":"PAY-APPROVAL-1","amountCents":500,"currency":"CNY","reason":"撤回退款","entitlementAction":"keep"}' "用于撤回测试" "approval-cancel-1" "$WORK_DIR/cancel-request.json"
CANCEL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/cancel-request.json")"
api_post "$FINANCE_TOKEN" "/dashboard/saasAdmin/approvalCancel" "{\"approvalId\":$CANCEL_ID,\"expectedVersion\":1,\"reason\":\"资料需要调整\"}" "$WORK_DIR/canceled.json"
test "$(jq -r '.data.approval.status' "$WORK_DIR/canceled.json")" = "canceled"

request_approval "$FINANCE_TOKEN" "payment.refund.create" '{"orderNo":"PAY-APPROVAL-1","amountCents":600,"currency":"CNY","reason":"过期测试退款","entitlementAction":"keep"}' "用于过期测试" "approval-expire-1" "$WORK_DIR/expire-request.json"
EXPIRE_ID="$(jq -r '.data.approval.id' "$WORK_DIR/expire-request.json")"
mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_approvals SET expires_at = DATE_SUB(NOW(), INTERVAL 1 MINUTE) WHERE id = $EXPIRE_ID"
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/approvals?approvalId=$EXPIRE_ID" "$WORK_DIR/expired.json"
test "$(jq -r '.data.items[0].status' "$WORK_DIR/expired.json")" = "expired"

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals")" = "9"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_events")" = "40"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.approval.request'")" = "9"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.approval.execute_start'")" = "8"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.approval.execute_finish'")" = "8"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE status = 'executed' AND effect_applied_at IS NOT NULL")" = "6"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE requester_user_id = reviewer_user_id AND reviewer_user_id <> 0")" = "0"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'aria-label="高风险审批中心"' "$WORK_DIR/page.html"
grep -q 'function requestHighRiskApproval(actionType, payload, reason, options)' "$WORK_DIR/page.html"
grep -q "'tenant.enable'" "$WORK_DIR/page.html"
grep -q '提交启用审批' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/approvalExecute'" "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/approvalPolicies' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/approvalExecute' "$WORK_DIR/routes.json"
grep -q 'approval_required=true' "$GO_LOG"

echo "SaaS admin approval smoke passed"
