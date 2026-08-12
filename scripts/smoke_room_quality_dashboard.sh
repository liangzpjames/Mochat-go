#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-quality-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18127}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13367}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26427}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-quality.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-quality-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001827}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1827}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91827}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91827}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918271}"
WORK_CONTACT_ID="${MOCHAT_SMOKE_WORK_CONTACT_ID:-918272}"
QUALITY_CONTACT_ID="${MOCHAT_SMOKE_ROOM_QUALITY_CONTACT_ID:-918273}"

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

mkdir -p "$FILE_STORAGE_ROOT/roomQuality"

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
  -tenant-name "群质检验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "群质检管理员" \
  -role-name "群质检超级管理员" \
  -package-code "room-quality-standard" \
  -package-name "群质检标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -room-qualities 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_room_quality_contact WHERE quality_id IN (SELECT id FROM mc_room_quality WHERE corp_id = $CORP_ID) OR id = $QUALITY_CONTACT_ID OR room_id = $ROOM_ID OR contact_id = $WORK_CONTACT_ID;
DELETE FROM mc_room_quality WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '群质检企业', 'ww-room-quality', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-quality-user', $CORP_ID, '群质检员工', '$PHONE', 'avatar/room-quality.png', 'avatar/room-quality-thumb.png', '群质检员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);
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

STORE_BODY="$(python3 - "$ROOM_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

