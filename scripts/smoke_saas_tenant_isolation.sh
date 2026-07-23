#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-isolation-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13342}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26392}"
GO_ADDR="${MOCHAT_SAAS_ISOLATION_GO_ADDR:-127.0.0.1:18112}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-isolation.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
STORAGE_ROOT="$WORK_DIR/upload"

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-isolation-secret}"
TENANT_A=301
TENANT_B=302
PHONE_A=13800000301
PHONE_B=13800000302
PASSWORD_A=secret301
PASSWORD_B=secret302
CORP_A=301001
CORP_B=302001
EMPLOYEE_A=301001
EMPLOYEE_B=302001
DEPARTMENT_A=301001
DEPARTMENT_B=302001
APP_TODAY="$(python3 - <<'PY'
from datetime import date
print(date.today().isoformat())
PY
)"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" \
    MOCHAT_REDIS_PORT="$REDIS_PORT" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
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
  local port="$1"
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
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  [ -f "$GO_LOG" ] && tail -100 "$GO_LOG" >&2 || true
  exit 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat_saas_isolation_check -e "$query" | tr -d '\r'
}

auth_token() {
  local phone="$1"
  local password="$2"
  local out="$3"
  curl -sS -f \
    -H "Content-Type: application/json" \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$out"
  python3 - "$out" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload["code"] == 200, payload
token = payload["data"]["token"]
assert token, payload
print(token)
PY
}

get_json() {
  local token="$1"
  local path="$2"
  local out="$3"
  curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR$path" >"$out"
}

post_json_code() {
  local token="$1"
  local path="$2"
  local body="$3"
  local out="$4"
  curl -sS \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$body" \
    -o "$out" \
    -w '%{http_code}' \
    "http://$GO_ADDR$path"
}

upload_file() {
  local token="$1"
  local path="$2"
  local filename="$3"
  local content="$4"
  local out="$5"
  printf '%s' "$content" >"$path"
  curl -sS -f \
    -X POST \
    -H "Authorization: Bearer $token" \
    -F "file=@$path;type=image/png;filename=$filename" \
    "http://$GO_ADDR/dashboard/common/upload" >"$out"
}

redis_get() {
  local key="$1"
  compose exec -T redis redis-cli GET "$key" | tr -d '\r'
}

