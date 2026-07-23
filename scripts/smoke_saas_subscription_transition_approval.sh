#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-subscription-transition-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13412}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26462}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18186}"
DATABASE="mochat_saas_subscription_transition_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-subscription-transition-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-subscription-transition-approval-jwt-secret}"

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
trap 'echo "SaaS subscription transition approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
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

request_transition_approval() {
  local payload="$1" key="$2" output="$3" expected="${4:-200}" request_body
  request_body="$(jq -cn --argjson payload "$payload" --arg key "$key" \
    '{actionType:"tenant.subscription.transition",payload:$payload,reason:"双人复核租户订阅状态迁移",idempotencyKey:$key}')"
  api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$output" "$expected"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认合同与客户通知\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认访问权变化\"}" \
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
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.subscription.transition' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,订阅治理平台,13800000051,secret051,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
992,订阅迁移租户,13800000992,secret992,租户管理员,SaaS租户超级管理员,growth,成长版,2,100,1000,50,5,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000052', password, '订阅运营申请人', 0, '运营部', '订阅运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000051' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000053', password, '订阅复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000051' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000054', password, '订阅复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000051' LIMIT 1;

INSERT INTO mochat_go_saas_subscriptions
  (tenant_id, package_code, package_name, status, billing_cycle, current_period_starts_at,
   current_period_ends_at, grace_ends_at, version, state_reason, metadata_json, created_at, updated_at, deleted_at)
SELECT tenant_id, package_code, package_name, 'active', 'custom', NOW(), '2037-01-01 00:00:00',
       '2037-01-08 00:00:00', 1, '0088 smoke initial active subscription',
       JSON_OBJECT('source', '0088_smoke'), NOW(), NOW(), NULL
FROM mochat_go_saas_tenant_packages
WHERE tenant_id = 992 AND deleted_at IS NULL
LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000052'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000053'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000054'")"
FINANCE_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_finance'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $FINANCE_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());
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

PLATFORM_TOKEN="$(login_token 13800000051 secret051 "$WORK_DIR/platform-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000052 secret051 "$WORK_DIR/operator-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000053 secret051 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000054 secret051 "$WORK_DIR/approver-two-auth.json")"
TENANT_TOKEN="$(login_token 13800000992 secret992 "$WORK_DIR/tenant-auth.json")"

api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "tenant.subscription.transition")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.finance.manage" and .targetType == "subscription" and .governanceLocked == true)' "$WORK_DIR/policies.json" >/dev/null

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/subscriptionTransition" \
  '{"tenantId":992,"status":"suspended","expectedVersion":1,"reason":"合同暂停"}' \
  "$WORK_DIR/direct-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "tenant.subscription.transition" and .data.requiredApprovals == 2' "$WORK_DIR/direct-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_subscriptions WHERE tenant_id = 992")" = "active:1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscription_events WHERE tenant_id = 992")" = "0"

SUSPEND_PAYLOAD='{"tenantId":992,"status":"suspended","expectedVersion":1,"reason":"合同暂停"}'
request_transition_approval "$SUSPEND_PAYLOAD" "subscription-transition-suspend-0088" "$WORK_DIR/suspend-request.json"
SUSPEND_APPROVAL_ID="$(json_value "$WORK_DIR/suspend-request.json" data.approval.id)"
SUSPEND_APPROVAL_VERSION="$(json_value "$WORK_DIR/suspend-request.json" data.approval.version)"
jq -e '.data.approval.actionType == "tenant.subscription.transition" and .data.approval.riskLevel == "critical" and .data.approval.targetType == "subscription" and .data.approval.requiredApprovals == 2 and .data.approval.request.subscription.tenantId == 992 and .data.approval.request.subscription.tenantStatus == 1 and .data.approval.request.subscription.status == "active" and .data.approval.request.subscription.version == 1 and .data.approval.request.transition.status == "suspended" and .data.approval.request.transition.expectedVersion == 1 and .data.approval.request.transition.source == "approval" and (.data.approval.request.transition.idempotencyKey | startswith("approval:"))' "$WORK_DIR/suspend-request.json" >/dev/null
request_transition_approval "$SUSPEND_PAYLOAD" "subscription-transition-suspend-0088" "$WORK_DIR/suspend-idempotent.json"
jq -e --argjson approvalId "$SUSPEND_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/suspend-idempotent.json" >/dev/null

