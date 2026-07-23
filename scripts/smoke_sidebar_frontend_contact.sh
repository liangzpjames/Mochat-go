#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-sidebar-front-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18141}"
GO_HOST="${GO_ADDR%:*}"
GO_PORT="${GO_ADDR##*:}"
SIDEBAR_ADDR="${MOCHAT_SIDEBAR_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 1))}"
OPERATION_ADDR="${MOCHAT_OPERATION_FRONTEND_ADDR:-$GO_HOST:$((GO_PORT + 2))}"
WECOM_ADDR="${MOCHAT_FAKE_WECOM_ADDR:-$GO_HOST:$((GO_PORT + 3))}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13341}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26391}"
SIDEBAR_SECRET="${MOCHAT_SIDEBAR_JWT_SECRET:-Br3LXhp&Ysha1zRDh}"
PLAYWRIGHT_NODE_PATH="${PLAYWRIGHT_NODE_PATH:-$(npm root -g 2>/dev/null || true)}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-sidebar-front.XXXXXX")"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
PLAYWRIGHT_SCRIPT="$WORK_DIR/sidebar-e2e.js"
PLAYWRIGHT_RESULT="$WORK_DIR/playwright-result.json"
SCREENSHOT_PATH="../output/playwright/sidebar-contact-e2e.png"
SCREENSHOT_PATH_OUT="../output/playwright/sidebar-contact-e2e-screenshot-path.txt"
GO_PID=""
WECOM_PID=""

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  local rc=$?
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$WECOM_PID" ] && kill -0 "$WECOM_PID" 2>/dev/null; then
    kill "$WECOM_PID" 2>/dev/null || true
    wait "$WECOM_PID" 2>/dev/null || true
  fi
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
  exit "$rc"
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

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 90))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
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

