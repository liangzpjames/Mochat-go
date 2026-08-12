#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-admin-dashboard-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13357}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26397}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18127}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-admin-dashboard.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-admin-dashboard-secret}"
PLATFORM_TENANT_ID="${MOCHAT_SAAS_PLATFORM_ADMIN_TENANT_ID:-1}"
TENANT_ID="${MOCHAT_SAAS_ADMIN_TENANT_ID:-902}"
REPORT_DATE="$(date +%F)"
read -r PACKAGE_EXPIRES_ON DIRECT_RENEWAL_EXPIRES_ON TASK_RENEWAL_EXPIRES_ON PAID_ON FORECAST_FOLLOW_UP_ON <<<"$(python3 - <<'PY'
from datetime import date, timedelta

today = date.today()
print(
    (today + timedelta(days=90)).isoformat(),
    (today + timedelta(days=180)).isoformat(),
    (today + timedelta(days=270)).isoformat(),
    (today - timedelta(days=1)).isoformat(),
    (today + timedelta(days=12)).isoformat(),
)
PY
)"
PACKAGE_EXPIRES_AT="$PACKAGE_EXPIRES_ON 00:00:00"
DIRECT_RENEWAL_EXPIRES_AT="$DIRECT_RENEWAL_EXPIRES_ON 00:00:00"
TASK_RENEWAL_EXPIRES_AT="$TASK_RENEWAL_EXPIRES_ON 00:00:00"
FORECAST_FOLLOW_UP_AT="$FORECAST_FOLLOW_UP_ON 00:00:00"
export PACKAGE_EXPIRES_AT DIRECT_RENEWAL_EXPIRES_AT TASK_RENEWAL_EXPIRES_AT FORECAST_FOLLOW_UP_AT
OPERATION_QUEUE_ASSIGN_NEXT_AT="$(python3 - <<'PY'
from datetime import datetime, timedelta
print((datetime.now() + timedelta(days=15)).strftime("%Y-%m-%d 11:00:00"))
PY
)"
OPERATION_QUEUE_ASSIGN_OVERDUE_AT="$(python3 - <<'PY'
from datetime import datetime, timedelta
print((datetime.now() - timedelta(days=1)).strftime("%Y-%m-%d 09:00:00"))
PY
)"

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
    if [ "$status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  echo "$service did not become healthy" >&2
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_admin -e "$query" | tr -d '\r'
}

mysql_exec() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass mochat_saas_admin -e "$query" >/dev/null
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_admin;
CREATE DATABASE mochat_saas_admin CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_admin.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_admin?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0003_saas_provisioning\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0006_saas_alerts\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0008_saas_alert_notifications\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0023_saas_alert_settings\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0033_saas_admin_operation_logs\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0034_saas_billing_events\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0035_saas_admin_tasks\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0036_saas_notification_policy_controls\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0037_saas_notification_health_index\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0038_saas_notification_slo_index\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0039_saas_subscription_lifecycle\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0040_saas_payment_collection\tapplied_now' "$WORK_DIR/migrate.out"

cat >"$WORK_DIR/tenants.csv" <<CSV
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
$PLATFORM_TENANT_ID,SaaS平台租户,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
$TENANT_ID,SaaS普通租户,13800000902,secret902,租户超级管理员,SaaS租户超级管理员,growth,增长版,2,10,1000,50,3,12,7,9,11,13,15,17,19,21,23,25,27,29,512,30,20,8,6,4,2,1000,2037-01-01,missing
CSV

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

test "$(grep -c $'^tenant_id\t' "$WORK_DIR/bootstrap.out")" = "2"
grep -q $'tenant_id\t'"$PLATFORM_TENANT_ID" "$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_ID" "$WORK_DIR/bootstrap.out"

PLATFORM_USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE tenant_id = $PLATFORM_TENANT_ID AND phone = '13800000001' AND isSuperAdmin = 1 AND deleted_at IS NULL")"
TENANT_USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE tenant_id = $TENANT_ID AND phone = '13800000902' AND isSuperAdmin = 1 AND deleted_at IS NULL")"
test -n "$PLATFORM_USER_ID"
test -n "$TENANT_USER_ID"

mysql_root mochat_saas_admin <<SQL
UPDATE mochat_go_saas_tenant_packages
SET expires_at = DATE_ADD(NOW(), INTERVAL 5 DAY)
WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL;

UPDATE mochat_go_saas_usage_counters
SET used_value = 8, limit_value = 10, updated_by = 'smoke'
WHERE tenant_id = $TENANT_ID AND metric = 'users' AND period_key = 'lifetime' AND deleted_at IS NULL;

INSERT INTO mochat_go_saas_alerts
  (alert_key, tenant_id, alert_type, severity, status, metric, period_key, current_value, limit_value, additional_value, occurrence_count, source, message, context_json, first_seen_at, last_seen_at, resolved_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-saas-admin-open-alert', $TENANT_ID, 'quota_exceeded', 'warning', 'open', 'users', 'lifetime', 8, 10, 0, 1, 'smoke.saas-admin', 'SaaS总后台 smoke 告警', JSON_OBJECT('smoke', true), NOW(), NOW(), NULL, NOW(), NOW(), NULL),
  ('smoke-platform-only-open-alert', $PLATFORM_TENANT_ID, 'quota_exceeded', 'critical', 'open', 'users', 'lifetime', 99, 100, 0, 1, 'smoke.business-tenant-scope', '平台控制租户专用告警', JSON_OBJECT('controlPlaneOnly', true), NOW(), NOW(), NULL, NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-saas-admin-pending-notification', 'smoke-saas-admin-open-alert', $TENANT_ID, 'webhook', 'pending', 0, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $TENANT_ID, 'Metric', 'users', 'Current', 8, 'Limit', 10), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', 'SaaS总后台 smoke 告警'), '', DATE_SUB(NOW(), INTERVAL 30 MINUTE), NULL, DATE_SUB(NOW(), INTERVAL 30 MINUTE), NOW(), NULL),
  ('smoke-saas-admin-dead-notification', 'smoke-saas-admin-open-alert', $TENANT_ID, 'webhook', 'dead', 3, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $TENANT_ID, 'Metric', 'users', 'Current', 8, 'Limit', 10), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', 'SaaS总后台 smoke 告警'), 'smoke webhook failed', NULL, NULL, NOW(), NOW(), NULL),
  ('smoke-saas-admin-suppressed-notification', 'smoke-saas-admin-open-alert', $TENANT_ID, 'webhook', 'suppressed', 0, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $TENANT_ID, 'Metric', 'users', 'Current', 8, 'Limit', 10), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', 'SaaS总后台 smoke 告警'), 'policy suppressed: alert type is not subscribed', NULL, NULL, NOW(), NOW(), NULL),
  ('smoke-saas-admin-delivered-notification', 'smoke-saas-admin-open-alert', $TENANT_ID, 'webhook', 'delivered', 1, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $TENANT_ID, 'Metric', 'users', 'Current', 8, 'Limit', 10), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', 'SaaS总后台 smoke 告警'), '', NULL, NOW(), DATE_SUB(NOW(), INTERVAL 5 SECOND), NOW(), NULL),
  ('smoke-saas-admin-bulk-failed-notification-1', 'smoke-saas-admin-open-alert', $TENANT_ID, 'webhook', 'failed', 1, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $TENANT_ID, 'Metric', 'users', 'Current', 8, 'Limit', 10), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', 'SaaS总后台 smoke 告警'), 'bulk webhook failed 1', NOW(), NULL, NOW(), NOW(), NULL),
  ('smoke-saas-admin-bulk-failed-notification-2', 'smoke-saas-admin-open-alert', $TENANT_ID, 'webhook', 'failed', 2, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $TENANT_ID, 'Metric', 'users', 'Current', 8, 'Limit', 10), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', 'SaaS总后台 smoke 告警'), 'bulk webhook failed 2', NOW(), NULL, NOW(), NOW(), NULL),
  ('smoke-platform-only-pending-notification', 'smoke-platform-only-open-alert', $PLATFORM_TENANT_ID, 'webhook', 'failed', 1, 3, JSON_OBJECT('Status', JSON_OBJECT('TenantID', $PLATFORM_TENANT_ID, 'Metric', 'users', 'Current', 99, 'Limit', 100), 'AlertType', 'quota_exceeded', 'PeriodKey', 'lifetime', 'Message', '平台控制租户专用告警'), 'platform-only notification', NOW(), NULL, NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_billing_events
  (tenant_id, event_type, package_code, package_name, previous_expires_at, new_expires_at, amount_cents, currency, paid_at, payment_method, external_order_no, actor_user_id, actor_tenant_id, remark, metadata_json, created_at, updated_at, deleted_at)
VALUES
  ($PLATFORM_TENANT_ID, 'renewal', 'platform', '平台版', NOW(), DATE_ADD(NOW(), INTERVAL 1 YEAR), 99999999, 'CNY', NOW(), 'internal', 'SMOKE-PLATFORM-ONLY-BILLING', $PLATFORM_USER_ID, $PLATFORM_TENANT_ID, 'control-plane-only', JSON_OBJECT('controlPlaneOnly', true), NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_admin_tasks
  (task_type, status, tenant_id, package_code, actor_user_id, actor_tenant_id, request_json, preview_json, result_json, remark, last_error, applied_at, created_at, updated_at, deleted_at)
VALUES
  ('tenant_renewal', 'blocked', $PLATFORM_TENANT_ID, 'platform', $PLATFORM_USER_ID, $PLATFORM_TENANT_ID, JSON_OBJECT('controlPlaneOnly', true), NULL, NULL, 'control-plane-only', 'platform-only task', NULL, DATE_SUB(NOW(), INTERVAL 7 DAY), NOW(), NULL);

INSERT INTO mochat_go_saas_admin_operation_logs
  (tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name, before_json, after_json, remark, created_at, updated_at, deleted_at)
VALUES
  ($PLATFORM_TENANT_ID, $PLATFORM_USER_ID, $PLATFORM_TENANT_ID, 'tenant.risk.follow_up', 'tenant', CAST($PLATFORM_TENANT_ID AS CHAR), 'SaaS平台租户', NULL, JSON_OBJECT('status', 'contacted', 'owner', 'platform-only', 'nextFollowUpAt', '2000-01-01 00:00:00'), 'control-plane-only', NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID="$PLATFORM_TENANT_ID" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200

login_token() {
  local phone="$1"
  local password="$2"
  local output="$3"
  curl -sS -f \
    -H "Content-Type: application/json" \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$output"
  python3 - "$output" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
token = payload["data"]["token"]
assert token, payload
print(token)
PY
}

PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
TENANT_TOKEN="$(login_token 13800000902 secret902 "$WORK_DIR/tenant-auth.json")"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q "MoChat Go SaaS 总后台" "$WORK_DIR/page.html"
grep -q 'id="workspaceNav"' "$WORK_DIR/page.html"
grep -q 'aria-label="工作区切换"' "$WORK_DIR/page.html"
grep -q 'id="workspaceSectionJump"' "$WORK_DIR/page.html"
grep -q 'aria-label="租户风险与额度"' "$WORK_DIR/page.html"
grep -q 'function initWorkspaceNavigation()' "$WORK_DIR/page.html"
grep -q 'function revealCurrentWorkspaceTab(smooth)' "$WORK_DIR/page.html"
grep -q 'applyWorkspaceView(preferredWorkspace, false, false)' "$WORK_DIR/page.html"
grep -q "租户列表筛选" "$WORK_DIR/page.html"
grep -q "套餐维护" "$WORK_DIR/page.html"
grep -q "套餐变更影响" "$WORK_DIR/page.html"
grep -q "运营任务中心" "$WORK_DIR/page.html"
grep -q "任务负责人" "$WORK_DIR/page.html"
grep -q "任务SLA" "$WORK_DIR/page.html"
grep -q "日报任务SLA" "$WORK_DIR/page.html"
grep -q "adminTaskType" "$WORK_DIR/page.html"
grep -q "adminTaskOwners" "$WORK_DIR/page.html"
grep -q "taskOwners" "$WORK_DIR/page.html"
grep -q "adminTaskSla" "$WORK_DIR/page.html"
grep -q "taskSla" "$WORK_DIR/page.html"
grep -q "createTaskSlaNotifications" "$WORK_DIR/page.html"
grep -q "applyAdminTask" "$WORK_DIR/page.html"
grep -q "traceAdminTaskOperations" "$WORK_DIR/page.html"
grep -q "trace-admin-task" "$WORK_DIR/page.html"
grep -q "operationChangeDetails" "$WORK_DIR/page.html"
grep -q "查看变更" "$WORK_DIR/page.html"
grep -q "taskCancel" "$WORK_DIR/page.html"
grep -q "taskBulkCancel" "$WORK_DIR/page.html"
grep -q "taskBulkReset" "$WORK_DIR/page.html"
grep -q "taskReset" "$WORK_DIR/page.html"
grep -q "bulkResetAdminTasks" "$WORK_DIR/page.html"
grep -q "bulkApplyRenewalTasks" "$WORK_DIR/page.html"
grep -q "bulkApplyProvisionTasks" "$WORK_DIR/page.html"
grep -q "createCustomerSuccessRenewalNotifications" "$WORK_DIR/page.html"
grep -q "续费预测提醒" "$WORK_DIR/page.html"
grep -q "createRenewalForecastNotifications" "$WORK_DIR/page.html"
grep -q "reset-admin-task" "$WORK_DIR/page.html"
grep -q "exportTasks" "$WORK_DIR/page.html"
grep -q "exportTaskSla" "$WORK_DIR/page.html"
grep -q "套餐快照同步" "$WORK_DIR/page.html"
grep -q "套餐同步任务" "$WORK_DIR/page.html"
grep -q "createPackageSyncTask" "$WORK_DIR/page.html"
grep -q "packageSyncTaskApply" "$WORK_DIR/page.html"
grep -q "bulkApplyPackageSyncTasks" "$WORK_DIR/page.html"
grep -q "packageSyncTaskBulkApply" "$WORK_DIR/page.html"
grep -q "续费任务" "$WORK_DIR/page.html"
grep -q "tenantRenewalTask" "$WORK_DIR/page.html"
grep -q "tenantRenewalTaskApply" "$WORK_DIR/page.html"
grep -q "tenantRenewalTaskBulkApply" "$WORK_DIR/page.html"
grep -q "新租户开户" "$WORK_DIR/page.html"
grep -q "平台开户任务" "$WORK_DIR/page.html"
grep -q "tenantProvisionTask" "$WORK_DIR/page.html"
grep -q "tenantProvisionTaskApply" "$WORK_DIR/page.html"
grep -q "tenantProvisionTaskBulkApply" "$WORK_DIR/page.html"
grep -q "租户状态" "$WORK_DIR/page.html"
grep -q "租户详情" "$WORK_DIR/page.html"
grep -q "生命周期审计" "$WORK_DIR/page.html"
grep -q "tenantLifecycle" "$WORK_DIR/page.html"
grep -q "生命周期筛选" "$WORK_DIR/page.html"
grep -q "tenantLifecycleSource" "$WORK_DIR/page.html"
grep -q "tenantLifecycleEventType" "$WORK_DIR/page.html"
grep -q "tenantLifecycleKeyword" "$WORK_DIR/page.html"
grep -q "导出审计 CSV" "$WORK_DIR/page.html"
grep -q "exportTenantLifecycle" "$WORK_DIR/page.html"
grep -q "用量明细" "$WORK_DIR/page.html"
grep -q "经营指标" "$WORK_DIR/page.html"
grep -q "businessMetrics" "$WORK_DIR/page.html"
grep -q "businessPackages" "$WORK_DIR/page.html"
grep -q "经营趋势" "$WORK_DIR/page.html"
grep -q "businessTrends" "$WORK_DIR/page.html"
grep -q "businessRenewalFunnel" "$WORK_DIR/page.html"
grep -q "导出经营指标 CSV" "$WORK_DIR/page.html"
grep -q "导出经营趋势 CSV" "$WORK_DIR/page.html"
grep -q "exportBusinessMetrics" "$WORK_DIR/page.html"
grep -q "exportBusinessTrends" "$WORK_DIR/page.html"
grep -q "运营待办队列" "$WORK_DIR/page.html"
grep -q "operationQueue" "$WORK_DIR/page.html"
grep -q "导出待办 CSV" "$WORK_DIR/page.html"
grep -q "运营待办批量分派" "$WORK_DIR/page.html"
grep -q "assignOperationQueue" "$WORK_DIR/page.html"
grep -q "分派当前待办" "$WORK_DIR/page.html"
grep -q "生成认领到期提醒" "$WORK_DIR/page.html"
grep -q "createOperationQueueAssignmentNotifications" "$WORK_DIR/page.html"
grep -q "认领到期" "$WORK_DIR/page.html"
grep -q "认领视图" "$WORK_DIR/page.html"
grep -q "operationQueueAssignmentCurrentOnly" "$WORK_DIR/page.html"
grep -q "closeOperationQueueAssignment" "$WORK_DIR/page.html"
grep -q "运营待办负责人工作台" "$WORK_DIR/page.html"
grep -q "operationQueueOwners" "$WORK_DIR/page.html"
grep -q "导出待办负责人 CSV" "$WORK_DIR/page.html"
grep -q "运营待办认领记录" "$WORK_DIR/page.html"
grep -q "operationQueueAssignments" "$WORK_DIR/page.html"
grep -q "导出待办认领 CSV" "$WORK_DIR/page.html"
grep -q "续费预测" "$WORK_DIR/page.html"
grep -q "renewalForecast" "$WORK_DIR/page.html"
grep -q "renewalForecastBuckets" "$WORK_DIR/page.html"
grep -q "renewalForecastOwners" "$WORK_DIR/page.html"
grep -q "续费预测负责人工作台" "$WORK_DIR/page.html"
grep -q "createRenewalForecastTasks" "$WORK_DIR/page.html"
grep -q "导出续费预测 CSV" "$WORK_DIR/page.html"
grep -q "导出续费预测负责人 CSV" "$WORK_DIR/page.html"
grep -q "续费预测筛选" "$WORK_DIR/page.html"
grep -q "renewalForecastTaskStatus" "$WORK_DIR/page.html"
grep -q "续费预测分派" "$WORK_DIR/page.html"
grep -q "assignRenewalForecast" "$WORK_DIR/page.html"
grep -q "运营日报" "$WORK_DIR/page.html"
grep -q "日报日期" "$WORK_DIR/page.html"
grep -q "统计天数" "$WORK_DIR/page.html"
grep -q "刷新日报" "$WORK_DIR/page.html"
grep -q "运营日报明细" "$WORK_DIR/page.html"
grep -q "日报负责人" "$WORK_DIR/page.html"
grep -q "日报通知" "$WORK_DIR/page.html"
grep -q "日报待办认领" "$WORK_DIR/page.html"
grep -q "日报操作动作" "$WORK_DIR/page.html"
grep -q "日报账单流水" "$WORK_DIR/page.html"
grep -q "客户成功队列" "$WORK_DIR/page.html"
grep -q "客户成功队列筛选" "$WORK_DIR/page.html"
grep -q "客户成功负责人工作台" "$WORK_DIR/page.html"
grep -q "客户成功批量分派" "$WORK_DIR/page.html"
grep -q "客户成功续费任务" "$WORK_DIR/page.html"
grep -q "customerSuccessPriority" "$WORK_DIR/page.html"
grep -q "customerSuccessOwners" "$WORK_DIR/page.html"
grep -q "assignCustomerSuccess" "$WORK_DIR/page.html"
grep -q "createCustomerSuccessRenewalTasks" "$WORK_DIR/page.html"
grep -q "exportCustomerSuccess" "$WORK_DIR/page.html"
grep -q "exportCustomerSuccessOwners" "$WORK_DIR/page.html"
grep -q "customerSuccess" "$WORK_DIR/page.html"
grep -q "风险看板" "$WORK_DIR/page.html"
grep -q "风险跟进" "$WORK_DIR/page.html"
grep -q "风险跟进任务" "$WORK_DIR/page.html"
grep -q "负责人工作台" "$WORK_DIR/page.html"
grep -q "批量关闭任务" "$WORK_DIR/page.html"
grep -q "告警处置" "$WORK_DIR/page.html"
grep -q "批量解决" "$WORK_DIR/page.html"
grep -q "通知重试" "$WORK_DIR/page.html"
grep -q "通知送达健康度" "$WORK_DIR/page.html"
grep -q "notificationHealthWindowHours" "$WORK_DIR/page.html"
grep -q "notificationHealthState" "$WORK_DIR/page.html"
grep -q "notificationHealthStaleMinutes" "$WORK_DIR/page.html"
grep -q "notificationHealthAssignOwner" "$WORK_DIR/page.html"
grep -q "notificationHealthAssignNextAt" "$WORK_DIR/page.html"
grep -q "notificationFailureReasons" "$WORK_DIR/page.html"
grep -q "exportNotificationHealth" "$WORK_DIR/page.html"
grep -q "notification_health" "$WORK_DIR/page.html"
grep -q "assignNotificationHealthQueue" "$WORK_DIR/page.html"
grep -q "recoverNotificationHealthQueue" "$WORK_DIR/page.html"
grep -q "notificationHealth" "$WORK_DIR/page.html"
grep -q "租户通知策略" "$WORK_DIR/page.html"
grep -q "notificationPolicyState" "$WORK_DIR/page.html"
grep -q "notificationPolicyWebhookUrl" "$WORK_DIR/page.html"
grep -q "notificationPolicyMinimumSeverity" "$WORK_DIR/page.html"
grep -q "notificationPolicyAlertQuota" "$WORK_DIR/page.html"
grep -q "notificationPolicyQuietEnabled" "$WORK_DIR/page.html"
grep -q "notificationPolicyTimezone" "$WORK_DIR/page.html"
grep -q "notificationPolicyHourlyLimit" "$WORK_DIR/page.html"
grep -q "saveNotificationPolicy" "$WORK_DIR/page.html"
grep -q "testNotificationPolicy" "$WORK_DIR/page.html"
grep -q "notificationPolicyTest" "$WORK_DIR/page.html"
grep -q "生成测试通知" "$WORK_DIR/page.html"
grep -q "批量重试" "$WORK_DIR/page.html"
grep -q "bulkCloseNotifications" "$WORK_DIR/page.html"
grep -q "已关闭" "$WORK_DIR/page.html"
grep -q "账单事件" "$WORK_DIR/page.html"
grep -q "账单对账" "$WORK_DIR/page.html"
grep -q "只看对账异常" "$WORK_DIR/page.html"
grep -q "followUpBillingReconciliation" "$WORK_DIR/page.html"
grep -q "billingReconciliationFollowUp" "$WORK_DIR/page.html"
grep -q "账单跟进任务" "$WORK_DIR/page.html"
grep -q "账单跟进负责人工作台" "$WORK_DIR/page.html"
grep -q "billingReconciliationFollowUps" "$WORK_DIR/page.html"
grep -q "billingReconciliationFollowUpOwners" "$WORK_DIR/page.html"
grep -q "applyBillingFollowFilters" "$WORK_DIR/page.html"
grep -q "批量关闭账单跟进" "$WORK_DIR/page.html"
grep -q "bulkCloseBillingFollowUps" "$WORK_DIR/page.html"
grep -q "billingReconciliationFollowUpBulkClose" "$WORK_DIR/page.html"
grep -q "操作记录" "$WORK_DIR/page.html"
grep -q "操作与账单筛选" "$WORK_DIR/page.html"
grep -q "数据导出" "$WORK_DIR/page.html"
grep -q "导出用量 CSV" "$WORK_DIR/page.html"
grep -q "导出风险 CSV" "$WORK_DIR/page.html"
grep -q "导出跟进 CSV" "$WORK_DIR/page.html"
grep -q "导出风险负责人 CSV" "$WORK_DIR/page.html"
grep -q "导出日报 CSV" "$WORK_DIR/page.html"
grep -q "导出套餐 CSV" "$WORK_DIR/page.html"
grep -q "导出对账 CSV" "$WORK_DIR/page.html"
grep -q "导出账单跟进 CSV" "$WORK_DIR/page.html"
grep -q "导出账单负责人 CSV" "$WORK_DIR/page.html"
grep -q "租户上线准备度" "$WORK_DIR/page.html"
grep -q "tenantReadinessFilterState" "$WORK_DIR/page.html"
grep -q "tenantReadinessTenants" "$WORK_DIR/page.html"
grep -q "/dashboard/saasAdmin/tenantReadiness" "$WORK_DIR/page.html"
grep -q "tenantPopulation === 'business_tenants' ? '业务租户' : '租户数'" "$WORK_DIR/page.html"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=platform&expiringDays=10" >"$WORK_DIR/platform.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=platform&expiringDays=10&limit=1" >"$WORK_DIR/platform-limit-one.json"

jq -e --argjson tenant_id "$TENANT_ID" --argjson platform_tenant_id "$PLATFORM_TENANT_ID" '
  .code == 200 and
  .data.scope == "platform" and
  .data.tenantPopulation == "business_tenants" and
  .data.summary.tenantCount == 1 and
  (.data.tenants | length) == 1 and
  .data.tenants[0].tenantId == $tenant_id and
  ([.data.tenants[].tenantId] | index($platform_tenant_id)) == null
' "$WORK_DIR/platform-limit-one.json" >/dev/null

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=tenant&tenantId=$PLATFORM_TENANT_ID&expiringDays=10&limit=1" >"$WORK_DIR/platform-control-tenant.json"

jq -e --argjson platform_tenant_id "$PLATFORM_TENANT_ID" '
  .code == 200 and
  .data.scope == "tenant" and
  .data.tenantPopulation == "selected_tenant" and
  .data.summary.tenantCount == 1 and
  (.data.tenants | length) == 1 and
  .data.tenants[0].tenantId == $platform_tenant_id
' "$WORK_DIR/platform-control-tenant.json" >/dev/null

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantReadiness?tenantId=$TENANT_ID&state=all&limit=100" >"$WORK_DIR/tenant-readiness.json"

jq -e --argjson tenant_id "$TENANT_ID" --argjson platform_tenant_id "$PLATFORM_TENANT_ID" '
  .code == 200 and
  .data.scope == "business_tenants" and
  .data.platformAdminTenantId == $platform_tenant_id and
  .data.summary.tenantCount == 1 and
  .data.summary.filteredCount == 1 and
  (.data.truncated == false) and
  (.data.tenants | length) == 1 and
  .data.tenants[0].tenantId == $tenant_id and
  .data.tenants[0].checkCount == 11 and
  .data.tenants[0].requiredCheckCount == 8 and
  ([.data.tenants[0].checks[].code] | sort) == (["baseline_seed","branding","identity_policy","notification_policy","package","primary_domain","subscription","super_admin","tenant_active","wecom_corp","wecom_credentials"] | sort) and
  (.data.tenants[0].state == "ready" or .data.tenants[0].state == "attention" or .data.tenants[0].state == "blocked")
' "$WORK_DIR/tenant-readiness.json" >/dev/null

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantReadiness?state=all&limit=100" >"$WORK_DIR/tenant-readiness-all.json"
jq -e --argjson tenant_id "$TENANT_ID" --argjson platform_tenant_id "$PLATFORM_TENANT_ID" '
  .code == 200 and .data.scope == "business_tenants" and
  .data.platformAdminTenantId == $platform_tenant_id and
  .data.summary.tenantCount == 1 and .data.summary.filteredCount == 1 and
  (.data.tenants | length) == 1 and .data.tenants[0].tenantId == $tenant_id and
  ([.data.tenants[].tenantId] | index($platform_tenant_id)) == null
' "$WORK_DIR/tenant-readiness-all.json" >/dev/null

tenant_readiness_platform_code="$(curl -sS -o "$WORK_DIR/tenant-readiness-platform.json" -w '%{http_code}' \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantReadiness?tenantId=$PLATFORM_TENANT_ID")"
test "$tenant_readiness_platform_code" = "400"
jq -e '.code == 400 and .msg == "平台管理租户不参与上线准备度"' "$WORK_DIR/tenant-readiness-platform.json" >/dev/null

READINESS_STATE="$(jq -r '.data.tenants[0].state' "$WORK_DIR/tenant-readiness.json")"
curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantReadiness?tenantId=$TENANT_ID&state=$READINESS_STATE&limit=1" >"$WORK_DIR/tenant-readiness-filtered.json"
jq -e --arg state "$READINESS_STATE" --argjson tenant_id "$TENANT_ID" '
  .code == 200 and .data.filters.state == $state and .data.summary.filteredCount == 1 and
  (.data.tenants | length) == 1 and .data.tenants[0].tenantId == $tenant_id and .data.tenants[0].state == $state
' "$WORK_DIR/tenant-readiness-filtered.json" >/dev/null

tenant_readiness_invalid_code="$(curl -sS -o "$WORK_DIR/tenant-readiness-invalid.json" -w '%{http_code}' \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantReadiness?state=invalid")"
test "$tenant_readiness_invalid_code" = "400"
jq -e '.code == 400 and (.msg | contains("state"))' "$WORK_DIR/tenant-readiness-invalid.json" >/dev/null

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=platform&keyword=SaaS%E6%99%AE%E9%80%9A&tenantStatus=1&packageCode=growth&dueState=expiring&expiringDays=10" >"$WORK_DIR/platform-filtered.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?expiringDays=10" >"$WORK_DIR/tenant.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=tenant&tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/platform-tenant.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenant?tenantId=$TENANT_ID&operationLimit=10&expiringDays=10" >"$WORK_DIR/platform-tenant-detail.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenant?operationLimit=10&expiringDays=10" >"$WORK_DIR/tenant-detail.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/usage?tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/platform-tenant-usage.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/usage?expiringDays=10" >"$WORK_DIR/tenant-usage.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/risk?scope=platform&expiringDays=10&highUsageRatio=0.8&limit=50" >"$WORK_DIR/platform-risk.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/risk?expiringDays=10&highUsageRatio=0.8&limit=50" >"$WORK_DIR/tenant-risk.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccess?tenantLimit=200&limit=50&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/customer-success.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=customerSuccess&tenantLimit=200&limit=1000&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/export-customer-success.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=customerSuccessOwners&tenantLimit=200&limit=1000&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/export-customer-success-owners-initial.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=risk&tenantId=$TENANT_ID&highUsageRatio=0.8&limit=1000" >"$WORK_DIR/export-risk-initial.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/alerts?tenantId=$TENANT_ID&status=open&metric=users&alertType=quota_exceeded&perPage=10" >"$WORK_DIR/platform-alerts.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/alerts?status=open&metric=users&alertType=quota_exceeded&perPage=10" >"$WORK_DIR/tenant-alerts.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$TENANT_ID&status=dead&channel=webhook&keyword=smoke&limit=10" >"$WORK_DIR/platform-notifications.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?status=dead&channel=webhook&keyword=smoke&limit=10" >"$WORK_DIR/tenant-notifications.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$TENANT_ID&status=suppressed&channel=webhook&keyword=smoke&limit=10" >"$WORK_DIR/platform-suppressed-notifications.json"

python3 - "$WORK_DIR/platform-suppressed-notifications.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])
assert payload["code"] == 200, payload
data = payload["data"]
assert data["filters"]["status"] == "suppressed", data
assert data["summary"]["notificationCount"] == 1, data
assert data["summary"]["suppressedCount"] == 1, data
assert data["summary"]["retryableCount"] == 0, data
assert data["returnedCount"] == 1, data
notification = data["notifications"][0]
assert notification["tenantId"] == tenant_id, notification
assert notification["status"] == "suppressed", notification
assert notification["attempts"] == 0, notification
assert "not subscribed" in notification["lastError"], notification
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationHealth?tenantId=$TENANT_ID&state=critical&channel=webhook&windowHours=24&staleMinutes=15&limit=10" >"$WORK_DIR/notification-health.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=notificationHealth&tenantId=$TENANT_ID&state=critical&channel=webhook&windowHours=24&staleMinutes=15&limit=1000" >"$WORK_DIR/export-notification-health.csv"

python3 - "$WORK_DIR/notification-health.json" "$WORK_DIR/export-notification-health.csv" "$TENANT_ID" <<'PY'
import csv
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
csv_path = pathlib.Path(sys.argv[2])
tenant_id = int(sys.argv[3])
assert payload["code"] == 200, payload
data = payload["data"]
filters = data["filters"]
assert filters["tenantId"] == tenant_id, filters
assert filters["state"] == "critical", filters
assert filters["channel"] == "webhook", filters
assert filters["windowHours"] == 24, filters
assert filters["staleMinutes"] == 15, filters
summary = data["summary"]
assert summary["tenantCount"] == 1, summary
assert summary["matchedTenantCount"] == 1, summary
assert summary["criticalTenantCount"] == 1, summary
assert summary["notificationCount"] == 6, summary
assert summary["attemptedCount"] == 4, summary
assert summary["deliveredCount"] == 1, summary
assert summary["pendingCount"] == 1, summary
assert summary["readyPendingCount"] == 1, summary
assert summary["deferredCount"] == 0, summary
assert summary["stalePendingCount"] == 1, summary
assert summary["failedCount"] == 2, summary
assert summary["deadCount"] == 1, summary
assert summary["suppressedCount"] == 1, summary
assert summary["deliverySuccessRate"] == 0.25, summary
assert summary["totalAttempts"] == 7, summary
assert data["returnedCount"] == 1, data
tenant = data["tenants"][0]
assert tenant["tenantId"] == tenant_id, tenant
assert tenant["healthState"] == "critical", tenant
assert tenant["policyConfigured"] is False, tenant
assert tenant["deliverySuccessRate"] == 0.25, tenant
assert tenant["stalePendingCount"] == 1, tenant
assert tenant["deadCount"] == 1, tenant
assert any("已耗尽" in reason for reason in tenant["reasons"]), tenant
assert any("积压超过" in reason for reason in tenant["reasons"]), tenant
assert len(data["failureReasons"]) == 3, data["failureReasons"]
assert {item["reason"] for item in data["failureReasons"]} == {"smoke webhook failed", "bulk webhook failed 1", "bulk webhook failed 2"}, data["failureReasons"]

with csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    rows = list(csv.reader(fh))
assert rows[0][0:13] == ["windowStartAt", "windowEndAt", "windowHours", "staleMinutes", "tenantId", "tenantName", "tenantStatus", "packageCode", "packageName", "healthState", "notificationCount", "attemptedCount", "deliverySuccessRate"], rows[0]
assert len(rows) == 2, rows
assert rows[1][4] == str(tenant_id), rows[1]
assert rows[1][9] == "critical", rows[1]
assert rows[1][10:13] == ["6", "4", "0.2500"], rows[1]
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=alerts&tenantId=$TENANT_ID&status=open&metric=users&alertType=quota_exceeded&limit=1000" >"$WORK_DIR/export-alerts.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=notifications&tenantId=$TENANT_ID&status=dead&channel=webhook&keyword=smoke&limit=1000" >"$WORK_DIR/export-notifications.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicies?state=unconfigured&limit=100" >"$WORK_DIR/notification-policies-unconfigured.json"

curl -sS -f \
  -X PUT \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenantId\":$TENANT_ID,\"channel\":\"webhook\",\"enabled\":true,\"webhookUrl\":\"https://alerts.example.com/mochat-smoke\",\"webhookSecret\":\"smoke-policy-secret\",\"webhookTimeoutSeconds\":8,\"webhookRetryAttempts\":2,\"webhookRetryDelayMs\":125,\"webhookTitleTemplate\":\"租户 {{.TenantID}} 通知\",\"webhookBodyTemplate\":\"{{.Message}} / {{.AlertType}}\",\"notificationMaxAttempts\":5,\"notificationRetryDelaySeconds\":120,\"minimumSeverity\":\"critical\",\"alertTypes\":[\"quota_exceeded\",\"admin_task_sla_reminder\"],\"quietHoursEnabled\":true,\"quietHoursStart\":\"22:30\",\"quietHoursEnd\":\"07:30\",\"timezone\":\"Asia/Shanghai\",\"hourlyLimit\":20,\"remark\":\"smoke 平台代管通知策略\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicy" >"$WORK_DIR/notification-policy-save.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicy?tenantId=$TENANT_ID&channel=webhook" >"$WORK_DIR/notification-policy-show.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicies?tenantId=$TENANT_ID&state=enabled&limit=10" >"$WORK_DIR/notification-policies-enabled.json"

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenantId\":$TENANT_ID,\"channel\":\"webhook\",\"remark\":\"smoke 生成通知策略测试消息\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicyTest" >"$WORK_DIR/notification-policy-test.json"

python3 - "$WORK_DIR/notification-policies-unconfigured.json" "$WORK_DIR/notification-policy-save.json" "$WORK_DIR/notification-policy-show.json" "$WORK_DIR/notification-policies-enabled.json" "$WORK_DIR/notification-policy-test.json" "$PLATFORM_TENANT_ID" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

unconfigured = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
saved = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
shown = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
enabled = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
tested = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
platform_tenant_id = int(sys.argv[6])
tenant_id = int(sys.argv[7])

assert unconfigured["code"] == 200, unconfigured
unconfigured_data = unconfigured["data"]
assert unconfigured_data["platformAdminTenantId"] == platform_tenant_id, unconfigured_data
assert unconfigured_data["summary"]["tenantCount"] >= 2, unconfigured_data
assert unconfigured_data["summary"]["unconfiguredCount"] >= 2, unconfigured_data
assert {item["tenantId"] for item in unconfigured_data["policies"]} >= {platform_tenant_id, tenant_id}, unconfigured_data
security = unconfigured_data["webhookSecurity"]
assert security["requireHttps"] is True, security
assert security["privateNetworksBlocked"] is True, security
assert security["metadataAddressesBlocked"] is True, security
assert security["dnsPinningEnabled"] is True, security
assert security["sameOriginRedirectsOnly"] is True, security
assert security["environmentProxyDisabled"] is True, security
assert security["allowedCidrCount"] == 0, security

for payload in (saved, shown, enabled, tested):
    assert payload["code"] == 200, payload

for payload in (saved, shown):
    policy = payload["data"]["policy"]
    assert policy["tenantId"] == tenant_id, policy
    assert policy["configured"] is True, policy
    setting = policy["setting"]
    assert setting["enabled"] is True, setting
    assert setting["webhookUrl"] == "https://alerts.example.com/mochat-smoke", setting
    assert setting["webhookSecretConfigured"] is True, setting
    assert "webhookSecret" not in setting, setting
    assert setting["webhookRetryAttempts"] == 2, setting
    assert setting["notificationMaxAttempts"] == 5, setting
    assert setting["minimumSeverity"] == "critical", setting
    assert setting["alertTypes"] == ["admin_task_sla_reminder", "quota_exceeded"], setting
    assert setting["quietHoursEnabled"] is True, setting
    assert setting["quietHoursStart"] == "22:30", setting
    assert setting["quietHoursEnd"] == "07:30", setting
    assert setting["timezone"] == "Asia/Shanghai", setting
    assert setting["hourlyLimit"] == 20, setting
    assert setting["urlSecurity"]["configured"] is True, setting
    assert setting["urlSecurity"]["allowed"] is True, setting
    assert setting["urlSecurity"]["error"] == "", setting

enabled_data = enabled["data"]
assert enabled_data["summary"]["enabledCount"] == 1, enabled_data
assert enabled_data["summary"]["matchedCount"] == 1, enabled_data
assert [item["tenantId"] for item in enabled_data["policies"]] == [tenant_id], enabled_data

notification = tested["data"]["notification"]
assert notification["tenantId"] == tenant_id, notification
assert notification["status"] == "pending", notification
assert notification["channel"] == "webhook", notification
assert notification["metric"] == "notification_policy", notification
assert notification["alertType"] == "notification_policy_test", notification
assert notification["maxAttempts"] == 5, notification
PY

unsafe_notification_policy_code="$(curl -sS -o "$WORK_DIR/notification-policy-unsafe.json" -w '%{http_code}' \
  -X PUT \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenantId\":$TENANT_ID,\"channel\":\"webhook\",\"enabled\":true,\"webhookUrl\":\"https://127.0.0.1/internal\",\"remark\":\"smoke SSRF 拒绝\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicy")"
