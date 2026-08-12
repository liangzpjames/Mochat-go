#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-greeting-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18118}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13358}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26418}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-greeting.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-greeting-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001818}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1818}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91818}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91818}"
MEDIUM_ID="${MOCHAT_SMOKE_MEDIUM_ID:-91818}"
LOG_OPERATION_ID=0

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

mkdir -p "$FILE_STORAGE_ROOT/greeting"
printf '\xff\xd8\xff\xe0mochat-go-greeting\xff\xd9' >"$FILE_STORAGE_ROOT/greeting/image.jpg"

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
  -tenant-name "好友欢迎语验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "好友欢迎语管理员" \
  -role-name "好友欢迎语超级管理员" \
  -package-code "greeting-standard" \
  -package-name "好友欢迎语标准版" \
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
DELETE FROM mc_business_log WHERE business_id IN (SELECT id FROM mc_greeting WHERE corp_id = $CORP_ID) OR operation_id = $EMPLOYEE_ID;
DELETE FROM mc_greeting WHERE corp_id = $CORP_ID;
DELETE FROM mc_medium WHERE id = $MEDIUM_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '好友欢迎语企业', 'ww-greeting', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'greeting-user', $CORP_ID, '好友欢迎语员工', '$PHONE', 'avatar/greeting.png', 'avatar/greeting-thumb.png', '欢迎语员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_medium
  (id, media_id, last_upload_time, type, is_sync, content, corp_id, medium_group_id, user_id, user_name, created_at, updated_at, deleted_at)
VALUES
  ($MEDIUM_ID, '', 0, 2, 1, JSON_OBJECT('imagePath', 'greeting/image.jpg', 'title', '好友欢迎语图片'), $CORP_ID, 0, $USER_ID, '好友欢迎语管理员', NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
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

api_json POST "/dashboard/greeting/store" '{"rangeType":1,"type":"1","employees":[],"words":"全员好友欢迎语","mediumId":0}' "$WORK_DIR/greeting-store-general.json"
GENERAL_ID="$(mysql_scalar "SELECT id FROM mc_greeting WHERE corp_id = $CORP_ID AND range_type = 1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$GENERAL_ID"
test "$(mysql_scalar "SELECT type FROM mc_greeting WHERE id = $GENERAL_ID")" = "-1-"
test "$(mysql_scalar "SELECT JSON_LENGTH(employees) FROM mc_greeting WHERE id = $GENERAL_ID")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_business_log WHERE business_id = $GENERAL_ID AND event = 300 AND operation_id = $LOG_OPERATION_ID AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.words')) = '全员好友欢迎语'")" = "1"

STORE_BODY="$(python3 - "$EMPLOYEE_ID" "$MEDIUM_ID" <<'PY'
import json
import sys

