#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-work-fission-dashboard-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18117}"
OPERATION_ADDR="${MOCHAT_OPERATION_ADDR:-127.0.0.1:19117}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13357}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26417}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19097}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-work-fission-dashboard.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-work-fission-dashboard-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800002015}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret2015}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-92015}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-92015}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-92016}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-92017}"
CONTACT_ONE_ID="${MOCHAT_SMOKE_CONTACT_ONE_ID:-92018}"
CONTACT_TWO_ID="${MOCHAT_SMOKE_CONTACT_TWO_ID:-92019}"
CONTACT_OTHER_ID="${MOCHAT_SMOKE_CONTACT_OTHER_ID:-92020}"
CONTACT_CHILD_ONE_ID="${MOCHAT_SMOKE_CONTACT_CHILD_ONE_ID:-92021}"
CONTACT_CHILD_TWO_ID="${MOCHAT_SMOKE_CONTACT_CHILD_TWO_ID:-92022}"
FISSION_PARENT_ID="${MOCHAT_SMOKE_FISSION_PARENT_ID:-920151}"
FISSION_CHILD_ONE_ID="${MOCHAT_SMOKE_FISSION_CHILD_ONE_ID:-920152}"
FISSION_CHILD_TWO_ID="${MOCHAT_SMOKE_FISSION_CHILD_TWO_ID:-920153}"

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

api_json() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local out="$4"
  if [ -n "$body" ]; then
    curl -sS -f -X "$method" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "$body" \
      "http://$GO_ADDR$path" >"$out"
  else
    curl -sS -f -X "$method" \
      -H "Authorization: Bearer $TOKEN" \
      "http://$GO_ADDR$path" >"$out"
  fi
  python3 - "$out" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("code") != 200:
    raise SystemExit(json.dumps(payload, ensure_ascii=False))
PY
}

assert_port_free "$GO_ADDR"
assert_port_free "$OPERATION_ADDR"
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
uploadimg_count = 0
contact_way_count = 0
template_count = 0


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def json_response(self, data, status=200):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def record(self, path, query, payload):
        with events.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps({"path": path, "query": query, "body": payload}, ensure_ascii=False) + "\n")

    def do_GET(self):
        path = urlparse(self.path)
        query = parse_qs(path.query)
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            assert query.get("corpid", [""])[0] == "ww-work-fission", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "work-fission-token", "expires_in": 7200})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        global uploadimg_count, contact_way_count, template_count
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        if self.headers.get("Content-Type", "").startswith("application/json"):
            payload = json.loads(raw.decode("utf-8") or "{}")
        else:
            payload = {"multipart": True, "bytes": len(raw)}
        self.record(path.path, query, payload)

        if path.path == "/cgi-bin/media/uploadimg":
            assert query.get("access_token") == "work-fission-token", query
            uploadimg_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "url": f"https://wecom.example/work-fission-upload-{uploadimg_count}.jpg"})
            return
        if path.path == "/cgi-bin/externalcontact/add_contact_way":
            assert query.get("access_token") == "work-fission-token", query
            assert payload.get("type") == 2, payload
            assert payload.get("scene") == 2, payload
            assert payload.get("user") == ["fission-user"], payload
            assert payload.get("skip_verify") is True, payload
            contact_way_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "qr_code": f"https://wecom.example/work-fission-qrcode-{contact_way_count}.png", "config_id": f"cfg-work-fission-{contact_way_count}"})
            return
        if path.path == "/cgi-bin/externalcontact/add_msg_template":
            assert query.get("access_token") == "work-fission-token", query
            assert payload.get("chat_type") == "single", payload
            assert payload.get("external_userid") == ["external-fission-a", "external-fission-b"], payload
            assert payload.get("text", {}).get("content") == "二次邀请文案", payload
            attachments = payload.get("attachments", [])
            assert len(attachments) == 1, payload
            assert attachments[0].get("msgtype") == "link", payload
            assert attachments[0].get("link", {}).get("title") == "二次邀请标题", payload
            template_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": f"msg-work-fission-{template_count}"})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" "$WECOM_EVENTS" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

