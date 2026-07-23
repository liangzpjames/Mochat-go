#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-radar-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18121}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13361}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26421}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-radar.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-radar-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001821}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1821}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91821}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91821}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918211}"
CONTACT_ID="${MOCHAT_SMOKE_CONTACT_ID:-918212}"

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

mkdir -p "$FILE_STORAGE_ROOT/radar"

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
  -tenant-name "互动雷达验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "互动雷达管理员" \
  -role-name "互动雷达超级管理员" \
  -package-code "radar-standard" \
  -package-name "互动雷达标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -radars 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_radar_record WHERE corp_id = $CORP_ID OR radar_id IN (SELECT id FROM mc_radar WHERE corp_id = $CORP_ID);
DELETE FROM mc_radar_channel_link WHERE corp_id = $CORP_ID OR radar_id IN (SELECT id FROM mc_radar WHERE corp_id = $CORP_ID);
DELETE FROM mc_radar_channel WHERE corp_id = $CORP_ID;
DELETE FROM mc_radar WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact WHERE id = $CONTACT_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '互动雷达企业', 'ww-radar', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'radar-user', $CORP_ID, '互动雷达员工', '$PHONE', 'avatar/radar.png', 'avatar/radar-thumb.png', '雷达员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-radar-tag', $CORP_ID, '雷达客户标签', 1, 0, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ID, $CORP_ID, 'external-radar-1', '雷达客户A', '雷达客户A', 'avatar/radar-contact.png', 1, 1, 1, 'union-radar-a', '', '', '', JSON_OBJECT(), 'RADAR-A', NOW(), NOW(), NULL);
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
    "type": 1,
    "title": "互动雷达验收",
    "link": "https://example.com/radar-a",
    "linkTitle": "雷达链接标题",
    "linkDescription": "雷达链接摘要",
    "linkCover": "radar/cover-old.png",
    "pdfName": "雷达PDF",
    "pdf": "radar/file-old.pdf",
    "articleType": 2,
    "article": {"content": "互动雷达文章内容"},
    "employeeCard": 1,
    "actionNotice": 1,
    "dynamicNotice": 1,
    "contactTags": [{"id": tag_id, "name": "雷达客户标签"}],
    "tagStatus": 1,
    "contactGrade": [{"score": 5, "label": "高意向"}],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/radar/store" "$STORE_BODY" "$WORK_DIR/radar-store.json"
RADAR_ID="$(mysql_scalar "SELECT id FROM mc_radar WHERE corp_id = $CORP_ID AND title = '互动雷达验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$RADAR_ID"
test "$(mysql_scalar "SELECT type FROM mc_radar WHERE id = $RADAR_ID")" = "1"
test "$(mysql_scalar "SELECT link FROM mc_radar WHERE id = $RADAR_ID")" = "https://example.com/radar-a"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(article, '\$.content')) FROM mc_radar WHERE id = $RADAR_ID")" = "互动雷达文章内容"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(contact_tags, '\$[0].name')) FROM mc_radar WHERE id = $RADAR_ID")" = "雷达客户标签"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(contact_grade, '\$[0].label')) FROM mc_radar WHERE id = $RADAR_ID")" = "高意向"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'radars' AND deleted_at IS NULL")" = "1"

api_json POST "/dashboard/radar/storeChannel" '{"name":"雷达渠道A"}' "$WORK_DIR/channel-store.json"
CHANNEL_ID="$(mysql_scalar "SELECT id FROM mc_radar_channel WHERE corp_id = $CORP_ID AND name = '雷达渠道A' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$CHANNEL_ID"

api_json POST "/dashboard/radar/storeChannelLink" "{\"radarId\":$RADAR_ID,\"channelId\":$CHANNEL_ID,\"employeeId\":$EMPLOYEE_ID,\"type\":1}" "$WORK_DIR/channel-link-store.json"
LINK_ID="$(mysql_scalar "SELECT id FROM mc_radar_channel_link WHERE radar_id = $RADAR_ID AND channel_id = $CHANNEL_ID AND employee_id = $EMPLOYEE_ID AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$LINK_ID"
LINK_URL="$(mysql_scalar "SELECT link FROM mc_radar_channel_link WHERE id = $LINK_ID")"
case "$LINK_URL" in
  http://operation.example.com/auth/radar?id="$RADAR_ID"*) ;;
  *)
    echo "unexpected radar channel link: $LINK_URL" >&2
    exit 1
    ;;
