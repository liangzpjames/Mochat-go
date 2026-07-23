#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-admin-access-rbac-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13387}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26437}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18161}"
DATABASE="mochat_saas_admin_access_rbac"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-admin-access-rbac.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-admin-access-rbac-jwt-secret}"

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
trap 'echo "SaaS admin access RBAC smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
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
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

login_token() {
  local phone="$1"
  local password="$2"
  local output="$3"
  curl -sS -f -H "Content-Type: application/json" -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  python3 - "$output" <<'PY'
import json, pathlib, sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
assert payload.get("code") == 200, payload
print(payload["data"]["token"])
PY
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis
mysql_root <<SQL
DROP DATABASE IF EXISTS $DATABASE;
CREATE DATABASE $DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON $DATABASE.* TO 'mochat'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0045_saas_admin_rbac\tapplied_now' "$WORK_DIR/migrate.out"
grep -q '0046_saas_admin_approvals' "$WORK_DIR/migrate.out"
grep -q '0047_saas_admin_approval_governance' "$WORK_DIR/migrate.out"
grep -q '0049_saas_service_accounts' "$WORK_DIR/migrate.out"
grep -q '0052_saas_compliance_lifecycle' "$WORK_DIR/migrate.out"
grep -q '0064_saas_audit_anchor_remote_immutability' "$WORK_DIR/migrate.out"
grep -q '0067_wecom_credential_encryption' "$WORK_DIR/migrate.out"
grep -q '0068_wechat_open_credential_encryption' "$WORK_DIR/migrate.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_schema_migrations")" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_roles WHERE is_system = 1 AND status = 1")" = "6"
test "$(mysql_scalar "SELECT COUNT(DISTINCT permission_code) FROM mochat_go_saas_admin_role_permissions")" = "31"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,权限治理平台,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
951,权限隔离租户,13800000951,secret951,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000002', password, '平台财务', 0, '财务部', '财务', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000003', password, '平台运营', 0, '运营部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000004', password, '平台审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000005', password, '平台安全', 0, '安全部', '权限管理员', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
SQL

FINANCE_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000002' ORDER BY id DESC LIMIT 1")"
OPERATIONS_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000003' ORDER BY id DESC LIMIT 1")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000004' ORDER BY id DESC LIMIT 1")"
SECURITY_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000005' ORDER BY id DESC LIMIT 1")"
TENANT_USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000951' ORDER BY id DESC LIMIT 1")"
FINANCE_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_finance'")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-auth.json")"
FINANCE_TOKEN="$(login_token 13800000002 secret001 "$WORK_DIR/finance-auth.json")"
OPERATIONS_TOKEN="$(login_token 13800000003 secret001 "$WORK_DIR/operations-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000004 secret001 "$WORK_DIR/auditor-auth.json")"
SECURITY_TOKEN="$(login_token 13800000005 secret001 "$WORK_DIR/security-auth.json")"
TENANT_TOKEN="$(login_token 13800000951 secret951 "$WORK_DIR/tenant-auth.json")"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessProfile" "$WORK_DIR/platform-profile.json"
python3 - "$WORK_DIR/platform-profile.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["profile"]["isPlatformSuperAdmin"] is True, data
assert data["profile"]["permissions"] == ["*"], data
assert len(data["permissions"]) == 32, data
PY
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRoles" "$WORK_DIR/roles.json"
python3 - "$WORK_DIR/roles.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert len(data["roles"]) == 6 and all(role["isSystem"] for role in data["roles"]), data
assert any(role["code"] == "platform_finance" and "platform.finance.manage" in role["permissions"] for role in data["roles"]), data
PY
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/accessRoles" "$WORK_DIR/tenant-access-forbidden.json" 403
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/overview" "$WORK_DIR/unassigned-forbidden.json" 403

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$FINANCE_ID,\"roleIds\":[$FINANCE_ROLE_ID],\"expectedVersion\":0}" "$WORK_DIR/assign-finance.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$OPERATIONS_ID,\"roleIds\":[$OPERATIONS_ROLE_ID],\"expectedVersion\":0}" "$WORK_DIR/assign-operations.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$AUDITOR_ID,\"roleIds\":[$AUDITOR_ROLE_ID],\"expectedVersion\":0}" "$WORK_DIR/assign-auditor.json"

