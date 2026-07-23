#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-admin-approval-governance-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13389}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26439}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18163}"
DATABASE="mochat_saas_admin_approval_governance"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-admin-approval-governance.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-admin-approval-governance-jwt-secret}"

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
trap 'echo "SaaS admin approval governance smoke failed at line $LINENO" >&2; tail -240 "$GO_LOG" >&2 || true' ERR
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
    if [ "$status" = "healthy" ]; then return 0; fi
    sleep 2
  done
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

wait_mysql_value() {
  local query="$1" expected="$2" deadline=$((SECONDS + 30))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if [ "$(mysql_scalar "$query")" = "$expected" ]; then return 0; fi
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
  local phone="$1" output="$2"
  curl -sS -f -H 'Content-Type: application/json' -d "{\"phone\":\"$phone\",\"password\":\"secret011\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

request_approval() {
  local token="$1" order="$2" amount="$3" key="$4" output="$5" body
  body="$(jq -cn --arg order "$order" --arg key "$key" --argjson amount "$amount" '{actionType:"payment.refund.create",payload:{orderNo:$order,amountCents:$amount,currency:"CNY",reason:"治理验收退款",entitlementAction:"keep"},reason:"复核治理验收退款",expiresInHours:24,idempotencyKey:$key}')"
  api_post "$token" '/dashboard/saasAdmin/approvalRequest' "$body" "$output"
}

request_policy_update() {
  local token="$1" action_type="$2" enabled="$3" threshold="$4" approvals="$5" expected_version="$6" key="$7" output="$8" body
  body="$(jq -cn --arg actionType "$action_type" --arg key "$key" --argjson enabled "$enabled" --argjson threshold "$threshold" --argjson approvals "$approvals" --argjson expectedVersion "$expected_version" '{actionType:"approval.policy.update",payload:{actionType:$actionType,enabled:$enabled,amountThresholdCents:$threshold,requiredApprovals:$approvals,slaMinutes:180,reminderMinutes:60,expiryHours:24,expectedVersion:$expectedVersion},reason:("\u53d8\u66f4\u5ba1\u6279\u7b56\u7565\u3002"+$actionType),expiresInHours:12,idempotencyKey:$key}')"
  api_post "$token" '/dashboard/saasAdmin/approvalRequest' "$body" "$output"
}

start_go() {
  local cron_enabled="$1" run_on_start="$2"
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
    MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON="$cron_enabled" \
    MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS=2 \
    MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START="$run_on_start" \
    MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT=100 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
}

stop_go() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID"
    wait "$GO_PID" 2>/dev/null || true
  fi
  GO_PID=""
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
grep -q $'0047_saas_admin_approval_governance\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0049_saas_service_accounts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0052_saas_compliance_lifecycle\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0073_saas_compliance_export_deletion_saga\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0076_saas_identity_policy_change_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0077_saas_tenant_disable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0079_saas_service_account_key_revoke_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0080_saas_service_account_update_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0081_saas_service_account_key_rotate_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0082_saas_service_account_create_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0083_saas_identity_mfa_reset_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0084_saas_package_definition_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0096_saas_tenant_enable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0097_saas_release_evidence_action_tracking\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies')" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type IN ('tenant.disable','tenant.enable','tenant.provision','tenant.renewal','tenant.subscription.transition','package.upsert','tenant.package.update','tenant.domain.create','tenant.domain.command','payment.order.create','payment.refund.create','payment.settlement.close','payment.settlement.reopen','payment.settlement.resolve','billing.invoice.issue','access.role.save','access.assignment.save','tenant.data.erase','release.candidate.gate','approval.policy.update','backup.policy.update','backup.retention.cleanup','compliance.policy.update','compliance.export.delete','compliance.legal_hold.release','identity.policy.update','identity.mfa.reset','service_account.create','service_account.update','service_account.key.rotate','service_account.key.revoke') AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals >= 2")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.approvals.manage'")" = "1"

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 1 -tenant-name '审批治理平台' -phone 13800000011 -password secret011 -user-name '平台超级管理员' -role-name '平台超级管理员' -package-code platform -package-name 平台版 -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000012', password, '治理财务发起人', 0, '财务部', '财务', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000013', password, '治理审批人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000014', password, '治理审批人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000015', password, '治理受托人', 0, '运营部', '轮值', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000011' LIMIT 1;
SQL

