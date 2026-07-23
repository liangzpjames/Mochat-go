#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-wecom-credential-check}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18169}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13369}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26419}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-wecom-credentials.XXXXXX")"
PROBE_DIR="$PWD/.tmp-wecom-credential-probe-$$"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""

TENANT_ID=967
CORP_ID=967001
AGENT_ID=967001
READONLY_USER_ID=967002
ADMIN_PHONE=13800000967
ADMIN_PASSWORD=secret967
JWT_SECRET=wecom-credential-smoke-jwt-secret
KEY_Q1=7171717171717171717171717171717171717171717171717171717171717171
KEY_Q2=7272727272727272727272727272727272727272727272727272727272727272
KEY_ID_Q1=wecom-smoke-q1
KEY_ID_Q2=wecom-smoke-q2

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

stop_go() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  GO_PID=""
}

cleanup() {
  stop_go
  rm -rf "$PROBE_DIR" "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "WeCom credential encryption smoke failed at line $LINENO" >&2; tail -200 "$GO_LOG" >&2 || true' ERR
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
  tail -160 "$GO_LOG" >&2 || true
  exit 1
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$1" | tr -d '\r'
}

login_token() {
  local phone="$1"
  local password="$2"
  local out="$3"
  curl -sS -f -H 'Content-Type: application/json' \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$out"
  jq -er '.data.token' "$out"
}

api_get() {
  local token="$1"
  local path="$2"
  local out="$3"
  curl -sS -f -H "Authorization: Bearer $token" "http://$GO_ADDR$path" >"$out"
}

api_post() {
  local token="$1"
  local path="$2"
  local body="$3"
  local out="$4"
  curl -sS -f -X POST \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -d "$body" \
    "http://$GO_ADDR$path" >"$out"
}

start_go() {
  local key="$1"
  local key_id="$2"
  stop_go
  : >"$GO_LOG"
  env -u GOROOT \
    MOCHAT_GO_STANDALONE=1 \
    MOCHAT_GO_ADDR="$GO_ADDR" \
    MOCHAT_MYSQL_DSN="$DSN" \
    MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
    MOCHAT_SIMPLE_JWT_SECRET="$JWT_SECRET" \
    MOCHAT_GO_MIGRATE_AUTH=1 \
    MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
    MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID="$TENANT_ID" \
    MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY="$key" \
    MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID="$key_id" \
    MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
  grep -q "WeCom credential protection: encryption_configured=true require_encryption=true" "$GO_LOG"
}

run_probe() {
  local mode="$1"
  local key="$2"
  local keys="$3"
  local key_id="$4"
  env -u GOROOT \
    MOCHAT_MYSQL_DSN="$DSN" \
    MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY="$key" \
    MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS="$keys" \
    MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID="$key_id" \
    go run "$PROBE_DIR/main.go" "$mode" "$CORP_ID" "$AGENT_ID"
}

assert_port_free "${GO_ADDR##*:}"
assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"

compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/mochat?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action baseline >"$WORK_DIR/migrate-baseline.out"
grep -q $'0068_wechat_open_credential_encryption\tbaselined' "$WORK_DIR/migrate-baseline.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'mochat' AND table_name = 'mc_corp' AND column_name IN ('wecom_credentials_ciphertext', 'wecom_credentials_key_id')")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'mochat' AND table_name = 'mc_work_agent' AND column_name IN ('wecom_credentials_ciphertext', 'wecom_credentials_key_id')")" = "2"

"$BOOTSTRAP_BIN" \
  -dsn "$DSN" \
  -secret "$JWT_SECRET" \
  -tenant-id "$TENANT_ID" \
  -tenant-name "企业微信凭据验收租户" \
  -phone "$ADMIN_PHONE" \
  -password "$ADMIN_PASSWORD" \
  -user-name "企业微信凭据平台管理员" \
  -role-name "企业微信凭据超管" \
  -package-code "wecom-credential-smoke" \
  -package-name "企业微信凭据验收版" >"$WORK_DIR/bootstrap.out"
ADMIN_USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$ADMIN_USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
DELETE FROM mc_work_agent WHERE id = $AGENT_ID OR wx_agent_id = '1000967';
DELETE FROM mc_corp WHERE id = $CORP_ID OR wx_corpid IN ('ww-wecom-legacy', 'ww-wecom-new');
DELETE FROM mochat_go_saas_admin_user_roles WHERE user_id = $READONLY_USER_ID;
DELETE FROM mochat_go_saas_admin_user_access WHERE user_id = $READONLY_USER_ID;
DELETE FROM mc_user WHERE id = $READONLY_USER_ID;

