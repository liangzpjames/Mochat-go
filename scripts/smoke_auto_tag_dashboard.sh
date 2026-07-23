#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-auto-tag-dashboard-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18135}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13375}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26435}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-auto-tag-dashboard.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-auto-tag-dashboard-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001835}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1835}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91850}"
ADMIN_EMPLOYEE_ID="${MOCHAT_SMOKE_ADMIN_EMPLOYEE_ID:-91850}"
MEMBER_EMPLOYEE_ID="${MOCHAT_SMOKE_MEMBER_EMPLOYEE_ID:-91851}"
CONTACT_ID="${MOCHAT_SMOKE_CONTACT_ID:-91852}"
CONTACT_EMPLOYEE_ID="${MOCHAT_SMOKE_CONTACT_EMPLOYEE_ID:-91853}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-91854}"
CONTACT_ROOM_ID="${MOCHAT_SMOKE_CONTACT_ROOM_ID:-91855}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-918501}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918502}"
MESSAGE_ID="${MOCHAT_SMOKE_MESSAGE_ID:-91856}"

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

assert_api_ok() {
  local out="$1"
  python3 - "$out" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("code") != 200:
    raise SystemExit(json.dumps(payload, ensure_ascii=False))
PY
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
  assert_api_ok "$out"
}

json_id_from_response() {
  local file="$1"
  python3 - "$file" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
data = payload.get("data")
if not isinstance(data, list) or not data:
    raise SystemExit(json.dumps(payload, ensure_ascii=False))
print(int(data[0]))
PY
}

assert_port_free "$GO_ADDR"
assert_port_free "127.0.0.1:$MYSQL_PORT"
assert_port_free "127.0.0.1:$REDIS_PORT"

mkdir -p "$FILE_STORAGE_ROOT"

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
grep -q $'0014_auto_tag\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0022_work_message_archive_sync\tbaselined' "$WORK_DIR/migrate-baseline.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id 1 \
  -tenant-name "自动标签后台验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "自动标签后台管理员" \
  -role-name "自动标签后台超级管理员" \
  -package-code "auto-tag-dashboard-standard" \
  -package-name "自动标签后台标准版" \
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
DELETE FROM mc_auto_tag_record
WHERE corp_id = $CORP_ID
   OR auto_tag_id IN (SELECT id FROM mc_auto_tag WHERE corp_id = $CORP_ID)
   OR contact_id = $CONTACT_ID
   OR contact_room_id = $CONTACT_ROOM_ID;