body = {
    "rangeType": 2,
    "type": "1,2,1",
    "employees": [int(sys.argv[1]), int(sys.argv[1]), 0],
    "words": "指定员工好友欢迎语",
    "mediumId": int(sys.argv[2]),
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/greeting/store" "$STORE_BODY" "$WORK_DIR/greeting-store-employee.json"
GREETING_ID="$(mysql_scalar "SELECT id FROM mc_greeting WHERE corp_id = $CORP_ID AND range_type = 2 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$GREETING_ID"
test "$(mysql_scalar "SELECT type FROM mc_greeting WHERE id = $GREETING_ID")" = "-1-2-"
test "$(mysql_scalar "SELECT words FROM mc_greeting WHERE id = $GREETING_ID")" = "指定员工好友欢迎语"
test "$(mysql_scalar "SELECT medium_id FROM mc_greeting WHERE id = $GREETING_ID")" = "$MEDIUM_ID"
test "$(mysql_scalar "SELECT JSON_EXTRACT(employees, '\$[0]') FROM mc_greeting WHERE id = $GREETING_ID")" = "$EMPLOYEE_ID"
test "$(mysql_scalar "SELECT JSON_LENGTH(employees) FROM mc_greeting WHERE id = $GREETING_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_business_log WHERE business_id = $GREETING_ID AND event = 300 AND operation_id = $LOG_OPERATION_ID AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.type')) = '-1-2-'")" = "1"

api_json GET "/dashboard/greeting/index?page=1&perPage=10" "" "$WORK_DIR/greeting-index.json"
api_json GET "/dashboard/greeting/show?greetingId=$GREETING_ID" "" "$WORK_DIR/greeting-show.json"

python3 - "$WORK_DIR/greeting-index.json" "$WORK_DIR/greeting-show.json" "$GENERAL_ID" "$GREETING_ID" "$EMPLOYEE_ID" "$MEDIUM_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
general_id = int(sys.argv[3])
greeting_id = int(sys.argv[4])
employee_id = int(sys.argv[5])
medium_id = int(sys.argv[6])
go_addr = sys.argv[7]

data = index_payload["data"]
assert data["hadGeneral"] == 1, data
assert employee_id in data["hadEmployees"], data
items = data["list"]
general = next((item for item in items if int(item["greetingId"]) == general_id), None)
item = next((item for item in items if int(item["greetingId"]) == greeting_id), None)
assert general, items
assert item, items
assert general["rangeTypeText"] == "全体成员", general
assert item["typeText"] == "文本+图片", item
assert item["rangeTypeText"] == "指定企业成员", item
assert item["employees"] == ["好友欢迎语员工"], item
assert item["mediumId"] == medium_id, item
assert item["mediumContent"]["imagePath"] == "greeting/image.jpg", item
assert item["mediumContent"]["imageFullPath"] == f"http://{go_addr}/static/greeting/image.jpg", item

show = show_payload["data"]
assert int(show["greetingId"]) == greeting_id, show
assert show["rangeType"] == 2, show
assert show["words"] == "指定员工好友欢迎语", show
assert show["mediumId"] == medium_id, show
assert show["employees"][0]["employeeId"] == employee_id, show
assert show["employees"][0]["name"] == "好友欢迎语员工", show
assert show["employees"][0]["wxUserId"] == "greeting-user", show
assert show["employees"][0]["select"] is True, show
assert show["mediumContent"]["imageFullPath"] == f"http://{go_addr}/static/greeting/image.jpg", show
PY

UPDATE_BODY="$(python3 - "$GREETING_ID" "$MEDIUM_ID" <<'PY'
import json
import sys

body = {
    "greetingId": int(sys.argv[1]),
    "rangeType": 1,
    "type": "2",
    "employees": [],
    "words": "更新后的好友欢迎语",
    "mediumId": int(sys.argv[2]),
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/greeting/update" "$UPDATE_BODY" "$WORK_DIR/greeting-update.json"
test "$(mysql_scalar "SELECT type FROM mc_greeting WHERE id = $GREETING_ID")" = "-2-"
test "$(mysql_scalar "SELECT range_type FROM mc_greeting WHERE id = $GREETING_ID")" = "1"
test "$(mysql_scalar "SELECT words FROM mc_greeting WHERE id = $GREETING_ID")" = "更新后的好友欢迎语"
test "$(mysql_scalar "SELECT JSON_LENGTH(employees) FROM mc_greeting WHERE id = $GREETING_ID")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_business_log WHERE business_id = $GREETING_ID AND event = 301 AND operation_id = $LOG_OPERATION_ID AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.words')) = '更新后的好友欢迎语'")" = "1"

api_json GET "/dashboard/greeting/show?greetingId=$GREETING_ID" "" "$WORK_DIR/greeting-show-updated.json"
python3 - "$WORK_DIR/greeting-show-updated.json" "$GREETING_ID" "$MEDIUM_ID" "$GO_ADDR" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
greeting_id = int(sys.argv[2])
medium_id = int(sys.argv[3])
go_addr = sys.argv[4]
data = payload["data"]
assert int(data["greetingId"]) == greeting_id, data
assert data["rangeType"] == 1, data
assert data["employees"] == [], data
assert data["words"] == "更新后的好友欢迎语", data
assert data["mediumId"] == medium_id, data
assert data["mediumContent"]["imageFullPath"] == f"http://{go_addr}/static/greeting/image.jpg", data
PY

api_json DELETE "/dashboard/greeting/destroy" "{\"greetingId\":$GREETING_ID}" "$WORK_DIR/greeting-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_greeting WHERE id = $GREETING_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_greeting WHERE id = $GENERAL_ID AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/greeting/index?page=1&perPage=10" "" "$WORK_DIR/greeting-index-after-delete.json"
python3 - "$WORK_DIR/greeting-index-after-delete.json" "$GENERAL_ID" "$GREETING_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
general_id = int(sys.argv[2])
deleted_id = int(sys.argv[3])
ids = {int(item["greetingId"]) for item in payload["data"]["list"]}
assert general_id in ids, ids
assert deleted_id not in ids, ids
assert payload["data"]["hadGeneral"] == 1, payload
PY

grep -q "go migrated route enabled: GET /dashboard/greeting/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/greeting/show" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/greeting/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/greeting/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/greeting/destroy" "$GO_LOG"

echo "greeting dashboard smoke passed"