api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/accessProfile" "$WORK_DIR/finance-profile.json"
python3 - "$WORK_DIR/finance-profile.json" <<'PY'
import json, pathlib, sys
profile = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["profile"]
assert profile["isPlatformSuperAdmin"] is False and [role["code"] for role in profile["roles"]] == ["platform_finance"], profile
assert "platform.finance.read" in profile["permissions"] and "platform.finance.manage" in profile["permissions"], profile
assert "platform.operations.manage" not in profile["permissions"], profile
PY
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/paymentOrders?limit=5" "$WORK_DIR/finance-payment-orders.json"
api_post "$FINANCE_TOKEN" "/dashboard/saasAdmin/paymentOrder" '{}' "$WORK_DIR/finance-payment-write-validation.json" 400
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/operationQueue?limit=5" "$WORK_DIR/finance-operation-forbidden.json" 403
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/operations?limit=5" "$WORK_DIR/finance-audit.json"

api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/operationQueue?limit=5" "$WORK_DIR/operations-queue.json"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/operationQueueAssign" '{}' "$WORK_DIR/operations-write-validation.json" 400
api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/paymentOrders?limit=5" "$WORK_DIR/operations-finance-read.json"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/paymentOrder" '{}' "$WORK_DIR/operations-finance-write-forbidden.json" 403
api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/backupOverview?limit=5" "$WORK_DIR/operations-backup-overview.json"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/backupRun" '{}' "$WORK_DIR/operations-backup-write-validation.json" 400
api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/complianceOverview?limit=5" "$WORK_DIR/operations-compliance-overview.json"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/complianceExport" '{}' "$WORK_DIR/operations-compliance-write-validation.json" 400
api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/operations-branding-read.json"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" '{}' "$WORK_DIR/operations-branding-write-validation.json" 400
api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/tenantDomains?status=all" "$WORK_DIR/operations-domains-read.json"
api_post "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/tenantDomain" '{}' "$WORK_DIR/operations-domains-write-validation.json" 400

