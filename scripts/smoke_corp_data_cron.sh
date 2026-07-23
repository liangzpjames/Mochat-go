#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-corp-data-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18088}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13320}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-corp-data-cron.XXXXXX")"
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

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

TODAY="$(date +%F)"
YESTERDAY="$(date -v-1d +%F 2>/dev/null || date -d 'yesterday' +%F)"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
DELETE FROM mc_corp_day_data WHERE corp_id = 1;
DELETE FROM mc_work_update_time WHERE corp_id = 1 AND type = 6;
DELETE FROM mc_work_contact_room WHERE room_id IN (1, 2);
DELETE FROM mc_work_room WHERE id IN (1, 2);
DELETE FROM mc_work_contact_employee WHERE id IN (1, 2, 3, 4);
DELETE FROM mc_corp WHERE id = 1;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go首页数据企业', 'ww-corp-data', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);

INSERT INTO mc_work_contact_employee (
  id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles,
  add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at
) VALUES
  (1, 1, 101, '', '', '', JSON_ARRAY(), 0, '', '', 1, 1, '$TODAY 12:00:00', '$TODAY 12:00:00', '$TODAY 12:00:00', NULL),
  (2, 1, 102, '', '', '', JSON_ARRAY(), 0, '', '', 1, 2, '$YESTERDAY 12:00:00', '$YESTERDAY 12:00:00', '$TODAY 12:00:00', '$TODAY 12:10:00'),
  (3, 1, 103, '', '', '', JSON_ARRAY(), 0, '', '', 1, 3, '$YESTERDAY 12:00:00', '$YESTERDAY 12:00:00', '$TODAY 12:00:00', '$TODAY 12:20:00'),
  (4, 1, 104, '', '', '', JSON_ARRAY(), 0, '', '', 1, 1, '$YESTERDAY 12:00:00', '$YESTERDAY 12:00:00', '$YESTERDAY 12:00:00', NULL);

INSERT INTO mc_work_room (
  id, corp_id, wx_chat_id, name, owner_id, notice, status,
  create_time, room_max, room_group_id, created_at, updated_at, deleted_at
) VALUES
  (1, 1, 'room-today', '今日新增群', 1, '', 0, '$TODAY 11:00:00', 500, 0, '$TODAY 11:00:00', '$TODAY 11:00:00', NULL),
  (2, 1, 'room-old', '历史群', 1, '', 0, '$YESTERDAY 11:00:00', 500, 0, '$YESTERDAY 11:00:00', '$YESTERDAY 11:00:00', NULL);

INSERT INTO mc_work_contact_room (
  id, wx_user_id, contact_id, employee_id, unionid, room_id,
  join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at
) VALUES
  (1, 'contact-a', 101, 1, '', 1, 3, 2, 1, '$TODAY 12:30:00', '', '$TODAY 12:30:00', '$TODAY 12:30:00', NULL),
  (2, 'contact-b', 102, 1, '', 1, 3, 2, 2, '$YESTERDAY 12:30:00', '$TODAY 13:00:00', '$YESTERDAY 12:30:00', '$TODAY 13:00:00', NULL),
  (3, 'contact-c', 103, 1, '', 2, 3, 2, 1, '$YESTERDAY 12:30:00', '', '$YESTERDAY 12:30:00', '$YESTERDAY 12:30:00', NULL);
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_GO_ENABLE_CORP_DATA_CRON=1 \
  MOCHAT_GO_CORP_DATA_CRON_RUN_ON_START=1 \
  MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS=60 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as fh:
    status = json.load(fh)
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
task = tasks.get("cron-corp-data")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
assert task.get("run_id"), task
PY

wait_mysql_scalar "SELECT CONCAT(add_contact_num, ',', add_room_num, ',', add_into_room_num, ',', loss_contact_num, ',', quit_room_num) FROM mc_corp_day_data WHERE corp_id = 1 AND DATE(date) = '$TODAY' ORDER BY id DESC LIMIT 1" "1,1,1,2,1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 6" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_background_tasks'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_background_task_runs'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_background_task_executions'" "1"
wait_mysql_scalar "SELECT CONCAT(status, ',', start_count, ',', failure_count) FROM mochat_go_background_tasks WHERE name = 'cron-corp-data'" "running,1,0"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_runs WHERE name = 'cron-corp-data' AND status = 'running' AND run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-corp-data' AND kind = 'periodic_tick' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"

grep -q "go cron enabled: corpData" "$GO_LOG"
grep -q "background task recorder enabled: mysql tables=mochat_go_background_tasks,mochat_go_background_task_runs,mochat_go_background_task_executions" "$GO_LOG"
grep -q "corpData cron refreshed: corp=1" "$GO_LOG"

echo "corpData cron smoke passed"
