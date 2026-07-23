#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-media-id-worker-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18102}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13346}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26404}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19084}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-media-id-worker.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
STORAGE_ROOT="$WORK_DIR/storage/upload/static"
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
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
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
from urllib.parse import parse_qs, urlparse


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
        parsed = urlparse(self.path)
        if parsed.path == "/cgi-bin/media/upload":
            length = int(self.headers.get("Content-Length", "0"))
            if length:
                self.rfile.read(length)
            media_type = parse_qs(parsed.query).get("type", ["file"])[0]
            with events.open("a", encoding="utf-8") as fh:
                fh.write(json.dumps({"path": parsed.path, "type": media_type}, ensure_ascii=False) + "\n")
            self.json_response({"errcode": 0, "errmsg": "ok", "media_id": f"worker-{media_type}-media-id"})
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

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
"$MIGRATE_BIN" \
  -dsn "mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  -project-root "$PWD" \
  -action baseline >"$WORK_DIR/migrate-baseline.out"
grep -q $'0001_initial_schema\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0006_saas_alerts\tbaselined' "$WORK_DIR/migrate-baseline.out"

mkdir -p "$STORAGE_ROOT/image"
printf 'hello media\n' >"$STORAGE_ROOT/image/hello.txt"

TODAY="$(date +%F)"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
DELETE FROM mochat_go_background_task_executions WHERE task_name = 'medium-media-id-update';
DELETE FROM mochat_go_background_task_runs WHERE name = 'medium-media-id-update';
DELETE FROM mochat_go_background_tasks WHERE name = 'medium-media-id-update';
DELETE FROM mc_medium WHERE id = 1;
DELETE FROM mc_corp WHERE id = 1;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go素材队列企业', 'ww-media-worker', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);

INSERT INTO mc_medium (
  id, media_id, last_upload_time, type, is_sync, content,
  corp_id, medium_group_id, user_id, user_name, created_at, updated_at, deleted_at
) VALUES (
  1, 'old-media-id', 1, 2, 1, JSON_OBJECT('imagePath', 'image/hello.txt', 'imageName', 'hello.txt'),
  1, 0, 1, 'Go测试用户', '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_FILE_STORAGE_ROOT="$STORAGE_ROOT" \
  MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER=1 \
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
task = tasks.get("medium-media-id-update")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
assert task.get("run_id"), task
PY

cat >"$WORK_DIR/event.json" <<'JSON'
[1,[1],"php.media_id_update"]
JSON
compose exec -T redis redis-cli -x RPUSH mochat-go:medium-media-id-update <"$WORK_DIR/event.json" >/dev/null

wait_mysql_scalar "SELECT media_id FROM mc_medium WHERE id = 1" "worker-image-media-id"
wait_mysql_scalar "SELECT IF(last_upload_time > 1, 'yes', 'no') FROM mc_medium WHERE id = 1" "yes"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'medium-media-id-update' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "1/0/runtime"
wait_redis_scalar "0" LLEN mochat-go:medium-media-id-update
wait_redis_scalar "0" LLEN mochat-go:medium-media-id-update:processing
wait_redis_scalar "0" LLEN mochat-go:medium-media-id-update:dead
wait_file_line_count "$WECOM_EVENTS" "1"

grep -q "go worker enabled: MediaIDUpdate Redis consumer" "$GO_LOG"
grep -q "medium media_id update finished: corp=1 scanned=1 updated=1 skipped=0" "$GO_LOG"

echo "mediaIdUpdate worker smoke passed"
