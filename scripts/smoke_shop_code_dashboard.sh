#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-shop-code-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18120}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13360}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26420}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-shop-code.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-shop-code-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001820}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1820}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91820}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91820}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918201}"

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

mkdir -p "$FILE_STORAGE_ROOT/shopCode"

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
  -tenant-name "门店活码验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "门店活码管理员" \
  -role-name "门店活码超级管理员" \
  -package-code "shop-code-standard" \
  -package-name "门店活码标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -shop-codes 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_shop_code_record WHERE corp_id = $CORP_ID OR shop_id IN (SELECT id FROM mc_shop_code WHERE corp_id = $CORP_ID);
DELETE FROM mc_shop_code_page WHERE corp_id = $CORP_ID;
DELETE FROM mc_shop_code WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '门店活码企业', 'ww-shop-code', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'shop-code-user', $CORP_ID, '门店活码员工', '$PHONE', 'avatar/shop-code.png', 'avatar/shop-code-thumb.png', '门店员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-shop-code-tag', $CORP_ID, '门店客户标签', 1, 0, NOW(), NOW(), NULL);
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

STORE_BODY="$(python3 - "$EMPLOYEE_ID" <<'PY'
import json
import sys

employee_id = int(sys.argv[1])
body = {
    "name": "门店活码验收",
    "type": 1,
    "employee": [{"id": employee_id, "name": "门店活码员工"}],
    "employeeQrcode": [{"url": "shopCode/employee-old.png", "name": "店主二维码"}],
    "qwCode": [{"roomQrcodeUrl": "shopCode/room-old.png", "roomName": "门店群"}],
    "searchKeyword": "西湖门店",
    "address": "杭州市西湖区测试路1号",
    "country": "中国",
    "province": "浙江省",
    "city": "杭州市",
    "district": "西湖区",
    "lat": "30.123",
    "lng": "120.456",
    "status": 1,
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/shopCode/store" "$STORE_BODY" "$WORK_DIR/shop-store.json"
SHOP_ID="$(mysql_scalar "SELECT id FROM mc_shop_code WHERE corp_id = $CORP_ID AND name = '门店活码验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$SHOP_ID"
test "$(mysql_scalar "SELECT type FROM mc_shop_code WHERE id = $SHOP_ID")" = "1"
test "$(mysql_scalar "SELECT status FROM mc_shop_code WHERE id = $SHOP_ID")" = "1"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(employee, '\$[0].name')) FROM mc_shop_code WHERE id = $SHOP_ID")" = "门店活码员工"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(employee_qrcode, '\$[0].url')) FROM mc_shop_code WHERE id = $SHOP_ID")" = "shopCode/employee-old.png"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(qw_code, '\$[0].roomQrcodeUrl')) FROM mc_shop_code WHERE id = $SHOP_ID")" = "shopCode/room-old.png"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'shop_codes' AND deleted_at IS NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_shop_code_record
  (type, corp_id, shop_id, created_at, updated_at, deleted_at)
VALUES
  (1, $CORP_ID, $SHOP_ID, '2026-07-01 09:00:00', NOW(), NULL),
  (1, $CORP_ID, $SHOP_ID, '2026-07-02 09:00:00', NOW(), NULL);
SQL

api_json GET "/dashboard/shopCode/index?type=1&status=1&name=%E8%A5%BF%E6%B9%96&page=1&perPage=10" "" "$WORK_DIR/shop-index.json"
api_json GET "/dashboard/shopCode/info?shopCodeId=$SHOP_ID" "" "$WORK_DIR/shop-info.json"
api_json GET "/dashboard/shopCode/location?id=$SHOP_ID" "" "$WORK_DIR/shop-location.json"
api_json GET "/dashboard/shopCode/searchCity?keyword=%E6%9D%AD%E5%B7%9E" "" "$WORK_DIR/shop-search-city.json"
api_json GET "/dashboard/shopCode/addressKeyWordList?keyword=%E8%A5%BF%E6%B9%96&city=%E6%9D%AD%E5%B7%9E" "" "$WORK_DIR/shop-address.json"
api_json GET "/dashboard/shopCode/share?id=$SHOP_ID" "" "$WORK_DIR/shop-share.json"
api_json GET "/dashboard/shopCode/show?type=1" "" "$WORK_DIR/shop-show.json"
api_json GET "/dashboard/shopCode/showContact?shopCodeId=$SHOP_ID&page=1&perPage=15" "" "$WORK_DIR/shop-contact.json"
api_json GET "/dashboard/shopCode/showShop?type=1&page=1&perPage=15" "" "$WORK_DIR/shop-shop.json"