test "$unsafe_notification_policy_code" = "400"
jq -e '.code == 400 and (.msg | contains("出站安全策略"))' "$WORK_DIR/notification-policy-unsafe.json" >/dev/null

if grep -q "smoke-policy-secret" "$WORK_DIR/notification-policy-save.json" "$WORK_DIR/notification-policy-show.json" "$WORK_DIR/notification-policies-enabled.json"; then
  echo "notification policy response leaked webhook secret" >&2
  exit 1
fi

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_settings WHERE tenant_id = $TENANT_ID AND channel = 'webhook' AND enabled = 1 AND webhook_url = 'https://alerts.example.com/mochat-smoke' AND webhook_secret = 'smoke-policy-secret' AND webhook_retry_attempts = 2 AND notification_max_attempts = 5 AND minimum_severity = 'critical' AND allowed_alert_types_json = '[\"admin_task_sla_reminder\",\"quota_exceeded\"]' AND quiet_hours_enabled = 1 AND quiet_hours_start = '22:30' AND quiet_hours_end = '07:30' AND timezone = 'Asia/Shanghai' AND hourly_limit = 20 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE tenant_id = $TENANT_ID AND notification_key LIKE '$TENANT_ID:notification_policy:notification_policy_test:%:webhook' AND status = 'pending' AND max_attempts = 5 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = $TENANT_ID AND action = 'tenant.notification_policy.update' AND target_type = 'notification_policy' AND actor_user_id = $PLATFORM_USER_ID AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id = $TENANT_ID AND action = 'tenant.notification_policy.test' AND target_type = 'alert_notification' AND actor_user_id = $PLATFORM_USER_ID AND deleted_at IS NULL")" = "1"

tenant_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=platform")"
test "$tenant_forbidden_code" = "403"

tenant_detail_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-detail-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenant?tenantId=$PLATFORM_TENANT_ID")"
test "$tenant_detail_forbidden_code" = "403"

tenant_readiness_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-readiness-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantReadiness?tenantId=$TENANT_ID")"
test "$tenant_readiness_forbidden_code" = "403"

tenant_lifecycle_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-lifecycle-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantLifecycle?tenantId=$TENANT_ID")"
test "$tenant_lifecycle_forbidden_code" = "403"

tenant_lifecycle_export_forbidden_code="$(curl -s -o "$WORK_DIR/export-tenant-lifecycle-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=tenantLifecycle&tenantId=$TENANT_ID")"
test "$tenant_lifecycle_export_forbidden_code" = "403"

tenant_usage_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-usage-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/usage?tenantId=$PLATFORM_TENANT_ID")"
test "$tenant_usage_forbidden_code" = "403"

tenant_risk_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-risk-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/risk?tenantId=$PLATFORM_TENANT_ID")"
test "$tenant_risk_forbidden_code" = "403"

tenant_business_metrics_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-business-metrics-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/businessMetrics")"
test "$tenant_business_metrics_forbidden_code" = "403"

tenant_business_trends_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-business-trends-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/businessTrends")"
test "$tenant_business_trends_forbidden_code" = "403"

tenant_operation_queue_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueue")"
test "$tenant_operation_queue_forbidden_code" = "403"

tenant_operation_queue_export_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operationQueue")"
test "$tenant_operation_queue_export_code" = "403"

tenant_operation_queue_owners_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-owners-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueOwners")"
test "$tenant_operation_queue_owners_forbidden_code" = "403"

tenant_operation_queue_owners_export_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-owners-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operationQueueOwners")"
test "$tenant_operation_queue_owners_export_code" = "403"

tenant_operation_queue_assignments_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-assignments-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignments")"
test "$tenant_operation_queue_assignments_forbidden_code" = "403"

tenant_operation_queue_assignments_export_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-assignments-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operationQueueAssignments")"
test "$tenant_operation_queue_assignments_export_code" = "403"

tenant_operation_queue_assignment_notifications_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-assignment-notifications-forbidden.json" -w '%{http_code}' \
  -X POST \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"remark":"tenant"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignmentNotifications?dueState=overdue")"
test "$tenant_operation_queue_assignment_notifications_forbidden_code" = "403"

tenant_operation_queue_assignment_close_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-assignment-close-forbidden.json" -w '%{http_code}' \
  -X POST \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"operationId":1,"closeStatus":"resolved"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignmentClose")"
test "$tenant_operation_queue_assignment_close_forbidden_code" = "403"

tenant_operation_queue_assign_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-operation-queue-assign-forbidden.json" -w '%{http_code}' \
  -X POST \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"owner":"tenant-ops"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssign")"
test "$tenant_operation_queue_assign_forbidden_code" = "403"

tenant_renewal_forecast_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-renewal-forecast-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecast")"
test "$tenant_renewal_forecast_forbidden_code" = "403"

tenant_renewal_forecast_tasks_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-renewal-forecast-tasks-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"months":12}' \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecastTasks")"
test "$tenant_renewal_forecast_tasks_forbidden_code" = "403"

tenant_renewal_forecast_assign_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-renewal-forecast-assign-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"owner":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecastAssign")"
test "$tenant_renewal_forecast_assign_forbidden_code" = "403"

tenant_renewal_forecast_notifications_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-renewal-forecast-notifications-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"reminderDays":30}' \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecastNotifications")"
test "$tenant_renewal_forecast_notifications_forbidden_code" = "403"

tenant_customer_success_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-customer-success-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccess")"
test "$tenant_customer_success_forbidden_code" = "403"

tenant_customer_success_export_code="$(curl -s -o "$WORK_DIR/tenant-customer-success-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=customerSuccess")"
test "$tenant_customer_success_export_code" = "403"

tenant_customer_success_owners_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-customer-success-owners-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessOwners")"
test "$tenant_customer_success_owners_forbidden_code" = "403"

tenant_customer_success_owners_export_code="$(curl -s -o "$WORK_DIR/tenant-customer-success-owners-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=customerSuccessOwners")"
test "$tenant_customer_success_owners_export_code" = "403"

tenant_customer_success_assign_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-customer-success-assign-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"owner":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessAssign")"
test "$tenant_customer_success_assign_forbidden_code" = "403"

tenant_customer_success_renewal_tasks_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-customer-success-renewal-tasks-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"months":12}' \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessRenewalTasks")"
test "$tenant_customer_success_renewal_tasks_forbidden_code" = "403"

tenant_risk_follow_up_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-risk-follow-up-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"status\":\"contacted\",\"remark\":\"tenant-forbidden\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUp")"
test "$tenant_risk_follow_up_forbidden_code" = "403"

tenant_risk_follow_ups_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-risk-follow-ups-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUps?tenantId=$TENANT_ID")"
test "$tenant_risk_follow_ups_forbidden_code" = "403"

tenant_risk_follow_up_owners_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-risk-follow-up-owners-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUpOwners?tenantId=$TENANT_ID")"
test "$tenant_risk_follow_up_owners_forbidden_code" = "403"

tenant_daily_report_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-daily-report-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/dailyReport")"
test "$tenant_daily_report_forbidden_code" = "403"

tenant_risk_follow_up_bulk_close_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-risk-follow-up-bulk-close-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"closeStatus":"resolved","remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUpBulkClose")"
test "$tenant_risk_follow_up_bulk_close_forbidden_code" = "403"

tenant_billing_follow_up_bulk_close_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-billing-follow-up-bulk-close-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"closeStatus":"resolved","remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose")"
test "$tenant_billing_follow_up_bulk_close_forbidden_code" = "403"

tenant_alert_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-alert-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/alerts?tenantId=$PLATFORM_TENANT_ID")"
test "$tenant_alert_forbidden_code" = "403"

tenant_alert_bulk_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-alert-bulk-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$PLATFORM_TENANT_ID,\"alertType\":\"quota_exceeded\",\"limit\":10}" \
  "http://$GO_ADDR/dashboard/saasAdmin/alertBulkResolve")"
test "$tenant_alert_bulk_forbidden_code" = "403"

tenant_notification_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$PLATFORM_TENANT_ID")"
test "$tenant_notification_forbidden_code" = "403"

tenant_notification_health_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-health-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationHealth")"
test "$tenant_notification_health_forbidden_code" = "403"

tenant_notification_health_recovery_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-health-recovery-forbidden.json" -w '%{http_code}' \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationHealthRecovery")"
test "$tenant_notification_health_recovery_forbidden_code" = "403"

tenant_notification_policies_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-policies-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicies")"
test "$tenant_notification_policies_forbidden_code" = "403"

tenant_notification_policy_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-policy-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicy?tenantId=$TENANT_ID")"
test "$tenant_notification_policy_forbidden_code" = "403"

tenant_notification_policy_save_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-policy-save-forbidden.json" -w '%{http_code}' \
  -X PUT \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenantId\":$TENANT_ID,\"enabled\":false}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicy")"
test "$tenant_notification_policy_save_forbidden_code" = "403"

tenant_notification_policy_test_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-policy-test-forbidden.json" -w '%{http_code}' \
  -X POST \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenantId\":$TENANT_ID}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationPolicyTest")"
test "$tenant_notification_policy_test_forbidden_code" = "403"

tenant_notification_bulk_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-bulk-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$PLATFORM_TENANT_ID,\"status\":\"failed\",\"limit\":10}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationBulkRetry")"
test "$tenant_notification_bulk_forbidden_code" = "403"

tenant_notification_bulk_close_forbidden_code="$(curl -s -o "$WORK_DIR/tenant-notification-bulk-close-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$PLATFORM_TENANT_ID,\"status\":\"failed\",\"limit\":10}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationBulkClose")"
test "$tenant_notification_bulk_close_forbidden_code" = "403"

python3 - "$WORK_DIR/platform.json" "$WORK_DIR/platform-filtered.json" "$WORK_DIR/tenant.json" "$WORK_DIR/platform-tenant.json" "$WORK_DIR/platform-tenant-detail.json" "$WORK_DIR/tenant-detail.json" "$WORK_DIR/platform-tenant-usage.json" "$WORK_DIR/tenant-usage.json" "$WORK_DIR/platform-risk.json" "$WORK_DIR/tenant-risk.json" "$WORK_DIR/export-risk-initial.csv" "$WORK_DIR/platform-alerts.json" "$WORK_DIR/tenant-alerts.json" "$WORK_DIR/platform-notifications.json" "$WORK_DIR/tenant-notifications.json" "$WORK_DIR/export-alerts.csv" "$WORK_DIR/export-notifications.csv" "$PLATFORM_TENANT_ID" "$TENANT_ID" <<'PY'
import csv
import json
import pathlib
import sys

platform = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
filtered = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
tenant = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
platform_tenant = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
platform_tenant_detail = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
tenant_detail = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
platform_tenant_usage = json.loads(pathlib.Path(sys.argv[7]).read_text(encoding="utf-8"))
tenant_usage = json.loads(pathlib.Path(sys.argv[8]).read_text(encoding="utf-8"))
platform_risk = json.loads(pathlib.Path(sys.argv[9]).read_text(encoding="utf-8"))
tenant_risk = json.loads(pathlib.Path(sys.argv[10]).read_text(encoding="utf-8"))
risk_export_path = pathlib.Path(sys.argv[11])
platform_alerts = json.loads(pathlib.Path(sys.argv[12]).read_text(encoding="utf-8"))
tenant_alerts = json.loads(pathlib.Path(sys.argv[13]).read_text(encoding="utf-8"))
platform_notifications = json.loads(pathlib.Path(sys.argv[14]).read_text(encoding="utf-8"))
tenant_notifications = json.loads(pathlib.Path(sys.argv[15]).read_text(encoding="utf-8"))
alerts_csv_path = pathlib.Path(sys.argv[16])
notifications_csv_path = pathlib.Path(sys.argv[17])
platform_tenant_id = int(sys.argv[18])
tenant_id = int(sys.argv[19])

for payload in (platform, tenant, platform_tenant):
    assert payload["code"] == 200, payload
    assert payload["data"]["summary"], payload
    assert payload["data"]["tenants"], payload
    assert payload["data"]["metrics"], payload

platform_data = platform["data"]
assert platform_data["scope"] == "platform", platform_data
assert platform_data["tenantPopulation"] == "business_tenants", platform_data
assert platform_data["tenantId"] == 0, platform_data
assert platform_data["canPlatformScope"] is True, platform_data
assert platform_data["platformAdminTenantId"] == platform_tenant_id, platform_data
platform_summary = platform_data["summary"]
assert platform_summary["tenantCount"] == 1, platform_summary
assert platform_summary["activeTenantPackageCount"] == 1, platform_summary
assert platform_summary["openAlertCount"] == 1, platform_summary
assert platform_summary["pendingNotificationCount"] == 3, platform_summary
assert platform_summary["expiringSoonTenantCount"] == 1, platform_summary
assert [item["tenantId"] for item in platform_data["tenants"]] == [tenant_id], platform_data["tenants"]
assert all(item["tenantId"] != platform_tenant_id for item in platform_data["tenants"]), platform_data["tenants"]
tenant_row = next(item for item in platform_data["tenants"] if item["tenantId"] == tenant_id)
assert tenant_row["expiringSoon"] is True, tenant_row
assert tenant_row["openAlertCount"] == 1, tenant_row
assert tenant_row["maxUsageMetric"] == "users", tenant_row
assert abs(float(tenant_row["maxUsageRatio"]) - 0.8) < 0.000001, tenant_row
platform_users_metric = next(item for item in platform_data["metrics"] if item["metric"] == "users")
assert platform_users_metric["current"] == 8, platform_users_metric
assert platform_users_metric["limit"] == 10, platform_users_metric
assert platform_users_metric["openAlertCount"] == 1, platform_users_metric

assert filtered["code"] == 200, filtered
filtered_data = filtered["data"]
assert filtered_data["scope"] == "platform", filtered_data
assert filtered_data["tenantPopulation"] == "business_tenants", filtered_data
assert filtered_data["summary"]["tenantCount"] == 1, filtered_data["summary"]
assert [item["tenantId"] for item in filtered_data["tenants"]] == [tenant_id], filtered_data["tenants"]
filters = filtered_data["filters"]
assert filters["keyword"] == "SaaS普通", filters
assert filters["tenantStatus"] == 1, filters
assert filters["packageCode"] == "growth", filters
assert filters["dueState"] == "expiring", filters

tenant_data = tenant["data"]
assert tenant_data["scope"] == "tenant", tenant_data
assert tenant_data["tenantPopulation"] == "selected_tenant", tenant_data
assert tenant_data["tenantId"] == tenant_id, tenant_data
assert tenant_data["canPlatformScope"] is False, tenant_data
assert tenant_data["summary"]["tenantCount"] == 1, tenant_data
assert tenant_data["summary"]["openAlertCount"] == 1, tenant_data
assert tenant_data["summary"]["pendingNotificationCount"] == 3, tenant_data
assert [item["tenantId"] for item in tenant_data["tenants"]] == [tenant_id], tenant_data["tenants"]

platform_tenant_data = platform_tenant["data"]
assert platform_tenant_data["scope"] == "tenant", platform_tenant_data
assert platform_tenant_data["tenantPopulation"] == "selected_tenant", platform_tenant_data
assert platform_tenant_data["tenantId"] == tenant_id, platform_tenant_data
assert platform_tenant_data["canPlatformScope"] is True, platform_tenant_data
assert platform_tenant_data["summary"]["tenantCount"] == 1, platform_tenant_data
assert [item["tenantId"] for item in platform_tenant_data["tenants"]] == [tenant_id], platform_tenant_data["tenants"]

metric = next(item for item in platform_tenant_data["metrics"] if item["metric"] == "users")
assert metric["current"] == 8, metric
assert metric["limit"] == 10, metric
assert metric["openAlertCount"] == 1, metric

for detail, can_platform_scope in ((platform_tenant_detail, True), (tenant_detail, False)):
    assert detail["code"] == 200, detail
    detail_data = detail["data"]
    assert detail_data["tenantId"] == tenant_id, detail_data
    assert detail_data["canPlatformScope"] is can_platform_scope, detail_data
    assert detail_data["summary"]["tenantCount"] == 1, detail_data
    assert detail_data["tenant"]["tenantId"] == tenant_id, detail_data
    assert detail_data["tenant"]["tenantName"] == "SaaS普通租户", detail_data
    assert detail_data["tenant"]["maxUsageMetric"] == "users", detail_data
    users_metric = next(item for item in detail_data["metrics"] if item["metric"] == "users")
    assert users_metric["current"] == 8, users_metric
    assert users_metric["limit"] == 10, users_metric

for usage_payload, can_platform_scope in ((platform_tenant_usage, True), (tenant_usage, False)):
    assert usage_payload["code"] == 200, usage_payload
    usage = usage_payload["data"]
    assert usage["tenantId"] == tenant_id, usage
    assert usage["canPlatformScope"] is can_platform_scope, usage
    assert usage["tenant"]["tenantName"] == "SaaS普通租户", usage
    summary = usage["summary"]
    assert summary["metricCount"] >= 26, summary
    assert summary["limitedMetricCount"] >= 26, summary
    assert summary["highestUsageMetric"], summary
    metrics = usage["usageMetrics"]
    assert len(metrics) >= 26, metrics
    users_metric = next(item for item in metrics if item["metric"] == "users")
    assert users_metric["current"] == 8, users_metric
    assert users_metric["limit"] == 10, users_metric
    assert users_metric["remaining"] == 2, users_metric
    assert users_metric["status"] in {"warning", "normal"}, users_metric
    assert users_metric["openAlertCount"] == 1, users_metric

for risk_payload, can_platform_scope in ((platform_risk, True), (tenant_risk, False)):
    assert risk_payload["code"] == 200, risk_payload
    risk = risk_payload["data"]
    assert risk["canPlatformScope"] is can_platform_scope, risk
    assert risk["filters"]["highUsageRatio"] == 0.8, risk["filters"]
    summary = risk["summary"]
    assert summary["evaluatedTenantCount"] == 1, summary
    assert summary["riskTenantCount"] >= 1, summary
    assert summary["openAlertTenantCount"] >= 1, summary
    items = risk["riskTenants"]
    assert all(item["tenantId"] != platform_tenant_id for item in items), items
    tenant_risk_row = next(item for item in items if item["tenantId"] == tenant_id)
    assert tenant_risk_row["riskLevel"] in {"medium", "high", "critical"}, tenant_risk_row
    assert tenant_risk_row["riskScore"] >= 25, tenant_risk_row
    assert tenant_risk_row["riskReasons"], tenant_risk_row
    assert "告警" in "；".join(tenant_risk_row["riskReasons"]), tenant_risk_row
    assert tenant_risk_row["suggestedAction"], tenant_risk_row
    assert tenant_risk_row["topUsageMetrics"], tenant_risk_row
    top_metric = tenant_risk_row["topUsageMetrics"][0]
    assert top_metric["metric"] == "users", top_metric
    assert top_metric["current"] == 8, top_metric
    assert top_metric["limit"] == 10, top_metric

with risk_export_path.open("r", encoding="utf-8-sig", newline="") as fh:
    risk_rows = list(csv.reader(fh))
assert risk_rows[0][:6] == ["tenantId", "tenantName", "riskLevel", "riskScore", "riskReasons", "suggestedAction"], risk_rows[0]
assert risk_rows[0][-1] == "topUsageMetrics", risk_rows[0]
assert len(risk_rows) == 2, risk_rows
risk_row = risk_rows[1]
assert int(risk_row[0]) == tenant_id, risk_row
assert risk_row[1] == "SaaS普通租户", risk_row
assert risk_row[2] in {"medium", "high", "critical"}, risk_row
assert int(risk_row[3]) >= 25, risk_row
assert "告警" in risk_row[4], risk_row
assert risk_row[5], risk_row
assert risk_row[-1] and "users=8/10" in risk_row[-1], risk_row

for alert_payload, can_platform_scope in ((platform_alerts, True), (tenant_alerts, False)):
    assert alert_payload["code"] == 200, alert_payload
    alert_data = alert_payload["data"]
    assert alert_data["tenantId"] == tenant_id, alert_data
    assert alert_data["canPlatformScope"] is can_platform_scope, alert_data
    filters = alert_data["filters"]
    assert filters["status"] == "open", filters
    assert filters["metric"] == "users", filters
    assert filters["alertType"] == "quota_exceeded", filters
    alerts = alert_data["alerts"]
    assert len(alerts) == 1, alerts
    assert alert_data["returnedCount"] == len(alerts), alert_data
    summary = alert_data["summary"]
    assert summary["alertCount"] == 1, summary
    assert summary["openCount"] == 1, summary
    assert summary["resolvedCount"] == 0, summary
    assert summary["warningCount"] == 1, summary
    assert summary["tenantCount"] == 1, summary
    assert summary["metricCount"] == 1, summary
    alert = alerts[0]
    assert alert["tenantId"] == tenant_id, alert
    assert alert["metric"] == "users", alert
    assert alert["metricLabel"] == "子账号数", alert
    assert alert["status"] == "open", alert
    assert alert["currentValue"] == 8, alert
    assert alert["limitValue"] == 10, alert

for notification_payload, can_platform_scope in ((platform_notifications, True), (tenant_notifications, False)):
    assert notification_payload["code"] == 200, notification_payload
    data = notification_payload["data"]
    assert data["tenantId"] == tenant_id, data
    assert data["canPlatformScope"] is can_platform_scope, data
    filters = data["filters"]
    assert filters["status"] == "dead", filters
    assert filters["channel"] == "webhook", filters
    assert filters["keyword"] == "smoke", filters
    notifications = data["notifications"]
    assert len(notifications) == 1, notifications
    assert data["returnedCount"] == len(notifications), data
    summary = data["summary"]
    assert summary["notificationCount"] == 1, summary
    assert summary["deadCount"] == 1, summary
    assert summary["retryableCount"] == 1, summary
    assert summary["pendingCount"] == 0, summary
    assert summary["failedCount"] == 0, summary
    assert summary["tenantCount"] == 1, summary
    assert summary["channelCount"] == 1, summary
    notification = notifications[0]
    assert notification["tenantId"] == tenant_id, notification
    assert notification["status"] == "dead", notification
    assert notification["attempts"] == 3, notification
    assert notification["maxAttempts"] == 3, notification
    assert notification["metric"] == "users", notification
    assert notification["metricLabel"] == "子账号数", notification
    assert notification["lastError"] == "smoke webhook failed", notification

with alerts_csv_path.open(newline="", encoding="utf-8-sig") as fh:
    alert_rows = list(csv.reader(fh))
assert alert_rows[0][:8] == ["id", "alertKey", "tenantId", "alertType", "severity", "status", "metric", "metricLabel"], alert_rows[0]
assert len(alert_rows) == 2, alert_rows
assert int(alert_rows[1][2]) == tenant_id, alert_rows[1]
assert alert_rows[1][5] == "open", alert_rows[1]
assert alert_rows[1][6] == "users", alert_rows[1]
assert alert_rows[1][7] == "子账号数", alert_rows[1]
assert alert_rows[1][15], alert_rows[1]

with notifications_csv_path.open(newline="", encoding="utf-8-sig") as fh:
    notification_rows = list(csv.reader(fh))
assert notification_rows[0][:10] == ["id", "notificationKey", "alertKey", "tenantId", "channel", "status", "attempts", "maxAttempts", "metric", "metricLabel"], notification_rows[0]
assert len(notification_rows) == 2, notification_rows
assert int(notification_rows[1][3]) == tenant_id, notification_rows[1]
assert notification_rows[1][4] == "webhook", notification_rows[1]
assert notification_rows[1][5] == "dead", notification_rows[1]
assert notification_rows[1][8] == "users", notification_rows[1]
assert notification_rows[1][9] == "子账号数", notification_rows[1]
assert notification_rows[1][15] == "smoke webhook failed", notification_rows[1]
PY

