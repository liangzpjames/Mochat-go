#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-tenant-renewal-approval-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13411}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26461}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18185}"
DATABASE="mochat_saas_tenant_renewal_approval"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-tenant-renewal-approval.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-tenant-renewal-approval-jwt-secret}"

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
trap 'echo "SaaS tenant renewal approval smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
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

request_renewal_approval() {
  local payload="$1" key="$2" output="$3" expected="${4:-200}" request_body
  request_body="$(jq -cn --argjson payload "$payload" --arg key "$key" \
    '{actionType:"tenant.renewal",payload:$payload,reason:"双人复核租户续费生效",idempotencyKey:$key}')"
  api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$output" "$expected"
}

approve_twice() {
  local approval_id="$1" approval_version="$2" prefix="$3"
  api_post "$APPROVER1_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第一复核人确认续费合同与到账\"}" \
    "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1' "$WORK_DIR/$prefix-vote-one.json" >/dev/null
  api_post "$APPROVER2_TOKEN" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version,\"decision\":\"approve\",\"reason\":\"第二复核人确认续费权益与到期日\"}" \
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
grep -q $'0086_saas_tenant_provision_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.renewal' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND sla_minutes = 120 AND reminder_minutes = 30 AND expiry_hours = 12")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,续费治理平台,13800000041,secret041,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
991,续费审批租户,13800000991,secret991,租户管理员,SaaS租户超级管理员,growth,成长版,2,100,1000,50,5,20,20,20,20,20,20,20,20,20,20,20,20,20,512,20,20,20,20,20,2,1000,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code, ':', DATE_FORMAT(expires_at, '%Y-%m-%d')) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991 AND deleted_at IS NULL")" = "1:growth:2037-01-01"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscriptions WHERE tenant_id = 991 AND deleted_at IS NULL")" = "0"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000042', password, '续费运营申请人', 0, '运营部', '续费运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000041' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000043', password, '续费复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000041' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000044', password, '续费复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000041' LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000042'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000043'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000044'")"
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

PLATFORM_TOKEN="$(login_token 13800000041 secret041 "$WORK_DIR/platform-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000042 secret041 "$WORK_DIR/operator-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000043 secret041 "$WORK_DIR/approver-one-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000044 secret041 "$WORK_DIR/approver-two-auth.json")"

api_get "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalPolicies" "$WORK_DIR/policies.json"
jq -e '.data.required == true and (.data.policies | length) == 31 and ([.data.policies[] | select(.riskLevel == "critical")] | length) == 31 and ([.data.policies[] | select(.actionType == "tenant.renewal")][0] | .enabled == true and .requiredApprovals == 2 and .requiredPermission == "platform.tenants.manage" and .targetType == "saas_tenant_renewal")' "$WORK_DIR/policies.json" >/dev/null

TASK_BODY='{"tenantId":991,"packageCode":"growth","expiresAt":"2037-06-01","amountCents":880000,"currency":"CNY","paidAt":"2026-07-16","paymentMethod":"bank","externalOrderNo":"RENEW-0087-TASK","remark":"0087任务续费"}'
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/tenantRenewal" "$TASK_BODY" "$WORK_DIR/direct-route-blocked.json" 428
jq -e '.code == 428 and .data.actionType == "tenant.renewal" and .data.requiredApprovals == 2' "$WORK_DIR/direct-route-blocked.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991")" = "0"
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "1:growth"

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/tenantRenewalTask" "$TASK_BODY" "$WORK_DIR/task-create.json"
TASK_ID="$(json_value "$WORK_DIR/task-create.json" data.task.id)"
TASK_VERSION="$(json_value "$WORK_DIR/task-create.json" data.task.version)"
jq -e '.data.task.taskType == "tenant_renewal" and .data.task.status == "pending" and .data.task.version == 1 and .data.result.blocked == false and .data.result.previousPackageVersion == 1 and .data.result.packageVersion > 0 and .data.result.subscriptionExists == false and .data.result.subscriptionVersion == 0' "$WORK_DIR/task-create.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_tasks WHERE id = $TASK_ID AND task_type = 'tenant_renewal' AND status = 'pending' AND version = 1 AND JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.externalOrderNo')) = 'RENEW-0087-TASK'")" = "1"