DELETE FROM mc_auto_tag WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_message_1 WHERE corp_id = $CORP_ID OR id = $MESSAGE_ID;
DELETE FROM mc_work_contact_room WHERE id = $CONTACT_ROOM_ID OR room_id = $ROOM_ID OR contact_id = $CONTACT_ID;
DELETE FROM mc_work_contact_employee WHERE id = $CONTACT_EMPLOYEE_ID OR corp_id = $CORP_ID OR contact_id = $CONTACT_ID OR employee_id IN ($ADMIN_EMPLOYEE_ID, $MEMBER_EMPLOYEE_ID);
DELETE FROM mc_work_contact WHERE id = $CONTACT_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_room WHERE id = $ROOM_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag_group WHERE id = $TAG_GROUP_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id IN ($ADMIN_EMPLOYEE_ID, $MEMBER_EMPLOYEE_ID) OR corp_id = $CORP_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, social_code, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '自动标签验收企业', 'ww-auto-tag-dashboard', '91310000AUTOTAG', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, position, gender, email, avatar, thumb_avatar, telephone, alias, extattr, status, qr_code, external_profile, external_position, address, open_user_id, wx_main_department_id, main_department_id, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($ADMIN_EMPLOYEE_ID, 'auto-tag-admin', $CORP_ID, '自动标签管理员', '$PHONE', '运营', 1, '', 'avatar/auto-tag-admin.png', 'avatar/auto-tag-admin-thumb.png', '', '自动标签管理员别名', JSON_OBJECT(), 1, '', JSON_OBJECT(), '', '', 'open-auto-tag-admin', 1, 0, $USER_ID, 1, 1, NOW(), NOW(), NULL),
  ($MEMBER_EMPLOYEE_ID, 'auto-tag-member', $CORP_ID, '自动标签成员', '13800001851', '销售', 2, '', 'avatar/auto-tag-member.png', 'avatar/auto-tag-member-thumb.png', '', '自动标签成员别名', JSON_OBJECT(), 1, '', JSON_OBJECT(), '', '', 'open-auto-tag-member', 1, 0, 0, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ID, $CORP_ID, 'external-auto-tag-dashboard', '自动标签客户', '自动标签客户昵称', 'avatar/auto-tag-contact.png', 2, 1, 2, 'union-auto-tag-dashboard', '', '客户企业', '客户企业全称', JSON_OBJECT(), 'AUTO-TAG-CUSTOMER', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_EMPLOYEE_ID, $ADMIN_EMPLOYEE_ID, $CONTACT_ID, '自动标签客户备注', '自动标签客户描述', '客户企业', JSON_ARRAY('13800001852'), 2, 'auto-tag-admin', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'wr-auto-tag-dashboard', '自动标签客户群', $ADMIN_EMPLOYEE_ID, '自动标签群公告', 0, NOW(), 200, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_room
  (id, wx_user_id, contact_id, employee_id, unionid, room_id, join_scene, type, status, join_time, out_time, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ROOM_ID, 'external-auto-tag-dashboard', $CONTACT_ID, $ADMIN_EMPLOYEE_ID, 'union-auto-tag-dashboard', $ROOM_ID, 3, 2, 1, NOW(), '', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'wx-auto-tag-dashboard-group', $CORP_ID, '自动标签标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-auto-tag-dashboard-tag', $CORP_ID, '高意向', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_message_1
  (id, corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, deleted_at, created_at, updated_at)
VALUES
  ($MESSAGE_ID, $CORP_ID, 'auto-tag-dashboard-msg-1', 1, $ADMIN_EMPLOYEE_ID, 1, $CONTACT_ID, 0, 0, 1, 1, JSON_OBJECT('content', '自动标签会话内容'), '自动标签会话内容', 0, 1, NOW(), NULL, NOW(), NOW());
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=1 \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_FILE_STORAGE_ROOT="$FILE_STORAGE_ROOT" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

grep -q "go migrated route enabled: POST /dashboard/autoTag/store" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/autoTag/showContactRoom" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/workMessage/index" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/workMessageConfig/stepUpdate" "$GO_LOG"

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

KEYWORD_BODY="$(python3 - "$ADMIN_EMPLOYEE_ID" "$TAG_ID" <<'PY'
import json
import sys

employee = int(sys.argv[1])
tag = int(sys.argv[2])
print(json.dumps({
    "type": 1,
    "name": "关键词自动标签验收",
    "employees": [{"id": employee, "name": "自动标签管理员", "wxUserId": "auto-tag-admin"}],
    "fuzzy_match_keyword": ["报价"],
    "exact_match_keyword": ["马上下单"],
    "tag_rule": [{
        "id": 101,
        "time_type": 1,
        "trigger_count": 1,
        "tags": [{"tagid": tag, "tagname": "高意向"}],
    }],
    "on_off": 1,
}, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/autoTag/store" "$KEYWORD_BODY" "$WORK_DIR/auto-tag-keyword-store.json"
KEYWORD_TAG_ID="$(json_id_from_response "$WORK_DIR/auto-tag-keyword-store.json")"
test -n "$KEYWORD_TAG_ID"

ROOM_BODY="$(python3 - "$ROOM_ID" "$TAG_ID" <<'PY'
import json
import sys

room = int(sys.argv[1])
tag = int(sys.argv[2])
print(json.dumps({
    "type": 2,
    "name": "入群自动标签验收",
    "tag_rule": [{
        "id": 201,
        "rooms": [room],
        "tags": [{"tagid": tag, "tagname": "高意向"}],
    }],
    "onOff": 1,
}, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/autoTag/store" "$ROOM_BODY" "$WORK_DIR/auto-tag-room-store.json"
ROOM_TAG_ID="$(json_id_from_response "$WORK_DIR/auto-tag-room-store.json")"
test -n "$ROOM_TAG_ID"

TIME_BODY="$(python3 - "$ADMIN_EMPLOYEE_ID" "$TAG_ID" <<'PY'
import json
import sys

employee = int(sys.argv[1])
tag = int(sys.argv[2])
print(json.dumps({
    "type": 3,
    "name": "分时段自动标签验收",
    "employees": [employee],
    "tagRule": [{
        "id": 301,
        "time_type": 1,
        "start_time": "00:00",
        "end_time": "23:59",
        "tags": [{"tagid": tag, "tagname": "高意向"}],
    }],
    "on_off": 1,
}, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/autoTag/store" "$TIME_BODY" "$WORK_DIR/auto-tag-time-store.json"
TIME_TAG_ID="$(json_id_from_response "$WORK_DIR/auto-tag-time-store.json")"
test -n "$TIME_TAG_ID"

test "$(mysql_scalar "SELECT COUNT(*) FROM mc_auto_tag WHERE corp_id = $CORP_ID AND id IN ($KEYWORD_TAG_ID, $ROOM_TAG_ID, $TIME_TAG_ID) AND tenant_id = 1 AND create_user_id = $USER_ID AND on_off = 1 AND deleted_at IS NULL")" = "3"
test "$(mysql_scalar "SELECT type, name, JSON_LENGTH(fuzzy_match_keyword), JSON_LENGTH(exact_match_keyword), JSON_LENGTH(tag_rule), JSON_LENGTH(tags) FROM mc_auto_tag WHERE id = $KEYWORD_TAG_ID")" = $'1\t关键词自动标签验收\t1\t1\t1\t1'
test "$(mysql_scalar "SELECT type, name, JSON_EXTRACT(tag_rule, '\$[0].rooms[0]'), JSON_UNQUOTE(JSON_EXTRACT(tags, '\$[0]')) FROM mc_auto_tag WHERE id = $ROOM_TAG_ID")" = $'2\t入群自动标签验收\t'"$ROOM_ID"$'\t高意向'
test "$(mysql_scalar "SELECT type, name, JSON_EXTRACT(employees, '\$[0]'), JSON_UNQUOTE(JSON_EXTRACT(tags, '\$[0]')) FROM mc_auto_tag WHERE id = $TIME_TAG_ID")" = $'3\t分时段自动标签验收\t'"$ADMIN_EMPLOYEE_ID"$'\t高意向'

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_auto_tag_record
  (auto_tag_id, contact_id, tag_rule_id, wx_external_userid, employee_id, keyword, contact_room_id, tags, corp_id, trigger_count, status, created_at, updated_at, deleted_at)
VALUES
  ($KEYWORD_TAG_ID, $CONTACT_ID, 101, 'external-auto-tag-dashboard', $ADMIN_EMPLOYEE_ID, '报价', 0, JSON_ARRAY(JSON_OBJECT('tagid', $TAG_ID, 'tagname', '高意向')), $CORP_ID, 1, 1, NOW(), NOW(), NULL),
  ($ROOM_TAG_ID, $CONTACT_ID, 201, 'external-auto-tag-dashboard', $ADMIN_EMPLOYEE_ID, '', $CONTACT_ROOM_ID, JSON_ARRAY(JSON_OBJECT('tagid', $TAG_ID, 'tagname', '高意向')), $CORP_ID, 1, 1, NOW(), NOW(), NULL),
  ($TIME_TAG_ID, $CONTACT_ID, 301, 'external-auto-tag-dashboard', $ADMIN_EMPLOYEE_ID, '', 0, JSON_ARRAY(JSON_OBJECT('tagid', $TAG_ID, 'tagname', '高意向')), $CORP_ID, 1, 1, NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/autoTag/index?type=1&name=关键词&page=1&perPage=10" "" "$WORK_DIR/auto-tag-index.json"
api_json GET "/dashboard/autoTag/show?id=$KEYWORD_TAG_ID" "" "$WORK_DIR/auto-tag-show-keyword.json"
api_json GET "/dashboard/autoTag/showContactKeyWord?id=$KEYWORD_TAG_ID&page=1&perPage=10" "" "$WORK_DIR/auto-tag-keyword-records.json"
api_json GET "/dashboard/autoTag/showContactRoom?id=$ROOM_TAG_ID&roomName=自动标签&page=1&perPage=10" "" "$WORK_DIR/auto-tag-room-records.json"
api_json GET "/dashboard/autoTag/showContactTime?id=$TIME_TAG_ID&contactName=自动标签客户&page=1&perPage=10" "" "$WORK_DIR/auto-tag-time-records.json"

python3 - "$WORK_DIR/auto-tag-index.json" "$WORK_DIR/auto-tag-show-keyword.json" "$WORK_DIR/auto-tag-keyword-records.json" "$WORK_DIR/auto-tag-room-records.json" "$WORK_DIR/auto-tag-time-records.json" "$KEYWORD_TAG_ID" "$ROOM_TAG_ID" "$TIME_TAG_ID" "$ROOM_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))["data"]
keyword_records = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))["data"]
room_records = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))["data"]
time_records = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))["data"]
keyword_id = int(sys.argv[6])
room_id = int(sys.argv[7])
time_id = int(sys.argv[8])
work_room_id = int(sys.argv[9])

items = index_payload.get("list") or []
item = next((value for value in items if int(value["id"]) == keyword_id), None)
assert item, index_payload
assert item["name"] == "关键词自动标签验收", item
assert item["tags"] == ["高意向"], item
assert item["fuzzy_match_keyword"] == ["报价"], item
assert item["exact_match_keyword"] == ["马上下单"], item

auto_tag = show_payload["auto_tag"]
assert int(auto_tag["id"]) == keyword_id, auto_tag
assert show_payload["statistics"]["total_count"] == 1, show_payload

keyword_list = keyword_records.get("list") or []
assert len(keyword_list) == 1, keyword_records
keyword_record = keyword_list[0]
assert int(keyword_record["auto_tag_id"]) == keyword_id, keyword_record
assert keyword_record["contact_name"] == "自动标签客户", keyword_record
assert keyword_record["employee_name"] == "自动标签管理员", keyword_record
assert keyword_record["keyword"] == "报价", keyword_record
assert keyword_record["tags"][0]["tagname"] == "高意向", keyword_record

room_list = room_records.get("list") or []
assert len(room_list) == 1, room_records
room_record = room_list[0]
assert int(room_record["auto_tag_id"]) == room_id, room_record
assert int(room_record["room_id"]) == work_room_id, room_record
assert room_record["room_name"] == "自动标签客户群", room_record
assert room_record["join_scene"] == 3, room_record

time_list = time_records.get("list") or []
assert len(time_list) == 1, time_records
time_record = time_list[0]
assert int(time_record["auto_tag_id"]) == time_id, time_record
assert time_record["contact_name"] == "自动标签客户", time_record
assert time_record["employee_name"] == "自动标签管理员", time_record
PY

api_json PUT "/dashboard/autoTag/onOff" "{\"id\":$KEYWORD_TAG_ID,\"onOff\":2}" "$WORK_DIR/auto-tag-onoff.json"
test "$(mysql_scalar "SELECT on_off FROM mc_auto_tag WHERE id = $KEYWORD_TAG_ID AND corp_id = $CORP_ID")" = "2"

api_json DELETE "/dashboard/autoTag/destroy" "{\"autoTagId\":$ROOM_TAG_ID}" "$WORK_DIR/auto-tag-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_auto_tag WHERE id = $ROOM_TAG_ID AND corp_id = $CORP_ID AND deleted_at IS NOT NULL")" = "1"

api_json GET "/dashboard/workMessage/fromUsers?name=自动标签&page=1&perPage=10" "" "$WORK_DIR/work-message-from-users.json"
api_json GET "/dashboard/workMessage/toUsers?workEmployeeId=$ADMIN_EMPLOYEE_ID&toUsertype=1&page=1&perPage=10" "" "$WORK_DIR/work-message-to-users.json"
api_json GET "/dashboard/workMessage/index?workEmployeeId=$ADMIN_EMPLOYEE_ID&toUserType=1&toUserId=$CONTACT_ID&content=会话&page=1&perPage=10" "" "$WORK_DIR/work-message-index.json"

python3 - "$WORK_DIR/work-message-from-users.json" "$WORK_DIR/work-message-to-users.json" "$WORK_DIR/work-message-index.json" "$ADMIN_EMPLOYEE_ID" "$CONTACT_ID" <<'PY'
import json
import pathlib
import sys

from_users = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
to_users = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))["data"]
messages = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))["data"]
employee_id = int(sys.argv[4])
contact_id = int(sys.argv[5])