python3 - "$WORK_DIR/customer-success.json" "$WORK_DIR/export-customer-success.csv" "$WORK_DIR/export-customer-success-owners-initial.csv" "$PLATFORM_TENANT_ID" "$TENANT_ID" <<'PY'
import csv
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
customer_success_csv_path = pathlib.Path(sys.argv[2])
customer_success_owners_csv_path = pathlib.Path(sys.argv[3])
platform_tenant_id = int(sys.argv[4])
tenant_id = int(sys.argv[5])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["canPlatformScope"] is True, data
assert data["filters"]["tenantLimit"] == 200, data["filters"]
assert data["filters"]["highUsageRatio"] == 0.8, data["filters"]
summary = data["summary"]
assert summary["queueCount"] >= 1, summary
assert summary["returnedCount"] == len(data["items"]), summary
assert summary["riskTenantCount"] >= 1, summary
assert summary["retryableNotificationCount"] >= 3, summary
assert all(row["tenantId"] != platform_tenant_id for row in data["items"]), data["items"]
item = next(row for row in data["items"] if row["tenantId"] == tenant_id)
assert item["tenantName"] == "SaaS普通租户", item
assert item["priority"] in {"critical", "high", "medium"}, item
assert item["healthScore"] >= item["riskScore"], item
assert item["riskLevel"] in {"medium", "high", "critical"}, item
assert item["reasons"], item
assert item["nextAction"], item
assert item["retryableNotificationCount"] >= 3, item
assert item["tenant"]["tenantId"] == tenant_id, item
assert item["risk"]["tenantId"] == tenant_id, item

with customer_success_csv_path.open(newline="", encoding="utf-8-sig") as fh:
    rows = list(csv.reader(fh))
assert rows[0][:8] == ["tenantId", "tenantName", "priority", "healthScore", "owner", "dueState", "reasons", "nextAction"], rows[0]
assert rows[0][24:29] == ["billingFollowUpCount", "actionableTaskCount", "retryableNotificationCount", "failedNotificationCount", "deadNotificationCount"], rows[0]
assert all(int(row[0]) != platform_tenant_id for row in rows[1:]), rows
csv_row = next(row for row in rows[1:] if int(row[0]) == tenant_id)
assert csv_row[1] == "SaaS普通租户", csv_row
assert csv_row[2] in {"critical", "high", "medium"}, csv_row
assert int(csv_row[3]) >= 25, csv_row
assert csv_row[6], csv_row
assert csv_row[7], csv_row
assert csv_row[8] in {"critical", "high", "medium"}, csv_row
assert csv_row[18] == "users", csv_row
assert "users=8/10" in csv_row[-1], csv_row

with customer_success_owners_csv_path.open(newline="", encoding="utf-8-sig") as fh:
    owner_rows = list(csv.reader(fh))
assert owner_rows[0][:8] == ["owner", "tenantCount", "criticalCount", "highCount", "mediumCount", "normalCount", "overdueCount", "dueSoonCount"], owner_rows[0]
assert owner_rows[0][10:16] == ["actionableTaskCount", "retryableNotificationCount", "failedNotificationCount", "deadNotificationCount", "maxHealthScore", "averageHealthScore"], owner_rows[0]
assert len(owner_rows) >= 2, owner_rows
assert all("SaaS平台租户#" not in row[-1] for row in owner_rows[1:]), owner_rows
assert any("SaaS普通租户#" in row[-1] for row in owner_rows[1:]), owner_rows
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"owner":"smoke-csm","nextFollowUpAt":"2026-07-19","remark":"smoke-customer-success-assign"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessAssign?tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/customer-success-assign.json"

python3 - "$WORK_DIR/customer-success-assign.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["matchedCount"] >= 1, data
assert data["assignedCount"] >= 1, data
assert data["owner"] == "smoke-csm", data
assert data["status"] == "pending", data
assert data["nextFollowUpAt"] == "2026-07-19 00:00:00", data
follow_up = next(item for item in data["followUps"] if item["tenantId"] == tenant_id)
assert follow_up["owner"] == "smoke-csm", follow_up
assert follow_up["status"] == "pending", follow_up
assert follow_up["nextFollowUpAt"] == "2026-07-19 00:00:00", follow_up
assert follow_up["remark"] == "smoke-customer-success-assign", follow_up
assert int(follow_up["operationId"]) > 0, follow_up
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessOwners?tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&owner=smoke-csm" >"$WORK_DIR/customer-success-owners.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=customerSuccessOwners&tenantLimit=200&limit=1000&expiringDays=10&highUsageRatio=0.8&owner=smoke-csm" >"$WORK_DIR/export-customer-success-owners.csv"

python3 - "$WORK_DIR/customer-success-owners.json" "$WORK_DIR/export-customer-success-owners.csv" "$TENANT_ID" <<'PY'
import csv
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
csv_path = pathlib.Path(sys.argv[2])
tenant_id = int(sys.argv[3])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["canPlatformScope"] is True, data
assert data["filters"]["owner"] == "smoke-csm", data["filters"]
assert data["ownerCount"] >= 1, data
assert data["returnedCount"] == len(data["owners"]), data
assert data["summary"]["queueCount"] >= 1, data["summary"]
owner = next(item for item in data["owners"] if item["owner"] == "smoke-csm")
assert owner["tenantCount"] >= 1, owner
assert owner["criticalCount"] + owner["highCount"] + owner["mediumCount"] + owner["normalCount"] >= 1, owner
assert owner["retryableNotificationCount"] >= 3, owner
assert owner["maxHealthScore"] >= owner["averageHealthScore"], owner
assert owner["topTenants"], owner
target = next(item for item in owner["topTenants"] if item["tenantId"] == tenant_id)
assert target["tenantName"] == "SaaS普通租户", target

with csv_path.open(newline="", encoding="utf-8-sig") as fh:
    rows = list(csv.reader(fh))
assert rows[0][:8] == ["owner", "tenantCount", "criticalCount", "highCount", "mediumCount", "normalCount", "overdueCount", "dueSoonCount"], rows[0]
csv_row = next(row for row in rows[1:] if row[0] == "smoke-csm")
assert int(csv_row[1]) >= 1, csv_row
assert int(csv_row[11]) >= 3, csv_row
assert "SaaS普通租户#" in csv_row[-1], csv_row
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"status\":\"contacted\",\"owner\":\"smoke-csm\",\"nextFollowUpAt\":\"2026-07-20\",\"remark\":\"smoke-risk-follow-up\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUp" >"$WORK_DIR/risk-follow-up.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.risk.follow_up&targetType=tenant&keyword=smoke-risk-follow-up" >"$WORK_DIR/operations-risk-follow-up.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/risk?scope=platform&tenantId=$TENANT_ID&expiringDays=10&highUsageRatio=0.8&limit=50" >"$WORK_DIR/platform-risk-after-follow-up.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUps?tenantId=$TENANT_ID&status=contacted&owner=smoke&keyword=smoke-risk-follow-up&limit=20" >"$WORK_DIR/risk-follow-ups.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUpOwners?tenantId=$TENANT_ID&status=contacted&owner=smoke&keyword=smoke-risk-follow-up&limit=1000" >"$WORK_DIR/risk-follow-up-owners.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/dailyReport?date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/daily-report.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=dailyReport&date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/export-daily-report.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=riskFollowUps&tenantId=$TENANT_ID&status=contacted&owner=smoke&keyword=smoke-risk-follow-up&limit=1000" >"$WORK_DIR/export-risk-follow-ups.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=riskFollowUpOwners&tenantId=$TENANT_ID&status=contacted&owner=smoke&keyword=smoke-risk-follow-up&limit=1000" >"$WORK_DIR/export-risk-follow-up-owners.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=risk&tenantId=$TENANT_ID&highUsageRatio=0.8&limit=1000" >"$WORK_DIR/export-risk-after-follow-up.csv"

REPORT_DATE="$REPORT_DATE" python3 - "$WORK_DIR/risk-follow-up.json" "$WORK_DIR/operations-risk-follow-up.json" "$WORK_DIR/platform-risk-after-follow-up.json" "$WORK_DIR/risk-follow-ups.json" "$WORK_DIR/risk-follow-up-owners.json" "$WORK_DIR/daily-report.json" "$WORK_DIR/export-daily-report.csv" "$WORK_DIR/export-risk-follow-ups.csv" "$WORK_DIR/export-risk-follow-up-owners.csv" "$WORK_DIR/export-risk-after-follow-up.csv" "$TENANT_ID" <<'PY'
import csv
import json
import os
import pathlib
import sys

follow_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
risk_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
risk_tasks_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
risk_owner_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
daily_report_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
daily_report_csv_path = pathlib.Path(sys.argv[7])
risk_tasks_csv_path = pathlib.Path(sys.argv[8])
risk_owner_csv_path = pathlib.Path(sys.argv[9])
risk_csv_path = pathlib.Path(sys.argv[10])
tenant_id = int(sys.argv[11])
report_date = os.environ["REPORT_DATE"]

assert follow_payload["code"] == 200, follow_payload
follow = follow_payload["data"]
assert follow["tenantId"] == tenant_id, follow
assert follow["status"] == "contacted", follow
assert follow["owner"] == "smoke-csm", follow
assert follow["nextFollowUpAt"] == "2026-07-20 00:00:00", follow
assert follow["remark"] == "smoke-risk-follow-up", follow
assert follow["operationId"] > 0, follow

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert operations, operations
operation = operations[0]
assert operation["id"] == follow["operationId"], operation
assert operation["tenantId"] == tenant_id, operation
assert operation["action"] == "tenant.risk.follow_up", operation
assert operation["targetType"] == "tenant", operation
assert operation["targetId"] == str(tenant_id), operation
assert operation["after"]["status"] == "contacted", operation
assert operation["after"]["owner"] == "smoke-csm", operation
assert operation["after"]["nextFollowUpAt"] == "2026-07-20 00:00:00", operation
assert operation["remark"] == "smoke-risk-follow-up", operation

assert risk_payload["code"] == 200, risk_payload
risk = risk_payload["data"]
assert risk["summary"]["followUpTenantCount"] >= 1, risk["summary"]
assert risk["summary"]["pendingFollowUpCount"] >= 1, risk["summary"]
risk_row = next(item for item in risk["riskTenants"] if item["tenantId"] == tenant_id)
follow_up = risk_row["followUp"]
assert follow_up["operationId"] == follow["operationId"], follow_up
assert follow_up["status"] == "contacted", follow_up
assert follow_up["owner"] == "smoke-csm", follow_up
assert follow_up["nextFollowUpAt"] == "2026-07-20 00:00:00", follow_up
assert follow_up["remark"] == "smoke-risk-follow-up", follow_up

assert risk_tasks_payload["code"] == 200, risk_tasks_payload
risk_tasks = risk_tasks_payload["data"]
task_filters = risk_tasks["filters"]
assert task_filters["tenantId"] == tenant_id, task_filters
assert task_filters["status"] == "contacted", task_filters
assert task_filters["owner"] == "smoke", task_filters
assert task_filters["keyword"] == "smoke-risk-follow-up", task_filters
assert risk_tasks["summary"]["totalCount"] >= 1, risk_tasks["summary"]
assert risk_tasks["summary"]["contactedCount"] >= 1, risk_tasks["summary"]
task = next(item for item in risk_tasks["followUps"] if item["operationId"] == follow["operationId"])
assert task["tenantId"] == tenant_id, task
assert task["status"] == "contacted", task
assert task["owner"] == "smoke-csm", task
assert task["nextFollowUpAt"] == "2026-07-20 00:00:00", task
assert task["remark"] == "smoke-risk-follow-up", task
assert task["dueState"] in {"overdue", "due_soon", "future", "no_date", "closed"}, task
assert isinstance(task["overdue"], bool), task
assert isinstance(task["daysUntil"], int), task

assert risk_owner_payload["code"] == 200, risk_owner_payload
owners = risk_owner_payload["data"]["owners"]
owner = next(item for item in owners if item["owner"] == "smoke-csm")
assert owner["totalCount"] >= 1, owner
assert owner["openCount"] >= 1, owner
assert owner["contactedCount"] >= 1, owner
assert owner["latestFollowUpAt"], owner
assert owner["nextFollowUpAt"] == "2026-07-20 00:00:00", owner

assert daily_report_payload["code"] == 200, daily_report_payload
daily = daily_report_payload["data"]
assert daily["window"]["date"] == report_date, daily["window"]
assert daily["window"]["days"] == 1, daily["window"]
daily_summary = daily["summary"]
assert daily_summary["tenantCount"] == 1, daily_summary
assert daily_summary["riskTenantCount"] >= 1, daily_summary
assert daily_summary["openRiskFollowUpCount"] >= 1, daily_summary
assert daily_summary["openAlertCount"] >= 1, daily_summary
assert daily_summary["retryableNotificationCount"] >= 3, daily_summary
assert daily_summary["windowOperationCount"] >= 1, daily_summary
assert daily_summary["windowRiskFollowUpCount"] >= 1, daily_summary
daily_owners = daily["riskFollowUps"]["owners"]
assert all(item["owner"] != "platform-only" for item in daily_owners), daily_owners
daily_owner = next(item for item in daily_owners if item["owner"] == "smoke-csm")
assert daily_owner["openCount"] >= 1, daily_owner
daily_actions = daily["operations"]["summary"]
assert any(item["action"] == "tenant.risk.follow_up" for item in daily_actions), daily_actions

with daily_report_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    daily_rows = list(csv.reader(fh))
assert daily_rows[0] == ["section", "metric", "value", "remark"], daily_rows[0]
assert ["window", "date", report_date, ""] in daily_rows, daily_rows
assert ["window", "days", "1", ""] in daily_rows, daily_rows
assert any(row[:3] == ["summary", "openRiskFollowUpCount", str(daily_summary["openRiskFollowUpCount"])] for row in daily_rows), daily_rows
assert any(row[0] == "riskOwner" and row[1] == "smoke-csm" for row in daily_rows), daily_rows
assert any(row[0] == "operationAction" and row[1] == "tenant.risk.follow_up" for row in daily_rows), daily_rows

with risk_tasks_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    task_rows = list(csv.reader(fh))
task_header = task_rows[0]
assert task_header == ["operationId", "tenantId", "tenantName", "status", "owner", "nextFollowUpAt", "dueState", "overdue", "daysUntil", "remark", "createdAt"], task_header
task_row = next(row for row in task_rows[1:] if int(row[0]) == follow["operationId"])
assert int(task_row[1]) == tenant_id, task_row
assert task_row[3] == "contacted", task_row
assert task_row[4] == "smoke-csm", task_row
assert task_row[5] == "2026-07-20 00:00:00", task_row
assert task_row[6] in {"overdue", "due_soon", "future", "no_date", "closed"}, task_row
assert task_row[7] in {"true", "false"}, task_row
assert task_row[9] == "smoke-risk-follow-up", task_row

with risk_owner_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    owner_rows = list(csv.reader(fh))
owner_header = owner_rows[0]
assert owner_header == ["owner", "totalCount", "openCount", "pendingCount", "contactedCount", "renewalPendingCount", "resolvedCount", "ignoredCount", "overdueCount", "dueSoonCount", "futureCount", "noDateCount", "closedCount", "latestFollowUpAt", "nextFollowUpAt"], owner_header
owner_row = next(row for row in owner_rows[1:] if row[0] == "smoke-csm")
assert int(owner_row[1]) >= 1, owner_row
assert int(owner_row[2]) >= 1, owner_row
assert int(owner_row[4]) >= 1, owner_row
assert owner_row[14] == "2026-07-20 00:00:00", owner_row

with risk_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    rows = list(csv.reader(fh))
header = rows[0]
assert "followUpStatus" in header, header
assert "followUpOwner" in header, header
assert "nextFollowUpAt" in header, header
assert header[-1] == "topUsageMetrics", header
row = rows[1]
assert int(row[0]) == tenant_id, row
assert row[header.index("followUpStatus")] == "contacted", row
assert row[header.index("followUpOwner")] == "smoke-csm", row
assert row[header.index("nextFollowUpAt")] == "2026-07-20 00:00:00", row
assert row[header.index("followUpRemark")] == "smoke-risk-follow-up", row
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"filterStatus\":\"contacted\",\"keyword\":\"smoke-risk-follow-up\",\"limit\":10,\"closeStatus\":\"resolved\",\"remark\":\"smoke-risk-follow-up-close\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUpBulkClose" >"$WORK_DIR/risk-follow-up-bulk-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUps?tenantId=$TENANT_ID&status=resolved&keyword=smoke-risk-follow-up-close&dueState=closed&limit=20" >"$WORK_DIR/risk-follow-ups-closed.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.risk.follow_up&targetType=tenant&keyword=smoke-risk-follow-up-close" >"$WORK_DIR/operations-risk-follow-up-close.json"

python3 - "$WORK_DIR/risk-follow-up-bulk-close.json" "$WORK_DIR/risk-follow-ups-closed.json" "$WORK_DIR/operations-risk-follow-up-close.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

close_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
closed_tasks_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])

assert close_payload["code"] == 200, close_payload
closed = close_payload["data"]
assert closed["closedCount"] == 1, closed
assert closed["status"] == "resolved", closed
assert closed["remark"] == "smoke-risk-follow-up-close", closed
closed_item = closed["followUps"][0]
assert closed_item["tenantId"] == tenant_id, closed_item
assert closed_item["status"] == "resolved", closed_item
assert closed_item["owner"] == "smoke-csm", closed_item
assert closed_item["nextFollowUpAt"] == "", closed_item
assert closed_item["remark"] == "smoke-risk-follow-up-close", closed_item
assert closed_item["operationId"] > 0, closed_item

assert closed_tasks_payload["code"] == 200, closed_tasks_payload
tasks = closed_tasks_payload["data"]
assert tasks["summary"]["resolvedCount"] >= 1, tasks["summary"]
assert tasks["summary"]["closedCount"] >= 1, tasks["summary"]
task = next(item for item in tasks["followUps"] if item["operationId"] == closed_item["operationId"])
assert task["tenantId"] == tenant_id, task
assert task["status"] == "resolved", task
assert task["dueState"] == "closed", task
assert task["remark"] == "smoke-risk-follow-up-close", task

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
operation = next(item for item in operations if item["id"] == closed_item["operationId"])
assert operation["tenantId"] == tenant_id, operation
assert operation["action"] == "tenant.risk.follow_up", operation
assert operation["after"]["status"] == "resolved", operation
assert operation["remark"] == "smoke-risk-follow-up-close", operation
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000902","password":"secret902"}' \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/tenant-auth-before-expire.json"

python3 - "$WORK_DIR/tenant-auth-before-expire.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
assert payload["data"]["token"], payload
PY

mysql_root mochat_saas_admin <<SQL
UPDATE mochat_go_saas_tenant_packages
SET expires_at = DATE_SUB(NOW(), INTERVAL 1 DAY)
WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL;
SQL

tenant_expired_auth_code="$(curl -s -o "$WORK_DIR/tenant-auth-expired.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000902","password":"secret902"}' \
  "http://$GO_ADDR/dashboard/user/auth")"
test "$tenant_expired_auth_code" = "403"

tenant_expired_overview_code="$(curl -s -o "$WORK_DIR/tenant-expired-self-overview.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?expiringDays=10")"
test "$tenant_expired_overview_code" = "401"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"packageCode\":\"growth\",\"expiresAt\":\"2037-01-01\",\"amountCents\":0,\"currency\":\"CNY\",\"remark\":\"smoke-expired-restore\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewal" >"$WORK_DIR/tenant-expired-renewal.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000902","password":"secret902"}' \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/tenant-auth-after-expire-renewal.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?expiringDays=10" >"$WORK_DIR/tenant-after-expire-renewal-overview.json"

python3 - "$WORK_DIR/tenant-auth-expired.json" "$WORK_DIR/tenant-expired-renewal.json" "$WORK_DIR/tenant-auth-after-expire-renewal.json" "$WORK_DIR/tenant-after-expire-renewal-overview.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

expired_auth = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
renewal_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
restored_auth = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
overview = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[5])

assert expired_auth["code"] == 403, expired_auth
assert expired_auth["msg"] == "租户套餐已到期", expired_auth

assert renewal_payload["code"] == 200, renewal_payload
renewal = renewal_payload["data"]
assert renewal["tenantId"] == tenant_id, renewal
assert renewal["packageCode"] == "growth", renewal
assert renewal["expiresAt"] == "2037-01-01 00:00:00", renewal
assert renewal["billingEventId"] > 0, renewal

assert restored_auth["code"] == 200, restored_auth
assert restored_auth["data"]["token"], restored_auth

assert overview["code"] == 200, overview
assert overview["data"]["tenantId"] == tenant_id, overview
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/packages" >"$WORK_DIR/packages.json"

tenant_packages_code="$(curl -s -o "$WORK_DIR/tenant-packages-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/packages")"
test "$tenant_packages_code" = "403"

tenant_operations_code="$(curl -s -o "$WORK_DIR/tenant-operations-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations")"
test "$tenant_operations_code" = "403"

tenant_billing_code="$(curl -s -o "$WORK_DIR/tenant-billing-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingEvents")"
test "$tenant_billing_code" = "403"

tenant_billing_follow_ups_code="$(curl -s -o "$WORK_DIR/tenant-billing-follow-ups-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUps")"
test "$tenant_billing_follow_ups_code" = "403"

tenant_billing_follow_up_owners_code="$(curl -s -o "$WORK_DIR/tenant-billing-follow-up-owners-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUpOwners")"
test "$tenant_billing_follow_up_owners_code" = "403"

tenant_export_code="$(curl -s -o "$WORK_DIR/tenant-export-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=tenants")"
test "$tenant_export_code" = "403"

tenant_alert_export_code="$(curl -s -o "$WORK_DIR/tenant-alert-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=alerts")"
test "$tenant_alert_export_code" = "403"

tenant_notification_export_code="$(curl -s -o "$WORK_DIR/tenant-notification-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=notifications")"
test "$tenant_notification_export_code" = "403"

tenant_risk_follow_up_export_code="$(curl -s -o "$WORK_DIR/tenant-risk-follow-up-export-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=riskFollowUps")"
test "$tenant_risk_follow_up_export_code" = "403"

tenant_risk_follow_up_owner_export_code="$(curl -s -o "$WORK_DIR/tenant-risk-follow-up-owner-export-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=riskFollowUpOwners")"
test "$tenant_risk_follow_up_owner_export_code" = "403"

tenant_billing_follow_up_export_code="$(curl -s -o "$WORK_DIR/tenant-billing-follow-up-export-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=billingReconciliationFollowUps")"
test "$tenant_billing_follow_up_export_code" = "403"

tenant_billing_follow_up_owner_export_code="$(curl -s -o "$WORK_DIR/tenant-billing-follow-up-owner-export-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners")"
test "$tenant_billing_follow_up_owner_export_code" = "403"

tenant_daily_report_export_code="$(curl -s -o "$WORK_DIR/tenant-daily-report-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=dailyReport")"
test "$tenant_daily_report_export_code" = "403"

tenant_business_metrics_export_code="$(curl -s -o "$WORK_DIR/tenant-business-metrics-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=businessMetrics")"
test "$tenant_business_metrics_export_code" = "403"

tenant_business_trends_export_code="$(curl -s -o "$WORK_DIR/tenant-business-trends-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=businessTrends")"
test "$tenant_business_trends_export_code" = "403"

tenant_renewal_forecast_export_code="$(curl -s -o "$WORK_DIR/tenant-renewal-forecast-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=renewalForecast")"
test "$tenant_renewal_forecast_export_code" = "403"

tenant_renewal_forecast_owner_export_code="$(curl -s -o "$WORK_DIR/tenant-renewal-forecast-owner-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=renewalForecastOwners")"
test "$tenant_renewal_forecast_owner_export_code" = "403"

tenant_provision_code="$(curl -s -o "$WORK_DIR/tenant-provision-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"tenantName":"租户越权","adminPhone":"13800000988","password":"secret988","packageCode":"growth"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvision")"
test "$tenant_provision_code" = "403"

tenant_provision_task_code="$(curl -s -o "$WORK_DIR/tenant-provision-task-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"tenantName":"租户越权任务","adminPhone":"13800000989","password":"secret989","packageCode":"growth"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTask")"
test "$tenant_provision_task_code" = "403"

tenant_provision_task_apply_code="$(curl -s -o "$WORK_DIR/tenant-provision-task-apply-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskId":1}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTaskApply")"
test "$tenant_provision_task_apply_code" = "403"

tenant_provision_task_bulk_apply_code="$(curl -s -o "$WORK_DIR/tenant-provision-task-bulk-apply-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskType":"tenant_provision","status":"pending"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTaskBulkApply")"
test "$tenant_provision_task_bulk_apply_code" = "403"

tenant_task_cancel_code="$(curl -s -o "$WORK_DIR/tenant-task-cancel-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskId":1,"remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskCancel")"
test "$tenant_task_cancel_code" = "403"

tenant_task_bulk_cancel_code="$(curl -s -o "$WORK_DIR/tenant-task-bulk-cancel-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskType":"package_sync","status":"pending","limit":10,"remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskBulkCancel")"
test "$tenant_task_bulk_cancel_code" = "403"

tenant_task_bulk_reset_code="$(curl -s -o "$WORK_DIR/tenant-task-bulk-reset-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskType":"package_sync","status":"all","limit":10,"remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskBulkReset")"
test "$tenant_task_bulk_reset_code" = "403"

tenant_task_reset_code="$(curl -s -o "$WORK_DIR/tenant-task-reset-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskId":1,"remark":"tenant-forbidden"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskReset")"
test "$tenant_task_reset_code" = "403"

tenant_task_owners_code="$(curl -s -o "$WORK_DIR/tenant-task-owners-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/taskOwners?taskType=all&limit=10")"
test "$tenant_task_owners_code" = "403"

tenant_task_sla_code="$(curl -s -o "$WORK_DIR/tenant-task-sla-forbidden.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/taskSla?taskType=all&limit=10")"
test "$tenant_task_sla_code" = "403"

tenant_task_sla_export_code="$(curl -s -o "$WORK_DIR/tenant-task-sla-export-forbidden.csv" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=taskSla&taskType=all&limit=10")"
test "$tenant_task_sla_export_code" = "403"

tenant_task_sla_notifications_code="$(curl -s -o "$WORK_DIR/tenant-task-sla-notifications-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"slaStatus":"warning"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskSlaNotifications?taskType=all&limit=10")"
test "$tenant_task_sla_notifications_code" = "403"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"tenantName":"SaaS新开租户","adminPhone":"13800000988","adminName":"新租户管理员","password":"secret988","roleName":"新租户超级管理员","packageCode":"growth","expiresAt":"2028-03-04","configCopyMode":"missing","remark":"smoke-provision"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvision" >"$WORK_DIR/tenant-provision.json"

python3 - "$WORK_DIR/tenant-provision.json" <<'PY' >"$WORK_DIR/tenant-provision.env"
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
data = payload["data"]
assert data["tenantId"] > 0, data
assert data["tenantName"] == "SaaS新开租户", data
assert data["adminUserId"] > 0, data
assert data["adminPhone"] == "13800000988", data
assert data["adminName"] == "新租户管理员", data
assert data["roleId"] > 0, data
assert data["roleName"] == "新租户超级管理员", data
assert data["packageCode"] == "growth", data
assert data["packageName"] == "增长版", data
assert data["expiresAt"] == "2028-03-04 00:00:00", data
assert data["menuCount"] > 0, data
assert data["metricsRefreshed"] == 26, data
assert data["operationId"] > 0, data
print(f"PROVISION_TENANT_ID={int(data['tenantId'])}")
print(f"PROVISION_USER_ID={int(data['adminUserId'])}")
PY
# shellcheck disable=SC1091
source "$WORK_DIR/tenant-provision.env"

test "$(mysql_scalar "SELECT name FROM mc_tenant WHERE id = $PROVISION_TENANT_ID AND status = 1 AND deleted_at IS NULL")" = "SaaS新开租户"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE id = $PROVISION_USER_ID AND tenant_id = $PROVISION_TENANT_ID AND phone = '13800000988' AND isSuperAdmin = 1 AND status = 1 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT package_code FROM mochat_go_saas_tenant_packages WHERE tenant_id = $PROVISION_TENANT_ID AND deleted_at IS NULL")" = "growth"
test "$(mysql_scalar "SELECT limit_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $PROVISION_TENANT_ID AND metric = 'users' AND period_key = 'lifetime' AND deleted_at IS NULL")" = "10"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $PROVISION_TENANT_ID AND metric = 'users' AND period_key = 'lifetime' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_tenant_provision_runs WHERE tenant_id = $PROVISION_TENANT_ID AND admin_phone = '13800000988' AND status = 1")" = "1"

curl -sS -f \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000988","password":"secret988"}' \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/provision-auth.json"

PROVISION_TOKEN="$(python3 - "$WORK_DIR/provision-auth.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
print(payload["data"]["token"])
PY
)"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenant?tenantId=$PROVISION_TENANT_ID&operationLimit=10&expiringDays=10" >"$WORK_DIR/provision-detail.json"

curl -sS -f \
  -H "Authorization: Bearer $PROVISION_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?expiringDays=10" >"$WORK_DIR/provision-self-overview.json"

python3 - "$WORK_DIR/provision-auth.json" "$WORK_DIR/provision-detail.json" "$WORK_DIR/provision-self-overview.json" "$PROVISION_TENANT_ID" <<'PY'
import json
import pathlib
import sys

auth_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
detail_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
self_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])

assert auth_payload["code"] == 200, auth_payload
assert auth_payload["data"]["token"], auth_payload

assert detail_payload["code"] == 200, detail_payload
detail = detail_payload["data"]
assert detail["tenantId"] == tenant_id, detail
assert detail["tenant"]["tenantName"] == "SaaS新开租户", detail
assert detail["tenant"]["packageCode"] == "growth", detail
users_metric = next(item for item in detail["metrics"] if item["metric"] == "users")
assert users_metric["current"] == 1, users_metric
assert users_metric["limit"] == 10, users_metric
provision_ops = [item for item in detail["operations"] if item["action"] == "tenant.provision"]
assert provision_ops, detail["operations"]
assert provision_ops[0]["after"]["adminPhone"] == "13800000988", provision_ops[0]

assert self_payload["code"] == 200, self_payload
self_data = self_payload["data"]
assert self_data["tenantId"] == tenant_id, self_data
assert self_data["canPlatformScope"] is False, self_data
assert self_data["summary"]["tenantCount"] == 1, self_data
assert self_data["tenants"][0]["tenantId"] == tenant_id, self_data
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"tenantName":"SaaS任务开户租户","adminPhone":"13800000989","adminName":"任务开户管理员","password":"secret989","roleName":"任务超级管理员","packageCode":"growth","expiresAt":"2028-04-05","configCopyMode":"missing","remark":"smoke-provision-task"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTask" >"$WORK_DIR/tenant-provision-task-created.json"

