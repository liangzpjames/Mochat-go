#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-auto-pull-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18113}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13353}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26413}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19093}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-auto-pull.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-auto-pull-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001813}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1813}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91813}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91813}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-918130}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918131}"
ROOM_ONE_ID="${MOCHAT_SMOKE_ROOM_ONE_ID:-918132}"
ROOM_TWO_ID="${MOCHAT_SMOKE_ROOM_TWO_ID:-918133}"

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
            assert query.get("corpid", [""])[0] == "ww-auto-pull", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "auto-pull-token", "expires_in": 7200})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        payload = json.loads(raw.decode("utf-8") or "{}")
        self.record(path.path, query, payload)
        if path.path == "/cgi-bin/externalcontact/contact_way/create":
            config = payload.get("config", {})
            assert payload.get("type") == 2, payload
            assert payload.get("scene") == 2, payload
            assert config.get("skip_verify") is True, payload
            assert config.get("user") == ["auto-pull-user"], payload
            assert str(config.get("state", "")).startswith("workRoomAutoPullId-"), payload
            self.json_response({"errcode": 0, "errmsg": "ok", "config_id": "auto-pull-config-1", "qr_code": "https://wecom.example/auto-pull-create.png"})
            return
        if path.path == "/cgi-bin/externalcontact/contact_way/update":
            assert payload.get("config_id") == "auto-pull-config-1", payload
            assert payload.get("skip_verify") is False, payload
            assert payload.get("user") == ["auto-pull-user"], payload
            assert str(payload.get("state", "")).startswith("workRoomAutoPullId-"), payload
            self.json_response({"errcode": 0, "errmsg": "ok"})
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
  -tenant-name "自动拉群验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "自动拉群管理员" \
  -role-name "自动拉群超级管理员" \
  -package-code "auto-pull-standard" \
  -package-name "自动拉群标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -work-room-auto-pulls 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_work_contact_room WHERE room_id IN ($ROOM_ONE_ID, $ROOM_TWO_ID);
DELETE FROM mc_work_room_auto_pull WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_room WHERE id IN ($ROOM_ONE_ID, $ROOM_TWO_ID);
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID;
DELETE FROM mc_work_contact_tag_group WHERE id = $TAG_GROUP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '自动拉群企业', 'ww-auto-pull', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'auto-pull-user', $CORP_ID, '自动拉群员工', '$PHONE', 'avatar/auto-pull.png', 1, 1, $USER_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'wx-auto-pull-group', $CORP_ID, '自动拉群标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-auto-pull-tag', $CORP_ID, '自动拉群标签', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ONE_ID, $CORP_ID, 'wr-auto-pull-a', '自动拉群一群', $EMPLOYEE_ID, '', 0, NOW(), 200, 0, NOW(), NOW(), NULL),
  ($ROOM_TWO_ID, $CORP_ID, 'wr-auto-pull-b', '自动拉群二群', $EMPLOYEE_ID, '', 0, NOW(), 200, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_room
  (wx_user_id, contact_id, employee_id, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at)
VALUES
  ('external-a', 0, $EMPLOYEE_ID, $ROOM_ONE_ID, 3, 2, 1, NOW(), '', NOW(), NOW(), NULL),
  ('employee-b', 0, $EMPLOYEE_ID, $ROOM_TWO_ID, 3, 1, 1, NOW(), '', NOW(), NOW(), NULL);
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

ROOMS_CREATE="$(python3 - "$ROOM_ONE_ID" "$ROOM_TWO_ID" <<'PY'
import json
import sys

room_one = int(sys.argv[1])
room_two = int(sys.argv[2])
print(json.dumps([
    {"roomId": room_one, "maxNum": 60, "roomQrcodeUrl": "image/workRoomAutoPull/room-a.png"},
    {"roomId": room_two, "maxNum": 1, "roomQrcodeUrl": "image/workRoomAutoPull/room-b.png"},
], ensure_ascii=False))
PY
)"

CREATE_BODY="$(python3 - "$CORP_ID" "$EMPLOYEE_ID" "$TAG_ID" "$ROOMS_CREATE" <<'PY'
import json
import sys

