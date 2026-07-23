#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-contact-tag-write-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18104}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13348}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26406}"
WECOM_ADDR="${MOCHAT_WECOM_ADDR:-127.0.0.1:19086}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-contact-tag-write.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
WECOM_LOG="$WORK_DIR/wecom.log"
WECOM_EVENTS="$WORK_DIR/wecom-events.jsonl"
GO_PID=""
WECOM_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-contact-tag-write-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001804}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1804}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91804}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91804}"

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
add_count = 0


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def json_response(self, data):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def record(self, path, payload):
        with events.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps({"path": path, "body": payload}, ensure_ascii=False) + "\n")

    def do_GET(self):
        path = urlparse(self.path)
        query = parse_qs(path.query)
        if path.path == "/healthz":
            self.json_response({"ok": True})
            return
        if path.path == "/cgi-bin/gettoken":
            assert query.get("corpid", [""])[0] == "ww-tag-write", query
            assert query.get("corpsecret", [""])[0] == "contact-secret", query
            self.json_response({"errcode": 0, "errmsg": "ok", "access_token": "contact-token", "expires_in": 7200})
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        global add_count
        path = urlparse(self.path)
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        payload = json.loads(raw.decode("utf-8") or "{}")
        self.record(path.path, payload)
        if path.path == "/cgi-bin/externalcontact/add_corp_tag":
            add_count += 1
            group_id = payload.get("group_id") or f"wx-group-created-{add_count}"
            tags = []
            for idx, tag in enumerate(payload.get("tag", []), start=1):
                tags.append({"id": f"wx-tag-created-{add_count}-{idx}", "name": tag.get("name", ""), "order": idx})
            self.json_response({
                "errcode": 0,
                "errmsg": "ok",
                "tag_group": {"group_id": group_id, "group_name": payload.get("group_name", ""), "order": add_count, "tag": tags},
            })
            return
        if path.path in {"/cgi-bin/externalcontact/edit_corp_tag", "/cgi-bin/externalcontact/del_corp_tag"}:
            self.json_response({"errcode": 0, "errmsg": "ok"})
            return
        self.send_response(404)
        self.end_headers()


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
  -tenant-name "标签远端写验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "标签管理员" \
  -role-name "标签超级管理员" \
  -package-code "tag-write-standard" \
  -package-name "标签写标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -channel-codes 120 \
  -storage-mb 1024 \
  -contact-message-batches 300 \
  -room-message-batches 150 \
  -room-tag-pulls 80 \
  -work-room-auto-pulls 60 \
  -work-fissions 40 \
  -official-accounts 10 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_work_contact_tag_pivot;
DELETE FROM mc_work_contact_tag;
DELETE FROM mc_work_contact_tag_group;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '标签远端写企业', 'ww-tag-write', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'tag-write-user', $CORP_ID, '标签员工', '$PHONE', 1, 1, $USER_ID, NOW(), NOW(), NULL);
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

api_json POST "/dashboard/workContactTagGroup/store" '{"groupName":"远端同步组"}' "$WORK_DIR/group-create.json"
GROUP_ID="$(mysql_scalar "SELECT id FROM mc_work_contact_tag_group WHERE corp_id = $CORP_ID AND group_name = '远端同步组' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$GROUP_ID"
api_json PUT "/dashboard/workContactTagGroup/update" "{\"groupId\":$GROUP_ID,\"groupName\":\"远端同步组改名\",\"isUpdate\":1}" "$WORK_DIR/group-update.json"
test "$(mysql_scalar "SELECT group_name FROM mc_work_contact_tag_group WHERE id = $GROUP_ID")" = "远端同步组改名"

api_json POST "/dashboard/workContactTag/store" "{\"groupId\":$GROUP_ID,\"tagName\":[\"远端同步标签\"]}" "$WORK_DIR/tag-create.json"
test "$(mysql_scalar "SELECT wx_group_id FROM mc_work_contact_tag_group WHERE id = $GROUP_ID")" = "wx-group-created-1"
TAG_ID="$(mysql_scalar "SELECT id FROM mc_work_contact_tag WHERE corp_id = $CORP_ID AND name = '远端同步标签' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$TAG_ID"
test "$(mysql_scalar "SELECT wx_contact_tag_id FROM mc_work_contact_tag WHERE id = $TAG_ID")" = "wx-tag-created-1-1"