TENANT_PROVISION_TASK_ID="$(python3 - "$WORK_DIR/tenant-provision-task-created.json" <<'PY'
import json
import pathlib
import sys
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
assert "secret989" not in text, text
assert '"adminPasswordHash"' not in text, text
payload = json.loads(text)
assert payload["code"] == 200, payload
data = payload["data"]
task = data["task"]
result = data["result"]
assert task["taskType"] == "tenant_provision", task
assert task["status"] == "pending", task
assert task["request"]["hasAdminPasswordHash"] is True, task
assert "password" not in task["request"], task
assert "adminPasswordHash" not in task["request"], task
assert result["tenantName"] == "SaaS任务开户租户", result
assert result["adminPhone"] == "13800000989", result
assert result["packageCode"] == "growth", result
assert result["packageName"] == "增长版", result
print(int(task["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskId\":$TENANT_PROVISION_TASK_ID}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTaskApply" >"$WORK_DIR/tenant-provision-task-applied.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskType=tenant_provision&packageCode=growth&limit=20" >"$WORK_DIR/tenant-provision-tasks.json"

python3 - "$WORK_DIR/tenant-provision-task-created.json" "$WORK_DIR/tenant-provision-task-applied.json" "$WORK_DIR/tenant-provision-tasks.json" "$TENANT_PROVISION_TASK_ID" <<'PY' >"$WORK_DIR/tenant-provision-task.env"
import json
import pathlib
import sys

created_text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
applied_text = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
tasks_text = pathlib.Path(sys.argv[3]).read_text(encoding="utf-8")
task_id = int(sys.argv[4])
for text in (created_text, applied_text, tasks_text):
    assert "secret989" not in text, text
    assert '"adminPasswordHash"' not in text, text

created_payload = json.loads(created_text)
applied_payload = json.loads(applied_text)
tasks_payload = json.loads(tasks_text)

assert created_payload["code"] == 200, created_payload
created_task = created_payload["data"]["task"]
assert created_task["id"] == task_id, created_task
assert created_task["request"]["hasAdminPasswordHash"] is True, created_task

assert applied_payload["code"] == 200, applied_payload
applied_task = applied_payload["data"]["task"]
applied_result = applied_payload["data"]["result"]
assert applied_task["id"] == task_id, applied_task
assert applied_task["taskType"] == "tenant_provision", applied_task
assert applied_task["status"] == "applied", applied_task
assert applied_task["request"]["hasAdminPasswordHash"] is True, applied_task
assert applied_result["tenantId"] > 0, applied_result
assert applied_result["tenantName"] == "SaaS任务开户租户", applied_result
assert applied_result["adminUserId"] > 0, applied_result
assert applied_result["adminPhone"] == "13800000989", applied_result
assert applied_result["adminName"] == "任务开户管理员", applied_result
assert applied_result["packageCode"] == "growth", applied_result
assert applied_result["metricsRefreshed"] == 26, applied_result
assert applied_result["operationId"] > 0, applied_result

assert tasks_payload["code"] == 200, tasks_payload
task_items = tasks_payload["data"]["tasks"]
matched = [item for item in task_items if item["id"] == task_id]
assert matched and matched[0]["status"] == "applied", task_items
assert matched[0]["request"]["hasAdminPasswordHash"] is True, matched[0]
assert tasks_payload["data"]["filters"]["taskType"] == "tenant_provision", tasks_payload["data"]["filters"]

print(f"TASK_PROVISION_TENANT_ID={int(applied_result['tenantId'])}")
print(f"TASK_PROVISION_USER_ID={int(applied_result['adminUserId'])}")
PY
# shellcheck disable=SC1091
source "$WORK_DIR/tenant-provision-task.env"

test "$(mysql_scalar "SELECT name FROM mc_tenant WHERE id = $TASK_PROVISION_TENANT_ID AND status = 1 AND deleted_at IS NULL")" = "SaaS任务开户租户"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE id = $TASK_PROVISION_USER_ID AND tenant_id = $TASK_PROVISION_TENANT_ID AND phone = '13800000989' AND isSuperAdmin = 1 AND status = 1 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT package_code FROM mochat_go_saas_tenant_packages WHERE tenant_id = $TASK_PROVISION_TENANT_ID AND deleted_at IS NULL")" = "growth"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_tenant_provision_runs WHERE tenant_id = $TASK_PROVISION_TENANT_ID AND admin_phone = '13800000989' AND status = 1")" = "1"

curl -sS -f \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000989","password":"secret989"}' \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/tenant-provision-task-auth.json"

python3 - "$WORK_DIR/tenant-provision-task-auth.json" <<'PY'
import json
import pathlib
import sys
auth_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert auth_payload["code"] == 200, auth_payload
assert auth_payload["data"]["token"], auth_payload
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"tenantName":"SaaS批量任务开户租户","adminPhone":"13800000990","adminName":"批量任务管理员","password":"secret990","roleName":"批量超级管理员","packageCode":"growth","expiresAt":"2028-04-06","configCopyMode":"missing","remark":"smoke-provision-task-bulk"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTask" >"$WORK_DIR/tenant-provision-task-bulk-created.json"

TENANT_PROVISION_BULK_TASK_ID="$(python3 - "$WORK_DIR/tenant-provision-task-bulk-created.json" <<'PY'
import json
import pathlib
import sys
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
assert "secret990" not in text, text
assert '"adminPasswordHash"' not in text, text
payload = json.loads(text)
assert payload["code"] == 200, payload
task = payload["data"]["task"]
result = payload["data"]["result"]
assert task["taskType"] == "tenant_provision", task
assert task["status"] == "pending", task
assert task["request"]["hasAdminPasswordHash"] is True, task
assert result["tenantName"] == "SaaS批量任务开户租户", result
assert result["adminPhone"] == "13800000990", result
print(int(task["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"taskType":"tenant_provision","status":"pending","packageCode":"growth","limit":20,"remark":"smoke-provision-task-bulk-apply"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantProvisionTaskBulkApply" >"$WORK_DIR/tenant-provision-task-bulk-applied.json"

python3 - "$WORK_DIR/tenant-provision-task-bulk-created.json" "$WORK_DIR/tenant-provision-task-bulk-applied.json" "$TENANT_PROVISION_BULK_TASK_ID" <<'PY' >"$WORK_DIR/tenant-provision-task-bulk.env"
import json
import pathlib
import sys

created_text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
applied_text = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
task_id = int(sys.argv[3])
for text in (created_text, applied_text):
    assert "secret990" not in text, text
    assert '"adminPasswordHash"' not in text, text

created_payload = json.loads(created_text)
applied_payload = json.loads(applied_text)
assert created_payload["code"] == 200, created_payload
assert applied_payload["code"] == 200, applied_payload
data = applied_payload["data"]
assert data["applied"] is True, data
assert data["appliedCount"] >= 1, data
assert data["failedCount"] == 0, data
assert data["skippedCanceledCount"] >= 0, data
filters = data["filters"]
assert filters["taskType"] == "tenant_provision", filters
assert filters["packageCode"] == "growth", filters
tasks = data["tasks"]
matched = [item for item in tasks if item["id"] == task_id]
assert matched and matched[0]["status"] == "applied", tasks
task = matched[0]
assert task["request"]["hasAdminPasswordHash"] is True, task
result = task["result"]
assert result["tenantId"] > 0, result
assert result["tenantName"] == "SaaS批量任务开户租户", result
assert result["adminUserId"] > 0, result
assert result["adminPhone"] == "13800000990", result
assert result["adminName"] == "批量任务管理员", result
assert result["packageCode"] == "growth", result
assert result["metricsRefreshed"] == 26, result
assert result["operationId"] > 0, result

print(f"BULK_TASK_PROVISION_TENANT_ID={int(result['tenantId'])}")
print(f"BULK_TASK_PROVISION_USER_ID={int(result['adminUserId'])}")
PY
# shellcheck disable=SC1091
source "$WORK_DIR/tenant-provision-task-bulk.env"

test "$(mysql_scalar "SELECT name FROM mc_tenant WHERE id = $BULK_TASK_PROVISION_TENANT_ID AND status = 1 AND deleted_at IS NULL")" = "SaaS批量任务开户租户"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE id = $BULK_TASK_PROVISION_USER_ID AND tenant_id = $BULK_TASK_PROVISION_TENANT_ID AND phone = '13800000990' AND isSuperAdmin = 1 AND status = 1 AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT package_code FROM mochat_go_saas_tenant_packages WHERE tenant_id = $BULK_TASK_PROVISION_TENANT_ID AND deleted_at IS NULL")" = "growth"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_tenant_provision_runs WHERE tenant_id = $BULK_TASK_PROVISION_TENANT_ID AND admin_phone = '13800000990' AND status = 1")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_tasks WHERE id = $TENANT_PROVISION_BULK_TASK_ID AND task_type = 'tenant_provision' AND status = 'applied' AND applied_at IS NOT NULL AND deleted_at IS NULL")" = "1"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"status\":2,\"remark\":\"smoke-disable\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantStatus" >"$WORK_DIR/tenant-status-disable.json"

tenant_status_code="$(curl -s -o "$WORK_DIR/tenant-status-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$PLATFORM_TENANT_ID,\"status\":2}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantStatus")"
test "$tenant_status_code" = "401"

platform_disable_code="$(curl -s -o "$WORK_DIR/platform-status-disable.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$PLATFORM_TENANT_ID,\"status\":2}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantStatus")"
test "$platform_disable_code" = "400"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=tenant&tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/tenant-status-disabled-overview.json"

tenant_disabled_overview_code="$(curl -s -o "$WORK_DIR/tenant-disabled-self-overview.json" -w '%{http_code}' \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?expiringDays=10")"
test "$tenant_disabled_overview_code" = "401"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"status\":1,\"remark\":\"smoke-enable\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantStatus" >"$WORK_DIR/tenant-status-enable.json"

curl -sS -f \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?expiringDays=10" >"$WORK_DIR/tenant-enabled-self-overview.json"

python3 - "$WORK_DIR/tenant-status-disable.json" "$WORK_DIR/tenant-status-disabled-overview.json" "$WORK_DIR/tenant-status-enable.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

disabled = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
overview = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
enabled = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])

assert disabled["code"] == 200, disabled
disabled_data = disabled["data"]
assert disabled_data["tenantId"] == tenant_id, disabled_data
assert disabled_data["previousStatus"] == 1, disabled_data
assert disabled_data["status"] == 2, disabled_data
assert disabled_data["remark"] == "smoke-disable", disabled_data
assert disabled_data["operationId"] > 0, disabled_data

assert overview["code"] == 200, overview
tenant = overview["data"]["tenants"][0]
assert tenant["tenantId"] == tenant_id, tenant
assert tenant["tenantStatus"] == 2, tenant

assert enabled["code"] == 200, enabled
enabled_data = enabled["data"]
assert enabled_data["tenantId"] == tenant_id, enabled_data
assert enabled_data["previousStatus"] == 2, enabled_data
assert enabled_data["status"] == 1, enabled_data
assert enabled_data["operationId"] > disabled_data["operationId"], enabled_data
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"code":"scale","name":"规模版","description":"总后台规模套餐","status":1,"expectedVersion":0,"limits":{"maxUsers":300,"channelCodes":33,"asyncExecutions":1000}}' \
  "http://$GO_ADDR/dashboard/saasAdmin/package" >"$WORK_DIR/upsert-package.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/packages" >"$WORK_DIR/packages-after-upsert.json"

tenant_package_upsert_code="$(curl -s -o "$WORK_DIR/tenant-package-upsert-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"code":"tenant-only","name":"Tenant Only"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/package")"
test "$tenant_package_upsert_code" = "403"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"code":"scale","name":"规模版","description":"总后台规模套餐","status":2,"expectedVersion":1,"limits":{"maxUsers":300,"channelCodes":33,"asyncExecutions":1000}}' \
  "http://$GO_ADDR/dashboard/saasAdmin/package" >"$WORK_DIR/disable-package.json"

tenant_package_version="$(mysql_scalar "SELECT version FROM mochat_go_saas_tenant_packages WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL LIMIT 1")"
disabled_package_assign_code="$(curl -s -o "$WORK_DIR/disabled-package-assign.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"packageCode\":\"scale\",\"expectedVersion\":$tenant_package_version}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantPackage")"
test "$disabled_package_assign_code" = "400"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"code":"scale","name":"规模版","description":"总后台规模套餐","status":1,"expectedVersion":2,"limits":{"maxUsers":120,"channelCodes":33,"asyncExecutions":1000}}' \
  "http://$GO_ADDR/dashboard/saasAdmin/package" >"$WORK_DIR/enable-package.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/packages" >"$WORK_DIR/packages-after-enable.json"

tenant_package_version="$(mysql_scalar "SELECT version FROM mochat_go_saas_tenant_packages WHERE tenant_id = $TENANT_ID AND deleted_at IS NULL LIMIT 1")"
curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"packageCode\":\"scale\",\"expiresAt\":\"$PACKAGE_EXPIRES_ON\",\"expectedVersion\":$tenant_package_version}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantPackage" >"$WORK_DIR/update-package.json"

tenant_update_code="$(curl -s -o "$WORK_DIR/tenant-update-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$PLATFORM_TENANT_ID,\"packageCode\":\"growth\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantPackage")"
test "$tenant_update_code" = "403"

# tenantStatus 会按真实用户数刷新计数；这里重新注入超额场景，验证降额阻断与任务 SLA。
mysql_exec "UPDATE mochat_go_saas_usage_counters SET used_value = 8, updated_by = 'smoke-over-limit' WHERE tenant_id = $TENANT_ID AND metric = 'users' AND period_key = 'lifetime' AND deleted_at IS NULL"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=tenant&tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/platform-tenant-updated.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"code":"scale","name":"规模版","description":"总后台规模套餐","status":1,"expectedVersion":3,"limits":{"maxUsers":5,"channelCodes":33,"asyncExecutions":1000}}' \
  "http://$GO_ADDR/dashboard/saasAdmin/package" >"$WORK_DIR/impact-package.json"

tenant_package_sync_code="$(curl -s -o "$WORK_DIR/tenant-package-sync-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"packageCode":"scale"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSync")"
test "$tenant_package_sync_code" = "403"

tenant_package_sync_task_code="$(curl -s -o "$WORK_DIR/tenant-package-sync-task-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"packageCode":"scale"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask")"
test "$tenant_package_sync_task_code" = "403"

tenant_package_sync_task_bulk_apply_code="$(curl -s -o "$WORK_DIR/tenant-package-sync-task-bulk-apply-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskType":"package_sync","status":"pending"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTaskBulkApply")"
test "$tenant_package_sync_task_bulk_apply_code" = "403"

tenant_renewal_task_code="$(curl -s -o "$WORK_DIR/tenant-renewal-task-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"expiresAt\":\"$DIRECT_RENEWAL_EXPIRES_ON\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewalTask")"
test "$tenant_renewal_task_code" = "403"

tenant_renewal_task_bulk_apply_code="$(curl -s -o "$WORK_DIR/tenant-renewal-task-bulk-apply-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"taskType":"tenant_renewal","status":"pending"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewalTaskBulkApply")"
test "$tenant_renewal_task_bulk_apply_code" = "403"

tenant_customer_success_renewal_notifications_code="$(curl -s -o "$WORK_DIR/customer-success-renewal-notifications-forbidden.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_TOKEN" \
  -d '{"reminderDays":10}' \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessRenewalNotifications")"
test "$tenant_customer_success_renewal_notifications_code" = "403"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","dryRun":true,"limit":100}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSync" >"$WORK_DIR/package-sync-preview.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","allowOverLimit":false,"limit":100,"remark":"smoke-package-sync-blocked-task"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask" >"$WORK_DIR/package-sync-task-blocked.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","allowOverLimit":false,"limit":100,"remark":"smoke-reset-blocked-task"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask" >"$WORK_DIR/package-sync-task-reset-blocked.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","allowOverLimit":false,"limit":100,"remark":"smoke-bulk-reset-blocked-task"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask" >"$WORK_DIR/package-sync-task-bulk-reset-blocked.json"

mysql_exec "UPDATE mochat_go_saas_admin_tasks SET created_at = DATE_SUB(NOW(), INTERVAL 6 HOUR), updated_at = DATE_SUB(NOW(), INTERVAL 6 HOUR) WHERE task_type = 'package_sync' AND package_code = 'scale' AND status = 'blocked' AND remark IN ('smoke-package-sync-blocked-task', 'smoke-reset-blocked-task', 'smoke-bulk-reset-blocked-task')"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/taskSla?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10" >"$WORK_DIR/admin-task-sla-blocked.json"

python3 - "$WORK_DIR/admin-task-sla-blocked.json" <<'PY'
import json
import os
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
data = payload["data"]
assert data["filters"]["taskType"] == "package_sync", data["filters"]
assert data["filters"]["packageCode"] == "scale", data["filters"]
assert data["filters"]["warningHours"] == 4, data["filters"]
assert data["filters"]["overdueHours"] == 24, data["filters"]
summary = data["summary"]
task_summary = data["taskSummary"]
assert task_summary["blockedCount"] >= 3, task_summary
assert summary["taskCount"] >= 3, summary
assert summary["blockedCount"] >= 3, summary
assert summary["packageSyncCount"] >= 3, summary
assert summary["warningCount"] >= 3, summary
assert data["ownerCount"] >= 1, data
assert data["returnedCount"] == len(data["tasks"]), data
assert data["returnedOwners"] == len(data["owners"]), data
assert all(item["task"]["status"] in {"pending", "blocked", "failed"} for item in data["tasks"]), data["tasks"]
assert all(item["slaStatus"] in {"fresh", "warning", "overdue", "unknown"} for item in data["tasks"]), data["tasks"]
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=taskSla&taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=1000" >"$WORK_DIR/export-task-sla.csv"

python3 - "$WORK_DIR/export-task-sla.csv" <<'PY'
import csv
import pathlib
import sys