INSERT INTO mc_user
  (id, phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, deleted_at, isSuperAdmin)
SELECT
  $READONLY_USER_ID, '13800000968', password, '企业微信凭据只读审计员', 0, '', '', NULL, 1, $TENANT_ID, NOW(), NOW(), NULL, 0
FROM mc_user WHERE id = $ADMIN_USER_ID LIMIT 1;
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($READONLY_USER_ID, 1, $ADMIN_USER_ID, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
SELECT $READONLY_USER_ID, id, $ADMIN_USER_ID, NOW()
FROM mochat_go_saas_admin_roles WHERE code = 'platform_readonly';

INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key,
   chat_status, chat_secret, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($CORP_ID, '企业微信旧明文企业', 'ww-wecom-legacy', 'employee-secret-q1', 'contact-secret-q1',
   'callback-token-q1', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, 'chat-secret-q1',
   $TENANT_ID, NOW(), NOW(), NULL);
INSERT INTO mc_work_agent
  (id, corp_id, wx_agent_id, wx_secret, name, square_logo_url, description, close,
   redirect_domain, report_location_flag, is_reportenter, home_url, created_at, updated_at, deleted_at)
VALUES
  ($AGENT_ID, $CORP_ID, '1000967', 'agent-secret-q1', '企业微信旧明文应用', '', '', 0, '', 0, 0, '', NOW(), NOW(), NULL);
SQL

mkdir -p "$PROBE_DIR"
cat >"$PROBE_DIR/main.go" <<'GO'
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/store"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) != 4 {
		panic("usage: probe mode corp-id agent-id")
	}
	corpID, err := strconv.Atoi(os.Args[2])
	must(err)
	agentID, err := strconv.Atoi(os.Args[3])
	must(err)
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"),
		EncryptionKeys: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS"),
		EncryptionKeyID: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID"),
		RequireEncryption: true,
	})
	must(err)
	db, err := mysqlconn.Open(os.Getenv("MOCHAT_MYSQL_DSN"))
	must(err)
	defer db.Close()
	s := store.NewMySQLStore(db).WithWeComCredentialCipher(manager)
	ctx := context.Background()

	if os.Args[1] == "write" {
		must(s.UpdateCorp(ctx, corpID, dashboard.CorpUpdateValues{
			Name: "企业微信密文企业", WxCorpID: "ww-wecom-legacy",
			EmployeeSecret: "employee-secret-q2", ContactSecret: "contact-secret-q2",
		}))
		createdCorpID, err := s.CreateCorp(ctx, dashboard.CorpCreateValues{
			Name: "企业微信新建密文企业", WxCorpID: "ww-wecom-new",
			EmployeeSecret: "employee-secret-new", ContactSecret: "contact-secret-new",
			EventCallback: "/weWork/callback",
			Token: "callback-token-new", EncodingAESKey: "BCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefgh",
			TenantID: 967,
		})
		must(err)
		createdAgentID, err := s.CreateWorkAgent(ctx, dashboard.WorkAgentWriteValues{
			CorpID: createdCorpID, WXAgentID: "1000968", WXSecret: "agent-secret-new",
		}, dashboard.WorkAgentDetail{Name: "企业微信新建密文应用"})
		must(err)
		fmt.Printf("write_ok\t%d\t%d\n", createdCorpID, createdAgentID)
		return
	}

	corp, found, err := s.CorpDetailByID(ctx, corpID)
	must(err)
	if !found || corp.EmployeeSecret == "" || corp.ContactSecret == "" {
		panic("corp credentials missing")
	}
	callback, found, err := s.WeWorkCallbackCorpByID(ctx, corpID)
	must(err)
	if !found || callback.Token != "callback-token-q1" || callback.EncodingAESKey != "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" {
		panic("callback credentials mismatch")
	}
	agent, found, err := s.WorkAgentCredentialByID(ctx, agentID)
	must(err)
	if !found || agent.WXSecret != "agent-secret-q1" {
		panic("agent credentials mismatch")
	}
	corps, err := s.WorkMessageArchiveEnabledCorps(ctx)
	must(err)
	archiveFound := false
	for _, item := range corps {
		if item.CorpID == corpID && item.ChatSecret == "chat-secret-q1" {
			archiveFound = true
		}
	}
	if !archiveFound {
		panic("archive credentials mismatch")
	}
	fmt.Println("read_ok")
}
GO

