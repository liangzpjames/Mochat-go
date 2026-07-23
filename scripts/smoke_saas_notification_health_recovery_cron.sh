#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-notification-health-recovery-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18129}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13359}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
DATABASE="mochat_health_recovery_cron"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-notification-health-recovery-cron.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

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
  local addr="$1"
  local port="${addr##*:}"
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

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" | tr -d '\r'
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

wait_mysql_scalar() {
  local query="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(mysql_scalar "$query" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for MySQL query to return $expected" >&2
  echo "$query" >&2
  echo "last value: $(mysql_scalar "$query" || true)" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

start_go() {
  env -u GOROOT \
    MOCHAT_GO_STANDALONE=1 \
    MOCHAT_GO_ADDR="$GO_ADDR" \
    MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local" \
    MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
    MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON=1 \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START=1 \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS=24 \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES=15 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

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
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0033_saas_admin_operation_logs\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0037_saas_notification_health_index\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0038_saas_notification_slo_index\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0039_saas_subscription_lifecycle\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0040_saas_payment_collection\tapplied_now' "$WORK_DIR/migrate.out"

compose exec -T mysql mariadb -umochat -pmochat_pass "$DATABASE" <<'SQL'
DELETE FROM mochat_go_saas_alert_notifications
WHERE notification_key LIKE 'smoke-health-recovery-cron-%';
DELETE FROM mochat_go_saas_admin_operation_logs
WHERE target_type = 'notification_health' AND target_id IN ('901', '902', '903', '904');
DELETE FROM mc_tenant
WHERE id IN (901, 902, 903, 904);

INSERT INTO mc_tenant (id, name, status, created_at, updated_at, deleted_at)
VALUES
  (901, 'Cron 已恢复租户', 1, NOW(), NOW(), NULL),
  (902, 'Cron 仍异常租户', 1, NOW(), NOW(), NULL),
  (903, 'Cron 无数据租户', 1, NOW(), NOW(), NULL),
  (904, 'Cron 只有抑制租户', 1, NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('smoke-health-recovery-cron-delivered-1', 'smoke-health-recovery-cron-alert-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_SUB(NOW(), INTERVAL 4 MINUTE), DATE_SUB(NOW(), INTERVAL 5 MINUTE), NOW(), NULL),
  ('smoke-health-recovery-cron-delivered-2', 'smoke-health-recovery-cron-alert-901', 901, 'webhook', 'delivered', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), '', NULL, DATE_SUB(NOW(), INTERVAL 3 MINUTE), DATE_SUB(NOW(), INTERVAL 4 MINUTE), NOW(), NULL),
  ('smoke-health-recovery-cron-failed', 'smoke-health-recovery-cron-alert-902', 902, 'webhook', 'failed', 1, 3, JSON_OBJECT('AlertType', 'quota_warning'), 'smoke webhook failed', NOW(), NULL, DATE_SUB(NOW(), INTERVAL 5 MINUTE), NOW(), NULL),
  ('smoke-health-recovery-cron-suppressed', 'smoke-health-recovery-cron-alert-904', 904, 'webhook', 'suppressed', 0, 3, JSON_OBJECT('AlertType', 'quota_warning'), 'policy suppressed', NULL, NULL, DATE_SUB(NOW(), INTERVAL 5 MINUTE), NOW(), NULL);

INSERT INTO mochat_go_saas_admin_operation_logs
  (tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name, before_json, after_json, remark, created_at)
VALUES
  (901, 1, 1, 'saas.admin.operation_queue.assign', 'notification_health', '901', 'Cron 已恢复租户通知健康', NULL,
   JSON_OBJECT('tenantId', 901, 'source', 'notification_health', 'objectType', 'notification_health', 'objectId', '901', 'targetName', 'Cron 已恢复租户通知健康', 'owner', 'Ops-Health', 'status', 'contacted', 'nextFollowUpAt', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 1 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-health-901'),
   'cron-health-901', DATE_SUB(NOW(), INTERVAL 4 HOUR)),
  (902, 1, 1, 'saas.admin.operation_queue.assign', 'notification_health', '902', 'Cron 仍异常租户通知健康', NULL,
   JSON_OBJECT('tenantId', 902, 'source', 'notification_health', 'objectType', 'notification_health', 'objectId', '902', 'targetName', 'Cron 仍异常租户通知健康', 'owner', 'Ops-Health', 'status', 'contacted', 'nextFollowUpAt', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 1 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-health-902'),
   'cron-health-902', DATE_SUB(NOW(), INTERVAL 3 HOUR)),
  (903, 1, 1, 'saas.admin.operation_queue.assign', 'notification_health', '903', 'Cron 无数据租户通知健康', NULL,
   JSON_OBJECT('tenantId', 903, 'source', 'notification_health', 'objectType', 'notification_health', 'objectId', '903', 'targetName', 'Cron 无数据租户通知健康', 'owner', 'Ops-Health', 'status', 'contacted', 'nextFollowUpAt', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 1 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-health-903'),
   'cron-health-903', DATE_SUB(NOW(), INTERVAL 2 HOUR)),
  (904, 1, 1, 'saas.admin.operation_queue.assign', 'notification_health', '904', 'Cron 只有抑制租户通知健康', NULL,
   JSON_OBJECT('tenantId', 904, 'source', 'notification_health', 'objectType', 'notification_health', 'objectId', '904', 'targetName', 'Cron 只有抑制租户通知健康', 'owner', 'Ops-Health', 'status', 'contacted', 'nextFollowUpAt', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 1 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-health-904'),
   'cron-health-904', DATE_SUB(NOW(), INTERVAL 1 HOUR));
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
start_go

wait_url "http://$GO_ADDR/readyz" "200"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.operation_queue.assignment_close' AND target_type = 'notification_health' AND target_id = '901' AND actor_user_id = 0 AND actor_tenant_id = 1 AND remark = '自动通知健康恢复结案' AND JSON_UNQUOTE(JSON_EXTRACT(after_json, '$.closeContext.reason')) = 'notification_health_recovered' AND JSON_UNQUOTE(JSON_EXTRACT(after_json, '$.closeContext.health.healthState')) = 'healthy' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.operation_queue.assignment_close' AND target_type = 'notification_health' AND target_id IN ('902', '903', '904') AND deleted_at IS NULL" "0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'cron-saas-notification-health-recovery' AND status = 'running'" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-notification-health-recovery' AND kind = 'periodic_tick' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"

curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {item["name"]: item for item in status.get("background_tasks", [])}
task = tasks.get("cron-saas-notification-health-recovery")
assert task, status
assert task.get("status") == "running", task
assert task.get("run_id"), task
PY

grep -q "go cron enabled: SaaS notification health recovery interval=1h0m0s run_on_start=true window_hours=24 stale_minutes=15" "$GO_LOG"
grep -q "SaaS notification health recovery cron finished: matched=4 active=4 recovered=1 closed=1 already_closed=0 unhealthy=1 no_data=1 no_delivery_evidence=1 missing_tenant=0" "$GO_LOG"

kill "$GO_PID"
wait "$GO_PID" 2>/dev/null || true
GO_PID=""
start_go

wait_url "http://$GO_ADDR/readyz" "200"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.operation_queue.assignment_close' AND target_type = 'notification_health' AND target_id = '901' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 2, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-notification-health-recovery' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"
grep -q "SaaS notification health recovery cron finished: matched=4 active=3 recovered=0 closed=0 already_closed=1 unhealthy=1 no_data=1 no_delivery_evidence=1 missing_tenant=0" "$GO_LOG"

echo "SaaS notification health recovery cron smoke passed"