SUPER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000011'")"
FINANCE_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000012'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000013'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000014'")"
DELEGATE_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000015'")"
FINANCE_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_finance'")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
READONLY_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_readonly'")"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($FINANCE_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW()), ($DELEGATE_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($FINANCE_ID, $FINANCE_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()),
       ($FINANCE_ID, $OPERATIONS_ROLE_ID, 0, NOW()),
       ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $FINANCE_ROLE_ID, 0, NOW()),
       ($DELEGATE_ID, $READONLY_ROLE_ID, 0, NOW());
INSERT INTO mochat_go_saas_payment_orders
  (order_no, tenant_id, provider, status, package_code, package_name, billing_cycle, service_expires_at, amount_cents, currency, paid_at, version, created_by_user_id, created_by_tenant_id, remark, created_at, updated_at)
VALUES
  ('PAY-GOV-SMALL', 1, 'gateway', 'paid', 'platform', '平台版', 'yearly', '2037-01-01', 10000, 'CNY', NOW(), 1, $FINANCE_ID, 1, '小额直退', NOW(), NOW()),
  ('PAY-GOV-QUORUM', 1, 'gateway', 'paid', 'platform', '平台版', 'yearly', '2037-01-01', 10000, 'CNY', NOW(), 1, $FINANCE_ID, 1, '会签退款', NOW(), NOW()),
  ('PAY-GOV-SELF', 1, 'gateway', 'paid', 'platform', '平台版', 'yearly', '2037-01-01', 10000, 'CNY', NOW(), 1, $APPROVER2_ID, 1, '委托自批拦截', NOW(), NOW()),
  ('PAY-GOV-SLA', 1, 'gateway', 'paid', 'platform', '平台版', 'yearly', '2037-01-01', 10000, 'CNY', NOW(), 1, $FINANCE_ID, 1, 'SLA 提醒', NOW(), NOW()),
  ('PAY-GOV-DISABLED', 1, 'gateway', 'paid', 'platform', '平台版', 'yearly', '2037-01-01', 10000, 'CNY', NOW(), 1, $FINANCE_ID, 1, '策略关闭直退', NOW(), NOW());
SQL

start_go 0 0
SUPER_TOKEN="$(login_token 13800000011 "$WORK_DIR/super-auth.json")"
FINANCE_TOKEN="$(login_token 13800000012 "$WORK_DIR/finance-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000013 "$WORK_DIR/approver1-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000014 "$WORK_DIR/approver2-auth.json")"
DELEGATE_TOKEN="$(login_token 13800000015 "$WORK_DIR/delegate-auth.json")"