redis_set() {
  local key="$1"
  local value="$2"
  compose exec -T redis redis-cli SET "$key" "$value" >/dev/null
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

mysql_root <<'SQL'
DROP DATABASE IF EXISTS mochat_saas_isolation_check;
CREATE DATABASE mochat_saas_isolation_check CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON mochat_saas_isolation_check.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat_saas_isolation_check?parseTime=true&loc=Local"

"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0001_initial_schema\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0002_seed_core_data\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0003_saas_provisioning\tapplied_now' "$WORK_DIR/migrate.out"

cat >"$WORK_DIR/tenants.csv" <<CSV
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,config_copy_mode
$TENANT_A,SaaS隔离租户A,$PHONE_A,$PASSWORD_A,SaaS隔离管理员A,SaaS隔离超级管理员A,isolation-a,隔离A版,3,20,2000,100,5,30,15,12,9,7,6,5,5,5,5,512,50,40,20,10,5,3,1000,missing
$TENANT_B,SaaS隔离租户B,$PHONE_B,$PASSWORD_B,SaaS隔离管理员B,SaaS隔离超级管理员B,isolation-b,隔离B版,3,20,2000,100,5,30,15,12,9,7,6,5,5,5,5,512,50,40,20,10,5,3,1000,missing
CSV

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_A" "$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t'"$TENANT_B" "$WORK_DIR/bootstrap.out"

USER_A="$(mysql_scalar "SELECT id FROM mc_user WHERE tenant_id = $TENANT_A AND phone = '$PHONE_A' AND status = 1 AND isSuperAdmin = 1 AND deleted_at IS NULL")"
USER_B="$(mysql_scalar "SELECT id FROM mc_user WHERE tenant_id = $TENANT_B AND phone = '$PHONE_B' AND status = 1 AND isSuperAdmin = 1 AND deleted_at IS NULL")"
test -n "$USER_A"
test -n "$USER_B"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat_saas_isolation_check <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_A, '隔离企业A', 'ww-isolation-a', 'employee-secret-a', 'contact-secret-a', 'callback-token-a', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_A, NOW(), NOW(), NULL),
  ($CORP_B, '隔离企业B', 'ww-isolation-b', 'employee-secret-b', 'contact-secret-b', 'callback-token-b', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_B, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  tenant_id = VALUES(tenant_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_department
  (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at, deleted_at)
VALUES
  ($DEPARTMENT_A, 1, $CORP_A, '隔离总部A', 0, 0, 100, 1, '#$DEPARTMENT_A#', NOW(), NOW(), NULL),
  ($DEPARTMENT_B, 1, $CORP_B, '隔离总部B', 0, 0, 100, 1, '#$DEPARTMENT_B#', NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  name = VALUES(name),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, main_department_id, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_A, 'isolation-user-a', $CORP_A, '隔离员工A', '$PHONE_A', 1, 1, $USER_A, $DEPARTMENT_A, NOW(), NOW(), NULL),
  ($EMPLOYEE_B, 'isolation-user-b', $CORP_B, '隔离员工B', '$PHONE_B', 1, 1, $USER_B, $DEPARTMENT_B, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  name = VALUES(name),
  mobile = VALUES(mobile),
  log_user_id = VALUES(log_user_id),
  main_department_id = VALUES(main_department_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_employee_department
  (id, employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($EMPLOYEE_A, $EMPLOYEE_A, $DEPARTMENT_A, 0, 1, NOW(), NOW(), NULL),
  ($EMPLOYEE_B, $EMPLOYEE_B, $DEPARTMENT_B, 0, 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  employee_id = VALUES(employee_id),
  department_id = VALUES(department_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_corp_day_data
  (id, corp_id, add_contact_num, add_room_num, add_into_room_num, loss_contact_num, quit_room_num, date, created_at, updated_at)
VALUES
  ($CORP_A, $CORP_A, 7, 2, 3, 1, 1, '$APP_TODAY', NOW(), NOW()),
  ($CORP_B, $CORP_B, 11, 4, 5, 2, 1, '$APP_TODAY', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  add_contact_num = VALUES(add_contact_num),
  add_room_num = VALUES(add_room_num),
  add_into_room_num = VALUES(add_into_room_num),
  loss_contact_num = VALUES(loss_contact_num),
  quit_room_num = VALUES(quit_room_num),
  date = VALUES(date),
  updated_at = NOW();

INSERT INTO mc_work_update_time
  (id, corp_id, type, last_update_time, created_at, updated_at)
VALUES
  ($CORP_A, $CORP_A, 6, '2026-07-02 12:01:00', NOW(), NOW()),
  ($CORP_B, $CORP_B, 6, '2026-07-02 12:02:00', NOW(), NOW()),
  ($((CORP_A + 10)), $CORP_A, 1, '2026-07-02 09:01:00', NOW(), NOW()),
  ($((CORP_B + 10)), $CORP_B, 1, '2026-07-02 09:02:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  type = VALUES(type),
  last_update_time = VALUES(last_update_time),
  updated_at = NOW();
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_FILE_STORAGE_ROOT="$STORAGE_ROOT" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200

TOKEN_A="$(auth_token "$PHONE_A" "$PASSWORD_A" "$WORK_DIR/auth-a.json")"
TOKEN_B="$(auth_token "$PHONE_B" "$PASSWORD_B" "$WORK_DIR/auth-b.json")"

cross_a_code="$(post_json_code "$TOKEN_A" "/dashboard/corp/bind" "{\"corpId\":$CORP_B}" "$WORK_DIR/cross-bind-a.json")"
if [ "$cross_a_code" != "400" ]; then
  echo "tenant A cross bind returned $cross_a_code" >&2
  cat "$WORK_DIR/cross-bind-a.json" >&2
  exit 1
fi
if [ -n "$(redis_get "mc:user.$USER_A")" ]; then
  echo "tenant A cache should not be written after rejected cross bind" >&2
  exit 1
fi

cross_b_code="$(post_json_code "$TOKEN_B" "/dashboard/corp/bind" "{\"corpId\":$CORP_A}" "$WORK_DIR/cross-bind-b.json")"
if [ "$cross_b_code" != "400" ]; then
  echo "tenant B cross bind returned $cross_b_code" >&2
  cat "$WORK_DIR/cross-bind-b.json" >&2
  exit 1
fi
if [ -n "$(redis_get "mc:user.$USER_B")" ]; then
  echo "tenant B cache should not be written after rejected cross bind" >&2
  exit 1
fi

redis_set "mc:user.$USER_A" "$CORP_B-0"
redis_set "mc:user.$USER_B" "$CORP_A-0"
get_json "$TOKEN_A" "/dashboard/user/loginShow" "$WORK_DIR/poison-login-show-a.json"
get_json "$TOKEN_B" "/dashboard/user/loginShow" "$WORK_DIR/poison-login-show-b.json"
get_json "$TOKEN_A" "/dashboard/corpData/index" "$WORK_DIR/poison-corp-data-a.json"
get_json "$TOKEN_B" "/dashboard/corpData/index" "$WORK_DIR/poison-corp-data-b.json"
get_json "$TOKEN_A" "/dashboard/workEmployee/searchCondition" "$WORK_DIR/poison-work-employee-search-a.json"
get_json "$TOKEN_B" "/dashboard/workEmployee/searchCondition" "$WORK_DIR/poison-work-employee-search-b.json"

own_a_code="$(post_json_code "$TOKEN_A" "/dashboard/corp/bind" "{\"corpId\":$CORP_A}" "$WORK_DIR/own-bind-a.json")"
own_b_code="$(post_json_code "$TOKEN_B" "/dashboard/corp/bind" "{\"corpId\":$CORP_B}" "$WORK_DIR/own-bind-b.json")"
test "$own_a_code" = "200"
test "$own_b_code" = "200"
test "$(redis_get "mc:user.$USER_A")" = "$CORP_A-0"
test "$(redis_get "mc:user.$USER_B")" = "$CORP_B-0"

get_json "$TOKEN_A" "/dashboard/corp/select" "$WORK_DIR/corp-select-a.json"
get_json "$TOKEN_B" "/dashboard/corp/select" "$WORK_DIR/corp-select-b.json"
get_json "$TOKEN_A" "/dashboard/corp/select?corpName=隔离企业B" "$WORK_DIR/corp-select-a-search-b.json"
get_json "$TOKEN_A" "/dashboard/role/index?page=1&perPage=10" "$WORK_DIR/role-index-a.json"
get_json "$TOKEN_B" "/dashboard/role/index?page=1&perPage=10" "$WORK_DIR/role-index-b.json"
get_json "$TOKEN_A" "/dashboard/role/permissionByUser" "$WORK_DIR/permission-by-user-a.json"
get_json "$TOKEN_B" "/dashboard/role/permissionByUser" "$WORK_DIR/permission-by-user-b.json"
get_json "$TOKEN_A" "/dashboard/corpData/index" "$WORK_DIR/corp-data-a.json"
get_json "$TOKEN_B" "/dashboard/corpData/index" "$WORK_DIR/corp-data-b.json"
get_json "$TOKEN_A" "/dashboard/workEmployee/searchCondition" "$WORK_DIR/work-employee-search-a.json"
get_json "$TOKEN_B" "/dashboard/workEmployee/searchCondition" "$WORK_DIR/work-employee-search-b.json"
get_json "$TOKEN_A" "/dashboard/workDepartment/index" "$WORK_DIR/work-department-a.json"
get_json "$TOKEN_B" "/dashboard/workDepartment/index" "$WORK_DIR/work-department-b.json"
get_json "$TOKEN_A" "/dashboard/workEmployee/index?page=1&perPage=10" "$WORK_DIR/work-employee-a.json"
get_json "$TOKEN_B" "/dashboard/workEmployee/index?page=1&perPage=10" "$WORK_DIR/work-employee-b.json"
upload_file "$TOKEN_A" "$WORK_DIR/upload-a.png" "tenant-a.png" "tenant-a-upload" "$WORK_DIR/upload-a.json"
upload_file "$TOKEN_B" "$WORK_DIR/upload-b.png" "tenant-b.png" "tenant-b-upload" "$WORK_DIR/upload-b.json"

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_A AND original_name = 'tenant-a.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_A AND original_name = 'tenant-b.png' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_B AND original_name = 'tenant-b.png' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_B AND original_name = 'tenant-a.png' AND deleted_at IS NULL")" = "0"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_A AND metric = 'storage_mb' AND deleted_at IS NULL")" = "1"
test "$(mysql_scalar "SELECT used_value FROM mochat_go_saas_usage_counters WHERE tenant_id = $TENANT_B AND metric = 'storage_mb' AND deleted_at IS NULL")" = "1"

python3 - "$WORK_DIR" "$CORP_A" "$CORP_B" "$EMPLOYEE_A" "$EMPLOYEE_B" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
corp_a = int(sys.argv[2])
corp_b = int(sys.argv[3])
employee_a = int(sys.argv[4])
employee_b = int(sys.argv[5])

def read(name):
    return json.loads((work / name).read_text(encoding="utf-8"))

cross_a = read("cross-bind-a.json")
cross_b = read("cross-bind-b.json")
assert cross_a["code"] == 400, cross_a
assert cross_b["code"] == 400, cross_b

poison_login_show_a = read("poison-login-show-a.json")
poison_login_show_b = read("poison-login-show-b.json")
assert poison_login_show_a["data"]["corpId"] == corp_a, poison_login_show_a
assert poison_login_show_a["data"]["employeeId"] == employee_a, poison_login_show_a
assert poison_login_show_b["data"]["corpId"] == corp_b, poison_login_show_b
assert poison_login_show_b["data"]["employeeId"] == employee_b, poison_login_show_b

poison_corp_data_a = read("poison-corp-data-a.json")
poison_corp_data_b = read("poison-corp-data-b.json")
assert poison_corp_data_a["code"] == 200 and poison_corp_data_a["data"]["addContactNum"] == 7, poison_corp_data_a
assert poison_corp_data_b["code"] == 200 and poison_corp_data_b["data"]["addContactNum"] == 11, poison_corp_data_b

poison_search_a = read("poison-work-employee-search-a.json")
poison_search_b = read("poison-work-employee-search-b.json")
assert poison_search_a["data"]["syncTime"] == "2026-07-02 09:01:00", poison_search_a
assert poison_search_b["data"]["syncTime"] == "2026-07-02 09:02:00", poison_search_b

corp_select_a = read("corp-select-a.json")
corp_select_b = read("corp-select-b.json")
corp_select_a_search_b = read("corp-select-a-search-b.json")
assert corp_select_a["data"] == [{"corpId": corp_a, "corpName": "隔离企业A"}], corp_select_a
assert corp_select_b["data"] == [{"corpId": corp_b, "corpName": "隔离企业B"}], corp_select_b
assert corp_select_a_search_b["data"] == [], corp_select_a_search_b

role_index_a = read("role-index-a.json")
role_index_b = read("role-index-b.json")
role_names_a = [item["name"] for item in role_index_a["data"]["list"]]
role_names_b = [item["name"] for item in role_index_b["data"]["list"]]
assert "SaaS隔离超级管理员A" in role_names_a, role_index_a
assert "SaaS隔离超级管理员B" not in role_names_a, role_index_a
assert "SaaS隔离超级管理员B" in role_names_b, role_index_b
assert "SaaS隔离超级管理员A" not in role_names_b, role_index_b
permission_a = read("permission-by-user-a.json")
permission_b = read("permission-by-user-b.json")
assert permission_a["code"] == 200 and permission_a["data"], permission_a
assert permission_b["code"] == 200 and permission_b["data"], permission_b

corp_data_a = read("corp-data-a.json")
corp_data_b = read("corp-data-b.json")
assert corp_data_a["code"] == 200 and corp_data_a["data"]["addContactNum"] == 7, corp_data_a
assert corp_data_a["data"]["updateTime"] == "2026-07-02 12:01:00", corp_data_a
assert corp_data_b["code"] == 200 and corp_data_b["data"]["addContactNum"] == 11, corp_data_b
assert corp_data_b["data"]["updateTime"] == "2026-07-02 12:02:00", corp_data_b

search_a = read("work-employee-search-a.json")
search_b = read("work-employee-search-b.json")
assert search_a["data"]["syncTime"] == "2026-07-02 09:01:00", search_a
assert search_b["data"]["syncTime"] == "2026-07-02 09:02:00", search_b

department_a = read("work-department-a.json")
department_b = read("work-department-b.json")
assert department_a["data"]["department"][0]["name"] == "隔离总部A", department_a
assert department_b["data"]["department"][0]["name"] == "隔离总部B", department_b
assert department_a["data"]["employee"][0]["employeeId"] == employee_a, department_a
assert department_b["data"]["employee"][0]["employeeId"] == employee_b, department_b

employee_page_a = read("work-employee-a.json")
employee_page_b = read("work-employee-b.json")
assert employee_page_a["data"]["page"]["total"] == 1, employee_page_a
assert employee_page_b["data"]["page"]["total"] == 1, employee_page_b
assert employee_page_a["data"]["list"][0]["name"] == "隔离员工A", employee_page_a
assert employee_page_b["data"]["list"][0]["name"] == "隔离员工B", employee_page_b
upload_a = read("upload-a.json")
upload_b = read("upload-b.json")
assert upload_a["code"] == 200 and upload_a["data"]["name"] == "tenant-a.png", upload_a
assert upload_b["code"] == 200 and upload_b["data"]["name"] == "tenant-b.png", upload_b
print("saas tenant isolation smoke passed")
PY
