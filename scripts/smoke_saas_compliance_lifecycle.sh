#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-compliance-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13401}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26451}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18175}"
DATABASE="mochat_saas_compliance"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-compliance.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-compliance-jwt-secret}"
EXPORT_KEY="${MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY:-1111111111111111111111111111111111111111111111111111111111111111}"
ARTIFACT_ROOT="$WORK_DIR/compliance-artifacts"
UPLOAD_ROOT="$WORK_DIR/uploads"
PII_MARKER="COMPLIANCE-PII-982-SECRET"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ "${KEEP_WORK_DIR:-0}" = "1" ]; then
    echo "SaaS compliance smoke work directory kept at $WORK_DIR" >&2
  else
    rm -rf "$WORK_DIR"
  fi
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "SaaS compliance lifecycle smoke failed at line $LINENO" >&2; tail -240 "$GO_LOG" >&2 || true' ERR
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
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  return 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 90))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$status" = "$expected" ]; then return 0; fi
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
  curl -sS -f -H 'Content-Type: application/json' \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

request_approval() {
  local token="$1" request_id="$2" output="$3" body
  body="$(jq -cn --argjson requestId "$request_id" '{actionType:"tenant.data.erase",payload:{requestId:$requestId},reason:"复核不可逆租户数据擦除",idempotencyKey:("compliance-erasure-"+($requestId|tostring))}')"
  api_post "$token" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

request_export_deletion_approval() {
  local token="$1" export_id="$2" output="$3" expected="${4:-200}" body
  body="$(jq -cn --argjson exportId "$export_id" '{actionType:"compliance.export.delete",payload:{exportId:$exportId},reason:"复核提前删除合规导出工件",idempotencyKey:("compliance-export-delete-"+($exportId|tostring))}')"
  api_post "$token" "/dashboard/saasAdmin/approvalRequest" "$body" "$output" "$expected"
}

request_hold_release_approval() {
  local token="$1" hold_id="$2" version="$3" output="$4" body
  body="$(jq -cn --argjson holdId "$hold_id" --argjson expectedVersion "$version" '{actionType:"compliance.legal_hold.release",payload:{holdId:$holdId,reason:"监管调查已结束",expectedVersion:$expectedVersion},reason:"复核法律保留解除依据",idempotencyKey:("compliance-hold-release-"+($holdId|tostring)+"-v"+($expectedVersion|tostring))}')"
  api_post "$token" "/dashboard/saasAdmin/approvalRequest" "$body" "$output"
}

apply_compliance_policy_update() {
  local token="$1" payload="$2" idempotency_key="$3" prefix="$4" body approval_id approval_version
  body="$(jq -cn --argjson payload "$payload" --arg key "$idempotency_key" '{actionType:"compliance.policy.update",payload:$payload,reason:"复核租户数据合规生命周期策略变更",idempotencyKey:$key}')"
  api_post "$token" "/dashboard/saasAdmin/approvalRequest" "$body" "$WORK_DIR/$prefix-request.json"
  approval_id="$(jq -er '.data.approval.id' "$WORK_DIR/$prefix-request.json")"
  approval_version="$(jq -er '.data.approval.version' "$WORK_DIR/$prefix-request.json")"
  jq -e '.data.approval.actionType == "compliance.policy.update" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == "1" and .data.approval.targetName == "租户数据合规策略"' "$WORK_DIR/$prefix-request.json" >/dev/null
  test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.status')) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id")" = "$(jq -r '.status' <<<"$payload")"
  test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.billingRetentionDays')) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id")" = "$(jq -r '.billingRetentionDays' <<<"$payload")"
  test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.auditRetentionDays')) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id")" = "$(jq -r '.auditRetentionDays' <<<"$payload")"
  test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.expectedVersion')) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id")" = "$(jq -r '.expectedVersion' <<<"$payload")"
  approve "$APPROVER1_TOKEN" "$approval_id" "$approval_version" "第一复核人确认合规策略变更" "$WORK_DIR/$prefix-approval-one.json"
  approval_version="$(jq -er '.data.approval.version' "$WORK_DIR/$prefix-approval-one.json")"
  approve "$APPROVER2_TOKEN" "$approval_id" "$approval_version" "第二复核人确认合规策略变更" "$WORK_DIR/$prefix-approval-two.json"
  approval_version="$(jq -er '.data.approval.version' "$WORK_DIR/$prefix-approval-two.json")"
  jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/$prefix-approval-two.json" >/dev/null
  api_post "$SUPER_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version}" "$WORK_DIR/$prefix-execute.json"
  jq -e '.data.approval.status == "executed" and .data.result.policy.id == 1' "$WORK_DIR/$prefix-execute.json" >/dev/null
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = (SELECT effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id = $approval_id) AND action = 'saas.admin.compliance.policy.update'")" = "1"
}

approve() {
  local token="$1" id="$2" version="$3" reason="$4" output="$5"
  api_post "$token" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$id,\"expectedVersion\":$version,\"decision\":\"approve\",\"reason\":\"$reason\"}" "$output"
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
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
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
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.compliance.read'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.compliance.manage'")" = "1"
test "$(mysql_scalar "SELECT required_approvals FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.data.erase'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.export.delete' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.legal_hold.release' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.policy.update' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_data_exports' AND column_name LIKE 'deletion_%'")" = "12"

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" \
  -tenant-id 1 -tenant-name '合规验收平台' -phone 13800000070 -password secret070 \
  -user-name '平台超级管理员' -role-name '平台超级管理员' \
  -package-code compliance-platform -package-name '平台版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-platform.out"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" \
  -tenant-id 982 -tenant-name '待擦除业务租户' -phone 13800000982 -password secret982 \
  -user-name '业务租户管理员' -role-name '租户超级管理员' \
  -package-code compliance-business -package-name '业务版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-business.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000071', password, '平台合规运营', 0, '安全部', '合规运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000070' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000072', password, '合规复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000070' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000073', password, '合规复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000070' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000074', password, '平台合规审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000070' LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000071'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000072'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000073'")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000074'")"
OPERATOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()),
       ($APPROVER2_ID, 1, 0, NOW(), NOW()), ($AUDITOR_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $OPERATOR_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()),
       ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 0, NOW());

