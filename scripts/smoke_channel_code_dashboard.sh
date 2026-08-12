#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-channel-code-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18119}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13359}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26419}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19099}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-channel-code.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-channel-code-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001819}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1819}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91819}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91819}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-918190}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918191}"
CONTACT_ONE_ID="${MOCHAT_SMOKE_CONTACT_ONE_ID:-918192}"
CONTACT_TWO_ID="${MOCHAT_SMOKE_CONTACT_TWO_ID:-918193}"
LOG_OPERATION_ID=0

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  if [ -n "$WECOM_PID" ] && kill -0 "$WECOM_PID" 2>/dev/null; then
    kill "$WECOM_PID" 2>/dev/null || true
    wait "$WECOM_PID" 2>/dev/null || true
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
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
  exit 1
}

wait_file_line_count() {
  local file="$1"
  local expected="$2"
  local deadline=$((SECONDS + 45))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local actual=0
    if [ -f "$file" ]; then
      actual="$(wc -l <"$file" | tr -d ' ')"
    fi
    if [ "$actual" -ge "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $file to reach $expected lines" >&2
  [ -f "$WECOM_LOG" ] && tail -120 "$WECOM_LOG" >&2 || true
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
assert_port_free "$WECOM_ADDR"

cat >"$WORK_DIR/fake_wecom.py" <<'PY'
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

events = pathlib.Path(sys.argv[3])


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def json_response(self, data, status=200):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def record(self, path, query, payload):
        with events.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps({"path": path, "query": query, "body": payload}, ensure_ascii=False) + "\n")

    def do_GET(self):
        path = urlparse(self.path)
        query = parse_qs(path.query)
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            assert query.get("corpid", [""])[0] == "ww-channel-code", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "channel-code-token", "expires_in": 7200})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        payload = json.loads(raw.decode("utf-8") or "{}")
        self.record(path.path, query, payload)
        if path.path == "/cgi-bin/externalcontact/add_contact_way":
            assert payload.get("type") == 2, payload
            assert payload.get("scene") == 2, payload
            assert payload.get("skip_verify") is True, payload
            assert payload.get("user") == ["channel-code-user"], payload
            assert str(payload.get("state", "")).startswith("channelCode-"), payload
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "qr_code": "https://wecom.example/channel-code-create.png",
                "config_id": "channel-code-config-1",
            })
            return
        if path.path == "/cgi-bin/externalcontact/update_contact_way":
            assert payload.get("config_id") == "channel-code-config-1", payload
            assert payload.get("skip_verify") is False, payload
            assert payload.get("user") == ["channel-code-user"], payload
            assert str(payload.get("state", "")).startswith("channelCode-"), payload
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)


host = sys.argv[1]
port = int(sys.argv[2])
ThreadingHTTPServer((host, port), Handler).serve_forever()
PY

python3 "$WORK_DIR/fake_wecom.py" "${WECOM_ADDR%:*}" "${WECOM_ADDR##*:}" "$WECOM_EVENTS" >"$WECOM_LOG" 2>&1 &
WECOM_PID="$!"
wait_url "http://$WECOM_ADDR/healthz" 200

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
  -tenant-name "渠道活码验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "渠道活码管理员" \
  -role-name "渠道活码超级管理员" \
  -package-code "channel-code-standard" \
  -package-name "渠道活码标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -channel-codes 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_business_log WHERE business_id IN (SELECT id FROM mc_channel_code WHERE corp_id = $CORP_ID) AND event IN (100, 101);
DELETE FROM mc_work_contact_employee WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact WHERE corp_id = $CORP_ID;
DELETE FROM mc_channel_code WHERE corp_id = $CORP_ID;
DELETE FROM mc_channel_code_group WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag_group WHERE id = $TAG_GROUP_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '渠道活码企业', 'ww-channel-code', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'channel-code-user', $CORP_ID, '渠道活码员工', '$PHONE', 'avatar/channel-code.png', 'avatar/channel-code-thumb.png', '渠道员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'wx-channel-code-tag-group', $CORP_ID, '渠道活码标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-channel-code-tag', $CORP_ID, '重点渠道客户', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
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

