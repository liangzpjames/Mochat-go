#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-admin-core-dashboard-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18141}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13344}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26394}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-admin-core.XXXXXX")"
TOKEN_GEN_DIR="$PWD/.tmp-admin-core-token-$$"
FILE_STORAGE_ROOT="$WORK_DIR/static"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-admin-core-dashboard-secret}"
SIDEBAR_SECRET="${MOCHAT_SIDEBAR_JWT_SECRET:-admin-core-sidebar-secret}"
TENANT_ID=711
PHONE=13800000711
PASSWORD=secret711
CORP_ID=711001
ADMIN_EMPLOYEE_ID=711001
NEW_EMPLOYEE_ID=711002
DEPARTMENT_ID=711001
NEW_USER_PHONE=13800000712

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
  rm -rf "$TOKEN_GEN_DIR"
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
  [ -f "$GO_LOG" ] && tail -160 "$GO_LOG" >&2 || true
  exit 1
}

mysql_scalar() {
  local query="$1"
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$query" | tr -d '\r'
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

json_data_field() {
  local file="$1"
  local key="$2"
  python3 - "$file" "$key" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
value = payload["data"]
for part in sys.argv[2].split("."):
    value = value[part]
print(value)
PY
}

assert_port_free "${GO_ADDR##*:}"
assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"

mkdir -p "$FILE_STORAGE_ROOT"

mkdir -p "$TOKEN_GEN_DIR"
cat >"$TOKEN_GEN_DIR/main.go" <<'GO'
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: make_sidebar_token secret employee_id")
	}
	employeeID, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}
	token, _, err := authjwt.MakeToken(authjwt.TokenOptions{
		Secret: os.Args[1],
		UID:    employeeID,
		TTL:    24 * time.Hour,
		Issuer: "admin-core-dashboard-smoke",
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(token)
}
GO

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
grep -q $'0032_saas_sensitive_word_limit\tbaselined' "$WORK_DIR/migrate-baseline.out"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "管理端核心验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "管理端核心管理员" \
  -role-name "管理端核心超管" \
  -package-code "admin-core-standard" \
  -package-name "管理端核心标准版" \
  -max-corps 3 \
  -max-users 20 \
  -max-contacts 500 \
  -max-rooms 200 \
  -max-agents 5 \
  -storage-mb 128 \
  -async-executions 1000 >"$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"
ADMIN_ROLE_ID="$(mysql_scalar "SELECT id FROM mc_rbac_role WHERE tenant_id = $TENANT_ID AND name = '管理端核心超管' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ADMIN_ROLE_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;

DELETE FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID;
DELETE FROM mc_work_employee_department WHERE employee_id IN ($ADMIN_EMPLOYEE_ID, $NEW_EMPLOYEE_ID);
DELETE FROM mc_work_employee WHERE id IN ($ADMIN_EMPLOYEE_ID, $NEW_EMPLOYEE_ID);
DELETE FROM mc_work_department WHERE id = $DEPARTMENT_ID;
DELETE FROM mc_corp WHERE id = $CORP_ID;

UPDATE mc_contact_field
SET deleted_at = NOW(), updated_at = NOW()
WHERE label IN ('验收一', '验收二', '验收三') AND deleted_at IS NULL;
UPDATE mc_medium
SET deleted_at = NOW(), updated_at = NOW()
WHERE corp_id = $CORP_ID AND deleted_at IS NULL;
UPDATE mc_medium_group
SET deleted_at = NOW(), updated_at = NOW()
WHERE corp_id = $CORP_ID AND deleted_at IS NULL;
UPDATE mc_work_room_group
SET deleted_at = NOW(), updated_at = NOW()
WHERE corp_id = $CORP_ID AND deleted_at IS NULL;
UPDATE mc_rbac_role
SET deleted_at = NOW(), updated_at = NOW()
WHERE tenant_id = $TENANT_ID AND name IN ('验收角色', '验收角色改') AND deleted_at IS NULL;
UPDATE mc_rbac_menu
SET deleted_at = NOW(), updated_at = NOW()
WHERE name IN ('验收菜单', '验收菜单改') AND deleted_at IS NULL;

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '管理端核心企业', 'ww-admin-core', 'employee-secret', 'contact-secret', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', $TENANT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_department
  (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at, deleted_at)
VALUES
  ($DEPARTMENT_ID, 1, $CORP_ID, '总部', 0, 0, 1, 1, '#$DEPARTMENT_ID#', NOW(), NOW(), NULL);

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, main_department_id, created_at, updated_at, deleted_at)
VALUES
  ($ADMIN_EMPLOYEE_ID, 'admin-core-admin', $CORP_ID, '管理端核心管理员', '$PHONE', 1, 1, $USER_ID, $DEPARTMENT_ID, NOW(), NOW(), NULL),
  ($NEW_EMPLOYEE_ID, 'admin-core-user', $CORP_ID, '管理端核心成员', '$NEW_USER_PHONE', 1, 1, 0, $DEPARTMENT_ID, NOW(), NOW(), NULL);

