#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-auto-tag-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18098}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13342}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26400}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19080}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-auto-tag.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
TASK_RESPONSE="$WORK_DIR/task-response.json"
ROUTES_RESPONSE="$WORK_DIR/routes.json"
HASH_GO="$WORK_DIR/hash.go"
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
  local deadline=$((SECONDS + 60))
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
        body = self.rfile.read(length).decode("utf-8")
        if path.path == "/cgi-bin/externalcontact/mark_tag":
            with events.open("a", encoding="utf-8") as fh:
                fh.write(json.dumps({"path": path.path, "body": json.loads(body)}, ensure_ascii=False) + "\n")
            self.json_response({"errcode": 0, "errmsg": "ok"})
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
compose exec -T redis redis-cli FLUSHDB >/dev/null

cat >"$HASH_GO" <<'GO'
package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	sum := md5.Sum([]byte("auto123456" + "auto-tag-secret"))
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(sum[:])), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(hash))
}
GO
password_hash="$(env -u GOROOT go run "$HASH_GO")"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mochat_go_background_task_executions WHERE task_name = 'mark-tags';
DELETE FROM mochat_go_background_task_runs WHERE name = 'mark-tags';
DELETE FROM mochat_go_background_tasks WHERE name = 'mark-tags';
DELETE FROM mochat_go_saas_usage_counters WHERE tenant_id = 701 AND metric = 'async_executions';
DELETE FROM mc_contact_employee_track WHERE corp_id = 701 AND contact_id = 7401 AND employee_id = 7301;
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id = 7401 AND employee_id = 7301;
DELETE FROM mc_auto_tag_record WHERE auto_tag_id = 7601 OR contact_id = 7401;
DELETE FROM mc_auto_tag WHERE id = 7601;
DELETE FROM mc_work_message_1 WHERE id IN (7701, 7702, 7703);
DELETE FROM mc_work_contact_tag WHERE id IN (7501, 7502);
DELETE FROM mc_work_contact_employee WHERE id = 7402;
DELETE FROM mc_work_contact WHERE id = 7401;
DELETE FROM mc_work_employee WHERE id = 7301;
DELETE FROM mc_user WHERE id = 7201;
DELETE FROM mc_corp WHERE id = 701;
DELETE FROM mc_tenant WHERE id = 701;

INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at, deleted_at)
VALUES (701, '自动标签租户', 1, '', '', '', '', NOW(), NOW(), NULL);

INSERT INTO mc_corp (
  id, name, wx_corpid, employee_secret, contact_secret,
  token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at
) VALUES (
  701, '自动标签企业', 'ww-auto-tag', 'employee-secret', 'contact-secret',
  'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 701, NOW(), NOW(), NULL
);

INSERT INTO mc_user (
  id, phone, password, name, gender, department, position,
  login_time, status, tenant_id, created_at, updated_at, deleted_at, isSuperAdmin
) VALUES (
  7201, '13900007201', '$password_hash', '自动标签管理员', 1, '迁移组', '管理员',
  NOW(), 1, 701, NOW(), NOW(), NULL, 1
);

INSERT INTO mc_work_employee (
  id, wx_user_id, corp_id, name, mobile, position, gender, email,
  avatar, thumb_avatar, telephone, alias, extattr, status, qr_code,
  external_profile, external_position, address, open_user_id,
  wx_main_department_id, main_department_id, log_user_id, contact_auth,
  audit_status, created_at, updated_at, deleted_at
) VALUES (
  7301, 'auto-employee', 701, '自动标签员工', '13900007301', '', 1, '',
  '', '', '', '', JSON_OBJECT(), 1, '',
  JSON_OBJECT(), '', '', 'open-auto-employee',
  1, 0, 7201, 1, 1, NOW(), NOW(), NULL
);

INSERT INTO mc_work_contact (
  id, corp_id, wx_external_userid, name, nick_name, avatar,
  follow_up_status, type, gender, unionid, position, corp_name,
  corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at
) VALUES (
  7401, 701, 'external-auto-7401', '自动标签客户', '', '',
  1, 1, 0, 'union-auto-7401', '', '', '', JSON_OBJECT(), '', NOW(), NOW(), NULL
);

INSERT INTO mc_work_contact_employee (
  id, employee_id, contact_id, remark, description, remark_corp_name,
  remark_mobiles, add_way, oper_userid, state, corp_id, status,
  create_time, created_at, updated_at, deleted_at
) VALUES (
  7402, 7301, 7401, '', '', '', JSON_ARRAY(), 0, '',
  '', 701, 1, NOW(), NOW(), NOW(), NULL
);