body = {
    "corpId": int(sys.argv[1]),
    "qrcodeName": "Go自动拉群验收",
    "isVerified": 2,
    "leadingWords": "扫码后请按提示入群",
    "employees": str(int(sys.argv[2])),
    "tags": str(int(sys.argv[3])),
    "rooms": sys.argv[4],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/workRoomAutoPull/store" "$CREATE_BODY" "$WORK_DIR/auto-pull-create.json"

AUTO_PULL_ID="$(mysql_scalar "SELECT id FROM mc_work_room_auto_pull WHERE corp_id = $CORP_ID AND qrcode_name = 'Go自动拉群验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$AUTO_PULL_ID"
test "$(mysql_scalar "SELECT qrcode_url FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID")" = "https://wecom.example/auto-pull-create.png"
test "$(mysql_scalar "SELECT wx_config_id FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID")" = "auto-pull-config-1"
test "$(mysql_scalar "SELECT is_verified FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID AND employees LIKE '%$EMPLOYEE_ID%' AND tags LIKE '%$TAG_ID%' AND rooms LIKE '%room-a.png%' AND rooms LIKE '%room-b.png%'")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'work_room_auto_pulls' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/workRoomAutoPull/index?qrcodeName=Go%E8%87%AA%E5%8A%A8&page=1&perPage=10" "" "$WORK_DIR/auto-pull-index.json"
api_json GET "/dashboard/workRoomAutoPull/show?workRoomAutoPullId=$AUTO_PULL_ID" "" "$WORK_DIR/auto-pull-show.json"
python3 - "$WORK_DIR/auto-pull-index.json" "$WORK_DIR/auto-pull-show.json" "$AUTO_PULL_ID" "$TAG_ID" "$ROOM_ONE_ID" "$ROOM_TWO_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
auto_pull_id = int(sys.argv[3])
tag_id = int(sys.argv[4])
room_one_id = int(sys.argv[5])
room_two_id = int(sys.argv[6])

items = index_payload["data"]["list"]
item = next((value for value in items if int(value["workRoomAutoPullId"]) == auto_pull_id), None)
assert item, items
assert item["qrcodeName"] == "Go自动拉群验收", item
assert item["qrcodeUrl"] == "https://wecom.example/auto-pull-create.png", item
assert "自动拉群员工" in item["employees"], item
assert "自动拉群标签" in item["tags"], item
room_states = {room["roomName"]: room["stateText"] for room in item["rooms"]}
assert room_states["自动拉群一群"] == "拉人中", room_states
assert room_states["自动拉群二群"] == "已拉满", room_states

data = show_payload["data"]
assert int(data["workRoomAutoPullId"]) == auto_pull_id, data
assert data["qrcodeName"] == "Go自动拉群验收", data
assert data["qrcodeUrl"] == "https://wecom.example/auto-pull-create.png", data
assert int(data["isVerified"]) == 2, data
assert tag_id in [int(value) for value in data["selectedTags"]], data
employees = data["employees"]
assert len(employees) == 1 and employees[0]["wxUserId"] == "auto-pull-user", employees
rooms = {int(room["roomId"]): room for room in data["rooms"]}
assert set(rooms) == {room_one_id, room_two_id}, rooms
assert rooms[room_one_id]["state"] == 2, rooms
assert rooms[room_two_id]["state"] == 3, rooms
assert rooms[room_one_id]["longRoomQrcodeUrl"].endswith("/static/image/workRoomAutoPull/room-a.png"), rooms
PY

ROOMS_UPDATE="$(python3 - "$ROOM_TWO_ID" <<'PY'
import json
import sys

print(json.dumps([
    {"roomId": int(sys.argv[1]), "maxNum": 80, "roomQrcodeUrl": "image/workRoomAutoPull/room-b-new.png"},
], ensure_ascii=False))
PY
)"

UPDATE_BODY="$(python3 - "$AUTO_PULL_ID" "$EMPLOYEE_ID" "$TAG_ID" "$ROOMS_UPDATE" <<'PY'
import json
import sys

body = {
    "workRoomAutoPullId": int(sys.argv[1]),
    "isVerified": 1,
    "employees": str(int(sys.argv[2])),
    "tags": str(int(sys.argv[3])),
    "rooms": sys.argv[4],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/workRoomAutoPull/update" "$UPDATE_BODY" "$WORK_DIR/auto-pull-update.json"
test "$(mysql_scalar "SELECT is_verified FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID AND rooms LIKE '%room-b-new.png%' AND rooms NOT LIKE '%room-a.png%'")" = "1"

api_json GET "/dashboard/workRoomAutoPull/show?workRoomAutoPullId=$AUTO_PULL_ID" "" "$WORK_DIR/auto-pull-show-updated.json"
python3 - "$WORK_DIR/auto-pull-show-updated.json" "$AUTO_PULL_ID" "$ROOM_TWO_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
auto_pull_id = int(sys.argv[2])
room_two_id = int(sys.argv[3])
data = payload["data"]
assert int(data["workRoomAutoPullId"]) == auto_pull_id, data
assert int(data["isVerified"]) == 1, data
rooms = {int(room["roomId"]): room for room in data["rooms"]}
assert set(rooms) == {room_two_id}, rooms
assert rooms[room_two_id]["state"] == 2, rooms
assert rooms[room_two_id]["longRoomQrcodeUrl"].endswith("/static/image/workRoomAutoPull/room-b-new.png"), rooms
PY

api_json PUT "/dashboard/workRoomAutoPull/move" "{\"workRoomAutoPullId\":$AUTO_PULL_ID,\"groupId\":1}" "$WORK_DIR/auto-pull-move.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_auto_pull WHERE id = $AUTO_PULL_ID AND deleted_at IS NULL")" = "1"

python3 - "$WECOM_EVENTS" "$AUTO_PULL_ID" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
auto_pull_id = int(sys.argv[2])
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/externalcontact/contact_way/create") == 1, paths
assert paths.count("/cgi-bin/externalcontact/contact_way/update") == 1, paths

create = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/contact_way/create")
config = create["config"]
assert config["state"] == f"workRoomAutoPullId-{auto_pull_id}", create
assert config["skip_verify"] is True, create
assert config["user"] == ["auto-pull-user"], create

update = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/contact_way/update")
assert update["config_id"] == "auto-pull-config-1", update
assert update["state"] == f"workRoomAutoPullId-{auto_pull_id}", update
assert update["skip_verify"] is False, update
assert update["user"] == ["auto-pull-user"], update
PY

grep -q "go migrated route enabled: POST /dashboard/workRoomAutoPull/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/workRoomAutoPull/update" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/workRoomAutoPull/move" "$GO_LOG"

echo "work room auto pull dashboard smoke passed"