INSERT INTO mc_work_employee_department
  (employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($ADMIN_EMPLOYEE_ID, $DEPARTMENT_ID, 1, 1, NOW(), NOW(), NULL),
  ($NEW_EMPLOYEE_ID, $DEPARTMENT_ID, 0, 2, NOW(), NOW(), NULL);
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_API_BASE_URL="http://$GO_ADDR" \
  MOCHAT_DASHBOARD_BASE_URL="http://$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_SIDEBAR_JWT_SECRET="$SIDEBAR_SECRET" \
  MOCHAT_SIDEBAR_JWT_PREFIX=default \
  MOCHAT_FILE_STORAGE_ROOT="$FILE_STORAGE_ROOT" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"

wait_url "http://$GO_ADDR/readyz" 200
grep -q "go migrated route enabled: POST /dashboard/common/uploadFile" "$GO_LOG"
grep -q "go migrated route enabled: POST /sidebar/common/upload" "$GO_LOG"
grep -q "go migrated route enabled: PUT /dashboard/contactField/batchUpdate" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/role/store" "$GO_LOG"
grep -q "go migrated route enabled: POST /dashboard/menu/store" "$GO_LOG"
grep -q "go migrated route enabled: GET /{wxVerifyTxt:WW_verify_" "$GO_LOG"

# Manifest route: GET /{wxVerifyTxt:WW_verify_[0-9a-zA-Z]{16}\.txt$}
test "$(curl -sS -f "http://$GO_ADDR/WW_verify_ABCDEF1234567890.txt")" = "ABCDEF1234567890"

curl -sS -f \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"password\":\"$PASSWORD\"}" \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth.json"
TOKEN="$(json_data_field "$WORK_DIR/auth.json" token)"

api_json POST "/dashboard/corp/bind" "{\"corpId\":$CORP_ID}" "$WORK_DIR/corp-bind.json"

printf 'dashboard upload file\n' >"$WORK_DIR/dashboard-upload.txt"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@$WORK_DIR/dashboard-upload.txt;type=text/plain" \
  -F "name=dashboard-upload.txt" \
  "http://$GO_ADDR/dashboard/common/uploadFile" >"$WORK_DIR/dashboard-upload.json"
assert_api_ok "$WORK_DIR/dashboard-upload.json"
DASHBOARD_UPLOAD_PATH="$(json_data_field "$WORK_DIR/dashboard-upload.json" path)"
test -f "$FILE_STORAGE_ROOT/$DASHBOARD_UPLOAD_PATH"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND source = 'dashboard.common.uploadFile' AND relative_path = '$DASHBOARD_UPLOAD_PATH' AND deleted_at IS NULL")" = "1"

SIDEBAR_TOKEN="$(env -u GOROOT go run "$TOKEN_GEN_DIR/main.go" "$SIDEBAR_SECRET" "$ADMIN_EMPLOYEE_ID")"
printf 'sidebar upload file\n' >"$WORK_DIR/sidebar-upload.txt"
curl -sS -f \
  -H "Authorization: Bearer $SIDEBAR_TOKEN" \
  -F "file=@$WORK_DIR/sidebar-upload.txt;type=text/plain" \
  -F "path=sidebar/admin-core" \
  -F "name=sidebar-upload.txt" \
  "http://$GO_ADDR/sidebar/common/upload" >"$WORK_DIR/sidebar-upload.json"
assert_api_ok "$WORK_DIR/sidebar-upload.json"
test -f "$FILE_STORAGE_ROOT/sidebar/admin-core/sidebar-upload.txt"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_storage_objects WHERE tenant_id = $TENANT_ID AND corp_id = $CORP_ID AND employee_id = $ADMIN_EMPLOYEE_ID AND source = 'sidebar.common.upload' AND relative_path = 'sidebar/admin-core/sidebar-upload.txt' AND deleted_at IS NULL")" = "1"

api_json POST "/dashboard/mediumGroup/store" '{"name":"验收素材组"}' "$WORK_DIR/medium-group-store.json"
MEDIUM_GROUP_ID="$(json_data_field "$WORK_DIR/medium-group-store.json" id)"
api_json PUT "/dashboard/mediumGroup/update" "{\"id\":$MEDIUM_GROUP_ID,\"name\":\"验收素材组改\"}" "$WORK_DIR/medium-group-update.json"
test "$(mysql_scalar "SELECT name FROM mc_medium_group WHERE id = $MEDIUM_GROUP_ID AND corp_id = $CORP_ID AND deleted_at IS NULL")" = "验收素材组改"

api_json POST "/dashboard/medium/store" "{\"type\":1,\"mediumGroupId\":$MEDIUM_GROUP_ID,\"content\":{\"title\":\"验收文本\",\"content\":\"独立 Go 素材\"}}" "$WORK_DIR/medium-store.json"
MEDIUM_ID="$(json_data_field "$WORK_DIR/medium-store.json" id)"
api_json GET "/dashboard/medium/show?id=$MEDIUM_ID" "" "$WORK_DIR/medium-show.json"
api_json PUT "/dashboard/medium/groupUpdate" "{\"id\":$MEDIUM_ID,\"mediumGroupId\":0}" "$WORK_DIR/medium-group-update-item.json"
test "$(mysql_scalar "SELECT medium_group_id FROM mc_medium WHERE id = $MEDIUM_ID AND corp_id = $CORP_ID AND deleted_at IS NULL")" = "0"
api_json DELETE "/dashboard/medium/destroy" "{\"id\":$MEDIUM_ID}" "$WORK_DIR/medium-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_medium WHERE id = $MEDIUM_ID AND deleted_at IS NOT NULL")" = "1"
api_json DELETE "/dashboard/mediumGroup/destroy" "{\"id\":$MEDIUM_GROUP_ID}" "$WORK_DIR/medium-group-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_medium_group WHERE id = $MEDIUM_GROUP_ID AND deleted_at IS NOT NULL")" = "1"