rows = list(csv.reader(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8-sig").splitlines()))
assert len(rows) >= 2, rows
header = rows[0]
assert header[:11] == ["taskId", "taskType", "status", "tenantId", "packageCode", "owner", "actorUserId", "actorTenantId", "slaStatus", "ageHours", "breachHours"], header
assert any(
    row[1] == "package_sync"
    and row[2] == "blocked"
    and row[4] == "scale"
    and row[8] in {"warning", "overdue"}
    for row in rows[1:]
), rows
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/dailyReport?date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/daily-report-after-task-sla.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=dailyReport&date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/export-daily-report-after-task-sla.csv"

python3 - "$WORK_DIR/daily-report-after-task-sla.json" "$WORK_DIR/export-daily-report-after-task-sla.csv" <<'PY'
import csv
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
daily = payload["data"]
summary = daily["summary"]
assert summary["taskSlaActiveCount"] >= 3, summary
assert summary["taskSlaWarningCount"] >= 3, summary
assert summary["taskSlaOwnerCount"] >= 1, summary
task_sla = daily["taskSla"]
assert task_sla["filters"]["warningHours"] == 4, task_sla["filters"]
assert task_sla["filters"]["overdueHours"] == 24, task_sla["filters"]
assert task_sla["summary"]["warningCount"] >= 3, task_sla["summary"]
assert any(
    item["task"]["taskType"] == "package_sync"
    and item["task"]["status"] == "blocked"
    and item["task"]["packageCode"] == "scale"
    and item["slaStatus"] in {"warning", "overdue"}
    for item in task_sla["tasks"]
), task_sla["tasks"]

rows = list(csv.reader(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8-sig").splitlines()))
assert any(row[:2] == ["summary", "taskSlaWarningCount"] for row in rows), rows
assert any(row[0] == "taskSlaOwner" for row in rows), rows
assert any(row[0] == "taskSlaTask" and row[1] in {"package_sync warning", "package_sync overdue"} for row in rows), rows
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"slaStatus":"warning","maxAttempts":4,"remark":"smoke-task-sla-notify"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskSlaNotifications?taskType=package_sync&packageCode=scale&warningHours=4&overdueHours=24&limit=10" >"$WORK_DIR/admin-task-sla-notifications.json"

python3 - "$WORK_DIR/admin-task-sla-notifications.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
data = payload["data"]
assert data["filters"]["taskType"] == "package_sync", data["filters"]
assert data["filters"]["packageCode"] == "scale", data["filters"]
assert data["slaStatus"] == "warning", data
assert data["channel"] == "webhook", data
assert data["maxAttempts"] == 4, data
assert data["taskSLANotificationKey"] == "admin_task_sla_reminder", data
assert data["matchedCount"] >= 3, data
assert data["eligibleCount"] >= 3, data
assert data["enqueuedCount"] >= 3, data
assert data["skippedExistingCount"] == 0, data
assert data["skippedInvalidCount"] == 0, data
notifications = data["notifications"]
assert len(notifications) >= 3, notifications
assert all(item["status"] == "pending" for item in notifications), notifications
assert all(item["metric"] == "admin_task_sla" for item in notifications), notifications
assert all(item["alertType"] == "admin_task_sla_reminder" for item in notifications), notifications
assert any("运营任务" in item["message"] for item in notifications), notifications
PY

package_sync_block_code="$(curl -s -o "$WORK_DIR/package-sync-blocked.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","dryRun":false,"allowOverLimit":false,"limit":100}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSync")"
test "$package_sync_block_code" = "400"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","dryRun":false,"allowOverLimit":true,"limit":100}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSync" >"$WORK_DIR/package-sync-applied.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"code":"scale","name":"规模版","description":"总后台规模套餐","status":1,"expectedVersion":4,"limits":{"maxUsers":120,"channelCodes":33,"asyncExecutions":1000}}' \
  "http://$GO_ADDR/dashboard/saasAdmin/package" >"$WORK_DIR/restore-package.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","allowOverLimit":false,"limit":100,"remark":"smoke-package-sync-restore-task"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask" >"$WORK_DIR/package-sync-task-created.json"

PACKAGE_SYNC_TASK_ID="$(python3 - "$WORK_DIR/package-sync-task-created.json" <<'PY'
import json
import pathlib
import sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"]["task"]["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskId\":$PACKAGE_SYNC_TASK_ID}" \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTaskApply" >"$WORK_DIR/package-sync-task-applied.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","allowOverLimit":false,"limit":100,"remark":"smoke-package-sync-bulk-apply-task"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask" >"$WORK_DIR/package-sync-task-bulk-apply-created.json"

PACKAGE_SYNC_BULK_APPLY_TASK_ID="$(python3 - "$WORK_DIR/package-sync-task-bulk-apply-created.json" <<'PY'
import json
import pathlib
import sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
task = payload["data"]["task"]
assert task["taskType"] == "package_sync", task
assert task["status"] == "pending", task
assert task["packageCode"] == "scale", task
print(int(task["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"taskType":"package_sync","status":"pending","packageCode":"scale","limit":20,"remark":"smoke-bulk-apply-package-sync-tasks"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTaskBulkApply" >"$WORK_DIR/package-sync-task-bulk-applied.json"

python3 - "$WORK_DIR/package-sync-task-bulk-apply-created.json" "$WORK_DIR/package-sync-task-bulk-applied.json" "$PACKAGE_SYNC_BULK_APPLY_TASK_ID" <<'PY'
import json
import pathlib
import sys

created_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
bulk_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
task_id = int(sys.argv[3])

assert created_payload["code"] == 200, created_payload
assert bulk_payload["code"] == 200, bulk_payload
data = bulk_payload["data"]
assert data["applied"] is True, data
assert data["appliedCount"] >= 1, data
assert data["failedCount"] == 0, data
filters = data["filters"]
assert filters["taskType"] == "package_sync", filters
assert filters["status"] == "pending", filters
assert filters["packageCode"] == "scale", filters
tasks = data["tasks"]
matched = [item for item in tasks if item["id"] == task_id]
assert matched and matched[0]["status"] == "applied", tasks
task = matched[0]
assert task["request"]["packageCode"] == "scale", task
result = task["result"]
assert result["packageCode"] == "scale", result
assert result["tenantSnapshotsUpdated"] is True, result
assert result["matchedTenantCount"] >= 1, result
assert result["syncedTenantCount"] >= 1, result
assert result["metricsRefreshed"] >= 26, result
assert data["errors"] == [], data
PY

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_tasks WHERE id = $PACKAGE_SYNC_BULK_APPLY_TASK_ID AND task_type = 'package_sync' AND status = 'applied' AND applied_at IS NOT NULL AND deleted_at IS NULL")" = "1"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskType=package_sync&packageCode=scale&limit=10" >"$WORK_DIR/package-sync-tasks.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","dryRun":false,"allowOverLimit":false,"limit":100}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSync" >"$WORK_DIR/package-sync-restored.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=50" >"$WORK_DIR/operations.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.package.update&targetType=saas_tenant_package&keyword=scale" >"$WORK_DIR/operations-filtered.json"

python3 - "$WORK_DIR/packages.json" "$WORK_DIR/upsert-package.json" "$WORK_DIR/packages-after-upsert.json" "$WORK_DIR/disable-package.json" "$WORK_DIR/disabled-package-assign.json" "$WORK_DIR/enable-package.json" "$WORK_DIR/packages-after-enable.json" "$WORK_DIR/update-package.json" "$WORK_DIR/platform-tenant-updated.json" "$WORK_DIR/impact-package.json" "$WORK_DIR/package-sync-preview.json" "$WORK_DIR/package-sync-task-blocked.json" "$WORK_DIR/package-sync-blocked.json" "$WORK_DIR/package-sync-applied.json" "$WORK_DIR/restore-package.json" "$WORK_DIR/package-sync-task-created.json" "$WORK_DIR/package-sync-task-applied.json" "$WORK_DIR/package-sync-tasks.json" "$WORK_DIR/package-sync-restored.json" "$WORK_DIR/operations.json" "$WORK_DIR/operations-filtered.json" "$TENANT_ID" <<'PY'
import json
import os
import pathlib
import sys

packages_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
upsert_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
packages_after_upsert_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
disable_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
disabled_assign_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
enable_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
packages_after_enable_payload = json.loads(pathlib.Path(sys.argv[7]).read_text(encoding="utf-8"))
update_payload = json.loads(pathlib.Path(sys.argv[8]).read_text(encoding="utf-8"))
overview_payload = json.loads(pathlib.Path(sys.argv[9]).read_text(encoding="utf-8"))
impact_payload = json.loads(pathlib.Path(sys.argv[10]).read_text(encoding="utf-8"))
package_sync_preview_payload = json.loads(pathlib.Path(sys.argv[11]).read_text(encoding="utf-8"))
package_sync_task_blocked_payload = json.loads(pathlib.Path(sys.argv[12]).read_text(encoding="utf-8"))
package_sync_blocked_payload = json.loads(pathlib.Path(sys.argv[13]).read_text(encoding="utf-8"))
package_sync_applied_payload = json.loads(pathlib.Path(sys.argv[14]).read_text(encoding="utf-8"))
restore_payload = json.loads(pathlib.Path(sys.argv[15]).read_text(encoding="utf-8"))
package_sync_task_created_payload = json.loads(pathlib.Path(sys.argv[16]).read_text(encoding="utf-8"))
package_sync_task_applied_payload = json.loads(pathlib.Path(sys.argv[17]).read_text(encoding="utf-8"))
package_sync_tasks_payload = json.loads(pathlib.Path(sys.argv[18]).read_text(encoding="utf-8"))
package_sync_restored_payload = json.loads(pathlib.Path(sys.argv[19]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[20]).read_text(encoding="utf-8"))
operations_filtered_payload = json.loads(pathlib.Path(sys.argv[21]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[22])

assert packages_payload["code"] == 200, packages_payload
packages = packages_payload["data"]["packages"]
codes = {item["code"] for item in packages}
assert {"platform", "growth"} <= codes, packages
platform = next(item for item in packages if item["code"] == "platform")
assert platform["limits"]["maxUsers"] == 100, platform
assert platform["limits"]["channelCodes"] == 500, platform

assert upsert_payload["code"] == 200, upsert_payload
upserted = upsert_payload["data"]
assert upserted["code"] == "scale", upserted
assert upserted["name"] == "规模版", upserted
assert upserted["description"] == "总后台规模套餐", upserted
assert upserted["status"] == 1, upserted
assert upserted["version"] == 1, upserted
assert upserted["limits"]["maxUsers"] == 300, upserted
assert upserted["limits"]["channelCodes"] == 33, upserted
assert upserted["limits"]["asyncExecutions"] == 1000, upserted

packages_after_upsert = packages_after_upsert_payload["data"]["packages"]
scale = next(item for item in packages_after_upsert if item["code"] == "scale")
assert scale["status"] == 1, scale
assert scale["version"] == 1, scale
assert scale["limits"]["maxUsers"] == 300, scale

assert disable_payload["code"] == 200, disable_payload
disabled = disable_payload["data"]
assert disabled["code"] == "scale", disabled
assert disabled["status"] == 2, disabled
assert disabled["version"] == 2, disabled
assert disabled_assign_payload["code"] == 400, disabled_assign_payload

assert enable_payload["code"] == 200, enable_payload
enabled = enable_payload["data"]
assert enabled["code"] == "scale", enabled
assert enabled["status"] == 1, enabled
assert enabled["version"] == 3, enabled
assert enabled["limits"]["maxUsers"] == 120, enabled

packages_after_enable = packages_after_enable_payload["data"]["packages"]
scale = next(item for item in packages_after_enable if item["code"] == "scale")
assert scale["status"] == 1, scale
assert scale["version"] == 3, scale
assert scale["limits"]["maxUsers"] == 120, scale
assert scale["limits"]["channelCodes"] == 33, scale

assert update_payload["code"] == 200, update_payload
updated = update_payload["data"]
assert updated["tenantId"] == tenant_id, updated
assert updated["packageCode"] == "scale", updated
assert updated["packageName"] == "规模版", updated
assert updated["expiresAt"] == os.environ["PACKAGE_EXPIRES_AT"], updated
assert updated["metricsRefreshed"] == 26, updated

assert overview_payload["code"] == 200, overview_payload
data = overview_payload["data"]
tenant = data["tenants"][0]
assert tenant["tenantId"] == tenant_id, tenant
assert tenant["packageCode"] == "scale", tenant
assert tenant["packageName"] == "规模版", tenant
metric = next(item for item in data["metrics"] if item["metric"] == "users")
assert metric["current"] == 8, metric
assert metric["limit"] == 120, metric

assert impact_payload["code"] == 200, impact_payload
assert impact_payload["data"]["version"] == 4, impact_payload
impact = impact_payload["data"]["impact"]
assert impact["existing"] is True, impact
assert impact["assignedTenantCount"] == 1, impact
assert impact["checkedTenantCount"] == 1, impact
assert impact["changedLimitCount"] == 1, impact
assert impact["decreasedLimitCount"] == 1, impact
assert impact["overLimitTenantCount"] == 1, impact
assert impact["tenantSnapshotsUpdated"] is False, impact
change = impact["changes"][0]
assert change["field"] == "maxUsers", change
assert change["metric"] == "users", change
assert change["direction"] == "decrease", change
assert change["before"] == 120 and change["after"] == 5, change
over_limit = impact["overLimitTenants"][0]
assert over_limit["tenantId"] == tenant_id, over_limit
assert over_limit["metric"] == "users", over_limit
assert over_limit["current"] == 8 and over_limit["limit"] == 5, over_limit

assert package_sync_preview_payload["code"] == 200, package_sync_preview_payload
sync_preview = package_sync_preview_payload["data"]
assert sync_preview["packageCode"] == "scale", sync_preview
assert sync_preview["dryRun"] is True, sync_preview
assert sync_preview["allowOverLimit"] is False, sync_preview
assert sync_preview["tenantSnapshotsUpdated"] is False, sync_preview
assert sync_preview["matchedTenantCount"] == 1, sync_preview
assert sync_preview["checkedTenantCount"] == 1, sync_preview
assert sync_preview["syncedTenantCount"] == 0, sync_preview
assert sync_preview["overLimitTenantCount"] == 1, sync_preview
preview_tenant = sync_preview["tenants"][0]
assert preview_tenant["tenantId"] == tenant_id, preview_tenant
assert preview_tenant["synced"] is False, preview_tenant
assert preview_tenant["overLimitMetrics"][0]["metric"] == "users", preview_tenant

assert package_sync_task_blocked_payload["code"] == 200, package_sync_task_blocked_payload
blocked_task = package_sync_task_blocked_payload["data"]["task"]
blocked_result = package_sync_task_blocked_payload["data"]["result"]
assert blocked_task["taskType"] == "package_sync", blocked_task
assert blocked_task["status"] == "blocked", blocked_task
assert blocked_task["packageCode"] == "scale", blocked_task
assert blocked_task["request"]["dryRun"] is False, blocked_task
assert blocked_task["request"]["allowOverLimit"] is False, blocked_task
assert blocked_result["blocked"] is True, blocked_result
assert blocked_result["overLimitTenantCount"] == 1, blocked_result
assert blocked_result["tenantSnapshotsUpdated"] is False, blocked_result

assert package_sync_blocked_payload["code"] == 400, package_sync_blocked_payload
sync_blocked = package_sync_blocked_payload["data"]
assert sync_blocked["blocked"] is True, sync_blocked
assert sync_blocked["tenantSnapshotsUpdated"] is False, sync_blocked
assert sync_blocked["syncedTenantCount"] == 0, sync_blocked
assert sync_blocked["overLimitTenantCount"] == 1, sync_blocked

assert package_sync_applied_payload["code"] == 200, package_sync_applied_payload
sync_applied = package_sync_applied_payload["data"]
assert sync_applied["dryRun"] is False, sync_applied
assert sync_applied["allowOverLimit"] is True, sync_applied
assert sync_applied["tenantSnapshotsUpdated"] is True, sync_applied
assert sync_applied["syncedTenantCount"] == 1, sync_applied
assert sync_applied["metricsRefreshed"] == 26, sync_applied
assert sync_applied["tenants"][0]["synced"] is True, sync_applied
assert sync_applied["tenants"][0]["metricsRefreshed"] == 26, sync_applied

assert restore_payload["code"] == 200, restore_payload
assert restore_payload["data"]["version"] == 5, restore_payload
restore_impact = restore_payload["data"]["impact"]
assert restore_impact["assignedTenantCount"] == 1, restore_impact
assert restore_impact["increasedLimitCount"] == 1, restore_impact
assert restore_impact["overLimitTenantCount"] == 0, restore_impact
assert restore_payload["data"]["limits"]["maxUsers"] == 120, restore_payload

assert package_sync_task_created_payload["code"] == 200, package_sync_task_created_payload
created_task = package_sync_task_created_payload["data"]["task"]
created_result = package_sync_task_created_payload["data"]["result"]
assert created_task["taskType"] == "package_sync", created_task
assert created_task["status"] == "pending", created_task
assert created_task["packageCode"] == "scale", created_task
assert created_task["request"]["dryRun"] is False, created_task
assert created_task["request"]["allowOverLimit"] is False, created_task
assert created_result["overLimitTenantCount"] == 0, created_result
assert created_result["tenantSnapshotsUpdated"] is False, created_result

assert package_sync_task_applied_payload["code"] == 200, package_sync_task_applied_payload
applied_task = package_sync_task_applied_payload["data"]["task"]
applied_result = package_sync_task_applied_payload["data"]["result"]
assert applied_task["id"] == created_task["id"], applied_task
assert applied_task["status"] == "applied", applied_task
assert applied_task["appliedAt"], applied_task
assert applied_result["dryRun"] is False, applied_result
assert applied_result["overLimitTenantCount"] == 0, applied_result
assert applied_result["tenantSnapshotsUpdated"] is True, applied_result
assert applied_result["syncedTenantCount"] == 1, applied_result
assert applied_result["metricsRefreshed"] == 26, applied_result

assert package_sync_tasks_payload["code"] == 200, package_sync_tasks_payload
task_items = package_sync_tasks_payload["data"]["tasks"]
task_status_by_id = {item["id"]: item["status"] for item in task_items}
assert created_task["id"] in task_status_by_id, task_items
assert task_status_by_id[created_task["id"]] == "applied", task_items
assert any(item["status"] == "blocked" and item["packageCode"] == "scale" for item in task_items), task_items

assert package_sync_restored_payload["code"] == 200, package_sync_restored_payload
sync_restored = package_sync_restored_payload["data"]
assert sync_restored["dryRun"] is False, sync_restored
assert sync_restored["allowOverLimit"] is False, sync_restored
assert sync_restored["overLimitTenantCount"] == 0, sync_restored
assert sync_restored["tenantSnapshotsUpdated"] is True, sync_restored
assert sync_restored["syncedTenantCount"] == 1, sync_restored
assert sync_restored["metricsRefreshed"] == 26, sync_restored

assert operations_payload["code"] == 200, operations_payload
operation_data = operations_payload["data"]
operations = operation_data["operations"]
operation_summary = operation_data["summary"]
assert operation_data["returnedCount"] == len(operations), operation_data
assert operation_summary["operationCount"] >= len(operations), operation_summary
assert operation_summary["tenantCount"] >= 1, operation_summary
assert operation_summary["actorUserCount"] >= 1, operation_summary
assert operation_summary["actionCount"] >= 3, operation_summary
actions = [item["action"] for item in operations]
assert "tenant.provision" in actions, operations
assert "package.upsert" in actions, operations
assert "tenant.package.update" in actions, operations
assert actions.count("tenant.status") >= 2, operations
status_ops = [item for item in operations if item["action"] == "tenant.status" and item["tenantId"] == tenant_id]
assert any(item["after"]["status"] == 2 for item in status_ops), status_ops
assert any(item["after"]["status"] == 1 for item in status_ops), status_ops
package_op = next(item for item in operations if item["action"] == "tenant.package.update" and item["tenantId"] == tenant_id)
assert package_op["after"]["packageCode"] == "scale", package_op

assert operations_filtered_payload["code"] == 200, operations_filtered_payload
filtered = operations_filtered_payload["data"]
assert filtered["filters"]["tenantId"] == tenant_id, filtered
assert filtered["filters"]["action"] == "tenant.package.update", filtered
assert filtered["filters"]["targetType"] == "saas_tenant_package", filtered
assert filtered["filters"]["keyword"] == "scale", filtered
filtered_ops = filtered["operations"]
filtered_summary = filtered["summary"]
assert filtered["returnedCount"] == len(filtered_ops), filtered
assert filtered_summary["operationCount"] >= len(filtered_ops), filtered_summary
assert filtered_summary["tenantCount"] == 1, filtered_summary
assert filtered_summary["actionCount"] == 1, filtered_summary
assert filtered_summary["targetTypeCount"] == 1, filtered_summary
assert filtered_ops, filtered
assert {item["action"] for item in filtered_ops} == {"tenant.package.update"}, filtered_ops
assert all(item["tenantId"] == tenant_id for item in filtered_ops), filtered_ops
assert any(item["after"]["packageCode"] == "scale" for item in filtered_ops), filtered_ops
PY

PACKAGE_SYNC_BLOCKED_TASK_ID="$(python3 - "$WORK_DIR/package-sync-task-blocked.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"]["task"]["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskId\":$PACKAGE_SYNC_BLOCKED_TASK_ID,\"remark\":\"smoke-cancel-blocked-task\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/taskCancel" >"$WORK_DIR/package-sync-task-canceled.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskType=package_sync&status=canceled&packageCode=scale&limit=20" >"$WORK_DIR/package-sync-tasks-canceled.json"

package_sync_task_canceled_apply_code="$(curl -s -o "$WORK_DIR/package-sync-task-canceled-apply.json" -w '%{http_code}' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskId\":$PACKAGE_SYNC_BLOCKED_TASK_ID}" \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTaskApply")"
test "$package_sync_task_canceled_apply_code" = "400"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&action=saas.admin.task.cancel&targetType=admin_task&keyword=$PACKAGE_SYNC_BLOCKED_TASK_ID" >"$WORK_DIR/operations-task-cancel.json"

python3 - "$WORK_DIR/package-sync-task-canceled.json" "$WORK_DIR/package-sync-tasks-canceled.json" "$WORK_DIR/package-sync-task-canceled-apply.json" "$WORK_DIR/operations-task-cancel.json" "$PACKAGE_SYNC_BLOCKED_TASK_ID" <<'PY'
import json
import pathlib
import sys

canceled_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
apply_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
task_id = int(sys.argv[5])

assert canceled_payload["code"] == 200, canceled_payload
task = canceled_payload["data"]["task"]
result = canceled_payload["data"]["result"]
assert task["id"] == task_id, task
assert task["status"] == "canceled", task
assert task["canApply"] is False, task
assert task["canCancel"] is False, task
assert result["canceled"] is True, result
assert result["remark"] == "smoke-cancel-blocked-task", result

assert tasks_payload["code"] == 200, tasks_payload
filters = tasks_payload["data"]["filters"]
assert filters["taskType"] == "package_sync", filters
assert filters["status"] == "canceled", filters
matched = [item for item in tasks_payload["data"]["tasks"] if item["id"] == task_id]
assert matched and matched[0]["status"] == "canceled", tasks_payload

assert apply_payload["code"] == 400, apply_payload
assert apply_payload["msg"] == "task canceled", apply_payload

assert operations_payload["code"] == 200, operations_payload
operation_data = operations_payload["data"]
assert operation_data["filters"]["action"] == "saas.admin.task.cancel", operation_data
assert operation_data["filters"]["targetType"] == "admin_task", operation_data
operations = operation_data["operations"]
operation = next(item for item in operations if item["targetId"] == str(task_id))
assert operation["action"] == "saas.admin.task.cancel", operation
assert operation["targetType"] == "admin_task", operation
assert operation["before"]["status"] == "blocked", operation
assert operation["before"]["canCancel"] is True, operation
assert operation["after"]["status"] == "canceled", operation
assert operation["after"]["canApply"] is False, operation
assert operation["after"]["canCancel"] is False, operation
assert operation["remark"] == "smoke-cancel-blocked-task", operation
PY

PACKAGE_SYNC_RESET_BLOCKED_TASK_ID="$(python3 - "$WORK_DIR/package-sync-task-reset-blocked.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"]["task"]["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskId\":$PACKAGE_SYNC_RESET_BLOCKED_TASK_ID,\"remark\":\"smoke-reset-blocked-task\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/taskReset" >"$WORK_DIR/package-sync-task-reset.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskId=$PACKAGE_SYNC_RESET_BLOCKED_TASK_ID&limit=1" >"$WORK_DIR/package-sync-task-reset-pending.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&action=saas.admin.task.reset&targetType=admin_task&keyword=$PACKAGE_SYNC_RESET_BLOCKED_TASK_ID" >"$WORK_DIR/operations-task-reset.json"

python3 - "$WORK_DIR/package-sync-task-reset.json" "$WORK_DIR/package-sync-task-reset-pending.json" "$WORK_DIR/operations-task-reset.json" "$PACKAGE_SYNC_RESET_BLOCKED_TASK_ID" <<'PY'
import json
import pathlib
import sys

reset_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
task_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
task_id = int(sys.argv[4])

assert reset_payload["code"] == 200, reset_payload
task = reset_payload["data"]["task"]
result = reset_payload["data"]["result"]
assert task["id"] == task_id, task
assert task["status"] == "pending", task
assert task["lastError"] == "", task
assert task["canApply"] is True, task
assert task["canCancel"] is True, task
assert result["reset"] is True, result
assert result["previousStatus"] == "blocked", result
assert result["remark"] == "smoke-reset-blocked-task", result

assert task_payload["code"] == 200, task_payload
items = task_payload["data"]["tasks"]
assert len(items) == 1, items
assert items[0]["id"] == task_id, items
assert items[0]["status"] == "pending", items[0]

assert operations_payload["code"] == 200, operations_payload
operation_data = operations_payload["data"]
assert operation_data["filters"]["action"] == "saas.admin.task.reset", operation_data
assert operation_data["filters"]["targetType"] == "admin_task", operation_data
operation = next(item for item in operation_data["operations"] if item["targetId"] == str(task_id))
assert operation["action"] == "saas.admin.task.reset", operation
assert operation["before"]["status"] == "blocked", operation
assert operation["after"]["status"] == "pending", operation
assert operation["after"]["reset"] is True, operation
assert operation["after"]["previousStatus"] == "blocked", operation
assert operation["remark"] == "smoke-reset-blocked-task", operation
PY

PACKAGE_SYNC_BULK_RESET_BLOCKED_TASK_ID="$(python3 - "$WORK_DIR/package-sync-task-bulk-reset-blocked.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"]["task"]["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"taskType":"package_sync","status":"all","packageCode":"scale","limit":20,"remark":"smoke-bulk-reset-tasks"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskBulkReset" >"$WORK_DIR/package-sync-task-bulk-reset.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskId=$PACKAGE_SYNC_BULK_RESET_BLOCKED_TASK_ID&limit=1" >"$WORK_DIR/package-sync-task-bulk-reset-pending.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&action=saas.admin.task.bulk_reset&targetType=admin_task&keyword=$PACKAGE_SYNC_BULK_RESET_BLOCKED_TASK_ID" >"$WORK_DIR/operations-task-bulk-reset.json"

python3 - "$WORK_DIR/package-sync-task-bulk-reset.json" "$WORK_DIR/package-sync-task-bulk-reset-pending.json" "$WORK_DIR/operations-task-bulk-reset.json" "$PACKAGE_SYNC_BULK_RESET_BLOCKED_TASK_ID" <<'PY'
import json
import pathlib
import sys

bulk_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
task_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
task_id = int(sys.argv[4])

assert bulk_payload["code"] == 200, bulk_payload
data = bulk_payload["data"]
assert data["filters"]["taskType"] == "package_sync", data
assert data["filters"]["packageCode"] == "scale", data
assert data["filters"]["limit"] == 20, data
assert data["reset"] is True, data
assert data["resetCount"] >= 1, data
assert data["skippedPendingCount"] >= 1, data
assert data["skippedAppliedCount"] >= 1, data
assert data["skippedCanceledCount"] >= 1, data
assert any(item["id"] == task_id and item["status"] == "pending" for item in data["tasks"]), data

assert task_payload["code"] == 200, task_payload
items = task_payload["data"]["tasks"]
assert len(items) == 1, items
assert items[0]["id"] == task_id, items[0]
assert items[0]["status"] == "pending", items[0]
assert items[0]["lastError"] == "", items[0]
assert items[0]["canApply"] is True, items[0]
assert items[0]["canCancel"] is True, items[0]

assert operations_payload["code"] == 200, operations_payload
operation_data = operations_payload["data"]
assert operation_data["filters"]["action"] == "saas.admin.task.bulk_reset", operation_data
assert operation_data["filters"]["targetType"] == "admin_task", operation_data
operation = next(item for item in operation_data["operations"] if item["targetId"] == str(task_id))
assert operation["action"] == "saas.admin.task.bulk_reset", operation
assert operation["targetType"] == "admin_task", operation
assert operation["before"]["status"] == "blocked", operation
assert operation["after"]["status"] == "pending", operation
assert operation["after"]["reset"] is True, operation
assert operation["after"]["bulkReset"] is True, operation
assert operation["after"]["previousStatus"] == "blocked", operation
assert operation["remark"] == "smoke-bulk-reset-tasks", operation
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"packageCode":"scale","dryRun":false,"allowOverLimit":false,"limit":100}' \
  "http://$GO_ADDR/dashboard/saasAdmin/packageSyncTask" >"$WORK_DIR/package-sync-task-bulk-pending.json"

PACKAGE_SYNC_BULK_PENDING_TASK_ID="$(python3 - "$WORK_DIR/package-sync-task-bulk-pending.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"]["task"]["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"taskType":"package_sync","status":"all","packageCode":"scale","limit":20,"remark":"smoke-bulk-cancel-tasks"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/taskBulkCancel" >"$WORK_DIR/package-sync-task-bulk-cancel.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskType=package_sync&status=canceled&packageCode=scale&limit=20" >"$WORK_DIR/package-sync-tasks-bulk-canceled.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&action=saas.admin.task.bulk_cancel&targetType=admin_task&keyword=$PACKAGE_SYNC_BULK_PENDING_TASK_ID" >"$WORK_DIR/operations-task-bulk-cancel.json"

python3 - "$WORK_DIR/package-sync-task-bulk-pending.json" "$WORK_DIR/package-sync-task-bulk-cancel.json" "$WORK_DIR/package-sync-tasks-bulk-canceled.json" "$WORK_DIR/operations-task-bulk-cancel.json" "$PACKAGE_SYNC_BULK_PENDING_TASK_ID" <<'PY'
import json
import pathlib
import sys

pending_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
bulk_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
canceled_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
task_id = int(sys.argv[5])

assert pending_payload["code"] == 200, pending_payload
pending_task = pending_payload["data"]["task"]
assert pending_task["id"] == task_id, pending_task
assert pending_task["status"] == "pending", pending_task

assert bulk_payload["code"] == 200, bulk_payload
data = bulk_payload["data"]
assert data["filters"]["taskType"] == "package_sync", data
assert data["filters"]["packageCode"] == "scale", data
assert data["filters"]["limit"] == 20, data
assert data["canceledCount"] >= 1, data
assert data["skippedAppliedCount"] >= 1, data
assert data["skippedCanceledCount"] >= 1, data
assert any(item["id"] == task_id and item["status"] == "canceled" for item in data["tasks"]), data

assert canceled_payload["code"] == 200, canceled_payload
assert any(item["id"] == task_id and item["status"] == "canceled" for item in canceled_payload["data"]["tasks"]), canceled_payload

assert operations_payload["code"] == 200, operations_payload
operation_data = operations_payload["data"]
assert operation_data["filters"]["action"] == "saas.admin.task.bulk_cancel", operation_data
assert operation_data["filters"]["targetType"] == "admin_task", operation_data
operations = operation_data["operations"]
operation = next(item for item in operations if item["targetId"] == str(task_id))
assert operation["action"] == "saas.admin.task.bulk_cancel", operation
assert operation["targetType"] == "admin_task", operation
assert operation["before"]["status"] == "pending", operation
assert operation["before"]["canCancel"] is True, operation
assert operation["after"]["status"] == "canceled", operation
assert operation["after"]["canApply"] is False, operation
assert operation["after"]["canCancel"] is False, operation
assert operation["after"]["bulkCancel"] is True, operation
assert operation["remark"] == "smoke-bulk-cancel-tasks", operation
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"packageCode\":\"scale\",\"expiresAt\":\"$DIRECT_RENEWAL_EXPIRES_ON\",\"amountCents\":1280050,\"currency\":\"CNY\",\"paidAt\":\"$PAID_ON\",\"paymentMethod\":\"bank\",\"externalOrderNo\":\"SMOKE-RENEWAL-$TENANT_ID\",\"remark\":\"smoke-renewal\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewal" >"$WORK_DIR/tenant-renewal.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingEvents?tenantId=$TENANT_ID&limit=20" >"$WORK_DIR/billing-events.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingEvents?tenantId=$TENANT_ID&limit=20&eventType=renewal&packageCode=scale&keyword=SMOKE-RENEWAL-$TENANT_ID" >"$WORK_DIR/billing-events-filtered.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliation?tenantId=$TENANT_ID&limit=20&eventType=renewal&packageCode=scale&keyword=SMOKE-RENEWAL-$TENANT_ID" >"$WORK_DIR/billing-reconciliation-matched.json"

mysql_exec "INSERT INTO mochat_go_saas_billing_events (tenant_id, event_type, package_code, package_name, previous_expires_at, new_expires_at, amount_cents, currency, paid_at, payment_method, external_order_no, actor_user_id, actor_tenant_id, remark, metadata_json, created_at, updated_at, deleted_at) VALUES ($TENANT_ID, 'renewal', 'growth', '增长版', '$DIRECT_RENEWAL_EXPIRES_AT', '2031-02-03 00:00:00', 880000, 'CNY', '$PAID_ON 00:00:00', 'manual', 'SMOKE-RECONCILE-DRIFT-$TENANT_ID', $PLATFORM_USER_ID, $PLATFORM_TENANT_ID, 'smoke-reconcile-drift', JSON_OBJECT('source', 'smoke'), NOW(), NOW(), NULL)"
DRIFT_BILLING_EVENT_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_billing_events WHERE tenant_id = $TENANT_ID AND external_order_no = 'SMOKE-RECONCILE-DRIFT-$TENANT_ID' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliation?tenantId=$TENANT_ID&limit=20&keyword=SMOKE-RECONCILE-DRIFT-$TENANT_ID&mismatchOnly=1" >"$WORK_DIR/billing-reconciliation-mismatch.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"billingEventId\":$DRIFT_BILLING_EVENT_ID,\"status\":\"resolved\",\"owner\":\"finance\",\"nextFollowUpAt\":\"2026-07-10\",\"remark\":\"smoke-reconciliation-follow-up\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUp" >"$WORK_DIR/billing-reconciliation-follow-up.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUps?tenantId=$TENANT_ID&status=resolved&owner=finance&keyword=SMOKE-RECONCILE-DRIFT-$TENANT_ID&dueState=closed&limit=20" >"$WORK_DIR/billing-reconciliation-follow-ups.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUpOwners?tenantId=$TENANT_ID&owner=finance&limit=1000" >"$WORK_DIR/billing-reconciliation-follow-up-owners.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=tenant&tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/platform-tenant-renewed.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=80" >"$WORK_DIR/operations-after-renewal.json"

python3 - "$WORK_DIR/tenant-renewal.json" "$WORK_DIR/billing-events.json" "$WORK_DIR/billing-events-filtered.json" "$WORK_DIR/billing-reconciliation-matched.json" "$WORK_DIR/billing-reconciliation-mismatch.json" "$WORK_DIR/billing-reconciliation-follow-up.json" "$WORK_DIR/billing-reconciliation-follow-ups.json" "$WORK_DIR/billing-reconciliation-follow-up-owners.json" "$WORK_DIR/platform-tenant-renewed.json" "$WORK_DIR/operations-after-renewal.json" "$TENANT_ID" "$DRIFT_BILLING_EVENT_ID" <<'PY'
import json
import os
import pathlib
import sys

renewal_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
billing_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
billing_filtered_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
reconcile_matched_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
reconcile_mismatch_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
follow_up_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
follow_ups_payload = json.loads(pathlib.Path(sys.argv[7]).read_text(encoding="utf-8"))
follow_up_owners_payload = json.loads(pathlib.Path(sys.argv[8]).read_text(encoding="utf-8"))
overview_payload = json.loads(pathlib.Path(sys.argv[9]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[10]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[11])
drift_billing_event_id = int(sys.argv[12])

assert renewal_payload["code"] == 200, renewal_payload
renewal = renewal_payload["data"]
assert renewal["tenantId"] == tenant_id, renewal
assert renewal["packageCode"] == "scale", renewal
assert renewal["previousExpiresAt"] == os.environ["PACKAGE_EXPIRES_AT"], renewal
assert renewal["expiresAt"] == os.environ["DIRECT_RENEWAL_EXPIRES_AT"], renewal
assert renewal["amountCents"] == 1280050, renewal
assert renewal["currency"] == "CNY", renewal
assert renewal["billingEventId"] > 0, renewal
assert renewal["metricsRefreshed"] == 26, renewal

assert billing_payload["code"] == 200, billing_payload
summary = billing_payload["data"]["summary"]
assert summary["eventCount"] >= 1, summary
assert summary["renewalCount"] >= 1, summary
assert summary["amountCents"] >= renewal["amountCents"], summary
assert billing_payload["data"]["returnedCount"] == len(billing_payload["data"]["billingEvents"]), billing_payload
events = billing_payload["data"]["billingEvents"]
assert len(events) >= 1, events
event = events[0]
assert event["id"] == renewal["billingEventId"], event
assert event["tenantId"] == tenant_id, event
assert event["eventType"] == "renewal", event
assert event["packageCode"] == "scale", event
assert event["previousExpiresAt"] == os.environ["PACKAGE_EXPIRES_AT"], event
assert event["newExpiresAt"] == os.environ["DIRECT_RENEWAL_EXPIRES_AT"], event
assert event["amountCents"] == 1280050, event
assert event["externalOrderNo"] == f"SMOKE-RENEWAL-{tenant_id}", event

assert billing_filtered_payload["code"] == 200, billing_filtered_payload
filtered = billing_filtered_payload["data"]
assert filtered["filters"]["tenantId"] == tenant_id, filtered
assert filtered["filters"]["eventType"] == "renewal", filtered
assert filtered["filters"]["packageCode"] == "scale", filtered
assert filtered["filters"]["keyword"] == f"SMOKE-RENEWAL-{tenant_id}", filtered
filtered_summary = filtered["summary"]
assert filtered_summary["eventCount"] == 1, filtered_summary
assert filtered_summary["renewalCount"] == 1, filtered_summary
assert filtered_summary["amountCents"] == renewal["amountCents"], filtered_summary
assert filtered["returnedCount"] == 1, filtered
filtered_events = filtered["billingEvents"]
assert len(filtered_events) == 1, filtered_events
assert filtered_events[0]["id"] == renewal["billingEventId"], filtered_events

assert reconcile_matched_payload["code"] == 200, reconcile_matched_payload
reconcile_matched = reconcile_matched_payload["data"]
matched_summary = reconcile_matched["summary"]
assert matched_summary["checkedCount"] == 1, matched_summary
assert matched_summary["matchedCount"] == 1, matched_summary
assert matched_summary["mismatchedCount"] == 0, matched_summary
assert reconcile_matched["returnedCount"] == 1, reconcile_matched
matched_item = reconcile_matched["items"][0]
assert matched_item["id"] == renewal["billingEventId"], matched_item
assert matched_item["reconcileStatus"] == "matched", matched_item
assert matched_item["currentPackageFound"] is True, matched_item
assert matched_item["currentPackageCode"] == "scale", matched_item
assert matched_item["currentExpiresAt"] == os.environ["DIRECT_RENEWAL_EXPIRES_AT"], matched_item
assert matched_item["mismatchReasons"] == [], matched_item

assert reconcile_mismatch_payload["code"] == 200, reconcile_mismatch_payload
reconcile_mismatch = reconcile_mismatch_payload["data"]
mismatch_summary = reconcile_mismatch["summary"]
assert mismatch_summary["checkedCount"] == 1, mismatch_summary
assert mismatch_summary["matchedCount"] == 0, mismatch_summary
assert mismatch_summary["mismatchedCount"] == 1, mismatch_summary
assert mismatch_summary["packageMismatchCount"] == 1, mismatch_summary
assert mismatch_summary["expiresMismatchCount"] == 1, mismatch_summary
assert reconcile_mismatch["filters"]["mismatchOnly"] is True, reconcile_mismatch
assert reconcile_mismatch["returnedCount"] == 1, reconcile_mismatch
mismatch_item = reconcile_mismatch["items"][0]
assert mismatch_item["externalOrderNo"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", mismatch_item
assert mismatch_item["reconcileStatus"] == "mismatch", mismatch_item
assert set(mismatch_item["mismatchReasons"]) == {"package_mismatch", "expires_mismatch"}, mismatch_item
assert mismatch_item["currentPackageCode"] == "scale", mismatch_item

assert follow_up_payload["code"] == 200, follow_up_payload
follow_up = follow_up_payload["data"]
assert follow_up["operationId"] > 0, follow_up
assert follow_up["billingEventId"] == drift_billing_event_id, follow_up
assert follow_up["tenantId"] == tenant_id, follow_up
assert follow_up["status"] == "resolved", follow_up
assert follow_up["owner"] == "finance", follow_up
assert follow_up["nextFollowUpAt"] == "2026-07-10 00:00:00", follow_up
assert follow_up["remark"] == "smoke-reconciliation-follow-up", follow_up
assert follow_up["billingEvent"]["externalOrderNo"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", follow_up

assert follow_ups_payload["code"] == 200, follow_ups_payload
follow_ups = follow_ups_payload["data"]
follow_filters = follow_ups["filters"]
assert follow_filters["tenantId"] == tenant_id, follow_filters
assert follow_filters["status"] == "resolved", follow_filters
assert follow_filters["owner"] == "finance", follow_filters
assert follow_filters["keyword"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", follow_filters
assert follow_filters["dueState"] == "closed", follow_filters
assert follow_ups["summary"]["totalCount"] == 1, follow_ups["summary"]
assert follow_ups["summary"]["resolvedCount"] == 1, follow_ups["summary"]
assert follow_ups["summary"]["closedCount"] == 1, follow_ups["summary"]
assert len(follow_ups["followUps"]) == 1, follow_ups
follow_task = follow_ups["followUps"][0]
assert follow_task["operationId"] == follow_up["operationId"], follow_task
assert follow_task["billingEventId"] == drift_billing_event_id, follow_task
assert follow_task["tenantId"] == tenant_id, follow_task
assert follow_task["status"] == "resolved", follow_task
assert follow_task["owner"] == "finance", follow_task
assert follow_task["dueState"] == "closed", follow_task
assert follow_task["overdue"] is False, follow_task
assert follow_task["nextFollowUpAt"] == "2026-07-10 00:00:00", follow_task
assert follow_task["remark"] == "smoke-reconciliation-follow-up", follow_task
assert follow_task["packageCode"] == "growth", follow_task
assert follow_task["amountCents"] == 880000, follow_task
assert follow_task["externalOrderNo"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", follow_task

assert follow_up_owners_payload["code"] == 200, follow_up_owners_payload
owners_data = follow_up_owners_payload["data"]
assert owners_data["summary"]["totalCount"] >= 1, owners_data["summary"]
owners = owners_data["owners"]
owner = next(item for item in owners if item["owner"] == "finance")
assert owner["totalCount"] >= 1, owner
assert owner["resolvedCount"] >= 1, owner
assert owner["closedCount"] >= 1, owner
assert owner["latestFollowUpAt"], owner

assert overview_payload["code"] == 200, overview_payload
tenant = overview_payload["data"]["tenants"][0]
assert tenant["tenantId"] == tenant_id, tenant
assert tenant["expiresAt"] == os.environ["DIRECT_RENEWAL_EXPIRES_AT"], tenant

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
renewal_ops = [item for item in operations if item["action"] == "tenant.renewal" and item["tenantId"] == tenant_id]
assert renewal_ops, operations
assert renewal_ops[0]["after"]["billingEventId"] == renewal["billingEventId"], renewal_ops[0]
assert renewal_ops[0]["after"]["amountCents"] == 1280050, renewal_ops[0]
follow_up_ops = [item for item in operations if item["action"] == "billing.reconciliation.follow_up" and item["targetType"] == "billing_event" and item["targetId"] == str(drift_billing_event_id)]
assert follow_up_ops, operations
follow_up_op = follow_up_ops[0]
assert follow_up_op["tenantId"] == tenant_id, follow_up_op
assert follow_up_op["targetName"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", follow_up_op
assert follow_up_op["remark"] == "smoke-reconciliation-follow-up", follow_up_op
assert follow_up_op["before"]["externalOrderNo"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", follow_up_op
assert follow_up_op["after"]["billingEventId"] == drift_billing_event_id, follow_up_op
assert follow_up_op["after"]["status"] == "resolved", follow_up_op
assert follow_up_op["after"]["owner"] == "finance", follow_up_op
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"billingEventId\":$DRIFT_BILLING_EVENT_ID,\"status\":\"contacted\",\"owner\":\"finance\",\"nextFollowUpAt\":\"2037-01-01\",\"remark\":\"smoke-reconciliation-bulk-open\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUp" >"$WORK_DIR/billing-reconciliation-bulk-open.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"filterStatus\":\"contacted\",\"owner\":\"finance\",\"keyword\":\"smoke-reconciliation-bulk-open\",\"dueState\":\"future\",\"limit\":10,\"closeStatus\":\"resolved\",\"remark\":\"smoke-reconciliation-bulk-close\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose" >"$WORK_DIR/billing-reconciliation-follow-up-bulk-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingReconciliationFollowUps?tenantId=$TENANT_ID&status=resolved&owner=finance&keyword=smoke-reconciliation-bulk-close&dueState=closed&limit=20" >"$WORK_DIR/billing-reconciliation-follow-ups-bulk-closed.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=80&tenantId=$TENANT_ID&action=billing.reconciliation.follow_up&targetType=billing_event&keyword=smoke-reconciliation-bulk-close" >"$WORK_DIR/operations-billing-follow-up-bulk-close.json"

python3 - "$WORK_DIR/billing-reconciliation-bulk-open.json" "$WORK_DIR/billing-reconciliation-follow-up-bulk-close.json" "$WORK_DIR/billing-reconciliation-follow-ups-bulk-closed.json" "$WORK_DIR/operations-billing-follow-up-bulk-close.json" "$TENANT_ID" "$DRIFT_BILLING_EVENT_ID" <<'PY'
import json
import pathlib
import sys

open_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
bulk_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
closed_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[5])
drift_billing_event_id = int(sys.argv[6])

assert open_payload["code"] == 200, open_payload
opened = open_payload["data"]
assert opened["operationId"] > 0, opened
assert opened["billingEventId"] == drift_billing_event_id, opened
assert opened["tenantId"] == tenant_id, opened
assert opened["status"] == "contacted", opened
assert opened["owner"] == "finance", opened
assert opened["nextFollowUpAt"] == "2037-01-01 00:00:00", opened
assert opened["remark"] == "smoke-reconciliation-bulk-open", opened

assert bulk_payload["code"] == 200, bulk_payload
bulk = bulk_payload["data"]
assert bulk["closedCount"] == 1, bulk
assert bulk["status"] == "resolved", bulk
assert bulk["remark"] == "smoke-reconciliation-bulk-close", bulk
assert len(bulk["followUps"]) == 1, bulk
bulk_follow_up = bulk["followUps"][0]
assert bulk_follow_up["operationId"] > opened["operationId"], bulk_follow_up
assert bulk_follow_up["billingEventId"] == drift_billing_event_id, bulk_follow_up
assert bulk_follow_up["tenantId"] == tenant_id, bulk_follow_up
assert bulk_follow_up["status"] == "resolved", bulk_follow_up
assert bulk_follow_up["owner"] == "finance", bulk_follow_up
assert bulk_follow_up["nextFollowUpAt"] == "", bulk_follow_up
assert bulk_follow_up["remark"] == "smoke-reconciliation-bulk-close", bulk_follow_up

assert closed_payload["code"] == 200, closed_payload
closed = closed_payload["data"]
assert closed["filters"]["tenantId"] == tenant_id, closed["filters"]
assert closed["filters"]["status"] == "resolved", closed["filters"]
assert closed["filters"]["owner"] == "finance", closed["filters"]
assert closed["filters"]["keyword"] == "smoke-reconciliation-bulk-close", closed["filters"]
assert closed["filters"]["dueState"] == "closed", closed["filters"]
assert closed["summary"]["totalCount"] == 1, closed["summary"]
assert closed["summary"]["resolvedCount"] == 1, closed["summary"]
assert closed["summary"]["closedCount"] == 1, closed["summary"]
assert len(closed["followUps"]) == 1, closed
closed_task = closed["followUps"][0]
assert closed_task["operationId"] == bulk_follow_up["operationId"], closed_task
assert closed_task["billingEventId"] == drift_billing_event_id, closed_task
assert closed_task["tenantId"] == tenant_id, closed_task
assert closed_task["status"] == "resolved", closed_task
assert closed_task["owner"] == "finance", closed_task
assert closed_task["remark"] == "smoke-reconciliation-bulk-close", closed_task
assert closed_task["dueState"] == "closed", closed_task
assert closed_task["externalOrderNo"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", closed_task

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert operations_payload["data"]["summary"]["operationCount"] == 1, operations_payload["data"]["summary"]
assert len(operations) == 1, operations
operation = operations[0]
assert operation["id"] == bulk_follow_up["operationId"], operation
assert operation["tenantId"] == tenant_id, operation
assert operation["action"] == "billing.reconciliation.follow_up", operation
assert operation["targetType"] == "billing_event", operation
assert operation["targetId"] == str(drift_billing_event_id), operation
assert operation["targetName"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", operation
assert operation["remark"] == "smoke-reconciliation-bulk-close", operation
assert operation["before"]["externalOrderNo"] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", operation
assert operation["after"]["billingEventId"] == drift_billing_event_id, operation
assert operation["after"]["status"] == "resolved", operation
assert operation["after"]["owner"] == "finance", operation
assert operation["after"]["remark"] == "smoke-reconciliation-bulk-close", operation
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"packageCode\":\"scale\",\"expiresAt\":\"$TASK_RENEWAL_EXPIRES_ON\",\"amountCents\":2380050,\"currency\":\"CNY\",\"paidAt\":\"$PAID_ON\",\"paymentMethod\":\"bank\",\"externalOrderNo\":\"SMOKE-RENEWAL-TASK-$TENANT_ID\",\"remark\":\"smoke-renewal-task\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewalTask" >"$WORK_DIR/tenant-renewal-task-created.json"

TENANT_RENEWAL_TASK_ID="$(python3 - "$WORK_DIR/tenant-renewal-task-created.json" <<'PY'
import json
import pathlib
import sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"]["task"]["id"]))
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskId\":$TENANT_RENEWAL_TASK_ID}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewalTaskApply" >"$WORK_DIR/tenant-renewal-task-applied.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskType=tenant_renewal&tenantId=$TENANT_ID&packageCode=scale&limit=10" >"$WORK_DIR/tenant-renewal-tasks.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/billingEvents?tenantId=$TENANT_ID&limit=20&eventType=renewal&packageCode=scale&keyword=SMOKE-RENEWAL-TASK-$TENANT_ID" >"$WORK_DIR/billing-events-renewal-task.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/overview?scope=tenant&tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/platform-tenant-renewed-by-task.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/businessMetrics?tenantLimit=500&expiringDays=10&highUsageRatio=0.8&billingLimit=1000" >"$WORK_DIR/business-metrics.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/businessTrends?months=6&billingLimit=1000&taskLimit=1000" >"$WORK_DIR/business-trends.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=businessMetrics&tenantLimit=500&expiringDays=10&highUsageRatio=0.8&billingLimit=1000" >"$WORK_DIR/export-business-metrics.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=businessTrends&months=6&billingLimit=1000&taskLimit=1000" >"$WORK_DIR/export-business-trends.csv"

python3 - "$WORK_DIR/business-metrics.json" "$WORK_DIR/business-trends.json" "$WORK_DIR/export-business-metrics.csv" "$WORK_DIR/export-business-trends.csv" "$TENANT_RENEWAL_TASK_ID" <<'PY'
import csv
import json
import pathlib
import sys

business_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
trends_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
metrics_csv_path = pathlib.Path(sys.argv[3])
trends_csv_path = pathlib.Path(sys.argv[4])
task_id = sys.argv[5]

def read_csv(path):
    return list(csv.reader(path.read_text(encoding="utf-8-sig").splitlines()))

def require_row(rows, section, metric, predicate):
    for row in rows:
        if len(row) >= 3 and row[0] == section and row[1] == metric and predicate(row):
            return row
    raise AssertionError((section, metric, rows))

business = business_payload["data"]
scale = next(item for item in business["packages"] if item["packageCode"] == "scale")
metrics_rows = read_csv(metrics_csv_path)
assert metrics_rows[0] == ["section", "metric", "value", "remark"], metrics_rows[0]
require_row(metrics_rows, "filter", "billingLimit", lambda row: row[2] == "1000")
require_row(metrics_rows, "summary", "estimatedMrrCents", lambda row: int(row[2]) == int(business["summary"]["estimatedMrrCents"]))
require_row(metrics_rows, "summary", "recentBillingAmountCents", lambda row: int(row[2]) >= 2380050)
require_row(metrics_rows, "package", "scale", lambda row: int(row[2]) == int(scale["estimatedMrrCents"]) and "latestAmountCents=2380050" in row[3])
require_row(metrics_rows, "recentBilling", "renewal scale", lambda row: int(row[2]) == 2380050)

trends = trends_payload["data"]
trend_rows = read_csv(trends_csv_path)
assert trend_rows[0] == ["section", "metric", "value", "remark"], trend_rows[0]
require_row(trend_rows, "filter", "months", lambda row: row[2] == "6")
require_row(trend_rows, "summary", "billingAmountCents", lambda row: int(row[2]) == int(trends["summary"]["billingAmountCents"]))
assert any(len(row) >= 3 and row[0] == "trendPackage" and row[1].endswith(" scale") and int(row[2]) >= 2380050 for row in trend_rows), trend_rows
require_row(trend_rows, "renewalFunnel", "tenantRenewalCount", lambda row: int(row[2]) >= 1)
require_row(trend_rows, "renewalTask", task_id, lambda row: row[2] == "applied")
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecast?tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000" >"$WORK_DIR/renewal-forecast.json"

FORECAST_FILTER_QUERY="$(python3 - "$WORK_DIR/renewal-forecast.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys
import urllib.parse

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])
tenant = next(item for item in payload["data"]["tenants"] if int(item["tenantId"]) == tenant_id)
latest_task = tenant.get("latestTask") or {}
params = {
    "packageCode": tenant["packageCode"],
    "bucket": tenant["bucket"],
    "priced": "priced" if tenant.get("priced") else "unknown",
    "taskStatus": latest_task.get("status") or "none",
}
owner = (tenant.get("owner") or "").strip()
if owner:
    params["owner"] = owner
print(urllib.parse.urlencode(params))
PY
)"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecast?tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000&$FORECAST_FILTER_QUERY" >"$WORK_DIR/renewal-forecast-filtered.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"owner\":\"forecast-csm\",\"nextFollowUpAt\":\"$FORECAST_FOLLOW_UP_ON\",\"remark\":\"smoke-renewal-forecast-assign\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecastAssign?tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000&$FORECAST_FILTER_QUERY" >"$WORK_DIR/renewal-forecast-assign.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"months\":12,\"externalOrderNoPrefix\":\"SMOKE-FORECAST\",\"remark\":\"smoke-renewal-forecast-task\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecastTasks?tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000" >"$WORK_DIR/renewal-forecast-tasks.json"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"reminderDays":30,"maxAttempts":4,"remark":"smoke-renewal-forecast-notify"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/renewalForecastNotifications?tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000&owner=forecast-csm" >"$WORK_DIR/renewal-forecast-notifications.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=renewalForecast&tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000" >"$WORK_DIR/export-renewal-forecast.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=renewalForecastOwners&tenantLimit=500&days=365&billingLimit=1000&taskLimit=1000" >"$WORK_DIR/export-renewal-forecast-owners.csv"

python3 - "$WORK_DIR/tenant-renewal-task-created.json" "$WORK_DIR/tenant-renewal-task-applied.json" "$WORK_DIR/tenant-renewal-tasks.json" "$WORK_DIR/billing-events-renewal-task.json" "$WORK_DIR/platform-tenant-renewed-by-task.json" "$WORK_DIR/business-metrics.json" "$WORK_DIR/business-trends.json" "$WORK_DIR/renewal-forecast.json" "$WORK_DIR/renewal-forecast-filtered.json" "$WORK_DIR/renewal-forecast-assign.json" "$WORK_DIR/renewal-forecast-tasks.json" "$WORK_DIR/renewal-forecast-notifications.json" "$WORK_DIR/export-renewal-forecast.csv" "$WORK_DIR/export-renewal-forecast-owners.csv" "$TENANT_ID" "$TENANT_RENEWAL_TASK_ID" "$PLATFORM_TENANT_ID" <<'PY'
import csv
import json
import os
import pathlib
import sys

created_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
applied_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
tasks_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
billing_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
overview_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
business_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
business_trends_payload = json.loads(pathlib.Path(sys.argv[7]).read_text(encoding="utf-8"))
renewal_forecast_payload = json.loads(pathlib.Path(sys.argv[8]).read_text(encoding="utf-8"))
renewal_forecast_filtered_payload = json.loads(pathlib.Path(sys.argv[9]).read_text(encoding="utf-8"))
renewal_forecast_assign_payload = json.loads(pathlib.Path(sys.argv[10]).read_text(encoding="utf-8"))
renewal_forecast_tasks_payload = json.loads(pathlib.Path(sys.argv[11]).read_text(encoding="utf-8"))
renewal_forecast_notifications_payload = json.loads(pathlib.Path(sys.argv[12]).read_text(encoding="utf-8"))
renewal_forecast_csv_path = pathlib.Path(sys.argv[13])
renewal_forecast_owner_csv_path = pathlib.Path(sys.argv[14])
tenant_id = int(sys.argv[15])
task_id = int(sys.argv[16])
platform_tenant_id = int(sys.argv[17])

assert created_payload["code"] == 200, created_payload
created_task = created_payload["data"]["task"]
created_preview = created_payload["data"]["result"]
assert created_task["id"] == task_id, created_task
assert created_task["taskType"] == "tenant_renewal", created_task
assert created_task["status"] == "pending", created_task
assert created_task["tenantId"] == tenant_id, created_task
assert created_task["packageCode"] == "scale", created_task
assert created_task["request"]["expiresAt"] == os.environ["TASK_RENEWAL_EXPIRES_AT"], created_task
assert created_preview["previousExpiresAt"] == os.environ["DIRECT_RENEWAL_EXPIRES_AT"], created_preview
assert created_preview["expiresAt"] == os.environ["TASK_RENEWAL_EXPIRES_AT"], created_preview
assert created_preview["amountCents"] == 2380050, created_preview
assert created_preview["blocked"] is False, created_preview

assert applied_payload["code"] == 200, applied_payload
applied_task = applied_payload["data"]["task"]
applied_result = applied_payload["data"]["result"]
assert applied_task["id"] == task_id, applied_task
assert applied_task["status"] == "applied", applied_task
assert applied_task["appliedAt"], applied_task
assert applied_result["tenantId"] == tenant_id, applied_result
assert applied_result["packageCode"] == "scale", applied_result
assert applied_result["previousExpiresAt"] == os.environ["DIRECT_RENEWAL_EXPIRES_AT"], applied_result
assert applied_result["expiresAt"] == os.environ["TASK_RENEWAL_EXPIRES_AT"], applied_result
assert applied_result["billingEventId"] > 0, applied_result
assert applied_result["metricsRefreshed"] == 26, applied_result

assert tasks_payload["code"] == 200, tasks_payload
tasks = tasks_payload["data"]["tasks"]
task_by_id = {item["id"]: item for item in tasks}
assert task_id in task_by_id, tasks
assert task_by_id[task_id]["status"] == "applied", task_by_id[task_id]
assert tasks_payload["data"]["filters"]["taskType"] == "tenant_renewal", tasks_payload["data"]["filters"]

assert billing_payload["code"] == 200, billing_payload
events = billing_payload["data"]["billingEvents"]
assert len(events) == 1, events
event = events[0]
assert event["id"] == applied_result["billingEventId"], event
assert event["eventType"] == "renewal", event
assert event["newExpiresAt"] == os.environ["TASK_RENEWAL_EXPIRES_AT"], event
assert event["externalOrderNo"] == f"SMOKE-RENEWAL-TASK-{tenant_id}", event

assert overview_payload["code"] == 200, overview_payload
tenant = overview_payload["data"]["tenants"][0]
assert tenant["tenantId"] == tenant_id, tenant
assert tenant["expiresAt"] == os.environ["TASK_RENEWAL_EXPIRES_AT"], tenant

assert business_payload["code"] == 200, business_payload
business = business_payload["data"]
assert business["estimated"] is True, business
assert business["filters"]["tenantLimit"] == 500, business["filters"]
business_summary = business["summary"]
assert business_summary["tenantCount"] >= 1, business_summary
assert business_summary["estimatedMrrCents"] > 0, business_summary
assert business_summary["estimatedArrCents"] == business_summary["estimatedMrrCents"] * 12, business_summary
assert business_summary["pricedTenantCount"] >= 1, business_summary
assert business_summary["recentBillingEventCount"] >= 1, business_summary
assert business_summary["recentRenewalCount"] >= 1, business_summary
assert business_summary["recentBillingAmountCents"] >= 2380050, business_summary
packages = business["packages"]
scale = next(item for item in packages if item["packageCode"] == "scale")
assert scale["tenantCount"] >= 1, scale
assert scale["latestAmountCents"] == 2380050, scale
assert scale["latestBillingEventId"] == event["id"], scale
assert scale["estimated"] is True, scale
assert scale["estimatedMrrCents"] > 0, scale
assert business["recentBillingEvents"], business
assert all(item["tenantId"] != platform_tenant_id for item in business["recentBillingEvents"]), business["recentBillingEvents"]
platform_packages = [item for item in packages if item["packageCode"] == "platform"]
assert not platform_packages or platform_packages[0]["tenantCount"] == 0, platform_packages

assert business_trends_payload["code"] == 200, business_trends_payload
trends = business_trends_payload["data"]
assert trends["filters"]["months"] == 6, trends["filters"]
trend_summary = trends["summary"]
assert trend_summary["billingEventCount"] >= 1, trend_summary
assert trend_summary["renewalCount"] >= 1, trend_summary
assert trend_summary["billingAmountCents"] >= 2380050, trend_summary
assert trend_summary["billingAmountCents"] < 99999999, trend_summary
assert trend_summary["taskCount"] >= 1, trend_summary
assert trend_summary["appliedTaskCount"] >= 1, trend_summary
assert trends["renewalFunnel"]["summary"]["tenantRenewalCount"] >= 1, trends["renewalFunnel"]
assert trends["renewalFunnel"]["summary"]["appliedCount"] >= 1, trends["renewalFunnel"]
trend_tasks = trends["renewalFunnel"]["recentTasks"]
assert all(item["tenantId"] != platform_tenant_id for item in trend_tasks), trend_tasks
trend_task = next(item for item in trend_tasks if item["id"] == task_id)
assert trend_task["status"] == "applied", trend_task
trend_packages = [pkg for month in trends["months"] for pkg in month["packages"]]
trend_scale = [pkg for pkg in trend_packages if pkg["packageCode"] == "scale"]
assert trend_scale and max(pkg["amountCents"] for pkg in trend_scale) >= 2380050, trend_scale

assert renewal_forecast_payload["code"] == 200, renewal_forecast_payload
forecast = renewal_forecast_payload["data"]
assert forecast["estimated"] is True, forecast
assert forecast["filters"]["days"] == 365, forecast["filters"]
forecast_summary = forecast["summary"]
assert all(item["tenantId"] != platform_tenant_id for item in forecast["tenants"]), forecast["tenants"]
assert forecast_summary["forecastTenantCount"] >= 1, forecast_summary
assert forecast_summary["pricedTenantCount"] >= 1, forecast_summary
assert forecast_summary["renewalAmountCents"] >= 2380050, forecast_summary
assert forecast_summary["estimatedMrrCents"] > 0, forecast_summary
assert forecast_summary["appliedTaskCount"] >= 1, forecast_summary
assert forecast["renewalTasks"]["summary"]["tenantRenewalCount"] >= 1, forecast["renewalTasks"]
forecast_tenant = next(item for item in forecast["tenants"] if item["tenantId"] == tenant_id)
assert forecast_tenant["packageCode"] == "scale", forecast_tenant
assert forecast_tenant["renewalAmountCents"] >= 2380050, forecast_tenant
assert forecast_tenant["latestBillingEventId"] == event["id"], forecast_tenant
assert forecast_tenant["latestTask"]["id"] == task_id, forecast_tenant
assert forecast_tenant["latestTask"]["status"] == "applied", forecast_tenant
forecast_buckets = forecast["buckets"]
assert any(bucket["tenantCount"] >= 1 and bucket["renewalAmountCents"] >= 2380050 for bucket in forecast_buckets), forecast_buckets
forecast_owners = forecast["owners"]
forecast_owner_name = forecast_tenant.get("owner") or "未分配"
forecast_owner = next(item for item in forecast_owners if item["owner"] == forecast_owner_name)
assert forecast_owner["tenantCount"] >= 1, forecast_owner
assert forecast_owner["renewalAmountCents"] >= forecast_tenant["renewalAmountCents"], forecast_owner
assert forecast_owner["pricedTenantCount"] >= 1, forecast_owner
assert forecast_owner["appliedTaskCount"] >= 1, forecast_owner
assert any(item["tenantId"] == tenant_id for item in forecast_owner["topTenants"]), forecast_owner

assert renewal_forecast_filtered_payload["code"] == 200, renewal_forecast_filtered_payload
filtered_forecast = renewal_forecast_filtered_payload["data"]
filtered_filters = filtered_forecast["filters"]
assert filtered_filters["packageCode"] == forecast_tenant["packageCode"], filtered_filters
assert filtered_filters["bucket"] == forecast_tenant["bucket"], filtered_filters
assert filtered_filters["priced"] == ("priced" if forecast_tenant["priced"] else "unknown"), filtered_filters
assert filtered_filters["taskStatus"] == forecast_tenant["latestTask"]["status"], filtered_filters
if forecast_tenant.get("owner"):
    assert filtered_filters["owner"] == forecast_tenant["owner"], filtered_filters
filtered_tenants = filtered_forecast["tenants"]
assert len(filtered_tenants) <= len(forecast["tenants"]), filtered_tenants
filtered_tenant = next(item for item in filtered_tenants if item["tenantId"] == tenant_id)
assert filtered_tenant["packageCode"] == forecast_tenant["packageCode"], filtered_tenant
assert filtered_tenant["bucket"] == forecast_tenant["bucket"], filtered_tenant
assert filtered_tenant["priced"] == forecast_tenant["priced"], filtered_tenant
assert filtered_tenant["latestTask"]["status"] == forecast_tenant["latestTask"]["status"], filtered_tenant
filtered_owner = next(item for item in filtered_forecast["owners"] if item["owner"] == forecast_owner_name)
assert filtered_owner["tenantCount"] >= 1, filtered_owner
assert any(item["tenantId"] == tenant_id for item in filtered_owner["topTenants"]), filtered_owner

assert renewal_forecast_assign_payload["code"] == 200, renewal_forecast_assign_payload
forecast_assign_data = renewal_forecast_assign_payload["data"]
assert forecast_assign_data["filters"]["packageCode"] == forecast_tenant["packageCode"], forecast_assign_data["filters"]
assert forecast_assign_data["filters"]["bucket"] == forecast_tenant["bucket"], forecast_assign_data["filters"]
assert forecast_assign_data["filters"]["priced"] == ("priced" if forecast_tenant["priced"] else "unknown"), forecast_assign_data["filters"]
assert forecast_assign_data["matchedCount"] >= 1, forecast_assign_data
assert forecast_assign_data["assignedCount"] >= 1, forecast_assign_data
assert forecast_assign_data["status"] == "renewal_pending", forecast_assign_data
assert forecast_assign_data["owner"] == "forecast-csm", forecast_assign_data
assert forecast_assign_data["remark"] == "smoke-renewal-forecast-assign", forecast_assign_data
forecast_assign_follow_up = next(item for item in forecast_assign_data["followUps"] if item["tenantId"] == tenant_id)
assert forecast_assign_follow_up["status"] == "renewal_pending", forecast_assign_follow_up
assert forecast_assign_follow_up["owner"] == "forecast-csm", forecast_assign_follow_up
assert forecast_assign_follow_up["nextFollowUpAt"] == os.environ["FORECAST_FOLLOW_UP_AT"], forecast_assign_follow_up
assert forecast_assign_follow_up["remark"] == "smoke-renewal-forecast-assign", forecast_assign_follow_up

assert renewal_forecast_tasks_payload["code"] == 200, renewal_forecast_tasks_payload
forecast_tasks_data = renewal_forecast_tasks_payload["data"]
assert forecast_tasks_data["filters"]["days"] == 365, forecast_tasks_data["filters"]
assert forecast_tasks_data["matchedCount"] >= 1, forecast_tasks_data
assert forecast_tasks_data["createdCount"] >= 1, forecast_tasks_data
assert forecast_tasks_data["pendingCount"] >= 1, forecast_tasks_data
assert forecast_tasks_data["remark"] == "smoke-renewal-forecast-task", forecast_tasks_data
forecast_task = next(task for task in forecast_tasks_data["tasks"] if task["tenantId"] == tenant_id)
assert forecast_task["taskType"] == "tenant_renewal", forecast_task
assert forecast_task["status"] == "pending", forecast_task
assert forecast_task["packageCode"] == "scale", forecast_task
assert forecast_task["request"]["externalOrderNo"] == f"SMOKE-FORECAST-{tenant_id}", forecast_task
assert forecast_task["request"]["amountCents"] >= 2380050, forecast_task
assert forecast_task["request"]["remark"] == "smoke-renewal-forecast-task", forecast_task
assert forecast_task["preview"]["tenantId"] == tenant_id, forecast_task

assert renewal_forecast_notifications_payload["code"] == 200, renewal_forecast_notifications_payload
forecast_notifications_data = renewal_forecast_notifications_payload["data"]
assert forecast_notifications_data["filters"]["owner"] == "forecast-csm", forecast_notifications_data["filters"]
assert forecast_notifications_data["matchedCount"] >= 1, forecast_notifications_data
assert forecast_notifications_data["enqueuedCount"] >= 1, forecast_notifications_data
assert forecast_notifications_data["channel"] == "webhook", forecast_notifications_data
assert forecast_notifications_data["maxAttempts"] == 4, forecast_notifications_data
assert forecast_notifications_data["reminderDays"] == 30, forecast_notifications_data
assert forecast_notifications_data["remark"] == "smoke-renewal-forecast-notify", forecast_notifications_data
forecast_notification = next(item for item in forecast_notifications_data["notifications"] if item["tenantId"] == tenant_id)
assert forecast_notification["status"] == "pending", forecast_notification
assert forecast_notification["metric"] == "tenant_renewal", forecast_notification
assert forecast_notification["alertType"] == "tenant_renewal_reminder", forecast_notification
assert "SaaS普通租户" in forecast_notification["message"], forecast_notification
assert "tenant_renewal_reminder" in forecast_notification["notificationKey"], forecast_notification

with renewal_forecast_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    forecast_rows = list(csv.reader(fh))
assert forecast_rows[0][:10] == ["tenantId", "tenantName", "tenantStatus", "packageCode", "packageName", "expiresAt", "bucket", "bucketLabel", "daysUntil", "renewalAmountCents"], forecast_rows[0]
assert forecast_rows[0][14:22] == ["taskCount", "actionableTaskCount", "pendingTaskCount", "blockedTaskCount", "failedTaskCount", "appliedTaskCount", "latestTaskId", "latestTaskStatus"], forecast_rows[0]
assert forecast_rows[0][23:26] == ["owner", "riskFollowUpStatus", "riskFollowUpNextAt"], forecast_rows[0]
assert all(int(row[0]) != platform_tenant_id for row in forecast_rows[1:]), forecast_rows
forecast_csv_row = next(row for row in forecast_rows[1:] if int(row[0]) == tenant_id)
assert forecast_csv_row[3] == "scale", forecast_csv_row
assert int(forecast_csv_row[9]) >= 2380050, forecast_csv_row
assert forecast_csv_row[11] == "true", forecast_csv_row
assert int(forecast_csv_row[12]) == event["id"], forecast_csv_row
assert int(forecast_csv_row[15]) >= 1, forecast_csv_row
assert int(forecast_csv_row[16]) >= 1, forecast_csv_row
assert forecast_csv_row[21] == "pending", forecast_csv_row
assert forecast_csv_row[23] == "forecast-csm", forecast_csv_row
assert forecast_csv_row[24] == "renewal_pending", forecast_csv_row
assert forecast_csv_row[25] == os.environ["FORECAST_FOLLOW_UP_AT"], forecast_csv_row

with renewal_forecast_owner_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    forecast_owner_rows = list(csv.reader(fh))
assert forecast_owner_rows[0][:6] == ["owner", "tenantCount", "pricedTenantCount", "unknownPriceTenantCount", "renewalAmountCents", "estimatedMrrCents"], forecast_owner_rows[0]
assert forecast_owner_rows[0][11:19] == ["actionableTaskCount", "pendingTaskCount", "blockedTaskCount", "failedTaskCount", "appliedTaskCount", "canceledTaskCount", "nextFollowUpAt", "topTenants"], forecast_owner_rows[0]
forecast_owner_csv_row = next(row for row in forecast_owner_rows[1:] if row[0] == "forecast-csm")
assert int(forecast_owner_csv_row[1]) >= 1, forecast_owner_csv_row
assert int(forecast_owner_csv_row[2]) >= 1, forecast_owner_csv_row
assert int(forecast_owner_csv_row[4]) >= 2380050, forecast_owner_csv_row
assert int(forecast_owner_csv_row[11]) >= 1, forecast_owner_csv_row
assert int(forecast_owner_csv_row[12]) >= 1, forecast_owner_csv_row
assert forecast_owner_csv_row[17] == os.environ["FORECAST_FOLLOW_UP_AT"], forecast_owner_csv_row
assert f"#{tenant_id}" in forecast_owner_csv_row[18], forecast_owner_csv_row
assert "due_" in forecast_owner_csv_row[18] or "expired" in forecast_owner_csv_row[18], forecast_owner_csv_row
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tasks?taskType=all&status=applied&limit=50" >"$WORK_DIR/admin-tasks-applied.json"

python3 - "$WORK_DIR/admin-tasks-applied.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))

def keys(value):
  if isinstance(value, dict):
    for key, child in value.items():
      yield key
      yield from keys(child)
  elif isinstance(value, list):
    for child in value:
      yield from keys(child)

assert payload["code"] == 200, payload
data = payload["data"]
assert data["filters"]["taskType"] == "", data["filters"]
assert data["filters"]["status"] == "applied", data["filters"]
tasks = data["tasks"]
summary = data["summary"]
assert data["returnedCount"] == len(tasks), data
assert summary["taskCount"] >= len(tasks), summary
assert summary["appliedCount"] == summary["taskCount"], summary
assert summary["pendingCount"] == 0, summary
assert summary["blockedCount"] == 0, summary
assert summary["failedCount"] == 0, summary
assert summary["canceledCount"] == 0, summary
assert summary["actionableCount"] == 0, summary
assert summary["packageSyncCount"] >= 1, summary
assert summary["tenantProvisionCount"] >= 1, summary
assert summary["tenantRenewalCount"] >= 1, summary
assert summary["tenantCount"] >= 1, summary
assert summary["actorUserCount"] >= 1, summary
types = {item["taskType"] for item in tasks}
assert {"package_sync", "tenant_provision", "tenant_renewal"}.issubset(types), tasks
assert all(item["status"] == "applied" for item in tasks), tasks
assert all(item["canApply"] is False for item in tasks), tasks
assert all("adminPasswordHash" not in set(keys(item)) for item in tasks), tasks
assert "secret989" not in json.dumps(tasks, ensure_ascii=False), tasks
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/taskOwners?taskType=all&status=applied&limit=20" >"$WORK_DIR/admin-task-owners-applied.json"

python3 - "$WORK_DIR/admin-task-owners-applied.json" "$PLATFORM_USER_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
platform_user_id = int(sys.argv[2])

def keys(value):
  if isinstance(value, dict):
    for key, child in value.items():
      yield key
      yield from keys(child)
  elif isinstance(value, list):
    for child in value:
      yield from keys(child)

assert payload["code"] == 200, payload
data = payload["data"]
assert data["filters"]["taskType"] == "", data["filters"]
assert data["filters"]["status"] == "applied", data["filters"]
assert data["ownerCount"] >= 1, data
assert data["returnedCount"] == len(data["owners"]), data
assert data["scannedTaskCount"] >= data["returnedCount"], data
assert data["partial"] is False, data
summary = data["summary"]
assert summary["appliedCount"] == summary["taskCount"], summary
assert summary["actionableCount"] == 0, summary
assert summary["packageSyncCount"] >= 1, summary
assert summary["tenantProvisionCount"] >= 1, summary
assert summary["tenantRenewalCount"] >= 1, summary
owner = next(item for item in data["owners"] if item["actorUserId"] == platform_user_id)
owner_summary = owner["summary"]
assert owner_summary["taskCount"] >= 3, owner
assert owner_summary["appliedCount"] == owner_summary["taskCount"], owner
assert owner_summary["actionableCount"] == 0, owner
recent_tasks = owner["recentTasks"]
assert recent_tasks, owner
assert all(item["status"] == "applied" for item in recent_tasks), recent_tasks
assert all("adminPasswordHash" not in set(keys(item)) for item in recent_tasks), recent_tasks
assert "secret989" not in json.dumps(data["owners"], ensure_ascii=False), data["owners"]
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantLifecycle?tenantId=$TENANT_ID&limit=50&expiringDays=10" >"$WORK_DIR/tenant-lifecycle.json"

python3 - "$WORK_DIR/tenant-lifecycle.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["tenantId"] == tenant_id, data
assert data["canPlatformScope"] is True, data
assert data["filters"]["limit"] == 50, data["filters"]
tenant = data["tenant"]
assert tenant["tenantId"] == tenant_id, tenant

summary = data["summary"]
assert summary["operationCount"] >= 1, summary
assert summary["billingEventCount"] >= 1, summary
assert summary["taskCount"] >= 1, summary
assert summary["alertCount"] >= 1, summary
assert summary["notificationCount"] >= 1, summary
assert summary["timelineCount"] >= 5, summary
assert summary["returnedEventCount"] == len(data["timeline"]), summary
assert summary["returnedEventCount"] <= 50, summary

sources = {item["source"] for item in data["timeline"]}
assert {"operation", "billing", "task", "alert", "notification"}.issubset(sources), data["timeline"]
assert all(item["tenantId"] == tenant_id for item in data["operations"]), data["operations"]
assert all(item["tenantId"] == tenant_id for item in data["billingEvents"]), data["billingEvents"]
assert all(item["tenantId"] == tenant_id for item in data["tasks"]), data["tasks"]
assert all(item["tenantId"] == tenant_id for item in data["alerts"]), data["alerts"]
assert all(item["tenantId"] == tenant_id for item in data["notifications"]), data["notifications"]
assert any(item["eventType"] == "tenant_renewal" and item["status"] == "applied" for item in data["timeline"]), data["timeline"]
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantLifecycle?tenantId=$TENANT_ID&limit=20&source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=tenant_renewal_reminder" >"$WORK_DIR/tenant-lifecycle-notification-filtered.json"

python3 - "$WORK_DIR/tenant-lifecycle-notification-filtered.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])

assert payload["code"] == 200, payload
data = payload["data"]
filters = data["filters"]
assert filters["tenantId"] == tenant_id, filters
assert filters["source"] == "notification", filters
assert filters["status"] == "pending", filters
assert filters["eventType"] == "tenant_renewal_reminder", filters
assert filters["keyword"] == "tenant_renewal_reminder", filters

summary = data["summary"]
assert summary["filterActive"] is True, summary
assert summary["rawTimelineCount"] >= summary["timelineCount"] >= 1, summary
assert summary["returnedEventCount"] == len(data["timeline"]), summary
assert summary["returnedEventCount"] <= 20, summary

for item in data["timeline"]:
  assert item["source"] == "notification", item
  assert item["status"] == "pending", item
  assert item["eventType"] == "tenant_renewal_reminder", item
  assert "tenant_renewal_reminder" in item["title"], item
  assert item["payload"]["tenantId"] == tenant_id, item
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=tenantLifecycle&tenantId=$TENANT_ID&limit=1000&source=notification&status=pending&eventType=tenant_renewal_reminder&keyword=tenant_renewal_reminder" >"$WORK_DIR/export-tenant-lifecycle.csv"

python3 - "$WORK_DIR/export-tenant-lifecycle.csv" "$TENANT_ID" <<'PY'
import csv
import pathlib
import sys

csv_path = pathlib.Path(sys.argv[1])
tenant_id = sys.argv[2]

rows = list(csv.DictReader(csv_path.read_text(encoding="utf-8-sig").splitlines()))
assert rows, rows
for row in rows:
  assert row["tenantId"] == tenant_id, row
  assert row["source"] == "notification", row
  assert row["status"] == "pending", row
  assert row["eventType"] == "tenant_renewal_reminder", row
  assert "tenant_renewal_reminder" in row["title"], row
  assert '"tenantId":' + tenant_id in row["payload"], row
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"metric\":\"users\",\"alertType\":\"quota_exceeded\",\"periodKey\":\"lifetime\",\"remark\":\"smoke-alert-resolve\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/alertResolve" >"$WORK_DIR/alert-resolve.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/alerts?tenantId=$TENANT_ID&status=resolved&metric=users&alertType=quota_exceeded&perPage=10" >"$WORK_DIR/alerts-resolved.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/usage?tenantId=$TENANT_ID&expiringDays=10" >"$WORK_DIR/platform-tenant-usage-after-alert-resolve.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.alert.resolve&targetType=alert&keyword=users" >"$WORK_DIR/operations-alert-resolve.json"

python3 - "$WORK_DIR/alert-resolve.json" "$WORK_DIR/alerts-resolved.json" "$WORK_DIR/platform-tenant-usage-after-alert-resolve.json" "$WORK_DIR/operations-alert-resolve.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

resolve_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
alerts_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
usage_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[5])

assert resolve_payload["code"] == 200, resolve_payload
resolved = resolve_payload["data"]
assert resolved["resolved"] is True, resolved
assert resolved["tenantId"] == tenant_id, resolved
assert resolved["metric"] == "users", resolved
assert resolved["metricLabel"] == "子账号数", resolved
assert resolved["alertType"] == "quota_exceeded", resolved
assert resolved["periodKey"] == "lifetime", resolved
assert resolved["previousStatus"] == "open", resolved
assert resolved["status"] == "resolved", resolved
assert resolved["operationId"] > 0, resolved

assert alerts_payload["code"] == 200, alerts_payload
alerts = alerts_payload["data"]["alerts"]
assert any(item["tenantId"] == tenant_id and item["metric"] == "users" and item["status"] == "resolved" for item in alerts), alerts

assert usage_payload["code"] == 200, usage_payload
users_metric = next(item for item in usage_payload["data"]["usageMetrics"] if item["metric"] == "users")
assert users_metric["openAlertCount"] == 0, users_metric

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert operations, operations
operation = operations[0]
assert operation["action"] == "tenant.alert.resolve", operation
assert operation["tenantId"] == tenant_id, operation
assert operation["targetType"] == "alert", operation
assert operation["after"]["status"] == "resolved", operation
assert operation["remark"] == "smoke-alert-resolve", operation
PY

mysql_root mochat_saas_admin <<SQL
INSERT INTO mochat_go_saas_alerts
  (alert_key, tenant_id, alert_type, severity, status, metric, period_key, current_value, limit_value, additional_value, occurrence_count, source, message, context_json, first_seen_at, last_seen_at, resolved_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-saas-admin-bulk-alert-users', $TENANT_ID, 'quota_exceeded', 'warning', 'open', 'users', 'bulk-users', 9, 10, 0, 1, 'smoke.saas-admin.bulk', 'SaaS总后台 bulk 告警 users', JSON_OBJECT('smoke', 'bulk'), NOW(), NOW(), NULL, NOW(), NOW(), NULL),
  ('smoke-saas-admin-bulk-alert-contacts', $TENANT_ID, 'quota_exceeded', 'warning', 'open', 'contacts', 'bulk-contacts', 120, 100, 0, 1, 'smoke.saas-admin.bulk', 'SaaS总后台 bulk 告警 contacts', JSON_OBJECT('smoke', 'bulk'), NOW(), NOW(), NULL, NOW(), NOW(), NULL);
SQL

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"alertType\":\"quota_exceeded\",\"limit\":10,\"remark\":\"smoke-alert-bulk-resolve\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/alertBulkResolve" >"$WORK_DIR/alert-bulk-resolve.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/alerts?tenantId=$TENANT_ID&status=resolved&alertType=quota_exceeded&perPage=10" >"$WORK_DIR/alerts-bulk-resolved.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.alert.resolve&targetType=alert&keyword=bulk" >"$WORK_DIR/operations-alert-bulk-resolve.json"

python3 - "$WORK_DIR/alert-bulk-resolve.json" "$WORK_DIR/alerts-bulk-resolved.json" "$WORK_DIR/operations-alert-bulk-resolve.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

resolve_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
alerts_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])

assert resolve_payload["code"] == 200, resolve_payload
resolved = resolve_payload["data"]
assert resolved["resolved"] is True, resolved
assert resolved["resolvedCount"] == 2, resolved
assert resolved["tenantId"] == tenant_id, resolved
assert resolved["filters"]["alertType"] == "quota_exceeded", resolved
assert len(resolved["alerts"]) == 2, resolved
for item in resolved["alerts"]:
    assert item["tenantId"] == tenant_id, item
    assert item["previousStatus"] == "open", item
    assert item["status"] == "resolved", item
    assert item["operationId"] > 0, item

assert alerts_payload["code"] == 200, alerts_payload
resolved_alerts = alerts_payload["data"]["alerts"]
bulk_resolved = [item for item in resolved_alerts if str(item["periodKey"]).startswith("bulk-")]
assert len(bulk_resolved) == 2, resolved_alerts
assert {item["metric"] for item in bulk_resolved} == {"users", "contacts"}, bulk_resolved
for item in bulk_resolved:
    assert item["status"] == "resolved", item

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
bulk_operations = [item for item in operations if item["remark"] == "smoke-alert-bulk-resolve"]
assert len(bulk_operations) >= 2, operations
for operation in bulk_operations[:2]:
    assert operation["action"] == "tenant.alert.resolve", operation
    assert operation["tenantId"] == tenant_id, operation
    assert operation["targetType"] == "alert", operation
    assert "bulk" in operation["targetId"], operation
    assert operation["after"]["status"] == "resolved", operation
    assert operation["after"]["bulkResolve"] is True, operation
PY

NOTIFICATION_ID="$(python3 - "$WORK_DIR/platform-notifications.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(payload["data"]["notifications"][0]["id"])
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"notificationId\":$NOTIFICATION_ID,\"remark\":\"smoke-notification-retry\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationRetry" >"$WORK_DIR/notification-retry.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$TENANT_ID&status=pending&channel=webhook&keyword=dead&limit=10" >"$WORK_DIR/notifications-retried.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.notification.retry&targetType=alert_notification&keyword=$NOTIFICATION_ID" >"$WORK_DIR/operations-notification-retry.json"

python3 - "$WORK_DIR/notification-retry.json" "$WORK_DIR/notifications-retried.json" "$WORK_DIR/operations-notification-retry.json" "$TENANT_ID" "$NOTIFICATION_ID" <<'PY'
import json
import pathlib
import sys

retry_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
notifications_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])
notification_id = int(sys.argv[5])

assert retry_payload["code"] == 200, retry_payload
retried = retry_payload["data"]
assert retried["retried"] is True, retried
assert retried["notificationId"] == notification_id, retried
assert retried["tenantId"] == tenant_id, retried
assert retried["previousStatus"] == "dead", retried
assert retried["status"] == "pending", retried
assert retried["attempts"] == 0, retried
assert retried["operationId"] > 0, retried

assert notifications_payload["code"] == 200, notifications_payload
notifications = notifications_payload["data"]["notifications"]
assert len(notifications) == 1, notifications
notification = notifications[0]
assert notification["id"] == notification_id, notification
assert notification["status"] == "pending", notification
assert notification["attempts"] == 0, notification
assert notification["lastError"] == "", notification

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert operations, operations
operation = operations[0]
assert operation["action"] == "tenant.notification.retry", operation
assert operation["tenantId"] == tenant_id, operation
assert operation["targetType"] == "alert_notification", operation
assert operation["targetId"] == str(notification_id), operation
assert operation["after"]["status"] == "pending", operation
assert operation["remark"] == "smoke-notification-retry", operation
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"status\":\"failed\",\"channel\":\"webhook\",\"keyword\":\"bulk\",\"limit\":10,\"remark\":\"smoke-notification-bulk-retry\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationBulkRetry" >"$WORK_DIR/notification-bulk-retry.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$TENANT_ID&status=pending&channel=webhook&keyword=bulk&limit=10" >"$WORK_DIR/notifications-bulk-retried.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.notification.retry&targetType=alert_notification&keyword=bulk" >"$WORK_DIR/operations-notification-bulk-retry.json"

python3 - "$WORK_DIR/notification-bulk-retry.json" "$WORK_DIR/notifications-bulk-retried.json" "$WORK_DIR/operations-notification-bulk-retry.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

retry_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
notifications_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])

assert retry_payload["code"] == 200, retry_payload
retried = retry_payload["data"]
assert retried["retried"] is True, retried
assert retried["retriedCount"] == 2, retried
assert retried["tenantId"] == tenant_id, retried
assert retried["filters"]["status"] == "failed", retried
assert retried["filters"]["keyword"] == "bulk", retried
assert len(retried["notifications"]) == 2, retried
for item in retried["notifications"]:
    assert item["tenantId"] == tenant_id, item
    assert item["previousStatus"] == "failed", item
    assert item["status"] == "pending", item
    assert item["attempts"] == 0, item
    assert item["operationId"] > 0, item

assert notifications_payload["code"] == 200, notifications_payload
notifications = notifications_payload["data"]["notifications"]
assert len(notifications) == 2, notifications
for notification in notifications:
    assert notification["tenantId"] == tenant_id, notification
    assert notification["status"] == "pending", notification
    assert notification["attempts"] == 0, notification
    assert notification["lastError"] == "", notification
    assert "bulk" in notification["notificationKey"], notification

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
bulk_operations = [item for item in operations if item["remark"] == "smoke-notification-bulk-retry"]
assert len(bulk_operations) >= 2, operations
for operation in bulk_operations[:2]:
    assert operation["action"] == "tenant.notification.retry", operation
    assert operation["tenantId"] == tenant_id, operation
    assert operation["targetType"] == "alert_notification", operation
    assert "bulk" in operation["targetName"], operation
    assert operation["after"]["status"] == "pending", operation
    assert operation["after"]["bulkRetry"] is True, operation
PY

CLOSE_NOTIFICATION_ID="$(python3 - "$WORK_DIR/notifications-bulk-retried.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(payload["data"]["notifications"][0]["id"])
PY
)"

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"notificationId\":$CLOSE_NOTIFICATION_ID,\"remark\":\"smoke-notification-close\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationClose" >"$WORK_DIR/notification-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$TENANT_ID&status=closed&channel=webhook&keyword=bulk&limit=10" >"$WORK_DIR/notifications-closed-single.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.notification.close&targetType=alert_notification&keyword=$CLOSE_NOTIFICATION_ID" >"$WORK_DIR/operations-notification-close.json"

python3 - "$WORK_DIR/notification-close.json" "$WORK_DIR/notifications-closed-single.json" "$WORK_DIR/operations-notification-close.json" "$TENANT_ID" "$CLOSE_NOTIFICATION_ID" <<'PY'
import json
import pathlib
import sys

close_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
notifications_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])
notification_id = int(sys.argv[5])

assert close_payload["code"] == 200, close_payload
closed = close_payload["data"]
assert closed["closed"] is True, closed
assert closed["notificationId"] == notification_id, closed
assert closed["tenantId"] == tenant_id, closed
assert closed["previousStatus"] == "pending", closed
assert closed["status"] == "closed", closed
assert closed["remark"] == "smoke-notification-close", closed
assert closed["operationId"] > 0, closed

assert notifications_payload["code"] == 200, notifications_payload
notifications = notifications_payload["data"]["notifications"]
notification = next(item for item in notifications if item["id"] == notification_id)
assert notification["status"] == "closed", notification
assert notification["lastError"] == "smoke-notification-close", notification
assert notification["nextRetryAt"] == "", notification

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
operation = next(item for item in operations if item["targetId"] == str(notification_id))
assert operation["action"] == "tenant.notification.close", operation
assert operation["tenantId"] == tenant_id, operation
assert operation["targetType"] == "alert_notification", operation
assert operation["after"]["status"] == "closed", operation
assert operation["after"]["previous"] == "pending", operation
assert operation["remark"] == "smoke-notification-close", operation
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"tenantId\":$TENANT_ID,\"status\":\"pending\",\"channel\":\"webhook\",\"keyword\":\"bulk\",\"limit\":10,\"remark\":\"smoke-notification-bulk-close\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationBulkClose" >"$WORK_DIR/notification-bulk-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?tenantId=$TENANT_ID&status=closed&channel=webhook&keyword=bulk&limit=10" >"$WORK_DIR/notifications-bulk-closed.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?limit=20&tenantId=$TENANT_ID&action=tenant.notification.close&targetType=alert_notification&keyword=bulk" >"$WORK_DIR/operations-notification-bulk-close.json"

python3 - "$WORK_DIR/notification-bulk-close.json" "$WORK_DIR/notifications-bulk-closed.json" "$WORK_DIR/operations-notification-bulk-close.json" "$TENANT_ID" "$CLOSE_NOTIFICATION_ID" <<'PY'
import json
import pathlib
import sys

close_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
notifications_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])
single_closed_id = int(sys.argv[5])