INSERT INTO mc_corp
  (id, name, wx_corpid, social_code, chat_admin, chat_admin_phone, chat_admin_idcard, chat_secret,
   employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at)
VALUES
  (982001, '合规测试企业', 'wx-compliance-982', 'SC982', '$PII_MARKER', '13900000982', 'ID982SECRET', '$PII_MARKER',
   'employee-secret-982', 'contact-secret-982', 'callback-token-982', 'encoding-key-982', 982, NOW(), NOW());
INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, unionid, created_at, updated_at)
VALUES (982001, 982001, 'external-982', '$PII_MARKER', '隐私客户', 'https://example.invalid/pii.png', 'union-982', NOW(), NOW());
INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at)
VALUES (982001, 982001, 'room-982', '隐私客户群', 0, '$PII_MARKER', 0, NOW(), 200, 0, NOW(), NOW());
INSERT INTO mc_work_contact_room
  (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, created_at, updated_at)
VALUES (982001, 'external-982', 982001, 0, 'union-982', 982001, 1, 2, 1, NOW(), NOW(), NOW());
INSERT INTO mochat_go_saas_alerts
  (alert_key, tenant_id, alert_type, severity, status, metric, period_key, current_value, limit_value,
   occurrence_count, source, message, context_json, first_seen_at, last_seen_at, created_at, updated_at)
VALUES ('compliance-alert-982', 982, 'quota_exceeded', 'warning', 'open', 'contacts', 'lifetime', 11, 10,
        1, 'smoke', '$PII_MARKER', JSON_OBJECT('marker', '$PII_MARKER'), NOW(), NOW(), NOW(), NOW());
INSERT INTO mochat_go_saas_service_accounts
  (tenant_id, code, name, description, status, scopes_json, allowed_cidrs_json, version, created_by, updated_by, created_at, updated_at)
VALUES (982, 'compliance_agent', '$PII_MARKER', '待擦除服务账号', 'active', JSON_ARRAY('profile.read'), JSON_ARRAY(), 1, $OPERATOR_ID, $OPERATOR_ID, NOW(), NOW());
INSERT INTO mochat_go_saas_service_account_keys
  (service_account_id, name, key_prefix, key_hash, last_four, status, version, created_by, created_at, updated_at)
SELECT id, '待擦除密钥', 'mch_live_982', REPEAT('a', 64), '0982', 'active', 1, $OPERATOR_ID, NOW(), NOW()
FROM mochat_go_saas_service_accounts WHERE tenant_id = 982;
INSERT INTO mochat_go_saas_service_account_usage_daily
  (service_account_id, usage_date, route_key, request_count, rejected_count, last_used_at, last_used_ip, created_at, updated_at)
SELECT id, CURDATE(), 'GET /api/saas/v1/whoami', 3, 1, NOW(), '127.0.0.1', NOW(), NOW()
FROM mochat_go_saas_service_accounts WHERE tenant_id = 982;
INSERT INTO mochat_go_saas_billing_events
  (tenant_id, event_type, package_code, package_name, amount_cents, currency, paid_at, payment_method,
   external_order_no, actor_user_id, actor_tenant_id, remark, metadata_json, created_at, updated_at)