api_json PUT "/dashboard/workContactTag/update" "{\"tagId\":$TAG_ID,\"groupId\":$GROUP_ID,\"tagName\":\"远端同步标签改名\",\"isUpdate\":1}" "$WORK_DIR/tag-update.json"
test "$(mysql_scalar "SELECT name FROM mc_work_contact_tag WHERE id = $TAG_ID")" = "远端同步标签改名"

api_json POST "/dashboard/workContactTagGroup/store" '{"groupName":"移动目标组"}' "$WORK_DIR/group-target-create.json"
TARGET_GROUP_ID="$(mysql_scalar "SELECT id FROM mc_work_contact_tag_group WHERE corp_id = $CORP_ID AND group_name = '移动目标组' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$TARGET_GROUP_ID"
api_json PUT "/dashboard/workContactTag/move" "{\"tagId\":\"$TAG_ID\",\"groupId\":$TARGET_GROUP_ID}" "$WORK_DIR/tag-move.json"
test "$(mysql_scalar "SELECT contact_tag_group_id FROM mc_work_contact_tag WHERE id = $TAG_ID")" = "$TARGET_GROUP_ID"
test "$(mysql_scalar "SELECT wx_group_id FROM mc_work_contact_tag_group WHERE id = $TARGET_GROUP_ID")" = "wx-group-created-2"
test "$(mysql_scalar "SELECT wx_contact_tag_id FROM mc_work_contact_tag WHERE id = $TAG_ID")" = "wx-tag-created-2-1"

api_json DELETE "/dashboard/workContactTag/destroy" "{\"tagId\":\"$TAG_ID\"}" "$WORK_DIR/tag-delete.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag WHERE id = $TAG_ID AND deleted_at IS NOT NULL")" = "1"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
INSERT INTO mc_work_contact_tag_group
  (corp_id, wx_group_id, group_name, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, 'wx-group-to-delete', '待删除远端组', NOW(), NOW(), NULL);
SQL
DELETE_GROUP_ID="$(mysql_scalar "SELECT id FROM mc_work_contact_tag_group WHERE corp_id = $CORP_ID AND group_name = '待删除远端组' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
api_json DELETE "/dashboard/workContactTagGroup/destroy" "{\"groupId\":$DELETE_GROUP_ID}" "$WORK_DIR/group-delete.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_contact_tag_group WHERE id = $DELETE_GROUP_ID AND deleted_at IS NOT NULL")" = "1"

python3 - "$WECOM_EVENTS" <<'PY'
import json
import pathlib
import sys

events = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if line.strip()]
paths = [event["path"] for event in events]
assert paths.count("/cgi-bin/externalcontact/add_corp_tag") == 2, paths
assert "/cgi-bin/externalcontact/edit_corp_tag" in paths, paths
assert paths.count("/cgi-bin/externalcontact/del_corp_tag") >= 3, paths

adds = [event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/add_corp_tag"]
assert adds[0].get("group_name") == "远端同步组改名", adds
assert adds[0].get("tag", [{}])[0].get("name") == "远端同步标签", adds
assert adds[1].get("group_name") == "移动目标组", adds
assert adds[1].get("tag", [{}])[0].get("name") == "远端同步标签改名", adds

edits = [event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/edit_corp_tag"]
assert edits[0].get("id") == "wx-tag-created-1-1", edits
assert edits[0].get("name") == "远端同步标签改名", edits

deletes = [event["body"] for event in events if event["path"] == "/cgi-bin/externalcontact/del_corp_tag"]
assert any("wx-tag-created-1-1" in body.get("tag_id", []) for body in deletes), deletes
assert any("wx-tag-created-2-1" in body.get("tag_id", []) for body in deletes), deletes
assert any("wx-group-to-delete" in body.get("group_id", []) for body in deletes), deletes
PY

grep -q "go migrated route enabled: POST /dashboard/workContactTag/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/workContactTag/move" "$GO_LOG"

echo "work contact tag remote write smoke passed"