assert close_payload["code"] == 200, close_payload
closed = close_payload["data"]
assert closed["closed"] is True, closed
assert closed["closedCount"] == 1, closed
assert closed["tenantId"] == tenant_id, closed
assert closed["filters"]["status"] == "pending", closed
assert closed["filters"]["keyword"] == "bulk", closed
assert closed["remark"] == "smoke-notification-bulk-close", closed
assert len(closed["notifications"]) == 1, closed
bulk_closed = closed["notifications"][0]
assert bulk_closed["notificationId"] != single_closed_id, bulk_closed
assert bulk_closed["tenantId"] == tenant_id, bulk_closed
assert bulk_closed["previousStatus"] == "pending", bulk_closed
assert bulk_closed["status"] == "closed", bulk_closed
assert bulk_closed["operationId"] > 0, bulk_closed

assert notifications_payload["code"] == 200, notifications_payload
notifications = notifications_payload["data"]["notifications"]
assert len(notifications) == 2, notifications
assert notifications_payload["data"]["summary"]["closedCount"] == 2, notifications_payload["data"]["summary"]
assert {item["status"] for item in notifications} == {"closed"}, notifications
assert {item["lastError"] for item in notifications} == {"smoke-notification-close", "smoke-notification-bulk-close"}, notifications

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
bulk_operations = [item for item in operations if item["remark"] == "smoke-notification-bulk-close"]
assert len(bulk_operations) >= 1, operations
operation = bulk_operations[0]
assert operation["action"] == "tenant.notification.close", operation
assert operation["tenantId"] == tenant_id, operation
assert operation["targetType"] == "alert_notification", operation
assert "bulk" in operation["targetName"], operation
assert operation["after"]["status"] == "closed", operation
assert operation["after"]["bulkClose"] is True, operation
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/dailyReport?date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/daily-report-after-notification-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=dailyReport&date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/export-daily-report-after-notification-close.csv"