VALUES (982, 'renewal', 'compliance-business', '业务版', 9800, 'CNY', NOW(), 'gateway', 'COMPLIANCE-ORDER-982',
        $OPERATOR_ID, 1, '法定保留测试', JSON_OBJECT('marker', '$PII_MARKER'), NOW(), NOW());
INSERT INTO mochat_go_saas_payment_settlement_batches
  (batch_no, provider, provider_settlement_no, currency, status, source_sha256, entry_count, matched_count,
   total_amount_cents, total_net_cents, imported_by_user_id, imported_by_tenant_id, created_at, updated_at)
VALUES ('SET-COMPLIANCE-982', 'gateway', 'GW-SET-COMPLIANCE-982', 'CNY', 'reconciled', REPEAT('b', 64), 1, 1,
        9800, 9700, $OPERATOR_ID, 1, NOW(), NOW());
INSERT INTO mochat_go_saas_payment_settlement_entries
  (batch_id, line_no, provider, provider_transaction_no, transaction_type, order_no, amount_cents, fee_cents,
   net_amount_cents, currency, occurred_at, matched_tenant_id, matched_internal_no, expected_amount_cents,
   expected_currency, expected_status, reconciliation_status, handling_status, raw_json, created_at, updated_at)
SELECT id, 1, 'gateway', 'GW-TXN-COMPLIANCE-982', 'payment', 'COMPLIANCE-ORDER-982', 9800, -100,
       9700, 'CNY', NOW(), 982, 'COMPLIANCE-ORDER-982', 9800, 'CNY', 'paid', 'matched', 'none',
       JSON_OBJECT('marker', '$PII_MARKER'), NOW(), NOW()
FROM mochat_go_saas_payment_settlement_batches WHERE batch_no = 'SET-COMPLIANCE-982';
INSERT INTO mochat_go_saas_admin_operation_logs
  (tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name, before_json, after_json, remark, created_at, updated_at)
VALUES (982, $OPERATOR_ID, 1, 'smoke.pii', 'tenant', '982', '$PII_MARKER', JSON_OBJECT('marker', '$PII_MARKER'),
        JSON_OBJECT('marker', '$PII_MARKER'), '$PII_MARKER', NOW(), NOW());
SQL

mkdir -p "$UPLOAD_ROOT/tenant982" "$ARTIFACT_ROOT"
printf '%s\n' "$PII_MARKER" >"$UPLOAD_ROOT/tenant982/pii.txt"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_storage_objects
  (tenant_id, user_id, employee_id, corp_id, source, original_name, relative_path, content_type, size_bytes, created_at, updated_at)
VALUES (982, 0, 0, 982001, 'compliance_smoke', 'pii.txt', 'tenant982/pii.txt', 'text/plain',
        $(wc -c <"$UPLOAD_ROOT/tenant982/pii.txt" | tr -d ' '), NOW(), NOW());
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
  MOCHAT_FILE_STORAGE_ROOT="$UPLOAD_ROOT" \
  MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT="$ARTIFACT_ROOT" \
  MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY="$EXPORT_KEY" \
  MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID="compliance-smoke-v1" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

SUPER_TOKEN="$(login_token 13800000070 secret070 "$WORK_DIR/super-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000071 secret070 "$WORK_DIR/operator-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000072 secret070 "$WORK_DIR/approver1-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000073 secret070 "$WORK_DIR/approver2-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000074 secret070 "$WORK_DIR/auditor-auth.json")"
TENANT_TOKEN="$(login_token 13800000982 secret982 "$WORK_DIR/tenant-auth.json")"

api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceOverview?tenantId=982&limit=100" "$WORK_DIR/overview.json"
jq -e '.data.policy.version == 1 and .data.config.encryptionConfigured == true and .data.config.inventoryUnknownTables == [] and .data.config.inventoryTableCount == 158' "$WORK_DIR/overview.json" >/dev/null
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/complianceOverview?tenantId=982" "$WORK_DIR/auditor-overview.json"
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/complianceOverview?tenantId=982" "$WORK_DIR/tenant-forbidden.json" 403
api_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/complianceExport" '{"action":"request","tenantId":982,"reason":"无权限导出"}' "$WORK_DIR/auditor-export-forbidden.json" 403

