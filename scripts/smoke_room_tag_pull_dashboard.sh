#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-tag-pull-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18114}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13354}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26414}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19094}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-tag-pull.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-tag-pull-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001814}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1814}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91814}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91814}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-918140}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918141}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918142}"
CONTACT_ONE_ID="${MOCHAT_SMOKE_CONTACT_ONE_ID:-918143}"
CONTACT_TWO_ID="${MOCHAT_SMOKE_CONTACT_TWO_ID:-918144}"
CONTACT_OTHER_ID="${MOCHAT_SMOKE_CONTACT_OTHER_ID:-918145}"
AGENT_ID="${MOCHAT_SMOKE_AGENT_ID:-918146}"

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
upload_count = 0
message_count = 0


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
            assert query.get("corpid", [""])[0] == "ww-room-tag-pull", query
            secret = query.get("corpsecret", [""])[0]
            if secret == "contact-secret":
                self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "room-tag-pull-contact-token", "expires_in": 7200})
                return
            if secret == "agent-secret":
                self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "room-tag-pull-agent-token", "expires_in": 7200})
                return
            raise AssertionError(query)
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        global upload_count, message_count
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        payload = None
        if self.headers.get("Content-Type", "").startswith("application/json"):
            payload = json.loads(raw.decode("utf-8") or "{}")
        else:
            payload = {"multipart": True, "bytes": len(raw)}
        self.record(path.path, query, payload)

        if path.path == "/cgi-bin/media/uploadimg":
            assert query.get("access_token") == "room-tag-pull-contact-token", query
            upload_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "url": f"https://wecom.example/room-tag-pull-{upload_count}.jpg"})
            return
        if path.path == "/cgi-bin/externalcontact/add_msg_template":
            assert query.get("access_token") == "room-tag-pull-contact-token", query
            assert payload.get("sender") == "tag-pull-user", payload
            assert payload.get("external_userid") == ["external-tag-a", "external-tag-b"], payload
            assert payload.get("text", {}).get("content") == "请扫码加入标签建群验收群", payload
            assert payload.get("image", {}).get("pic_url") == "https://wecom.example/room-tag-pull-1.jpg", payload
            message_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": f"msg-room-tag-pull-{message_count}"})
            return
        if path.path == "/cgi-bin/message/send":
            assert query.get("access_token") == "room-tag-pull-agent-token", query
            assert payload.get("touser") == "tag-pull-user", payload
            assert payload.get("agentid") == "10091814", payload
            assert payload.get("msgtype") == "text", payload
            content = payload.get("text", {}).get("content", "")
            assert "管理员提醒你发送群发任务" in content, payload
            assert "客户甲等2个客户" in content, payload
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": "agent-remind-1"})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)


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
  -tenant-name "标签建群验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "标签建群管理员" \
  -role-name "标签建群超级管理员" \
  -package-code "room-tag-pull-standard" \
  -package-name "标签建群标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-tag-pulls 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_tag_pull_contact WHERE contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID);
DELETE FROM mc_room_tag_pull WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_room WHERE room_id = $ROOM_ID OR contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID);
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID);
DELETE FROM mc_work_contact_employee WHERE contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID);
DELETE FROM mc_work_contact WHERE id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID);
DELETE FROM mc_work_room WHERE id = $ROOM_ID;
DELETE FROM mc_work_agent WHERE id = $AGENT_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID;
DELETE FROM mc_work_contact_tag_group WHERE id = $TAG_GROUP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '标签建群企业', 'ww-room-tag-pull', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'tag-pull-user', $CORP_ID, '标签建群员工', '$PHONE', 'avatar/room-tag-pull.png', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_agent
  (id, corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close, redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at, deleted_at)