api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/tenantRenewalTaskApply" "{\"taskId\":$TASK_ID}" "$WORK_DIR/task-direct-apply-blocked.json" 428
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/tenantRenewalTaskBulkApply" '{"taskType":"tenant_renewal","status":"pending","limit":20,"remark":"不应直接批量续费"}' "$WORK_DIR/task-bulk-apply-blocked.json" 428
test "$(mysql_scalar "SELECT CONCAT(status, ':', version) FROM mochat_go_saas_admin_tasks WHERE id = $TASK_ID")" = "pending:1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991")" = "0"

TASK_APPROVAL_PAYLOAD="$(jq -cn --argjson taskId "$TASK_ID" --argjson expectedTaskVersion "$TASK_VERSION" '{taskId:$taskId,expectedTaskVersion:$expectedTaskVersion}')"
request_renewal_approval "$TASK_APPROVAL_PAYLOAD" "tenant-renewal-task-0087" "$WORK_DIR/task-approval-request.json"
TASK_APPROVAL_ID="$(json_value "$WORK_DIR/task-approval-request.json" data.approval.id)"
TASK_APPROVAL_VERSION="$(json_value "$WORK_DIR/task-approval-request.json" data.approval.version)"
jq -e --argjson taskId "$TASK_ID" '.data.approval.actionType == "tenant.renewal" and .data.approval.riskLevel == "critical" and .data.approval.targetType == "saas_tenant_renewal" and .data.approval.targetId == ("task:" + ($taskId|tostring)) and .data.approval.requiredApprovals == 2 and .data.approval.request.source == "task" and .data.approval.request.taskId == $taskId and .data.approval.request.expectedTaskVersion == 1 and (.data.approval.request.expectedTaskRequestSha256 | length) == 64 and .data.approval.request.expectedTenantStatus == 1 and .data.approval.request.currentPackage.version == 1 and .data.approval.request.targetPackage.version > 0 and .data.approval.request.subscription.exists == false and .data.approval.request.subscription.version == 0 and .data.approval.request.renewal.expiresAt == "2037-06-01 00:00:00"' "$WORK_DIR/task-approval-request.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.expectedTaskVersion')), ':', LENGTH(JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.expectedTaskRequestSha256'))), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.currentPackage.version')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.targetPackage.version')), ':', JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.subscription.version'))) FROM mochat_go_saas_admin_approvals WHERE id = $TASK_APPROVAL_ID")" = "1:64:1:1:0"

approve_twice "$TASK_APPROVAL_ID" "$TASK_APPROVAL_VERSION" "task"
TASK_APPROVAL_VERSION="$(json_value "$WORK_DIR/task-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$TASK_APPROVAL_ID,\"expectedVersion\":$TASK_APPROVAL_VERSION}" "$WORK_DIR/task-execute.json"
TASK_OPERATION_ID="$(json_value "$WORK_DIR/task-execute.json" data.result.operationId)"
TASK_BILLING_ID="$(json_value "$WORK_DIR/task-execute.json" data.result.billingEventId)"
jq -e --argjson taskId "$TASK_ID" '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.source == "task" and .data.result.taskId == $taskId and .data.result.tenantId == 991 and .data.result.packageCode == "growth" and .data.result.expiresAt == "2037-06-01 00:00:00" and .data.result.amountCents == 880000 and .data.result.billingEventId > 0 and .data.result.operationId > 0 and .data.result.metricsRefreshPending == false' "$WORK_DIR/task-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code, ':', DATE_FORMAT(expires_at, '%Y-%m-%d')) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991 AND deleted_at IS NULL")" = "2:growth:2037-06-01"
test "$(mysql_scalar "SELECT CONCAT(version, ':', status, ':', package_code, ':', latest_billing_event_id, ':', DATE_FORMAT(current_period_ends_at, '%Y-%m-%d')) FROM mochat_go_saas_subscriptions WHERE tenant_id = 991 AND deleted_at IS NULL")" = "2:active:growth:$TASK_BILLING_ID:2037-06-01"
test "$(mysql_scalar "SELECT CONCAT(status, ':', version, ':', tenant_id, ':', package_code) FROM mochat_go_saas_admin_tasks WHERE id = $TASK_ID")" = "applied:2:991:growth"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE id = $TASK_BILLING_ID AND tenant_id = 991 AND event_type = 'renewal' AND external_order_no = 'RENEW-0087-TASK' AND amount_cents = 880000")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_subscription_events WHERE tenant_id = 991 AND event_type = 'renewed' AND idempotency_key = 'billing:$TASK_BILLING_ID'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $TASK_OPERATION_ID AND action = 'tenant.renewal' AND target_type = 'tenant' AND target_id = '991'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.task.apply' AND target_type = 'admin_task' AND target_id = '$TASK_ID' AND tenant_id = 991")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $TASK_APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_operation_id = $TASK_OPERATION_ID AND effect_applied_at IS NOT NULL")" = "1"

