#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-clock-in-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18124}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13364}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26424}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-clock-in.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-clock-in-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001824}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1824}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91824}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91824}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918241}"
WORK_CONTACT_ID="${MOCHAT_SMOKE_WORK_CONTACT_ID:-918242}"
CLOCK_IN_CONTACT_ID="${MOCHAT_SMOKE_CLOCK_IN_CONTACT_ID:-918243}"
CLOCK_IN_RECORD_ID="${MOCHAT_SMOKE_CLOCK_IN_RECORD_ID:-918244}"

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

mkdir -p "$FILE_STORAGE_ROOT/roomClockIn"

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
  -tenant-name "群打卡验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "群打卡管理员" \
  -role-name "群打卡超级管理员" \
  -package-code "room-clock-in-standard" \
  -package-name "群打卡标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-clock-ins 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_clock_in_record WHERE clock_in_id IN (SELECT id FROM mc_room_clock_in WHERE corp_id = $CORP_ID) OR id = $CLOCK_IN_RECORD_ID;
DELETE FROM mc_room_clock_in_contact WHERE clock_in_id IN (SELECT id FROM mc_room_clock_in WHERE corp_id = $CORP_ID) OR id = $CLOCK_IN_CONTACT_ID;
DELETE FROM mc_room_clock_in WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id = $WORK_CONTACT_ID OR contact_tag_id = $TAG_ID;
DELETE FROM mc_work_contact WHERE id = $WORK_CONTACT_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '群打卡企业', 'ww-room-clock-in', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-clock-in-user', $CORP_ID, '群打卡员工', '$PHONE', 'avatar/room-clock-in.png', 'avatar/room-clock-in-thumb.png', '群打卡员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-room-clock-in-tag', $CORP_ID, '群打卡客户标签', 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($WORK_CONTACT_ID, $CORP_ID, 'external-room-clock-in-1', '群打卡客户A', '群打卡客户A', 'avatar/room-clock-in-contact.png', 1, 1, 1, 'union-room-clock-in-a', '', '', '', JSON_OBJECT(), 'CLOCK-IN-A', NOW(), NOW(), NULL);
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

STORE_BODY="$(python3 - "$TAG_ID" <<'PY'
import json
import sys

tag_id = int(sys.argv[1])
body = {
    "clockIn": {
        "official_account_id": 3,
        "active_name": "群打卡验收",
        "description": "连续打卡说明",
        "type": 1,
        "start_time": "2026-07-01 00:00:00",
        "end_time": "2026-12-31 23:59:59",
        "tasks": [{"count": 7, "name": "连续7天", "prize": "优惠券"}],
        "employee_qrcode": "roomClockIn/employee-old.png",
        "contact_tags": [tag_id],
        "corp_card_status": 1,
        "corp_card": {"name": "群打卡企业名片", "logo": "roomClockIn/logo-old.png"},
        "status": 1,
    }
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomClockIn/store" "$STORE_BODY" "$WORK_DIR/room-clock-in-store.json"
CLOCK_IN_ID="$(mysql_scalar "SELECT id FROM mc_room_clock_in WHERE corp_id = $CORP_ID AND active_name = '群打卡验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$CLOCK_IN_ID"
test "$(mysql_scalar "SELECT official_account_id FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "3"
test "$(mysql_scalar "SELECT active_name FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "群打卡验收"
test "$(mysql_scalar "SELECT description FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "连续打卡说明"
test "$(mysql_scalar "SELECT type FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "1"
test "$(mysql_scalar "SELECT DATE_FORMAT(start_time, '%Y-%m-%d %H:%i:%s') FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "2026-07-01 00:00:00"
test "$(mysql_scalar "SELECT DATE_FORMAT(end_time, '%Y-%m-%d %H:%i:%s') FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "2026-12-31 23:59:59"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(tasks, '\$[0].name')) FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "连续7天"
test "$(mysql_scalar "SELECT employee_qrcode FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "roomClockIn/employee-old.png"
test "$(mysql_scalar "SELECT JSON_EXTRACT(contact_tags, '\$[0]') FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "$TAG_ID"
test "$(mysql_scalar "SELECT corp_card_status FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "1"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(corp_card, '\$.name')) FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "群打卡企业名片"
test "$(mysql_scalar "SELECT status FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_clock_ins' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_room_clock_in_contact
  (id, clock_in_id, union_id, openid, nickname, avatar, city, contact_id, employee_ids, contact_tags, day_count, status, receive_level, write_off, first_clock_at, last_clock_at, created_at, updated_at, deleted_at)
