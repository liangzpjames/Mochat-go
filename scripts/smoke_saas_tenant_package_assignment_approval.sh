#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-tenant-package-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13409}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26459}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18183}"
DATABASE="mochat_saas_tenant_package_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-tenant-package-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-tenant-package-approval-jwt-secret}"

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
trap 'echo "SaaS tenant package assignment approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
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

request_assignment_approval() {
  local payload="$1" key="$2" output="$3" expected="${4:-200}" request_body
  request_body="$(jq -cn --argjson payload "$payload" --arg key "$key" \
    '{actionType:"tenant.package.update",payload:$payload,reason:"双人复核租户套餐权益变更",idempotencyKey:$key}')"
  api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$output" "$expected"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认租户权益\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认租户权益\"}" \
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
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.package.update' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'mochat_go_saas_tenant_packages' AND COLUMN_NAME = 'version'")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,权益治理平台,13800000021,secret021,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
991,权益变更租户,13800000991,secret991,租户管理员,SaaS租户超级管理员,growth,成长版,2,100,1000,50,5,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
992,首次分配租户,13800000992,secret992,租户管理员,SaaS租户超级管理员,starter,入门版,1,5,100,10,2,5,5,5,5,5,5,5,5,5,5,5,5,5,128,5,5,5,5,5,1,100,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "1:growth"
mysql_scalar "UPDATE mochat_go_saas_usage_counters SET used_value = 8, updated_by = 'tenant-package-approval-smoke' WHERE tenant_id = 991 AND metric = 'users' AND period_key = 'lifetime' AND deleted_at IS NULL" >/dev/null
mysql_scalar "DELETE FROM mochat_go_saas_tenant_packages WHERE tenant_id = 992; DELETE FROM mochat_go_saas_subscriptions WHERE tenant_id = 992; DELETE FROM mochat_go_saas_subscription_events WHERE tenant_id = 992;" >/dev/null

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000022', password, '权益运营申请人', 0, '运营部', '权益运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000021' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000023', password, '权益复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000021' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000024', password, '权益复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000021' LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000022'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000023'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000024'")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $OPERATIONS_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());
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

PLATFORM_TOKEN="$(login_token 13800000021 secret021 "$WORK_DIR/platform-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000022 secret021 "$WORK_DIR/operator-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000023 secret021 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000024 secret021 "$WORK_DIR/approver-two-auth.json")"

api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "tenant.package.update")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.tenants.manage" and .targetType == "saas_tenant_package")' "$WORK_DIR/policies.json" >/dev/null

ASSIGN_BODY='{"tenantId":991,"packageCode":"starter","expiresAt":"2027-06-30","remark":"合同降配","expectedVersion":1}'
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/tenantPackage" "$ASSIGN_BODY" "$WORK_DIR/assign-direct.json" 428
jq -e '.code == 428 and .data.actionType == "tenant.package.update" and .data.requiredApprovals == 2' "$WORK_DIR/assign-direct.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "1:growth"

request_assignment_approval "$ASSIGN_BODY" "tenant-package-991-v1" "$WORK_DIR/assign-request.json"
ASSIGN_APPROVAL_ID="$(json_value "$WORK_DIR/assign-request.json" data.approval.id)"
ASSIGN_APPROVAL_VERSION="$(json_value "$WORK_DIR/assign-request.json" data.approval.version)"
jq -e '.data.approval.actionType == "tenant.package.update" and .data.approval.riskLevel == "critical" and .data.approval.targetType == "saas_tenant_package" and .data.approval.targetId == "991" and .data.approval.requiredApprovals == 2' "$WORK_DIR/assign-request.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.update.expectedVersion')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.update.expectedPackageVersion')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.update.expectedTenantStatus')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.current.version')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.current.limits.maxUsers')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.targetPackage.limits.maxUsers')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.impact.overLimitTenantCount'))) FROM mochat_go_saas_admin_approvals WHERE id = $ASSIGN_APPROVAL_ID")" = "1:1:1:1:100:5:1"
approve_twice "$ASSIGN_APPROVAL_ID" "$ASSIGN_APPROVAL_VERSION" "assign"
ASSIGN_APPROVAL_VERSION="$(json_value "$WORK_DIR/assign-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$ASSIGN_APPROVAL_ID,\"expectedVersion\":$ASSIGN_APPROVAL_VERSION}" "$WORK_DIR/assign-execute.json"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId > 0 and .data.result.tenantId == 991 and .data.result.packageCode == "starter" and .data.result.version == 2 and .data.result.operationId > 0 and .data.result.metricsRefreshPending == false and .data.result.impact.overLimitTenantCount == 1' "$WORK_DIR/assign-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code, ':', JSON_UNQUOTE(JSON_EXTRACT(limits_json, '$.maxUsers'))) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "2:starter:5"
test "$(mysql_scalar "SELECT package_code FROM mochat_go_saas_subscriptions WHERE tenant_id = 991 AND deleted_at IS NULL")" = "starter"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = (SELECT effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id = $ASSIGN_APPROVAL_ID) AND action = 'tenant.package.update' AND target_type = 'saas_tenant_package' AND target_id = '991'")" = "1"

