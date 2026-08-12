#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-fission-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18132}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13372}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26432}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-fission.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-fission-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001832}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1832}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91832}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91832}"
CONTACT_RECORD_ID="${MOCHAT_SMOKE_ROOM_FISSION_CONTACT_ID:-918321}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_FISSION_ROOM_ID:-918322}"
WORK_CONTACT_ID="${MOCHAT_SMOKE_WORK_CONTACT_ID:-918323}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
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

mkdir -p "$FILE_STORAGE_ROOT/roomFission"

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
  -tenant-name "群裂变验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "群裂变管理员" \
  -role-name "群裂变超级管理员" \
  -package-code "room-fission-standard" \
  -package-name "群裂变标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-fissions 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_fission_contact WHERE fission_id IN (SELECT id FROM mc_room_fission WHERE corp_id = $CORP_ID) OR id = $CONTACT_RECORD_ID;
DELETE FROM mc_room_fission_invite WHERE fission_id IN (SELECT id FROM mc_room_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_room_fission_welcome WHERE fission_id IN (SELECT id FROM mc_room_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_room_fission_room WHERE fission_id IN (SELECT id FROM mc_room_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_room_fission_poster WHERE fission_id IN (SELECT id FROM mc_room_fission WHERE corp_id = $CORP_ID);
DELETE FROM mc_room_fission WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact WHERE id = $WORK_CONTACT_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '群裂变企业', 'ww-room-fission', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-fission-user', $CORP_ID, '群裂变员工', '$PHONE', 'avatar/room-fission.png', 'avatar/room-fission-thumb.png', '群裂变员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($WORK_CONTACT_ID, $CORP_ID, 'external-room-fission-1', '群裂变客户A', '群裂变客户A', 'avatar/room-fission-contact.png', 1, 1, 1, 'union-room-fission-a', '', '', '', JSON_OBJECT(), 'ROOM-FISSION-A', NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_OPERATION_BASE_URL="http://operation.example.com" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
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

STORE_BODY="$(python3 - "$EMPLOYEE_ID" "$ROOM_ID" <<'PY'
import json
import sys

employee_id = int(sys.argv[1])
room_id = int(sys.argv[2])
body = {
    "fission": {
        "officialAccountId": 3,
        "activeName": "群裂变验收",
        "endTime": "2026-12-31 23:59:59",
        "targetCount": 3,
        "newFriend": 1,
        "deleteInvalid": 1,
        "receiveEmployees": [employee_id],
        "autoPass": 1,
        "status": 1,
    },
    "poster": {
        "coverPic": "roomFission/poster-old.png",
        "avatarShow": 1,
        "nicknameShow": 1,
        "nicknameColor": "#112233",
        "qrcodeW": "120",
        "qrcodeH": "121",
        "qrcodeX": "10",
        "qrcodeY": "11",
    },
    "rooms": [{
        "roomQrcode": "roomFission/room-old.png",
        "roomWxQrcode": "https://wecom.example/room-old.png",
        "roomMax": 200,
        "room": {"id": room_id, "name": "群裂变验收群"},
    }],
    "welcome": {
        "text": "欢迎参与群裂变",
        "linkTitle": "群裂变欢迎标题",
        "linkDesc": "群裂变欢迎描述",
        "linkPic": "roomFission/welcome-old.png",
        "linkWxUrl": "https://wecom.example/welcome-old.png",
        "templateId": "tpl-room-fission-old",
    },
    "invite": {
        "type": 1,
        "employees": [employee_id],
        "chooseContact": {"source": "smoke"},
        "text": "邀请好友入群",
        "linkTitle": "群裂变邀请标题",
        "linkDesc": "群裂变邀请描述",
        "linkPic": "roomFission/invite-old.png",
        "wxLinkPic": "https://wecom.example/invite-old.png",
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomFission/store" "$STORE_BODY" "$WORK_DIR/room-fission-store.json"
FISSION_ID="$(mysql_scalar "SELECT id FROM mc_room_fission WHERE corp_id = $CORP_ID AND active_name = '群裂变验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$FISSION_ID"
test "$(mysql_scalar "SELECT official_account_id FROM mc_room_fission WHERE id = $FISSION_ID")" = "3"
test "$(mysql_scalar "SELECT active_name FROM mc_room_fission WHERE id = $FISSION_ID")" = "群裂变验收"
test "$(mysql_scalar "SELECT DATE_FORMAT(end_time, '%Y-%m-%d %H:%i:%s') FROM mc_room_fission WHERE id = $FISSION_ID")" = "2026-12-31 23:59:59"
test "$(mysql_scalar "SELECT target_count FROM mc_room_fission WHERE id = $FISSION_ID")" = "3"
test "$(mysql_scalar "SELECT JSON_EXTRACT(receive_employees, '\$[0]') FROM mc_room_fission WHERE id = $FISSION_ID")" = "$EMPLOYEE_ID"
test "$(mysql_scalar "SELECT cover_pic FROM mc_room_fission_poster WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/poster-old.png"
test "$(mysql_scalar "SELECT room_qrcode FROM mc_room_fission_room WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/room-old.png"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(room, '\$.name')) FROM mc_room_fission_room WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "群裂变验收群"
test "$(mysql_scalar "SELECT link_pic FROM mc_room_fission_welcome WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/welcome-old.png"
test "$(mysql_scalar "SELECT link_pic FROM mc_room_fission_invite WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/invite-old.png"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_fissions' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_room_fission_contact
  (id, fission_id, union_id, nickname, avatar, parent_union_id, level, contact_id, employee, invite_count, loss, status, receive_status, is_new, external_user_id, room_id, join_status, write_off, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_RECORD_ID, $FISSION_ID, 'union-room-fission-a', '群裂变客户A', 'avatar/room-fission-contact.png', '0', 1, $WORK_CONTACT_ID, '$EMPLOYEE_ID', 3, 0, 1, 0, 1, 'external-room-fission-1', $ROOM_ID, 1, 0, NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/roomFission/index?activeName=%E7%BE%A4%E8%A3%82%E5%8F%98&page=1&perPage=10" "" "$WORK_DIR/room-fission-index.json"
api_json GET "/dashboard/roomFission/info?id=$FISSION_ID" "" "$WORK_DIR/room-fission-info.json"
api_json GET "/dashboard/roomFission/show?id=$FISSION_ID" "" "$WORK_DIR/room-fission-show.json"
api_json GET "/dashboard/roomFission/showRoom?fissionId=$FISSION_ID&page=1&perPage=10" "" "$WORK_DIR/room-fission-rooms.json"
api_json GET "/dashboard/roomFission/showContact?fissionId=$FISSION_ID&status=1&writeOff=0&joinStatus=1&receiveStatus=0&nickname=%E7%BE%A4%E8%A3%82%E5%8F%98&page=1&perPage=10" "" "$WORK_DIR/room-fission-contacts.json"

python3 - \
  "$WORK_DIR/room-fission-index.json" \
  "$WORK_DIR/room-fission-info.json" \
  "$WORK_DIR/room-fission-show.json" \
  "$WORK_DIR/room-fission-rooms.json" \
  "$WORK_DIR/room-fission-contacts.json" \
  "$FISSION_ID" "$CONTACT_RECORD_ID" "$EMPLOYEE_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
info_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
rooms_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
contacts_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
fission_id = int(sys.argv[6])
contact_record_id = int(sys.argv[7])
employee_id = int(sys.argv[8])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["fissionId"]) == fission_id), None)
assert item, items
assert item["activeName"] == "群裂变验收", item
assert item["shareUrl"].startswith("http://operation.example.com/auth/roomFission"), item
assert int(item["contactNum"]) == 1 and int(item["completeNum"]) == 1 and int(item["roomNum"]) == 1, item

for payload in (info_payload, show_payload):
    data = payload["data"]
    assert int(data["id"]) == fission_id, data
    assert int(data["fission"]["fissionId"]) == fission_id, data
    assert data["fission"]["activeName"] == "群裂变验收", data
    assert data["poster"]["coverPic"] == "roomFission/poster-old.png", data
    assert data["rooms"][0]["roomQrcode"] == "roomFission/room-old.png", data
    assert data["welcome"]["linkPic"] == "roomFission/welcome-old.png", data
    assert data["invite"]["linkPic"] == "roomFission/invite-old.png", data
    assert data["invite"]["employees"][0] == employee_id, data

overview = show_payload["data"]["overview"]
assert int(overview["contactNum"]) == 1, overview
assert int(overview["completeNum"]) == 1, overview
assert int(overview["joinRoomNum"]) == 1, overview
assert int(overview["inviteCount"]) == 3, overview

rooms = rooms_payload["data"]
assert int(rooms["page"]["total"]) == 1, rooms
assert rooms["list"][0]["room"]["name"] == "群裂变验收群", rooms

contacts = contacts_payload["data"]
assert int(contacts["page"]["total"]) == 1, contacts
contact = contacts["list"][0]
assert int(contact["id"]) == contact_record_id, contact
assert contact["nickname"] == "群裂变客户A", contact
assert int(contact["status"]) == 1 and int(contact["joinStatus"]) == 1 and int(contact["receiveStatus"]) == 0, contact
assert int(contact["inviteCount"]) == 3, contact
PY

INVITE_BODY="$(python3 - "$FISSION_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

fission_id = int(sys.argv[1])
employee_id = int(sys.argv[2])
body = {
    "fissionId": fission_id,
    "invite": {
        "type": 1,
        "employees": [employee_id],
        "chooseContact": {"source": "smoke-updated"},
        "text": "邀请好友入群更新",
        "linkTitle": "群裂变邀请标题更新",
        "linkDesc": "群裂变邀请描述更新",
        "linkPic": "roomFission/invite-new.png",
        "wxLinkPic": "https://wecom.example/invite-new.png",
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomFission/invite" "$INVITE_BODY" "$WORK_DIR/room-fission-invite.json"
test "$(mysql_scalar "SELECT link_pic FROM mc_room_fission_invite WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/invite-new.png"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(choose_contact, '\$.source')) FROM mc_room_fission_invite WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "smoke-updated"

UPDATE_BODY="$(python3 - "$FISSION_ID" "$EMPLOYEE_ID" "$ROOM_ID" <<'PY'
import json
import sys

fission_id = int(sys.argv[1])
employee_id = int(sys.argv[2])
room_id = int(sys.argv[3])
body = {
    "fissionId": fission_id,
    "fission": {
        "activeName": "群裂变验收更新",
        "endTime": "2026-12-30 23:59:59",
        "targetCount": 5,
        "newFriend": 0,
        "deleteInvalid": 0,
        "receiveEmployees": [employee_id],
        "autoPass": 0,
        "status": 2,
    },
    "poster": {
        "coverPic": "roomFission/poster-new.png",
        "avatarShow": 0,
        "nicknameShow": 1,
        "nicknameColor": "#445566",
        "qrcodeW": "130",
        "qrcodeH": "131",
        "qrcodeX": "12",
        "qrcodeY": "13",
    },
    "rooms": [{
        "roomQrcode": "roomFission/room-new.png",
        "roomWxQrcode": "https://wecom.example/room-new.png",
        "roomMax": 300,
        "room": {"id": room_id, "name": "群裂变验收群更新"},
    }],
    "welcome": {
        "text": "欢迎参与群裂变更新",
        "linkTitle": "群裂变欢迎标题更新",
        "linkDesc": "群裂变欢迎描述更新",
        "linkPic": "roomFission/welcome-new.png",
        "linkWxUrl": "https://wecom.example/welcome-new.png",
        "templateId": "tpl-room-fission-new",
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomFission/update" "$UPDATE_BODY" "$WORK_DIR/room-fission-update.json"
test "$(mysql_scalar "SELECT active_name FROM mc_room_fission WHERE id = $FISSION_ID")" = "群裂变验收更新"
test "$(mysql_scalar "SELECT DATE_FORMAT(end_time, '%Y-%m-%d %H:%i:%s') FROM mc_room_fission WHERE id = $FISSION_ID")" = "2026-12-30 23:59:59"
test "$(mysql_scalar "SELECT target_count FROM mc_room_fission WHERE id = $FISSION_ID")" = "5"
test "$(mysql_scalar "SELECT status FROM mc_room_fission WHERE id = $FISSION_ID")" = "2"
test "$(mysql_scalar "SELECT cover_pic FROM mc_room_fission_poster WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/poster-new.png"
test "$(mysql_scalar "SELECT room_qrcode FROM mc_room_fission_room WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/room-new.png"
test "$(mysql_scalar "SELECT link_pic FROM mc_room_fission_welcome WHERE fission_id = $FISSION_ID AND deleted_at IS NULL")" = "roomFission/welcome-new.png"

api_json GET "/dashboard/roomFission/writeOff?fissionId=$FISSION_ID&contactId=$CONTACT_RECORD_ID" "" "$WORK_DIR/room-fission-write-off.json"
test "$(mysql_scalar "SELECT CONCAT(write_off, '/', receive_status) FROM mc_room_fission_contact WHERE id = $CONTACT_RECORD_ID")" = "1/1"

api_json GET "/dashboard/roomFission/show?id=$FISSION_ID" "" "$WORK_DIR/room-fission-show-updated.json"
python3 - "$WORK_DIR/room-fission-show-updated.json" "$FISSION_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
fission_id = int(sys.argv[2])
data = payload["data"]
assert int(data["id"]) == fission_id, data
assert int(data["fission"]["fissionId"]) == fission_id, data
assert data["fission"]["activeName"] == "群裂变验收更新", data
assert data["fission"]["statusText"] == "已完成", data
assert data["poster"]["coverPic"] == "roomFission/poster-new.png", data
assert data["rooms"][0]["room"]["name"] == "群裂变验收群更新", data
assert data["welcome"]["templateId"] == "tpl-room-fission-new", data
assert data["invite"]["linkPic"] == "roomFission/invite-new.png", data
assert int(data["overview"]["writeOffNum"]) == 1, data
PY

api_json DELETE "/dashboard/roomFission/destroy" "{\"fissionId\":$FISSION_ID}" "$WORK_DIR/room-fission-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission WHERE id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_poster WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_room WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_welcome WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_invite WHERE fission_id = $FISSION_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_fission_contact WHERE id = $CONTACT_RECORD_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_fissions' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomFission/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomFission/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomFission/store" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomFission/info" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomFission/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomFission/destroy" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomFission/invite" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomFission/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomFission/showRoom" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomFission/showContact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomFission/writeOff" "$GO_LOG"

echo "room fission dashboard smoke passed"