POLICY_PAYLOAD_V1='{"status":"active","exportRetentionDays":30,"erasureGraceDays":0,"requireRecentExport":true,"recentExportMaxAgeDays":7,"billingRetentionDays":365,"auditRetentionDays":365,"serviceAccountUsageRetentionDays":90,"expectedVersion":1}'
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/compliancePolicy" "$POLICY_PAYLOAD_V1" "$WORK_DIR/direct-policy-blocked.json" 428
jq -e '.data.actionType == "compliance.policy.update" and .data.requiredApprovals == 2' "$WORK_DIR/direct-policy-blocked.json" >/dev/null
test "$(mysql_scalar 'SELECT version FROM mochat_go_saas_compliance_policies WHERE id = 1')" = "1"
apply_compliance_policy_update "$OPERATOR_TOKEN" "$POLICY_PAYLOAD_V1" "compliance-policy-v1" "policy-v1"
jq -e '.data.result.policy.version == 2 and .data.result.policy.erasureGraceDays == 0 and .data.result.policy.billingRetentionDays == 365 and .data.result.policy.serviceAccountUsageRetentionDays == 90' "$WORK_DIR/policy-v1-execute.json" >/dev/null

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExport" \
  '{"action":"request","tenantId":982,"reason":"租户注销前完整数据导出"}' "$WORK_DIR/export-request.json" 201