mkdir -p "$FILE_STORAGE_ROOT/workFission"
python3 - "$FILE_STORAGE_ROOT/workFission" <<'PY'
import base64
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
raw = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
for name in [
    "welcome.png",
    "poster.png",
    "logo.png",
    "push.png",
    "invite.png",
    "invite-again.png",
    "welcome-new.png",
    "avatar-parent.png",
    "avatar-child-a.png",
    "avatar-child-b.png",
]:
    (root / name).write_bytes(raw)
PY

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action baseline >"$WORK_DIR/migrate-baseline.out"
grep -q $'0001_initial_schema\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0002_seed_core_data\tbaselined' "$WORK_DIR/migrate-baseline.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id 1 \
  -tenant-name "裂变活动验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "裂变活动管理员" \
  -role-name "裂变活动超级管理员" \
  -package-code "work-fission-standard" \
  -package-name "裂变活动标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -work-fissions 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_work_fission_contact WHERE id IN ($FISSION_PARENT_ID, $FISSION_CHILD_ONE_ID, $FISSION_CHILD_TWO_ID);
DELETE FROM mc_work_fission_invite WHERE fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_work_fission_push WHERE fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_work_fission_welcome WHERE fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_work_fission_poster WHERE fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_work_fission WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_employee WHERE corp_id = $CORP_ID OR employee_id = $EMPLOYEE_ID OR contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID, $CONTACT_CHILD_ONE_ID, $CONTACT_CHILD_TWO_ID);
DELETE FROM mc_work_contact_tag_pivot WHERE employee_id = $EMPLOYEE_ID OR contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID, $CONTACT_CHILD_ONE_ID, $CONTACT_CHILD_TWO_ID);
DELETE FROM mc_work_contact WHERE id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID, $CONTACT_CHILD_ONE_ID, $CONTACT_CHILD_TWO_ID) OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID;
DELETE FROM mc_work_contact_tag_group WHERE id = $TAG_GROUP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '裂变活动企业', 'ww-work-fission', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'fission-user', $CORP_ID, '裂变活动员工', '$PHONE', 'workFission/avatar-parent.png', 'workFission/avatar-parent.png', '裂变员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'tag-group-work-fission', $CORP_ID, '裂变标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'tag-work-fission', $CORP_ID, '裂变标签', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ONE_ID, $CORP_ID, 'external-fission-a', '裂变目标客户A', '裂变目标A', 'workFission/avatar-parent.png', 1, 1, 1, 'union-fission-a', NOW(), NOW(), NULL),
  ($CONTACT_TWO_ID, $CORP_ID, 'external-fission-b', '裂变目标客户B', '裂变目标B', 'workFission/avatar-child-a.png', 1, 1, 1, 'union-fission-b', NOW(), NOW(), NULL),
  ($CONTACT_OTHER_ID, $CORP_ID, 'external-fission-c', '裂变目标客户C', '裂变目标C', 'workFission/avatar-child-b.png', 1, 1, 2, 'union-fission-c', NOW(), NOW(), NULL),
  ($CONTACT_CHILD_ONE_ID, $CORP_ID, 'external-fission-child-a', '助力客户A', '助力客户A', 'workFission/avatar-child-a.png', 1, 1, 1, 'union-fission-child-a', NOW(), NOW(), NULL),
  ($CONTACT_CHILD_TWO_ID, $CORP_ID, 'external-fission-child-b', '助力客户B', '助力客户B', 'workFission/avatar-child-b.png', 1, 1, 2, 'union-fission-child-b', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, $CONTACT_ONE_ID, '', '', '', JSON_ARRAY(), 1, 'fission-user', '', $CORP_ID, 1, '2026-07-01 10:00:00', NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_TWO_ID, '', '', '', JSON_ARRAY(), 1, 'fission-user', '', $CORP_ID, 1, '2026-07-01 11:00:00', NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_OTHER_ID, '', '', '', JSON_ARRAY(), 1, 'fission-user', '', $CORP_ID, 1, '2026-07-01 12:00:00', NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_OPERATION_BASE_URL="http://$OPERATION_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_FILE_STORAGE_ROOT="$FILE_STORAGE_ROOT" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

curl -sS -f \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"password\":\"$PASSWORD\"}" \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth.json"

TOKEN="$(python3 - "$WORK_DIR/auth.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("code") != 200:
    raise SystemExit(json.dumps(payload, ensure_ascii=False))
print(payload["data"]["token"])
PY
)"

