#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-lottery-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18122}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13362}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26422}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-lottery.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-lottery-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001822}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1822}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91822}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91822}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918221}"
LOTTERY_CONTACT_ID="${MOCHAT_SMOKE_LOTTERY_CONTACT_ID:-918222}"
WORK_CONTACT_ID="${MOCHAT_SMOKE_WORK_CONTACT_ID:-918223}"

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

mkdir -p "$FILE_STORAGE_ROOT/lottery"

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
  -tenant-name "抽奖活动验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "抽奖活动管理员" \
  -role-name "抽奖活动超级管理员" \
  -package-code "lottery-standard" \
  -package-name "抽奖活动标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -lotteries 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_lottery_contact_record WHERE lottery_id IN (SELECT id FROM mc_lottery WHERE corp_id = $CORP_ID) OR contact_id = $LOTTERY_CONTACT_ID;
DELETE FROM mc_lottery_contact WHERE id = $LOTTERY_CONTACT_ID OR lottery_id IN (SELECT id FROM mc_lottery WHERE corp_id = $CORP_ID);
DELETE FROM mc_lottery_prize WHERE lottery_id IN (SELECT id FROM mc_lottery WHERE corp_id = $CORP_ID);
DELETE FROM mc_lottery WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag_pivot WHERE contact_id = $WORK_CONTACT_ID OR contact_tag_id = $TAG_ID;
DELETE FROM mc_work_contact WHERE id = $WORK_CONTACT_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '抽奖活动企业', 'ww-lottery', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'lottery-user', $CORP_ID, '抽奖活动员工', '$PHONE', 'avatar/lottery.png', 'avatar/lottery-thumb.png', '抽奖员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-lottery-tag', $CORP_ID, '抽奖客户标签', 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($WORK_CONTACT_ID, $CORP_ID, 'external-lottery-1', '抽奖客户A', '抽奖客户A', 'avatar/lottery-contact.png', 1, 1, 1, 'union-lottery-a', '', '', '', JSON_OBJECT(), 'LOTTERY-A', NOW(), NOW(), NULL);
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
    "name": "抽奖活动验收",
    "description": "抽奖活动说明",
    "type": "roulette",
    "timeType": 2,
    "startTime": "2026-07-01",
    "endTime": "2026-12-31",
    "contactTags": [tag_id],
    "prizeSet": [{"name": "一等奖", "image": "lottery/prize-old.png", "total": 3}],
    "isShow": 1,
    "exchangeSet": {"type": 1, "qrcode": "lottery/exchange-old.png"},
    "drawSet": {"daily": 3, "cover": "lottery/draw-old.png"},
    "winSet": {"limit": 1, "picUrl": "lottery/win-old.png"},
    "corpCard": {"name": "抽奖企业名片", "logo": "lottery/logo-old.png"},
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/lottery/store" "$STORE_BODY" "$WORK_DIR/lottery-store.json"
LOTTERY_ID="$(mysql_scalar "SELECT id FROM mc_lottery WHERE corp_id = $CORP_ID AND name = '抽奖活动验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$LOTTERY_ID"
PRIZE_ID="$(mysql_scalar "SELECT id FROM mc_lottery_prize WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$PRIZE_ID"
test "$(mysql_scalar "SELECT description FROM mc_lottery WHERE id = $LOTTERY_ID")" = "抽奖活动说明"
test "$(mysql_scalar "SELECT time_type FROM mc_lottery WHERE id = $LOTTERY_ID")" = "2"
test "$(mysql_scalar "SELECT JSON_EXTRACT(contact_tags, '\$[0]') FROM mc_lottery WHERE id = $LOTTERY_ID")" = "$TAG_ID"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(prize_set, '\$[0].name')) FROM mc_lottery_prize WHERE id = $PRIZE_ID")" = "一等奖"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(exchange_set, '\$.qrcode')) FROM mc_lottery_prize WHERE id = $PRIZE_ID")" = "lottery/exchange-old.png"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'lotteries' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_lottery_contact
  (id, lottery_id, union_id, contact_id, nickname, avatar, employee_ids, city, source, grade, contact_tags, draw_num, win_num, status, write_off, created_at, updated_at, deleted_at)
