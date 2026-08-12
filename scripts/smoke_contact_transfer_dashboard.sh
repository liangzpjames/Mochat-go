#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-contact-transfer-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18129}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13369}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26429}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19099}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-contact-transfer.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-contact-transfer-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001829}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1829}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91829}"
ADMIN_EMPLOYEE_ID="${MOCHAT_SMOKE_ADMIN_EMPLOYEE_ID:-91829}"
HANDOVER_EMPLOYEE_ID="${MOCHAT_SMOKE_HANDOVER_EMPLOYEE_ID:-918291}"
TAKEOVER_EMPLOYEE_ID="${MOCHAT_SMOKE_TAKEOVER_EMPLOYEE_ID:-918292}"
CONTACT_ID="${MOCHAT_SMOKE_CONTACT_ID:-918293}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918294}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918295}"
TAG_PIVOT_ID="${MOCHAT_SMOKE_TAG_PIVOT_ID:-918296}"
CONTACT_ROOM_ID="${MOCHAT_SMOKE_CONTACT_ROOM_ID:-918297}"

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
            assert query.get("corpid", [""])[0] == "ww-contact-transfer", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-transfer-token", "expires_in": 7200})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b""
        payload = json.loads(raw.decode("utf-8") or "{}")
        self.record(path.path, query, payload)
        if path.path == "/cgi-bin/externalcontact/get_unassigned_list":
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "info": [{
                    "handover_userid": "transfer-handover",
                    "external_userid": "external-transfer-a",
                    "dimission_time": 1783330000,
                }],
            })
            return
        if path.path == "/cgi-bin/externalcontact/transfer_customer":
            assert payload["external_userid"] == ["external-transfer-a"], payload
            assert payload["handover_userid"] == "transfer-handover", payload
            assert payload["takeover_userid"] == "transfer-takeover", payload
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        if path.path == "/cgi-bin/externalcontact/groupchat/transfer":
            assert payload["chat_id_list"] == ["room-transfer-a"], payload
            assert payload["new_owner"] == "transfer-takeover", payload
            self.json_response({"errcode": 0, "errmsg": "ok", "failed_chat_list": []})
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
  -tenant-name "转接验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "转接管理员" \
  -role-name "转接超级管理员" \
  -package-code "contact-transfer-standard" \
  -package-name "转接标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_work_transfer_log WHERE corp_id = $CORP_ID OR contact_id IN ('external-transfer-a', 'room-transfer-a');