api_json POST "/dashboard/corp/bind" "{\"corpId\":$CORP_ID}" "$WORK_DIR/corp-bind.json"

STORE_BODY="$(python3 - "$EMPLOYEE_ID" "$TAG_ID" <<'PY'
import json
import sys

employee_id = int(sys.argv[1])
tag_id = int(sys.argv[2])
employee = {"id": employee_id, "name": "裂变活动员工", "wxUserId": "fission-user"}
body = {
    "fission": {
        "active_name": "Go裂变活动",
        "service_employees": [employee],
        "auto_pass": True,
        "auto_add_tag": True,
        "contact_tags": [{"id": tag_id, "name": "裂变标签"}],
        "end_time": "2037-03-01 00:00:00",
        "qr_code_invalid": 7,
        "tasks": [{"level": 1, "num": 1, "prize": "一阶奖励"}, {"level": 2, "num": 2, "prize": "二阶奖励"}],
        "new_friend": True,
        "delete_invalid": False,
        "receive_prize": 0,
        "receive_prize_employees": [employee],
        "receive_links": [{"url": "https://gift.example/work-fission"}],
    },
    "welcome": {
        "msg_text": "欢迎加入裂变活动",
        "link_title": "欢迎标题",
        "link_desc": "欢迎描述",
        "link_cover_url": "workFission/welcome.png",
    },
    "poster": {
        "poster_type": 1,
        "cover_pic": "workFission/poster.png",
        "foward_text": "裂变转发话术",
        "avatar_show": True,
        "nickname_show": False,
        "nickname_color": "#333333",
        "card_corp_image_name": "企业形象",
        "card_corp_name": "裂变企业",
        "card_corp_logo": "workFission/logo.png",
        "qrcode_w": "120",
        "qrcode_h": "120",
        "qrcode_x": "10",
        "qrcode_y": "20",
    },
    "push": {
        "push_employee": True,
        "push_contact": False,
        "msg_text": "裂变推送文案",
        "msg_complex": {"msg_complex_type": "image", "image": "workFission/push.png"},
    },
    "invite": {
        "text": "邀请文案",
        "link_title": "邀请标题",
        "link_desc": "邀请描述",
        "link_pic": "workFission/invite.png",
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/workFission/store" "$STORE_BODY" "$WORK_DIR/work-fission-store.json"

FISSION_ID="$(python3 - "$WORK_DIR/work-fission-store.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(int(payload["data"][0]))
PY
)"
test -n "$FISSION_ID"

test "$(mysql_scalar "SELECT active_name FROM mc_work_fission WHERE id = $FISSION_ID AND corp_id = $CORP_ID AND deleted_at IS NULL")" = "Go裂变活动"
test "$(mysql_scalar "SELECT auto_pass FROM mc_work_fission WHERE id = $FISSION_ID")" = "1"
test "$(mysql_scalar "SELECT new_friend FROM mc_work_fission WHERE id = $FISSION_ID")" = "1"
test "$(mysql_scalar "SELECT delete_invalid FROM mc_work_fission WHERE id = $FISSION_ID")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission WHERE id = $FISSION_ID AND receive_qrcode LIKE '%cfg-work-fission-1%' AND receive_qrcode LIKE '%work-fission-qrcode-1.png%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_welcome WHERE fission_id = $FISSION_ID AND link_cover_url = 'workFission/welcome.png' AND link_wx_url = 'https://wecom.example/work-fission-upload-1.jpg' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_poster WHERE fission_id = $FISSION_ID AND cover_pic = 'workFission/poster.png' AND card_corp_logo = 'workFission/logo.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_push WHERE fission_id = $FISSION_ID AND msg_complex_type = 'image' AND CAST(msg_complex AS CHAR) LIKE '%work-fission-upload-2.jpg%' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_invite WHERE fission_id = $FISSION_ID AND text = '邀请文案' AND link_pic = 'workFission/invite.png' AND wx_link_pic = 'https://wecom.example/work-fission-upload-3.jpg' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'work_fissions' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_work_fission_contact
  (id, fission_id, union_id, nickname, avatar, contact_superior_user_parent, level, employee, invite_count, loss, status, receive_level, is_new, external_user_id, qrcode_id, qrcode_url, created_at, updated_at, deleted_at)
VALUES
  ($FISSION_PARENT_ID, $FISSION_ID, 'union-fission-a', '裂变参与客户', 'workFission/avatar-parent.png', 0, 1, 'fission-user', 2, 0, 1, 0, 1, 'external-fission-a', 'existing-qrcode-id', 'workFission/qrcode.png', NOW(), NOW(), NULL),
  ($FISSION_CHILD_ONE_ID, $FISSION_ID, 'union-fission-child-a', '助力客户A', 'workFission/avatar-child-a.png', $FISSION_PARENT_ID, 2, 'fission-user', 0, 0, 1, 0, 1, 'external-fission-child-a', '', '', NOW(), NOW(), NULL),
  ($FISSION_CHILD_TWO_ID, $FISSION_ID, 'union-fission-child-b', '助力客户B', 'workFission/avatar-child-b.png', $FISSION_PARENT_ID, 2, 'fission-user', 0, 1, 0, 0, 0, 'external-fission-child-b', '', '', NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/workFission/index?active_name=Go&page=1&perPage=10" "" "$WORK_DIR/work-fission-index.json"
api_json GET "/dashboard/workFission/show?id=$FISSION_ID" "" "$WORK_DIR/work-fission-show.json"
api_json GET "/dashboard/workFission/info?id=$FISSION_ID" "" "$WORK_DIR/work-fission-info.json"
api_json GET "/dashboard/workFission/statistics?fission_ids=%5B$FISSION_ID%5D" "" "$WORK_DIR/work-fission-statistics.json"
api_json GET "/dashboard/workFission/chooseContact?employee_ids=%5B$EMPLOYEE_ID%5D&is_all=1&start_time=2026-01-01%2000%3A00%3A00&end_time=2038-01-01%2000%3A00%3A00&gender=1" "" "$WORK_DIR/work-fission-choose-contact.json"
api_json GET "/dashboard/workFission/inviteData?fission_ids=%5B$FISSION_ID%5D&page=1&perPage=10" "" "$WORK_DIR/work-fission-invite-data.json"
api_json GET "/dashboard/workFission/inviteDetail?id=$FISSION_PARENT_ID" "" "$WORK_DIR/work-fission-invite-detail.json"

python3 - "$WORK_DIR/work-fission-index.json" "$WORK_DIR/work-fission-show.json" "$WORK_DIR/work-fission-info.json" "$WORK_DIR/work-fission-statistics.json" "$WORK_DIR/work-fission-choose-contact.json" "$WORK_DIR/work-fission-invite-data.json" "$WORK_DIR/work-fission-invite-detail.json" "$FISSION_ID" "$FISSION_PARENT_ID" "$GO_ADDR" "$OPERATION_ADDR" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
info_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
stats_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
choose_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
invite_data_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
invite_detail_payload = json.loads(pathlib.Path(sys.argv[7]).read_text(encoding="utf-8"))
fission_id = int(sys.argv[8])
parent_id = int(sys.argv[9])
go_addr = sys.argv[10]
operation_addr = sys.argv[11]

items = index_payload["data"]["list"]
item = next((value for value in items if int(value["id"]) == fission_id), None)
assert item, items
assert item["active_name"] == "Go裂变活动", item
assert item["employeeNum"] == 3, item
assert "一阶奖励" in item["tasks"], item

show = show_payload["data"]
assert int(show["id"]) == fission_id, show
assert show["active_name"] == "Go裂变活动", show
assert show["link"] == f"http://{operation_addr}/auth/workFission?id={fission_id}&target=%2FworkFission%3Fid%3D{fission_id}", show
assert show["welcome_title"] == "欢迎标题", show

info = info_payload["data"]
assert info["fission"]["active_name"] == "Go裂变活动", info
assert info["fission"]["auto_pass"] == "true", info
assert info["welcome"]["link_cover_url"] == f"http://{go_addr}/static/workFission/welcome.png", info
assert info["poster"]["cover_pic"] == f"http://{go_addr}/static/workFission/poster.png", info
assert info["poster"]["card_corp_logo"] == "workFission/logo.png", info
assert info["push"]["msg_complex"]["image"] == f"http://{go_addr}/static/workFission/push.png", info
assert info["push"]["msg_complex"]["pic_url"] == "https://wecom.example/work-fission-upload-2.jpg", info
assert info["invite"]["link_pic"] == f"http://{go_addr}/static/workFission/invite.png", info

stats = stats_payload["data"]
assert stats["user"]["user_count"] == 3, stats
assert stats["user"]["loss_count"] == 1, stats
assert stats["user"]["insert_count"] == 2, stats
assert stats["user"]["invite_count"] == 2, stats
assert stats["level"]["first_level"] == 1, stats
assert stats["level"]["second_level"] == 2, stats
assert stats["user"]["fission_rate"] == "66.67%", stats
assert stats["employee"][0]["name"] == "裂变活动员工", stats
assert any(int(active["id"]) == fission_id for active in stats["active"]), stats

assert choose_payload["data"] == [2], choose_payload

invite_items = invite_data_payload["data"]["list"]
assert len(invite_items) == 3, invite_items
parent = next((value for value in invite_items if int(value["id"]) == parent_id), None)
assert parent, invite_items
assert parent["nickname"] == "裂变参与客户", parent
assert parent["active_name"] == "Go裂变活动", parent
assert parent["employees"] == "裂变活动员工", parent
assert parent["loss"] == "未流失", parent
assert parent["status"] == "已完成", parent
assert parent["invite_count"] == 2, parent
assert parent["contact_id"] > 0 and parent["employee_id"] > 0, parent

detail = invite_detail_payload["data"]
assert detail["total_count"] == 2, detail
assert detail["new_count"] == 1, detail
assert detail["loss"] == 1, detail
assert detail["insert"] == 1, detail
children = {item["nickname"]: item for item in detail["user_list"]}
assert set(children) == {"助力客户A", "助力客户B"}, children
assert children["助力客户A"]["avatar"] == f"http://{go_addr}/static/workFission/avatar-child-a.png", children
PY

INVITE_BODY="$(python3 - "$FISSION_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

body = {
    "fission_id": int(sys.argv[1]),
    "text": "二次邀请文案",
    "link_title": "二次邀请标题",
    "link_desc": "二次邀请描述",
    "link_pic": "workFission/invite-again.png",
    "filter": {
        "employee_ids": [int(sys.argv[2])],
        "is_all": 1,
        "start_time": "2026-01-01 00:00:00",
        "end_time": "2038-01-01 00:00:00",
        "gender": 1,
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/workFission/invite" "$INVITE_BODY" "$WORK_DIR/work-fission-invite.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_invite WHERE fission_id = $FISSION_ID AND text = '二次邀请文案' AND link_title = '二次邀请标题' AND link_pic = 'workFission/invite-again.png' AND deleted_at IS NULL")" = "1"

UPDATE_BODY="$(python3 - "$FISSION_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

fission_id = int(sys.argv[1])
employee_id = int(sys.argv[2])
employee = {"id": employee_id, "name": "裂变活动员工", "wxUserId": "fission-user"}
body = {
    "fission": {
        "id": fission_id,
        "active_name": "Go裂变活动更新",
        "service_employees": [employee],
        "auto_pass": False,
        "auto_add_tag": False,
        "contact_tags": [],
        "end_time": "2037-04-01 00:00:00",
        "qr_code_invalid": 3,
        "tasks": [{"level": 1, "num": 1, "prize": "更新奖励"}],
        "new_friend": False,
        "delete_invalid": True,
        "receive_prize": 1,
        "receive_prize_employees": [],
        "receive_links": [{"url": "https://gift.example/work-fission-updated"}],
    },
    "welcome": {
        "msg_text": "欢迎语更新",
        "link_title": "欢迎标题更新",
        "link_desc": "欢迎描述更新",
        "link_cover_url": "workFission/welcome-new.png",
    },
    "poster": {
        "poster_type": 0,
        "cover_pic": "",
        "foward_text": "转发话术更新",
        "avatar_show": False,
        "nickname_show": True,
        "nickname_color": "#111111",
        "card_corp_image_name": "",
        "card_corp_name": "",
        "card_corp_logo": "",
        "qrcode_w": "90",
        "qrcode_h": "90",
        "qrcode_x": "1",
        "qrcode_y": "2",
    },
    "push": {
        "push_employee": False,
        "push_contact": True,
        "msg_text": "推送更新",
        "msg_complex_type": "",
        "msg_complex": {},
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/workFission/update" "$UPDATE_BODY" "$WORK_DIR/work-fission-update.json"

test "$(mysql_scalar "SELECT active_name FROM mc_work_fission WHERE id = $FISSION_ID")" = "Go裂变活动更新"
test "$(mysql_scalar "SELECT auto_pass FROM mc_work_fission WHERE id = $FISSION_ID")" = "0"
test "$(mysql_scalar "SELECT auto_add_tag FROM mc_work_fission WHERE id = $FISSION_ID")" = "0"
test "$(mysql_scalar "SELECT new_friend FROM mc_work_fission WHERE id = $FISSION_ID")" = "0"
test "$(mysql_scalar "SELECT delete_invalid FROM mc_work_fission WHERE id = $FISSION_ID")" = "1"
test "$(mysql_scalar "SELECT receive_prize FROM mc_work_fission WHERE id = $FISSION_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_welcome WHERE fission_id = $FISSION_ID AND msg_text = '欢迎语更新' AND link_wx_url = 'https://wecom.example/work-fission-upload-4.jpg' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_push WHERE fission_id = $FISSION_ID AND push_employee = 0 AND push_contact = 1 AND msg_complex_type = '' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/workFission/info?id=$FISSION_ID" "" "$WORK_DIR/work-fission-info-updated.json"
python3 - "$WORK_DIR/work-fission-info-updated.json" "$FISSION_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
fission_id = int(sys.argv[2])
go_addr = sys.argv[3]
data = payload["data"]
assert int(data["fission"]["id"]) == fission_id, data
assert data["fission"]["active_name"] == "Go裂变活动更新", data
assert data["fission"]["auto_pass"] == "false", data
assert data["fission"]["delete_invalid"] == "true", data
assert data["welcome"]["link_cover_url"] == f"http://{go_addr}/static/workFission/welcome-new.png", data
assert data["push"]["push_contact"] == "true", data
assert data["push"]["msg_complex"] == {}, data
PY

python3 - "$WECOM_EVENTS" "$FISSION_ID" "$GO_ADDR" "$OPERATION_ADDR" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
fission_id = int(sys.argv[2])
go_addr = sys.argv[3]
operation_addr = sys.argv[4]
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/media/uploadimg") == 4, paths
assert paths.count("/cgi-bin/externalcontact/add_contact_way") == 1, paths
assert paths.count("/cgi-bin/externalcontact/add_msg_template") == 1, paths

contact_way = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/add_contact_way")
assert contact_way["user"] == ["fission-user"], contact_way
assert contact_way["skip_verify"] is True, contact_way
assert contact_way.get("state") == "", contact_way

template = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/add_msg_template")
assert template["chat_type"] == "single", template
assert template["external_userid"] == ["external-fission-a", "external-fission-b"], template
assert template["text"]["content"] == "二次邀请文案", template
link = template["attachments"][0]["link"]
assert link["title"] == "二次邀请标题", template
assert link["desc"] == "二次邀请描述", template
assert link["picurl"] == f"http://{go_addr}/static/workFission/invite-again.png", template
assert link["url"] == f"http://{operation_addr}/auth/workFission?id={fission_id}&target=%2FworkFission%3Fid%3D{fission_id}", template
PY

api_json DELETE "/dashboard/workFission/destroy" "{\"id\":$FISSION_ID}" "$WORK_DIR/work-fission-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission WHERE id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_welcome WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_poster WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_push WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_fission_invite WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'work_fissions' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/workFission/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workFission/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workFission/info" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workFission/statistics" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workFission/chooseContact" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/workFission/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/workFission/update" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/workFission/invite" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workFission/inviteData" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workFission/inviteDetail" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/workFission/destroy" "$GO_LOG"

echo "work fission dashboard smoke passed"