api_json POST "/dashboard/channelCodeGroup/store" '{"name":["渠道活码分组A","渠道活码分组B"]}' "$WORK_DIR/group-store.json"
GROUP_A_ID="$(mysql_scalar "SELECT id FROM mc_channel_code_group WHERE corp_id = $CORP_ID AND name = '渠道活码分组A' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
GROUP_B_ID="$(mysql_scalar "SELECT id FROM mc_channel_code_group WHERE corp_id = $CORP_ID AND name = '渠道活码分组B' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$GROUP_A_ID"
test -n "$GROUP_B_ID"

api_json PUT "/dashboard/channelCodeGroup/update" "{\"groupId\":$GROUP_B_ID,\"name\":\"渠道活码分组B更新\"}" "$WORK_DIR/group-update.json"
test "$(mysql_scalar "SELECT name FROM mc_channel_code_group WHERE id = $GROUP_B_ID")" = "渠道活码分组B更新"
api_json GET "/dashboard/channelCodeGroup/index" "" "$WORK_DIR/group-index.json"
api_json GET "/dashboard/channelCodeGroup/detail?groupId=$GROUP_B_ID" "" "$WORK_DIR/group-detail.json"

python3 - "$WORK_DIR/group-index.json" "$WORK_DIR/group-detail.json" "$GROUP_A_ID" "$GROUP_B_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
detail_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
group_a = int(sys.argv[3])
group_b = int(sys.argv[4])
groups = {int(item["groupId"]): item["name"] for item in index_payload["data"]}
assert groups[group_a] == "渠道活码分组A", groups
assert groups[group_b] == "渠道活码分组B更新", groups
assert groups[0] == "未分组", groups
detail = detail_payload["data"]
assert int(detail["groupId"]) == group_b, detail
assert detail["name"] == "渠道活码分组B更新", detail
PY

STORE_BODY="$(python3 - "$GROUP_A_ID" "$TAG_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

body = {
    "baseInfo": {
        "groupId": int(sys.argv[1]),
        "name": "渠道活码验收",
        "autoAddFriend": 1,
        "tags": [int(sys.argv[2]), int(sys.argv[2]), 0],
    },
    "drainageEmployee": {
        "type": 1,
        "employees": [],
        "specialPeriod": {
            "status": 1,
            "detail": [{
                "startDate": "2026-01-01",
                "endDate": "2037-12-31",
                "timeSlot": [{
                    "startTime": "00:00",
                    "endTime": "00:00",
                    "employeeId": [int(sys.argv[3]), int(sys.argv[3]), 0],
                }],
            }],
        },
        "addMax": {"status": 2, "employees": [], "spareEmployeeIds": []},
    },
    "welcomeMessage": {
        "scanCodePush": 2,
        "messageDetail": [{"type": 0, "text": "欢迎通过渠道活码添加"}],
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/channelCode/store" "$STORE_BODY" "$WORK_DIR/channel-store.json"
CHANNEL_ID="$(mysql_scalar "SELECT id FROM mc_channel_code WHERE corp_id = $CORP_ID AND name = '渠道活码验收' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$CHANNEL_ID"
test "$(mysql_scalar "SELECT qrcode_url FROM mc_channel_code WHERE id = $CHANNEL_ID")" = "https://wecom.example/channel-code-create.png"
test "$(mysql_scalar "SELECT wx_config_id FROM mc_channel_code WHERE id = $CHANNEL_ID")" = "channel-code-config-1"
test "$(mysql_scalar "SELECT JSON_EXTRACT(tags, '\$[0]') FROM mc_channel_code WHERE id = $CHANNEL_ID")" = "$TAG_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_business_log WHERE business_id = $CHANNEL_ID AND event = 100 AND operation_id = $LOG_OPERATION_ID AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.baseInfo.name')) = '渠道活码验收'")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'channel_codes' AND deleted_at IS NULL")" = "1"

api_json PUT "/dashboard/channelCodeGroup/move" "{\"channelCodeId\":$CHANNEL_ID,\"groupId\":$GROUP_B_ID}" "$WORK_DIR/group-move.json"
test "$(mysql_scalar "SELECT group_id FROM mc_channel_code WHERE id = $CHANNEL_ID")" = "$GROUP_B_ID"

UPDATE_BODY="$(python3 - "$CHANNEL_ID" "$GROUP_B_ID" "$TAG_ID" "$EMPLOYEE_ID" <<'PY'
import json
import sys

body = {
    "channelCodeId": int(sys.argv[1]),
    "baseInfo": {
        "groupId": int(sys.argv[2]),
        "name": "渠道活码验收更新",
        "autoAddFriend": 2,
        "tags": [int(sys.argv[3])],
    },
    "drainageEmployee": {
        "type": 1,
        "employees": [],
        "specialPeriod": {
            "status": 1,
            "detail": [{
                "startDate": "2026-01-01",
                "endDate": "2037-12-31",
                "timeSlot": [{
                    "startTime": "00:00",
                    "endTime": "00:00",
                    "employeeId": [int(sys.argv[4])],
                }],
            }],
        },
        "addMax": {"status": 2, "employees": [], "spareEmployeeIds": []},
    },
    "welcomeMessage": {
        "scanCodePush": 2,
        "messageDetail": [{"type": 0, "text": "更新后的渠道活码欢迎语"}],
    },
}
print(json.dumps(body, ensure_ascii=False))
PY
)"
api_json PUT "/dashboard/channelCode/update" "$UPDATE_BODY" "$WORK_DIR/channel-update.json"
test "$(mysql_scalar "SELECT name FROM mc_channel_code WHERE id = $CHANNEL_ID")" = "渠道活码验收更新"
test "$(mysql_scalar "SELECT auto_add_friend FROM mc_channel_code WHERE id = $CHANNEL_ID")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_business_log WHERE business_id = $CHANNEL_ID AND event = 101 AND operation_id = $LOG_OPERATION_ID AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.baseInfo.name')) = '渠道活码验收更新'")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_work_contact
  (id, corp_id, wx_external_userid, name, nick_name, avatar, follow_up_status, type, gender, unionid, position, corp_name, corp_full_name, external_profile, business_no, created_at, updated_at, deleted_at)