INSERT INTO mc_work_contact_tag (
  id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id,
  created_at, updated_at, deleted_at
) VALUES
  (7501, 'wx-auto-tag-7501', 701, '高意向', 0, 0, NOW(), NOW(), NULL),
  (7502, 'wx-auto-tag-7502', 701, '无关标签', 0, 0, NOW(), NOW(), NULL);

INSERT INTO mc_auto_tag (
  id, type, name, employees, fuzzy_match_keyword, exact_match_keyword,
  tag_rule, tags, on_off, mark_tag_count, tenant_id, corp_id,
  create_user_id, created_at, updated_at, deleted_at
) VALUES (
  7601, 1, '自动标签关键词规则', JSON_ARRAY('auto-employee'),
  JSON_ARRAY('报价'), JSON_ARRAY('马上下单'),
  JSON_ARRAY(JSON_OBJECT(
    'time_type', 1,
    'trigger_count', 2,
    'tags', JSON_ARRAY(JSON_OBJECT('tagid', 7501, 'tagname', '高意向'))
  )),
  JSON_ARRAY('高意向'), 1, 0, 701, 701, 7201, NOW(), NOW(), NULL
);

INSERT INTO mc_work_message_1 (
  id, corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id,
  sender_type, action, type, msg_type, content, content_text, room_id,
  status, msg_data_time, deleted_at, created_at, updated_at
) VALUES
  (7701, 701, 'auto-tag-msg-1', 1, 7301, 1, 7401, 1, 0, 1, 1, JSON_OBJECT('content', '这个报价可以'), '这个报价可以', 0, 0, NOW(), NULL, NOW(), NOW()),
  (7702, 701, 'auto-tag-msg-2', 2, 7301, 1, 7401, 1, 0, 1, 1, JSON_OBJECT('content', '马上下单'), '马上下单', 0, 0, NOW(), NULL, NOW(), NOW()),
  (7703, 701, 'auto-tag-msg-3', 3, 7301, 1, 7401, 1, 0, 1, 1, JSON_OBJECT('content', '只是普通咨询'), '只是普通咨询', 0, 0, NOW(), NULL, NOW(), NOW());
SQL

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=1 \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_AUTO_TAG_DASHBOARD=1 \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="auto-tag-secret" \
  MOCHAT_SIMPLE_JWT_PREFIX="mc_jwt_" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_GO_ENABLE_MARK_TAGS_WORKER=1 \
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
task = tasks.get("mark-tags")
assert task, status
assert task.get("status") == "running", task
PY
grep -q "go migrated route enabled: POST /dashboard/user/auth" "$GO_LOG" || {
  echo "auth route registration log missing" >&2
  tail -160 "$GO_LOG" >&2 || true
  exit 1
}
grep -q "go migrated route enabled: GET /Task/AutoTag/KeyWordTag" "$GO_LOG" || {
  echo "auto tag keyword task route registration log missing" >&2
  tail -160 "$GO_LOG" >&2 || true
  exit 1
}

curl -sS -f "http://$GO_ADDR/compat/routes" >"$ROUTES_RESPONSE"
python3 - "$ROUTES_RESPONSE" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
routes = set(payload.get("migrated_routes") or [])
assert "POST /dashboard/user/auth" in routes, routes
assert "GET /Task/AutoTag/KeyWordTag" in routes, routes
PY

curl -sS -f \
  -H "Content-Type: application/json" \
  -d '{"phone":"13900007201","password":"auto123456"}' \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth.json"
if [ ! -s "$WORK_DIR/auth.json" ]; then
  echo "empty auth response" >&2
  tail -120 "$GO_LOG" >&2 || true
  exit 1
fi
AUTH_TOKEN="$(python3 - "$WORK_DIR/auth.json" <<'PY'
import json
import pathlib
import sys

raw = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
try:
    payload = json.loads(raw)
except json.JSONDecodeError as exc:
    print(f"invalid auth json: {exc}: {raw[:1000]!r}", file=sys.stderr)
    raise
if payload.get("code") != 200:
    raise SystemExit(json.dumps(payload, ensure_ascii=False))
token = ((payload.get("data") or {}).get("token") or "").strip()
assert token, payload
print(token)
PY
)"

curl -sS -f -H "Authorization: Bearer $AUTH_TOKEN" "http://$GO_ADDR/Task/AutoTag/KeyWordTag" >"$TASK_RESPONSE"
if [ ! -s "$TASK_RESPONSE" ]; then
  echo "empty auto tag keyword task response" >&2
  tail -120 "$GO_LOG" >&2 || true
  exit 1
fi
if ! python3 - "$TASK_RESPONSE" <<'PY'
import json
import pathlib
import sys