STALE_BODY='{"tenantId":991,"packageCode":"growth","expiresAt":"2028-01-01","remark":"等待期后升级","expectedVersion":2}'
request_assignment_approval "$STALE_BODY" "tenant-package-991-stale" "$WORK_DIR/stale-request.json"
STALE_APPROVAL_ID="$(json_value "$WORK_DIR/stale-request.json" data.approval.id)"
STALE_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-request.json" data.approval.version)"
approve_twice "$STALE_APPROVAL_ID" "$STALE_APPROVAL_VERSION" "stale"
STALE_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-vote-two.json" data.approval.version)"
ASSIGN_OPERATION_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'tenant.package.update' AND target_id = '991'")"
# Simulate a concurrent package write without coupling this smoke to the renewal approval workflow.
mysql_scalar "UPDATE mochat_go_saas_tenant_packages SET expires_at = '2027-12-31 00:00:00', version = version + 1, updated_at = NOW() WHERE tenant_id = 991 AND deleted_at IS NULL" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "3:starter"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$STALE_APPROVAL_ID,\"expectedVersion\":$STALE_APPROVAL_VERSION}" "$WORK_DIR/stale-execute.json" 409
jq -e '.code == 409 and (.msg | contains("版本已变化"))' "$WORK_DIR/stale-execute.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $STALE_APPROVAL_ID AND status = 'approved' AND effect_applied_at IS NULL AND effect_operation_id = 0 AND last_error LIKE '%版本已变化%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'tenant.package.update' AND target_id = '991'")" = "$ASSIGN_OPERATION_COUNT"

CREATE_BODY='{"tenantId":992,"packageCode":"growth","expiresAt":"2028-06-30","remark":"首次开通权益","expectedVersion":0}'
request_assignment_approval "$CREATE_BODY" "tenant-package-992-create" "$WORK_DIR/create-request.json"
CREATE_APPROVAL_ID="$(json_value "$WORK_DIR/create-request.json" data.approval.id)"
CREATE_APPROVAL_VERSION="$(json_value "$WORK_DIR/create-request.json" data.approval.version)"
test "$(mysql_scalar "SELECT CONCAT(JSON_EXTRACT(request_json, '$.current') IS NULL, ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.update.expectedVersion')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.targetPackage.code'))) FROM mochat_go_saas_admin_approvals WHERE id = $CREATE_APPROVAL_ID")" = "1:0:growth"
approve_twice "$CREATE_APPROVAL_ID" "$CREATE_APPROVAL_VERSION" "create"
CREATE_APPROVAL_VERSION="$(json_value "$WORK_DIR/create-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$CREATE_APPROVAL_ID,\"expectedVersion\":$CREATE_APPROVAL_VERSION}" "$WORK_DIR/create-execute.json"
jq -e '.data.approval.status == "executed" and .data.result.tenantId == 992 and .data.result.packageCode == "growth" and .data.result.version == 1 and .data.result.operationId > 0' "$WORK_DIR/create-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 992")" = "1:growth"
test "$(mysql_scalar "SELECT package_code FROM mochat_go_saas_subscriptions WHERE tenant_id = 992 AND deleted_at IS NULL")" = "growth"

CREATE_OPERATION_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'tenant.package.update' AND target_id = '992'")"
mysql_scalar "UPDATE mochat_go_saas_admin_approvals SET status = 'executing', execution_started_at = DATE_SUB(NOW(), INTERVAL 20 MINUTE), executed_at = NULL, version = version + 1 WHERE id = $CREATE_APPROVAL_ID" >/dev/null
RECOVERY_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_admin_approvals WHERE id = $CREATE_APPROVAL_ID")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$CREATE_APPROVAL_ID,\"expectedVersion\":$RECOVERY_VERSION}" "$WORK_DIR/create-recovery.json"
jq -e '.data.approval.status == "executed" and .data.result.recovered == true and .data.result.effectOperationId > 0' "$WORK_DIR/create-recovery.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 992")" = "1:growth"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'tenant.package.update' AND target_id = '992'")" = "$CREATE_OPERATION_COUNT"

echo "SaaS tenant package assignment approval smoke passed"