VALUES
  ($CLOCK_IN_CONTACT_ID, $CLOCK_IN_ID, 'union-room-clock-in-a', 'openid-room-clock-in-a', '群打卡客户A', 'avatar/room-clock-in-contact.png', '杭州', $WORK_CONTACT_ID, JSON_ARRAY($EMPLOYEE_ID), JSON_ARRAY(), 3, 1, 1, 0, '2026-07-02 09:00:00', '2026-07-04 09:00:00', NOW(), NOW(), NULL);

INSERT INTO mc_room_clock_in_record
  (id, clock_in_id, contact_id, union_id, day, created_at, updated_at, deleted_at)
VALUES
  ($CLOCK_IN_RECORD_ID, $CLOCK_IN_ID, $CLOCK_IN_CONTACT_ID, 'union-room-clock-in-a', '2026-07-02', NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/roomClockIn/index?activeName=%E7%BE%A4%E6%89%93%E5%8D%A1&status=1&page=1&perPage=10" "" "$WORK_DIR/room-clock-in-index.json"
api_json GET "/dashboard/roomClockIn/show?id=$CLOCK_IN_ID" "" "$WORK_DIR/room-clock-in-show.json"
api_json GET "/dashboard/roomClockIn/info?clockInId=$CLOCK_IN_ID" "" "$WORK_DIR/room-clock-in-info.json"
api_json GET "/dashboard/roomClockIn/showContact?clockInId=$CLOCK_IN_ID&status=1&writeOff=0&nickname=%E6%89%93%E5%8D%A1&page=1&perPage=15" "" "$WORK_DIR/room-clock-in-contact.json"
api_json GET "/dashboard/roomClockIn/dayDetail?clockInId=$CLOCK_IN_ID&contactId=$CLOCK_IN_CONTACT_ID&page=1&perPage=31" "" "$WORK_DIR/room-clock-in-day.json"

python3 - \
  "$WORK_DIR/room-clock-in-index.json" \
  "$WORK_DIR/room-clock-in-show.json" \
  "$WORK_DIR/room-clock-in-info.json" \
  "$WORK_DIR/room-clock-in-contact.json" \
  "$WORK_DIR/room-clock-in-day.json" \
  "$CLOCK_IN_ID" "$CLOCK_IN_CONTACT_ID" "$EMPLOYEE_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
info_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
contact_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
day_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
clock_in_id = int(sys.argv[6])
clock_in_contact_id = int(sys.argv[7])
employee_id = int(sys.argv[8])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["clockInId"]) == clock_in_id), None)
assert item, items
assert item["activeName"] == "群打卡验收", item
assert item["tasks"][0]["name"] == "连续7天", item
assert item["employeeQrcode"] == "roomClockIn/employee-old.png", item
assert item["corpCard"]["name"] == "群打卡企业名片", item
assert int(item["contactNum"]) == 1 and int(item["clockInNum"]) == 3 and int(item["receiveNum"]) == 1, item
assert item["shareUrl"] == f"http://operation.example.com/roomClockIn?id={clock_in_id}", item

for payload in (show_payload, info_payload):
    data = payload["data"]
    assert int(data["clockInId"]) == clock_in_id, data
    assert data["name"] == "群打卡验收", data
    assert data["clockIn"]["activeName"] == "群打卡验收", data
    assert data["clockIn"]["tasks"][0]["prize"] == "优惠券", data
    assert data["shareUrl"] == f"http://operation.example.com/roomClockIn?id={clock_in_id}", data

contacts = contact_payload["data"]
assert int(contacts["page"]["total"]) == 1, contacts
contact = contacts["list"][0]
assert int(contact["id"]) == clock_in_contact_id, contact
assert contact["nickname"] == "群打卡客户A", contact
assert contact["employeeIds"][0] == employee_id, contact
assert int(contact["dayCount"]) == 3, contact
assert int(contact["status"]) == 1 and int(contact["writeOff"]) == 0, contact
assert int(contact["receiveLevel"]) == 1, contact

