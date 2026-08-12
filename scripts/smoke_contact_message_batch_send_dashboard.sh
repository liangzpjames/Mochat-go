#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-contact-batch-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18115}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13355}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26415}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19095}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-contact-batch.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-contact-batch-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001815}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1815}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91815}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91815}"
AGENT_ID="${MOCHAT_SMOKE_AGENT_ID:-918150}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-918151}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918152}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918153}"
CONTACT_ONE_ID="${MOCHAT_SMOKE_CONTACT_ONE_ID:-918154}"
CONTACT_TWO_ID="${MOCHAT_SMOKE_CONTACT_TWO_ID:-918155}"
CONTACT_OTHER_ID="${MOCHAT_SMOKE_CONTACT_OTHER_ID:-918156}"

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
            assert query.get("corpid", [""])[0] == "ww-contact-batch", query
            secret = query.get("corpsecret", [""])[0]
            if secret == "contact-secret":
                self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-batch-token", "expires_in": 7200})
                return
            if secret == "agent-secret":
                self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-batch-agent-token", "expires_in": 7200})
                return
            raise AssertionError(query)
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        global upload_count, template_count
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        if self.headers.get("Content-Type", "").startswith("application/json"):
            payload = json.loads(raw.decode("utf-8") or "{}")
        else:
            payload = {"multipart": True, "bytes": len(raw)}
        self.record(path.path, query, payload)

        if path.path == "/cgi-bin/media/upload":
            assert query.get("access_token") == "contact-batch-token", query
            assert query.get("type") == "image", query
            upload_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "media_id": f"media-contact-batch-{upload_count}"})
            return
        if path.path == "/cgi-bin/externalcontact/add_msg_template":
            assert query.get("access_token") == "contact-batch-token", query
            assert payload.get("chat_type") == "single", payload
            assert payload.get("sender") == "contact-batch-user", payload
            assert payload.get("external_userid") == ["external-batch-a", "external-batch-b"], payload
            assert payload.get("text", {}).get("content") == "Go客户群发文本", payload
            attachments = payload.get("attachments", [])
            assert len(attachments) == 1, payload
            assert attachments[0].get("msgtype") == "image", payload
            assert attachments[0].get("image", {}).get("media_id") == "media-contact-batch-1", payload
            template_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": f"msg-contact-batch-{template_count}"})
            return
        if path.path == "/cgi-bin/message/send":
            assert query.get("access_token") == "contact-batch-agent-token", query
            assert payload.get("touser") == "contact-batch-user", payload
            assert payload.get("agentid") == "10091815", payload
            content = payload.get("text", {}).get("content", "")
            assert "客户群发任务" in content, payload
            self.json_response({"errcode": 0, "errmsg": "ok", "msgid": "agent-contact-batch-1"})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" "$WECOM_EVENTS" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

mkdir -p "$FILE_STORAGE_ROOT/image/contactMessageBatchSend"
printf '\xff\xd8\xff\xe0mochat-go-contact-batch\xff\xd9' >"$FILE_STORAGE_ROOT/image/contactMessageBatchSend/batch.jpg"

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
  -tenant-name "客户群发验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "客户群发管理员" \
  -role-name "客户群发超级管理员" \
  -package-code "contact-batch-standard" \
  -package-name "客户群发标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -contact-message-batches 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_contact_message_batch_send_result WHERE contact_id IN ($CONTACT_ONE_ID, $CONTACT_TWO_ID, $CONTACT_OTHER_ID);
