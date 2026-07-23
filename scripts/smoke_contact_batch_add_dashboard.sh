#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-contact-batch-add-dashboard-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18130}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13370}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26430}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-contact-batch-add.XXXXXX")"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-contact-batch-add-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800001830}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret1830}"
CORP_ID="${MOCHAT_SMOKE_CORP_ID:-91830}"
ADMIN_EMPLOYEE_ID="${MOCHAT_SMOKE_ADMIN_EMPLOYEE_ID:-91830}"
MEMBER_EMPLOYEE_ID="${MOCHAT_SMOKE_MEMBER_EMPLOYEE_ID:-91831}"
TAG_GROUP_ID="${MOCHAT_SMOKE_TAG_GROUP_ID:-918301}"
TAG_ID="${MOCHAT_SMOKE_TAG_ID:-918302}"

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
grep -q $'0009_contact_batch_add_rbac\tbaselined' "$WORK_DIR/migrate-baseline.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id 1 \
  -tenant-name "批量加好友验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "批量加好友管理员" \
  -role-name "批量加好友超级管理员" \
  -package-code "contact-batch-add-standard" \
  -package-name "批量加好友标准版" \
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
DELETE FROM mc_contact_batch_add_allot
WHERE import_id IN (SELECT id FROM mc_contact_batch_add_import WHERE corp_id = $CORP_ID)
   OR employee_id IN ($ADMIN_EMPLOYEE_ID, $MEMBER_EMPLOYEE_ID);