api_json POST "/dashboard/workRoomGroup/store" "{\"corpId\":$CORP_ID,\"workRoomGroupName\":\"验收群分组\"}" "$WORK_DIR/room-group-store.json"
ROOM_GROUP_ID="$(mysql_scalar "SELECT id FROM mc_work_room_group WHERE corp_id = $CORP_ID AND name = '验收群分组' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROOM_GROUP_ID"
api_json PUT "/dashboard/workRoomGroup/update" "{\"workRoomGroupId\":$ROOM_GROUP_ID,\"workRoomGroupName\":\"验收群分组改\"}" "$WORK_DIR/room-group-update.json"
test "$(mysql_scalar "SELECT name FROM mc_work_room_group WHERE id = $ROOM_GROUP_ID AND deleted_at IS NULL")" = "验收群分组改"
api_json DELETE "/dashboard/workRoomGroup/destroy" "{\"workRoomGroupId\":$ROOM_GROUP_ID}" "$WORK_DIR/room-group-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_room_group WHERE id = $ROOM_GROUP_ID AND deleted_at IS NOT NULL")" = "1"

api_json POST "/dashboard/contactField/store" '{"label":"验收一","type":1,"order":10,"status":1,"options":[]}' "$WORK_DIR/contact-field-store-a.json"
FIELD_A_ID="$(mysql_scalar "SELECT id FROM mc_contact_field WHERE label = '验收一' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$FIELD_A_ID"
api_json POST "/dashboard/contactField/store" '{"label":"验收二","type":1,"order":20,"status":1,"options":[]}' "$WORK_DIR/contact-field-store-b.json"
FIELD_B_ID="$(mysql_scalar "SELECT id FROM mc_contact_field WHERE label = '验收二' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$FIELD_B_ID"
api_json PUT "/dashboard/contactField/update" "{\"id\":$FIELD_A_ID,\"label\":\"验收三\",\"type\":1,\"order\":11,\"status\":1,\"options\":[]}" "$WORK_DIR/contact-field-update.json"
api_json PUT "/dashboard/contactField/statusUpdate" "{\"id\":$FIELD_A_ID,\"status\":0}" "$WORK_DIR/contact-field-status.json"
api_json PUT "/dashboard/contactField/batchUpdate" "{\"update\":[{\"id\":$FIELD_A_ID,\"label\":\"验收三\",\"type\":1,\"order\":12,\"status\":1,\"options\":[]}],\"destroy\":[$FIELD_B_ID]}" "$WORK_DIR/contact-field-batch.json"
test "$(mysql_scalar "SELECT CONCAT(label, ':', status, ':', \`order\`) FROM mc_contact_field WHERE id = $FIELD_A_ID AND deleted_at IS NULL")" = "验收三:1:12"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_field WHERE id = $FIELD_B_ID AND deleted_at IS NOT NULL")" = "1"
api_json DELETE "/dashboard/contactField/destroy" "{\"id\":$FIELD_A_ID}" "$WORK_DIR/contact-field-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_contact_field WHERE id = $FIELD_A_ID AND deleted_at IS NOT NULL")" = "1"

api_json POST "/dashboard/menu/store" '{"name":"验收菜单","level":1,"icon":"setting"}' "$WORK_DIR/menu-store.json"
MENU_ID="$(mysql_scalar "SELECT id FROM mc_rbac_menu WHERE name = '验收菜单' AND level = 1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$MENU_ID"
api_json PUT "/dashboard/menu/update" "{\"menuId\":$MENU_ID,\"name\":\"验收菜单改\",\"icon\":\"setting\"}" "$WORK_DIR/menu-update.json"
api_json PUT "/dashboard/menu/statusUpdate" "{\"menuId\":$MENU_ID,\"status\":2}" "$WORK_DIR/menu-status.json"
test "$(mysql_scalar "SELECT CONCAT(name, ':', status) FROM mc_rbac_menu WHERE id = $MENU_ID AND deleted_at IS NULL")" = "验收菜单改:2"

api_json POST "/dashboard/role/store" '{"name":"验收角色","remarks":"验收角色备注","dataPermission":1,"roleId":0}' "$WORK_DIR/role-store.json"
ROLE_ID="$(mysql_scalar "SELECT id FROM mc_rbac_role WHERE tenant_id = $TENANT_ID AND name = '验收角色' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$ROLE_ID"
api_json PUT "/dashboard/role/update" "{\"roleId\":$ROLE_ID,\"name\":\"验收角色改\",\"remarks\":\"验收角色备注改\",\"dataPermission\":2}" "$WORK_DIR/role-update.json"
api_json POST "/dashboard/role/permissionStore" "{\"roleId\":$ROLE_ID,\"menuIds\":[$MENU_ID]}" "$WORK_DIR/role-permission.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_role_menu WHERE role_id = $ROLE_ID AND menu_id = $MENU_ID")" = "1"
api_json PUT "/dashboard/role/statusUpdate" "{\"roleId\":$ROLE_ID,\"status\":2}" "$WORK_DIR/role-status.json"
test "$(mysql_scalar "SELECT CONCAT(name, ':', status) FROM mc_rbac_role WHERE id = $ROLE_ID AND tenant_id = $TENANT_ID AND deleted_at IS NULL")" = "验收角色改:2"
api_json DELETE "/dashboard/role/destroy" "{\"roleId\":$ROLE_ID}" "$WORK_DIR/role-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_role WHERE id = $ROLE_ID AND deleted_at IS NOT NULL")" = "1"
api_json DELETE "/dashboard/menu/destroy" "{\"menuId\":$MENU_ID}" "$WORK_DIR/menu-destroy.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_rbac_menu WHERE id = $MENU_ID AND deleted_at IS NOT NULL")" = "1"

api_json POST "/dashboard/user/store" "{\"userName\":\"管理端核心成员\",\"phone\":\"$NEW_USER_PHONE\",\"gender\":1,\"department\":\"总部\",\"status\":1,\"roleId\":$ADMIN_ROLE_ID,\"password\":\"secret712\"}" "$WORK_DIR/user-store.json"
NEW_USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE tenant_id = $TENANT_ID AND phone = '$NEW_USER_PHONE' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1")"
test -n "$NEW_USER_ID"
api_json GET "/dashboard/user/show?userId=$NEW_USER_ID" "" "$WORK_DIR/user-show.json"
api_json PUT "/dashboard/user/update" "{\"userId\":$NEW_USER_ID,\"userName\":\"管理端核心成员改\",\"phone\":\"$NEW_USER_PHONE\",\"gender\":1,\"department\":\"总部改\",\"status\":1,\"roleId\":$ADMIN_ROLE_ID}" "$WORK_DIR/user-update.json"
api_json PUT "/dashboard/user/statusUpdate" "{\"userId\":[$NEW_USER_ID],\"status\":2}" "$WORK_DIR/user-status.json"
api_json PUT "/dashboard/user/passwordReset" "{\"id\":$NEW_USER_ID,\"newPassword\":\"secret713\"}" "$WORK_DIR/user-password-reset.json"
test "$(mysql_scalar "SELECT CONCAT(name, ':', department, ':', status) FROM mc_user WHERE id = $NEW_USER_ID AND deleted_at IS NULL")" = "管理端核心成员改:总部改:2"
test "$(mysql_scalar "SELECT log_user_id FROM mc_work_employee WHERE id = $NEW_EMPLOYEE_ID AND deleted_at IS NULL")" = "$NEW_USER_ID"

api_json PUT "/dashboard/user/passwordUpdate" "{\"oldPassword\":\"$PASSWORD\",\"newPassword\":\"secret711new\",\"againNewPassword\":\"secret711new\"}" "$WORK_DIR/user-password-update.json"
curl -sS -f \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"password\":\"secret711new\"}" \
  "http://$GO_ADDR/dashboard/user/auth" >"$WORK_DIR/auth-new-password.json"
assert_api_ok "$WORK_DIR/auth-new-password.json"

echo "admin core dashboard smoke passed"
