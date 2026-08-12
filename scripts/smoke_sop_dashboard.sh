#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-sop-dashboard-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18134}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13374}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26434}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-sop-dashboard.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-sop-dashboard-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001834}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1834}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91834}"
ADMIN_EMPLOYEE_ID="${MOCHAT_SMOKE_ADMIN_EMPLOYEE_ID:-91834}"
MEMBER_EMPLOYEE_ID="${MOCHAT_SMOKE_MEMBER_EMPLOYEE_ID:-91835}"
CONTACT_ID="${MOCHAT_SMOKE_CONTACT_ID:-91836}"
CONTACT_EMPLOYEE_ID="${MOCHAT_SMOKE_CONTACT_EMPLOYEE_ID:-91837}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-91838}"
ROOM_TWO_ID="${MOCHAT_SMOKE_ROOM_TWO_ID:-91839}"

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
grep -q $'0011_sop_rbac\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0031_saas_sop_limits\tbaselined' "$WORK_DIR/migrate-baseline.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id 1 \
  -tenant-name "SOP 验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "SOP 管理员" \
  -role-name "SOP 超级管理员" \
  -package-code "sop-dashboard-standard" \
  -package-name "SOP 标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -contact-sops 20 \
  -room-sops 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_contact_sop_log
WHERE corp_id = $CORP_ID OR contact_sop_id IN (SELECT id FROM mc_contact_sop WHERE corp_id = $CORP_ID);
DELETE FROM mc_room_sop_log
WHERE corp_id = $CORP_ID OR room_sop_id IN (SELECT id FROM mc_room_sop WHERE corp_id = $CORP_ID);
DELETE FROM mc_contact_sop WHERE corp_id = $CORP_ID;
DELETE FROM mc_room_sop WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_employee
WHERE id = $CONTACT_EMPLOYEE_ID OR corp_id = $CORP_ID OR contact_id = $CONTACT_ID OR employee_id IN ($ADMIN_EMPLOYEE_ID, $MEMBER_EMPLOYEE_ID);
DELETE FROM mc_work_contact WHERE id = $CONTACT_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_room WHERE id IN ($ROOM_ID, $ROOM_TWO_ID) OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id IN ($ADMIN_EMPLOYEE_ID, $MEMBER_EMPLOYEE_ID);
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, 'SOP 验收企业', 'ww-sop-dashboard', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($ADMIN_EMPLOYEE_ID, 'sop-admin', $CORP_ID, 'SOP 管理员', '$PHONE', 'avatar/sop-admin.png', 'avatar/sop-admin-thumb.png', 'SOP管理员别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL),
  ($MEMBER_EMPLOYEE_ID, 'sop-member', $CORP_ID, 'SOP 成员', '13800001835', 'avatar/sop-member.png', 'avatar/sop-member-thumb.png', 'SOP成员别名', 2, 1, 0, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ID, $CORP_ID, 'external-sop-a', 'SOP 客户A', 'SOP 客户昵称', 'avatar/sop-contact.png', 2, 1, 2, 'union-sop-a', 'SOP-CUSTOMER-1', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (id, employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_EMPLOYEE_ID, $ADMIN_EMPLOYEE_ID, $CONTACT_ID, 'SOP 客户备注', 'SOP 客户描述', 'SOP 客户企业', JSON_ARRAY('13800001836'), 2, 'sop-admin', '', $CORP_ID, 1, NOW(), NOW(), NOW(), NULL);

INSERT INTO mc_work_room
  (id, corp_id, wx_chat_id, name, owner_id, notice, status, create_time, room_max, room_group_id, created_at, updated_at, deleted_at)
VALUES
  ($ROOM_ID, $CORP_ID, 'wr-sop-a', 'SOP 客户群A', $ADMIN_EMPLOYEE_ID, 'SOP 群公告A', 0, NOW(), 200, 0, NOW(), NOW(), NULL),
  ($ROOM_TWO_ID, $CORP_ID, 'wr-sop-b', 'SOP 客户群B', $MEMBER_EMPLOYEE_ID, 'SOP 群公告B', 0, NOW(), 200, 0, NOW(), NOW(), NULL);
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

grep -q "go migrated route enabled: POST /dashboard/contactSop/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/contactSop/setEmployee" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/contactSop/destroy" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/roomSop/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomSop/setRoom" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomSop/destroy" "$GO_LOG"

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

CONTACT_STORE_BODY="$(python3 - "$ADMIN_EMPLOYEE_ID" "$MEMBER_EMPLOYEE_ID" "$CONTACT_ID" <<'PY'
import json
import sys

admin = int(sys.argv[1])
member = int(sys.argv[2])
contact = int(sys.argv[3])
print(json.dumps({
    "name": "个人SOP验收",
    "setting": [{
        "name": "首触达",
        "content": [{"type": "text", "value": "个人SOP首触达"}],
        "delayMinutes": 0,
    }],
    "employeeIds": [admin, member],
    "contactIds": [contact],
    "state": 1,
}, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/contactSop/store" "$CONTACT_STORE_BODY" "$WORK_DIR/contact-sop-store.json"
CONTACT_SOP_ID="$(json_id_from_response "$WORK_DIR/contact-sop-store.json")"
test -n "$CONTACT_SOP_ID"
test "$(mysql_scalar "SELECT name, state, JSON_LENGTH(employee_ids), JSON_LENGTH(contact_ids), JSON_UNQUOTE(JSON_EXTRACT(setting, '\$[0].name')) FROM mc_contact_sop WHERE id = $CONTACT_SOP_ID AND corp_id = $CORP_ID")" = $'个人SOP验收\t1\t2\t1\t首触达'
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'contact_sops' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/contactSop/index?page=1&perPage=5" "" "$WORK_DIR/contact-sop-index.json"
api_json GET "/dashboard/contactSop/info?contactSopId=$CONTACT_SOP_ID" "" "$WORK_DIR/contact-sop-info.json"
python3 - "$WORK_DIR/contact-sop-index.json" "$WORK_DIR/contact-sop-info.json" "$CONTACT_SOP_ID" "$ADMIN_EMPLOYEE_ID" "$MEMBER_EMPLOYEE_ID" "$CONTACT_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
info = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))["data"]
sop_id = int(sys.argv[3])
admin = int(sys.argv[4])
member = int(sys.argv[5])
contact = int(sys.argv[6])
items = index_payload.get("list") or index_payload.get("items") or []
if not any(int(item.get("id", 0)) == sop_id for item in items):
    raise SystemExit(json.dumps(index_payload, ensure_ascii=False))
if info["id"] != sop_id or info["name"] != "个人SOP验收":
    raise SystemExit(json.dumps(info, ensure_ascii=False))
if info["employeeIds"] != [admin, member] or info["contactIds"] != [contact]:
    raise SystemExit(json.dumps(info, ensure_ascii=False))
if info["employeeNum"] != 2 or info["contactNum"] != 1:
    raise SystemExit(json.dumps(info, ensure_ascii=False))
if "个人SOP首触达" not in json.dumps(info["setting"], ensure_ascii=False):
    raise SystemExit(json.dumps(info, ensure_ascii=False))
PY

api_json PUT "/dashboard/contactSop/setEmployee" "{\"contactSopId\":$CONTACT_SOP_ID,\"employeeIds\":\"$MEMBER_EMPLOYEE_ID\"}" "$WORK_DIR/contact-sop-set-employee.json"
test "$(mysql_scalar "SELECT JSON_LENGTH(employee_ids), JSON_EXTRACT(employee_ids, '\$[0]') FROM mc_contact_sop WHERE id = $CONTACT_SOP_ID")" = $'1\t'"$MEMBER_EMPLOYEE_ID"

api_json PUT "/dashboard/contactSop/state" "{\"id\":$CONTACT_SOP_ID,\"status\":0}" "$WORK_DIR/contact-sop-state.json"
test "$(mysql_scalar "SELECT state FROM mc_contact_sop WHERE id = $CONTACT_SOP_ID")" = "0"

CONTACT_UPDATE_BODY="$(python3 - "$CONTACT_SOP_ID" "$ADMIN_EMPLOYEE_ID" "$MEMBER_EMPLOYEE_ID" "$CONTACT_ID" <<'PY'
import json
import sys

sop_id = int(sys.argv[1])
admin = int(sys.argv[2])
member = int(sys.argv[3])
contact = int(sys.argv[4])
print(json.dumps({
    "id": sop_id,
    "title": "个人SOP验收-已更新",
    "rules": [{
        "name": "复访",
        "content": [{"type": "text", "value": "个人SOP复访"}],
        "delayMinutes": 30,
    }],
    "employee": f"{admin},{member}",
    "contact": str(contact),
    "state": 1,
}, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/contactSop/update" "$CONTACT_UPDATE_BODY" "$WORK_DIR/contact-sop-update.json"
test "$(mysql_scalar "SELECT name, state, JSON_LENGTH(employee_ids), JSON_LENGTH(contact_ids), JSON_UNQUOTE(JSON_EXTRACT(setting, '\$[0].name')) FROM mc_contact_sop WHERE id = $CONTACT_SOP_ID")" = $'个人SOP验收-已更新\t1\t2\t1\t复访'

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_contact_sop_log
  (corp_id, contact_sop_id, employee, contact, task, created_at)
VALUES
  ($CORP_ID, $CONTACT_SOP_ID, 'sop-admin', 'external-sop-a', JSON_OBJECT('name', '复访', 'content', JSON_ARRAY(JSON_OBJECT('type', 'text', 'value', '个人SOP复访'))), NOW());
SQL
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop_log WHERE corp_id = $CORP_ID AND contact_sop_id = $CONTACT_SOP_ID")" = "1"

api_json DELETE "/dashboard/contactSop/destroy" "{\"contactSopId\":$CONTACT_SOP_ID}" "$WORK_DIR/contact-sop-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop WHERE corp_id = $CORP_ID AND id = $CONTACT_SOP_ID")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_sop_log WHERE corp_id = $CORP_ID AND contact_sop_id = $CONTACT_SOP_ID")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'contact_sops' AND deleted_at IS NULL")" = "0"

ROOM_STORE_BODY="$(python3 - "$ROOM_ID" <<'PY'
import json
import sys

room = int(sys.argv[1])
print(json.dumps({
    "name": "群SOP验收",
    "setting": [{
        "name": "入群提醒",
        "content": [{"type": "text", "value": "群SOP入群提醒"}],
        "targetAnchor": "room_join",
        "delayMinutes": 0,
    }],
    "roomIds": [room],
    "state": 1,
}, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/roomSop/store" "$ROOM_STORE_BODY" "$WORK_DIR/room-sop-store.json"
ROOM_SOP_ID="$(json_id_from_response "$WORK_DIR/room-sop-store.json")"
test -n "$ROOM_SOP_ID"
test "$(mysql_scalar "SELECT name, state, JSON_LENGTH(room_ids), JSON_UNQUOTE(JSON_EXTRACT(setting, '\$[0].name')) FROM mc_room_sop WHERE id = $ROOM_SOP_ID AND corp_id = $CORP_ID")" = $'群SOP验收\t1\t1\t入群提醒'
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_sops' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/roomSop/index?page=1&perPage=5" "" "$WORK_DIR/room-sop-index.json"
api_json GET "/dashboard/roomSop/info?roomSopId=$ROOM_SOP_ID" "" "$WORK_DIR/room-sop-info.json"
python3 - "$WORK_DIR/room-sop-index.json" "$WORK_DIR/room-sop-info.json" "$ROOM_SOP_ID" "$ROOM_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
info = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))["data"]
sop_id = int(sys.argv[3])
room = int(sys.argv[4])
items = index_payload.get("list") or index_payload.get("items") or []
if not any(int(item.get("id", 0)) == sop_id for item in items):
    raise SystemExit(json.dumps(index_payload, ensure_ascii=False))
if info["id"] != sop_id or info["name"] != "群SOP验收":
    raise SystemExit(json.dumps(info, ensure_ascii=False))
if info["roomIds"] != [room] or info["roomNum"] != 1:
    raise SystemExit(json.dumps(info, ensure_ascii=False))
if "群SOP入群提醒" not in json.dumps(info["setting"], ensure_ascii=False):
    raise SystemExit(json.dumps(info, ensure_ascii=False))
PY

api_json PUT "/dashboard/roomSop/setRoom" "{\"roomSopId\":$ROOM_SOP_ID,\"roomIds\":\"$ROOM_ID,$ROOM_TWO_ID\"}" "$WORK_DIR/room-sop-set-room.json"
test "$(mysql_scalar "SELECT JSON_LENGTH(room_ids), JSON_EXTRACT(room_ids, '\$[1]') FROM mc_room_sop WHERE id = $ROOM_SOP_ID")" = $'2\t'"$ROOM_TWO_ID"

api_json PUT "/dashboard/roomSop/state" "{\"id\":$ROOM_SOP_ID,\"status\":0}" "$WORK_DIR/room-sop-state.json"
test "$(mysql_scalar "SELECT state FROM mc_room_sop WHERE id = $ROOM_SOP_ID")" = "0"

ROOM_UPDATE_BODY="$(python3 - "$ROOM_SOP_ID" "$ROOM_TWO_ID" <<'PY'
import json
import sys

sop_id = int(sys.argv[1])
room = int(sys.argv[2])
print(json.dumps({
    "id": sop_id,
    "sopName": "群SOP验收-已更新",
    "settings": [{
        "name": "群复访",
        "content": [{"type": "text", "value": "群SOP复访"}],
        "delayHours": 1,
    }],
    "room": [room],
    "state": 1,
}, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/roomSop/update" "$ROOM_UPDATE_BODY" "$WORK_DIR/room-sop-update.json"
test "$(mysql_scalar "SELECT name, state, JSON_LENGTH(room_ids), JSON_EXTRACT(room_ids, '\$[0]'), JSON_UNQUOTE(JSON_EXTRACT(setting, '\$[0].name')) FROM mc_room_sop WHERE id = $ROOM_SOP_ID")" = $'群SOP验收-已更新\t1\t1\t'"$ROOM_TWO_ID"$'\t群复访'

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_room_sop_log
  (corp_id, room_sop_id, room_id, state, employee, contact, task, created_at, updated_at)
VALUES
  ($CORP_ID, $ROOM_SOP_ID, $ROOM_TWO_ID, 0, 'sop-member', 'external-sop-a', JSON_OBJECT('name', '群复访', 'content', JSON_ARRAY(JSON_OBJECT('type', 'text', 'value', '群SOP复访'))), NOW(), NOW());
SQL
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = $CORP_ID AND room_sop_id = $ROOM_SOP_ID")" = "1"

api_json DELETE "/dashboard/roomSop/destroy" "{\"roomSopId\":$ROOM_SOP_ID}" "$WORK_DIR/room-sop-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_sop WHERE corp_id = $CORP_ID AND id = $ROOM_SOP_ID")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_sop_log WHERE corp_id = $CORP_ID AND room_sop_id = $ROOM_SOP_ID")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'room_sops' AND deleted_at IS NULL")" = "0"

echo "SOP dashboard standalone smoke passed"