EXPORT_ID="$(jq -er '.data.export.id' "$WORK_DIR/export-request.json")"
jq -e '.data.export.status == "pending" and .data.export.encryptionKeyId == "compliance-smoke-v1"' "$WORK_DIR/export-request.json" >/dev/null
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExport" \
  "{\"action\":\"process\",\"exportId\":$EXPORT_ID}" "$WORK_DIR/export-complete.json"
jq -e '.data.export.status == "succeeded" and .data.export.encrypted == true and .data.export.encryptionKeyReady == true and .data.export.tableCount > 100 and .data.export.rowCount > 10 and .data.export.fileCount == 1 and (.data.export.sha256|length) == 64 and (.data.export.manifestSha256|length) == 64' "$WORK_DIR/export-complete.json" >/dev/null
ARTIFACT_FILE="$(find "$ARTIFACT_ROOT" -maxdepth 1 -type f -name '*.mgce' -print -quit)"
test -n "$ARTIFACT_FILE"
test "$(stat -f '%Lp' "$ARTIFACT_FILE")" = "600"
test "$(head -c 4 "$ARTIFACT_FILE")" = "MGCE"
if grep -a -F -q "$PII_MARKER" "$ARTIFACT_FILE"; then
  echo "plaintext PII leaked into encrypted compliance artifact" >&2
  exit 1
fi

api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/complianceExportDownload?exportId=$EXPORT_ID" "$WORK_DIR/auditor-download.json" 403
api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExportDownload?exportId=$EXPORT_ID" "$WORK_DIR/tenant-export.tar.gz"
tar -tzf "$WORK_DIR/tenant-export.tar.gz" | grep -q '^manifest.json$'
tar -tzf "$WORK_DIR/tenant-export.tar.gz" | grep -q '^data/mc_work_contact.jsonl$'
tar -tzf "$WORK_DIR/tenant-export.tar.gz" | grep -q '^data/mochat_go_saas_service_account_usage_daily.jsonl$'
tar -tzf "$WORK_DIR/tenant-export.tar.gz" | grep -q '^files/.*/pii.txt$'
tar -xOzf "$WORK_DIR/tenant-export.tar.gz" data/mc_work_contact.jsonl | grep -q "$PII_MARKER"
tar -xOzf "$WORK_DIR/tenant-export.tar.gz" files/"$(mysql_scalar "SELECT id FROM mochat_go_saas_storage_objects WHERE tenant_id = 982")"/pii.txt | grep -q "$PII_MARKER"
tar -xOzf "$WORK_DIR/tenant-export.tar.gz" manifest.json | jq -e '.tenant.id == 982 and .summary.fileCount == 1 and .summary.tableCount > 100' >/dev/null
test "$(mysql_scalar "SELECT download_count FROM mochat_go_saas_data_exports WHERE id = $EXPORT_ID")" = "1"

cp "$ARTIFACT_FILE" "$WORK_DIR/artifact-good.mgce"
printf 'X' | dd of="$ARTIFACT_FILE" bs=1 seek=20 conv=notrunc status=none
api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExportDownload?exportId=$EXPORT_ID" "$WORK_DIR/tampered-download.json" 409
cp "$WORK_DIR/artifact-good.mgce" "$ARTIFACT_FILE"

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExport" \
  '{"action":"request","tenantId":982,"reason":"提前删除双人审批验收导出"}' "$WORK_DIR/delete-export-request.json" 201
DELETE_EXPORT_ID="$(jq -er '.data.export.id' "$WORK_DIR/delete-export-request.json")"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExport" \
  "{\"action\":\"process\",\"exportId\":$DELETE_EXPORT_ID}" "$WORK_DIR/delete-export-complete.json"
jq -e '.data.export.status == "succeeded" and .data.export.deletionStatus == ""' "$WORK_DIR/delete-export-complete.json" >/dev/null
DELETE_ARTIFACT_NAME="$(mysql_scalar "SELECT artifact_name FROM mochat_go_saas_data_exports WHERE id = $DELETE_EXPORT_ID")"
DELETE_ARTIFACT_FILE="$ARTIFACT_ROOT/$DELETE_ARTIFACT_NAME"
test -f "$DELETE_ARTIFACT_FILE"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExport" \
  "{\"action\":\"delete\",\"exportId\":$DELETE_EXPORT_ID}" "$WORK_DIR/direct-delete-blocked.json" 428
jq -e '.data.actionType == "compliance.export.delete" and .data.requiredApprovals == 2' "$WORK_DIR/direct-delete-blocked.json" >/dev/null
test -f "$DELETE_ARTIFACT_FILE"
test "$(mysql_scalar "SELECT CONCAT(status, ':', deletion_status) FROM mochat_go_saas_data_exports WHERE id = $DELETE_EXPORT_ID")" = "succeeded:"

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  '{"action":"request","tenantId":982,"confirmation":"待擦除业务租户","reason":"仍启用时请求"}' "$WORK_DIR/active-tenant-erasure.json" 409
mysql_root "$DATABASE" -e "UPDATE mc_tenant SET status = 2 WHERE id = 982"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceLegalHold" \
  '{"action":"create","tenantId":982,"reason":"监管调查期间禁止删除"}' "$WORK_DIR/hold-create.json" 201
HOLD_ID="$(jq -er '.data.hold.id' "$WORK_DIR/hold-create.json")"
HOLD_VERSION="$(jq -er '.data.hold.version' "$WORK_DIR/hold-create.json")"
request_export_deletion_approval "$OPERATOR_TOKEN" "$DELETE_EXPORT_ID" "$WORK_DIR/hold-blocked-export-delete.json" 409
grep -q '法律保留' "$WORK_DIR/hold-blocked-export-delete.json"
test -f "$DELETE_ARTIFACT_FILE"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  '{"action":"request","tenantId":982,"confirmation":"待擦除业务租户","reason":"法律保留期间请求"}' "$WORK_DIR/hold-blocked-erasure.json" 409
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceLegalHold" \
  "{\"action\":\"release\",\"holdId\":$HOLD_ID,\"expectedVersion\":$HOLD_VERSION,\"reason\":\"监管调查已结束\"}" "$WORK_DIR/direct-hold-release-blocked.json" 428
jq -e '.data.actionType == "compliance.legal_hold.release" and .data.requiredApprovals == 2' "$WORK_DIR/direct-hold-release-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_legal_holds WHERE id = $HOLD_ID")" = "active:$HOLD_VERSION"

request_hold_release_approval "$OPERATOR_TOKEN" "$HOLD_ID" "$HOLD_VERSION" "$WORK_DIR/hold-release-approval-request.json"
HOLD_APPROVAL_ID="$(jq -er '.data.approval.id' "$WORK_DIR/hold-release-approval-request.json")"
HOLD_APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/hold-release-approval-request.json")"
jq -e --arg id "$HOLD_ID" '.data.approval.actionType == "compliance.legal_hold.release" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == $id and (.data.approval.targetName | startswith("HOLD-"))' "$WORK_DIR/hold-release-approval-request.json" >/dev/null
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.holdNo')) FROM mochat_go_saas_admin_approvals WHERE id = $HOLD_APPROVAL_ID")" = "$(mysql_scalar "SELECT hold_no FROM mochat_go_saas_legal_holds WHERE id = $HOLD_ID")"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.tenantId')) FROM mochat_go_saas_admin_approvals WHERE id = $HOLD_APPROVAL_ID")" = "982"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.expectedVersion')) FROM mochat_go_saas_admin_approvals WHERE id = $HOLD_APPROVAL_ID")" = "$HOLD_VERSION"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.releaseReason')) FROM mochat_go_saas_admin_approvals WHERE id = $HOLD_APPROVAL_ID")" = "监管调查已结束"
approve "$APPROVER1_TOKEN" "$HOLD_APPROVAL_ID" "$HOLD_APPROVAL_VERSION" "第一复核人确认解除法律保留" "$WORK_DIR/hold-release-approval-one.json"
HOLD_APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/hold-release-approval-one.json")"
approve "$APPROVER2_TOKEN" "$HOLD_APPROVAL_ID" "$HOLD_APPROVAL_VERSION" "第二复核人确认解除法律保留" "$WORK_DIR/hold-release-approval-two.json"
HOLD_APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/hold-release-approval-two.json")"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/hold-release-approval-two.json" >/dev/null
api_post "$SUPER_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$HOLD_APPROVAL_ID,\"expectedVersion\":$HOLD_APPROVAL_VERSION}" "$WORK_DIR/hold-release-approval-execute.json"
jq -e '.data.approval.status == "executed" and .data.result.hold.status == "released" and .data.result.hold.version == 2' "$WORK_DIR/hold-release-approval-execute.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $HOLD_APPROVAL_ID AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = (SELECT effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id = $HOLD_APPROVAL_ID) AND action = 'saas.admin.compliance.hold.release'")" = "1"

