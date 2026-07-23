#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-sensitive-word-monitor-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18114}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13346}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-sensitive-word-monitor-cron.XXXXXX")"
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

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
DELETE FROM mc_sensitive_words_monitor WHERE corp_id = 1;
DELETE FROM mc_sensitive_word WHERE id IN (900001, 900002, 900003);
DELETE FROM mc_sensitive_word_group WHERE id = 900001;
DELETE FROM mc_work_message_id WHERE corp_id = 1 AND type BETWEEN 21 AND 30;
DELETE FROM mc_work_message_1 WHERE corp_id = 1 AND id BETWEEN 900001 AND 900010;
DELETE FROM mc_work_message_2 WHERE corp_id = 1 AND id BETWEEN 900001 AND 900010;
DELETE FROM mc_work_employee WHERE id IN (900001, 900002);
DELETE FROM mc_work_contact WHERE id = 900001;
DELETE FROM mc_work_room WHERE id = 900001;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go敏感词企业', 'ww-sensitive-word', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  wx_corpid = VALUES(wx_corpid),
  employee_secret = VALUES(employee_secret),
  contact_secret = VALUES(contact_secret),
  token = VALUES(token),
  encoding_aes_key = VALUES(encoding_aes_key),
  tenant_id = VALUES(tenant_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_employee (
  id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar,
  thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile,
  external_position, address, open_user_id, wx_main_department_id,
  main_department_id, log_user_id, contact_auth, audit_status, created_at,
  updated_at, deleted_at
) VALUES
  (900001, 'sensitive-employee-a', 1, '敏感词员工A', '13800000001', '', 1, '', '', '', '', '', JSON_OBJECT(), 1, '', JSON_OBJECT(), '', '', '', 1, 1, 1, 1, 1, NOW(), NOW(), NULL),
  (900002, 'sensitive-employee-b', 1, '敏感词员工B', '13800000002', '', 1, '', '', '', '', '', JSON_OBJECT(), 1, '', JSON_OBJECT(), '', '', '', 1, 1, 2, 1, 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  name = VALUES(name),
  wx_user_id = VALUES(wx_user_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_contact (
  id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status,
  type, gender, unionid, position, corp_name, corp_full_name, external_profile,
  business_no, created_at, updated_at, deleted_at
) VALUES (
  900001, 1, 'external-sensitive-a', '敏感词客户A', '客户A', '', 1,
  1, 0, '', '', '', '', JSON_OBJECT(), 'SW-001', NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  wx_external_userid = VALUES(wx_external_userid),
  name = VALUES(name),
  nick_name = VALUES(nick_name),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_room (
  id, corp_id, wx_chat_id, name, owner_id, notice, status,
  create_time, room_max, room_group_id, created_at, updated_at, deleted_at
) VALUES (
  900001, 1, 'room-sensitive-a', '敏感词客户群A', 900001, '', 0,
  '2026-07-06 09:00:00', 500, 0, NOW(), NOW(), NULL
)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  wx_chat_id = VALUES(wx_chat_id),
  name = VALUES(name),
  owner_id = VALUES(owner_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_sensitive_word_group (id, corp_id, name, created_at, updated_at, deleted_at)
VALUES (900001, 1, 'Go敏感词分组', NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE corp_id = VALUES(corp_id), name = VALUES(name), updated_at = NOW(), deleted_at = NULL;

INSERT INTO mc_sensitive_word (id, corp_id, group_id, name, status, created_at, updated_at, deleted_at)
VALUES
  (900001, 1, 900001, '违规词', 1, NOW(), NOW(), NULL),
  (900002, 1, 900001, 'refund', 1, NOW(), NOW(), NULL),
  (900003, 1, 900001, '关闭词', 2, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  group_id = VALUES(group_id),
  name = VALUES(name),
  status = VALUES(status),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_message_1 (
  id, corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id,
  sender_type, action, type, msg_type, content, content_text, room_id,
  status, msg_data_time, created_at, updated_at, deleted_at
) VALUES
  (900001, 1, 'sw-normal', 1, 900001, 1, 900001, 0, 0, 1, 1, JSON_OBJECT('content', '普通消息'), '普通消息', 0, 0, '2026-07-06 09:00:00', NOW(), NOW(), NULL),
  (900002, 1, 'sw-employee', 2, 900001, 1, 900001, 0, 0, 1, 1, JSON_OBJECT('content', '员工发送违规词'), '', 0, 0, '2026-07-06 09:01:00', NOW(), NOW(), NULL),
  (900003, 1, 'sw-contact', 3, 900001, 1, 900001, 1, 0, 1, 1, JSON_OBJECT('text', 'customer asks for Refund'), '', 0, 0, '2026-07-06 09:02:00', NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  msgid = VALUES(msgid),
  seq = VALUES(seq),
  work_employee_id = VALUES(work_employee_id),
  to_user_type = VALUES(to_user_type),
  to_user_id = VALUES(to_user_id),
  sender_type = VALUES(sender_type),
  content = VALUES(content),
  content_text = VALUES(content_text),
  room_id = VALUES(room_id),
  status = VALUES(status),
  msg_data_time = VALUES(msg_data_time),
  updated_at = NOW(),
  deleted_at = NULL;
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON=1 \
  MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_RUN_ON_START=1 \
  MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_INTERVAL_SECONDS=2 \
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
task = tasks.get("cron-sensitive-word-monitor")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
assert task.get("run_id"), task
PY

wait_mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_words_monitor WHERE corp_id = 1 AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT CONCAT(SUM(CASE WHEN source = 2 THEN 1 ELSE 0 END), ',', SUM(CASE WHEN source = 1 THEN 1 ELSE 0 END)) FROM mc_sensitive_words_monitor WHERE corp_id = 1 AND deleted_at IS NULL" "1,1"
wait_mysql_scalar "SELECT CONCAT(source, ',', trigger_user_id, ',', trigger_name, ',', trigger_scenario) FROM mc_sensitive_words_monitor WHERE corp_id = 1 AND sensitive_word_id = 900002 AND deleted_at IS NULL LIMIT 1" "1,900001,敏感词客户A,客户会话"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_words_monitor WHERE corp_id = 1 AND sensitive_word_id = 900001 AND conversation_json LIKE '%违规词%' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT last_id FROM mc_work_message_id WHERE corp_id = 1 AND type = 21 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1" "900003"
sleep 4
wait_mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_words_monitor WHERE corp_id = 1 AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_background_tasks'" "1"
wait_mysql_scalar "SELECT CONCAT(status, ',', start_count, ',', failure_count) FROM mochat_go_background_tasks WHERE name = 'cron-sensitive-word-monitor'" "running,1,0"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 2, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-sensitive-word-monitor' AND kind = 'periodic_tick' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"

grep -q "go cron enabled: sensitiveWordsMonitor" "$GO_LOG"
grep -q "background task recorder enabled: mysql tables=mochat_go_background_tasks,mochat_go_background_task_runs,mochat_go_background_task_executions" "$GO_LOG"
grep -q "sensitiveWordsMonitor cron finished: words=2 corps=1 tables=10 messages=3 monitors_inserted=2" "$GO_LOG"

echo "sensitiveWordsMonitor cron smoke passed"