assert any(int(item["id"]) == employee_id and item["name"] == "自动标签管理员" for item in from_users), from_users
targets = to_users.get("list") or []
target = next((item for item in targets if int(item["to_user_id"]) == contact_id), None)
assert target, to_users
assert target["name"] == "自动标签客户", target
assert target["content"] == "自动标签会话内容", target
message_list = messages.get("list") or []
assert len(message_list) == 1, messages
message = message_list[0]
assert message["content"]["content"] == "自动标签会话内容", message
assert message["is_current_user"] == 1, message
assert message["name"] == "自动标签管理员", message
PY

api_json POST "/dashboard/workMessageConfig/corpStore" '{"socialCode":"91310000UPDATED","chatAdmin":"会话管理员","chatAdminPhone":"13800009999","chatAdminIdcard":"310101199001011234","chatApplyStatus":2,"chatStatus":1}' "$WORK_DIR/work-message-config-store.json"
test "$(mysql_scalar "SELECT social_code, chat_admin, chat_admin_phone, chat_admin_idcard, chat_apply_status, chat_status FROM mc_corp WHERE id = $CORP_ID")" = $'91310000UPDATED\t会话管理员\t13800009999\t310101199001011234\t2\t1'

api_json PUT "/dashboard/workMessageConfig/stepUpdate" '{"chatSecret":"chat-secret-dashboard","serviceContactUrl":"https://example.com/service-contact.png","chatWhitelistIp":["127.0.0.1","10.0.0.1"],"chatRsaKey":{"publicKey":"public-key-dashboard","privateKey":"private-key-dashboard"}}' "$WORK_DIR/work-message-config-step-update.json"
test "$(mysql_scalar "SELECT chat_secret, service_contact_url, JSON_LENGTH(chat_whitelist_ip), JSON_UNQUOTE(JSON_EXTRACT(chat_rsa_key, '\$.publicKey')) FROM mc_corp WHERE id = $CORP_ID")" = $'chat-secret-dashboard\thttps://example.com/service-contact.png\t2\tpublic-key-dashboard'