request_export_deletion_approval "$OPERATOR_TOKEN" "$DELETE_EXPORT_ID" "$WORK_DIR/delete-approval-request.json"
DELETE_APPROVAL_ID="$(jq -er '.data.approval.id' "$WORK_DIR/delete-approval-request.json")"
DELETE_APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/delete-approval-request.json")"
jq -e --arg id "$DELETE_EXPORT_ID" '.data.approval.actionType == "compliance.export.delete" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == $id' "$WORK_DIR/delete-approval-request.json" >/dev/null
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.artifactName')) FROM mochat_go_saas_admin_approvals WHERE id = $DELETE_APPROVAL_ID")" = "$DELETE_ARTIFACT_NAME"
approve "$APPROVER1_TOKEN" "$DELETE_APPROVAL_ID" "$DELETE_APPROVAL_VERSION" "第一复核人确认提前删除" "$WORK_DIR/delete-approval-one.json"
DELETE_APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/delete-approval-one.json")"
approve "$APPROVER2_TOKEN" "$DELETE_APPROVAL_ID" "$DELETE_APPROVAL_VERSION" "第二复核人确认提前删除" "$WORK_DIR/delete-approval-two.json"
DELETE_APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/delete-approval-two.json")"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/delete-approval-two.json" >/dev/null

