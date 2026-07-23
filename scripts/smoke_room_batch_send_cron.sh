#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-batch-send-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18098}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13330}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19064}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-batch-send-cron.XXXXXX")"
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
            assert query.get("corpid", [""])[0] == "ww-room-scheduled", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-token", "expires_in": 7200})
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads((self.rfile.read(length) if length else b"{}").decode("utf-8") or "{}")
        if path.path == "/cgi-bin/externalcontact/add_msg_template":
            assert payload.get("chat_type") == "group", payload
            assert payload.get("sender") == "employee-wx", payload
            assert payload.get("chat_id_list") == ["chat-1"], payload
            assert payload.get("text", {}).get("content") == "Go定时客户群群发", payload
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": "msg-room-scheduled"})
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
DUE_TIME="${MOCHAT_ROOM_BATCH_SEND_DUE_TIME:-2000-01-01 00:00:00}"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
DELETE FROM mc_room_message_batch_send_result WHERE batch_id = 1;
DELETE FROM mc_room_message_batch_send_employee WHERE batch_id = 1;
DELETE FROM mc_room_message_batch_send WHERE id = 1;
DELETE FROM mc_work_room WHERE id = 1;
DELETE FROM mc_work_employee WHERE id = 1;
DELETE FROM mc_corp WHERE id = 1;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go定时客户群企业', 'ww-room-scheduled', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);

INSERT INTO mc_work_employee (
  id, wx_user_id, corp_id, name, mobile, status, contact_auth, created_at, updated_at, deleted_at
) VALUES (
  1, 'employee-wx', 1, 'Go成员', '13800000000', 1, 1, '$TODAY 08:10:00', '$TODAY 08:10:00', NULL
);

INSERT INTO mc_work_room (
  id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, created_at, updated_at, deleted_at
) VALUES (
  1, 1, 'chat-1', 'Go客户群', 1, '', 0, '$TODAY 08:15:00', '$TODAY 08:15:00', '$TODAY 08:15:00', NULL
);

INSERT INTO mc_room_message_batch_send (
  id, corp_id, user_id, user_name, employee_ids, batch_title, content,
  send_way, definite_time, send_status, created_at, updated_at, deleted_at
) VALUES (
  1, 1, 1, 'Go管理员', JSON_ARRAY(1), 'Go定时客户群群发', JSON_ARRAY(JSON_OBJECT('msgType', 'text', 'content', 'Go定时客户群群发')),
  2, '$DUE_TIME', 0, '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON=1 \
  MOCHAT_GO_ROOM_BATCH_SEND_CRON_RUN_ON_START=1 \
  MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS=60 \
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
task = tasks.get("cron-room-batch-send")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
PY

wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_message_batch_send_employee WHERE batch_id = 1 AND employee_id = 1 AND wx_user_id = 'employee-wx' AND status = 1 AND receive_status = 1 AND msg_id = 'msg-room-scheduled'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_message_batch_send_result WHERE batch_id = 1 AND employee_id = 1 AND room_id = 1 AND chat_id = 'chat-1'" "1"
wait_mysql_scalar "SELECT CONCAT(send_status, '|', send_employee_total, '|', send_room_total, '|', send_total, '|', not_send_total) FROM mc_room_message_batch_send WHERE id = 1" "1|1|1|0|1"

grep -q "go cron enabled: RoomMessageBatchSend 定时发送" "$GO_LOG"
grep -q "RoomMessageBatchSend scheduled cron finished: scanned=1 sent=1 skipped=0 failed=0" "$GO_LOG"

echo "RoomMessageBatchSend scheduled cron smoke passed"