raw = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
try:
    payload = json.loads(raw)
except json.JSONDecodeError as exc:
    print(f"invalid auto tag keyword task json: {exc}: {raw[:1000]!r}", file=sys.stderr)
    raise
assert payload["code"] == 200, payload
data = payload["data"]
assert data["rule_count"] == 1, data
assert data["pending_messages"] == 3, data
assert data["matched_messages"] == 2, data
assert data["created_records"] == 1, data
assert data["queued_mark_tags"] == 1, data
PY
then
  tail -160 "$GO_LOG" >&2 || true
  exit 1
fi

wait_mysql_scalar "SELECT COUNT(*) FROM mc_auto_tag_record WHERE auto_tag_id = 7601 AND contact_id = 7401 AND employee_id = 7301 AND keyword IN ('报价', '马上下单') AND trigger_count = 2 AND status = 1 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT mark_tag_count FROM mc_auto_tag WHERE id = 7601 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot WHERE contact_id = 7401 AND employee_id = 7301 AND contact_tag_id = 7501 AND type = 1 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_employee_track WHERE corp_id = 701 AND contact_id = 7401 AND employee_id = 7301 AND event = 2 AND content = '系统对该客户打标签【高意向】' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_message_1 WHERE id IN (7701, 7702, 7703) AND status = 1 AND deleted_at IS NULL" "3"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = 701 AND task_name = 'mark-tags' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> ''" "1"
wait_redis_scalar "0" LLEN mochat-go:mark-tags
wait_redis_scalar "0" LLEN mochat-go:mark-tags:processing
wait_redis_scalar "0" LLEN mochat-go:mark-tags:dead
wait_file_line_count "$WECOM_EVENTS" 1

python3 - "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
assert len(events) == 1, events
body = events[0]["body"]
assert body["userid"] == "auto-employee", body
assert body["external_userid"] == "external-auto-7401", body
assert body["add_tag"] == ["wx-auto-tag-7501"], body
PY

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
INSERT INTO mc_auto_tag_record (
  id, auto_tag_id, contact_id, tag_rule_id, wx_external_userid, employee_id,
  keyword, contact_room_id, tags, corp_id, trigger_count, status,
  created_at, updated_at, deleted_at
) VALUES (
  7802, 7601, 7401, 0, 'external-auto-7401', 7301,
  'pending-retry', 0, JSON_ARRAY(JSON_OBJECT('tagid', 7502, 'tagname', '无关标签')), 701, 2, 0,
  NOW(), NOW(), NULL
);
SQL

curl -sS -f -H "Authorization: Bearer $AUTH_TOKEN" "http://$GO_ADDR/Task/AutoTag/KeyWordTag" >"$TASK_RESPONSE"
if ! python3 - "$TASK_RESPONSE" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
data = payload["data"]
assert data["rule_count"] == 1, data
assert data["pending_messages"] == 0, data
assert data["matched_messages"] == 0, data
assert data["created_records"] == 0, data
assert data["queued_mark_tags"] == 1, data
PY
then
  tail -160 "$GO_LOG" >&2 || true
  exit 1
fi

wait_mysql_scalar "SELECT status FROM mc_auto_tag_record WHERE id = 7802 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT mark_tag_count FROM mc_auto_tag WHERE id = 7601 AND deleted_at IS NULL" "2"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot WHERE contact_id = 7401 AND employee_id = 7301 AND contact_tag_id = 7502 AND type = 1 AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_employee_track WHERE corp_id = 701 AND contact_id = 7401 AND employee_id = 7301 AND event = 2 AND content = '系统对该客户打标签【无关标签】' AND deleted_at IS NULL" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE tenant_id = 701 AND task_name = 'mark-tags' AND kind = 'queue_item' AND status = 'succeeded' AND execution_id <> '' AND task_run_id <> ''" "2"
wait_redis_scalar "0" LLEN mochat-go:mark-tags
wait_redis_scalar "0" LLEN mochat-go:mark-tags:processing
wait_redis_scalar "0" LLEN mochat-go:mark-tags:dead
wait_file_line_count "$WECOM_EVENTS" 2

python3 - "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
assert len(events) == 2, events
second = events[1]["body"]
assert second["userid"] == "auto-employee", second
assert second["external_userid"] == "external-auto-7401", second
assert second["add_tag"] == ["wx-auto-tag-7502"], second
PY

grep -q "go migrated route enabled: GET /Task/AutoTag/KeyWordTag" "$GO_LOG"
grep -q "go worker enabled: MarkTags Redis consumer" "$GO_LOG"

echo "auto tag keyword task smoke passed"