mv "$DELETE_ARTIFACT_FILE" "$WORK_DIR/delete-artifact-good.mgce"
mkdir "$DELETE_ARTIFACT_FILE"
printf 'block deletion\n' >"$DELETE_ARTIFACT_FILE/blocker"
api_post "$SUPER_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DELETE_APPROVAL_ID,\"expectedVersion\":$DELETE_APPROVAL_VERSION}" "$WORK_DIR/delete-approval-execute.json"
jq -e '.data.approval.status == "executed" and .data.result.export.status == "succeeded" and .data.result.export.deletionStatus == "failed" and .data.result.export.deletionArtifactStatus == "failed" and .data.result.export.deletionRecordStatus == "pending" and .data.result.export.deletionAttempts == 1 and (.data.result.export.deletionLastError | length) > 0' "$WORK_DIR/delete-approval-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', deletion_status, ':', deletion_artifact_status, ':', deletion_record_status, ':', deletion_attempts, ':', deletion_approval_id) FROM mochat_go_saas_data_exports WHERE id = $DELETE_EXPORT_ID")" = "succeeded:failed:failed:pending:1:$DELETE_APPROVAL_ID"
api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExportDownload?exportId=$DELETE_EXPORT_ID" "$WORK_DIR/delete-failed-download.json" 409
rm "$DELETE_ARTIFACT_FILE/blocker"
rmdir "$DELETE_ARTIFACT_FILE"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceExport" \
  "{\"action\":\"delete-retry\",\"exportId\":$DELETE_EXPORT_ID}" "$WORK_DIR/delete-retry.json"
jq -e --argjson approvalId "$DELETE_APPROVAL_ID" '.data.export.status == "deleted" and .data.export.deletionStatus == "succeeded" and .data.export.deletionArtifactStatus == "missing" and .data.export.deletionRecordStatus == "deleted" and .data.export.deletionAttempts == 2 and .data.export.deletionApprovalId == $approvalId and (.data.export.deletedAt | length) > 0' "$WORK_DIR/delete-retry.json" >/dev/null
test ! -e "$DELETE_ARTIFACT_FILE"

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  '{"action":"request","tenantId":982,"confirmation":"错误名称","reason":"名称错误"}' "$WORK_DIR/wrong-confirmation.json" 400
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  '{"action":"request","tenantId":982,"confirmation":"待擦除业务租户","reason":"合同终止并已完成数据交付"}' "$WORK_DIR/erasure-request.json" 201
ERASURE_ID="$(jq -er '.data.erasure.id' "$WORK_DIR/erasure-request.json")"
jq -e '.data.erasure.status == "pending_approval" and .data.approval.actionType == "tenant.data.erase"' "$WORK_DIR/erasure-request.json" >/dev/null
request_export_deletion_approval "$OPERATOR_TOKEN" "$EXPORT_ID" "$WORK_DIR/erasure-reference-blocked-export-delete.json" 409
grep -q '擦除请求' "$WORK_DIR/erasure-reference-blocked-export-delete.json"
test -f "$ARTIFACT_FILE"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  "{\"action\":\"process\",\"requestId\":$ERASURE_ID}" "$WORK_DIR/unapproved-process.json" 409

request_approval "$OPERATOR_TOKEN" "$ERASURE_ID" "$WORK_DIR/approval-request.json"
APPROVAL_ID="$(jq -er '.data.approval.id' "$WORK_DIR/approval-request.json")"
APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/approval-request.json")"
jq -e '.data.approval.requiredApprovals == 2 and .data.approval.status == "pending"' "$WORK_DIR/approval-request.json" >/dev/null
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
  "{\"approvalId\":$APPROVAL_ID,\"expectedVersion\":$APPROVAL_VERSION,\"decision\":\"approve\",\"reason\":\"自批\"}" "$WORK_DIR/self-approval.json" 403
approve "$APPROVER1_TOKEN" "$APPROVAL_ID" "$APPROVAL_VERSION" "第一复核人确认" "$WORK_DIR/approval-one.json"
APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/approval-one.json")"
jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/approval-one.json" >/dev/null
approve "$APPROVER2_TOKEN" "$APPROVAL_ID" "$APPROVAL_VERSION" "第二复核人确认" "$WORK_DIR/approval-two.json"
APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/approval-two.json")"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2' "$WORK_DIR/approval-two.json" >/dev/null
api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$APPROVAL_ID,\"expectedVersion\":$APPROVAL_VERSION}" "$WORK_DIR/approval-execute.json"
jq -e '.data.approval.status == "executed" and .data.result.requestId > 0 and (.data.result.status == "approved" or .data.result.status == "waiting")' "$WORK_DIR/approval-execute.json" >/dev/null
test "$(mysql_scalar "SELECT approval_id FROM mochat_go_saas_erasure_requests WHERE id = $ERASURE_ID")" = "$APPROVAL_ID"

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  "{\"action\":\"process\",\"requestId\":$ERASURE_ID}" "$WORK_DIR/retention-block.json" 409
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_erasure_requests WHERE id = $ERASURE_ID")" = "blocked"
grep -q '法定保留期' "$WORK_DIR/retention-block.json"

POLICY_PAYLOAD_V2='{"status":"active","exportRetentionDays":30,"erasureGraceDays":0,"requireRecentExport":true,"recentExportMaxAgeDays":7,"billingRetentionDays":0,"auditRetentionDays":365,"serviceAccountUsageRetentionDays":90,"expectedVersion":2}'
apply_compliance_policy_update "$OPERATOR_TOKEN" "$POLICY_PAYLOAD_V2" "compliance-policy-v2" "policy-v2"
jq -e '.data.result.policy.version == 3 and .data.result.policy.billingRetentionDays == 0' "$WORK_DIR/policy-v2-execute.json" >/dev/null
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  "{\"action\":\"process\",\"requestId\":$ERASURE_ID}" "$WORK_DIR/erasure-complete.json"
jq -e '.data.erasure.status == "succeeded" and .data.erasure.totalSteps == 161 and .data.erasure.completedSteps == .data.erasure.totalSteps and .data.erasure.deletedRows > 10 and .data.erasure.redactedRows > 0 and (.data.erasure.verificationSha256|length) == 64 and (.data.erasure.report.steps | all(.status == "succeeded"))' "$WORK_DIR/erasure-complete.json" >/dev/null || {
  jq . "$WORK_DIR/erasure-complete.json" >&2
  exit 1
}

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_tenant WHERE id = 982 AND status = 2 AND deleted_at IS NOT NULL AND name LIKE 'erased-%' AND logo = '' AND server_ips IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact WHERE id = 982001")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room WHERE id = 982001")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_room WHERE id = 982001")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_account_usage_daily WHERE route_key = 'GET /api/saas/v1/whoami'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_service_accounts WHERE tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries WHERE matched_tenant_id = 982")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_payment_settlement_entries WHERE provider_transaction_no = 'GW-TXN-COMPLIANCE-982' AND matched_tenant_id = 0 AND matched_payment_order_id IS NULL AND raw_json IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_tenant_tombstones WHERE tenant_id = 982 AND erasure_request_id = $ERASURE_ID AND CHAR_LENGTH(verification_sha256) = 64")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_erasure_steps WHERE request_id = $ERASURE_ID AND status <> 'succeeded'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = 982 AND (target_name = '$PII_MARKER' OR CAST(before_json AS CHAR) LIKE '%$PII_MARKER%' OR CAST(after_json AS CHAR) LIKE '%$PII_MARKER%' OR remark LIKE '%$PII_MARKER%')")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_data_exports WHERE tenant_id = 982 AND status = 'deleted' AND deleted_at IS NOT NULL AND request_reason = '[redacted by tenant erasure]'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_legal_holds WHERE tenant_id = 982 AND reason = '[redacted by tenant erasure]'")" = "1"
test ! -e "$UPLOAD_ROOT/tenant982/pii.txt"
test ! -e "$ARTIFACT_FILE"
if find "$ARTIFACT_ROOT" -maxdepth 1 -type f -name '*.mgce' | grep -q .; then
  echo "compliance export artifact remained after erasure" >&2
  exit 1
