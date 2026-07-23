#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-employee-statistic-cron-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18092}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13324}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26394}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19058}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-employee-statistic-cron.XXXXXX")"
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

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"
assert_port_free "$WECOM_ADDR"

YESTERDAY="$(python3 - <<'PY'
import datetime
print((datetime.datetime.now() - datetime.timedelta(days=1)).strftime("%Y-%m-%d"))
PY
)"
START_UNIX="$(python3 - <<'PY'
import datetime
import time
yesterday = datetime.datetime.now() - datetime.timedelta(days=1)
start = yesterday.replace(hour=0, minute=0, second=0, microsecond=0)
print(int(time.mktime(start.timetuple())))
PY
)"
END_UNIX="$((START_UNIX + 86399))"
STAT_KEY="EMPLOYEE_STATISTICS_APPLY_11${START_UNIX}"

cat >"$WORK_DIR/fake_wecom.py" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

expected_start = int(sys.argv[3])
expected_end = int(sys.argv[4])


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
            assert query.get("corpid", [""])[0] == "ww-stat", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-token", "expires_in": 7200})
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        payload = json.loads(raw.decode("utf-8") or "{}")
        if path.path == "/cgi-bin/externalcontact/get_user_behavior_data":
            assert payload.get("userid") == ["go-user"], payload
            assert payload.get("start_time") == expected_start, payload
            assert payload.get("end_time") == expected_end, payload
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "behavior_data": [{
                    "stat_time": expected_start,
                    "chat_cnt": 9,
                    "message_cnt": 11,
                    "reply_percentage": 0.83,
                    "avg_reply_time": 25,
                    "negative_feedback_cnt": 2,
                    "new_apply_cnt": 4
                }]
            })
            return
        self.send_response(404)
        self.end_headers()


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" "$START_UNIX" "$END_UNIX" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

TODAY="$(date +%F)"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
DELETE FROM mc_work_employee_statistic WHERE employee_id = 1;
DELETE FROM mc_work_employee WHERE id = 1;
DELETE FROM mc_corp WHERE id = 1;

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  1, 'Go成员统计企业', 'ww-stat', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, '$TODAY 08:00:00', '$TODAY 08:00:00', NULL
);

INSERT INTO mc_work_employee (
  id, wx_user_id, corp_id, name, mobile, status, contact_auth, created_at, updated_at, deleted_at
) VALUES (
  1, 'go-user', 1, 'Go成员', '13800000000', 1, 1, '$TODAY 08:10:00', '$TODAY 08:10:00', NULL
);
SQL

redis_scalar DEL "$STAT_KEY" >/dev/null

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON=1 \
  MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_RUN_ON_START=1 \
  MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS=60 \
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
task = tasks.get("cron-employee-statistic")
assert task, status
assert task.get("status") == "running", task
assert task.get("started_at"), task
PY

wait_mysql_scalar "SELECT CONCAT(corp_id, '|', employee_id, '|', new_apply_cnt, '|', new_contact_cnt, '|', chat_cnt, '|', message_cnt, '|', reply_percentage, '|', avg_reply_time, '|', negative_feedback_cnt, '|', DATE_FORMAT(syn_time, '%Y-%m-%d %H:%i:%s')) FROM mc_work_employee_statistic WHERE employee_id = 1 ORDER BY id DESC LIMIT 1" "1|1|4|4|9|11|83|25|2|$YESTERDAY 00:00:00"
wait_redis_scalar "1" EXISTS "$STAT_KEY"

grep -q "go cron enabled: employeeStatistic" "$GO_LOG"
grep -q "employeeStatistic cron finished: date=$YESTERDAY scanned=1 inserted=1 skipped=0 failed=0" "$GO_LOG"

echo "employeeStatistic cron smoke passed"