api_get "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalPolicies' "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and all(.data.policies[] | select(.riskLevel == "critical"); .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true and .enabled == true and .amountThresholdCents == 0 and .requiredApprovals >= 2) and any(.data.policies[]; .actionType == "tenant.enable" and .requiredApprovals == 2 and .requiredPermission == "platform.tenants.manage") and any(.data.policies[]; .actionType == "tenant.provision" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.renewal" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.subscription.transition" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "package.upsert" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.package.update" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.domain.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "tenant.domain.command" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.order.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.refund.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "billing.invoice.issue" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "identity.mfa.reset" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.create" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.update" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.key.rotate" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "service_account.key.revoke" and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.settlement.close" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true and .requiredApprovals == 2) and any(.data.policies[]; .actionType == "payment.settlement.reopen" and .governanceLocked == true and .minimumApprovals == 2 and .amountThresholdLocked == true and .requiredApprovals == 2)' "$WORK_DIR/policies.json" >/dev/null

api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalPolicy' '{"actionType":"payment.settlement.close","enabled":true,"amountThresholdCents":0,"requiredApprovals":2,"slaMinutes":180,"reminderMinutes":60,"expiryHours":24,"expectedVersion":1}' "$WORK_DIR/policy-direct.json" 428
jq -e '.data.actionType == "approval.policy.update" and .data.requiredApprovals == 2' "$WORK_DIR/policy-direct.json" >/dev/null
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"approval.policy.update","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},"reason":"\u5c1d\u8bd5\u5173\u95ed\u5ba1\u6279\u6cbb\u7406\u95e8\u7981"}' "$WORK_DIR/policy-governance-weaken.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"tenant.disable","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":240,"reminderMinutes":60,"expiryHours":24,"expectedVersion":2},"reason":"尝试关闭租户停用双人门禁"}' "$WORK_DIR/policy-tenant-disable-weaken.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"tenant.enable","enabled":false,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":240,"reminderMinutes":60,"expiryHours":24,"expectedVersion":1},"reason":"尝试关闭租户启用双人门禁"}' "$WORK_DIR/policy-tenant-enable-weaken.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"payment.refund.create","enabled":true,"amountThresholdCents":1000,"requiredApprovals":2,"slaMinutes":120,"reminderMinutes":30,"expiryHours":24,"expectedVersion":2},"reason":"尝试为退款设置绕过阈值"}' "$WORK_DIR/policy-refund-threshold-weaken.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"access.role.save","enabled":false,"amountThresholdCents":0,"requiredApprovals":2,"slaMinutes":120,"reminderMinutes":30,"expiryHours":12,"expectedVersion":1},"reason":"尝试关闭角色变更门禁"}' "$WORK_DIR/policy-access-role-weaken.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"release.candidate.gate","enabled":true,"amountThresholdCents":0,"requiredApprovals":1,"slaMinutes":30,"reminderMinutes":10,"expiryHours":6,"expectedVersion":1},"reason":"尝试把发布候选降为单人审批"}' "$WORK_DIR/policy-release-gate-weaken.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/compliancePolicy' '{"status":"active","exportRetentionDays":30,"erasureGraceDays":7,"requireRecentExport":true,"recentExportMaxAgeDays":7,"billingRetentionDays":365,"auditRetentionDays":365,"serviceAccountUsageRetentionDays":90,"expectedVersion":1}' "$WORK_DIR/compliance-policy-direct.json" 428
jq -e '.data.actionType == "compliance.policy.update" and .data.requiredApprovals == 2' "$WORK_DIR/compliance-policy-direct.json" >/dev/null

request_policy_update "$SUPER_TOKEN" payment.settlement.close true 0 2 1 approval-governance-policy-update "$WORK_DIR/policy-request.json"
POLICY_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/policy-request.json")"
jq -e '.data.approval.actionType == "approval.policy.update" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == "payment.settlement.close" and .data.approval.policyVersion == 1' "$WORK_DIR/policy-request.json" >/dev/null
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$POLICY_APPROVAL_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"\u7b2c\u4e00\u7968\u901a\u8fc7\"}" "$WORK_DIR/policy-vote-one.json"
api_post "$APPROVER2_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$POLICY_APPROVAL_ID,\"expectedVersion\":2,\"decision\":\"approve\",\"reason\":\"\u7b2c\u4e8c\u7968\u901a\u8fc7\"}" "$WORK_DIR/policy-vote-two.json"
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$POLICY_APPROVAL_ID,\"expectedVersion\":3}" "$WORK_DIR/policy-executed.json"
jq -e '.data.approval.status == "executed" and .data.result.policy.actionType == "payment.settlement.close" and .data.result.policy.version == 2 and .data.result.policy.amountThresholdCents == 0 and .data.result.policy.requiredApprovals == 2 and .data.result.policy.slaMinutes == 180 and .data.result.operationId > 0' "$WORK_DIR/policy-executed.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $POLICY_APPROVAL_ID AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"