VALUES
  ($LOTTERY_CONTACT_ID, $LOTTERY_ID, 'union-lottery-a', $WORK_CONTACT_ID, '抽奖客户A', 'avatar/lottery-contact.png', JSON_ARRAY($EMPLOYEE_ID), '杭州', '海报扫码', 5, JSON_ARRAY(), 2, 1, 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_lottery_contact_record
  (lottery_id, contact_id, prize_id, prize_name, receive_status, receive_qr, receive_type, receive_code, write_off, created_at, updated_at, deleted_at)
VALUES
  ($LOTTERY_ID, $LOTTERY_CONTACT_ID, $PRIZE_ID, '一等奖', 1, 'lottery/receive.png', 1, 'CODE-LOTTERY-1', 0, NOW(), NOW(), NULL);
SQL

api_json GET "/dashboard/lottery/index?name=%E6%8A%BD%E5%A5%96&page=1&perPage=10" "" "$WORK_DIR/lottery-index.json"
api_json GET "/dashboard/lottery/info?lotteryId=$LOTTERY_ID" "" "$WORK_DIR/lottery-info.json"
api_json GET "/dashboard/lottery/show?lotteryId=$LOTTERY_ID" "" "$WORK_DIR/lottery-show.json"
api_json GET "/dashboard/lottery/share?lotteryId=$LOTTERY_ID" "" "$WORK_DIR/lottery-share.json"
api_json GET "/dashboard/lottery/showContact?lotteryId=$LOTTERY_ID&status=1&writeOff=0&name=%E6%8A%BD%E5%A5%96&page=1&perPage=15" "" "$WORK_DIR/lottery-contact.json"

python3 - \
  "$WORK_DIR/lottery-index.json" \
  "$WORK_DIR/lottery-info.json" \
  "$WORK_DIR/lottery-show.json" \
  "$WORK_DIR/lottery-share.json" \
  "$WORK_DIR/lottery-contact.json" \
  "$LOTTERY_ID" "$LOTTERY_CONTACT_ID" "$TAG_ID" "$EMPLOYEE_ID" <<'PY'
import json
import pathlib
import sys

payloads = [json.loads(pathlib.Path(path).read_text(encoding="utf-8")) for path in sys.argv[1:6]]
index_payload, info_payload, show_payload, share_payload, contact_payload = payloads
lottery_id = int(sys.argv[6])
lottery_contact_id = int(sys.argv[7])
tag_id = int(sys.argv[8])
employee_id = int(sys.argv[9])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["lotteryId"]) == lottery_id), None)
assert item, items
assert item["name"] == "抽奖活动验收", item
assert item["description"] == "抽奖活动说明", item
assert int(item["contactNum"]) == 1 and int(item["winNum"]) == 1, item
assert item["shareUrl"] == f"http://operation.example.com/lottery?id={lottery_id}", item

for payload in (info_payload, show_payload, share_payload):
    data = payload["data"]
    assert int(data["lotteryId"]) == lottery_id, data
    assert data["contactTags"][0] == tag_id, data
    assert data["prizeSet"][0]["name"] == "一等奖", data
    assert data["prizeSet"][0]["image"] == "lottery/prize-old.png", data
    assert data["exchangeSet"]["qrcode"] == "lottery/exchange-old.png", data
    assert data["drawSet"]["cover"] == "lottery/draw-old.png", data
    assert data["winSet"]["picUrl"] == "lottery/win-old.png", data
    assert data["corpCard"]["logo"] == "lottery/logo-old.png", data

share = share_payload["data"]
assert share["link"] == f"http://operation.example.com/lottery?id={lottery_id}", share
assert share["qrcodeUrl"] == share["link"], share

contacts = contact_payload["data"]
assert int(contacts["page"]["total"]) == 1, contacts
contact = contacts["list"][0]
assert int(contact["contactRecordId"]) == lottery_contact_id, contact
assert int(contact["lotteryId"]) == lottery_id, contact
assert contact["nickname"] == "抽奖客户A", contact
assert contact["employeeIds"][0] == employee_id, contact
assert int(contact["drawNum"]) == 2 and int(contact["winNum"]) == 1, contact
assert int(contact["status"]) == 1 and int(contact["writeOff"]) == 0, contact
assert contact["prizeName"] == "一等奖", contact
assert contact["receiveQr"] == "lottery/receive.png", contact
PY

api_json GET "/dashboard/lottery/writeOff?lotteryId=$LOTTERY_ID&contactId=$LOTTERY_CONTACT_ID" "" "$WORK_DIR/lottery-write-off.json"
test "$(mysql_scalar "SELECT write_off FROM mc_lottery_contact WHERE id = $LOTTERY_CONTACT_ID")" = "1"
test "$(mysql_scalar "SELECT write_off FROM mc_lottery_contact_record WHERE lottery_id = $LOTTERY_ID AND contact_id = $LOTTERY_CONTACT_ID")" = "1"

