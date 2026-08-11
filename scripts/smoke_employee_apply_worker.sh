#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-employee-apply-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18087}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13319}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26392}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19054}"
ALERT_WEBHOOK_ADDR="${MOCHAT_ALERT_WEBHOOK_ADDR:-127.0.0.1:19064}"
ALERT_WEBHOOK_SECRET="${MOCHAT_ALERT_WEBHOOK_SECRET:-employee-apply-alert-secret}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-employee-apply.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
ALERT_WEBHOOK_LOG="$WORK_DIR/alert-webhook.log"
ALERT_WEBHOOK_EVENTS="$WORK_DIR/alert-webhook.jsonl"
ALERT_WEBHOOK_REQUESTS="$WORK_DIR/alert-webhook-requests.log"
WECOM_PID=""
ALERT_WEBHOOK_PID=""
GO_PID=""

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
  [ -f "$GO_LOG" ] && tail -80 "$GO_LOG" >&2 || true
  [ -f "$WECOM_LOG" ] && tail -80 "$WECOM_LOG" >&2 || true
  exit 1
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$1" | tr -d '\r'
}

redis_scalar() {
  compose exec -T redis redis-cli --raw "$@" | tr -d '\r'
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
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
  exit 1
}

wait_file_line_count() {
  local file="$1"
  local expected="$2"
  local deadline=$((SECONDS + 30))
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
  [ -f "$ALERT_WEBHOOK_LOG" ] && tail -80 "$ALERT_WEBHOOK_LOG" >&2 || true
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$WECOM_ADDR"
assert_port_free "$ALERT_WEBHOOK_ADDR"

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
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "fake-token", "expires_in": 7200})
            return
        if path.path == "/cgi-bin/department/list":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "department": [
                    {"id": 1, "name": "Go授权总部", "parentid": 0, "order": 100},
                    {"id": 2, "name": "Go授权销售部", "parentid": 1, "order": 90}
                ]
            })
            return
        if path.path == "/cgi-bin/user/list":
            department_id = parse_qs(path.query).get("department_id", [""])[0]
            users = []
            if department_id in {"1", "2"}:
                users.append({
                    "userid": "go-apply-user",
                    "name": "Go授权同步员工",
                    "mobile": "13600000000",
                    "position": "授权同步",
                    "gender": "1",
                    "email": "apply@example.com",
                    "avatar": "https://wecom.example/apply.png",
                    "thumb_avatar": "https://wecom.example/apply-thumb.png",
                    "telephone": "",
                    "alias": "apply",
                    "extattr": {"attrs": []},
                    "status": 1,
                    "qr_code": "https://wecom.example/apply-qr.png",
                    "external_profile": {},
                    "external_position": "授权负责人",
                    "address": "",
                    "open_userid": "open-go-apply-user",
                    "main_department": 2,
                    "department": [2],
                    "is_leader_in_dept": [0],
                    "order": [10]
                })
            self.json_response({"errcode": 0, "errmsg": "ok", "userlist": users})
            return
        if path.path == "/cgi-bin/externalcontact/get_follow_user_list":
            self.json_response({"errcode": 0, "errmsg": "ok", "follow_user": ["go-apply-user"]})
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

