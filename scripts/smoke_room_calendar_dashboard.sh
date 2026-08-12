#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-calendar-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18126}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13366}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26426}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-calendar.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-calendar-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001826}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1826}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91826}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91826}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918261}"
ROOM_RECORD_ID="${MOCHAT_SMOKE_ROOM_CALENDAR_RECORD_ID:-918262}"

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

mkdir -p "$FILE_STORAGE_ROOT/roomCalendar"

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
  -tenant-name "群日历验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "群日历管理员" \
  -role-name "群日历超级管理员" \
  -package-code "room-calendar-standard" \
  -package-name "群日历标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-calendars 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_calendar_record WHERE room_calendar_id IN (SELECT id FROM mc_room_calendar WHERE corp_id = $CORP_ID) OR id = $ROOM_RECORD_ID;
DELETE FROM mc_room_calendar_push WHERE CAST(room_calendar_id AS UNSIGNED) IN (SELECT id FROM mc_room_calendar WHERE corp_id = $CORP_ID);
DELETE FROM mc_room_calendar WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '群日历企业', 'ww-room-calendar', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-calendar-user', $CORP_ID, '群日历员工', '$PHONE', 'avatar/room-calendar.png', 'avatar/room-calendar-thumb.png', '群日历员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);
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

STORE_BODY="$(python3 - "$ROOM_ID" <<'PY'
import json
import sys

room_id = int(sys.argv[1])
body = {
    "calendar": {
        "name": "群日历验收",
        "rooms": [{"roomId": room_id, "wxChatId": "wr-room-calendar-a", "name": "日历群A"}],
        "onOff": 1,
        "pushes": [{
            "name": "首发通知",
            "day": "2026-08-01 09:30:00",
            "pushContent": [{"type": "text", "value": "群日历推送A"}],
            "onOff": 1,
            "status": 1,
        }],
    }
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomCalendar/store" "$STORE_BODY" "$WORK_DIR/room-calendar-store.json"
ROOM_CALENDAR_ID="$(mysql_scalar "SELECT id FROM mc_room_calendar WHERE corp_id = $CORP_ID AND name = '群日历验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROOM_CALENDAR_ID"
OLD_PUSH_ID="$(mysql_scalar "SELECT id FROM mc_room_calendar_push WHERE room_calendar_id = '$ROOM_CALENDAR_ID' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$OLD_PUSH_ID"
test "$(mysql_scalar "SELECT name FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "群日历验收"
test "$(mysql_scalar "SELECT JSON_LENGTH(rooms) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "1"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[0].wxChatId')) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "wr-room-calendar-a"
test "$(mysql_scalar "SELECT on_off FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "1"
test "$(mysql_scalar "SELECT name FROM mc_room_calendar_push WHERE id = $OLD_PUSH_ID")" = "首发通知"
test "$(mysql_scalar "SELECT day FROM mc_room_calendar_push WHERE id = $OLD_PUSH_ID")" = "2026-08-01 09:30:00"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(push_content, '\$[0].value')) FROM mc_room_calendar_push WHERE id = $OLD_PUSH_ID")" = "群日历推送A"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_calendars' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/roomCalendar/index?name=%E7%BE%A4%E6%97%A5%E5%8E%86&onOff=1&page=1&perPage=10" "" "$WORK_DIR/room-calendar-index.json"
api_json GET "/dashboard/roomCalendar/show?id=$ROOM_CALENDAR_ID" "" "$WORK_DIR/room-calendar-show.json"

python3 - "$WORK_DIR/room-calendar-index.json" "$WORK_DIR/room-calendar-show.json" "$ROOM_CALENDAR_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
calendar_id = int(sys.argv[3])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["roomCalendarId"]) == calendar_id), None)
assert item, items
assert item["name"] == "群日历验收", item
assert item["rooms"][0]["name"] == "日历群A", item
assert int(item["roomNum"]) == 1, item
assert int(item["pushNum"]) == 1, item

show = show_payload["data"]
assert int(show["roomCalendarId"]) == calendar_id, show
assert show["rooms"][0]["wxChatId"] == "wr-room-calendar-a", show
assert show["push"][0]["name"] == "首发通知", show
assert show["push"][0]["date"] == "2026-08-01", show
assert show["push"][0]["time"] == "09:30:00", show
assert show["push"][0]["pushContent"][0]["value"] == "群日历推送A", show
PY

ADD_ROOM_BODY="$(python3 - "$ROOM_CALENDAR_ID" "$ROOM_ID" <<'PY'
import json
import sys