api_json PUT "/dashboard/lottery/batchContactTags" "{\"lotteryId\":$LOTTERY_ID,\"contactIds\":[$LOTTERY_CONTACT_ID],\"tagIds\":[$TAG_ID]}" "$WORK_DIR/lottery-batch-tags.json"
test "$(mysql_scalar "SELECT JSON_EXTRACT(contact_tags, '\$[0]') FROM mc_lottery_contact WHERE id = $LOTTERY_CONTACT_ID")" = "$TAG_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_pivot WHERE contact_id = $WORK_CONTACT_ID AND contact_tag_id = $TAG_ID AND deleted_at IS NULL")" = "1"
python3 - "$WORK_DIR/lottery-batch-tags.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["data"]["affected"] == 1, payload
PY

api_json GET "/dashboard/lottery/showContact?lotteryId=$LOTTERY_ID&writeOff=1&page=1&perPage=15" "" "$WORK_DIR/lottery-contact-written-off.json"
python3 - "$WORK_DIR/lottery-contact-written-off.json" "$TAG_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tag_id = int(sys.argv[2])
contacts = payload["data"]
assert int(contacts["page"]["total"]) == 1, contacts
contact = contacts["list"][0]
assert int(contact["writeOff"]) == 1, contact
assert contact["contactTags"][0] == tag_id, contact
PY

UPDATE_BODY="$(python3 - "$LOTTERY_ID" "$TAG_ID" <<'PY'
import json
import sys

lottery_id = int(sys.argv[1])
tag_id = int(sys.argv[2])
body = {
    "lotteryId": lottery_id,
    "name": "抽奖活动验收更新",
    "description": "抽奖活动说明更新",
    "type": "roulette",
    "timeType": 1,
    "startTime": "",
    "endTime": "",
    "contactTags": [tag_id],
    "prizeSet": [{"name": "二等奖", "image": "lottery/prize-new.png", "total": 5}],
    "isShow": 0,
    "exchangeSet": {"type": 2, "qrcode": "lottery/exchange-new.png"},
    "drawSet": {"daily": 5, "cover": "lottery/draw-new.png"},
    "winSet": {"limit": 2, "picUrl": "lottery/win-new.png"},
    "corpCard": {"name": "抽奖企业名片更新", "logo": "lottery/logo-new.png"},
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/lottery/update" "$UPDATE_BODY" "$WORK_DIR/lottery-update.json"
test "$(mysql_scalar "SELECT name FROM mc_lottery WHERE id = $LOTTERY_ID")" = "抽奖活动验收更新"
test "$(mysql_scalar "SELECT description FROM mc_lottery WHERE id = $LOTTERY_ID")" = "抽奖活动说明更新"
test "$(mysql_scalar "SELECT time_type FROM mc_lottery WHERE id = $LOTTERY_ID")" = "1"
test "$(mysql_scalar "SELECT is_show FROM mc_lottery_prize WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(prize_set, '\$[0].name')) FROM mc_lottery_prize WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NULL")" = "二等奖"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(corp_card, '\$.logo')) FROM mc_lottery_prize WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NULL")" = "lottery/logo-new.png"

api_json GET "/dashboard/lottery/info?lotteryId=$LOTTERY_ID" "" "$WORK_DIR/lottery-info-updated.json"
python3 - "$WORK_DIR/lottery-info-updated.json" "$LOTTERY_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
lottery_id = int(sys.argv[2])
data = payload["data"]
assert int(data["lotteryId"]) == lottery_id, data
assert data["name"] == "抽奖活动验收更新", data
assert data["prizeSet"][0]["name"] == "二等奖", data
assert data["corpCard"]["logo"] == "lottery/logo-new.png", data
PY

api_json DELETE "/dashboard/lottery/destroy" "{\"lotteryId\":$LOTTERY_ID}" "$WORK_DIR/lottery-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery WHERE id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery_prize WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery_contact WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_lottery_contact_record WHERE lottery_id = $LOTTERY_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'lotteries' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/lottery/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/lottery/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/lottery/store" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/lottery/showContact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/lottery/show" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/lottery/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/lottery/share" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/lottery/update" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/lottery/info" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/lottery/writeOff" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/lottery/batchContactTags" "$GO_LOG"

echo "lottery dashboard smoke passed"
