#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-alert-notification-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18119}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13349}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26470}"
ALERT_WEBHOOK_ADDR="${MOCHAT_ALERT_WEBHOOK_ADDR:-127.0.0.1:19119}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-alert-notification-cron.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
ALERT_WEBHOOK_EVENTS="$WORK_DIR/alert-webhook-events.jsonl"
GO_PID=""
ALERT_WEBHOOK_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$ALERT_WEBHOOK_PID" ] && kill -0 "$ALERT_WEBHOOK_PID" 2>/dev/null; then
    kill "$ALERT_WEBHOOK_PID" 2>/dev/null || true
    wait "$ALERT_WEBHOOK_PID" 2>/dev/null || true
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

wait_file_line_count() {
  local file="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local count
    count="$(python3 - "$file" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
if not path.exists():
    print(0)
else:
    print(len([line for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]))
PY
)"
    if [ "$count" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $file to have $expected lines" >&2
  [ -f "$file" ] && cat "$file" >&2 || true
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$ALERT_WEBHOOK_ADDR"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

python3 - "$ALERT_WEBHOOK_ADDR" "$ALERT_WEBHOOK_EVENTS" <<'PY' &
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

host, port_text = sys.argv[1].rsplit(":", 1)
events_path = pathlib.Path(sys.argv[2])

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("content-length", "0") or "0")
        body = self.rfile.read(length)
        event = json.loads(body.decode("utf-8"))
        with events_path.open("a", encoding="utf-8") as f:
            f.write(json.dumps(event, ensure_ascii=False) + "\n")
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")

    def log_message(self, fmt, *args):
        return

ThreadingHTTPServer((host, int(port_text)), Handler).serve_forever()
PY
ALERT_WEBHOOK_PID="$!"
sleep 1

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
DELETE FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-cron-notification';
INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  (
    'smoke-cron-notification',
    'smoke-cron-alert',
    1,
    'webhook',
    'pending',
    0,
    3,
    '{"Status":{"Metric":"async_executions","TenantID":1,"Current":5,"Limit":1,"Additional":0},"AlertType":"quota_exceeded","Severity":"warning","PeriodKey":"lifetime","Source":"cron.outbox","Message":"Cron待发送告警","Context":{"nonBlocking":true}}',
    '',
    NOW(),
    NULL,
    NOW(),
    NOW(),
    NULL
  );
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON=1 \
  MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS=1 \
  MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_RUN_ON_START=1 \
  MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT=5 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL="http://$ALERT_WEBHOOK_ADDR/saas-alert" \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS=0 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS=127.0.0.0/8 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS=2 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS=1 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE='租户 {{.TenantID}} {{.Metric}} 告警' \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE='当前 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}}' \
  MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" "200"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', max_attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-cron-notification' AND tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL" "delivered/1/3"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'cron-saas-alert-notification-dispatch' AND status = 'running'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_runs WHERE name = 'cron-saas-alert-notification-dispatch' AND status = 'running' AND run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-alert-notification-dispatch' AND kind = 'periodic_tick' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"

curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" "$ALERT_WEBHOOK_EVENTS" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {item["name"]: item for item in status.get("background_tasks", [])}
task = tasks.get("cron-saas-alert-notification-dispatch")
assert task, status
assert task.get("status") == "running", task
assert task.get("run_id"), task

events = [json.loads(line) for line in pathlib.Path(sys.argv[2]).read_text(encoding="utf-8").splitlines() if line.strip()]
event = events[-1]
assert event.get("event") == "saas.quota_alert", event
assert event.get("title") == "租户 1 async_executions 告警", event
assert event.get("body") == "当前 5/1 来源 cron.outbox", event
assert event.get("tenantId") == 1, event
assert event.get("metric") == "async_executions", event
assert event.get("currentValue") == 5, event
assert event.get("limitValue") == 1, event
assert (event.get("context") or {}).get("nonBlocking") is True, event
PY

grep -q "go cron enabled: SaaS alert notification dispatch interval=1s run_on_start=true limit=5" "$GO_LOG"
grep -q "SaaS alert notification dispatch cron finished: scanned=1 delivered=1 deferred=0 suppressed=0 failed=0 dead=0" "$GO_LOG"

echo "SaaS alert notification dispatch cron smoke passed"