days = day_payload["data"]
assert int(days["page"]["total"]) == 1, days
assert days["days"] == ["2026-07-02"], days
day = days["list"][0]
assert int(day["clockInId"]) == clock_in_id, day
assert int(day["contactId"]) == clock_in_contact_id, day
assert day["day"] == "2026-07-02", day
PY

api_json PUT "/dashboard/roomClockIn/batchContactTags" "{\"clockInId\":$CLOCK_IN_ID,\"contactIds\":[$CLOCK_IN_CONTACT_ID],\"tagIds\":[$TAG_ID]}" "$WORK_DIR/room-clock-in-batch-tags.json"
test "$(mysql_scalar "SELECT JSON_EXTRACT(contact_tags, '\$[0]') FROM mc_room_clock_in_contact WHERE id = $CLOCK_IN_CONTACT_ID")" = "$TAG_ID"
python3 - "$WORK_DIR/room-clock-in-batch-tags.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["data"]["affected"] == 1, payload
PY

UPDATE_BODY="$(python3 - "$CLOCK_IN_ID" "$TAG_ID" <<'PY'
import json
import sys

clock_in_id = int(sys.argv[1])
tag_id = int(sys.argv[2])
body = {
    "clockInId": clock_in_id,
    "officialAccountId": 4,
    "activeName": "群打卡验收更新",
    "description": "累计打卡说明",
    "type": 2,
    "startTime": "2026-08-01 00:00:00",
    "endTime": "2026-12-30 23:59:59",
    "tasks": [{"count": 10, "name": "累计10天", "prize": "实物奖"}],
    "employeeQrcode": "roomClockIn/employee-new.png",
    "contactTags": [tag_id],
    "corpCardStatus": 0,
    "corpCard": {"name": "群打卡企业名片更新", "logo": "roomClockIn/logo-new.png"},
    "status": 0,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomClockIn/update" "$UPDATE_BODY" "$WORK_DIR/room-clock-in-update.json"
test "$(mysql_scalar "SELECT official_account_id FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "4"
test "$(mysql_scalar "SELECT active_name FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "群打卡验收更新"
test "$(mysql_scalar "SELECT description FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "累计打卡说明"
test "$(mysql_scalar "SELECT type FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "2"
test "$(mysql_scalar "SELECT DATE_FORMAT(start_time, '%Y-%m-%d %H:%i:%s') FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "2026-08-01 00:00:00"
test "$(mysql_scalar "SELECT DATE_FORMAT(end_time, '%Y-%m-%d %H:%i:%s') FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "2026-12-30 23:59:59"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(tasks, '\$[0].name')) FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "累计10天"
test "$(mysql_scalar "SELECT employee_qrcode FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "roomClockIn/employee-new.png"
test "$(mysql_scalar "SELECT JSON_EXTRACT(contact_tags, '\$[0]') FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "$TAG_ID"
test "$(mysql_scalar "SELECT corp_card_status FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "0"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(corp_card, '\$.name')) FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "群打卡企业名片更新"
test "$(mysql_scalar "SELECT status FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID")" = "0"

api_json GET "/dashboard/roomClockIn/show?clockInId=$CLOCK_IN_ID" "" "$WORK_DIR/room-clock-in-show-updated.json"
python3 - "$WORK_DIR/room-clock-in-show-updated.json" "$CLOCK_IN_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
clock_in_id = int(sys.argv[2])
data = payload["data"]
assert int(data["clockInId"]) == clock_in_id, data
assert data["activeName"] == "群打卡验收更新", data
assert int(data["status"]) == 0, data
assert data["statusText"] == "已停用", data
assert data["tasks"][0]["name"] == "累计10天", data
assert data["employeeQrcode"] == "roomClockIn/employee-new.png", data
PY

api_json DELETE "/dashboard/roomClockIn/destroy" "{\"clockInId\":$CLOCK_IN_ID}" "$WORK_DIR/room-clock-in-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_clock_in WHERE id = $CLOCK_IN_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_clock_in_contact WHERE id = $CLOCK_IN_CONTACT_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_clock_in_record WHERE id = $CLOCK_IN_RECORD_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_clock_ins' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomClockIn/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomClockIn/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomClockIn/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomClockIn/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomClockIn/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomClockIn/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomClockIn/showContact" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomClockIn/batchContactTags" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomClockIn/info" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomClockIn/dayDetail" "$GO_LOG"

echo "room clock in dashboard smoke passed"
