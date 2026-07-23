#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-room-welcome-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18112}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13352}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26412}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19092}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-room-welcome.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-room-welcome-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001812}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1812}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91812}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91812}"

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
uploadimg_count = 0
upload_count = 0


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
            assert query.get("corpid", [""])[0] == "ww-room-welcome", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "room-welcome-token", "expires_in": 7200})
            return
        self.json_response({"errcode": 404, "errmsg": "not found"}, 404)

    def do_POST(self):
        global uploadimg_count, upload_count
        path = urlparse(self.path)
        query = {key: values[0] for key, values in parse_qs(path.query).items()}
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b""
        payload = None
        if self.headers.get("Content-Type", "").startswith("application/json"):
            payload = json.loads(raw.decode("utf-8") or "{}")
        self.record(path.path, query, payload if payload is not None else {"multipart": True, "bytes": len(raw)})
        if path.path == "/cgi-bin/media/uploadimg":
            uploadimg_count += 1
            self.json_response({"errcode": 0, "errmsg": "ok", "url": f"https://wecom.example/uploadimg-{uploadimg_count}.jpg"})
            return
        if path.path == "/cgi-bin/media/upload":
            upload_count += 1
            assert query.get("type") == "image", query
            self.json_response({"errcode": 0, "errmsg": "ok", "media_id": f"media-mini-{upload_count}"})
            return
        if path.path == "/cgi-bin/externalcontact/group_welcome_template/add":
            self.json_response({"errcode": 0, "errmsg": "ok", "template_id": "tpl-room-welcome-1"})
            return
        if path.path == "/cgi-bin/externalcontact/group_welcome_template/edit":
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        if path.path == "/cgi-bin/externalcontact/group_welcome_template/del":
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
  -tenant-name "入群欢迎语验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "入群欢迎语管理员" \
  -role-name "入群欢迎语超级管理员" \
  -package-code "room-welcome-standard" \
  -package-name "入群欢迎语标准版" \
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
DELETE FROM mc_room_welcome_template;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '入群欢迎语企业', 'ww-room-welcome', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'room-welcome-user', $CORP_ID, '入群欢迎语员工', '$PHONE', 1, 1, $USER_ID, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_WECOM_API_BASE_URL="http://$WECOM_ADDR" \
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

CREATE_BODY='{
  "msg_text":"欢迎[用户昵称]加入客户群",
  "notice":1,
  "msg_complex":{
    "type":"link",
    "link":{
      "title":"入群指南",
      "desc":"查看群规则",
      "url":"https://example.com/room-guide",
      "pic":"data:image/jpeg;base64,MTIzNDU2Nzg5MA=="
    }
  }
}'
api_json POST "/dashboard/roomWelcome/store" "$CREATE_BODY" "$WORK_DIR/room-welcome-create.json"

ROOM_WELCOME_ID="$(mysql_scalar "SELECT id FROM mc_room_welcome_template WHERE corp_id = $CORP_ID AND complex_template_id = 'tpl-room-welcome-1' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROOM_WELCOME_ID"
test "$(mysql_scalar "SELECT complex_type FROM mc_room_welcome_template WHERE id = $ROOM_WELCOME_ID")" = "link"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_welcome_template WHERE id = $ROOM_WELCOME_ID AND msg_text = '欢迎[用户昵称]加入客户群' AND msg_complex LIKE '%https://wecom.example/uploadimg-1.jpg%'")" = "1"

api_json GET "/dashboard/roomWelcome/index?text=%E6%AC%A2%E8%BF%8E" "" "$WORK_DIR/room-welcome-index.json"
api_json GET "/dashboard/roomWelcome/show?id=$ROOM_WELCOME_ID" "" "$WORK_DIR/room-welcome-show.json"
api_json GET "/dashboard/roomWelcome/select?id=$ROOM_WELCOME_ID" "" "$WORK_DIR/room-welcome-select.json"
python3 - "$WORK_DIR/room-welcome-index.json" "$WORK_DIR/room-welcome-show.json" "$WORK_DIR/room-welcome-select.json" "$ROOM_WELCOME_ID" <<'PY'
import json
import pathlib
import sys

index_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
select_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
room_welcome_id = int(sys.argv[4])

items = index_payload["data"]["list"]
assert any(int(item["id"]) == room_welcome_id and "欢迎" in item["msg_text"] for item in items), items
for payload in (show_payload, select_payload):
    data = payload["data"]
    assert int(data["id"]) == room_welcome_id, data
    assert data["complexTemplateId"] == "tpl-room-welcome-1", data
PY

UPDATE_BODY="$(cat <<JSON
{
  "id": $ROOM_WELCOME_ID,
  "msg_text": "更新[用户昵称]入群说明",
  "notice": 0,
  "msg_complex": {
    "type": "miniprogram",
    "miniprogram": {
      "title": "入群小程序",
      "appid": "wx-mini-room",
      "page": "pages/index",
      "pic": "data:image/jpeg;base64,QUJDREVGR0hJSg=="
    }
  }
}
JSON
)"
api_json PUT "/dashboard/roomWelcome/update" "$UPDATE_BODY" "$WORK_DIR/room-welcome-update.json"
test "$(mysql_scalar "SELECT complex_type FROM mc_room_welcome_template WHERE id = $ROOM_WELCOME_ID")" = "miniprogram"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_welcome_template WHERE id = $ROOM_WELCOME_ID AND msg_text = '更新[用户昵称]入群说明' AND msg_complex LIKE '%media-mini-1%'")" = "1"

api_json DELETE "/dashboard/roomWelcome/destroy" "{\"id\":$ROOM_WELCOME_ID}" "$WORK_DIR/room-welcome-delete.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_room_welcome_template WHERE id = $ROOM_WELCOME_ID AND deleted_at IS NOT NULL")" = "1"

python3 - "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/media/uploadimg") == 1, paths
assert paths.count("/cgi-bin/media/upload") == 1, paths
assert paths.count("/cgi-bin/externalcontact/group_welcome_template/add") == 1, paths
assert paths.count("/cgi-bin/externalcontact/group_welcome_template/edit") == 1, paths
assert paths.count("/cgi-bin/externalcontact/group_welcome_template/del") == 1, paths

add = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/group_welcome_template/add")
assert add["notify"] == 1, add
assert add["text"]["content"] == "欢迎%NICKNAME%加入客户群", add
assert add["link"]["title"] == "入群指南", add
assert add["link"]["picurl"] == "https://wecom.example/uploadimg-1.jpg", add

edit = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/group_welcome_template/edit")
assert edit["notify"] == 0, edit
assert edit["template_id"] == "tpl-room-welcome-1", edit
assert edit["text"]["content"] == "更新%NICKNAME%入群说明", edit
assert edit["miniprogram"]["pic_media_id"] == "media-mini-1", edit
assert edit["miniprogram"]["appid"] == "wx-mini-room", edit

delete = next(event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/group_welcome_template/del")
assert delete["template_id"] == "tpl-room-welcome-1", delete
PY

grep -q "go migrated route enabled: POST /dashboard/roomWelcome/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/roomWelcome/update" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/roomWelcome/destroy" "$GO_LOG"

echo "room welcome dashboard smoke passed"