esac
case "$LINK_URL" in
  *employee_id%3D"$EMPLOYEE_ID"*target_id%3D"$LINK_ID"*) ;;
  *)
    echo "radar channel link missing encoded employee/target id: $LINK_URL" >&2
    exit 1
    ;;
esac

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
UPDATE mc_radar_channel_link
SET click_num = 2, click_person_num = 1, updated_at = NOW()
WHERE id = $LINK_ID;

INSERT INTO mc_radar_record
  (radar_id, channel_id, type, union_id, nickname, avatar, contact_id, employee_id, content, corp_id, created_at, updated_at, deleted_at)
VALUES
  ($RADAR_ID, $CHANNEL_ID, 1, 'union-radar-a', '雷达客户A', 'avatar/radar-a.png', $CONTACT_ID, $EMPLOYEE_ID, '第一次点击', $CORP_ID, '2026-07-01 09:00:00', NOW(), NULL),
  ($RADAR_ID, $CHANNEL_ID, 1, 'union-radar-a', '雷达客户A', 'avatar/radar-a.png', $CONTACT_ID, $EMPLOYEE_ID, '第二次点击', $CORP_ID, '2026-07-02 09:00:00', NOW(), NULL);
SQL

api_json GET "/dashboard/radar/index?type=1&title=%E4%BA%92%E5%8A%A8&page=1&perPage=10" "" "$WORK_DIR/radar-index.json"
api_json GET "/dashboard/radar/info?radarId=$RADAR_ID" "" "$WORK_DIR/radar-info.json"
api_json GET "/dashboard/radar/show?radarId=$RADAR_ID&type=1" "" "$WORK_DIR/radar-show.json"
api_json GET "/dashboard/radar/indexChannel?name=%E9%9B%B7%E8%BE%BE&page=1&perPage=10" "" "$WORK_DIR/channel-index.json"
api_json GET "/dashboard/radar/indexChannelLink?radarId=$RADAR_ID&page=1&perPage=10" "" "$WORK_DIR/channel-link-index.json"
api_json GET "/dashboard/radar/showContact?radarId=$RADAR_ID&channelId=$CHANNEL_ID&page=1&perPage=15" "" "$WORK_DIR/radar-contact.json"
api_json GET "/dashboard/radar/showChannel?radarId=$RADAR_ID&page=1&perPage=15" "" "$WORK_DIR/radar-channel.json"
api_json GET "/dashboard/radar/radarArticle?link=https%3A%2F%2Fexample.com%2Fradar-a&title=%E9%9B%B7%E8%BE%BE%E6%96%87%E7%AB%A0&description=%E9%9B%B7%E8%BE%BE%E6%91%98%E8%A6%81&cover=radar%2Fcover-old.png" "" "$WORK_DIR/radar-article.json"

python3 - \
  "$WORK_DIR/radar-index.json" \
  "$WORK_DIR/radar-info.json" \
  "$WORK_DIR/radar-show.json" \
  "$WORK_DIR/channel-index.json" \
  "$WORK_DIR/channel-link-index.json" \
  "$WORK_DIR/radar-contact.json" \
  "$WORK_DIR/radar-channel.json" \
  "$WORK_DIR/radar-article.json" \
  "$RADAR_ID" "$CHANNEL_ID" "$LINK_ID" "$EMPLOYEE_ID" <<'PY'
import json
import pathlib
import sys
from urllib.parse import unquote

payloads = [json.loads(pathlib.Path(path).read_text(encoding="utf-8")) for path in sys.argv[1:9]]
index_payload, info_payload, show_payload, channel_payload, link_payload, contact_payload, channel_stats_payload, article_payload = payloads
radar_id = int(sys.argv[9])
channel_id = int(sys.argv[10])
link_id = int(sys.argv[11])
employee_id = int(sys.argv[12])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["radarId"]) == radar_id), None)
assert item, items
assert item["title"] == "互动雷达验收", item
assert item["linkTitle"] == "雷达链接标题", item
assert item["linkCover"] == "radar/cover-old.png", item
assert int(item["clickNum"]) == 2, item
assert int(item["clickPersonNum"]) == 1, item
assert int(item["channelNum"]) == 1, item

info = info_payload["data"]
assert int(info["radarId"]) == radar_id, info
assert info["article"]["content"] == "互动雷达文章内容", info
assert info["contactTags"][0]["name"] == "雷达客户标签", info
assert info["contactGrade"][0]["label"] == "高意向", info

show = show_payload["data"]
assert int(show["radarId"]) == radar_id, show
assert int(show["clickNum"]) == 2, show
assert int(show["clickPersonNum"]) == 1, show
assert int(show["channelNum"]) == 1, show