api_json GET "/dashboard/workMessageConfig/corpShow?corpId=$CORP_ID" "" "$WORK_DIR/work-message-config-show.json"
api_json GET "/dashboard/workMessageConfig/corpIndex?corpName=自动标签&page=1&perPage=10" "" "$WORK_DIR/work-message-config-index.json"
api_json GET "/dashboard/workMessageConfig/stepCreate" "" "$WORK_DIR/work-message-config-step-create.json"

python3 - "$WORK_DIR/work-message-config-show.json" "$WORK_DIR/work-message-config-index.json" "$WORK_DIR/work-message-config-step-create.json" "$CORP_ID" <<'PY'
import json
import pathlib
import sys

show = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
index = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))["data"]
step = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))["data"]
corp_id = int(sys.argv[4])

assert show["corp_id"] == corp_id, show
assert show["corpName"] == "自动标签验收企业", show
assert show["social_code"] == "91310000UPDATED", show
assert show["chat_admin"] == "会话管理员", show
assert show["chat_status"] == 1, show
assert show["chat_secret"] == "chat-secret-dashboard", show
assert show["chat_whitelist_ip"] == ["127.0.0.1", "10.0.0.1"], show
items = index.get("list") or []
assert any(int(item["corp_id"]) == corp_id for item in items), index
assert step["corp_id"] == corp_id, step
assert step["rsaPublicKey"] == "public-key-dashboard", step
assert step["rsaPrivateKey"] == "private-key-dashboard", step
PY

echo "auto tag dashboard standalone smoke passed"
