#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-sensitive-word-dashboard-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18128}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13368}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26428}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-sensitive-word.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-sensitive-word-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001828}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1828}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91828}"
EMPLOYEE_ID="${MOCHAT_SMOKE_EMPLOYEE_ID:-91828}"
ROOM_ID="${MOCHAT_SMOKE_ROOM_ID:-918281}"
WORK_CONTACT_ID="${MOCHAT_SMOKE_WORK_CONTACT_ID:-918282}"
EMPLOYEE_MONITOR_ID="${MOCHAT_SMOKE_EMPLOYEE_MONITOR_ID:-918283}"
CONTACT_MONITOR_ID="${MOCHAT_SMOKE_CONTACT_MONITOR_ID:-918284}"

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

mkdir -p "$FILE_STORAGE_ROOT/sensitiveWord"

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
  -tenant-name "敏感词验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "敏感词管理员" \
  -role-name "敏感词超级管理员" \
  -package-code "sensitive-word-standard" \
  -package-name "敏感词标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -sensitive-words 20 \
  -storage-mb 1024 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_sensitive_words_monitor WHERE corp_id = $CORP_ID OR id IN ($EMPLOYEE_MONITOR_ID, $CONTACT_MONITOR_ID);
DELETE FROM mc_sensitive_word WHERE corp_id = $CORP_ID;
DELETE FROM mc_sensitive_word_group WHERE corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id = $EMPLOYEE_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '敏感词企业', 'ww-sensitive-word', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_ID, 'sensitive-word-user', $CORP_ID, '敏感词员工', '$PHONE', 'avatar/sensitive-word.png', 'avatar/sensitive-word-thumb.png', '敏感词员工别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL);
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

api_json POST "/dashboard/sensitiveWordGroup/store" "{\"name\":\"敏感词分组A，敏感词分组B\"}" "$WORK_DIR/group-store.json"
GROUP_A_ID="$(mysql_scalar "SELECT id FROM mc_sensitive_word_group WHERE corp_id = $CORP_ID AND name = '敏感词分组A' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
GROUP_B_ID="$(mysql_scalar "SELECT id FROM mc_sensitive_word_group WHERE corp_id = $CORP_ID AND name = '敏感词分组B' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$GROUP_A_ID"
test -n "$GROUP_B_ID"

api_json PUT "/dashboard/sensitiveWordGroup/update" "{\"groupId\":$GROUP_B_ID,\"name\":\"敏感词分组B更新\"}" "$WORK_DIR/group-update.json"
test "$(mysql_scalar "SELECT name FROM mc_sensitive_word_group WHERE id = $GROUP_B_ID")" = "敏感词分组B更新"

api_json GET "/dashboard/sensitiveWordGroup/select" "" "$WORK_DIR/group-select.json"
python3 - "$WORK_DIR/group-select.json" "$GROUP_A_ID" "$GROUP_B_ID" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
group_a = int(sys.argv[2])
group_b = int(sys.argv[3])
groups = {int(row["groupId"]): row["name"] for row in payload["data"]}
assert groups[group_a] == "敏感词分组A", groups
assert groups[group_b] == "敏感词分组B更新", groups
PY

api_json POST "/dashboard/sensitiveWord/store" "{\"groupId\":$GROUP_A_ID,\"name\":\"敏感词A，敏感词B、敏感词A\\n敏感词C\"}" "$WORK_DIR/word-store.json"
WORD_A_ID="$(mysql_scalar "SELECT id FROM mc_sensitive_word WHERE corp_id = $CORP_ID AND name = '敏感词A' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
WORD_B_ID="$(mysql_scalar "SELECT id FROM mc_sensitive_word WHERE corp_id = $CORP_ID AND name = '敏感词B' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
WORD_C_ID="$(mysql_scalar "SELECT id FROM mc_sensitive_word WHERE corp_id = $CORP_ID AND name = '敏感词C' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$WORD_A_ID"
test -n "$WORD_B_ID"
test -n "$WORD_C_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_word WHERE corp_id = $CORP_ID AND deleted_at IS NULL")" = "3"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "3"

api_json PUT "/dashboard/sensitiveWord/statusUpdate" "{\"sensitiveWordId\":$WORD_C_ID,\"status\":2}" "$WORK_DIR/word-status.json"
test "$(mysql_scalar "SELECT status FROM mc_sensitive_word WHERE id = $WORD_C_ID")" = "2"

api_json PUT "/dashboard/sensitiveWord/move" "{\"sensitiveWordId\":$WORD_B_ID,\"groupId\":$GROUP_B_ID}" "$WORK_DIR/word-move.json"
test "$(mysql_scalar "SELECT group_id FROM mc_sensitive_word WHERE id = $WORD_B_ID")" = "$GROUP_B_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_sensitive_words_monitor
  (id, corp_id, sensitive_word_id, sensitive_word_name, source, trigger_user_id, trigger_name, work_room_id, trigger_scenario, sender, msg_type, send_time, content, conversation_json, created_at, updated_at, deleted_at)