VALUES
  ($CONTACT_ONE_ID, $CORP_ID, 'external-channel-code-1', '渠道活码客户A', '渠道客户A', 'avatar/contact-a.png', 1, 1, 1, 'union-channel-a', '', '', '', JSON_OBJECT(), 'CC-A', NOW(), NOW(), NULL),
  ($CONTACT_TWO_ID, $CORP_ID, 'external-channel-code-2', '渠道活码客户B', '渠道客户B', 'avatar/contact-b.png', 1, 1, 2, 'union-channel-b', '', '', '', JSON_OBJECT(), 'CC-B', NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_employee
  (employee_id, contact_id, remark, description, remark_corp_name, remark_mobiles, add_way, oper_userid, state, corp_id, status, create_time, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, $CONTACT_ONE_ID, '渠道客户A备注', '渠道客户A描述', '', JSON_ARRAY(), 1, 'external-channel-code-1', CONCAT('channelCode-', $CHANNEL_ID), $CORP_ID, 1, '2026-07-01 09:00:00', NOW(), NOW(), NULL),
  ($EMPLOYEE_ID, $CONTACT_TWO_ID, '渠道客户B备注', '渠道客户B描述', '', JSON_ARRAY(), 1, 'external-channel-code-2', CONCAT('channelCode-', $CHANNEL_ID), $CORP_ID, 3, '2026-07-02 09:00:00', NOW(), NOW(), '2026-07-03 09:00:00');
SQL

api_json GET "/dashboard/channelCode/index?name=%E6%B8%A0%E9%81%93%E6%B4%BB%E7%A0%81&type=1&groupId=$GROUP_B_ID&page=1&perPage=10" "" "$WORK_DIR/channel-index.json"
api_json GET "/dashboard/channelCode/show?channelCodeId=$CHANNEL_ID" "" "$WORK_DIR/channel-show.json"
api_json GET "/dashboard/channelCode/contact?channelCodeId=$CHANNEL_ID&page=1&perPage=15" "" "$WORK_DIR/channel-contact.json"
api_json GET "/dashboard/channelCode/statistics?channelCodeId=$CHANNEL_ID&type=1&startTime=2026-07-01&endTime=2026-07-03" "" "$WORK_DIR/channel-statistics.json"
api_json GET "/dashboard/channelCode/statisticsIndex?channelCodeId=$CHANNEL_ID&type=1&startTime=2026-07-01&endTime=2026-07-03&page=2&perPage=2" "" "$WORK_DIR/channel-statistics-index.json"

python3 - \
  "$WORK_DIR/channel-index.json" \
  "$WORK_DIR/channel-show.json" \
  "$WORK_DIR/channel-contact.json" \
  "$WORK_DIR/channel-statistics.json" \
  "$WORK_DIR/channel-statistics-index.json" \
  "$CHANNEL_ID" "$GROUP_B_ID" "$TAG_ID" "$EMPLOYEE_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
contact_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
statistics_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
statistics_index_payload = json.loads(pathlib.Path(sys.argv[5]).read_text(encoding="utf-8"))
channel_id = int(sys.argv[6])
group_b = int(sys.argv[7])
tag_id = int(sys.argv[8])
employee_id = int(sys.argv[9])

items = index_payload["data"]["list"]
item = next((row for row in items if int(row["channelCodeId"]) == channel_id), None)
assert item, items
assert item["name"] == "渠道活码验收更新", item
assert int(item["groupId"]) == group_b, item
assert item["groupName"] == "渠道活码分组B更新", item
assert item["qrcodeUrl"] == "https://wecom.example/channel-code-create.png", item
assert item["autoAddFriend"] == "关闭", item
assert item["type"] == "单人", item
assert item["tags"] == ["重点渠道客户"], item
assert int(item["contactNum"]) == 1, item

show = show_payload["data"]
base = show["baseInfo"]
assert int(base["groupId"]) == group_b, base
assert base["groupName"] == "渠道活码分组B更新", base
assert base["name"] == "渠道活码验收更新", base
assert int(base["autoAddFriend"]) == 2, base
assert tag_id in [int(v) for v in base["selectedTags"]], base
tag_items = [tag for group in base["tags"] for tag in group["list"]]
selected = next(tag for tag in tag_items if int(tag["tagId"]) == tag_id)
assert selected["tagName"] == "重点渠道客户" and int(selected["isSelected"]) == 1, selected
slot = show["drainageEmployee"]["specialPeriod"]["detail"][0]["timeSlot"][0]
assert int(slot["employeeId"][0]) == employee_id, slot
assert slot["selectMembers"] == ["渠道活码员工"], slot
assert show["welcomeMessage"]["messageDetail"][0]["text"] == "更新后的渠道活码欢迎语", show

contacts = contact_payload["data"]
assert int(contacts["page"]["total"]) == 1, contacts
contact = contacts["list"][0]
assert contact["name"] == "渠道活码客户A", contact
assert contact["employees"] == "渠道活码企业 渠道活码员工", contact

statistics = statistics_payload["data"]
assert int(statistics["addNumLong"]) == 2, statistics
assert int(statistics["defriendNumLong"]) == 1, statistics
assert int(statistics["netNumLong"]) == 1, statistics
rows = {row["time"]: row for row in statistics["list"]}
assert int(rows["2026-07-01"]["addNumRange"]) == 1, rows
assert int(rows["2026-07-02"]["addNumRange"]) == 1, rows
assert int(rows["2026-07-03"]["defriendNumRange"]) == 1, rows

stat_index = statistics_index_payload["data"]
assert int(stat_index["page"]["total"]) == 3, stat_index
assert int(stat_index["page"]["totalPage"]) == 2, stat_index
assert len(stat_index["list"]) == 1, stat_index
assert stat_index["list"][0]["time"] == "2026-07-03", stat_index
assert int(stat_index["list"][0]["defriendNumRange"]) == 1, stat_index
PY

wait_file_line_count "$WECOM_EVENTS" 2
python3 - "$WECOM_EVENTS" "$CHANNEL_ID" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
channel_id = int(sys.argv[2])
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/externalcontact/add_contact_way") == 1, paths
assert paths.count("/cgi-bin/externalcontact/update_contact_way") == 1, paths
create = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/add_contact_way")
update = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/update_contact_way")
assert create["state"] == f"channelCode-{channel_id}", create
assert create["skip_verify"] is True, create
assert create["user"] == ["channel-code-user"], create
assert update["state"] == f"channelCode-{channel_id}", update
assert update["config_id"] == "channel-code-config-1", update
assert update["skip_verify"] is False, update
assert update["user"] == ["channel-code-user"], update
PY

grep -q "go migrated route enabled: GET /dashboard/channelCode/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/channelCode/show" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/channelCode/contact" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/channelCode/statistics" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/channelCode/statisticsIndex" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/channelCode/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/channelCode/update" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/channelCodeGroup/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/channelCodeGroup/detail" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/channelCodeGroup/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/channelCodeGroup/update" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/channelCodeGroup/move" "$GO_LOG"

echo "channel code dashboard smoke passed"