python3 - "$WORK_DIR/daily-report-after-notification-close.json" "$WORK_DIR/export-daily-report-after-notification-close.csv" "$TENANT_ID" <<'PY'
import csv
import json
import pathlib
import sys

daily_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
csv_path = pathlib.Path(sys.argv[2])
tenant_id = int(sys.argv[3])

assert daily_payload["code"] == 200, daily_payload
daily = daily_payload["data"]
summary = daily["summary"]
assert summary["closedNotificationCount"] >= 2, summary
assert summary["windowNotificationCloseCount"] >= 2, summary
notification_data = daily["notifications"]
assert notification_data["summary"]["closedCount"] >= 2, notification_data["summary"]
closed_items = notification_data["closedItems"]
assert len(closed_items) >= 2, closed_items
assert {item["status"] for item in closed_items} == {"closed"}, closed_items
assert any(item["tenantId"] == tenant_id and item["lastError"] == "smoke-notification-close" for item in closed_items), closed_items
assert any(item["tenantId"] == tenant_id and item["lastError"] == "smoke-notification-bulk-close" for item in closed_items), closed_items

with csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    rows = list(csv.reader(fh))
assert any(row[:3] == ["summary", "closedNotificationCount", str(summary["closedNotificationCount"])] for row in rows), rows
assert any(row[:3] == ["summary", "windowNotificationCloseCount", str(summary["windowNotificationCloseCount"])] for row in rows), rows
closed_rows = [row for row in rows if row[0] == "closedNotification"]
assert len(closed_rows) >= 2, rows
assert any(row[1].startswith("closed ") and row[2] == str(tenant_id) and "smoke-notification-close" in row[3] for row in closed_rows), closed_rows
assert any(row[1].startswith("closed ") and row[2] == str(tenant_id) and "smoke-notification-bulk-close" in row[3] for row in closed_rows), closed_rows
PY

mysql_exec "UPDATE mochat_go_saas_admin_tasks SET created_at = DATE_SUB(NOW(), INTERVAL 6 HOUR), updated_at = DATE_SUB(NOW(), INTERVAL 6 HOUR) WHERE task_type = 'tenant_renewal' AND status = 'pending' AND remark = 'smoke-renewal-forecast-task' AND deleted_at IS NULL"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_tasks WHERE task_type = 'tenant_renewal' AND status = 'pending' AND remark = 'smoke-renewal-forecast-task' AND created_at <= DATE_SUB(NOW(), INTERVAL 5 HOUR) AND deleted_at IS NULL")" -ge 1

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueue?tenantLimit=200&limit=100&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-after-notification-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operationQueue&tenantLimit=200&limit=1000&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/export-operation-queue.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueOwners?tenantLimit=200&limit=100&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-owners-after-notification-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operationQueueOwners&tenantLimit=200&limit=1000&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/export-operation-queue-owners.csv"

python3 - "$WORK_DIR/operation-queue-after-notification-close.json" "$WORK_DIR/export-operation-queue.csv" "$WORK_DIR/operation-queue-owners-after-notification-close.json" "$WORK_DIR/export-operation-queue-owners.csv" "$PLATFORM_TENANT_ID" "$TENANT_ID" <<'PY'
import csv
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
csv_path = pathlib.Path(sys.argv[2])
owners_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
owner_csv_path = pathlib.Path(sys.argv[4])
platform_tenant_id = int(sys.argv[5])
tenant_id = int(sys.argv[6])

assert payload["code"] == 200, payload
data = payload["data"]
summary = data["summary"]
items = data["items"]
assert summary["queueCount"] >= len(items) >= 1, data
sources = {item["source"] for item in items}
assert "task_sla" in sources, items
assert "closed_notification" in sources, items
assert "notification_health" in sources, items
assert summary["taskSlaCount"] >= 1, summary
assert summary["closedNotificationCount"] >= 2, summary
assert summary["notificationHealthCount"] >= 1, summary
assert all(item["tenantId"] != platform_tenant_id for item in items), items
assert any(item["tenantId"] == tenant_id for item in items), items
assert any(item["priority"] == "critical" for item in items), items

with csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    rows = list(csv.reader(fh))
assert rows[0][:4] == ["source", "priority", "tenantId", "tenantName"], rows[0]
assert any(row[0] == "task_sla" for row in rows), rows
assert any(row[0] == "closed_notification" for row in rows), rows
assert any(row[0] == "notification_health" for row in rows), rows
assert all(int(row[2]) != platform_tenant_id for row in rows[1:]), rows

assert owners_payload["code"] == 200, owners_payload
owner_data = owners_payload["data"]
owner_summary = owner_data["summary"]
owners = owner_data["owners"]
assert owner_summary["taskSlaCount"] >= 1, owner_summary
assert owner_summary["closedNotificationCount"] >= 2, owner_summary
assert owner_summary["notificationHealthCount"] >= 1, owner_summary
assert owner_data["ownerCount"] >= len(owners) >= 1, owner_data
assert owner_data["scannedQueueCount"] >= len(items), owner_data
assert any(owner["taskSlaCount"] >= 1 for owner in owners), owners
assert any(owner["closedNotificationCount"] >= 2 for owner in owners), owners
assert any(owner["notificationHealthCount"] >= 1 for owner in owners), owners
assert all(all(tenant["tenantId"] != platform_tenant_id for tenant in owner["topTenants"]) for owner in owners), owners
assert any(any(tenant["tenantId"] == tenant_id for tenant in owner["topTenants"]) for owner in owners), owners

with owner_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    owner_rows = list(csv.reader(fh))
assert owner_rows[0][:4] == ["owner", "queueCount", "tenantCount", "sourceCount"], owner_rows[0]
assert "notificationHealthCount" in owner_rows[0], owner_rows[0]
assert len(owner_rows) >= 2, owner_rows
assert any(int(row[9]) >= 1 for row in owner_rows[1:]), owner_rows
assert any(int(row[12]) >= 2 for row in owner_rows[1:]), owner_rows
PY

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"owner":"ops-queue","status":"contacted","nextFollowUpAt":"2026-07-25 10:00:00","remark":"smoke-operation-queue-assign"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssign?source=customer_success&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-assign.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/riskFollowUps?tenantId=$TENANT_ID&status=contacted&owner=ops-queue&keyword=smoke-operation-queue-assign&limit=20" >"$WORK_DIR/operation-queue-assign-risk-follow-ups.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueOwners?source=customer_success&owner=ops-queue&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-assign-owners.json"

python3 - "$WORK_DIR/operation-queue-assign.json" "$WORK_DIR/operation-queue-assign-risk-follow-ups.json" "$WORK_DIR/operation-queue-assign-owners.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

assign_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
follow_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
owners_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[4])

assert assign_payload["code"] == 200, assign_payload
assign = assign_payload["data"]
assert assign["filters"]["source"] == "customer_success", assign["filters"]
assert assign["matchedCount"] >= 1, assign
assert assign["assignableCount"] >= 1, assign
assert assign["assignedCount"] >= 1, assign
assert assign["customerSuccessAssignedCount"] >= 1, assign
assert assign["billingFollowUpAssignedCount"] == 0, assign
assert assign["owner"] == "ops-queue", assign
assert assign["status"] == "contacted", assign
assert assign["nextFollowUpAt"] == "2026-07-25 10:00:00", assign
assert assign["remark"] == "smoke-operation-queue-assign", assign
assert any(item["queueItem"]["source"] == "customer_success" and item.get("riskFollowUp", {}).get("tenantId") == tenant_id for item in assign["items"]), assign["items"]

assert follow_payload["code"] == 200, follow_payload
follow_ups = follow_payload["data"]["followUps"]
assert any(item["tenantId"] == tenant_id and item["owner"] == "ops-queue" and item["remark"] == "smoke-operation-queue-assign" for item in follow_ups), follow_ups

assert owners_payload["code"] == 200, owners_payload
owners = owners_payload["data"]["owners"]
assert any(owner["owner"] == "ops-queue" and owner["customerSuccessCount"] >= 1 for owner in owners), owners
PY

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"owner\":\"ops-task-sla\",\"status\":\"contacted\",\"nextFollowUpAt\":\"$OPERATION_QUEUE_ASSIGN_NEXT_AT\",\"remark\":\"smoke-operation-queue-task-sla-assign\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssign?source=task_sla&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-task-sla-assign.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueue?source=task_sla&owner=ops-task-sla&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-task-sla-assigned.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueOwners?source=task_sla&owner=ops-task-sla&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-task-sla-assign-owners.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignments?source=task_sla&owner=ops-task-sla&dueState=future&keyword=smoke-operation-queue-task-sla-assign&limit=20" >"$WORK_DIR/operation-queue-task-sla-assignments.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operationQueueAssignments&source=task_sla&owner=ops-task-sla&dueState=future&keyword=smoke-operation-queue-task-sla-assign&limit=1000" >"$WORK_DIR/export-operation-queue-assignments.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/dailyReport?date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/daily-report-after-operation-queue-assign.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=dailyReport&date=$REPORT_DATE&days=1&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8" >"$WORK_DIR/export-daily-report-after-operation-queue-assign.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantLifecycle?tenantId=$TENANT_ID&limit=20&source=operation&status=contacted&eventType=operation_queue.assign&keyword=smoke-operation-queue-task-sla-assign" >"$WORK_DIR/tenant-lifecycle-operation-queue-assign.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=tenantLifecycle&tenantId=$TENANT_ID&limit=1000&source=operation&status=contacted&eventType=operation_queue.assign&keyword=smoke-operation-queue-task-sla-assign" >"$WORK_DIR/export-tenant-lifecycle-operation-queue-assign.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?action=saas.admin.operation_queue.assign&targetType=admin_task&keyword=smoke-operation-queue-task-sla-assign&limit=20" >"$WORK_DIR/operation-queue-task-sla-assign-operations.json"

python3 - "$WORK_DIR/operation-queue-task-sla-assign.json" "$WORK_DIR/operation-queue-task-sla-assigned.json" "$WORK_DIR/operation-queue-task-sla-assign-owners.json" "$WORK_DIR/operation-queue-task-sla-assignments.json" "$WORK_DIR/export-operation-queue-assignments.csv" "$WORK_DIR/daily-report-after-operation-queue-assign.json" "$WORK_DIR/export-daily-report-after-operation-queue-assign.csv" "$WORK_DIR/tenant-lifecycle-operation-queue-assign.json" "$WORK_DIR/export-tenant-lifecycle-operation-queue-assign.csv" "$WORK_DIR/operation-queue-task-sla-assign-operations.json" <<'PY'
import csv
import json
import pathlib
import sys

assign_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
queue_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
owners_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
assignments_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
assignment_csv_path = pathlib.Path(sys.argv[5])
daily_report_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
daily_report_csv_path = pathlib.Path(sys.argv[7])
tenant_lifecycle_payload = json.loads(pathlib.Path(sys.argv[8]).read_text(encoding="utf-8"))
tenant_lifecycle_csv_path = pathlib.Path(sys.argv[9])
operations_payload = json.loads(pathlib.Path(sys.argv[10]).read_text(encoding="utf-8"))

assert assign_payload["code"] == 200, assign_payload
assign = assign_payload["data"]
assert assign["filters"]["source"] == "task_sla", assign["filters"]
assert assign["matchedCount"] >= 1, assign
assert assign["assignableCount"] >= 1, assign
assert assign["assignedCount"] >= 1, assign
assert assign["queueAssignmentAssignedCount"] >= 1, assign
assert assign["taskSlaAssignedCount"] >= 1, assign
assert assign["owner"] == "ops-task-sla", assign
assert any(item["queueItem"]["source"] == "task_sla" and item.get("operationQueueAssignment", {}).get("owner") == "ops-task-sla" for item in assign["items"]), assign["items"]

assert queue_payload["code"] == 200, queue_payload
queue_items = queue_payload["data"]["items"]
assert any(item["source"] == "task_sla" and item["owner"] == "ops-task-sla" and item.get("assignment", {}).get("remark") == "smoke-operation-queue-task-sla-assign" for item in queue_items), queue_items

assert owners_payload["code"] == 200, owners_payload
owners = owners_payload["data"]["owners"]
assert any(owner["owner"] == "ops-task-sla" and owner["taskSlaCount"] >= 1 for owner in owners), owners

assert assignments_payload["code"] == 200, assignments_payload
assignment_data = assignments_payload["data"]
assert assignment_data["filters"]["source"] == "task_sla", assignment_data["filters"]
assert assignment_data["filters"]["dueState"] == "future", assignment_data["filters"]
assert assignment_data["assignmentCount"] >= 1, assignment_data
assert assignment_data["summary"]["taskSlaCount"] >= 1, assignment_data["summary"]
assert assignment_data["summary"]["futureCount"] >= 1, assignment_data["summary"]
assignments = assignment_data["assignments"]
assert any(item["source"] == "task_sla" and item["owner"] == "ops-task-sla" and item["dueState"] == "future" and item["remark"] == "smoke-operation-queue-task-sla-assign" for item in assignments), assignments
with assignment_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    rows = list(csv.reader(fh))
assert rows and rows[0][:5] == ["operationId", "tenantId", "source", "objectType", "objectId"], rows[:2]
assert rows[0][8] == "dueState", rows[0]
assert any(row[2] == "task_sla" and row[6] == "ops-task-sla" and row[8] == "future" and row[10] == "smoke-operation-queue-task-sla-assign" for row in rows[1:]), rows

assert daily_report_payload["code"] == 200, daily_report_payload
daily = daily_report_payload["data"]
daily_summary = daily["summary"]
assert daily_summary["windowQueueAssignmentCount"] >= 1, daily_summary
assert daily_summary["windowTaskSlaAssignCount"] >= 1, daily_summary
daily_assignments = daily["operationQueueAssignments"]
assert daily_assignments["summary"]["taskSlaCount"] >= 1, daily_assignments
assert daily_assignments["summary"]["futureCount"] >= 1, daily_assignments
assert any(item["source"] == "task_sla" and item["owner"] == "ops-task-sla" and item["dueState"] == "future" and item["remark"] == "smoke-operation-queue-task-sla-assign" for item in daily_assignments["assignments"]), daily_assignments
with daily_report_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    daily_rows = list(csv.reader(fh))
assert any(row[:3] == ["summary", "windowQueueAssignmentCount", str(daily_summary["windowQueueAssignmentCount"])] for row in daily_rows), daily_rows
assert any(row[0] == "operationQueueAssignment" and row[1] == "task_sla contacted" and row[2] == "ops-task-sla" and "dueState=future" in row[3] and "smoke-operation-queue-task-sla-assign" in row[3] for row in daily_rows), daily_rows