STALE_TASK_BODY='{"tenantId":991,"packageCode":"growth","expiresAt":"2037-09-01","amountCents":990000,"currency":"CNY","externalOrderNo":"RENEW-0087-STALE-TASK","remark":"0087陈旧任务续费"}'
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/tenantRenewalTask" "$STALE_TASK_BODY" "$WORK_DIR/stale-task-create.json"
STALE_TASK_ID="$(json_value "$WORK_DIR/stale-task-create.json" data.task.id)"
STALE_TASK_VERSION="$(json_value "$WORK_DIR/stale-task-create.json" data.task.version)"
STALE_TASK_APPROVAL_PAYLOAD="$(jq -cn --argjson taskId "$STALE_TASK_ID" --argjson expectedTaskVersion "$STALE_TASK_VERSION" '{taskId:$taskId,expectedTaskVersion:$expectedTaskVersion}')"
request_renewal_approval "$STALE_TASK_APPROVAL_PAYLOAD" "tenant-renewal-task-stale-0087" "$WORK_DIR/stale-task-request.json"
STALE_TASK_APPROVAL_ID="$(json_value "$WORK_DIR/stale-task-request.json" data.approval.id)"
STALE_TASK_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-task-request.json" data.approval.version)"
approve_twice "$STALE_TASK_APPROVAL_ID" "$STALE_TASK_APPROVAL_VERSION" "stale-task"
STALE_TASK_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-task-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/taskCancel" \
  "{\"taskId\":$STALE_TASK_ID,\"remark\":\"审批等待期取消续费任务\"}" "$WORK_DIR/stale-task-cancel.json"
jq -e '.data.task.status == "canceled" and .data.task.version == 2' "$WORK_DIR/stale-task-cancel.json" >/dev/null
TASK_STALE_BILLING_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$STALE_TASK_APPROVAL_ID,\"expectedVersion\":$STALE_TASK_APPROVAL_VERSION}" "$WORK_DIR/stale-task-execute.json" 409
jq -e '.code == 409 and (.msg | contains("任务版本已变化"))' "$WORK_DIR/stale-task-execute.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991")" = "$TASK_STALE_BILLING_COUNT"
test "$(mysql_scalar "SELECT CONCAT(version, ':', DATE_FORMAT(expires_at, '%Y-%m-%d')) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "2:2037-06-01"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $STALE_TASK_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND last_error LIKE '%任务版本已变化%'")" = "1"

STALE_DIRECT_BODY='{"tenantId":991,"packageCode":"growth","expiresAt":"2037-12-01","amountCents":1080000,"currency":"CNY","externalOrderNo":"RENEW-0087-STALE-SUBSCRIPTION","remark":"0087订阅漂移续费"}'
request_renewal_approval "$STALE_DIRECT_BODY" "tenant-renewal-direct-stale-subscription-0087" "$WORK_DIR/stale-direct-request.json"
STALE_DIRECT_APPROVAL_ID="$(json_value "$WORK_DIR/stale-direct-request.json" data.approval.id)"
STALE_DIRECT_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-direct-request.json" data.approval.version)"
jq -e '.data.approval.request.source == "direct" and .data.approval.request.currentPackage.version == 2 and .data.approval.request.subscription.version == 2 and .data.approval.request.targetPackage.version == 1' "$WORK_DIR/stale-direct-request.json" >/dev/null
approve_twice "$STALE_DIRECT_APPROVAL_ID" "$STALE_DIRECT_APPROVAL_VERSION" "stale-direct"
STALE_DIRECT_APPROVAL_VERSION="$(json_value "$WORK_DIR/stale-direct-vote-two.json" data.approval.version)"
mysql_scalar "UPDATE mochat_go_saas_subscriptions SET version = version + 1, updated_at = NOW() WHERE tenant_id = 991 AND deleted_at IS NULL" >/dev/null
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_subscriptions WHERE tenant_id = 991")" = "3"
DIRECT_STALE_BILLING_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991")"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$STALE_DIRECT_APPROVAL_ID,\"expectedVersion\":$STALE_DIRECT_APPROVAL_VERSION}" "$WORK_DIR/stale-direct-execute.json" 409
jq -e '.code == 409 and (.msg | contains("租户订阅版本已变化"))' "$WORK_DIR/stale-direct-execute.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991")" = "$DIRECT_STALE_BILLING_COUNT"
test "$(mysql_scalar "SELECT CONCAT(version, ':', DATE_FORMAT(expires_at, '%Y-%m-%d')) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "2:2037-06-01"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $STALE_DIRECT_APPROVAL_ID AND status = 'approved' AND effect_operation_id = 0 AND last_error LIKE '%租户订阅版本已变化%'")" = "1"