approve_twice "$SUSPEND_APPROVAL_ID" "$SUSPEND_APPROVAL_VERSION" "suspend"
SUSPEND_APPROVAL_VERSION="$(json_value "$WORK_DIR/suspend-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$SUSPEND_APPROVAL_ID,\"expectedVersion\":$SUSPEND_APPROVAL_VERSION}" "$WORK_DIR/suspend-execute.json"
SUSPEND_OPERATION_ID="$(json_value "$WORK_DIR/suspend-execute.json" data.result.operationId)"
SUSPEND_EVENT_ID="$(json_value "$WORK_DIR/suspend-execute.json" data.result.eventId)"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.changed == true and .data.result.previousStatus == "active" and .data.result.subscription.status == "suspended" and .data.result.subscription.version == 2 and .data.result.eventId > 0 and .data.result.operationId > 0' "$WORK_DIR/suspend-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_subscriptions WHERE tenant_id = 992")" = "suspended:2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscription_events WHERE id = $SUSPEND_EVENT_ID AND tenant_id = 992 AND event_type = 'transition' AND from_status = 'active' AND to_status = 'suspended' AND source = 'approval' AND idempotency_key LIKE 'approval:%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $SUSPEND_OPERATION_ID AND tenant_id = 992 AND action = 'tenant.subscription.transition' AND target_type = 'subscription'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $SUSPEND_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $SUSPEND_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

NEW_LOGIN_STATUS="$(curl -sS -o "$WORK_DIR/suspended-login.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{"phone":"13800000992","password":"secret992"}' "http://$GO_ADDR/dashboard/user/auth")"
test "$NEW_LOGIN_STATUS" = "403"
grep -q '租户订阅已暂停' "$WORK_DIR/suspended-login.json"
OLD_TOKEN_STATUS="$(curl -sS -o "$WORK_DIR/suspended-old-token.json" -w '%{http_code}' -H "Authorization: Bearer $TENANT_TOKEN" "http://$GO_ADDR/dashboard/user/loginShow")"
test "$OLD_TOKEN_STATUS" = "401"
grep -q 'user not found' "$WORK_DIR/suspended-old-token.json"

STALE_VERSION_PAYLOAD='{"tenantId":992,"status":"active","currentPeriodEndsAt":"2037-06-01 00:00:00","expectedVersion":2,"reason":"恢复订阅"}'
request_transition_approval "$STALE_VERSION_PAYLOAD" "subscription-transition-stale-version-0088" "$WORK_DIR/stale-version-request.json"
STALE_VERSION_APPROVAL_ID="$(json_value "$WORK_DIR/stale-version-request.json" data.approval.id)"
STALE_VERSION_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-version-request.json" data.approval.version)"
approve_twice "$STALE_VERSION_APPROVAL_ID" "$STALE_VERSION_APPROVAL_VERSION" "stale-version"
STALE_VERSION_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-version-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_subscriptions SET version = version + 1, updated_at = NOW() WHERE tenant_id = 992" >/dev/null
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$STALE_VERSION_APPROVAL_ID,\"expectedVersion\":$STALE_VERSION_APPROVAL_VERSION}" "$WORK_DIR/stale-version-execute.json" 409
jq -e '.code == 409 and (.msg | contains("租户订阅版本已变化"))' "$WORK_DIR/stale-version-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_subscriptions WHERE tenant_id = 992")" = "suspended:3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $STALE_VERSION_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND effect_applied_at IS NULL AND last_error LIKE '%租户订阅版本已变化%'")" = "1"