room_id = int(sys.argv[1])
employee_id = int(sys.argv[2])
body = {
    "quality": {
        "name": "群质检验收",
        "description": "广告和二维码触发质检",
        "rule": [{
            "type": "keyword",
            "keyword": "广告",
            "threshold": 2,
            "timeRange": "1h",
            "showEmployee": [employee_id],
        }],
        "rooms": [{"roomId": room_id, "wxChatId": "wr-room-quality-a", "name": "质检群A"}],
        "status": 1,
    }
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomQuality/store" "$STORE_BODY" "$WORK_DIR/room-quality-store.json"
ROOM_QUALITY_ID="$(mysql_scalar "SELECT id FROM mc_room_quality WHERE corp_id = $CORP_ID AND name = '群质检验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROOM_QUALITY_ID"
test "$(mysql_scalar "SELECT description FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "广告和二维码触发质检"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(\`rule\`, '\$[0].keyword')) FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "广告"
test "$(mysql_scalar "SELECT JSON_EXTRACT(\`rule\`, '\$[0].showEmployee[0]') FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "$EMPLOYEE_ID"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[0].name')) FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "质检群A"
test "$(mysql_scalar "SELECT status FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_qualities' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_room_quality_contact
  (id, quality_id, room_id, room_name, contact_id, external_user_id, nickname, avatar, employee_ids, content, msg_type, trigger_at, status, created_at, updated_at, deleted_at)
VALUES
  ($QUALITY_CONTACT_ID, $ROOM_QUALITY_ID, $ROOM_ID, '质检群A', $WORK_CONTACT_ID, 'external-room-quality-a', '触发客户A', 'avatar/room-quality-contact.png', JSON_ARRAY($EMPLOYEE_ID), '客户连续发送广告内容', 'text', NOW(), 0, NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/roomQuality/index?name=%E7%BE%A4%E8%B4%A8%E6%A3%80&status=1&page=1&perPage=10" "" "$WORK_DIR/room-quality-index.json"
api_json GET "/dashboard/roomQuality/info?id=$ROOM_QUALITY_ID" "" "$WORK_DIR/room-quality-info.json"
api_json GET "/dashboard/roomQuality/showContact?roomQualityId=$ROOM_QUALITY_ID&status=0&nickname=%E8%A7%A6%E5%8F%91&page=1&perPage=10" "" "$WORK_DIR/room-quality-show-contact.json"
api_json GET "/dashboard/roomQuality/contactDetail?contactRecordId=$QUALITY_CONTACT_ID" "" "$WORK_DIR/room-quality-contact-detail.json"

python3 - "$WORK_DIR/room-quality-index.json" "$WORK_DIR/room-quality-info.json" "$WORK_DIR/room-quality-show-contact.json" "$WORK_DIR/room-quality-contact-detail.json" "$ROOM_QUALITY_ID" "$QUALITY_CONTACT_ID" "$EMPLOYEE_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
info_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
contacts_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
detail_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
quality_id = int(sys.argv[5])
record_id = int(sys.argv[6])
employee_id = int(sys.argv[7])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["roomQualityId"]) == quality_id), None)
assert item, items
assert item["name"] == "群质检验收", item
assert item["rule"][0]["keyword"] == "广告", item
assert item["rooms"][0]["name"] == "质检群A", item
assert int(item["roomNum"]) == 1 and int(item["contactNum"]) == 1, item
assert item["statusText"] == "已启用", item

info = info_payload["data"]
assert int(info["roomQualityId"]) == quality_id, info
assert info["description"] == "广告和二维码触发质检", info
assert info["rule"][0]["showEmployee"][0] == employee_id, info

contacts = contacts_payload["data"]["list"]
contact = next((row for row in contacts if int(row["contactRecordId"]) == record_id), None)
assert contact, contacts
assert contact["nickname"] == "触发客户A", contact
assert contact["roomName"] == "质检群A", contact
assert contact["employeeIds"][0] == employee_id, contact
assert contact["content"] == "客户连续发送广告内容", contact
assert int(contact["status"]) == 0, contact

detail = detail_payload["data"]
assert int(detail["contactRecordId"]) == record_id, detail
assert detail["externalUserId"] == "external-room-quality-a", detail
assert detail["msgType"] == "text", detail
PY

api_json PUT "/dashboard/roomQuality/status" "{\"roomQualityId\":$ROOM_QUALITY_ID,\"status\":0}" "$WORK_DIR/room-quality-status.json"
test "$(mysql_scalar "SELECT status FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "0"

api_json GET "/dashboard/roomQuality/info?roomQualityId=$ROOM_QUALITY_ID" "" "$WORK_DIR/room-quality-info-disabled.json"
python3 - "$WORK_DIR/room-quality-info-disabled.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
info = payload["data"]
assert int(info["status"]) == 0, info
assert info["statusText"] == "已停用", info
PY

UPDATE_BODY="$(python3 - "$ROOM_QUALITY_ID" "$ROOM_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

quality_id = int(sys.argv[1])
room_id = int(sys.argv[2])
employee_id = int(sys.argv[3])
body = {
    "roomQualityId": quality_id,
    "name": "群质检验收更新",
    "description": "更新后的质检说明",
    "rule": [{
        "type": "keyword",
        "keyword": "二维码",
        "threshold": 1,
        "showEmployee": [employee_id],
    }],
    "rooms": [
        {"roomId": room_id, "wxChatId": "wr-room-quality-a", "name": "质检群A更新"},
        {"roomId": room_id + 1, "wxChatId": "wr-room-quality-b", "name": "质检群B"},
    ],
    "status": 1,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomQuality/update" "$UPDATE_BODY" "$WORK_DIR/room-quality-update.json"
test "$(mysql_scalar "SELECT name FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "群质检验收更新"
test "$(mysql_scalar "SELECT description FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "更新后的质检说明"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(\`rule\`, '\$[0].keyword')) FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "二维码"
test "$(mysql_scalar "SELECT JSON_LENGTH(rooms) FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "2"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(rooms, '\$[1].name')) FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "质检群B"
test "$(mysql_scalar "SELECT status FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID")" = "1"

api_json GET "/dashboard/roomQuality/index?name=%E6%9B%B4%E6%96%B0&status=1&page=1&perPage=10" "" "$WORK_DIR/room-quality-index-updated.json"
python3 - "$WORK_DIR/room-quality-index-updated.json" "$ROOM_QUALITY_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
quality_id = int(sys.argv[2])
items = payload["data"]["list"]
item = next((row for row in items if int(row["qualityId"]) == quality_id), None)
assert item, items
assert item["name"] == "群质检验收更新", item
assert item["rule"][0]["keyword"] == "二维码", item
assert len(item["rooms"]) == 2, item
assert int(item["contactNum"]) == 1, item
PY

api_json DELETE "/dashboard/roomQuality/destroy" "{\"roomQualityId\":$ROOM_QUALITY_ID}" "$WORK_DIR/room-quality-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_quality WHERE id = $ROOM_QUALITY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_quality_contact WHERE id = $QUALITY_CONTACT_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_qualities' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/roomQuality/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomQuality/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomQuality/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomQuality/status" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomQuality/info" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomQuality/update" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomQuality/showContact" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomQuality/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/roomQuality/contactDetail" "$GO_LOG"

echo "room quality dashboard smoke passed"