DIRECT_BODY='{"tenantId":991,"packageCode":"growth","expiresAt":"2038-01-01","amountCents":1180000,"currency":"CNY","paidAt":"2026-07-16","paymentMethod":"bank","externalOrderNo":"RENEW-0087-DIRECT","remark":"0087直接审批续费"}'
request_renewal_approval "$DIRECT_BODY" "tenant-renewal-direct-0087" "$WORK_DIR/direct-request.json"
DIRECT_APPROVAL_ID="$(json_value "$WORK_DIR/direct-request.json" data.approval.id)"
DIRECT_APPROVAL_VERSION="$(json_value "$WORK_DIR/direct-request.json" data.approval.version)"
jq -e '.data.approval.request.source == "direct" and .data.approval.request.currentPackage.version == 2 and .data.approval.request.subscription.version == 3 and .data.approval.request.renewal.expiresAt == "2038-01-01 00:00:00"' "$WORK_DIR/direct-request.json" >/dev/null
request_renewal_approval "$DIRECT_BODY" "tenant-renewal-direct-0087" "$WORK_DIR/direct-idempotent.json"
jq -e --argjson approvalId "$DIRECT_APPROVAL_ID" '.data.idempotent == true and .data.approval.id == $approvalId' "$WORK_DIR/direct-idempotent.json" >/dev/null
approve_twice "$DIRECT_APPROVAL_ID" "$DIRECT_APPROVAL_VERSION" "direct"
DIRECT_APPROVAL_VERSION="$(json_value "$WORK_DIR/direct-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$DIRECT_APPROVAL_ID,\"expectedVersion\":$DIRECT_APPROVAL_VERSION}" "$WORK_DIR/direct-execute.json"
DIRECT_OPERATION_ID="$(json_value "$WORK_DIR/direct-execute.json" data.result.operationId)"
DIRECT_BILLING_ID="$(json_value "$WORK_DIR/direct-execute.json" data.result.billingEventId)"
jq -e '.data.approval.status == "executed" and .data.approval.effectOperationId == .data.result.operationId and .data.result.source == "direct" and .data.result.tenantId == 991 and .data.result.expiresAt == "2038-01-01 00:00:00" and .data.result.amountCents == 1180000 and .data.result.operationId > 0 and .data.result.billingEventId > 0' "$WORK_DIR/direct-execute.json" >/dev/null
test "$(mysql_scalar "SELECT CONCAT(version, ':', package_code, ':', DATE_FORMAT(expires_at, '%Y-%m-%d')) FROM mochat_go_saas_tenant_packages WHERE tenant_id = 991")" = "3:growth:2038-01-01"
test "$(mysql_scalar "SELECT CONCAT(version, ':', status, ':', latest_billing_event_id, ':', DATE_FORMAT(current_period_ends_at, '%Y-%m-%d')) FROM mochat_go_saas_subscriptions WHERE tenant_id = 991")" = "4:active:$DIRECT_BILLING_ID:2038-01-01"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_billing_events WHERE tenant_id = 991 AND event_type = 'renewal'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $DIRECT_OPERATION_ID AND action = 'tenant.renewal' AND target_id = '991'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'tenant.renewal' AND status = 'executed' AND effect_operation_id > 0")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE action_type = 'tenant.renewal' AND status = 'approved' AND effect_operation_id = 0 AND last_error <> ''")" = "2"

echo "SaaS tenant renewal approval smoke passed"
