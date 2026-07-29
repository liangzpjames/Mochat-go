#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-branding-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13406}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26456}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18180}"
DATABASE="mochat_saas_branding"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-branding.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-branding-jwt-secret}"
IDENTITY_KEY="${MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY:-4242424242424242424242424242424242424242424242424242424242424242}"

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
trap 'echo "SaaS branding smoke failed at line $LINENO" >&2; tail -180 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

wait_service_healthy() {
  local service="$1" deadline=$((SECONDS + 180))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(compose ps --format json "$service" 2>/dev/null | python3 -c 'import json,sys; data=sys.stdin.read().strip(); print(json.loads(data).get("Health", "")) if data else print("")' 2>/dev/null || true)"
    [ "$status" = "healthy" ] && return 0
    sleep 2
  done
  compose ps >&2 || true
  compose logs --tail=120 "$service" >&2 || true
  return 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    [ "$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)" = "$expected" ] && return 0
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
  local phone="$1" password="$2" output="$3"
  curl -sS -f -H 'Content-Type: application/json' -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
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
  test "$status" = "$expected" || { echo "GET $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_write() {
  local method="$1" token="$2" path="$3" body="$4" output="$5" expected="${6:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "$method $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

json_value() {
  local file="$1" expression="$2"
  python3 - "$file" "$expression" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split('.'):
    value = value[int(part)] if part.isdigit() else value[part]
if isinstance(value, bool):
    print(str(value).lower())
else:
    print(value)
PY
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
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_branding_profiles'")" = "mochat_go_saas_branding_profiles"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.branding.read'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.branding.manage'")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,品牌治理平台,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
991,品牌业务租户,13800000991,secret991,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000002', password, '平台运营', 0, '运营部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000003', password, '平台审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
SQL

OPERATIONS_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000002' ORDER BY id DESC LIMIT 1")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000003' ORDER BY id DESC LIMIT 1")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at) VALUES ($OPERATIONS_ID, 1, 1, NOW(), NOW()), ($AUDITOR_ID, 1, 1, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at) VALUES ($OPERATIONS_ID, $OPERATIONS_ROLE_ID, 1, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 1, NOW());
SQL

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
  MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY=1 \
  MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY" \
  MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID=branding-smoke \
  MOCHAT_DASHBOARD_DIST="$PWD/web/apps/dashboard/dist" \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200

PLATFORM_TOKEN="$(login_token 13800000001 secret001 "$WORK_DIR/platform-login.json")"
OPERATIONS_TOKEN="$(login_token 13800000002 secret001 "$WORK_DIR/operations-login.json")"
AUDITOR_TOKEN="$(login_token 13800000003 secret001 "$WORK_DIR/auditor-login.json")"
TENANT_TOKEN="$(login_token 13800000991 secret991 "$WORK_DIR/tenant-login.json")"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all&limit=100" "$WORK_DIR/profiles-default.json"
python3 - "$WORK_DIR/profiles-default.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["license"]["licenseType"] == "GPL-3.0", data
assert len(data["profiles"]) == 2, data
assert all(not item["configured"] and item["version"] == 0 for item in data["profiles"]), data
PY
api_get "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/operations-read.json"
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/auditor-read.json"
api_get "$TENANT_TOKEN" "/dashboard/saasAdmin/brandingProfiles?status=all" "$WORK_DIR/tenant-forbidden.json" 403

PLATFORM_BODY='{"tenantId":1,"status":"active","productName":"品牌控制台","productShortName":"品牌台","productSubtitle":"SaaS 客户运营平台","logoUrl":"/img/logo-no-word.c30823d0.png","faviconUrl":"/favicon.ico","loginBackgroundUrl":"/img/background.e06f03d5.png","primaryColor":"#0F766E","accentColor":"#115E59","websiteUrl":"https://example.com","supportUrl":"/support","supportQrUrl":"/img/logo-no-word.c30823d0.png","supportEmail":"support@example.com","docsUrl":"/docs","footerText":"品牌控制台","expectedVersion":0}'
TENANT_BODY_V0='{"tenantId":991,"status":"active","productName":"客户运营云","productShortName":"客户云","productSubtitle":"企业微信客户运营","logoUrl":"/img/logo-no-word.c30823d0.png","faviconUrl":"/favicon.ico","loginBackgroundUrl":"/img/background.e06f03d5.png","primaryColor":"#1D4ED8","accentColor":"#1E40AF","websiteUrl":"https://tenant.example.com","supportUrl":"/support","supportQrUrl":"/img/logo-no-word.c30823d0.png","supportEmail":"tenant@example.com","docsUrl":"/docs","footerText":"客户运营云","expectedVersion":0}'
api_write POST "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$PLATFORM_BODY" "$WORK_DIR/platform-create.json"
api_write POST "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$TENANT_BODY_V0" "$WORK_DIR/tenant-create.json"
test "$(json_value "$WORK_DIR/platform-create.json" data.profile.version)" = "1"
test "$(json_value "$WORK_DIR/tenant-create.json" data.profile.version)" = "1"

api_write PUT "$AUDITOR_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$TENANT_BODY_V0" "$WORK_DIR/auditor-write-forbidden.json" 403
INVALID_BODY='{"tenantId":991,"status":"active","productName":"客户运营云","logoUrl":"https://oss.mo.chat/logo.png","faviconUrl":"/favicon.ico","loginBackgroundUrl":"/img/background.e06f03d5.png","primaryColor":"#1D4ED8","accentColor":"#1E40AF","expectedVersion":1}'
api_write PUT "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$INVALID_BODY" "$WORK_DIR/external-asset-invalid.json" 400

TENANT_BODY_DISABLED='{"tenantId":991,"status":"disabled","productName":"客户运营云","productShortName":"客户云","productSubtitle":"企业微信客户运营","logoUrl":"/img/logo-no-word.c30823d0.png","faviconUrl":"/favicon.ico","loginBackgroundUrl":"/img/background.e06f03d5.png","primaryColor":"#1D4ED8","accentColor":"#1E40AF","websiteUrl":"https://tenant.example.com","supportUrl":"/support","supportQrUrl":"/img/logo-no-word.c30823d0.png","supportEmail":"tenant@example.com","docsUrl":"/docs","footerText":"客户运营云","expectedVersion":1}'
api_write PUT "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$TENANT_BODY_DISABLED" "$WORK_DIR/tenant-disabled.json"
test "$(json_value "$WORK_DIR/tenant-disabled.json" data.profile.version)" = "2"
api_get "$TENANT_TOKEN" "/dashboard/external/tenantIndex" "$WORK_DIR/external-disabled.json"
python3 - "$WORK_DIR/external-disabled.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["branding"]["tenantId"] == 991 and data["branding"]["productName"] == "MoChat Go", data
assert data["license"]["licenseType"] == "GPL-3.0", data
PY

TENANT_BODY_ACTIVE='{"tenantId":991,"status":"active","productName":"客户运营云","productShortName":"客户云","productSubtitle":"企业微信客户运营 SaaS","logoUrl":"/img/logo-no-word.c30823d0.png","faviconUrl":"/favicon.ico","loginBackgroundUrl":"/img/background.e06f03d5.png","primaryColor":"#1D4ED8","accentColor":"#1E40AF","websiteUrl":"https://tenant.example.com","supportUrl":"/support","supportQrUrl":"/img/logo-no-word.c30823d0.png","supportEmail":"tenant@example.com","docsUrl":"/docs","footerText":"客户运营云","expectedVersion":2}'
api_write PUT "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$TENANT_BODY_ACTIVE" "$WORK_DIR/tenant-active.json"
test "$(json_value "$WORK_DIR/tenant-active.json" data.profile.version)" = "3"
api_write PUT "$OPERATIONS_TOKEN" "/dashboard/saasAdmin/brandingProfile" "$TENANT_BODY_DISABLED" "$WORK_DIR/stale-version.json" 409

api_get "$TENANT_TOKEN" "/dashboard/external/tenantIndex" "$WORK_DIR/external-active.json"
python3 - "$WORK_DIR/external-active.json" <<'PY'
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]
assert data["branding"]["tenantId"] == 991, data
assert data["branding"]["productName"] == "客户运营云", data
assert data["branding"]["supportQrUrl"].startswith("/img/"), data
assert data["license"]["licenseType"] == "GPL-3.0", data
text = json.dumps(data, ensure_ascii=False)
assert "api.mo.chat" not in text and "oss.mo.chat" not in text, text
PY

test "$(mysql_scalar "SELECT product_name FROM mochat_go_saas_branding_profiles WHERE tenant_id = 991")" = "客户运营云"
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_branding_profiles WHERE tenant_id = 991")" = "3"
test "$(mysql_scalar "SELECT logo FROM mc_tenant WHERE id = 991")" = "/img/logo-no-word.c30823d0.png"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.branding.update' AND actor_user_id = $OPERATIONS_ID")" = "4"

curl -sS "http://$GO_ADDR/security/login" >"$WORK_DIR/security-login.html"
grep -q '品牌控制台' "$WORK_DIR/security-login.html"
grep -q -- '--blue:#0F766E' "$WORK_DIR/security-login.html"
grep -q '/img/logo-no-word.c30823d0.png' "$WORK_DIR/security-login.html"
! grep -q 'ZgotmplZ' "$WORK_DIR/security-login.html"

curl -sS "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/admin-page.html"
grep -q 'id="brandingCenter"' "$WORK_DIR/admin-page.html"
grep -q 'platform.branding.manage' "$WORK_DIR/admin-page.html"

curl -sS "http://$GO_ADDR/" >"$WORK_DIR/dashboard.html"
grep -q 'src="/vendor/vue-2.6.10.min.js"' "$WORK_DIR/dashboard.html"
grep -q 'src="/vendor/axios-0.19.0.min.js"' "$WORK_DIR/dashboard.html"
! grep -q 'startupscrmmochat.duoduoker.com' "$WORK_DIR/dashboard.html"
! grep -q 'hm.baidu.com' "$WORK_DIR/dashboard.html"
for asset in vue-2.6.10.min.js vue-router-3.1.3.min.js vuex-3.1.1.min.js axios-0.19.0.min.js; do
  test "$(curl -sS -o /dev/null -w '%{http_code}' "http://$GO_ADDR/vendor/$asset")" = "200"
done
curl -sS "http://$GO_ADDR/js/app.5432e6d2.js" >"$WORK_DIR/dashboard-app.js"
grep -q 'baseURL:"/dashboard"' "$WORK_DIR/dashboard-app.js"
! grep -q 'baseURL:"//api.mo.chat"' "$WORK_DIR/dashboard-app.js"
! grep -q 'oss.mo.chat' "$WORK_DIR/dashboard-app.js"

api_get "$PLATFORM_TOKEN" "/compat/routes" "$WORK_DIR/routes.json"
python3 - "$WORK_DIR/routes.json" <<'PY'
import json, pathlib, sys
payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
routes = set(payload.get("migrated_routes", []))
required = {
    "GET /dashboard/saasAdmin/brandingProfiles",
    "GET /dashboard/saasAdmin/brandingProfile",
    "POST /dashboard/saasAdmin/brandingProfile",
    "PUT /dashboard/saasAdmin/brandingProfile",
}
assert required <= routes, sorted(required - routes)
PY

echo "SaaS branding smoke passed"