VALUES
  (
    $EMPLOYEE_MONITOR_ID,
    $CORP_ID,
    $WORD_A_ID,
    '敏感词A',
    2,
    $EMPLOYEE_ID,
    '敏感词员工',
    $ROOM_ID,
    '客户会话',
    '敏感词员工',
    1,
    '2026-07-07 09:00:00',
    JSON_OBJECT('content', '员工提到敏感词A'),
    JSON_ARRAY(JSON_OBJECT('sender', '敏感词员工', 'msgType', 1, 'sendTime', '2026-07-07 09:00:00', 'isTrigger', 1, 'msgContent', JSON_OBJECT('content', '员工提到敏感词A'))),
    NOW(),
    NOW(),
    NULL
  ),
  (
    $CONTACT_MONITOR_ID,
    $CORP_ID,
    $WORD_A_ID,
    '敏感词A',
    1,
    $WORK_CONTACT_ID,
    '触发客户A',
    $ROOM_ID,
    '客户群',
    '触发客户A',
    1,
    '2026-07-07 09:05:00',
    JSON_OBJECT('content', '客户提到敏感词A'),
    JSON_ARRAY(JSON_OBJECT('sender', '触发客户A', 'msgType', 1, 'sendTime', '2026-07-07 09:05:00', 'isTrigger', 1, 'msgContent', JSON_OBJECT('content', '客户提到敏感词A'))),
    NOW(),
    NOW(),
    NULL
  );
SQL

api_json GET "/dashboard/sensitiveWord/index?groupId=$GROUP_A_ID&keyWords=%E6%95%8F%E6%84%9F%E8%AF%8DA&page=1&perPage=10" "" "$WORK_DIR/word-index-a.json"
api_json GET "/dashboard/sensitiveWord/index?groupId=$GROUP_B_ID&keyWords=%E6%95%8F%E6%84%9F%E8%AF%8DB&page=1&perPage=10" "" "$WORK_DIR/word-index-b.json"
api_json GET "/dashboard/sensitiveWordsMonitor/index?employeeId=$EMPLOYEE_ID&workRoomId=$ROOM_ID&intelligentGroupId=$GROUP_A_ID&triggerStart=2026-07-07%2000:00:00&triggerEnd=2026-07-07%2023:59:59&page=1&perPage=10" "" "$WORK_DIR/monitor-index.json"
api_json GET "/dashboard/sensitiveWordsMonitor/show?sensitiveWordsMonitorId=$EMPLOYEE_MONITOR_ID" "" "$WORK_DIR/monitor-show.json"

python3 - "$WORK_DIR/word-index-a.json" "$WORK_DIR/word-index-b.json" "$WORK_DIR/monitor-index.json" "$WORK_DIR/monitor-show.json" "$WORD_A_ID" "$WORD_B_ID" "$EMPLOYEE_MONITOR_ID" <<'PY'
import json
import pathlib
import sys

word_a_payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
word_b_payload = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
monitor_payload = json.loads(pathlib.Path(sys.argv[3]).read_text(encoding="utf-8"))
show_payload = json.loads(pathlib.Path(sys.argv[4]).read_text(encoding="utf-8"))
word_a = int(sys.argv[5])
word_b = int(sys.argv[6])
monitor_id = int(sys.argv[7])

items_a = word_a_payload["data"]["list"]
row_a = next((row for row in items_a if int(row["sensitiveWordId"]) == word_a), None)
assert row_a, items_a
assert row_a["name"] == "敏感词A", row_a
assert row_a["groupName"] == "敏感词分组A", row_a
assert int(row_a["employeeNum"]) == 1 and int(row_a["contactNum"]) == 1, row_a

items_b = word_b_payload["data"]["list"]
row_b = next((row for row in items_b if int(row["sensitiveWordId"]) == word_b), None)
assert row_b, items_b
assert row_b["groupName"] == "敏感词分组B更新", row_b

monitors = monitor_payload["data"]["list"]
monitor = next((row for row in monitors if int(row["sensitiveWordMonitorId"]) == monitor_id), None)
assert monitor, monitors
assert monitor["sensitiveWordName"] == "敏感词A", monitor
assert monitor["sourceText"] == "员工", monitor
assert monitor["triggerName"] == "敏感词员工", monitor
assert monitor["triggerScenario"] == "客户会话", monitor

messages = show_payload["data"]
assert len(messages) == 1, messages
assert messages[0]["sender"] == "敏感词员工", messages
assert messages[0]["msgContent"]["content"] == "员工提到敏感词A", messages
PY

api_json DELETE "/dashboard/sensitiveWord/destroy" "{\"sensitiveWordId\":$WORD_C_ID}" "$WORK_DIR/word-destroy-c.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_word WHERE id = $WORD_C_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "2"

api_json DELETE "/dashboard/sensitiveWord/destroy" "{\"sensitiveWordId\":$WORD_A_ID}" "$WORK_DIR/word-destroy-a.json"
api_json DELETE "/dashboard/sensitiveWord/destroy" "{\"sensitiveWordId\":$WORD_B_ID}" "$WORK_DIR/word-destroy-b.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_sensitive_word WHERE id IN ($WORD_A_ID, $WORD_B_ID) AND deleted_at IS NOT NULL")" = "2"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'sensitive_words' AND deleted_at IS NULL")" = "0"

grep -q "go migrated route enabled: GET /dashboard/sensitiveWords/page" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/sensitiveWord/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/sensitiveWord/store" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/sensitiveWord/destroy" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/sensitiveWord/statusUpdate" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/sensitiveWord/move" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/sensitiveWordGroup/select" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/sensitiveWordGroup/store" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/sensitiveWordGroup/update" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/sensitiveWordsMonitor/index" "$GO_LOG"
grep -q "go migrated route enabled: GET /dashboard/sensitiveWordsMonitor/show" "$GO_LOG"

echo "sensitive word dashboard smoke passed"