fi

status="$(curl -sS -o "$WORK_DIR/erased-login.json" -w '%{http_code}' -H 'Content-Type: application/json' \
  -d '{"phone":"13800000982","password":"secret982"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$status" = "401"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasure" \
  "{\"action\":\"process\",\"requestId\":$ERASURE_ID}" "$WORK_DIR/reprocess.json" 409
api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/complianceErasureSteps?requestId=$ERASURE_ID" "$WORK_DIR/steps.json"
jq -e '.data.steps | length > 100 and all(.status == "succeeded")' "$WORK_DIR/steps.json" >/dev/null
api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15" "$WORK_DIR/system-health.json"
jq -e '.data.summary.checkCount == 30 and any(.data.checks[]; .code == "saas_alert_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wecom_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wechat_open_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "service_account_key_protection" and .status == "healthy") and any(.data.checks[]; .code == "backup_automation" and .status == "critical") and any(.data.checks[]; .code == "backup_retention_cleanup" and .status == "healthy") and any(.data.checks[]; .code == "compliance_export_queue" and .status == "healthy") and any(.data.checks[]; .code == "compliance_erasure_queue" and .status == "healthy") and any(.data.checks[]; .code == "compliance_configuration" and .status == "healthy") and any(.data.checks[]; .code == "domain_delivery_queue" and .status == "healthy") and any(.data.checks[]; .code == "domain_certificate_lifecycle" and .status == "healthy")' "$WORK_DIR/system-health.json" >/dev/null

MOCHAT_FILE_STORAGE_ROOT="$UPLOAD_ROOT" \
MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT="$ARTIFACT_ROOT" \
MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY="$EXPORT_KEY" \
MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID=compliance-smoke-v1 \
"$MAINTENANCE_BIN" -dsn "$DSN" -action compliance-status >"$WORK_DIR/maintenance-status.out"
grep -q $'action\tcompliance-status' "$WORK_DIR/maintenance-status.out"
grep -q $'inventory_unknown_tables\t0' "$WORK_DIR/maintenance-status.out"
grep -q $'encryption_configured\ttrue' "$WORK_DIR/maintenance-status.out"
MOCHAT_FILE_STORAGE_ROOT="$UPLOAD_ROOT" \
MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT="$ARTIFACT_ROOT" \
MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY="$EXPORT_KEY" \
MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID=compliance-smoke-v1 \
"$MAINTENANCE_BIN" -dsn "$DSN" -action compliance-process -compliance-limit 10 >"$WORK_DIR/maintenance-process.out"
grep -q $'action\tcompliance-process' "$WORK_DIR/maintenance-process.out"
grep -q $'exports_processed\t0' "$WORK_DIR/maintenance-process.out"
grep -q $'export_deletions_processed\t0' "$WORK_DIR/maintenance-process.out"
grep -q $'erasures_processed\t0' "$WORK_DIR/maintenance-process.out"

if grep -F -q "$EXPORT_KEY" "$GO_LOG"; then
  echo "compliance encryption master key leaked into application logs" >&2
  exit 1
fi
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action IN ('saas.admin.compliance.export.request','saas.admin.compliance.export.complete','saas.admin.compliance.export.download','saas.admin.compliance.hold.create','saas.admin.compliance.hold.release','saas.admin.compliance.erasure.request','saas.admin.compliance.erasure.authorize','saas.admin.compliance.erasure.complete')")" -ge 8
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action IN ('saas.admin.compliance.export.delete.schedule','saas.admin.compliance.export.delete.retry','saas.admin.compliance.export.delete') AND target_id = (SELECT export_no FROM mochat_go_saas_data_exports WHERE id = $DELETE_EXPORT_ID)")" = "3"

echo "SaaS compliance lifecycle smoke passed"