BACKUP_POLICY_BODY='{"status":"disabled","intervalMinutes":60,"retentionDays":14,"minSuccessfulBackups":3,"maxBackupAgeMinutes":120,"restoreDrillIntervalDays":7,"requireEncryption":false,"requireOffsiteReplica":false,"expectedVersion":1}'
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/backupPolicy' "$BACKUP_POLICY_BODY" "$WORK_DIR/backup-policy-direct.json" 428
jq -e '.data.actionType == "backup.policy.update" and .data.requiredApprovals == 2' "$WORK_DIR/backup-policy-direct.json" >/dev/null
BACKUP_APPROVAL_BODY="$(jq -cn --argjson payload "$BACKUP_POLICY_BODY" '{actionType:"backup.policy.update",payload:$payload,reason:"变更平台数据库备份策略",expiresInHours:12,idempotencyKey:"approval-governance-backup-policy"}')"
api_post "$FINANCE_TOKEN" '/dashboard/saasAdmin/approvalRequest' "$BACKUP_APPROVAL_BODY" "$WORK_DIR/backup-policy-request.json"
BACKUP_POLICY_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/backup-policy-request.json")"
jq -e '.data.approval.actionType == "backup.policy.update" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == "1"' "$WORK_DIR/backup-policy-request.json" >/dev/null
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$BACKUP_POLICY_APPROVAL_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"灾备策略第一票\"}" "$WORK_DIR/backup-policy-vote-one.json"
api_post "$APPROVER2_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$BACKUP_POLICY_APPROVAL_ID,\"expectedVersion\":2,\"decision\":\"approve\",\"reason\":\"灾备策略第二票\"}" "$WORK_DIR/backup-policy-vote-two.json"
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$BACKUP_POLICY_APPROVAL_ID,\"expectedVersion\":3}" "$WORK_DIR/backup-policy-executed.json"
jq -e '.data.approval.status == "executed" and .data.result.policy.status == "disabled" and .data.result.policy.version == 2 and .data.result.policy.intervalMinutes == 60' "$WORK_DIR/backup-policy-executed.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $BACKUP_POLICY_APPROVAL_ID AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_policies WHERE id = 1 AND status = 'disabled' AND version = 2 AND updated_by = $SUPER_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = (SELECT effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id = $BACKUP_POLICY_APPROVAL_ID) AND action = 'saas.admin.backup.policy.update'")" = "1"

api_post "$FINANCE_TOKEN" '/dashboard/saasAdmin/paymentRefund' '{"orderNo":"PAY-GOV-SMALL","amountCents":500,"currency":"CNY","reason":"小额直退","entitlementAction":"keep"}' "$WORK_DIR/small-refund.json" 428
jq -e '.data.actionType == "payment.refund.create" and .data.requiredApprovals == 2 and .data.amountThresholdCents == 0' "$WORK_DIR/small-refund.json" >/dev/null
api_post "$FINANCE_TOKEN" '/dashboard/saasAdmin/paymentRefund' '{"orderNo":"PAY-GOV-QUORUM","amountCents":1500,"currency":"CNY","reason":"高额直退","entitlementAction":"keep"}' "$WORK_DIR/high-direct.json" 428
jq -e '.data.requiredApprovals == 2 and .data.amountThresholdCents == 0' "$WORK_DIR/high-direct.json" >/dev/null

DELEGATION_BODY="$(jq -cn --argjson delegator "$APPROVER2_ID" --argjson delegate "$DELEGATE_ID" '{delegatorUserId:$delegator,delegateUserId:$delegate,startsAt:(now-300|todateiso8601),endsAt:(now+86400|todateiso8601),status:1,reason:"审批轮值委托"}')"
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalDelegation' "$DELEGATION_BODY" "$WORK_DIR/delegation.json"
jq -e --argjson delegator "$APPROVER2_ID" --argjson delegate "$DELEGATE_ID" '.data.created == true and .data.delegation.delegatorUserId == $delegator and .data.delegation.delegateUserId == $delegate' "$WORK_DIR/delegation.json" >/dev/null

