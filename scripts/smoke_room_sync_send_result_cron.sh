#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-sync-send-result-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18095}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13327}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19061}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-sync-send-result-cron.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
GO_PID=""
WECOM_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$WECOM_PID" ] && kill -0 "$WECOM_PID" 2>/dev/null; then
    kill "$WECOM_PID" 2>/dev/null || true
    wait "$WECOM_PID" 2>/dev/null || true
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
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
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
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$WECOM_ADDR"

cat >"$WORK_DIR/fake_wecom.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def json_response(self, data):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        path = urlparse(self.path)
        query = parse_qs(path.query)
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            assert query.get("corpid", [""])[0] == "ww-batch", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-token", "expires_in": 7200})
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads((self.rfile.read(length) if length else b"{}").decode("utf-8") or "{}")
        if path.path == "/cgi-bin/externalcontact/get_groupmsg_task":
            assert payload.get("msgid") == "msg-room", payload
            assert payload.get("limit") == 500, payload
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "task_list": [{"userid": "employee-wx", "status": 1, "send_time": 1783159200}],
                "next_cursor": ""
            })
            return
        if path.path == "/cgi-bin/externalcontact/get_groupmsg_send_result":
            assert payload.get("msgid") == "msg-room", payload
            assert payload.get("userid") == "employee-wx", payload
            assert payload.get("limit") == 500, payload
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "send_list": [
                    {"chat_id": "chat-1", "userid": "employee-wx", "status": 1, "send_time": 1783159300},
                    {"chat_id": "chat-2", "userid": "employee-wx", "status": 3, "send_time": 1783159400}
                ],
                "next_cursor": ""
            })
            return
        self.send_response(404)
        self.end_headers()


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

TODAY="$(date +%F)"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
DELETE FROM mc_room_message_batch_send_result WHERE batch_id = 1;
DELETE FROM mc_room_message_batch_send_employee WHERE batch_id = 1;
DELETE FROM mc_room_message_batch_send WHERE id = 1;
DELETE FROM mc_corp WHERE id = 1;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go群群发企业', 'ww-batch', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);

INSERT INTO mc_room_message_batch_send (
  id, corp_id, user_id, user_name, employee_ids, batch_title, content,
  send_way, send_time, send_room_total, send_employee_total, send_total,
  not_send_total, received_total, not_received_total, send_status,
  created_at, updated_at, deleted_at
) VALUES (
  1, 1, 1, 'Go管理员', JSON_ARRAY(1), 'Go客户群群发', JSON_ARRAY(JSON_OBJECT('msgType', 'text', 'content', 'hello')),
  1, '$TODAY 08:10:00', 2, 1, 0,
  1, 0, 2, 1,
  '$TODAY 08:10:00', '$TODAY 08:10:00', NULL
);

INSERT INTO mc_room_message_batch_send_employee (
  id, batch_id, employee_id, wx_user_id, send_room_total, err_code, err_msg, msg_id,
  send_time, last_sync_time, status, receive_status, created_at, updated_at
) VALUES (
  1, 1, 1, 'employee-wx', 2, '0', '', 'msg-room',
  NULL, '$TODAY 08:11:00', 0, 1, '$TODAY 08:11:00', '$TODAY 08:11:00'
);

INSERT INTO mc_room_message_batch_send_result (
  id, batch_id, employee_id, room_id, room_name, room_employee_num, room_create_time,
  chat_id, user_id, status, send_time, created_at, updated_at
) VALUES
  (1, 1, 1, 1, '客户群1', 3, '$TODAY 08:12:00', 'chat-1', '', 0, 0, '$TODAY 08:12:00', '$TODAY 08:12:00'),
  (2, 1, 1, 2, '客户群2', 5, '$TODAY 08:12:00', 'chat-2', '', 0, 0, '$TODAY 08:12:00', '$TODAY 08:12:00');
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON=1 \
  MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_RUN_ON_START=1 \
  MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS=60 \
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
task = tasks.get("cron-room-sync-send-result")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
PY

wait_mysql_scalar "SELECT CONCAT(status, '|', err_code, '|', err_msg) FROM mc_room_message_batch_send_employee WHERE id = 1" "1|0|ok"
wait_mysql_scalar "SELECT GROUP_CONCAT(CONCAT(chat_id, ':', status, ':', user_id, ':', send_time) ORDER BY id SEPARATOR '|') FROM mc_room_message_batch_send_result WHERE batch_id = 1" "chat-1:1:employee-wx:1783159300|chat-2:3:employee-wx:1783159400"
wait_mysql_scalar "SELECT CONCAT(send_total, '|', not_send_total, '|', received_total, '|', not_received_total) FROM mc_room_message_batch_send WHERE id = 1" "1|0|1|1"

grep -q "go cron enabled: RoomSyncSendResultTask" "$GO_LOG"
grep -q "RoomSyncSendResultTask cron finished: scanned=1 updated=1 skipped=0 failed=0" "$GO_LOG"

echo "RoomSyncSendResultTask cron smoke passed"