DELETE FROM mc_work_unassigned WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_room WHERE id = $CONTACT_ROOM_ID OR room_id = $ROOM_ID OR contact_id = $CONTACT_ID;
DELETE FROM mc_work_room WHERE id = $ROOM_ID;
DELETE FROM mc_work_contact_tag_pivot WHERE id = $TAG_PIVOT_ID OR contact_id = $CONTACT_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID;
DELETE FROM mc_work_contact_employee WHERE contact_id = $CONTACT_ID OR employee_id IN ($ADMIN_EMPLOYEE_ID, $HANDOVER_EMPLOYEE_ID, $TAKEOVER_EMPLOYEE_ID);
DELETE FROM mc_work_contact WHERE id = $CONTACT_ID;
DELETE FROM mc_work_employee WHERE id IN ($ADMIN_EMPLOYEE_ID, $HANDOVER_EMPLOYEE_ID, $TAKEOVER_EMPLOYEE_ID);
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '转接企业', 'ww-contact-transfer', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($ADMIN_EMPLOYEE_ID, 'transfer-admin', $CORP_ID, '转接管理员员工', '$PHONE', 'avatar/admin.png', 'avatar/admin-thumb.png', '转接管理员', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL),
  ($HANDOVER_EMPLOYEE_ID, 'transfer-handover', $CORP_ID, '离职员工', '13800001830', 'avatar/handover.png', 'avatar/handover-thumb.png', '离职员工', 1, 1, 0, 1, 1, NOW(), NOW(), NULL),
  ($TAKEOVER_EMPLOYEE_ID, 'transfer-takeover', $CORP_ID, '接替员工', '13800001831', 'avatar/takeover.png', 'avatar/takeover-thumb.png', '接替员工', 1, 1, 0, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ID, $CORP_ID, 'external-transfer-a', '转接客户A', '转接昵称A', 'avatar/contact-transfer.png', 2, 1, 1, 'union-transfer-a', '', '转接客户企业', '转接客户企业全称', JSON_OBJECT(), 'TRANSFER-A', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($HANDOVER_EMPLOYEE_ID, $CONTACT_ID, '转接客户A', '转接客户描述', '转接客户企业', JSON_ARRAY('13800001832'), 202, 'transfer-handover', '', $CORP_ID, 1, '2026-07-07 09:00:00', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-transfer-tag', $CORP_ID, '转接标签', 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_pivot
  (id, contact_id, employee_id, contact_tag_id, type, created_at, updated_at, deleted_at)
VALUES
  ($TAG_PIVOT_ID, $CONTACT_ID, $HANDOVER_EMPLOYEE_ID, $TAG_ID, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'room-transfer-a', '转接客户群A', $HANDOVER_EMPLOYEE_ID, '转接客户群公告', 0, '2026-07-07 08:00:00', 200, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_room
  (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ROOM_ID, 'external-transfer-a', $CONTACT_ID, 0, 'union-transfer-a', $ROOM_ID, 3, 2, 1, '2026-07-07 09:05:00', '', NOW(), NOW(), NULL);
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
api_json GET "/dashboard/contactTransfer/saveUnassignedList" "" "$WORK_DIR/save-unassigned.json"
api_json GET "/dashboard/contactTransfer/info?contactName=%E8%BD%AC%E6%8E%A5%E5%AE%A2%E6%88%B7A&employeeId=%5B$HANDOVER_EMPLOYEE_ID%5D" "" "$WORK_DIR/info.json"
api_json GET "/dashboard/contactTransfer/unassignedList?contactName=%E8%BD%AC%E6%8E%A5%E5%AE%A2%E6%88%B7A&employeeId=%5B$HANDOVER_EMPLOYEE_ID%5D" "" "$WORK_DIR/unassigned.json"
api_json GET "/dashboard/contactTransfer/room?roomName=%E8%BD%AC%E6%8E%A5%E5%AE%A2%E6%88%B7%E7%BE%A4" "" "$WORK_DIR/room.json"

CUSTOMER_PAYLOAD='{"type":1,"takeoverUserId":"transfer-takeover","list":"[{\"contactWxId\":\"external-transfer-a\",\"employeeWxId\":\"transfer-handover\"}]"}'
api_json POST "/dashboard/contactTransfer/index" "$CUSTOMER_PAYLOAD" "$WORK_DIR/transfer-customer.json"
ROOM_PAYLOAD='{"takeoverUserId":"transfer-takeover","list":"[\"room-transfer-a\"]"}'
api_json POST "/dashboard/contactTransfer/room" "$ROOM_PAYLOAD" "$WORK_DIR/transfer-room.json"
api_json GET "/dashboard/contactTransfer/log?mode=1&name=%E8%BD%AC%E6%8E%A5%E5%AE%A2%E6%88%B7A" "" "$WORK_DIR/log-contact.json"
api_json GET "/dashboard/contactTransfer/log?mode=2&name=%E8%BD%AC%E6%8E%A5%E5%AE%A2%E6%88%B7%E7%BE%A4A" "" "$WORK_DIR/log-room.json"

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_unassigned WHERE corp_id = $CORP_ID AND handover_userid = 'transfer-handover' AND external_userid = 'external-transfer-a' AND deleted_at IS NULL")" = "1"
mysql_scalar "SELECT status, type, name, contact_id, handover_employee_id, takeover_employee_id FROM mc_work_transfer_log WHERE corp_id = $CORP_ID AND contact_id IN ('external-transfer-a', 'room-transfer-a') ORDER BY id ASC" >"$WORK_DIR/transfer-logs.tsv"

python3 - "$WORK_DIR" "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
events_path = pathlib.Path(sys.argv[2])


def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))


info = load("info.json")
unassigned = load("unassigned.json")
rooms = load("room.json")
customer = load("transfer-customer.json")
room = load("transfer-room.json")
log_contact = load("log-contact.json")
log_room = load("log-room.json")

info_row = next((item for item in info["data"] if item["contactWxId"] == "external-transfer-a"), None)
assert info_row, info
assert info_row["employeeWxId"] == "transfer-handover", info_row
assert "转接标签" in info_row["tags"], info_row
assert info_row["transferState"] == "", info_row

unassigned_row = next((item for item in unassigned["data"]["list"] if item["contactWxId"] == "external-transfer-a"), None)
assert unassigned_row, unassigned
assert unassigned["data"]["lastTime"] != "无数据", unassigned

room_row = next((item for item in rooms["data"] if item["chatId"] == "room-transfer-a"), None)
assert room_row, rooms
assert room_row["owner"] == "离职员工", room_row
assert int(room_row["userNum"]) == 1, room_row

assert customer["data"][0]["errcode"] == 0, customer
assert room["data"] == [], room

contact_log = next((item for item in log_contact["data"] if item["name"] == "转接客户A"), None)
assert contact_log, log_contact
assert contact_log["employee"] == "接替员工", contact_log

room_log = next((item for item in log_room["data"] if item["name"] == "转接客户群A"), None)
assert room_log, log_room
assert room_log["employee"] == "接替员工", room_log
assert int(room_log["roomNum"]) == 1, room_log

logs = (work / "transfer-logs.tsv").read_text(encoding="utf-8")
assert "1\t1\t转接客户A\texternal-transfer-a\ttransfer-handover\ttransfer-takeover" in logs, logs
assert "1\t2\t转接客户群A\troom-transfer-a\t\ttransfer-takeover" in logs, logs

events = [json.loads(line) for line in events_path.read_text(encoding="utf-8").splitlines() if line.strip()]
paths = [event["path"] for event in events]
assert "/cgi-bin/externalcontact/get_unassigned_list" in paths, events
assert "/cgi-bin/externalcontact/transfer_customer" in paths, events
assert "/cgi-bin/externalcontact/groupchat/transfer" in paths, events
PY

grep -q "go migrated route enabled: GET /dashboard/contactTransfer/info" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactTransfer/unassignedList" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactTransfer/room" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactTransfer/log" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/contactTransfer/saveUnassignedList" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/contactTransfer/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/contactTransfer/room" "$GO_LOG"

echo "contact transfer dashboard smoke passed"