python3 - \
  "$WORK_DIR/shop-index.json" \
  "$WORK_DIR/shop-info.json" \
  "$WORK_DIR/shop-location.json" \
  "$WORK_DIR/shop-search-city.json" \
  "$WORK_DIR/shop-address.json" \
  "$WORK_DIR/shop-share.json" \
  "$WORK_DIR/shop-show.json" \
  "$WORK_DIR/shop-contact.json" \
  "$WORK_DIR/shop-shop.json" \
  "$SHOP_ID" <<'PY'
import json
import pathlib
import sys
from urllib.parse import unquote

payloads = [json.loads(pathlib.Path(path).read_text(encoding="utf-8")) for path in sys.argv[1:10]]
index_payload, info_payload, location_payload, city_payload, address_payload, share_payload, show_payload, contact_payload, shop_payload = payloads
shop_id = int(sys.argv[10])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["shopCodeId"]) == shop_id), None)
assert item, items
assert item["name"] == "门店活码验收", item
assert item["searchKeyword"] == "西湖门店", item
assert item["city"] == "杭州市", item
assert item["employee"][0]["name"] == "门店活码员工", item
assert item["employeeQrcode"][0]["url"] == "shopCode/employee-old.png", item
assert item["qwCode"][0]["roomQrcodeUrl"] == "shopCode/room-old.png", item

info = info_payload["data"]
assert int(info["shopCodeId"]) == shop_id, info
assert info["address"] == "杭州市西湖区测试路1号", info
assert info["lat"] == "30.123" and info["lng"] == "120.456", info

location = location_payload["data"]
assert location["city"] == "杭州市", location
assert location["location"]["lat"] == "30.123", location

cities = city_payload["data"]
assert any(row["city"] == "杭州市" for row in cities), cities
addresses = address_payload["data"]
assert any(row["searchKeyword"] == "西湖门店" and row["address"] == "杭州市西湖区测试路1号" for row in addresses), addresses

share = share_payload["data"]
assert share["url"].startswith("http://operation.example.com/auth/shopCode?id="), share
assert f"id={shop_id}" in share["url"], share
assert "/shopCode" in unquote(share["target"]), share

show = show_payload["data"]
assert int(show["shopTotal"]) == 1, show
assert int(show["openTotal"]) == 1, show
assert int(show["closeTotal"]) == 0, show
assert int(show["recordTotal"]) == 2, show

contacts = contact_payload["data"]
assert int(contacts["page"]["total"]) == 2, contacts
assert {int(row["shopId"]) for row in contacts["list"]} == {shop_id}, contacts

shops = shop_payload["data"]
shop = next((row for row in shops["list"] if int(row["shopCodeId"]) == shop_id), None)
assert shop, shops
assert int(shop["recordTotal"]) == 2, shop
PY