run_probe read "$KEY_Q1" "" "$KEY_ID_Q1" >"$WORK_DIR/legacy-read.out"
grep -q '^read_ok$' "$WORK_DIR/legacy-read.out"

start_go "$KEY_Q1" "$KEY_ID_Q1"
ADMIN_TOKEN="$(login_token "$ADMIN_PHONE" "$ADMIN_PASSWORD" "$WORK_DIR/admin-auth.json")"
READONLY_TOKEN="$(login_token 13800000968 "$ADMIN_PASSWORD" "$WORK_DIR/readonly-auth.json")"
api_get "$ADMIN_TOKEN" '/dashboard/saasAdmin/wecomCredentialProtection' "$WORK_DIR/protection-legacy.json"
jq -e '.code == 200 and .data.credentialProtection.supported == true and .data.credentialProtection.encryptionConfigured == true and .data.credentialProtection.legacyPlaintextCount >= 2 and .data.credentialProtection.rotationRequiredCount >= 2' "$WORK_DIR/protection-legacy.json" >/dev/null
! grep -q 'employee-secret-q1\|agent-secret-q1\|chat-secret-q1' "$WORK_DIR/protection-legacy.json"

api_get "$READONLY_TOKEN" '/dashboard/saasAdmin/wecomCredentialProtection' "$WORK_DIR/protection-readonly.json"
jq -e '.code == 200 and .data.credentialProtection.supported == true' "$WORK_DIR/protection-readonly.json" >/dev/null
readonly_status="$(curl -sS -o "$WORK_DIR/rotation-readonly.json" -w '%{http_code}' -X POST \
  -H "Authorization: Bearer $READONLY_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"tenantId\":$TENANT_ID,\"limit\":10}" "http://$GO_ADDR/dashboard/saasAdmin/wecomCredentialRotation")"
test "$readonly_status" = "403"
jq -e '.code == 403' "$WORK_DIR/rotation-readonly.json" >/dev/null

api_post "$ADMIN_TOKEN" '/dashboard/saasAdmin/wecomCredentialRotation' \
  "{\"tenantId\":$TENANT_ID,\"limit\":10,\"remark\":\"Q1 存量轮换\"}" "$WORK_DIR/rotation-q1.json"
jq -e --arg key "$KEY_ID_Q1" '.code == 200 and .data.rotation.rotatedCount == 2 and .data.rotation.corpRotatedCount == 1 and .data.rotation.agentRotatedCount == 1 and .data.rotation.legacyCount == 2 and .data.rotation.activeKeyId == $key and .data.operationId > 0' "$WORK_DIR/rotation-q1.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE id = $CORP_ID AND employee_secret = '' AND contact_secret = '' AND token = '' AND encoding_aes_key = '' AND chat_secret = '' AND wecom_credentials_ciphertext <> '' AND wecom_credentials_key_id = '$KEY_ID_Q1'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_agent WHERE id = $AGENT_ID AND wx_secret = '' AND wecom_credentials_ciphertext <> '' AND wecom_credentials_key_id = '$KEY_ID_Q1'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.wecom_credential.rotate' AND target_id = '$TENANT_ID'")" = "1"
run_probe read "$KEY_Q1" "" "$KEY_ID_Q1" >"$WORK_DIR/q1-read.out"
grep -q '^read_ok$' "$WORK_DIR/q1-read.out"

start_go "$KEY_Q2" "$KEY_ID_Q2"
api_get "$ADMIN_TOKEN" '/dashboard/saasAdmin/wecomCredentialProtection' "$WORK_DIR/protection-missing-q1.json"
jq -e --arg key "$KEY_ID_Q1" '.code == 200 and .data.credentialProtection.healthy == false and .data.credentialProtection.unavailableKeyCount >= 1 and (.data.credentialProtection.unavailableKeyIds | index($key)) != null' "$WORK_DIR/protection-missing-q1.json" >/dev/null
api_get "$ADMIN_TOKEN" '/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15' "$WORK_DIR/health-missing-q1.json"
jq -e 'any(.data.checks[]; .code == "wecom_credential_protection" and .status == "critical")' "$WORK_DIR/health-missing-q1.json" >/dev/null
if run_probe read "$KEY_Q2" "" "$KEY_ID_Q2" >"$WORK_DIR/missing-key-read.out" 2>&1; then
  echo "read unexpectedly accepted ciphertext without its historical key" >&2
  exit 1
fi
grep -q "encryption key \"$KEY_ID_Q1\" is unavailable" "$WORK_DIR/missing-key-read.out"
stop_go

MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS="{\"$KEY_ID_Q1\":\"$KEY_Q1\",\"$KEY_ID_Q2\":\"$KEY_Q2\"}" \
MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID="$KEY_ID_Q2" \
MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION=1 \
  "$MAINTENANCE_BIN" \
  -dsn "$DSN" \
  -action rotate-wecom-credentials \
  -tenant-id "$TENANT_ID" \
  -wecom-credential-rotation-limit 10 >"$WORK_DIR/rotation-q2.out"
grep -q $'action\trotate-wecom-credentials' "$WORK_DIR/rotation-q2.out"
grep -q $'rotated\t2' "$WORK_DIR/rotation-q2.out"
grep -q $'legacy_plaintext\t0' "$WORK_DIR/rotation-q2.out"
grep -q $'reencrypted\t2' "$WORK_DIR/rotation-q2.out"
grep -q $'active_key_id\twecom-smoke-q2' "$WORK_DIR/rotation-q2.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE id = $CORP_ID AND wecom_credentials_key_id = '$KEY_ID_Q2'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_agent WHERE id = $AGENT_ID AND wecom_credentials_key_id = '$KEY_ID_Q2'")" = "1"

start_go "$KEY_Q2" "$KEY_ID_Q2"
run_probe read "$KEY_Q2" "" "$KEY_ID_Q2" >"$WORK_DIR/q2-read.out"
grep -q '^read_ok$' "$WORK_DIR/q2-read.out"
api_get "$ADMIN_TOKEN" '/dashboard/saasAdmin/wecomCredentialProtection' "$WORK_DIR/protection-q2.json"
jq -e --arg key "$KEY_ID_Q2" '.code == 200 and .data.credentialProtection.healthy == true and .data.credentialProtection.activeKeyId == $key and .data.credentialProtection.rotationRequiredCount == 0 and .data.credentialProtection.legacyPlaintextCount == 0' "$WORK_DIR/protection-q2.json" >/dev/null
api_get "$ADMIN_TOKEN" '/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15' "$WORK_DIR/health-q2.json"
jq -e 'any(.data.checks[]; .code == "wecom_credential_protection" and .status == "healthy")' "$WORK_DIR/health-q2.json" >/dev/null

run_probe write "$KEY_Q2" "" "$KEY_ID_Q2" >"$WORK_DIR/new-write.out"
grep -q $'^write_ok\t' "$WORK_DIR/new-write.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE wx_corpid = 'ww-wecom-new' AND employee_secret = '' AND contact_secret = '' AND token = '' AND encoding_aes_key = '' AND wecom_credentials_ciphertext <> '' AND wecom_credentials_key_id = '$KEY_ID_Q2'")" = "1"
NEW_CORP_ID="$(mysql_scalar "SELECT id FROM mc_corp WHERE wx_corpid = 'ww-wecom-new' AND deleted_at IS NULL LIMIT 1")"
test -n "$NEW_CORP_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_work_agent WHERE corp_id = $NEW_CORP_ID AND wx_agent_id = '1000968' AND wx_secret = '' AND wecom_credentials_ciphertext <> '' AND wecom_credentials_key_id = '$KEY_ID_Q2'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mc_corp WHERE id = $CORP_ID AND employee_secret = '' AND contact_secret = '' AND token = '' AND encoding_aes_key = '' AND chat_secret = '' AND wecom_credentials_ciphertext <> '' AND wecom_credentials_key_id = '$KEY_ID_Q2'")" = "1"
api_get "$ADMIN_TOKEN" '/dashboard/saasAdmin/wecomCredentialProtection' "$WORK_DIR/protection-final.json"
jq -e '.code == 200 and .data.credentialProtection.healthy == true and .data.credentialProtection.legacyPlaintextCount == 0 and .data.credentialProtection.rotationRequiredCount == 0 and .data.credentialProtection.corpCredentialCount >= 2 and .data.credentialProtection.agentCredentialCount >= 2' "$WORK_DIR/protection-final.json" >/dev/null

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'id="weComCredentialCenter"' "$WORK_DIR/page.html"
grep -q 'id="rotateWeComCredentials"' "$WORK_DIR/page.html"
grep -q "fetch('/dashboard/saasAdmin/wecomCredentialRotation'" "$WORK_DIR/page.html"
grep -q 'GET /dashboard/saasAdmin/wecomCredentialProtection' "$GO_LOG"
grep -q 'POST/PUT /dashboard/saasAdmin/wecomCredentialRotation' "$GO_LOG"

echo "WeCom credential encryption smoke passed"