VALUES
  ($AGENT_ID, $CORP_ID, '10091814', 'agent-secret', '标签建群提醒应用', '', '', 0, '', 0, 0, '', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'wx-room-tag-pull-group', $CORP_ID, '标签建群标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-room-tag-pull-tag', $CORP_ID, '标签建群标签', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'wr-room-tag-pull-a', '标签建群验收群', $EMPLOYEE_ID, '', 0, NOW(), 200, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ONE_ID, $CORP_ID, 'external-tag-a', '客户甲', '客户甲', 'avatar/customer-a.png', 1, 1, 2, 'union-a', '', '', '', NULL, 'C-A', NOW(), NOW(), NULL),
  ($CONTACT_TWO_ID, $CORP_ID, 'external-tag-b', '客户乙', '客户乙', 'avatar/customer-b.png', 1, 1, 2, 'union-b', '', '', '', NULL, 'C-B', NOW(), NOW(), NULL),
  ($CONTACT_OTHER_ID, $CORP_ID, 'external-tag-other', '其他客户', '其他客户', 'avatar/customer-other.png', 1, 1, 1, 'union-other', '', '', '', NULL, 'C-O', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, $CONTACT_ONE_ID, '', '', '', NULL, 1, 'tag-pull-user', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_TWO_ID, '', '', '', NULL, 1, 'tag-pull-user', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_OTHER_ID, '', '', '', NULL, 1, 'tag-pull-user', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_pivot
  (contact_id, employee_id, contact_tag_id, type, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ONE_ID, $EMPLOYEE_ID, $TAG_ID, 1, NOW(), NOW(), NULL),
  ($CONTACT_TWO_ID, $EMPLOYEE_ID, $TAG_ID, 1, NOW(), NOW(), NULL),
  ($CONTACT_OTHER_ID, $EMPLOYEE_ID, $TAG_ID, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_room
  (wx_user_id, contact_id, employee_id, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at)
VALUES
  ('external-tag-a', $CONTACT_ONE_ID, $EMPLOYEE_ID, $ROOM_ID, 3, 2, 1, NOW(), '', NOW(), NOW(), NULL),
  ('tag-pull-user', 0, $EMPLOYEE_ID, $ROOM_ID, 1, 1, 1, NOW(), '', NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
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

api_json GET "/dashboard/roomTagPull/chooseContact?employees[]=$EMPLOYEE_ID&is_all=1&gender=2&tag_ids[]=$TAG_ID" "" "$WORK_DIR/room-tag-pull-choose-contact.json"
python3 - "$WORK_DIR/room-tag-pull-choose-contact.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["data"] == [2], payload
PY

ROOM_TAG_IMAGE="data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2w=="
FILTER_BODY="$(python3 - "$EMPLOYEE_ID" "$TAG_ID" "$ROOM_ID" <<'PY'
import json
import sys

body = {
    "employees": [int(sys.argv[1])],
    "choose_contact": {"is_all": 1, "gender": 2, "tag_ids": [int(sys.argv[2])]},
    "rooms": [{"id": int(sys.argv[3]), "name": "标签建群验收群", "num": 2}],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomTagPull/filterContact" "$FILTER_BODY" "$WORK_DIR/room-tag-pull-filter-contact.json"
python3 - "$WORK_DIR/room-tag-pull-filter-contact.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["data"] == [1], payload
PY

STORE_BODY="$(python3 - "$EMPLOYEE_ID" "$TAG_ID" "$ROOM_ID" "$ROOM_TAG_IMAGE" <<'PY'
import json
import sys

body = {
    "name": "Go标签建群验收",
    "employees": [int(sys.argv[1])],
    "choose_contact": {"is_all": 1, "gender": 2, "tag_ids": [int(sys.argv[2])]},
    "guide": "请扫码加入标签建群验收群",
    "rooms": [{"id": int(sys.argv[3]), "name": "标签建群验收群", "num": 2, "image": sys.argv[4]}],
    "filter_contact": 0,
    "tenant_id": 1,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomTagPull/store" "$STORE_BODY" "$WORK_DIR/room-tag-pull-store.json"

ROOM_TAG_PULL_ID="$(python3 - "$WORK_DIR/room-tag-pull-store.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(payload["data"][0])
PY
)"
test -n "$ROOM_TAG_PULL_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull WHERE id = $ROOM_TAG_PULL_ID AND name = 'Go标签建群验收' AND corp_id = $CORP_ID AND tenant_id = 1 AND contact_num = 2 AND employees = '$EMPLOYEE_ID' AND rooms LIKE '%标签建群验收群%' AND rooms LIKE '%roomTagPull%' AND rooms LIKE '%https://wecom.example/room-tag-pull-1.jpg%' AND wx_tid LIKE '%msg-room-tag-pull-1%' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull_contact WHERE room_tag_pull_id = $ROOM_TAG_PULL_ID AND deleted_at IS NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull_contact WHERE room_tag_pull_id = $ROOM_TAG_PULL_ID AND contact_id = $CONTACT_ONE_ID AND is_join_room = 1 AND send_status = 0 AND room_id = $ROOM_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull_contact WHERE room_tag_pull_id = $ROOM_TAG_PULL_ID AND contact_id = $CONTACT_TWO_ID AND is_join_room = 0 AND send_status = 0 AND room_id = $ROOM_ID")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_tag_pulls' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/roomTagPull/index?name=Go&page=1&perPage=10" "" "$WORK_DIR/room-tag-pull-index.json"
api_json GET "/dashboard/roomTagPull/show?id=$ROOM_TAG_PULL_ID" "" "$WORK_DIR/room-tag-pull-show.json"
api_json GET "/dashboard/roomTagPull/showContact?id=$ROOM_TAG_PULL_ID&type=1&page=1&perPage=10" "" "$WORK_DIR/room-tag-pull-show-contact-type1.json"
api_json GET "/dashboard/roomTagPull/showContact?id=$ROOM_TAG_PULL_ID&type=2" "" "$WORK_DIR/room-tag-pull-show-contact-type2.json"
api_json GET "/dashboard/roomTagPull/roomList?employees[]=$EMPLOYEE_ID&type=1" "" "$WORK_DIR/room-tag-pull-room-list.json"

python3 - "$WORK_DIR/room-tag-pull-index.json" "$WORK_DIR/room-tag-pull-show.json" "$WORK_DIR/room-tag-pull-show-contact-type1.json" "$WORK_DIR/room-tag-pull-show-contact-type2.json" "$WORK_DIR/room-tag-pull-room-list.json" "$ROOM_TAG_PULL_ID" "$ROOM_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
contacts_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
tasks_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
rooms_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
activity_id = int(sys.argv[6])
room_id = int(sys.argv[7])

items = index_payload["data"]["list"]
item = next((value for value in items if int(value["id"]) == activity_id), None)
assert item, items
assert item["name"] == "Go标签建群验收", item
assert item["employees"] == ["标签建群员工"], item
assert item["rooms"] == ["标签建群验收群"], item
assert int(item["join_room_num"]) == 1, item
assert int(item["no_invite_num"]) == 2, item
assert int(item["no_send_num"]) == 1, item

show = show_payload["data"]
assert show["employees"][0]["name"] == "标签建群员工", show
assert show["employees"][0]["wxUserId"] == "tag-pull-user", show
assert show["rooms"][0]["id"] == room_id, show
assert show["rooms"][0]["name"] == "标签建群验收群", show
assert int(show["join_room_num"]) == 1, show
assert int(show["no_join_room_num"]) == 1, show
assert int(show["no_invite_num"]) == 2, show
assert int(show["no_send_num"]) == 1, show

contacts = contacts_payload["data"]["list"]
assert contacts_payload["data"]["contact_num"] == 2, contacts_payload
names = {item["contact_name"]: item for item in contacts}
assert set(names) == {"客户甲", "客户乙"}, contacts
assert names["客户甲"]["is_join_room"] == 1, contacts
assert names["客户乙"]["is_join_room"] == 0, contacts
assert names["客户甲"]["employee_name"] == "标签建群员工", contacts
assert names["客户乙"]["room_name"] == "标签建群验收群", contacts

tasks = tasks_payload["data"]["list"]
assert len(tasks) == 1, tasks
assert tasks[0]["wxUserId"] == "tag-pull-user", tasks
assert tasks[0]["task_num"] == 1, tasks
assert tasks[0]["contact_num"] == 2, tasks
assert tasks_payload["data"]["contact_num"] == 2, tasks_payload

rooms = rooms_payload["data"]
room = next((value for value in rooms if int(value["id"]) == room_id), None)
assert room, rooms
assert room["name"] == "标签建群验收群", room
assert room["wxChatId"] == "wr-room-tag-pull-a", room
assert room["contact_num"] == 1, room
PY

api_json GET "/dashboard/roomTagPull/remindSend?id=$ROOM_TAG_PULL_ID&wxUserId=tag-pull-user" "" "$WORK_DIR/room-tag-pull-remind.json"

python3 - "$WECOM_EVENTS" "$ROOM_TAG_PULL_ID" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/media/uploadimg") == 1, paths
assert paths.count("/cgi-bin/externalcontact/add_msg_template") == 1, paths
assert paths.count("/cgi-bin/message/send") == 1, paths

template = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/add_msg_template")
assert template["sender"] == "tag-pull-user", template
assert template["external_userid"] == ["external-tag-a", "external-tag-b"], template
assert template["text"]["content"] == "请扫码加入标签建群验收群", template
assert template["image"]["pic_url"] == "https://wecom.example/room-tag-pull-1.jpg", template

remind = next(event["body"] for event in events if event["path"] == "/cgi-bin/message/send")
assert remind["touser"] == "tag-pull-user", remind
assert remind["agentid"] == "10091814", remind
assert "客户甲等2个客户" in remind["text"]["content"], remind
PY

api_json DELETE "/dashboard/roomTagPull/destroy" "{\"id\":$ROOM_TAG_PULL_ID}" "$WORK_DIR/room-tag-pull-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_tag_pull WHERE id = $ROOM_TAG_PULL_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_tag_pulls' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomTagPull/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomTagPull/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomTagPull/showContact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomTagPull/roomList" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomTagPull/chooseContact" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomTagPull/store" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomTagPull/filterContact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomTagPull/remindSend" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomTagPull/destroy" "$GO_LOG"

echo "room tag pull dashboard smoke passed"