PAGE_BODY='{"type":1,"title":"门店活码页面设置","showType":2,"default":{"guide":"扫码添加店主","logo":"shopCode/page-logo.png"},"poster":"shopCode/page-poster.png","autoPass":1}'
api_json POST "/dashboard/shopCode/pageSet" "$PAGE_BODY" "$WORK_DIR/shop-page-set.json"
PAGE_ID="$(mysql_scalar "SELECT id FROM mc_shop_code_page WHERE corp_id = $CORP_ID AND type = 1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$PAGE_ID"
test "$(mysql_scalar "SELECT title FROM mc_shop_code_page WHERE id = $PAGE_ID")" = "门店活码页面设置"
test "$(mysql_scalar "SELECT show_type FROM mc_shop_code_page WHERE id = $PAGE_ID")" = "2"
test "$(mysql_scalar "SELECT autoPass FROM mc_shop_code_page WHERE id = $PAGE_ID")" = "1"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(\`default\`, '\$.logo')) FROM mc_shop_code_page WHERE id = $PAGE_ID")" = "shopCode/page-logo.png"

api_json GET "/dashboard/shopCode/pageInfo?type=1" "" "$WORK_DIR/shop-page-info.json"
python3 - "$WORK_DIR/shop-page-info.json" "$PAGE_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
page_id = int(sys.argv[2])
data = payload["data"]
assert int(data["id"]) == page_id, data
assert data["title"] == "门店活码页面设置", data
assert int(data["showType"]) == 2, data
assert data["default"]["guide"] == "扫码添加店主", data
assert data["poster"] == "shopCode/page-poster.png", data
assert int(data["autoPass"]) == 1, data
PY

api_json POST "/dashboard/shopCode/updateEmployee" "{\"shopCodeId\":$SHOP_ID,\"employee\":[{\"id\":$EMPLOYEE_ID,\"name\":\"门店活码员工更新\"}]}" "$WORK_DIR/shop-update-employee.json"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(employee, '\$[0].name')) FROM mc_shop_code WHERE id = $SHOP_ID")" = "门店活码员工更新"

api_json POST "/dashboard/shopCode/updateQrcode" "{\"shopCodeId\":$SHOP_ID,\"qrcode\":[{\"url\":\"shopCode/employee-new.png\",\"name\":\"新店主二维码\"}],\"qwCode\":[{\"roomQrcodeUrl\":\"shopCode/room-new.png\",\"roomName\":\"新门店群\"}]}" "$WORK_DIR/shop-update-qrcode.json"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(employee_qrcode, '\$[0].url')) FROM mc_shop_code WHERE id = $SHOP_ID")" = "shopCode/employee-new.png"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(qw_code, '\$[0].roomQrcodeUrl')) FROM mc_shop_code WHERE id = $SHOP_ID")" = "shopCode/room-new.png"

UPDATE_BODY='{"shopCodeId":'"$SHOP_ID"',"name":"门店活码验收更新","searchKeyword":"滨江门店","address":"杭州市滨江区测试路2号","province":"浙江省","city":"杭州市","district":"滨江区","lat":"30.222","lng":"120.333"}'
api_json PUT "/dashboard/shopCode/update" "$UPDATE_BODY" "$WORK_DIR/shop-update.json"
test "$(mysql_scalar "SELECT name FROM mc_shop_code WHERE id = $SHOP_ID")" = "门店活码验收更新"
test "$(mysql_scalar "SELECT search_keyword FROM mc_shop_code WHERE id = $SHOP_ID")" = "滨江门店"
test "$(mysql_scalar "SELECT district FROM mc_shop_code WHERE id = $SHOP_ID")" = "滨江区"

api_json PUT "/dashboard/shopCode/status" "{\"shopCodeId\":$SHOP_ID,\"status\":0}" "$WORK_DIR/shop-status.json"
test "$(mysql_scalar "SELECT status FROM mc_shop_code WHERE id = $SHOP_ID")" = "0"
api_json GET "/dashboard/shopCode/show?type=1" "" "$WORK_DIR/shop-show-closed.json"
python3 - "$WORK_DIR/shop-show-closed.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
data = payload["data"]
assert int(data["shopTotal"]) == 1, data
assert int(data["openTotal"]) == 0, data
assert int(data["closeTotal"]) == 1, data
assert int(data["recordTotal"]) == 2, data
PY

api_json PUT "/dashboard/shopCode/batchContactTags" "{\"shopCodeId\":$SHOP_ID,\"tagIds\":[$TAG_ID]}" "$WORK_DIR/shop-batch-tags.json"
python3 - "$WORK_DIR/shop-batch-tags.json" "$TAG_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
tag_id = int(sys.argv[2])
data = payload["data"]
assert data["applied"] == 0, data
assert tag_id in [int(v) for v in data["tagIds"]], data
PY

api_json DELETE "/dashboard/shopCode/destroy" "{\"shopCodeId\":$SHOP_ID}" "$WORK_DIR/shop-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_shop_code WHERE id = $SHOP_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'shop_codes' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/shopCode/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/location" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/addressKeyWordList" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/shopCode/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/shopCode/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/shopCode/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/info" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/shopCode/status" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/searchCity" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/share" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/pageInfo" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/shopCode/pageSet" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/showContact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/shopCode/showShop" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/shopCode/updateEmployee" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/shopCode/updateQrcode" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/shopCode/batchContactTags" "$GO_LOG"

echo "shop code dashboard smoke passed"