assert tenant_lifecycle_payload["code"] == 200, tenant_lifecycle_payload
lifecycle = tenant_lifecycle_payload["data"]
lifecycle_filters = lifecycle["filters"]
assigned_tenant_ids = {item["queueItem"]["tenantId"] for item in assign["items"] if item["queueItem"]["source"] == "task_sla" and item.get("operationQueueAssignment", {}).get("owner") == "ops-task-sla"}
assert lifecycle_filters["tenantId"] in assigned_tenant_ids, (lifecycle_filters, assigned_tenant_ids)
assert lifecycle_filters["source"] == "operation", lifecycle_filters
assert lifecycle_filters["status"] == "contacted", lifecycle_filters
assert lifecycle_filters["eventType"] == "operation_queue.assign", lifecycle_filters
assert lifecycle_filters["keyword"] == "smoke-operation-queue-task-sla-assign", lifecycle_filters
lifecycle_summary = lifecycle["summary"]
assert lifecycle_summary["filterActive"] is True, lifecycle_summary
assert lifecycle_summary["timelineCount"] >= 1, lifecycle_summary
lifecycle_events = lifecycle["timeline"]
assert any(
    item["source"] == "operation"
    and item["eventType"] == "saas.admin.operation_queue.assign"
    and item["status"] == "contacted"
    and item["remark"] == "smoke-operation-queue-task-sla-assign"
    and item["payload"]["after"]["owner"] == "ops-task-sla"
    and item["payload"]["after"]["source"] == "task_sla"
    for item in lifecycle_events
), lifecycle_events
with tenant_lifecycle_csv_path.open("r", encoding="utf-8-sig", newline="") as fh:
    lifecycle_rows = list(csv.reader(fh))
assert lifecycle_rows and lifecycle_rows[0][4:8] == ["source", "eventType", "title", "status"], lifecycle_rows[:2]
assert any(row[4] == "operation" and row[5] == "saas.admin.operation_queue.assign" and row[7] == "contacted" and row[11] == "smoke-operation-queue-task-sla-assign" for row in lifecycle_rows[1:]), lifecycle_rows

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert any(item["action"] == "saas.admin.operation_queue.assign" and item["targetType"] == "admin_task" and item["remark"] == "smoke-operation-queue-task-sla-assign" for item in operations), operations
PY

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"owner\":\"ops-task-sla-overdue\",\"status\":\"contacted\",\"nextFollowUpAt\":\"$OPERATION_QUEUE_ASSIGN_OVERDUE_AT\",\"remark\":\"smoke-operation-queue-task-sla-overdue-assign\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssign?source=task_sla&tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.8&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-task-sla-overdue-assign.json"

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"remark":"smoke-operation-queue-assignment-notify"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignmentNotifications?source=task_sla&owner=ops-task-sla-overdue&dueState=overdue&keyword=smoke-operation-queue-task-sla-overdue-assign&limit=20" >"$WORK_DIR/operation-queue-assignment-notifications.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notifications?channel=webhook&keyword=operation_queue_assignment_reminder&limit=20" >"$WORK_DIR/operation-queue-assignment-notification-outbox.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?action=saas.admin.operation_queue.assignment_notify&targetType=alert_notification&keyword=smoke-operation-queue-assignment-notify&limit=20" >"$WORK_DIR/operation-queue-assignment-notification-operations.json"

python3 - "$WORK_DIR/operation-queue-task-sla-overdue-assign.json" "$WORK_DIR/operation-queue-assignment-notifications.json" "$WORK_DIR/operation-queue-assignment-notification-outbox.json" "$WORK_DIR/operation-queue-assignment-notification-operations.json" <<'PY'
import json
import pathlib
import sys

assign_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
notify_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
outbox_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))

assert assign_payload["code"] == 200, assign_payload
assign = assign_payload["data"]
assert assign["assignedCount"] >= 1, assign
assert assign["owner"] == "ops-task-sla-overdue", assign
assert any(
    item["queueItem"]["source"] == "task_sla"
    and item.get("operationQueueAssignment", {}).get("owner") == "ops-task-sla-overdue"
    and item.get("operationQueueAssignment", {}).get("remark") == "smoke-operation-queue-task-sla-overdue-assign"
    for item in assign["items"]
), assign["items"]

assert notify_payload["code"] == 200, notify_payload
notify = notify_payload["data"]
assert notify["filters"]["source"] == "task_sla", notify["filters"]
assert notify["filters"]["owner"] == "ops-task-sla-overdue", notify["filters"]
assert notify["filters"]["dueState"] == "overdue", notify["filters"]
assert notify["operationQueueAssignmentNotificationKey"] == "operation_queue_assignment_reminder", notify
assert notify["matchedCount"] >= 1, notify
assert notify["eligibleCount"] >= 1, notify
assert notify["enqueuedCount"] >= 1, notify
notifications = notify["notifications"]
assert any(
    item["metric"] == "operation_queue_assignment"
    and item["alertType"] == "operation_queue_assignment_reminder"
    and item["status"] == "pending"
    for item in notifications
), notifications

assert outbox_payload["code"] == 200, outbox_payload
outbox_items = outbox_payload["data"]["notifications"]
assert any(item["alertType"] == "operation_queue_assignment_reminder" and item["status"] == "pending" for item in outbox_items), outbox_items

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert any(
    item["action"] == "saas.admin.operation_queue.assignment_notify"
    and item["targetType"] == "alert_notification"
    and item["remark"] == "smoke-operation-queue-assignment-notify"
    for item in operations
), operations
PY

OPERATION_QUEUE_CLOSE_OPERATION_ID="$(python3 - "$WORK_DIR/operation-queue-task-sla-overdue-assign.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
for item in payload["items"]:
    assignment = item.get("operationQueueAssignment") or {}
    if item["queueItem"]["source"] == "task_sla" and assignment.get("owner") == "ops-task-sla-overdue":
        print(assignment["operationId"])
        break
else:
    raise SystemExit("task SLA overdue assignment operation id not found")
PY
)"

OPERATION_QUEUE_CLOSE_OBJECT_ID="$(python3 - "$WORK_DIR/operation-queue-task-sla-overdue-assign.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
for item in payload["items"]:
    assignment = item.get("operationQueueAssignment") or {}
    if item["queueItem"]["source"] == "task_sla" and assignment.get("owner") == "ops-task-sla-overdue":
        print(assignment["objectId"])
        break
else:
    raise SystemExit("task SLA overdue assignment object id not found")
PY
)"

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"operationId\":$OPERATION_QUEUE_CLOSE_OPERATION_ID,\"closeStatus\":\"resolved\",\"remark\":\"smoke-operation-queue-assignment-close\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignmentClose" >"$WORK_DIR/operation-queue-assignment-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignments?source=task_sla&objectId=$OPERATION_QUEUE_CLOSE_OBJECT_ID&currentOnly=true&dueState=closed&keyword=smoke-operation-queue-assignment-close&limit=20" >"$WORK_DIR/operation-queue-assignment-closed-current.json"

curl -sS -f \
  -X POST \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"remark":"smoke-operation-queue-assignment-after-close"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignmentNotifications?source=task_sla&owner=ops-task-sla-overdue&objectId=$OPERATION_QUEUE_CLOSE_OBJECT_ID&dueState=overdue&limit=20" >"$WORK_DIR/operation-queue-assignment-notifications-after-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueue?source=task_sla&owner=ops-task-sla-overdue&keyword=$OPERATION_QUEUE_CLOSE_OBJECT_ID&tenantLimit=200&limit=20&warningHours=4&overdueHours=24" >"$WORK_DIR/operation-queue-after-assignment-close.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?action=saas.admin.operation_queue.assignment_close&targetType=admin_task&keyword=smoke-operation-queue-assignment-close&limit=20" >"$WORK_DIR/operation-queue-assignment-close-operations.json"

python3 - "$WORK_DIR/operation-queue-assignment-close.json" "$WORK_DIR/operation-queue-assignment-closed-current.json" "$WORK_DIR/operation-queue-assignment-notifications-after-close.json" "$WORK_DIR/operation-queue-after-assignment-close.json" "$WORK_DIR/operation-queue-assignment-close-operations.json" "$OPERATION_QUEUE_CLOSE_OPERATION_ID" "$OPERATION_QUEUE_CLOSE_OBJECT_ID" <<'PY'
import json
import pathlib
import sys

close_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
current_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
notify_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
queue_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
operations_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
previous_operation_id = int(sys.argv[6])
object_id = sys.argv[7]

assert close_payload["code"] == 200, close_payload
closed = close_payload["data"]
assert closed["closed"] is True and closed["alreadyClosed"] is False, closed
assert closed["previous"]["operationId"] == previous_operation_id, closed
assert closed["assignment"]["operationId"] > previous_operation_id, closed
assert closed["assignment"]["objectId"] == object_id, closed
assert closed["assignment"]["status"] == "resolved", closed
assert closed["assignment"]["dueState"] == "closed", closed
assert closed["assignment"]["nextFollowUpAt"] == "", closed
assert closed["assignment"]["remark"] == "smoke-operation-queue-assignment-close", closed

assert current_payload["code"] == 200, current_payload
current = current_payload["data"]
assert current["filters"]["currentOnly"] is True, current
assert current["filters"]["dueState"] == "closed", current
assert current["assignmentCount"] == 1 and current["returnedCount"] == 1, current
assert current["assignments"][0]["status"] == "resolved", current
assert current["assignments"][0]["objectId"] == object_id, current

assert notify_payload["code"] == 200, notify_payload
notify = notify_payload["data"]
assert notify["filters"]["currentOnly"] is True, notify
assert notify["matchedCount"] == 0, notify
assert notify["eligibleCount"] == 0, notify
assert notify["enqueuedCount"] == 0, notify

assert queue_payload["code"] == 200, queue_payload
assert not any(
    item.get("assignment", {}).get("owner") == "ops-task-sla-overdue"
    and item.get("objectId") == object_id
    for item in queue_payload["data"]["items"]
), queue_payload

assert operations_payload["code"] == 200, operations_payload
operations = operations_payload["data"]["operations"]
assert any(
    item["action"] == "saas.admin.operation_queue.assignment_close"
    and item["targetType"] == "admin_task"
    and item["targetId"] == object_id
    and item["remark"] == "smoke-operation-queue-assignment-close"
    for item in operations
), operations
PY

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=tenants&keyword=SaaS%E6%99%AE%E9%80%9A&tenantStatus=1&packageCode=scale&limit=1000" >"$WORK_DIR/export-tenants.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=usage&tenantId=$TENANT_ID&limit=1000" >"$WORK_DIR/export-usage.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=packages" >"$WORK_DIR/export-packages.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=operations&tenantId=$TENANT_ID&action=tenant.package.update&targetType=saas_tenant_package&keyword=scale&limit=1000" >"$WORK_DIR/export-operations.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=billingEvents&tenantId=$TENANT_ID&eventType=renewal&packageCode=scale&keyword=SMOKE-RENEWAL-$TENANT_ID&limit=1000" >"$WORK_DIR/export-billing.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=billingReconciliation&tenantId=$TENANT_ID&keyword=SMOKE-RECONCILE-DRIFT-$TENANT_ID&mismatchOnly=1&limit=1000" >"$WORK_DIR/export-billing-reconciliation.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=billingReconciliationFollowUps&tenantId=$TENANT_ID&status=resolved&owner=finance&keyword=smoke-reconciliation-bulk-close&dueState=closed&limit=1000" >"$WORK_DIR/export-billing-follow-ups.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=billingReconciliationFollowUpOwners&tenantId=$TENANT_ID&status=resolved&owner=finance&keyword=smoke-reconciliation-bulk-close&dueState=closed&limit=1000" >"$WORK_DIR/export-billing-follow-up-owners.csv"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/export?type=tasks&taskType=all&status=applied&limit=1000" >"$WORK_DIR/export-tasks.csv"

python3 - "$WORK_DIR/export-tenants.csv" "$WORK_DIR/export-usage.csv" "$WORK_DIR/export-packages.csv" "$WORK_DIR/export-operations.csv" "$WORK_DIR/export-billing.csv" "$WORK_DIR/export-billing-reconciliation.csv" "$WORK_DIR/export-billing-follow-ups.csv" "$WORK_DIR/export-billing-follow-up-owners.csv" "$WORK_DIR/export-tasks.csv" "$TENANT_ID" <<'PY'
import csv
import pathlib
import sys

tenant_path = pathlib.Path(sys.argv[1])
usage_path = pathlib.Path(sys.argv[2])
package_path = pathlib.Path(sys.argv[3])
operation_path = pathlib.Path(sys.argv[4])
billing_path = pathlib.Path(sys.argv[5])
reconciliation_path = pathlib.Path(sys.argv[6])
billing_follow_up_path = pathlib.Path(sys.argv[7])
billing_follow_up_owner_path = pathlib.Path(sys.argv[8])
task_path = pathlib.Path(sys.argv[9])
tenant_id = int(sys.argv[10])

def read_csv(path):
    with path.open("r", encoding="utf-8-sig", newline="") as fh:
        return list(csv.reader(fh))

tenant_rows = read_csv(tenant_path)
usage_rows = read_csv(usage_path)
package_rows = read_csv(package_path)
operation_rows = read_csv(operation_path)
billing_rows = read_csv(billing_path)
reconciliation_rows = read_csv(reconciliation_path)
billing_follow_up_rows = read_csv(billing_follow_up_path)
billing_follow_up_owner_rows = read_csv(billing_follow_up_owner_path)
task_rows = read_csv(task_path)

assert tenant_rows[0][:4] == ["tenantId", "tenantName", "tenantStatus", "packageCode"], tenant_rows[0]
assert tenant_rows[0][-1] == "maxUsageRatio", tenant_rows[0]
assert len(tenant_rows) == 2, tenant_rows
tenant = tenant_rows[1]
assert int(tenant[0]) == tenant_id, tenant
assert tenant[1] == "SaaS普通租户", tenant
assert tenant[3] == "scale", tenant

assert usage_rows[0][:8] == ["tenantId", "tenantName", "tenantStatus", "packageCode", "packageName", "expiresAt", "metric", "metricLabel"], usage_rows[0]
assert usage_rows[0][10] == "usageLimit", usage_rows[0]
usage = next(row for row in usage_rows[1:] if int(row[0]) == tenant_id and row[6] == "users")
assert usage[3] == "scale", usage
assert usage[7] == "子账号数", usage
assert int(usage[9]) >= 1, usage
assert usage[10].isdigit(), usage
assert usage[14] in {"normal", "warning", "exceeded", "unlimited"}, usage

assert package_rows[0][:6] == ["code", "name", "description", "status", "maxCorps", "maxUsers"], package_rows[0]
assert package_rows[0][-1] == "asyncExecutions", package_rows[0]
package = next(row for row in package_rows[1:] if row[0] == "scale")
assert package[1] == "规模版", package
assert package[3] == "1", package
assert int(package[5]) >= 10, package
assert package[22].isdigit(), package

assert operation_rows[0][:4] == ["id", "tenantId", "action", "targetType"], operation_rows[0]
assert len(operation_rows) >= 2, operation_rows
operation = next(row for row in operation_rows[1:] if int(row[1]) == tenant_id and row[2] == "tenant.package.update")
assert operation[3] == "saas_tenant_package", operation
assert '"packageCode":"scale"' in operation[11], operation

assert billing_rows[0][:4] == ["id", "tenantId", "eventType", "packageCode"], billing_rows[0]
assert len(billing_rows) == 2, billing_rows
billing = billing_rows[1]
assert int(billing[1]) == tenant_id, billing
assert billing[2] == "renewal", billing
assert billing[3] == "scale", billing
assert billing[11] == f"SMOKE-RENEWAL-{tenant_id}", billing
assert billing[15], billing

assert reconciliation_rows[0][:5] == ["id", "tenantId", "tenantName", "eventType", "packageCode"], reconciliation_rows[0]
assert reconciliation_rows[0][18:20] == ["reconcileStatus", "mismatchReasons"], reconciliation_rows[0]
assert len(reconciliation_rows) == 2, reconciliation_rows
reconciliation = reconciliation_rows[1]
assert int(reconciliation[1]) == tenant_id, reconciliation
assert reconciliation[4] == "growth", reconciliation
assert reconciliation[12] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", reconciliation
assert reconciliation[14] == "scale", reconciliation
assert reconciliation[18] == "mismatch", reconciliation
assert "package_mismatch" in reconciliation[19], reconciliation
assert "expires_mismatch" in reconciliation[19], reconciliation

assert billing_follow_up_rows[0][:5] == ["operationId", "billingEventId", "tenantId", "tenantName", "status"], billing_follow_up_rows[0]
assert billing_follow_up_rows[0][7:11] == ["dueState", "overdue", "daysUntil", "remark"], billing_follow_up_rows[0]
assert len(billing_follow_up_rows) == 2, billing_follow_up_rows
billing_follow_up = billing_follow_up_rows[1]
assert int(billing_follow_up[2]) == tenant_id, billing_follow_up
assert billing_follow_up[4] == "resolved", billing_follow_up
assert billing_follow_up[5] == "finance", billing_follow_up
assert billing_follow_up[7] == "closed", billing_follow_up
assert billing_follow_up[10] == "smoke-reconciliation-bulk-close", billing_follow_up
assert billing_follow_up[11] == "growth", billing_follow_up
assert billing_follow_up[14] == "880000", billing_follow_up
assert billing_follow_up[15] == "CNY", billing_follow_up
assert billing_follow_up[16] == f"SMOKE-RECONCILE-DRIFT-{tenant_id}", billing_follow_up

assert billing_follow_up_owner_rows[0] == ["owner", "totalCount", "openCount", "pendingCount", "contactedCount", "renewalPendingCount", "resolvedCount", "ignoredCount", "overdueCount", "dueSoonCount", "futureCount", "noDateCount", "closedCount", "latestFollowUpAt", "nextFollowUpAt"], billing_follow_up_owner_rows[0]
assert len(billing_follow_up_owner_rows) == 2, billing_follow_up_owner_rows
billing_owner = billing_follow_up_owner_rows[1]
assert billing_owner[0] == "finance", billing_owner
assert billing_owner[1] == "1", billing_owner
assert billing_owner[2] == "0", billing_owner
assert billing_owner[6] == "1", billing_owner
assert billing_owner[12] == "1", billing_owner
assert billing_owner[13], billing_owner
assert billing_owner[14] == "", billing_owner

assert task_rows[0][:5] == ["id", "taskType", "status", "tenantId", "packageCode"], task_rows[0]
assert task_rows[0][12:15] == ["request", "preview", "result"], task_rows[0]
task_types = {row[1] for row in task_rows[1:]}
assert {"package_sync", "tenant_provision", "tenant_renewal"}.issubset(task_types), task_rows
assert all(row[2] == "applied" for row in task_rows[1:]), task_rows
serialized_tasks = "\n".join(",".join(row) for row in task_rows)
assert "secret989" not in serialized_tasks, serialized_tasks
assert "adminPasswordHash" not in serialized_tasks, serialized_tasks
assert '"hasAdminPasswordHash":true' in serialized_tasks, serialized_tasks
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"reminderDays":10,"maxAttempts":3,"remark":"smoke-customer-success-renewal-notification"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessRenewalNotifications?tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.5" >"$WORK_DIR/customer-success-renewal-notifications.json"

python3 - "$WORK_DIR/customer-success-renewal-notifications.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["matchedCount"] >= 1, data
assert data["enqueuedCount"] + data["skippedExistingCount"] >= 1, data
assert data["channel"] == "webhook", data
assert data["maxAttempts"] == 3, data
assert data["reminderDays"] == 10, data
assert data["remark"] == "smoke-customer-success-renewal-notification", data
notifications = data["notifications"]
if data["enqueuedCount"]:
    target = next(item for item in notifications if item["tenantId"] == tenant_id and item["alertType"] == "tenant_renewal_reminder")
    assert target["status"] == "pending", target
    assert target["metric"] == "tenant_renewal", target
    assert target["metricLabel"] == "租户续费", target
    assert target["channel"] == "webhook", target
    assert "tenant_renewal_reminder" in target["notificationKey"], target
    assert target["message"], target
else:
    skipped = next(item for item in data["skipped"] if item["tenantId"] == tenant_id)
    assert skipped["reason"] == "tenant renewal notification already exists", skipped
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"months\":12,\"amount\":\"0.00\",\"paymentMethod\":\"manual\",\"externalOrderNoPrefix\":\"SMOKE-CS-RENEWAL\",\"remark\":\"smoke-customer-success-renewal-task\",\"forceCreate\":true}" \
  "http://$GO_ADDR/dashboard/saasAdmin/customerSuccessRenewalTasks?tenantLimit=200&limit=20&expiringDays=10&highUsageRatio=0.5" >"$WORK_DIR/customer-success-renewal-tasks.json"

python3 - "$WORK_DIR/customer-success-renewal-tasks.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["matchedCount"] >= 1, data
assert data["createdCount"] >= 1, data
assert data["pendingCount"] >= 1, data
assert data["months"] == 12, data
assert data["remark"] == "smoke-customer-success-renewal-task", data
assert data["forceCreate"] is True, data
tasks = data["tasks"]
target = next(task for task in tasks if task["tenantId"] == tenant_id)
assert target["taskType"] == "tenant_renewal", target
assert target["status"] == "pending", target
assert target["request"]["externalOrderNo"] == f"SMOKE-CS-RENEWAL-{tenant_id}", target
assert target["request"]["remark"] == "smoke-customer-success-renewal-task", target
assert target["preview"]["tenantId"] == tenant_id, target
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"taskType\":\"tenant_renewal\",\"status\":\"pending\",\"tenantId\":$TENANT_ID,\"limit\":20,\"remark\":\"smoke-customer-success-renewal-bulk-apply\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/tenantRenewalTaskBulkApply" >"$WORK_DIR/tenant-renewal-task-bulk-applied.json"

python3 - "$WORK_DIR/tenant-renewal-task-bulk-applied.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[2])

assert payload["code"] == 200, payload
data = payload["data"]
assert data["matchedCount"] >= 1, data
assert data["appliedCount"] >= 1, data
assert data["blockedCount"] == 0, data
assert data["failedCount"] == 0, data
tasks = data["tasks"]
target = next(task for task in tasks if task["tenantId"] == tenant_id and task["taskType"] == "tenant_renewal")
assert target["status"] == "applied", target
assert target["result"]["tenantId"] == tenant_id, target
assert target["result"]["billingEventId"] > 0, target
assert data["filters"]["taskType"] == "tenant_renewal", data
assert data["filters"]["status"] == "pending", data
PY

curl -sS -f \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d "{\"owner\":\"ops-notification-health\",\"status\":\"contacted\",\"nextFollowUpAt\":\"$OPERATION_QUEUE_ASSIGN_NEXT_AT\",\"remark\":\"smoke-notification-health-assignment\"}" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssign?source=notification_health&keyword=SaaS%E6%99%AE%E9%80%9A%E7%A7%9F%E6%88%B7&healthWindowHours=24&healthStaleMinutes=15&limit=100" >"$WORK_DIR/notification-health-assignment.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueue?source=notification_health&owner=ops-notification-health&healthWindowHours=24&healthStaleMinutes=15&limit=100" >"$WORK_DIR/notification-health-operation-queue.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueOwners?source=notification_health&owner=ops-notification-health&healthWindowHours=24&healthStaleMinutes=15&limit=100" >"$WORK_DIR/notification-health-operation-queue-owners.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignments?source=notification_health&owner=ops-notification-health&tenantId=$TENANT_ID&currentOnly=true&limit=100" >"$WORK_DIR/notification-health-assignments-before-recovery.json"

python3 - "$WORK_DIR/notification-health-assignment.json" "$WORK_DIR/notification-health-operation-queue.json" "$WORK_DIR/notification-health-operation-queue-owners.json" "$WORK_DIR/notification-health-assignments-before-recovery.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

assigned = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
queue = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
owners = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
assignments = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[5])

for payload in (assigned, queue, owners, assignments):
    assert payload["code"] == 200, payload

assigned_data = assigned["data"]
assert assigned_data["filters"]["source"] == "notification_health", assigned_data
assert assigned_data["filters"]["healthWindowHours"] == 24, assigned_data
assert assigned_data["filters"]["healthStaleMinutes"] == 15, assigned_data
assert assigned_data["matchedCount"] == 1, assigned_data
assert assigned_data["assignedCount"] == 1, assigned_data
assert assigned_data["notificationHealthAssignedCount"] == 1, assigned_data
assert assigned_data["queueAssignmentAssignedCount"] == 1, assigned_data
assignment = assigned_data["items"][0]["operationQueueAssignment"]
assert assignment["tenantId"] == tenant_id, assignment
assert assignment["source"] == "notification_health", assignment
assert assignment["objectType"] == "notification_health", assignment
assert assignment["owner"] == "ops-notification-health", assignment

queue_data = queue["data"]
assert queue_data["summary"]["notificationHealthCount"] == 1, queue_data
assert len(queue_data["items"]) == 1, queue_data
assert queue_data["items"][0]["tenantId"] == tenant_id, queue_data
assert queue_data["items"][0]["owner"] == "ops-notification-health", queue_data

owner_data = owners["data"]
assert owner_data["summary"]["notificationHealthCount"] == 1, owner_data
assert len(owner_data["owners"]) == 1, owner_data
assert owner_data["owners"][0]["notificationHealthCount"] == 1, owner_data

assignment_data = assignments["data"]
assert assignment_data["summary"]["notificationHealthCount"] == 1, assignment_data
assert len(assignment_data["assignments"]) == 1, assignment_data
assert assignment_data["assignments"][0]["tenantId"] == tenant_id, assignment_data
PY

mysql_exec "UPDATE mochat_go_saas_alert_notifications SET status = 'delivered', attempts = GREATEST(attempts, 1), delivered_at = NOW(), next_retry_at = NULL, last_error = '', updated_at = NOW() WHERE tenant_id = $TENANT_ID AND status IN ('pending', 'failed', 'dead') AND deleted_at IS NULL"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationHealth?tenantId=$TENANT_ID&state=healthy&windowHours=24&staleMinutes=15&limit=10" >"$WORK_DIR/notification-health-recovered.json"

curl -sS -f \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"remark":"smoke-notification-health-recovered"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationHealthRecovery?tenantId=$TENANT_ID&windowHours=24&staleMinutes=15" >"$WORK_DIR/notification-health-recovery.json"

curl -sS -f \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  -d '{"remark":"smoke-notification-health-recovered-repeat"}' \
  "http://$GO_ADDR/dashboard/saasAdmin/notificationHealthRecovery?tenantId=$TENANT_ID&windowHours=24&staleMinutes=15" >"$WORK_DIR/notification-health-recovery-repeat.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueueAssignments?source=notification_health&tenantId=$TENANT_ID&currentOnly=true&dueState=closed&keyword=smoke-notification-health-recovered&limit=100" >"$WORK_DIR/notification-health-assignments-after-recovery.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operationQueue?source=notification_health&keyword=SaaS%E6%99%AE%E9%80%9A%E7%A7%9F%E6%88%B7&healthWindowHours=24&healthStaleMinutes=15&limit=100" >"$WORK_DIR/notification-health-operation-queue-after-recovery.json"

curl -sS -f \
  -H "Authorization: Bearer $PLATFORM_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAdmin/operations?action=saas.admin.operation_queue.assignment_close&targetType=notification_health&keyword=smoke-notification-health-recovered&limit=20" >"$WORK_DIR/notification-health-recovery-operations.json"

python3 - "$WORK_DIR/notification-health-recovered.json" "$WORK_DIR/notification-health-recovery.json" "$WORK_DIR/notification-health-recovery-repeat.json" "$WORK_DIR/notification-health-assignments-after-recovery.json" "$WORK_DIR/notification-health-operation-queue-after-recovery.json" "$WORK_DIR/notification-health-recovery-operations.json" "$TENANT_ID" <<'PY'
import json
import pathlib
import sys

health = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
recovery = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
repeat = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
assignments = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
queue = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
operations = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
tenant_id = int(sys.argv[7])

for payload in (health, recovery, repeat, assignments, queue, operations):
    assert payload["code"] == 200, payload

health_data = health["data"]
assert health_data["returnedCount"] == 1, health_data
assert health_data["tenants"][0]["tenantId"] == tenant_id, health_data
assert health_data["tenants"][0]["healthState"] == "healthy", health_data

recovery_data = recovery["data"]
assert recovery_data["matchedAssignmentCount"] == 1, recovery_data
assert recovery_data["activeAssignmentCount"] == 1, recovery_data
assert recovery_data["recoveredTenantCount"] == 1, recovery_data
assert recovery_data["closedCount"] == 1, recovery_data
assert recovery_data["unhealthyCount"] == 0, recovery_data
assert recovery_data["noDataCount"] == 0, recovery_data
assert recovery_data["closed"][0]["assignment"]["status"] == "resolved", recovery_data

repeat_data = repeat["data"]
assert repeat_data["closedCount"] == 0, repeat_data
assert repeat_data["alreadyClosedCount"] == 1, repeat_data

assignment_data = assignments["data"]
assert assignment_data["summary"]["notificationHealthCount"] == 1, assignment_data
assert assignment_data["summary"]["closedCount"] == 1, assignment_data
assert assignment_data["assignments"][0]["status"] == "resolved", assignment_data
assert assignment_data["assignments"][0]["dueState"] == "closed", assignment_data

assert queue["data"]["summary"]["notificationHealthCount"] == 0, queue
assert queue["data"]["items"] == [], queue

operation_items = operations["data"]["operations"]
assert len(operation_items) == 1, operations
after = operation_items[0]["after"]
assert after["source"] == "notification_health", after
assert after["closeContext"]["reason"] == "notification_health_recovered", after
assert after["closeContext"]["health"]["healthState"] == "healthy", after
PY

grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/page" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/overview platform_admin_tenant_id=$PLATFORM_TENANT_ID" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/tenantReadiness" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/tenant" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/tenantLifecycle" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/usage" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/risk" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/businessMetrics" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/businessTrends" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/operationQueue" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/operationQueueOwners" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/operationQueueAssignmentClose" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/operationQueueAssign" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/renewalForecast" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/renewalForecastTasks" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/renewalForecastAssign" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/customerSuccess" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/customerSuccessOwners" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/customerSuccessAssign" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/customerSuccessRenewalTasks" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/riskFollowUp" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/riskFollowUps" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/riskFollowUpOwners" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/riskFollowUpBulkClose" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/alerts" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/alertResolve" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/alertBulkResolve" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/notifications" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/notificationHealth" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationHealthRecovery" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/notificationPolicies" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET/POST/PUT /dashboard/saasAdmin/notificationPolicy" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationPolicyTest" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationRetry" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationBulkRetry" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationClose" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationBulkClose" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/packages" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/operations" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/billingEvents" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/billingReconciliation" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUp" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/billingReconciliationFollowUps" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/tasks" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/taskOwners" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/taskSla" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskSlaNotifications" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskCancel" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskBulkCancel" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskBulkReset" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskReset" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/dailyReport" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/export" "$GO_LOG"
grep -q "go SaaS admin route enabled: GET /dashboard/saasAdmin/operationQueueAssignments" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/package" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSync" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSyncTask" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSyncTaskApply" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantStatus" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewal" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewalTask" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewalTaskApply" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/renewalForecastNotifications" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvision" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvisionTask" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvisionTaskApply" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply" "$GO_LOG"
grep -q "go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantPackage" "$GO_LOG"

echo "SaaS admin dashboard smoke passed"