DELETE FROM mc_contact_batch_add_import WHERE corp_id = $CORP_ID;
DELETE FROM mc_contact_batch_add_import_record WHERE corp_id = $CORP_ID;
DELETE FROM mc_contact_batch_add_config WHERE corp_id = $CORP_ID;
DELETE FROM mochat_go_saas_storage_objects
WHERE tenant_id = 1 AND (source = 'dashboard.contactBatchAdd.importStore' OR relative_path LIKE 'contactBatchAdd/import/%');
DELETE FROM mc_work_contact_tag WHERE id = $TAG_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_contact_tag_group WHERE id = $TAG_GROUP_ID OR corp_id = $CORP_ID;
DELETE FROM mc_work_employee WHERE id IN ($ADMIN_EMPLOYEE_ID, $MEMBER_EMPLOYEE_ID);
DELETE FROM mc_corp WHERE id = $CORP_ID;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '批量加好友企业', 'ww-contact-batch-add', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, avatar, thumb_avatar, alias, gender, status, log_user_id, contact_auth, audit_status, created_at, updated_at, deleted_at)
VALUES
  ($ADMIN_EMPLOYEE_ID, 'batch-add-admin', $CORP_ID, '批量加好友管理员', '$PHONE', 'avatar/batch-add-admin.png', 'avatar/batch-add-admin-thumb.png', '批量管理员别名', 1, 1, $USER_ID, 1, 1, NOW(), NOW(), NULL),
  ($MEMBER_EMPLOYEE_ID, 'batch-add-member', $CORP_ID, '批量加好友成员', '13800001831', 'avatar/batch-add-member.png', 'avatar/batch-add-member-thumb.png', '批量成员别名', 2, 1, 0, 1, 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag_group
  (id, wx_group_id, corp_id, group_name, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($TAG_GROUP_ID, 'wx-contact-batch-add-tag-group', $CORP_ID, '批量加好友标签组', 1, NOW(), NOW(), NULL);

INSERT INTO mc_work_contact_tag
  (id, wx_contact_tag_id, corp_id, name, \`order\`, contact_tag_group_id, created_at, updated_at, deleted_at)
VALUES
  ($TAG_ID, 'wx-contact-batch-add-tag', $CORP_ID, '批量加好友标签', 1, $TAG_GROUP_ID, NOW(), NOW(), NULL);
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

api_json GET "/dashboard/contactBatchAdd/settingEdit" "" "$WORK_DIR/setting-before.json"
SETTING_BODY="$(python3 - "$ADMIN_EMPLOYEE_ID" <<'PY'
import json
import sys

leader = int(sys.argv[1])
print(json.dumps({
    "pendingStatus": 1,
    "pendingTimeOut": 3,
    "pendingReminderTime": "09:15",
    "pendingLeaderId": leader,
    "undoneStatus": 1,
    "undoneTimeOut": 2,
    "undoneReminderTime": "16:30:00",
    "recycleStatus": 1,
    "recycleTimeOut": 7,
}, ensure_ascii=False))
PY
)"
api_json POST "/dashboard/contactBatchAdd/settingUpdate" "$SETTING_BODY" "$WORK_DIR/setting-update.json"
api_json GET "/dashboard/contactBatchAdd/settingEdit" "" "$WORK_DIR/setting-after.json"
test "$(mysql_scalar "SELECT pending_status, pending_time_out, pending_reminder_time, pending_leader_id, undone_status, undone_time_out, undone_reminder_time, recycle_status, recycle_time_out FROM mc_contact_batch_add_config WHERE corp_id = $CORP_ID")" = $'1\t3\t09:15:00\t'"$ADMIN_EMPLOYEE_ID"$'\t1\t2\t16:30:00\t1\t7'

cat >"$WORK_DIR/contacts.csv" <<'CSV'
13800003001,客户A
bad,忽略
13800003002,客户B
13800003001,重复
13800003003,客户C
CSV

curl -sS -f -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -F "title=批量加好友验收导入" \
  -F "allotEmployee[]=$ADMIN_EMPLOYEE_ID" \
  -F "allotEmployee[]=$MEMBER_EMPLOYEE_ID" \
  -F "tags[]=$TAG_ID" \
  -F "file=@$WORK_DIR/contacts.csv;type=text/csv" \
  "http://$GO_ADDR/dashboard/contactBatchAdd/importStore" >"$WORK_DIR/import-store.json"
assert_api_ok "$WORK_DIR/import-store.json"

RECORD_ID="$(mysql_scalar "SELECT id FROM mc_contact_batch_add_import_record WHERE corp_id = $CORP_ID AND title = '批量加好友验收导入' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$RECORD_ID"
IMPORT_A_ID="$(mysql_scalar "SELECT id FROM mc_contact_batch_add_import WHERE corp_id = $CORP_ID AND record_id = $RECORD_ID AND phone = '13800003001' AND deleted_at IS NULL")"
IMPORT_B_ID="$(mysql_scalar "SELECT id FROM mc_contact_batch_add_import WHERE corp_id = $CORP_ID AND record_id = $RECORD_ID AND phone = '13800003002' AND deleted_at IS NULL")"
IMPORT_C_ID="$(mysql_scalar "SELECT id FROM mc_contact_batch_add_import WHERE corp_id = $CORP_ID AND record_id = $RECORD_ID AND phone = '13800003003' AND deleted_at IS NULL")"
test -n "$IMPORT_A_ID"
test -n "$IMPORT_B_ID"
test -n "$IMPORT_C_ID"

FILE_URL="$(mysql_scalar "SELECT file_url FROM mc_contact_batch_add_import_record WHERE id = $RECORD_ID")"
test -n "$FILE_URL"
test -f "$FILE_STORAGE_ROOT/$FILE_URL"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = 1 AND relative_path = '$FILE_URL' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'storage_mb' AND deleted_at IS NULL")" = "1"

api_json GET "/dashboard/contactBatchAdd/importIndex?page=1&perPage=5" "" "$WORK_DIR/import-index.json"
api_json GET "/dashboard/contactBatchAdd/index?status=1&searchKey=13800003002&recordId=$RECORD_ID&page=1&perPage=5" "" "$WORK_DIR/index-search.json"
api_json GET "/dashboard/contactBatchAdd/dataStatistic?page=1&perPage=5" "" "$WORK_DIR/data-statistic-before-allot.json"
api_json GET "/dashboard/contactBatchAdd/remind" "" "$WORK_DIR/remind-get.json"
api_json POST "/dashboard/contactBatchAdd/remind" "{}" "$WORK_DIR/remind-post.json"

python3 - "$WORK_DIR" "$GO_ADDR" "$RECORD_ID" "$IMPORT_B_ID" "$ADMIN_EMPLOYEE_ID" "$MEMBER_EMPLOYEE_ID" "$TAG_ID" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
go_addr = sys.argv[2]
record_id = int(sys.argv[3])
import_b_id = int(sys.argv[4])
admin_id = int(sys.argv[5])
member_id = int(sys.argv[6])
tag_id = int(sys.argv[7])


def load(name):
    return json.loads((work / name).read_text(encoding="utf-8"))


assert load("import-store.json")["data"]["successNum"] == 3

setting_before = load("setting-before.json")["data"]
assert setting_before["pendingStatus"] == 0, setting_before
setting_after = load("setting-after.json")["data"]
assert setting_after["pendingStatus"] == 1, setting_after
assert setting_after["pendingReminderTime"] == "09:15:00", setting_after
assert setting_after["pendingLeaderId"] == admin_id, setting_after
assert setting_after["pendingLeader"]["name"] == "批量加好友管理员", setting_after

record_rows = load("import-index.json")["data"]["data"]
record = next((item for item in record_rows if int(item["id"]) == record_id), None)
assert record, record_rows
assert record["title"] == "批量加好友验收导入", record
assert record["importNum"] == 3, record
assert record["addNum"] == 0, record
assert record["fileName"] == "contacts.csv", record
assert record["fileUrl"].startswith(f"http://{go_addr}/static/contactBatchAdd/import/"), record
assert {item["id"] for item in record["allotEmployee"]} == {admin_id, member_id}, record
assert record["tags"] == [{"id": tag_id, "name": "批量加好友标签"}], record

index_rows = load("index-search.json")["data"]["data"]
assert len(index_rows) == 1, index_rows
row = index_rows[0]
assert int(row["id"]) == import_b_id, row
assert row["phone"] == "13800003002", row
assert row["status"] == 1 and row["statusText"] == "待添加", row
assert row["remark"] == "客户B", row
assert row["allotEmployee"]["id"] == member_id, row
assert row["tags"] == [{"id": tag_id, "name": "批量加好友标签"}], row

stats = load("data-statistic-before-allot.json")["data"]
dashboard = stats["dashboard"]
assert dashboard["contactNum"] == 3, dashboard
assert dashboard["toAddNum"] == 3, dashboard
assert dashboard["pendingNum"] == 0 and dashboard["passedNum"] == 0, dashboard
employee_stats = {item["id"]: item for item in stats["employees"]["data"]}
assert employee_stats[admin_id]["allotNum"] == 2, employee_stats
assert employee_stats[member_id]["allotNum"] == 1, employee_stats

assert load("remind-get.json")["data"] == []
assert load("remind-post.json")["data"] == []
PY

api_json POST "/dashboard/contactBatchAdd/allot" "{\"ids\":[$IMPORT_B_ID],\"allotEmployee\":[$ADMIN_EMPLOYEE_ID]}" "$WORK_DIR/allot.json"
test "$(mysql_scalar "SELECT employee_id, status, allot_num FROM mc_contact_batch_add_import WHERE id = $IMPORT_B_ID")" = $''"$ADMIN_EMPLOYEE_ID"$'\t1\t2'
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_allot WHERE import_id = $IMPORT_B_ID AND employee_id = $ADMIN_EMPLOYEE_ID AND type = 1")" = "1"

api_json DELETE "/dashboard/contactBatchAdd/destroy" "{\"ids\":[$IMPORT_C_ID]}" "$WORK_DIR/destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_import WHERE id = $IMPORT_C_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_import WHERE corp_id = $CORP_ID AND record_id = $RECORD_ID AND deleted_at IS NULL")" = "2"

api_json DELETE "/dashboard/contactBatchAdd/importDestroy" "{\"ids\":[$RECORD_ID]}" "$WORK_DIR/import-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_import_record WHERE id = $RECORD_ID AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_batch_add_import WHERE id IN ($IMPORT_A_ID, $IMPORT_B_ID, $IMPORT_C_ID) AND deleted_at IS NOT NULL")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = 1 AND relative_path = '$FILE_URL' AND deleted_at IS NOT NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = 1 AND metric = 'storage_mb' AND deleted_at IS NULL")" = "0"

python3 - "$WORK_DIR/destroy.json" "$WORK_DIR/import-destroy.json" <<'PY'
import json
import pathlib
import sys

destroy = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
import_destroy = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
assert destroy["data"]["delNum"] == 1, destroy
assert import_destroy["data"]["delRecordNum"] == 1, import_destroy
assert import_destroy["data"]["delContactNum"] == 2, import_destroy
PY

api_json GET "/dashboard/contactBatchAdd/index?recordId=$RECORD_ID&page=1&perPage=5" "" "$WORK_DIR/index-after-destroy.json"
python3 - "$WORK_DIR/index-after-destroy.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["data"]["total"] == 0, payload
PY

grep -q "go migrated route enabled: GET /dashboard/contactBatchAdd/index" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/contactBatchAdd/importStore" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/contactBatchAdd/allot" "$GO_LOG"
grep -q "go migrated route enabled: DELETE /dashboard/contactBatchAdd/importDestroy" "$GO_LOG"
grep -q "go migrated route enabled: GET/POST /dashboard/contactBatchAdd/remind" "$GO_LOG"

echo "contact batch add dashboard standalone smoke passed"