api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/operations?limit=5" "$WORK_DIR/auditor-operations.json"
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/paymentOrders?limit=5" "$WORK_DIR/auditor-finance-read.json"
api_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/paymentOrder" '{}' "$WORK_DIR/auditor-write-forbidden.json" 403
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/accessRoles" "$WORK_DIR/auditor-access-forbidden.json" 403
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/backupOverview?limit=5" "$WORK_DIR/auditor-backup-overview.json"
api_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/backupRun" '{}' "$WORK_DIR/auditor-backup-write-forbidden.json" 403
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/complianceOverview?limit=5" "$WORK_DIR/auditor-compliance-overview.json"
api_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/complianceExport" '{}' "$WORK_DIR/auditor-compliance-write-forbidden.json" 403
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/auditor-branding-read.json"
api_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/brandingProfile" '{}' "$WORK_DIR/auditor-branding-write-forbidden.json" 403
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/tenantDomains?status=all" "$WORK_DIR/auditor-domains-read.json"
api_post "$AUDITOR_TOKEN" "/dashboard/saasAdmin/tenantDomain" '{}' "$WORK_DIR/auditor-domains-write-forbidden.json" 403
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/backupOverview?limit=5" "$WORK_DIR/finance-backup-forbidden.json" 403
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/complianceOverview?limit=5" "$WORK_DIR/finance-compliance-forbidden.json" 403
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/finance-branding-forbidden.json" 403
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/tenantDomains?status=all" "$WORK_DIR/finance-domains-forbidden.json" 403
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/backupOverview?limit=5" "$WORK_DIR/tenant-backup-forbidden.json" 403
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/tenant-branding-forbidden.json" 403
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/tenantDomains?status=all" "$WORK_DIR/tenant-domains-forbidden.json" 403

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" '{"code":"security_admin","name":"权限管理员","description":"平台权限治理","status":1,"permissions":["platform.overview.read","platform.audit.read","platform.access.manage"]}' "$WORK_DIR/create-security-role.json"
SECURITY_ROLE_ID="$(python3 - "$WORK_DIR/create-security-role.json" <<'PY'
import json, pathlib, sys
print(json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["role"]["id"])
PY
)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$SECURITY_ID,\"roleIds\":[$SECURITY_ROLE_ID],\"expectedVersion\":0}" "$WORK_DIR/assign-security.json"
api_get "$SECURITY_TOKEN" "/dashboard/saasAdmin/accessRoles" "$WORK_DIR/security-roles.json"
api_get "$SECURITY_TOKEN" "/dashboard/saasAdmin/accessAssignments?limit=20" "$WORK_DIR/security-assignments.json"
api_post "$SECURITY_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$SECURITY_ID,\"roleIds\":[$SECURITY_ROLE_ID],\"expectedVersion\":1}" "$WORK_DIR/security-self-assignment.json" 400
api_post "$SECURITY_TOKEN" "/dashboard/saasAdmin/accessRole" "{\"id\":$SECURITY_ROLE_ID,\"code\":\"security_admin\",\"name\":\"权限管理员\",\"description\":\"自改\",\"status\":1,\"expectedVersion\":1,\"permissions\":[\"platform.overview.read\",\"platform.audit.read\",\"platform.access.manage\"]}" "$WORK_DIR/security-self-role.json" 400

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" '{"code":"bad_finance","name":"错误财务岗位","status":1,"permissions":["platform.finance.manage"]}' "$WORK_DIR/invalid-role-dependency.json" 400
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" '{"code":"bad_backup","name":"错误备份岗位","status":1,"permissions":["platform.backups.manage"]}' "$WORK_DIR/invalid-backup-role-dependency.json" 400
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" '{"code":"bad_compliance","name":"错误合规岗位","status":1,"permissions":["platform.compliance.manage","platform.compliance.read","platform.audit.read"]}' "$WORK_DIR/invalid-compliance-role-dependency.json" 400
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" '{"code":"bad_branding","name":"错误品牌岗位","status":1,"permissions":["platform.branding.manage","platform.branding.read"]}' "$WORK_DIR/invalid-branding-role-dependency.json" 400
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" '{"code":"bad_domains","name":"错误域名岗位","status":1,"permissions":["platform.domains.manage","platform.domains.read"]}' "$WORK_DIR/invalid-domains-role-dependency.json" 400
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" "{\"id\":$FINANCE_ROLE_ID,\"code\":\"platform_finance\",\"name\":\"平台财务\",\"status\":1,\"expectedVersion\":1,\"permissions\":[\"platform.overview.read\",\"platform.tenants.read\",\"platform.finance.read\",\"platform.finance.manage\",\"platform.audit.read\"]}" "$WORK_DIR/system-role-protected.json" 409
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$TENANT_USER_ID,\"roleIds\":[$FINANCE_ROLE_ID],\"expectedVersion\":0}" "$WORK_DIR/cross-tenant-assignment.json" 403
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$FINANCE_ID,\"roleIds\":[],\"expectedVersion\":0}" "$WORK_DIR/assignment-version-conflict.json" 409

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$FINANCE_ID,\"roleIds\":[],\"expectedVersion\":1}" "$WORK_DIR/revoke-finance.json"
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/paymentOrders?limit=5" "$WORK_DIR/revoked-finance-forbidden.json" 403
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignment" "{\"userId\":$FINANCE_ID,\"roleIds\":[$FINANCE_ROLE_ID],\"expectedVersion\":2}" "$WORK_DIR/restore-finance.json"
api_get "$FINANCE_TOKEN" "/dashboard/saasAdmin/paymentOrders?limit=5" "$WORK_DIR/restored-finance.json"

api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" "{\"id\":$SECURITY_ROLE_ID,\"code\":\"security_admin\",\"name\":\"权限治理管理员\",\"description\":\"平台权限治理\",\"status\":1,\"expectedVersion\":1,\"permissions\":[\"platform.overview.read\",\"platform.audit.read\",\"platform.access.manage\"]}" "$WORK_DIR/update-security-role.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessRole" "{\"id\":$SECURITY_ROLE_ID,\"code\":\"security_admin\",\"name\":\"过期更新\",\"description\":\"版本冲突\",\"status\":1,\"expectedVersion\":1,\"permissions\":[\"platform.overview.read\",\"platform.audit.read\",\"platform.access.manage\"]}" "$WORK_DIR/role-version-conflict.json" 409

test "$(mysql_scalar "SELECT version FROM mochat_go_saas_admin_user_access WHERE user_id = $FINANCE_ID")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_user_roles WHERE user_id = $FINANCE_ID AND role_id = $FINANCE_ROLE_ID")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.access.role.save'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.access.assignment.save'")" = "6"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/accessAssignments?keyword=%E5%B9%B3%E5%8F%B0&limit=20" "$WORK_DIR/assignments-final.json"
python3 - "$WORK_DIR/assignments-final.json" <<'PY'
import json, pathlib, sys
items = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["assignments"]
assert any(item["isSuperAdmin"] and item["permissions"] == ["*"] for item in items), items
assert any(item["userName"] == "平台财务" and item["version"] == 3 and [role["code"] for role in item["roles"]] == ["platform_finance"] for item in items), items
PY

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'aria-label="平台权限治理"' "$WORK_DIR/page.html"
grep -q 'id="accessPermissionOptions"' "$WORK_DIR/page.html"
grep -q 'function loadAccessGovernance()' "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/accessProfile' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/accessAssignment' "$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/backupOverview' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/restoreDrill' "$WORK_DIR/routes.json"

echo "SaaS admin access RBAC smoke passed"
