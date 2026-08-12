#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-infinite-pull-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18123}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13363}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26423}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-infinite-pull.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-infinite-pull-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001823}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1823}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91823}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91823}"

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

mkdir -p "$FILE_STORAGE_ROOT/roomInfinite"

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
  -tenant-name "无限拉群验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "无限拉群管理员" \
  -role-name "无限拉群超级管理员" \
  -package-code "room-infinite-pull-standard" \
  -package-name "无限拉群标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-infinite-pulls 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_infinite WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '无限拉群企业', 'ww-room-infinite', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-infinite-user', $CORP_ID, '无限拉群员工', '$PHONE', 'avatar/room-infinite.png', 'avatar/room-infinite-thumb.png', '无限员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);
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

STORE_BODY="$(python3 - <<'PY'
import json

body = {
    "name": "无限拉群验收",
    "avatar": "roomInfinite/avatar-old.png",
    "titleStatus": 1,
    "title": "无限拉群标题",
    "describeStatus": 1,
    "describe": "扫码加入客户群",
    "logo": "roomInfinite/logo-old.png",
    "qwCode": [
        {
            "roomName": "无限拉群A",
            "roomQrcodeUrl": "roomInfinite/room-a-old.png",
            "qrcode": "roomInfinite/qrcode-a-old.png",
            "upper_limit": 200,
            "status": 1,
        },
        {
            "roomName": "无限拉群B",
            "qrcodeUrl": "https://cdn.example.com/room-b.png",
            "upper_limit": 200,
            "status": 0,
        },
    ],
    "totalNum": 7,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomInfinitePull/store" "$STORE_BODY" "$WORK_DIR/room-infinite-store.json"
ROOM_INFINITE_ID="$(mysql_scalar "SELECT id FROM mc_room_infinite WHERE corp_id = $CORP_ID AND name = '无限拉群验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROOM_INFINITE_ID"
test "$(mysql_scalar "SELECT avatar FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "roomInfinite/avatar-old.png"
test "$(mysql_scalar "SELECT title_status FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "1"
test "$(mysql_scalar "SELECT title FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "无限拉群标题"
test "$(mysql_scalar "SELECT describe_status FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "1"
test "$(mysql_scalar "SELECT \`describe\` FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "扫码加入客户群"
test "$(mysql_scalar "SELECT logo FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "roomInfinite/logo-old.png"
test "$(mysql_scalar "SELECT total_num FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "7"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(qw_code, '\$[0].roomName')) FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "无限拉群A"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(qw_code, '\$[0].roomQrcodeUrl')) FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "roomInfinite/room-a-old.png"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_infinite_pulls' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/roomInfinitePull/index?name=%E6%97%A0%E9%99%90&page=1&perPage=10" "" "$WORK_DIR/room-infinite-index.json"
api_json GET "/dashboard/roomInfinitePull/info?id=$ROOM_INFINITE_ID" "" "$WORK_DIR/room-infinite-info.json"

python3 - "$WORK_DIR/room-infinite-index.json" "$WORK_DIR/room-infinite-info.json" "$ROOM_INFINITE_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
info_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
item_id = int(sys.argv[3])
go_addr = sys.argv[4]

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["roomInfinitePullId"]) == item_id), None)
assert item, items
assert item["name"] == "无限拉群验收", item
assert item["avatar"] == "roomInfinite/avatar-old.png", item
assert int(item["titleStatus"]) == 1, item
assert item["title"] == "无限拉群标题", item
assert int(item["describeStatus"]) == 1, item
assert item["describe"] == "扫码加入客户群", item
assert item["logo"] == "roomInfinite/logo-old.png", item
assert int(item["totalNum"]) == 7, item
assert item["qwCode"][0]["roomName"] == "无限拉群A", item
assert item["qwCode"][0]["roomQrcodeUrl"] == "roomInfinite/room-a-old.png", item
assert item["qwCode"][1]["qrcodeUrl"] == "https://cdn.example.com/room-b.png", item
assert item["link"] == f"http://{go_addr}/roomInfinitePull?id={item_id}", item

info = info_payload["data"]
assert int(info["id"]) == item_id, info
assert int(info["infiniteId"]) == item_id, info
assert info["name"] == "无限拉群验收", info
assert info["qwCode"][0]["qrcode"] == "roomInfinite/qrcode-a-old.png", info
assert info["link"] == f"http://{go_addr}/roomInfinitePull?id={item_id}", info
PY

UPDATE_BODY="$(python3 - "$ROOM_INFINITE_ID" <<'PY'
import json
import sys

item_id = int(sys.argv[1])
body = {
    "roomInfinitePullId": item_id,
    "name": "无限拉群验收更新",
    "avatar": "roomInfinite/avatar-new.png",
    "titleStatus": 0,
    "title": "无限拉群标题更新",
    "describeStatus": 0,
    "describe": "引导语更新",
    "logo": "roomInfinite/logo-new.png",
    "qwCode": [
        {
            "roomName": "无限拉群更新",
            "roomQrcodeUrl": "roomInfinite/room-new.png",
            "upper_limit": 300,
            "status": 1,
        }
    ],
    "totalNum": 11,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomInfinitePull/update" "$UPDATE_BODY" "$WORK_DIR/room-infinite-update.json"
test "$(mysql_scalar "SELECT name FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "无限拉群验收更新"
test "$(mysql_scalar "SELECT avatar FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "roomInfinite/avatar-new.png"
test "$(mysql_scalar "SELECT title_status FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "0"
test "$(mysql_scalar "SELECT title FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "无限拉群标题更新"
test "$(mysql_scalar "SELECT describe_status FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "0"
test "$(mysql_scalar "SELECT \`describe\` FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "引导语更新"
test "$(mysql_scalar "SELECT logo FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "roomInfinite/logo-new.png"
test "$(mysql_scalar "SELECT total_num FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "11"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(qw_code, '\$[0].roomName')) FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "无限拉群更新"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(qw_code, '\$[0].roomQrcodeUrl')) FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID")" = "roomInfinite/room-new.png"

api_json GET "/dashboard/roomInfinitePull/info?roomInfinitePullId=$ROOM_INFINITE_ID" "" "$WORK_DIR/room-infinite-info-updated.json"
python3 - "$WORK_DIR/room-infinite-info-updated.json" "$ROOM_INFINITE_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
item_id = int(sys.argv[2])
go_addr = sys.argv[3]
data = payload["data"]
assert int(data["roomInfinitePullId"]) == item_id, data
assert data["name"] == "无限拉群验收更新", data
assert data["avatar"] == "roomInfinite/avatar-new.png", data
assert int(data["titleStatus"]) == 0, data
assert data["title"] == "无限拉群标题更新", data
assert int(data["describeStatus"]) == 0, data
assert data["describe"] == "引导语更新", data
assert data["logo"] == "roomInfinite/logo-new.png", data
assert data["qwCode"][0]["roomName"] == "无限拉群更新", data
assert data["qwCode"][0]["roomQrcodeUrl"] == "roomInfinite/room-new.png", data
assert int(data["totalNum"]) == 11, data
assert data["link"] == f"http://{go_addr}/roomInfinitePull?id={item_id}", data
PY

api_json DELETE "/dashboard/roomInfinitePull/destroy" "{\"roomInfinitePullId\":$ROOM_INFINITE_ID}" "$WORK_DIR/room-infinite-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_infinite WHERE id = $ROOM_INFINITE_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_infinite_pulls' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomInfinitePull/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomInfinitePull/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomInfinitePull/info" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomInfinitePull/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomInfinitePull/destroy" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomInfinitePull/store" "$GO_LOG"

echo "room infinite pull dashboard smoke passed"
