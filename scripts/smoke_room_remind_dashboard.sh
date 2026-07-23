#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-remind-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18125}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13365}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26425}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-remind.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-remind-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001825}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1825}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91825}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91825}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918251}"
ROOM_RECORD_ID="${MOCHAT_SMOKE_ROOM_REMIND_RECORD_ID:-918252}"

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

mkdir -p "$FILE_STORAGE_ROOT/roomRemind"

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
  -tenant-name "客户群提醒验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "客户群提醒管理员" \
  -role-name "客户群提醒超级管理员" \
  -package-code "room-remind-standard" \
  -package-name "客户群提醒标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-reminds 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_remind_record WHERE corp_id = $CORP_ID OR remind_id IN (SELECT id FROM mc_room_remind WHERE corp_id = $CORP_ID) OR id = $ROOM_RECORD_ID;
DELETE FROM mc_room_remind WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '客户群提醒企业', 'ww-room-remind', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-remind-user', $CORP_ID, '客户群提醒员工', '$PHONE', 'avatar/room-remind.png', 'avatar/room-remind-thumb.png', '群提醒员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);
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
    "remind": {
        "name": "客户群提醒验收",
        "rooms": [{"roomId": room_id, "wxChatId": "wr-room-remind-a", "name": "提醒群A"}],
        "isQrcode": 1,
        "isLink": 1,
        "isMiniprogram": 1,
        "isCard": 0,
        "keyword": "报价",
        "status": 1,
    }
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomRemind/store" "$STORE_BODY" "$WORK_DIR/room-remind-store.json"
ROOM_REMIND_ID="$(mysql_scalar "SELECT id FROM mc_room_remind WHERE corp_id = $CORP_ID AND name = '客户群提醒验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROOM_REMIND_ID"
test "$(mysql_scalar "SELECT name FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "客户群提醒验收"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[0].name')) FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "提醒群A"
test "$(mysql_scalar "SELECT is_qrcode FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT is_link FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT is_miniprogram FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT is_card FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "0"
test "$(mysql_scalar "SELECT is_keyword FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT keyword FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "报价"
test "$(mysql_scalar "SELECT status FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_reminds' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_room_remind_record
  (id, remind_id, message_id, room_id, type, content, keyword, corp_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_RECORD_ID, $ROOM_REMIND_ID, 182501, $ROOM_ID, 5, '报价关键词命中', '报价', $CORP_ID, NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/roomRemind/index?name=%E6%8F%90%E9%86%92&remindKeyword=%E6%8A%A5%E4%BB%B7&page=1&perPage=10" "" "$WORK_DIR/room-remind-index.json"
api_json GET "/dashboard/roomRemind/info?id=$ROOM_REMIND_ID" "" "$WORK_DIR/room-remind-info.json"
api_json GET "/dashboard/task/roomRemind?remindKeyword=%E6%8A%A5%E4%BB%B7&page=1&perPage=10" "" "$WORK_DIR/room-remind-task.json"

python3 - "$WORK_DIR/room-remind-index.json" "$WORK_DIR/room-remind-info.json" "$WORK_DIR/room-remind-task.json" "$ROOM_REMIND_ID" "$ROOM_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
info_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
task_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
remind_id = int(sys.argv[4])
room_id = int(sys.argv[5])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["roomRemindId"]) == remind_id), None)
assert item, items
assert item["name"] == "客户群提醒验收", item
assert item["rooms"][0]["roomId"] == room_id, item
assert item["keyword"] == "报价", item
assert int(item["roomNum"]) == 1 and int(item["recordNum"]) == 1, item
assert item["enabledTypes"] == ["qrcode", "link", "miniprogram", "keyword"], item

info = info_payload["data"]
assert int(info["roomRemindId"]) == remind_id, info
assert info["rooms"][0]["wxChatId"] == "wr-room-remind-a", info
assert info["statusText"] == "已启用", info
assert int(info["messageRecordNum"]) == 1, info

tasks = task_payload["data"]["list"]
task = next((row for row in tasks if int(row["remindId"]) == remind_id), None)
assert task, tasks
assert task["keyword"] == "报价", task
assert int(task["status"]) == 1, task
PY

api_json GET "/dashboard/roomRemind/status?id=$ROOM_REMIND_ID&status=0" "" "$WORK_DIR/room-remind-status-off.json"
test "$(mysql_scalar "SELECT status FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "0"

UPDATE_BODY="$(python3 - "$ROOM_REMIND_ID" "$ROOM_ID" <<'PY'
import json
import sys

remind_id = int(sys.argv[1])
room_id = int(sys.argv[2])
body = {
    "roomRemindId": remind_id,
    "name": "客户群提醒验收更新",
    "rooms": [
        {"roomId": room_id, "wxChatId": "wr-room-remind-a", "name": "提醒群A更新"},
        {"roomId": room_id + 1, "wxChatId": "wr-room-remind-b", "name": "提醒群B"},
    ],
    "isQrcode": 0,
    "isLink": 1,
    "isMiniprogram": 0,
    "isCard": 1,
    "isKeyword": 1,
    "keyword": "二维码",
    "status": 1,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomRemind/update" "$UPDATE_BODY" "$WORK_DIR/room-remind-update.json"
test "$(mysql_scalar "SELECT name FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "客户群提醒验收更新"
test "$(mysql_scalar "SELECT JSON_LENGTH(rooms) FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "2"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[1].name')) FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "提醒群B"
test "$(mysql_scalar "SELECT is_qrcode FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "0"
test "$(mysql_scalar "SELECT is_link FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT is_miniprogram FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "0"
test "$(mysql_scalar "SELECT is_card FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT is_keyword FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"
test "$(mysql_scalar "SELECT keyword FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "二维码"
test "$(mysql_scalar "SELECT status FROM mc_room_remind WHERE id = $ROOM_REMIND_ID")" = "1"

api_json GET "/dashboard/roomRemind/info?roomRemindId=$ROOM_REMIND_ID" "" "$WORK_DIR/room-remind-info-updated.json"
api_json GET "/dashboard/task/roomRemind?remindKeyword=%E4%BA%8C%E7%BB%B4%E7%A0%81&page=1&perPage=10" "" "$WORK_DIR/room-remind-task-updated.json"
python3 - "$WORK_DIR/room-remind-info-updated.json" "$WORK_DIR/room-remind-task-updated.json" "$ROOM_REMIND_ID" <<'PY'
import json
import pathlib
import sys

info_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
task_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
remind_id = int(sys.argv[3])

info = info_payload["data"]
assert int(info["roomRemindId"]) == remind_id, info
assert info["name"] == "客户群提醒验收更新", info
assert [room["name"] for room in info["rooms"]] == ["提醒群A更新", "提醒群B"], info
assert info["enabledTypes"] == ["link", "card", "keyword"], info
tasks = task_payload["data"]["list"]
assert any(int(row["id"]) == remind_id and row["keyword"] == "二维码" for row in tasks), tasks
PY

api_json DELETE "/dashboard/roomRemind/destroy" "{\"roomRemindId\":$ROOM_REMIND_ID}" "$WORK_DIR/room-remind-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_remind WHERE id = $ROOM_REMIND_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_remind_record WHERE id = $ROOM_RECORD_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_reminds' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomRemind/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomRemind/index" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomRemind/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomRemind/info" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomRemind/status" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomRemind/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomRemind/update" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/task/roomRemind" "$GO_LOG"

echo "room remind dashboard smoke passed"
