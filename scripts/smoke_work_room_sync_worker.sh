#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-work-room-sync-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18099}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13343}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26401}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19081}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-work-room-sync.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
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
  [ -f "$WECOM_LOG" ] && tail -80 "$WECOM_LOG" >&2 || true
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
  [ -f "$WECOM_LOG" ] && tail -80 "$WECOM_LOG" >&2 || true
  exit 1
}

redis_scalar() {
  compose exec -T redis redis-cli --raw "$@" | tr -d '\r'
}

wait_redis_scalar() {
  local expected="$1"
  shift
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(redis_scalar "$@" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for Redis command to return $expected" >&2
  echo "redis-cli $*" >&2
  echo "last value: $(redis_scalar "$@" || true)" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

wait_file_line_count() {
  local file="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value="0"
    if [ -f "$file" ]; then
      value="$(wc -l <"$file" | tr -d ' ')"
    fi
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $file to contain $expected lines" >&2
  [ -f "$file" ] && cat "$file" >&2 || true
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  [ -f "$WECOM_LOG" ] && tail -80 "$WECOM_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$WECOM_ADDR"

cat >"$WORK_DIR/fake_wecom.py" <<'PY'
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse


events = pathlib.Path(sys.argv[3])


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

    def record(self, path, body):
        with events.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps({"path": path, "body": body}, ensure_ascii=False) + "\n")

    def do_GET(self):
        path = urlparse(self.path)
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "fake-token", "expires_in": 7200})
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length") or "0")
        raw = self.rfile.read(length).decode("utf-8")
        body = json.loads(raw) if raw else {}
        if path.path == "/cgi-bin/externalcontact/groupchat/list":
            self.record(path.path, body)
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat_list": [{"chat_id": "chat-1", "status": 0}],
                "next_cursor": ""
            })
            return
        if path.path == "/cgi-bin/externalcontact/groupchat/get":
            self.record(path.path, body)
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "group_chat": {
                    "chat_id": "chat-1",
                    "name": "Go客户群",
                    "owner": "owner-user",
                    "notice": "群公告",
                    "create_time": 1783300000,
                    "member_list": [
                        {"userid": "owner-user", "type": 1, "join_time": 1783300100, "join_scene": 1},
                        {"userid": "external-101", "type": 2, "join_time": 1783300200, "join_scene": 3, "unionid": "union-101"}
                    ]
                }
            })
            return
        self.send_response(404)
        self.end_headers()


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" "$WECOM_EVENTS" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
DELETE FROM mochat_go_background_task_executions WHERE task_name = 'work-room-sync';
DELETE FROM mochat_go_background_task_runs WHERE name = 'work-room-sync';
DELETE FROM mochat_go_background_tasks WHERE name = 'work-room-sync';
DELETE FROM mc_work_contact_room WHERE wx_user_id IN ('owner-user', 'external-101');
DELETE FROM mc_work_room WHERE corp_id = 1 OR wx_chat_id = 'chat-1';
DELETE FROM mc_work_contact WHERE id = 101 OR wx_external_userid = 'external-101';
DELETE FROM mc_work_employee WHERE id = 3 OR wx_user_id = 'owner-user';
DELETE FROM mc_corp WHERE id = 1;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go客户群同步企业', 'ww-work-room', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL
);

INSERT INTO mc_work_employee (
  id, wx_user_id, corp_id, name, mobile, position, gender, email,
  avatar, thumb_avatar, telephone, alias, extattr, status, qr_code,
  external_profile, external_position, address, open_user_id,
  wx_main_department_id, main_department_id, log_user_id, contact_auth,
  audit_status, created_at, updated_at, deleted_at
) VALUES (
  3, 'owner-user', 1, '群主', '13600000003', '', 1, '',
  '', '', '', '', JSON_OBJECT(), 1, '',
  JSON_OBJECT(), '', '', 'open-owner-user',
  1, 0, 0, 1, 0, NOW(), NOW(), NULL
);

INSERT INTO mc_work_contact (
  id, corp_id, wx_external_userid, name, nick_name, avatar,
  follow_up_status, type, gender, unionid, position, corp_name,
  corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at
) VALUES (
  101, 1, 'external-101', '客户101', '', '',
  1, 1, 0, 'union-101', '', '', '', JSON_OBJECT(), '', NOW(), NOW(), NULL
);
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER=1 \
  MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS=2 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
task = tasks.get("work-room-sync")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
assert task.get("run_id"), task
PY

cat >"$WORK_DIR/event.json" <<'JSON'
[{"ToUserName":"ww-work-room","ChatId":"chat-1"}]
JSON
compose exec -T redis redis-cli -x RPUSH mochat-go:work-room-sync <"$WORK_DIR/event.json" >/dev/null

wait_file_line_count "$WECOM_EVENTS" 2
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_room WHERE corp_id = 1 AND wx_chat_id = 'chat-1' AND name = 'Go客户群' AND owner_id = 3 AND status = 0 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_room member JOIN mc_work_room room ON room.id = member.room_id WHERE room.wx_chat_id = 'chat-1' AND member.wx_user_id = 'owner-user' AND member.employee_id = 3 AND member.type = 1 AND member.status = 1 AND member.deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_room member JOIN mc_work_room room ON room.id = member.room_id WHERE room.wx_chat_id = 'chat-1' AND member.wx_user_id = 'external-101' AND member.contact_id = 101 AND member.type = 2 AND member.unionid = 'union-101' AND member.status = 1 AND member.deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'work-room-sync' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "1/0/runtime"
wait_redis_scalar "0" LLEN mochat-go:work-room-sync
wait_redis_scalar "0" LLEN mochat-go:work-room-sync:processing
wait_redis_scalar "0" LLEN mochat-go:work-room-sync:dead

python3 - "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
paths = [event["path"] for event in events]
assert paths == ["/cgi-bin/externalcontact/groupchat/get", "/cgi-bin/externalcontact/groupchat/list"], events
assert events[0]["body"]["chat_id"] == "chat-1", events[0]
PY

grep -q "go worker enabled: WorkRoomSync Redis consumer" "$GO_LOG"

echo "work room sync worker smoke passed"