DELETE FROM mc_contact_message_batch_send_employee WHERE employee_id = $EMPLOYEE_ID;
DELETE FROM mc_contact_message_batch_send WHERE corp_id = $CORP_ID;
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
  ($CORP_ID, '客户群发企业', 'ww-contact-batch', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'contact-batch-user', $CORP_ID, '客户群发员工', '$PHONE', 'avatar/contact-batch.png', 'avatar/contact-batch-thumb.png', '群发员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_agent
  (id, corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close, redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at, deleted_at)
VALUES
  ($AGENT_ID, $CORP_ID, '10091815', 'agent-secret', '客户群发提醒应用', '', '', 0, '', 0, 0, '', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'wx-contact-batch-group', $CORP_ID, '客户群发标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-contact-batch-tag', $CORP_ID, '客户群发标签', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'wr-contact-batch-a', '客户群发验收群', $EMPLOYEE_ID, '', 0, NOW(), 200, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ONE_ID, $CORP_ID, 'external-batch-a', '群发客户甲', '群发甲', 'avatar/batch-a.png', 1, 1, 2, 'union-batch-a', '', '', '', NULL, 'CB-A', NOW(), NOW(), NULL),
  ($CONTACT_TWO_ID, $CORP_ID, 'external-batch-b', '群发客户乙', '群发乙', 'avatar/batch-b.png', 1, 1, 2, 'union-batch-b', '', '', '', NULL, 'CB-B', NOW(), NOW(), NULL),
  ($CONTACT_OTHER_ID, $CORP_ID, 'external-batch-other', '非目标客户', '非目标', 'avatar/batch-other.png', 1, 1, 1, 'union-batch-other', '', '', '', NULL, 'CB-O', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, $CONTACT_ONE_ID, '', '', '', NULL, 1, 'contact-batch-user', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_TWO_ID, '', '', '', NULL, 1, 'contact-batch-user', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_OTHER_ID, '', '', '', NULL, 1, 'contact-batch-user', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_pivot
  (contact_id, employee_id, contact_tag_id, type, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ONE_ID, $EMPLOYEE_ID, $TAG_ID, 1, NOW(), NOW(), NULL),
  ($CONTACT_TWO_ID, $EMPLOYEE_ID, $TAG_ID, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_room
  (wx_user_id, contact_id, employee_id, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at)
VALUES
  ('external-batch-a', $CONTACT_ONE_ID, $EMPLOYEE_ID, $ROOM_ID, 3, 2, 1, NOW(), '', NOW(), NOW(), NULL),
  ('external-batch-b', $CONTACT_TWO_ID, $EMPLOYEE_ID, $ROOM_ID, 3, 2, 1, NOW(), '', NOW(), NOW(), NULL),
  ('contact-batch-user', 0, $EMPLOYEE_ID, $ROOM_ID, 1, 1, 1, NOW(), '', NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
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

STORE_BODY="$(python3 - "$EMPLOYEE_ID" "$TAG_ID" "$ROOM_ID" <<'PY'
import json
import sys

body = {
    "employeeIds": [int(sys.argv[1])],
    "filterParams": {
        "gender": 2,
        "rooms": [int(sys.argv[3])],
        "tags": [int(sys.argv[2])],
    },
    "content": [
        {"msgType": "text", "content": "Go客户群发文本"},
        {"msgType": "image", "pic_url": "image/contactMessageBatchSend/batch.jpg"},
    ],
    "sendWay": 1,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/contactMessageBatchSend/store" "$STORE_BODY" "$WORK_DIR/contact-batch-store.json"

BATCH_ID="$(mysql_scalar "SELECT id FROM mc_contact_message_batch_send WHERE corp_id = $CORP_ID AND user_id = $USER_ID AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$BATCH_ID"
test "$(mysql_scalar "SELECT send_status FROM mc_contact_message_batch_send WHERE id = $BATCH_ID")" = "1"
test "$(mysql_scalar "SELECT send_employee_total FROM mc_contact_message_batch_send WHERE id = $BATCH_ID")" = "1"
test "$(mysql_scalar "SELECT send_contact_total FROM mc_contact_message_batch_send WHERE id = $BATCH_ID")" = "2"
test "$(mysql_scalar "SELECT not_send_total FROM mc_contact_message_batch_send WHERE id = $BATCH_ID")" = "1"
test "$(mysql_scalar "SELECT not_received_total FROM mc_contact_message_batch_send WHERE id = $BATCH_ID")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send WHERE id = $BATCH_ID AND employee_ids LIKE '%$EMPLOYEE_ID%' AND filter_params LIKE '%$TAG_ID%' AND filter_params_detail LIKE '%客户群发标签%' AND content LIKE '%contactMessageBatchSend/batch.jpg%'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send_employee WHERE batch_id = $BATCH_ID AND employee_id = $EMPLOYEE_ID AND wx_user_id = 'contact-batch-user' AND send_contact_total = 2 AND status = 1 AND receive_status = 1 AND msg_id = 'msg-contact-batch-1'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = $BATCH_ID AND employee_id = $EMPLOYEE_ID")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = $BATCH_ID AND external_user_id IN ('external-batch-a', 'external-batch-b')")" = "2"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'contact_message_batches' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/contactMessageBatchSend/index?page=1&perPage=10" "" "$WORK_DIR/contact-batch-index.json"
api_json GET "/dashboard/contactMessageBatchSend/show?batchId=$BATCH_ID" "" "$WORK_DIR/contact-batch-show.json"
api_json GET "/dashboard/contactMessageBatchSend/messageShow?batchId=$BATCH_ID" "" "$WORK_DIR/contact-batch-message-show.json"
api_json GET "/dashboard/contactMessageBatchSend/showRoom?workRoomId=$ROOM_ID" "" "$WORK_DIR/contact-batch-show-room.json"
api_json GET "/dashboard/contactMessageBatchSend/employeeSendIndex?batchId=$BATCH_ID&page=1&perPage=10" "" "$WORK_DIR/contact-batch-employee-index.json"
api_json GET "/dashboard/contactMessageBatchSend/contactReceiveIndex?batchId=$BATCH_ID&page=1&perPage=10" "" "$WORK_DIR/contact-batch-receive-index.json"

python3 - "$WORK_DIR/contact-batch-index.json" "$WORK_DIR/contact-batch-show.json" "$WORK_DIR/contact-batch-message-show.json" "$WORK_DIR/contact-batch-show-room.json" "$WORK_DIR/contact-batch-employee-index.json" "$WORK_DIR/contact-batch-receive-index.json" "$BATCH_ID" "$ROOM_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
message_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
room_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
employee_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
receive_payload = json.loads(pathlib.Path(sys.argv[6]).read_text(encoding="utf-8"))
batch_id = int(sys.argv[7])
room_id = int(sys.argv[8])

items = index_payload["data"]["list"]
item = next((value for value in items if int(value["id"]) == batch_id), None)
assert item, items
assert item["sendWay"] == 1, item
assert item["sendStatus"] == 1, item
assert item["sendTotal"] == 0, item
assert item["notSendTotal"] == 1, item
assert item["content"][0]["content"] == "Go客户群发文本", item
assert item["content"][1]["pic_url"].endswith("/static/image/contactMessageBatchSend/batch.jpg"), item

show = show_payload["data"]
assert show["creator"] == "客户群发管理员", show
assert show["filterParams"]["rooms"] == [room_id], show
assert show["filterParamsDetail"]["rooms"][0]["name"] == "客户群发验收群", show
assert show["filterParamsDetail"]["tags"][0]["name"] == "客户群发标签", show
assert show["sendEmployeeTotal"] == 1, show
assert show["sendContactTotal"] == 2, show
assert show["notReceivedTotal"] == 2, show

message = message_payload["data"]
assert message["message"][0]["content"] == "Go客户群发文本", message
assert message["list"][1]["pic_url"].endswith("/static/image/contactMessageBatchSend/batch.jpg"), message

room = room_payload["data"]
assert room["name"] == "客户群发验收群", room
assert room["ownerName"] == "客户群发员工", room
assert room["totalContact"] == 2, room

employees = employee_payload["data"]["list"]
assert len(employees) == 1, employees
assert employees[0]["employeeName"] == "客户群发员工", employees
assert employees[0]["employeeAlias"] == "群发员工别名", employees
assert employees[0]["status"] == 1, employees
assert employees[0]["sendContactTotal"] == 2, employees

contacts = receive_payload["data"]["list"]
assert len(contacts) == 2, contacts
names = {item["contactName"] for item in contacts}
assert names == {"群发客户甲", "群发客户乙"}, contacts
assert {item["employeeName"] for item in contacts} == {"客户群发员工"}, contacts
assert {item["status"] for item in contacts} == {0}, contacts
PY

api_json POST "/dashboard/contactMessageBatchSend/remind" "{\"batchId\":$BATCH_ID}" "$WORK_DIR/contact-batch-remind.json"

python3 - "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/media/upload") == 1, paths
assert paths.count("/cgi-bin/externalcontact/add_msg_template") == 1, paths
assert paths.count("/cgi-bin/message/send") == 1, paths
template = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/add_msg_template")
assert template["sender"] == "contact-batch-user", template
assert template["external_userid"] == ["external-batch-a", "external-batch-b"], template
assert template["attachments"][0]["image"]["media_id"] == "media-contact-batch-1", template
remind = next(event["body"] for event in events if event["path"] == "/cgi-bin/message/send")
assert remind["touser"] == "contact-batch-user", remind
assert "客户群发任务" in remind["text"]["content"], remind
PY

api_json DELETE "/dashboard/contactMessageBatchSend/destroy" "{\"batchId\":$BATCH_ID}" "$WORK_DIR/contact-batch-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send WHERE id = $BATCH_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send_employee WHERE batch_id = $BATCH_ID")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_message_batch_send_result WHERE batch_id = $BATCH_ID")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'contact_message_batches' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/contactMessageBatchSend/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactMessageBatchSend/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactMessageBatchSend/messageShow" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactMessageBatchSend/showRoom" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactMessageBatchSend/employeeSendIndex" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactMessageBatchSend/contactReceiveIndex" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/contactMessageBatchSend/store" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/contactMessageBatchSend/remind" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/contactMessageBatchSend/destroy" "$GO_LOG"

echo "contact message batch send dashboard smoke passed"