request_approval "$FINANCE_TOKEN" PAY-GOV-QUORUM 1500 approval-governance-quorum "$WORK_DIR/quorum-request.json"
QUORUM_ID="$(jq -r '.data.approval.id' "$WORK_DIR/quorum-request.json")"
jq -e '.data.approval.requiredApprovals == 2 and .data.approval.approvalCount == 0 and .data.approval.policyVersion == 2' "$WORK_DIR/quorum-request.json" >/dev/null
api_post "$FINANCE_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$QUORUM_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"发起人自批\"}" "$WORK_DIR/self-review.json" 403
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$QUORUM_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"第一票通过\"}" "$WORK_DIR/vote-one.json"
jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1 and .data.approval.version == 2' "$WORK_DIR/vote-one.json" >/dev/null
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$QUORUM_ID,\"expectedVersion\":2,\"decision\":\"approve\",\"reason\":\"重复投票\"}" "$WORK_DIR/duplicate-vote.json" 409
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$QUORUM_ID,\"expectedVersion\":2}" "$WORK_DIR/quorum-not-ready.json" 409
api_post "$DELEGATE_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$QUORUM_ID,\"expectedVersion\":2,\"decision\":\"approve\",\"reason\":\"受托轮值复核\"}" "$WORK_DIR/vote-two.json"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2 and .data.approval.version == 3' "$WORK_DIR/vote-two.json" >/dev/null
api_get "$SUPER_TOKEN" "/dashboard/saasAdmin/approvalDecisions?approvalId=$QUORUM_ID" "$WORK_DIR/decisions.json"
jq -e --argjson delegator "$APPROVER2_ID" '(.data.decisions | length) == 2 and (.data.decisions[] | select(.delegatedFromUserId == $delegator) | .decision) == "approve"' "$WORK_DIR/decisions.json" >/dev/null
api_post "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$QUORUM_ID,\"expectedVersion\":3}" "$WORK_DIR/quorum-executed.json"
jq -e '.data.approval.status == "executed" and .data.approval.version == 5' "$WORK_DIR/quorum-executed.json" >/dev/null

request_approval "$APPROVER2_TOKEN" PAY-GOV-SELF 1200 approval-governance-self "$WORK_DIR/delegated-self-request.json"
SELF_ID="$(jq -r '.data.approval.id' "$WORK_DIR/delegated-self-request.json")"
api_post "$DELEGATE_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$SELF_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"尝试代委托人自批\"}" "$WORK_DIR/delegated-self-review.json" 403

request_approval "$FINANCE_TOKEN" PAY-GOV-SLA 1200 approval-governance-sla "$WORK_DIR/sla-request.json"
SLA_ID="$(jq -r '.data.approval.id' "$WORK_DIR/sla-request.json")"
mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_approvals SET sla_due_at = DATE_SUB(NOW(), INTERVAL 1 MINUTE), next_reminder_at = DATE_SUB(NOW(), INTERVAL 1 MINUTE) WHERE id = $SLA_ID"
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalReminders' '{"limit":100,"maxAttempts":3}' "$WORK_DIR/reminder-one.json"
jq -e '.data.scanned == 1 and .data.enqueued == 1 and .data.skipped == 0' "$WORK_DIR/reminder-one.json" >/dev/null
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalReminders' '{"limit":100,"maxAttempts":3}' "$WORK_DIR/reminder-repeat.json"
jq -e '.data.scanned == 0 and .data.enqueued == 0' "$WORK_DIR/reminder-repeat.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE alert_key LIKE '1:approval_sla:approval_sla_reminder:approval_${SLA_ID}_reminder_%'")" = "1"
test "$(mysql_scalar "SELECT reminder_count FROM mochat_go_saas_admin_approvals WHERE id = $SLA_ID")" = "1"

mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_approvals SET next_reminder_at = DATE_SUB(NOW(), INTERVAL 1 MINUTE) WHERE id = $SLA_ID"
stop_go
start_go 1 1
wait_mysql_value "SELECT reminder_count FROM mochat_go_saas_admin_approvals WHERE id = $SLA_ID" "2"
grep -q 'go cron enabled: SaaS approval reminder interval=2s run_on_start=true limit=100' "$GO_LOG"
grep -q 'SaaS approval reminder cron finished: scanned=1 enqueued=1 skipped=0' "$GO_LOG"

SUPER_TOKEN="$(login_token 13800000011 "$WORK_DIR/super-auth-restart.json")"
FINANCE_TOKEN="$(login_token 13800000012 "$WORK_DIR/finance-auth-restart.json")"
APPROVER1_TOKEN="$(login_token 13800000013 "$WORK_DIR/approver1-auth-restart.json")"
APPROVER2_TOKEN="$(login_token 13800000014 "$WORK_DIR/approver2-auth-restart.json")"
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalPolicy' '{"actionType":"payment.refund.create","enabled":false,"amountThresholdCents":0,"requiredApprovals":2,"slaMinutes":120,"reminderMinutes":30,"expiryHours":24,"expectedVersion":2}' "$WORK_DIR/policy-disable-direct.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"payment.refund.create","enabled":false,"amountThresholdCents":0,"requiredApprovals":2,"slaMinutes":120,"reminderMinutes":30,"expiryHours":24,"expectedVersion":2},"reason":"尝试关闭退款审批","idempotencyKey":"approval-governance-refund-disable"}' "$WORK_DIR/policy-disable-request.json" 400
api_post "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalRequest' '{"actionType":"approval.policy.update","payload":{"actionType":"payment.refund.create","enabled":true,"amountThresholdCents":1000,"requiredApprovals":2,"slaMinutes":120,"reminderMinutes":30,"expiryHours":24,"expectedVersion":2},"reason":"尝试恢复小额直退","idempotencyKey":"approval-governance-refund-threshold"}' "$WORK_DIR/policy-threshold-request.json" 400
api_post "$FINANCE_TOKEN" '/dashboard/saasAdmin/paymentRefund' '{"orderNo":"PAY-GOV-DISABLED","amountCents":2000,"currency":"CNY","reason":"验证策略不可关闭","entitlementAction":"keep"}' "$WORK_DIR/disabled-direct.json" 428
jq -e '.data.actionType == "payment.refund.create" and .data.requiredApprovals == 2 and .data.amountThresholdCents == 0' "$WORK_DIR/disabled-direct.json" >/dev/null

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'aria-label="审批策略"' "$WORK_DIR/page.html"
grep -q 'aria-label="审批委托维护"' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/approvalReminders'" "$WORK_DIR/page.html"
grep -q "requestHighRiskApproval('approval.policy.update'" "$WORK_DIR/page.html"
grep -q "requestHighRiskApproval('backup.policy.update'" "$WORK_DIR/page.html"
grep -q "requestHighRiskApproval('compliance.legal_hold.release'" "$WORK_DIR/page.html"
grep -q "'tenant.enable'" "$WORK_DIR/page.html"
grep -q '强制启用' "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/approvalPolicy' "$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/approvalDecisions' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/approvalDelegation' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/approvalReminders' "$WORK_DIR/routes.json"

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_decisions WHERE approval_id = $QUORUM_ID")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_events WHERE approval_id = $SLA_ID AND event_type = 'sla_reminder_enqueued'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.approval.policy.save'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'approval.policy.update' AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'backup.policy.update' AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.approval.delegation.save'")" = "1"

echo "SaaS admin approval governance smoke passed"