channels = channel_payload["data"]["list"]
channel = next((row for row in channels if int(row["channelId"]) == channel_id), None)
assert channel and channel["name"] == "雷达渠道A", channels

links = link_payload["data"]["list"]
link = next((row for row in links if int(row["linkId"]) == link_id), None)
assert link, links
assert int(link["radarId"]) == radar_id, link
assert int(link["channelId"]) == channel_id, link
assert int(link["employeeId"]) == employee_id, link
assert link["employee"] == "互动雷达员工", link
assert int(link["clickNum"]) == 2 and int(link["clickPersonNum"]) == 1, link
assert link["link"].startswith("http://operation.example.com/auth/radar?id="), link
assert f"target_id={link_id}" in unquote(link["link"]), link

contacts = contact_payload["data"]
assert int(contacts["page"]["total"]) == 1, contacts
contact = contacts["list"][0]
assert contact["nickname"] == "雷达客户A", contact
assert contact["content"] == "第二次点击", contact
assert int(contact["clickNum"]) == 2, contact
assert [row["content"] for row in contact["clickInfo"]] == ["第二次点击", "第一次点击"], contact

channel_stats = channel_stats_payload["data"]
assert int(channel_stats["page"]["total"]) == 1, channel_stats
stat = channel_stats["list"][0]
assert int(stat["linkId"]) == link_id, stat
assert int(stat["clickNum"]) == 2 and int(stat["clickPersonNum"]) == 1, stat

article = article_payload["data"]
assert article["title"] == "雷达文章", article
assert article["link"] == "https://example.com/radar-a", article
assert article["description"] == "雷达摘要", article
assert article["cover"] == "radar/cover-old.png", article
PY

UPDATE_BODY="$(python3 - "$RADAR_ID" "$TAG_ID" <<'PY'
import json
import sys

radar_id = int(sys.argv[1])
tag_id = int(sys.argv[2])
body = {
    "radarId": radar_id,
    "type": 2,
    "title": "互动雷达验收更新",
    "link": "https://example.com/radar-b",
    "linkTitle": "雷达链接标题更新",
    "linkDescription": "雷达链接摘要更新",
    "linkCover": "radar/cover-new.png",
    "pdfName": "雷达PDF更新",
    "pdf": "radar/file-new.pdf",
    "articleType": 1,
    "article": {"content": "互动雷达文章内容更新"},
    "employeeCard": 0,
    "actionNotice": 0,
    "dynamicNotice": 0,
    "contactTags": [{"id": tag_id, "name": "雷达客户标签更新"}],
    "tagStatus": 0,
    "contactGrade": [{"score": 3, "label": "中意向"}],
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/radar/update" "$UPDATE_BODY" "$WORK_DIR/radar-update.json"
test "$(mysql_scalar "SELECT title FROM mc_radar WHERE id = $RADAR_ID")" = "互动雷达验收更新"
test "$(mysql_scalar "SELECT type FROM mc_radar WHERE id = $RADAR_ID")" = "2"
test "$(mysql_scalar "SELECT link_cover FROM mc_radar WHERE id = $RADAR_ID")" = "radar/cover-new.png"
test "$(mysql_scalar "SELECT pdf FROM mc_radar WHERE id = $RADAR_ID")" = "radar/file-new.pdf"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(article, '\$.content')) FROM mc_radar WHERE id = $RADAR_ID")" = "互动雷达文章内容更新"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(contact_tags, '\$[0].name')) FROM mc_radar WHERE id = $RADAR_ID")" = "雷达客户标签更新"

api_json GET "/dashboard/radar/info?radarId=$RADAR_ID" "" "$WORK_DIR/radar-info-updated.json"
python3 - "$WORK_DIR/radar-info-updated.json" "$RADAR_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
radar_id = int(sys.argv[2])
data = payload["data"]
assert int(data["radarId"]) == radar_id, data
assert data["title"] == "互动雷达验收更新", data
assert data["article"]["content"] == "互动雷达文章内容更新", data
assert data["contactTags"][0]["name"] == "雷达客户标签更新", data
PY

api_json DELETE "/dashboard/radar/destroy" "{\"radarId\":$RADAR_ID}" "$WORK_DIR/radar-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_radar WHERE id = $RADAR_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'radars' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/radar/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/radar/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/radar/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/radar/destroy" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/info" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/show" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/radar/storeChannel" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/indexChannel" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/radar/storeChannelLink" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/indexChannelLink" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/showContact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/showChannel" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/radar/radarArticle" "$GO_LOG"

echo "radar dashboard smoke passed"