cat >"$WORK_DIR/fake_alert_webhook.py" <<'PY'
import hashlib
import hmac
import pathlib
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    events_path = pathlib.Path(sys.argv[3])
    requests_path = pathlib.Path(sys.argv[4])
    secret = sys.argv[5] if len(sys.argv) > 5 else ""
    lock = threading.Lock()
    request_count = 0

    def log_message(self, fmt, *args):
        return

    def do_GET(self):
        if self.path == "/healthz":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.path != "/saas-alert":
            self.send_response(404)
            self.end_headers()
            return
        length = int(self.headers.get("content-length", "0") or "0")
        body_bytes = self.rfile.read(length)
        if self.secret:
            timestamp = self.headers.get("x-mochat-go-timestamp", "")
            signature = self.headers.get("x-mochat-go-signature", "")
            expected = "v1=" + hmac.new(
                self.secret.encode("utf-8"),
                timestamp.encode("utf-8") + b"." + body_bytes,
                hashlib.sha256,
            ).hexdigest()
            if not timestamp or not hmac.compare_digest(signature, expected):
                self.send_response(401)
                self.end_headers()
                return
        with self.lock:
            type(self).request_count += 1
            request_count = type(self).request_count
        if request_count == 1:
            with self.requests_path.open("a", encoding="utf-8") as handle:
                handle.write("1 502\n")
            self.send_response(502)
            self.end_headers()
            return
        body = body_bytes.decode("utf-8")
        with self.events_path.open("a", encoding="utf-8") as handle:
            handle.write(body.strip() + "\n")
        with self.requests_path.open("a", encoding="utf-8") as handle:
            handle.write(f"{request_count} 204\n")
        self.send_response(204)
        self.end_headers()


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_alert_webhook.py" "${ALERT_WEBHOOK_ADDR%:*}" "${ALERT_WEBHOOK_ADDR##*:}" "$ALERT_WEBHOOK_EVENTS" "$ALERT_WEBHOOK_REQUESTS" "$ALERT_WEBHOOK_SECRET" >"$ALERT_WEBHOOK_LOG" 2>&1 &
ALERT_WEBHOOK_PID="$!"
wait_url "http://$ALERT_WEBHOOK_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
"$MIGRATE_BIN" \
  -dsn "mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  -project-root "$PWD" \
  -action baseline >"$WORK_DIR/migrate-baseline.out"
grep -q $'0001_initial_schema\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0006_saas_alerts\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0008_saas_alert_notifications\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0009_contact_batch_add_rbac\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0010_sensitive_words\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0011_sop_rbac\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0012_shop_code\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0013_radar\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0014_auto_tag\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0015_lottery\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0016_room_fission\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0017_room_clock_in\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0018_room_quality\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0019_room_calendar\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0020_room_remind\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0021_room_infinite_pull\tbaselined' "$WORK_DIR/migrate-baseline.out"

"$BOOTSTRAP_BIN" \
  -dsn "mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  -secret "employee-apply-secret" \
  -tenant-id 1 \
  -tenant-name "Go授权租户" \
  -phone "13600000000" \
  -password "smoke123456" \
  -user-name "Go授权超管" \
  -async-executions 1 >/dev/null

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mc_corp (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES (1, 'Go授权企业', 'ww-apply', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL)
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

INSERT INTO mochat_go_saas_usage_counters
  (tenant_id, metric, period_key, used_value, limit_value, updated_by, created_at, updated_at, deleted_at)
VALUES
  (1, 'async_executions', 'lifetime', 0, 1, 'smoke', NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  used_value = 0,
  limit_value = 1,
  updated_by = 'smoke',
  updated_at = NOW(),
  deleted_at = NULL;
SQL

cat >"$WORK_DIR/stale-processing-event.json" <<'JSON'
{"queue":"employee-apply","payloadType":"dashboard.EmployeeApplyEvent.v1","idempotencyKey":"mochat-go:queue-idempotency:employee-apply:smoke-processing-recovery","enqueuedAt":"2000-01-01T00:00:00Z","payload":{"corpIds":[1],"userId":1,"source":"smoke-processing-recovery"},"attempts":0,"processingStartedAt":"2000-01-01T00:00:00Z"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:employee-apply:processing <"$WORK_DIR/stale-processing-event.json" >/dev/null

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="employee-apply-secret" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL="http://$ALERT_WEBHOOK_ADDR/saas-alert" \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS=0 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS=127.0.0.0/8 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET="$ALERT_WEBHOOK_SECRET" \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS=2 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS=2 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS=25 \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE='租户 {{.TenantID}} {{.Metric}} 告警' \
  MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE='当前 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}}' \
  MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS=4 \
  MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS=1 \
  MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER=1 \
  MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
curl -sS -f \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13600000000","password":"smoke123456"}' \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth.json"
AUTH_TOKEN="$(python3 - "$WORK_DIR/auth.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
token = ((payload.get("data") or {}).get("token") or "").strip()
assert token, payload
print(token)
PY
)"
curl -sS -f "http://$GO_ADDR/compat/status" >"$WORK_DIR/status.json"
python3 - "$WORK_DIR/status.json" <<'PY'
import json
import pathlib
import sys

status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tasks = {task.get("name"): task for task in status.get("background_tasks", [])}
task = tasks.get("employee-apply")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
assert task.get("run_id"), task
PY

wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_department WHERE corp_id = 1 AND wx_department_id = 2 AND name = 'Go授权销售部' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee we JOIN mc_work_department wd ON wd.id = we.main_department_id WHERE we.corp_id = 1 AND we.wx_user_id = 'go-apply-user' AND we.name = 'Go授权同步员工' AND we.mobile = '13600000000' AND we.contact_auth = 1 AND wd.wx_department_id = 2 AND we.deleted_at IS NULL AND wd.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_user WHERE phone = '13600000000' AND tenant_id = 1 AND password <> '' AND deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_employee_department wed JOIN mc_work_employee we ON we.id = wed.employee_id JOIN mc_work_department wd ON wd.id = wed.department_id WHERE we.wx_user_id = 'go-apply-user' AND wd.wx_department_id = 2 AND wed.deleted_at IS NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_update_time WHERE corp_id = 1 AND type = 1 AND last_update_time IS NOT NULL;" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_background_task_executions'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'employee-apply' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "1/1/runtime"
wait_redis_scalar "0" LLEN mochat-go:employee-apply
wait_redis_scalar "0" LLEN mochat-go:employee-apply:processing
wait_redis_scalar "0" LLEN mochat-go:employee-apply:dead

cat >"$WORK_DIR/over-quota-event.json" <<'JSON'
{"corpIds":[1],"userId":1,"source":"smoke-over-quota"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:employee-apply <"$WORK_DIR/over-quota-event.json" >/dev/null

wait_mysql_scalar "SELECT IF(COUNT(*) >= 2, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'employee-apply' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "2/1/runtime"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 1 AND metric = 'async_executions' AND alert_type = 'quota_exceeded' AND status = 'open' AND current_value = 2 AND limit_value = 1 AND occurrence_count = 1 AND source = 'worker.queue_item' AND message LIKE '%异步执行量 2/1%'" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', max_attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = '1:async_executions:quota_exceeded:lifetime:webhook' AND tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL" "delivered/1/4"
wait_file_line_count "$ALERT_WEBHOOK_REQUESTS" "2"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "1"
python3 - "$ALERT_WEBHOOK_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
event = events[-1]
assert event.get("event") == "saas.quota_alert", event
assert event.get("title") == "租户 1 async_executions 告警", event
assert event.get("body") == "当前 2/1 来源 worker.queue_item", event
assert event.get("tenantId") == 1, event
assert event.get("metric") == "async_executions", event
assert event.get("currentValue") == 2, event
assert event.get("limitValue") == 1, event
assert (event.get("context") or {}).get("nonBlocking") is True, event
PY

curl -sS -f \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAlert/index?metric=async_executions" >"$WORK_DIR/saas-alerts.json"
python3 - "$WORK_DIR/saas-alerts.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
data = payload.get("data") or {}
page = data.get("page") or {}
items = data.get("list") or []
assert page.get("total") == 1, payload
assert items, payload
item = items[0]
assert item.get("tenantId") == 1, item
assert item.get("status") == "open", item
assert item.get("metric") == "async_executions", item
assert item.get("currentValue") == 2, item
assert item.get("limitValue") == 1, item
assert item.get("source") == "worker.queue_item", item
context = item.get("context") or {}
assert context.get("nonBlocking") is True, item
PY

curl -sS -f "http://$GO_ADDR/dashboard/saasAlert/page" >"$WORK_DIR/saas-alert-page.html"
grep -q "MoChat Go SaaS 告警管理" "$WORK_DIR/saas-alert-page.html"
grep -q "/dashboard/saasAlert/index" "$WORK_DIR/saas-alert-page.html"
grep -q "/dashboard/saasAlert/resolve" "$WORK_DIR/saas-alert-page.html"
grep -q "mochat_go_saas_alert_token" "$WORK_DIR/saas-alert-page.html"

"$MAINTENANCE_BIN" \
  -dsn "mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  -action list-alerts \
  -tenant-id 1 \
  -metric async_executions >"$WORK_DIR/alerts.out"
grep -q $'action\tlist-alerts' "$WORK_DIR/alerts.out"
grep -q $'alerts\t1' "$WORK_DIR/alerts.out"
grep -q $'1\t1\topen\twarning\tasync_executions\t2\t1\t1' "$WORK_DIR/alerts.out"

curl -sS -f \
  -X PUT \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"metric":"async_executions"}' \
  "http://$GO_ADDR/dashboard/saasAlert/resolve" >"$WORK_DIR/dashboard-resolve-alert.json"
python3 - "$WORK_DIR/dashboard-resolve-alert.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
assert (payload.get("data") or {}).get("resolved") is True, payload
PY
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 1 AND metric = 'async_executions' AND alert_type = 'quota_exceeded' AND status = 'resolved' AND resolved_at IS NOT NULL" "1"

curl -sS -f \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://$GO_ADDR/dashboard/saasAlert/index?status=all&metric=async_executions" >"$WORK_DIR/saas-alerts-resolved.json"
python3 - "$WORK_DIR/saas-alerts-resolved.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
items = ((payload.get("data") or {}).get("list") or [])
assert items, payload
assert items[0].get("status") == "resolved", items[0]
PY

cat >"$WORK_DIR/reopen-over-quota-event.json" <<'JSON'
{"corpIds":[1],"userId":1,"source":"smoke-over-quota-reopen"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:employee-apply <"$WORK_DIR/reopen-over-quota-event.json" >/dev/null

wait_mysql_scalar "SELECT IF(COUNT(*) >= 3, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 1 AND task_name = 'employee-apply' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error IS NULL" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "3/1/runtime"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 1 AND metric = 'async_executions' AND alert_type = 'quota_exceeded' AND status = 'open' AND current_value = 3 AND limit_value = 1 AND occurrence_count = 2 AND source = 'worker.queue_item' AND message LIKE '%异步执行量 3/1%'" "1"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', max_attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = '1:async_executions:quota_exceeded:lifetime:webhook' AND tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL" "delivered/1/4"
wait_file_line_count "$ALERT_WEBHOOK_REQUESTS" "3"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "2"
python3 - "$ALERT_WEBHOOK_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
event = events[-1]
assert event.get("title") == "租户 1 async_executions 告警", event
assert event.get("body") == "当前 3/1 来源 worker.queue_item", event
assert event.get("tenantId") == 1, event
assert event.get("metric") == "async_executions", event
assert event.get("currentValue") == 3, event
assert event.get("limitValue") == 1, event
PY

"$MAINTENANCE_BIN" \
  -dsn "mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  -action resolve-alert \
  -tenant-id 1 \
  -metric async_executions >"$WORK_DIR/resolve-alert.out"
grep -q $'action\tresolve-alert' "$WORK_DIR/resolve-alert.out"
grep -q $'resolved\ttrue' "$WORK_DIR/resolve-alert.out"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alerts WHERE tenant_id = 1 AND metric = 'async_executions' AND alert_type = 'quota_exceeded' AND status = 'resolved' AND resolved_at IS NOT NULL" "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  (
    'smoke-maintenance-notification',
    'smoke-maintenance-alert',
    1,
    'webhook',
    'pending',
    0,
    3,
    '{"Status":{"Metric":"async_executions","TenantID":1,"Current":4,"Limit":1,"Additional":0},"AlertType":"quota_exceeded","Severity":"warning","PeriodKey":"lifetime","Source":"maintenance.outbox","Message":"手工待发送告警","Context":{"nonBlocking":true}}',
    '',
    NOW(),
    NULL,
    NOW(),
    NOW(),
    NULL
  )
ON DUPLICATE KEY UPDATE
  status = 'pending',
  attempts = 0,
  alert_json = VALUES(alert_json),
  next_retry_at = NOW(),
  delivered_at = NULL,
  updated_at = NOW(),
  deleted_at = NULL;
SQL

"$MAINTENANCE_BIN" \
  -dsn "mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  -action dispatch-alert-notifications \
  -alert-webhook-url "http://$ALERT_WEBHOOK_ADDR/saas-alert" \
  -alert-webhook-require-https=false \
  -alert-webhook-allowed-cidrs 127.0.0.0/8 \
  -alert-webhook-secret "$ALERT_WEBHOOK_SECRET" \
  -alert-webhook-timeout 2s \
  -alert-webhook-retry-attempts 1 \
  -alert-webhook-title-template '租户 {{.TenantID}} {{.Metric}} 告警' \
  -alert-webhook-body-template '当前 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}}' \
  -alert-notification-retry-delay 1s >"$WORK_DIR/dispatch-alert-notifications.out"
grep -q $'action\tdispatch-alert-notifications' "$WORK_DIR/dispatch-alert-notifications.out"
grep -q $'scanned\t1' "$WORK_DIR/dispatch-alert-notifications.out"
grep -q $'delivered\t1' "$WORK_DIR/dispatch-alert-notifications.out"
wait_mysql_scalar "SELECT CONCAT(status, '/', attempts, '/', max_attempts) FROM mochat_go_saas_alert_notifications WHERE notification_key = 'smoke-maintenance-notification' AND tenant_id = 1 AND channel = 'webhook' AND deleted_at IS NULL" "delivered/1/3"
wait_file_line_count "$ALERT_WEBHOOK_REQUESTS" "4"
wait_file_line_count "$ALERT_WEBHOOK_EVENTS" "3"
python3 - "$ALERT_WEBHOOK_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
event = events[-1]
assert event.get("title") == "租户 1 async_executions 告警", event
assert event.get("body") == "当前 4/1 来源 maintenance.outbox", event
assert event.get("tenantId") == 1, event
assert event.get("currentValue") == 4, event
assert event.get("limitValue") == 1, event
PY

cat >"$WORK_DIR/bad-event.json" <<'JSON'
{"corpIds":[999],"userId":1,"source":"smoke-dead-letter"}
JSON

compose exec -T redis redis-cli -x RPUSH mochat-go:employee-apply <"$WORK_DIR/bad-event.json" >/dev/null

wait_redis_scalar "1" LLEN mochat-go:employee-apply:dead
wait_redis_scalar "0" LLEN mochat-go:employee-apply
wait_redis_scalar "0" LLEN mochat-go:employee-apply:processing
compose exec -T redis redis-cli --raw LINDEX mochat-go:employee-apply:dead 0 >"$WORK_DIR/dead-letter.json"

python3 - "$WORK_DIR/dead-letter.json" <<'PY'
import json
import pathlib
import sys

dead = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
payload = dead.get("payload") or {}
assert dead.get("attempts") == 3, dead
assert payload.get("corpIds") == [999], dead
assert dead.get("lastError", "") == "SYNC_FAILED", dead
assert dead.get("lastFailedAt"), dead
PY
wait_mysql_scalar "SELECT IF(COUNT(*) >= 3, 1, 0) FROM mochat_go_background_task_executions WHERE tenant_id = 0 AND task_name = 'employee-apply' AND kind = 'queue_item' AND status = 'failed' AND execution_id <> '' AND task_run_id <> '' AND started_at IS NOT NULL AND stopped_at IS NOT NULL AND duration_ms IS NOT NULL AND last_error = 'SYNC_FAILED'" "1"
wait_mysql_scalar "SELECT CONCAT(used_value, '/', limit_value, '/', updated_by) FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'async_executions' AND period_key = 'lifetime' AND deleted_at IS NULL" "3/1/runtime"

grep -q "go worker enabled: EmployeeApply Redis consumer" "$GO_LOG"
grep -q "go SaaS alert route enabled: GET /dashboard/saasAlert/page" "$GO_LOG"
grep -q "go SaaS alert route enabled: GET /dashboard/saasAlert/index" "$GO_LOG"
grep -q "go SaaS alert webhook enabled: endpoint_configured=true timeout=2s signed=true retry_attempts=2 retry_delay=25ms templated=true outbox=true outbox_max_attempts=4 outbox_retry_delay=1s" "$GO_LOG"
if grep -q "http://$ALERT_WEBHOOK_ADDR/saas-alert" "$GO_LOG"; then
  echo "SaaS alert webhook URL leaked to runtime log" >&2
  exit 1
fi
grep -q "employee apply recovered processing jobs: 1" "$GO_LOG"
grep -q "async execution SaaS quota exceeded: tenant=1 current=2 limit=1" "$GO_LOG"

echo "employee apply worker smoke passed"