wait_service_healthy() {
  local service="$1"
  local deadline=$((SECONDS + ${MOCHAT_SERVICE_HEALTH_TIMEOUT_SECONDS:-360}))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local health_status
    health_status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    if [ "$health_status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  echo "$service did not become healthy" >&2
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  exit 1
}

wait_mysql_scalar() {
  local query="$1"
  local expected="$2"
  local deadline=$((SECONDS + 90))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$query" 2>/dev/null | tr -d '\r' | tail -n 1 || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for MySQL scalar [$query] to become [$expected]" >&2
  [ -f "$GO_LOG" ] && tail -120 "$GO_LOG" >&2 || true
  exit 1
}

if ! NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node -e 'require("playwright")' >/dev/null 2>&1; then
  echo "Node Playwright module is not available; set PLAYWRIGHT_NODE_PATH or install playwright" >&2
  exit 1
fi
if [ ! -f web/sidebar/dist/index.html ]; then
  echo "sidebar dist not found: web/sidebar/dist" >&2
  exit 1
fi

mkdir -p ../output/playwright
assert_port_free "$GO_ADDR"
assert_port_free "$SIDEBAR_ADDR"
assert_port_free "$OPERATION_ADDR"
assert_port_free "$WECOM_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

python3 - "$GO_HOST" "${WECOM_ADDR##*:}" >"$WECOM_LOG" 2>&1 <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

host = sys.argv[1]
port = int(sys.argv[2])

class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/cgi-bin/gettoken":
            self.write_json({"errcode": 0, "errmsg": "ok", "access_token": "sidebar-smoke-token", "expires_in": 7200})
            return
        if path == "/cgi-bin/auth/getuserinfo":
            self.write_json({"errcode": 0, "errmsg": "ok", "userid": "go-sidebar-user"})
            return
        if path in ("/cgi-bin/get_jsapi_ticket", "/cgi-bin/ticket/get"):
            self.write_json({"errcode": 0, "errmsg": "ok", "ticket": "sidebar-smoke-ticket", "expires_in": 7200})
            return
        self.write_json({"errcode": 0, "errmsg": "ok"})

    def do_POST(self):
        self.write_json({"errcode": 0, "errmsg": "ok"})

    def write_json(self, payload):
        raw = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

ThreadingHTTPServer((host, port), Handler).serve_forever()
PY
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/cgi-bin/gettoken" 200

compose down -v --remove-orphans >/dev/null 2>&1 || true
compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis
compose exec -T redis redis-cli FLUSHDB >/dev/null

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<'SQL'
SET NAMES utf8mb4;
INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at)
VALUES (1, 'Go独立侧边栏租户', 1, '', '', '', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), status=VALUES(status), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_user (id, phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, isSuperAdmin)
VALUES (1, '13800000001', '', 'Go独立侧边栏用户', 1, '迁移组', '管理员', NOW(), 1, 1, NOW(), NOW(), 1)
ON DUPLICATE KEY UPDATE phone=VALUES(phone), name=VALUES(name), status=VALUES(status), tenant_id=VALUES(tenant_id), isSuperAdmin=VALUES(isSuperAdmin), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_corp (id, name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at)
VALUES (1, 'Go独立侧边栏企业', 'wx-go-sidebar-corp', '', 'employee-secret', '', 'contact-secret', '', '', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), wx_corpid=VALUES(wx_corpid), employee_secret=VALUES(employee_secret), contact_secret=VALUES(contact_secret), tenant_id=VALUES(tenant_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close, redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at)
VALUES (1, 1, '1000002', 'agent-secret', 'Go独立侧边栏应用', '', '', 0, '', 0, 0, '', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_agent_id=VALUES(wx_agent_id), wx_secret=VALUES(wx_secret), name=VALUES(name), close=VALUES(close), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_employee (id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile, external_position, address, open_user_id, wx_main_department_id, main_department_id, log_user_id, contact_auth, audit_status, created_at, updated_at)
VALUES (1, 'go-sidebar-user', 1, 'Go迁移员工', '13800000001', '客户经理', 1, 'sidebar@example.com', '', '', '', '', NULL, 1, '', NULL, '客户经理', '', '', 0, 0, 1, 2, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_user_id=VALUES(wx_user_id), corp_id=VALUES(corp_id), name=VALUES(name), mobile=VALUES(mobile), status=VALUES(status), log_user_id=VALUES(log_user_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_contact_tag_group (id, wx_group_id, corp_id, group_name, `order`, created_at, updated_at)
VALUES (900001, 'wx-go-sidebar-group', 1, 'Go迁移标签组', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_group_id=VALUES(wx_group_id), corp_id=VALUES(corp_id), group_name=VALUES(group_name), `order`=VALUES(`order`), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_contact_tag (id, wx_contact_tag_id, corp_id, name, `order`, contact_tag_group_id, created_at, updated_at)
VALUES (900001, 'wx-go-sidebar-tag', 1, 'Go迁移标签', 1, 900001, NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_contact_tag_id=VALUES(wx_contact_tag_id), corp_id=VALUES(corp_id), name=VALUES(name), `order`=VALUES(`order`), contact_tag_group_id=VALUES(contact_tag_group_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_contact (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at)
VALUES (900001, 1, 'external-user-900001', 'Go迁移客户', 'Go迁移客户昵称', 'avatar/contact.png', 1, 1, 1, 'go-union-900001', '', '', '', NULL, 'GO-900001', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_external_userid=VALUES(wx_external_userid), name=VALUES(name), nick_name=VALUES(nick_name), avatar=VALUES(avatar), follow_up_status=VALUES(follow_up_status), type=VALUES(type), gender=VALUES(gender), unionid=VALUES(unionid), business_no=VALUES(business_no), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_contact_employee (id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at)
VALUES (900001, 1, 900001, 'Go迁移备注', 'Go迁移描述', '', NULL, 1, 'go-sidebar-user', 'go-sidebar-state', 1, 1, NOW(), NOW(), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), remark=VALUES(remark), description=VALUES(description), remark_corp_name=VALUES(remark_corp_name), remark_mobiles=VALUES(remark_mobiles), add_way=VALUES(add_way), oper_userid=VALUES(oper_userid), state=VALUES(state), corp_id=VALUES(corp_id), status=VALUES(status), create_time=VALUES(create_time), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_room (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at)
VALUES (900001, 1, 'go-room-900001', 'Go迁移客户群', 1, '', 0, NOW(), 500, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), wx_chat_id=VALUES(wx_chat_id), name=VALUES(name), owner_id=VALUES(owner_id), notice=VALUES(notice), status=VALUES(status), create_time=VALUES(create_time), room_max=VALUES(room_max), room_group_id=VALUES(room_group_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_contact_room (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at)
VALUES (900001, 'external-user-900001', 900001, 1, '', 900001, 3, 2, 1, NOW(), '', NOW(), NOW())
ON DUPLICATE KEY UPDATE wx_user_id=VALUES(wx_user_id), contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), unionid=VALUES(unionid), room_id=VALUES(room_id), join_scene=VALUES(join_scene), type=VALUES(type), status=VALUES(status), join_time=VALUES(join_time), out_time=VALUES(out_time), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_work_contact_tag_pivot (id, contact_id, employee_id, contact_tag_id, type, created_at, updated_at)
VALUES (900001, 900001, 1, 900001, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE contact_id=VALUES(contact_id), employee_id=VALUES(employee_id), contact_tag_id=VALUES(contact_tag_id), type=VALUES(type), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_contact_employee_track (id, employee_id, contact_id, event, content, corp_id, created_at, updated_at)
VALUES
  (900001, 1, 900001, 1, 'Go迁移互动轨迹', 1, NOW(), NOW()),
  (900002, 1, 900001, 1, 'Go迁移互动轨迹二', 1, DATE_SUB(NOW(), INTERVAL 1 MINUTE), NOW())
ON DUPLICATE KEY UPDATE employee_id=VALUES(employee_id), contact_id=VALUES(contact_id), event=VALUES(event), content=VALUES(content), corp_id=VALUES(corp_id), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_contact_field (id, name, label, type, options, `order`, status, is_sys, created_at, updated_at)
VALUES
  (900001, 'go_select', 'Go迁移字段', 3, '["选项A","选项B"]', 30, 1, 0, NOW(), NOW()),
  (900002, 'go_picture', 'Go迁移图片字段', 11, NULL, 20, 1, 0, NOW(), NOW()),
  (900003, 'go_checkbox', 'Go迁移多选字段', 2, '["多选A","多选B"]', 10, 1, 0, NOW(), NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name), label=VALUES(label), type=VALUES(type), options=VALUES(options), `order`=VALUES(`order`), status=VALUES(status), is_sys=VALUES(is_sys), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_contact_field_pivot (id, contact_id, contact_field_id, value, created_at, updated_at)
VALUES
  (900001, 900001, 900001, '选项A', NOW(), NOW()),
  (900002, 900001, 900002, 'portrait/a.png', NOW(), NOW()),
  (900003, 900001, 900003, '多选A,多选B', NOW(), NOW())
ON DUPLICATE KEY UPDATE contact_id=VALUES(contact_id), contact_field_id=VALUES(contact_field_id), value=VALUES(value), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_contact_sop (id, corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at)
VALUES (900001, 1, 1, 'Go迁移个人SOP', '{"content":[{"type":"text","value":"Go迁移SOP提醒"}]}', '[1]', 1, '[900001]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), employee_ids=VALUES(employee_ids), state=VALUES(state), contact_ids=VALUES(contact_ids), updated_at=NOW();

INSERT INTO mc_contact_sop (id, corp_id, creator_id, name, setting, employee_ids, state, contact_ids, created_at, updated_at)
VALUES (900002, 1, 1, 'Go迁移周期个人SOP', '[{"content":[{"type":"text","value":"Go迁移周期个人SOP提醒"}],"cycle":"daily","time":"00:00"}]', '[1]', 1, '[900001]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), employee_ids=VALUES(employee_ids), state=VALUES(state), contact_ids=VALUES(contact_ids), updated_at=NOW();

INSERT INTO mc_room_sop (id, corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES (900001, 1, 1, 'Go迁移群SOP', '{"content":[{"type":"text","value":"Go迁移群SOP提醒"}]}', '[900001]', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), room_ids=VALUES(room_ids), state=VALUES(state), updated_at=NOW();

INSERT INTO mc_room_sop (id, corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES (900002, 1, 1, 'Go迁移周期群SOP', '[{"content":[{"type":"text","value":"Go迁移周期群SOP提醒"}],"cycle":"daily","time":"00:00"}]', '[900001]', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), room_ids=VALUES(room_ids), state=VALUES(state), updated_at=NOW();

INSERT INTO mc_room_sop (id, corp_id, creator_id, name, setting, room_ids, state, created_at, updated_at)
VALUES (900003, 1, 1, 'Go迁移入群后群SOP', '[{"content":[{"type":"text","value":"Go迁移入群后群SOP提醒"}],"targetAnchor":"room_join","delayMinutes":0}]', '[900001]', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), creator_id=VALUES(creator_id), name=VALUES(name), setting=VALUES(setting), room_ids=VALUES(room_ids), state=VALUES(state), updated_at=NOW();

INSERT INTO mc_contact_batch_add_import_record (id, corp_id, title, upload_at, allot_employee, tags, import_num, add_num, file_name, file_url, created_at, updated_at)
VALUES (900001, 1, 'Go迁移批量加好友', NOW(), '[{"id":1,"name":"Go迁移员工"}]', '[]', 2, 0, 'batch.csv', '', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), title=VALUES(title), upload_at=VALUES(upload_at), allot_employee=VALUES(allot_employee), tags=VALUES(tags), import_num=VALUES(import_num), add_num=VALUES(add_num), file_name=VALUES(file_name), file_url=VALUES(file_url), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_contact_batch_add_import (id, corp_id, record_id, phone, upload_at, status, add_at, employee_id, allot_num, remark, tags, created_at, updated_at)
VALUES
  (900001, 1, 900001, '13800009001', NOW(), 1, NULL, 1, 1, 'Go迁移待添加客户', '[]', NOW(), NOW()),
  (900002, 1, 900001, '13800009002', NOW(), 2, NULL, 1, 1, 'Go迁移待通过客户', '[]', NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), record_id=VALUES(record_id), phone=VALUES(phone), upload_at=VALUES(upload_at), status=VALUES(status), add_at=VALUES(add_at), employee_id=VALUES(employee_id), allot_num=VALUES(allot_num), remark=VALUES(remark), tags=VALUES(tags), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_medium_group (id, corp_id, name, `order`, created_at, updated_at)
VALUES (900001, 1, 'Go迁移素材分组', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE corp_id=VALUES(corp_id), name=VALUES(name), `order`=VALUES(`order`), updated_at=NOW(), deleted_at=NULL;

INSERT INTO mc_medium (id, media_id, last_upload_time, type, is_sync, content, corp_id, medium_group_id, user_id, user_name, created_at, updated_at)
VALUES
  (900001, '', 0, 1, 1, '{"title":"Go迁移文本素材","content":"Go迁移文本素材内容"}', 1, 0, 1, 'Go迁移员工', NOW(), NOW())
ON DUPLICATE KEY UPDATE media_id=VALUES(media_id), last_upload_time=VALUES(last_upload_time), type=VALUES(type), is_sync=VALUES(is_sync), content=VALUES(content), corp_id=VALUES(corp_id), medium_group_id=VALUES(medium_group_id), user_id=VALUES(user_id), user_name=VALUES(user_name), updated_at=NOW(), deleted_at=NULL;
SQL

mkdir -p "$WORK_DIR/upload/avatar" "$WORK_DIR/upload/portrait"
python3 - "$WORK_DIR/upload/avatar/contact.png" "$WORK_DIR/upload/portrait/a.png" <<'PY'
import base64
import pathlib
import sys

raw = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
for target in sys.argv[1:]:
    pathlib.Path(target).write_bytes(raw)
PY

env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_SIDEBAR_FRONTEND_ADDR="$SIDEBAR_ADDR" \
  MOCHAT_OPERATION_FRONTEND_ADDR="$OPERATION_ADDR" \
  MOCHAT_PHP_UPSTREAM="not-a-valid-upstream-url" \
  MOCHAT_SOURCE_ROOT="$WORK_DIR/should-not-read-php-source" \
  MOCHAT_COMPAT_MANIFEST="$WORK_DIR/should-not-read-manifest.json" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_SIDEBAR_BASE_URL="http://$SIDEBAR_ADDR" \
  MOCHAT_FILE_STORAGE_ROOT="$WORK_DIR/upload" \
  MOCHAT_MYSQL_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_GO_ENABLE_SOP_LOG_CRON=1 \
  MOCHAT_GO_SOP_LOG_CRON_RUN_ON_START=1 \
  MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS=300 \
  MOCHAT_SIMPLE_JWT_SECRET="sidebar-front-secret" \
  MOCHAT_SIMPLE_JWT_PREFIX=mc_jwt_ \
  MOCHAT_SIDEBAR_JWT_SECRET="$SIDEBAR_SECRET" \
  MOCHAT_SIDEBAR_JWT_PREFIX=default \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
wait_url "http://$SIDEBAR_ADDR/contact" 200
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop_log WHERE corp_id = 1 AND contact_sop_id = 900001 AND employee = 'go-sidebar-user' AND contact = 'external-user-900001' AND task LIKE '%Go迁移SOP提醒%';" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = 1 AND room_sop_id = 900001 AND room_id = 900001 AND employee = 'go-sidebar-user' AND task LIKE '%Go迁移群SOP提醒%';" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop_log WHERE corp_id = 1 AND contact_sop_id = 900002 AND employee = 'go-sidebar-user' AND contact = 'external-user-900001' AND task LIKE '%Go迁移周期个人SOP提醒%' AND task LIKE '%_mochatGoOccurrence%';" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = 1 AND room_sop_id = 900002 AND room_id = 900001 AND employee = 'go-sidebar-user' AND task LIKE '%Go迁移周期群SOP提醒%' AND task LIKE '%_mochatGoOccurrence%';" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = 1 AND room_sop_id = 900003 AND room_id = 900001 AND employee = 'go-sidebar-user' AND contact = 'external-user-900001' AND task LIKE '%Go迁移入群后群SOP提醒%';" "1"

cat >"$PLAYWRIGHT_SCRIPT" <<'JS'
const fs = require("fs");
const { chromium } = require("playwright");

const [sidebarAddr, goAddr, resultPath, screenshotPath] = process.argv.slice(2);
const origin = `http://${sidebarAddr}`;
const goOrigin = `http://${goAddr}`;
const contactURL = `${origin}/contact?agentId=1`;
const loginURL = `${origin}/login?agentId=1&target=${encodeURIComponent("/contact?agentId=1")}`;
const authCallbackURL = `${goOrigin}/sidebar/agent/auth?agentId=1&target=${encodeURIComponent(contactURL)}&code=sidebar-oauth-code`;
const contactSOPURL = `http://${sidebarAddr}/contactSop?id=900001&agentId=1`;
const roomSOPURL = `http://${sidebarAddr}/roomSop?id=900001`;
const contactBatchAddURL = `http://${sidebarAddr}/contactBatchAdd?batchId=900001`;
const mediumURL = `http://${sidebarAddr}/medium?agentId=1`;

function normalizeSidebarPath(url) {
  const parsed = new URL(url);
  if (!parsed.pathname.includes("/sidebar/")) {
    return "";
  }
  return parsed.pathname.replace(/^\/undefined/, "");
}

function waitForEndpoint(endpoints, path, timeout = 15000) {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const tick = () => {
      if (endpoints[path]) {
        resolve(endpoints[path]);
        return;
      }
      if (Date.now() - started > timeout) {
        reject(new Error(`timed out waiting for ${path}`));
        return;
      }
      setTimeout(tick, 200);
    };
    tick();
  });
}

function waitForEndpointCount(endpointCounts, path, count, timeout = 15000) {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const tick = () => {
      if ((endpointCounts[path] || 0) >= count) {
        resolve(endpointCounts[path]);
        return;
      }
      if (Date.now() - started > timeout) {
        reject(new Error(`timed out waiting for ${path} count ${count}`));
        return;
      }
      setTimeout(tick, 200);
    };
    tick();
  });
}

async function navigateThroughRedirect(page, sourceURL, targetURL, attempts = 2) {
  const errors = [];
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      await page.goto(sourceURL, { waitUntil: "domcontentloaded", timeout: 30000 });
    } catch (error) {
      errors.push(`attempt ${attempt} goto: ${error.message}`);
    }
    try {
      await page.waitForURL(targetURL, { timeout: 15000 });
      if (page.url() === targetURL) {
        return [];
      }
    } catch (error) {
      errors.push(`attempt ${attempt} target: ${error.message}; current=${page.url()}`);
    }
    if (attempt < attempts) {
      await page.goto("about:blank", { waitUntil: "commit", timeout: 5000 }).catch(() => {});
    }
  }
  return errors;
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  await context.route(/res\.wx\.qq\.com|open\.weixin\.qq\.com|open\.work\.weixin\.qq\.com|hm\.baidu\.com/, route => {
    const url = route.request().url();
    if (url.includes("open.weixin.qq.com/connect/oauth2/authorize")) {
      return route.fulfill({
        status: 200,
        contentType: "text/html",
        body: "<!doctype html><title>fake wecom oauth</title>",
      });
    }
    if (url.includes("jweixin") || url.includes("jwxwork")) {
      return route.fulfill({
        status: 200,
        contentType: "application/javascript",
        body: `
          window.wx = {
            config(opts) { setTimeout(() => this._ready && this._ready(), 0); },
            ready(fn) { this._ready = fn; if (fn) setTimeout(fn, 0); },
            error() {},
            agentConfig(opts) { if (opts && opts.success) setTimeout(() => opts.success({ errMsg: "agentConfig:ok" }), 0); },
            invoke(name, params, cb) {
              if (name === "getContext") return cb({ err_msg: "getContext:ok", entry: "single_chat_tools" });
              if (name === "getCurExternalContact") return cb({ err_msg: "getCurExternalContact:ok", userId: "external-user-900001" });
              return cb({ err_msg: name + ":ok" });
            }
          };
          window.jWeixin = window.wx;
        `,
      });
    }
    return route.fulfill({ status: 204, body: "" });
  });

  const page = await context.newPage();
  const endpoints = {};
  const endpointCounts = {};
  const failures = [];
  let loginOAuthURL = "";
  const authChecks = {
    loginReachedOAuth: false,
    appid: false,
    redirectURI: false,
    responseType: false,
    scope: false,
    callbackLandedOnContact: false,
    tokenCookie: false,
    agentCookie: false,
  };
  page.on("response", async response => {
    const path = normalizeSidebarPath(response.url());
    if (!path) {
      return;
    }
    endpointCounts[path] = (endpointCounts[path] || 0) + 1;
    if (path === "/sidebar/agent/auth") {
      endpoints[path] = {
        status: response.status(),
        originalURL: response.url(),
        location: response.headers().location || "",
      };
      return;
    }
    let payload = null;
    try {
      payload = await response.json();
    } catch (_) {
      payload = null;
    }
    endpoints[path] = {
      status: response.status(),
      originalURL: response.url(),
      payload,
    };
    if (response.status() >= 400) {
      failures.push(`${response.status()} ${response.url()}`);
    }
  });
  page.on("requestfailed", request => {
    const path = normalizeSidebarPath(request.url());
    if (path) {
      failures.push(`request failed ${request.url()} ${request.failure()?.errorText || ""}`);
    }
  });
  page.on("console", message => {
    if (message.type() === "error") {
      failures.push(`console error ${message.text()}`);
    }
  });

  await page.goto(loginURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  await page.waitForURL(url => url.toString().startsWith("https://open.weixin.qq.com/connect/oauth2/authorize"), { timeout: 10000 }).catch(error => {
    failures.push(`sidebar login oauth redirect failed: ${error.message}; current=${page.url()}`);
  });
  loginOAuthURL = page.url();
  try {
    const oauthURL = new URL(loginOAuthURL);
    authChecks.loginReachedOAuth = oauthURL.origin === "https://open.weixin.qq.com" && oauthURL.pathname === "/connect/oauth2/authorize";
    authChecks.appid = oauthURL.searchParams.get("appid") === "wx-go-sidebar-corp";
    authChecks.responseType = oauthURL.searchParams.get("response_type") === "code";
    authChecks.scope = oauthURL.searchParams.get("scope") === "snsapi_base";
    const redirectURI = new URL(oauthURL.searchParams.get("redirect_uri") || "");
    authChecks.redirectURI =
      redirectURI.origin === goOrigin &&
      redirectURI.pathname === "/sidebar/agent/auth" &&
      redirectURI.searchParams.get("agentId") === "1" &&
      redirectURI.searchParams.get("target") === contactURL;
  } catch (error) {
    failures.push(`sidebar login oauth parse failed: ${error.message}`);
  }

  const authNavigationErrors = await navigateThroughRedirect(page, authCallbackURL, contactURL);
  if (authNavigationErrors.length > 0) {
    failures.push(`sidebar auth callback target failed: ${authNavigationErrors.join(" | ")}`);
  }
  authChecks.callbackLandedOnContact = page.url() === contactURL;
  const cookiesAfterAuth = await context.cookies(origin);
  const tokenCookie = cookiesAfterAuth.find(cookie => cookie.name === "token");
  const agentCookie = cookiesAfterAuth.find(cookie => cookie.name === "agentId");
  const tokenValue = decodeURIComponent(tokenCookie?.value || "");
  const rawToken = tokenValue.startsWith("Bearer ") ? tokenValue.slice("Bearer ".length).trim() : tokenValue;
  authChecks.tokenCookie = rawToken.split(".").length === 3;
  authChecks.agentCookie = Boolean(agentCookie && agentCookie.value === "1");

  await Promise.allSettled([
    waitForEndpoint(endpoints, "/sidebar/agent/jssdkConfig"),
    waitForEndpoint(endpoints, "/sidebar/workContact/detail"),
    waitForEndpoint(endpoints, "/sidebar/workContact/show"),
    waitForEndpoint(endpoints, "/sidebar/workContact/track"),
  ]);
  await page.waitForTimeout(1000);
  await page.getByText("客户画像").click({ timeout: 5000 }).catch(error => {
    failures.push(`click 客户画像 failed: ${error.message}`);
  });
  await waitForEndpoint(endpoints, "/sidebar/contactFieldPivot/index", 10000).catch(error => {
    failures.push(error.message);
  });
  await waitForEndpoint(endpoints, "/sidebar/contactSop/getSopTipInfo", 10000).catch(error => {
    failures.push(error.message);
  });
  await page.waitForTimeout(1000);

  await page.screenshot({ path: screenshotPath, fullPage: true });
  const contactText = await page.locator("body").innerText();

  await page.goto(contactSOPURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  await waitForEndpoint(endpoints, "/sidebar/contactSop/getSopInfo", 10000).catch(error => {
    failures.push(error.message);
  });
  await page.waitForTimeout(500);
  const contactSOPText = await page.locator("body").innerText();

  await page.goto(roomSOPURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  await waitForEndpoint(endpoints, "/sidebar/roomSop/getSopInfo", 10000).catch(error => {
    failures.push(error.message);
  });
  await page.waitForTimeout(500);
  const roomSOPTextBefore = await page.locator("body").innerText();
  await page.getByText("我已完成").click({ timeout: 5000 }).catch(error => {
    failures.push(`click 我已完成 failed: ${error.message}`);
  });
  await waitForEndpoint(endpoints, "/sidebar/roomSop/logState", 10000).catch(error => {
    failures.push(error.message);
  });
  await waitForEndpointCount(endpointCounts, "/sidebar/roomSop/getSopInfo", 2, 10000).catch(error => {
    failures.push(error.message);
  });

  await page.goto(contactBatchAddURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  await waitForEndpoint(endpoints, "/sidebar/contactBatchAdd/detail", 10000).catch(error => {
    failures.push(error.message);
  });
  await page.waitForTimeout(500);
  const contactBatchAddText = await page.locator("body").innerText();

  await page.goto(mediumURL, { waitUntil: "domcontentloaded", timeout: 30000 });
  await waitForEndpoint(endpoints, "/sidebar/mediumGroup/index", 10000).catch(error => {
    failures.push(error.message);
  });
  await waitForEndpoint(endpoints, "/sidebar/medium/index", 10000).catch(error => {
    failures.push(error.message);
  });
  await page.waitForTimeout(500);
  const mediumText = await page.locator("body").innerText();
  const required = [
    "/sidebar/agent/jssdkConfig",
    "/sidebar/workContact/detail",
    "/sidebar/workContact/show",
    "/sidebar/workContact/track",
    "/sidebar/contactFieldPivot/index",
    "/sidebar/contactSop/getSopTipInfo",
    "/sidebar/contactSop/getSopInfo",
    "/sidebar/roomSop/getSopInfo",
    "/sidebar/roomSop/logState",
    "/sidebar/contactBatchAdd/detail",
    "/sidebar/mediumGroup/index",
    "/sidebar/medium/index",
  ];
  const missing = required.filter(path => !endpoints[path]);
  const bad = Object.entries(endpoints)
    .filter(([path, entry]) => path !== "/sidebar/agent/auth" && (entry.status >= 400 || !entry.payload || entry.payload.code !== 200))
    .map(([path, entry]) => `${path} status=${entry.status} code=${entry.payload && entry.payload.code}`);

  await browser.close();

  const trackPayload = endpoints["/sidebar/workContact/track"]?.payload?.data;
  const portraitPayload = endpoints["/sidebar/contactFieldPivot/index"]?.payload?.data;
  const contactSOPTipPayload = endpoints["/sidebar/contactSop/getSopTipInfo"]?.payload?.data;
  const contactSOPInfoPayload = endpoints["/sidebar/contactSop/getSopInfo"]?.payload?.data;
  const result = {
    ok: missing.length === 0 && bad.length === 0 && failures.length === 0 && Object.values(authChecks).every(Boolean),
    loginURL,
    loginOAuthURL,
    authCallbackURL,
    contactURL,
    contactSOPURL,
    roomSOPURL,
    contactBatchAddURL,
    mediumURL,
    screenshotPath,
    endpoints,
    endpointCounts,
    missing,
    bad,
    failures,
    authChecks,
    textChecks: {
      customer: contactText.includes("Go迁移客户"),
      remark: contactText.includes("Go迁移备注"),
      room: contactText.includes("Go迁移客户群"),
      tag: contactText.includes("Go迁移标签"),
      track: JSON.stringify(trackPayload || "").includes("Go迁移互动轨迹"),
      portrait: JSON.stringify(portraitPayload || "").includes("Go迁移字段") && JSON.stringify(portraitPayload || "").includes("选项A"),
      contactSOPTipAPI: JSON.stringify(contactSOPTipPayload || "").includes("Go迁移SOP提醒"),
      contactSOPTipPopup: contactText.includes("个人SOP提醒") && contactText.includes("Go迁移SOP提醒"),
      contactSOPInfoAPI: JSON.stringify(contactSOPInfoPayload || "").includes("Go迁移SOP提醒") && JSON.stringify(contactSOPInfoPayload || "").includes("Go迁移客户"),
      contactSOPInfoPage: contactSOPText.includes("Go迁移SOP提醒") && contactSOPText.includes("Go迁移客户"),
      roomSOPTask: roomSOPTextBefore.includes("Go迁移群SOP提醒"),
      roomSOPRoom: roomSOPTextBefore.includes("Go迁移客户群"),
      contactBatchEmployee: contactBatchAddText.includes("Go迁移员工"),
      contactBatchPhoneA: contactBatchAddText.includes("13800009001"),
      contactBatchStatus: contactBatchAddText.includes("待添加") && contactBatchAddText.includes("待通过"),
      mediumGroup: JSON.stringify(endpoints["/sidebar/mediumGroup/index"]?.payload?.data || "").includes("Go迁移素材分组"),
      mediumPageText: mediumText.includes("Go迁移文本素材") && mediumText.includes("Go迁移文本素材内容"),
      mediumAPI: JSON.stringify(endpoints["/sidebar/medium/index"]?.payload?.data || "").includes("Go迁移文本素材"),
    },
  };
  fs.writeFileSync(resultPath, JSON.stringify(result, null, 2));
  if (!result.ok) {
    throw new Error(JSON.stringify(result, null, 2));
  }
  console.log(JSON.stringify(result, null, 2));
})();
JS

NODE_PATH="$PLAYWRIGHT_NODE_PATH${NODE_PATH:+:$NODE_PATH}" node "$PLAYWRIGHT_SCRIPT" "$SIDEBAR_ADDR" "$GO_ADDR" "$PLAYWRIGHT_RESULT" "$SCREENSHOT_PATH"
printf '%s\n' "$SCREENSHOT_PATH" >"$SCREENSHOT_PATH_OUT"

python3 - "$PLAYWRIGHT_RESULT" <<'PY'
import json
import sys

result = json.load(open(sys.argv[1], encoding="utf-8"))
if not result.get("ok"):
    raise SystemExit(json.dumps(result, ensure_ascii=False, indent=2))
for key, value in result.get("textChecks", {}).items():
    if not value:
        raise SystemExit(f"text check failed: {key}")
for key, value in result.get("authChecks", {}).items():
    if not value:
        raise SystemExit(f"auth check failed: {key}")
print("sidebar contactSOP frontend smoke passed")
PY

ROOM_SOP_STATE="$(compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "SELECT state FROM mc_room_sop_log WHERE corp_id = 1 AND room_sop_id = 900001 AND room_id = 900001 AND employee = 'go-sidebar-user' ORDER BY id DESC LIMIT 1;")"
test "$ROOM_SOP_STATE" = "1"
echo "sidebar room SOP frontend smoke passed"
echo "sidebar contact batch add frontend smoke passed"
echo "sidebar medium frontend smoke passed"