calendar_id = int(sys.argv[1])
room_id = int(sys.argv[2])
body = {
    "roomCalendarId": calendar_id,
    "rooms": [{"roomId": room_id + 1, "wxChatId": "wr-room-calendar-b", "name": "日历群B"}],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomCalendar/addRoom" "$ADD_ROOM_BODY" "$WORK_DIR/room-calendar-add-room.json"
test "$(mysql_scalar "SELECT JSON_LENGTH(rooms) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "2"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[1].name')) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "日历群B"

api_json DELETE "/dashboard/roomCalendar/destroyRoom" "{\"roomCalendarId\":$ROOM_CALENDAR_ID,\"wxChatId\":\"wr-room-calendar-a\"}" "$WORK_DIR/room-calendar-destroy-room.json"
test "$(mysql_scalar "SELECT JSON_LENGTH(rooms) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "1"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[0].wxChatId')) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "wr-room-calendar-b"

UPDATE_BODY="$(python3 - "$ROOM_CALENDAR_ID" "$ROOM_ID" <<'PY'
import json
import sys

calendar_id = int(sys.argv[1])
room_id = int(sys.argv[2])
body = {
    "roomCalendarId": calendar_id,
    "name": "群日历验收更新",
    "rooms": [{"roomId": room_id + 1, "wxChatId": "wr-room-calendar-b", "name": "日历群B更新"}],
    "onOff": 2,
    "pushes": [{
        "name": "更新通知",
        "date": "2026-08-02",
        "time": "10:45:00",
        "pushContent": [{"type": "text", "value": "群日历推送B"}],
        "onOff": 2,
        "status": 2,
    }],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomCalendar/update" "$UPDATE_BODY" "$WORK_DIR/room-calendar-update.json"
NEW_PUSH_ID="$(mysql_scalar "SELECT id FROM mc_room_calendar_push WHERE room_calendar_id = '$ROOM_CALENDAR_ID' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$NEW_PUSH_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_calendar_push WHERE id = $OLD_PUSH_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT name FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "群日历验收更新"
test "$(mysql_scalar "SELECT on_off FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "2"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[0].name')) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID")" = "日历群B更新"
test "$(mysql_scalar "SELECT name FROM mc_room_calendar_push WHERE id = $NEW_PUSH_ID")" = "更新通知"
test "$(mysql_scalar "SELECT day FROM mc_room_calendar_push WHERE id = $NEW_PUSH_ID")" = "2026-08-02 10:45:00"
test "$(mysql_scalar "SELECT on_off FROM mc_room_calendar_push WHERE id = $NEW_PUSH_ID")" = "2"
test "$(mysql_scalar "SELECT status FROM mc_room_calendar_push WHERE id = $NEW_PUSH_ID")" = "2"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(push_content, '\$[0].value')) FROM mc_room_calendar_push WHERE id = $NEW_PUSH_ID")" = "群日历推送B"

api_json GET "/dashboard/roomCalendar/show?roomCalendarId=$ROOM_CALENDAR_ID" "" "$WORK_DIR/room-calendar-show-updated.json"
python3 - "$WORK_DIR/room-calendar-show-updated.json" "$ROOM_CALENDAR_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
calendar_id = int(sys.argv[2])
item = payload["data"]
assert int(item["roomCalendarId"]) == calendar_id, item
assert item["name"] == "群日历验收更新", item
assert int(item["onOff"]) == 2, item
assert item["rooms"][0]["name"] == "日历群B更新", item
assert item["push"][0]["name"] == "更新通知", item
assert item["push"][0]["pushContent"][0]["value"] == "群日历推送B", item
PY

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_room_calendar_record
  (id, room_calendar_id, push_ids, day, room_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_RECORD_ID, $ROOM_CALENDAR_ID, '[$NEW_PUSH_ID]', '2026-08-02 10:45:00', 'wr-room-calendar-b', NOW(), NOW(), NULL);
SQL

api_json DELETE "/dashboard/roomCalendar/destroy" "{\"roomCalendarId\":$ROOM_CALENDAR_ID}" "$WORK_DIR/room-calendar-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_calendar WHERE id = $ROOM_CALENDAR_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_calendar_push WHERE room_calendar_id = '$ROOM_CALENDAR_ID' AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_calendar_record WHERE id = $ROOM_RECORD_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_calendars' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomCalendar/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomCalendar/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomCalendar/addRoom" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomCalendar/destroyRoom" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomCalendar/store" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomCalendar/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomCalendar/show" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomCalendar/update" "$GO_LOG"

echo "room calendar dashboard smoke passed"