RESTORE_PAYLOAD='{"tenantId":992,"status":"active","currentPeriodEndsAt":"2037-06-01 00:00:00","expectedVersion":3,"reason":"恢复订阅"}'
request_transition_approval "$RESTORE_PAYLOAD" "subscription-transition-restore-0088" "$WORK_DIR/restore-request.json"
RESTORE_APPROVAL_ID="$(json_value "$WORK_DIR/restore-request.json" data.approval.id)"
RESTORE_APPROVAL_VERSION="$(json_value "$WORK_DIR/restore-request.json" data.approval.version)"
approve_twice "$RESTORE_APPROVAL_ID" "$RESTORE_APPROVAL_VERSION" "restore"
RESTORE_APPROVAL_VERSION="$(json_value "$WORK_DIR/restore-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$RESTORE_APPROVAL_ID,\"expectedVersion\":$RESTORE_APPROVAL_VERSION}" "$WORK_DIR/restore-execute.json"
jq -e '.data.result.subscription.status == "active" and .data.result.subscription.version == 4 and .data.approval.effectOperationId == .data.result.operationId' "$WORK_DIR/restore-execute.json" >/dev/null
TENANT_TOKEN="$(login_token 13800000992 secret992 "$WORK_DIR/restored-tenant-auth.json")"

TENANT_DRIFT_PAYLOAD='{"tenantId":992,"status":"past_due","expectedVersion":4,"reason":"账期逾期"}'
request_transition_approval "$TENANT_DRIFT_PAYLOAD" "subscription-transition-tenant-drift-0088" "$WORK_DIR/tenant-drift-request.json"
TENANT_DRIFT_APPROVAL_ID="$(json_value "$WORK_DIR/tenant-drift-request.json" data.approval.id)"
TENANT_DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/tenant-drift-request.json" data.approval.version)"
approve_twice "$TENANT_DRIFT_APPROVAL_ID" "$TENANT_DRIFT_APPROVAL_VERSION" "tenant-drift"
TENANT_DRIFT_APPROVAL_VERSION="$(json_value "$WORK_DIR/tenant-drift-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mc_tenant SET status = 2, updated_at = NOW() WHERE id = 992" >/dev/null
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$TENANT_DRIFT_APPROVAL_ID,\"expectedVersion\":$TENANT_DRIFT_APPROVAL_VERSION}" "$WORK_DIR/tenant-drift-execute.json" 409
jq -e '.code == 409 and (.msg | contains("租户状态已变化"))' "$WORK_DIR/tenant-drift-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_subscriptions WHERE tenant_id = 992")" = "active:4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $TENANT_DRIFT_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND last_error LIKE '%租户状态已变化%'")" = "1"
mysql_scalar "UPDATE mc_tenant SET status = 1, updated_at = NOW() WHERE id = 992" >/dev/null

PLATFORM_BLOCK_PAYLOAD='{"tenantId":1,"status":"suspended","reason":"不应允许"}'
request_transition_approval "$PLATFORM_BLOCK_PAYLOAD" "subscription-transition-platform-block-0088" "$WORK_DIR/platform-block.json" 400
jq -e '.code == 400 and (.msg | contains("platform tenant subscription cannot be blocked"))' "$WORK_DIR/platform-block.json" >/dev/null

mysql_root "$DATABASE" <<'SQL'
UPDATE mochat_go_saas_subscriptions
SET status = 'active', current_period_ends_at = DATE_SUB(NOW(), INTERVAL 1 DAY),
    grace_ends_at = DATE_ADD(NOW(), INTERVAL 1 DAY), version = version + 1,
    state_reason = '0088 smoke reconcile due', updated_at = NOW()
WHERE tenant_id = 992;
SQL
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/subscriptionReconcile" \
  '{"tenantId":992,"limit":10,"dryRun":false}' "$WORK_DIR/reconcile.json"
jq -e '.data.changedCount == 1 and .data.failedCount == 0 and .data.transitions[0].subscription.status == "grace"' "$WORK_DIR/reconcile.json" >/dev/null
test "$(mysql_scalar "SELECT status FROM mochat_go_saas_subscriptions WHERE tenant_id = 992")" = "grace"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscription_events WHERE tenant_id = 992 AND event_type = 'reconciled' AND source = 'reconcile'")" = "1"
TENANT_TOKEN="$(login_token 13800000992 secret992 "$WORK_DIR/grace-tenant-auth.json")"
test -n "$TENANT_TOKEN"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/page" "$WORK_DIR/page.html"
grep -q 'id="transitionSubscription"' "$WORK_DIR/page.html"
grep -q '提交订阅审批' "$WORK_DIR/page.html"
grep -q "approvalActionRequired('tenant.subscription.transition', 0)" "$WORK_DIR/page.html"
grep -q "'tenant.subscription.transition'" "$WORK_DIR/page.html"

echo "SaaS subscription transition approval smoke passed"
