#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-operation-queue-assignment-reminder-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18121}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13351}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-operation-queue-assignment-reminder-cron.XXXXXX")"
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
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$1" | tr -d '\r'
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
    MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
    MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
    MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON=1 \
    MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START=1 \
    MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT=20 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
    MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS=4 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
DELETE FROM mochat_go_saas_alert_notifications
WHERE JSON_UNQUOTE(JSON_EXTRACT(alert_json, '$.AlertType')) = 'operation_queue_assignment_reminder';
DELETE FROM mochat_go_saas_admin_operation_logs
WHERE target_id IN ('7101', '7102', '7103', '7104')
   OR action IN ('saas.admin.operation_queue.assignment_notify', 'saas.admin.operation_queue.assignment_close');

INSERT INTO mochat_go_saas_admin_operation_logs
  (tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name, before_json, after_json, remark, created_at)
VALUES
  (12, 1, 1, 'saas.admin.operation_queue.assign', 'admin_task', '7101', 'Cron 当前逾期认领', NULL,
   JSON_OBJECT('tenantId', 12, 'source', 'task_sla', 'objectType', 'admin_task', 'objectId', '7101', 'targetName', 'Cron 当前逾期认领', 'owner', 'Ops-Overdue', 'status', 'pending', 'nextFollowUpAt', DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 2 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-current-overdue'),
   'cron-current-overdue', DATE_SUB(NOW(), INTERVAL 4 HOUR)),
  (12, 1, 1, 'saas.admin.operation_queue.assign', 'admin_task', '7102', 'Cron 当前即将到期认领', NULL,
   JSON_OBJECT('tenantId', 12, 'source', 'task_sla', 'objectType', 'admin_task', 'objectId', '7102', 'targetName', 'Cron 当前即将到期认领', 'owner', 'Ops-DueSoon', 'status', 'contacted', 'nextFollowUpAt', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 2 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-current-due-soon'),
   'cron-current-due-soon', DATE_SUB(NOW(), INTERVAL 3 HOUR)),
  (12, 1, 1, 'saas.admin.operation_queue.assign', 'admin_task', '7103', 'Cron 已被覆盖的旧逾期认领', NULL,
   JSON_OBJECT('tenantId', 12, 'source', 'task_sla', 'objectType', 'admin_task', 'objectId', '7103', 'targetName', 'Cron 已被覆盖的旧逾期认领', 'owner', 'Ops-Old', 'status', 'pending', 'nextFollowUpAt', DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 3 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-stale-overdue'),
   'cron-stale-overdue', DATE_SUB(NOW(), INTERVAL 2 HOUR)),
  (12, 1, 1, 'saas.admin.operation_queue.assign', 'admin_task', '7103', 'Cron 最新未来认领', NULL,
   JSON_OBJECT('tenantId', 12, 'source', 'task_sla', 'objectType', 'admin_task', 'objectId', '7103', 'targetName', 'Cron 最新未来认领', 'owner', 'Ops-New', 'status', 'contacted', 'nextFollowUpAt', DATE_FORMAT(DATE_ADD(NOW(), INTERVAL 30 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-latest-future'),
   'cron-latest-future', DATE_SUB(NOW(), INTERVAL 1 HOUR)),
  (12, 1, 1, 'saas.admin.operation_queue.assign', 'admin_task', '7104', 'Cron 已关闭前的逾期认领', NULL,
   JSON_OBJECT('tenantId', 12, 'source', 'task_sla', 'objectType', 'admin_task', 'objectId', '7104', 'targetName', 'Cron 已关闭前的逾期认领', 'owner', 'Ops-Closed', 'status', 'pending', 'nextFollowUpAt', DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 4 DAY), '%Y-%m-%d 09:00:00'), 'remark', 'cron-before-close'),
   'cron-before-close', DATE_SUB(NOW(), INTERVAL 50 MINUTE)),
  (12, 0, 1, 'saas.admin.operation_queue.assignment_close', 'admin_task', '7104', 'Cron 已关闭认领', NULL,
   JSON_OBJECT('tenantId', 12, 'source', 'task_sla', 'objectType', 'admin_task', 'objectId', '7104', 'targetName', 'Cron 已关闭认领', 'owner', 'Ops-Closed', 'status', 'resolved', 'nextFollowUpAt', '', 'remark', 'cron-closed'),
   'cron-closed', DATE_SUB(NOW(), INTERVAL 40 MINUTE));
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
start_go

wait_url "http://$GO_ADDR/readyz" "200"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE JSON_UNQUOTE(JSON_EXTRACT(alert_json, '$.AlertType')) = 'operation_queue_assignment_reminder' AND status = 'pending' AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.operation_queue.assignment_notify' AND actor_user_id = 0 AND actor_tenant_id = 1 AND remark = '自动运营待办认领到期提醒' AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.operation_queue.assignment_notify' AND JSON_UNQUOTE(JSON_EXTRACT(after_json, '$.assignment.objectId')) IN ('7103', '7104') AND deleted_at IS NULL" "0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'cron-saas-operation-queue-assignment-reminder' AND status = 'running'" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-operation-queue-assignment-reminder' AND kind = 'periodic_tick' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"

curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {item["name"]: item for item in status.get("background_tasks", [])}
task = tasks.get("cron-saas-operation-queue-assignment-reminder")
assert task, status
assert task.get("status") == "running", task
assert task.get("run_id"), task
PY

grep -q "go cron enabled: SaaS operation queue assignment reminder interval=1h0m0s run_on_start=true limit=20 due_states=overdue,due_soon" "$GO_LOG"
grep -q "SaaS operation queue assignment reminder cron finished: due_states=2 matched=2 eligible=2 enqueued=2" "$GO_LOG"

kill "$GO_PID"
wait "$GO_PID" 2>/dev/null || true
GO_PID=""
start_go

wait_url "http://$GO_ADDR/readyz" "200"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE JSON_UNQUOTE(JSON_EXTRACT(alert_json, '$.AlertType')) = 'operation_queue_assignment_reminder' AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.operation_queue.assignment_notify' AND remark = '自动运营待办认领到期提醒' AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 2, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-operation-queue-assignment-reminder' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"
grep -q "SaaS operation queue assignment reminder cron finished: due_states=2 matched=2 eligible=2 enqueued=0 skipped_existing=2" "$GO_LOG"

echo "SaaS operation queue assignment reminder cron smoke passed"
