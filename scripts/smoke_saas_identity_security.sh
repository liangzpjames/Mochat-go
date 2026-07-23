#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-identity-security-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13403}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26453}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18177}"
DATABASE="mochat_saas_identity_security"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-identity-security.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
JWT_SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-identity-security-jwt-secret}"
IDENTITY_KEY="${MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY:-5353535353535353535353535353535353535353535353535353535353535353}"

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
trap 'echo "SaaS identity security smoke failed at line $LINENO" >&2; tail -200 "$GO_LOG" >&2 || true' ERR
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
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)" = "$expected" ] && return 0
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

login_request() {
  local phone="$1" password="$2" output="$3" expected="$4" forwarded_ip="${5:-}"
  local args=(-sS -o "$output" -w '%{http_code}' -H 'Content-Type: application/json')
  [ -n "$forwarded_ip" ] && args+=(-H "X-Forwarded-For: $forwarded_ip")
  local status
  status="$(curl "${args[@]}" -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" "http://$GO_ADDR/dashboard/user/auth")"
  test "$status" = "$expected" || { echo "login $phone returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_post() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_put() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}"
  local status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X PUT -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "PUT $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

json_value() {
  local file="$1" expression="$2"
  python3 - "$file" "$expression" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split('.'):
    value = value[int(part)] if isinstance(value, list) else value[part]
if isinstance(value, bool):
    print(str(value).lower())
elif isinstance(value, (dict, list)):
    print(json.dumps(value, ensure_ascii=False, separators=(",", ":")))
else:
    print(value)
PY
}

totp_code() {
  local secret="$1" offset="${2:-0}"
  python3 - "$secret" "$offset" <<'PY'
import base64, hashlib, hmac, struct, sys, time
secret = sys.argv[1].strip().upper()
secret += "=" * ((8 - len(secret) % 8) % 8)
counter = int(time.time() // 30) + int(sys.argv[2])
digest = hmac.new(base64.b32decode(secret), struct.pack(">Q", counter), hashlib.sha1).digest()
pos = digest[-1] & 0x0f
value = (struct.unpack(">I", digest[pos:pos+4])[0] & 0x7fffffff) % 1000000
print(f"{value:06d}")
PY
}

identity_policy_payload() {
  local expected="$1" max_failed="$2" max_sessions="$3" require_mfa="$4" cidrs="$5"
  printf '{"tenantId":981,"status":"active","maxFailedAttempts":%s,"lockoutMinutes":5,"sessionTtlMinutes":120,"idleTimeoutMinutes":60,"maxConcurrentSessions":%s,"requireMfa":%s,"allowedIpCidrs":%s,"loginEventRetentionDays":30,"sessionRetentionDays":7,"expectedVersion":%s}' \
    "$max_failed" "$max_sessions" "$require_mfa" "$cidrs" "$expected"
}

approve() {
  local token="$1" approval_id="$2" version="$3" reason="$4" output="$5"
  api_post "$token" "/dashboard/saasAdmin/approvalDecision" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$version,\"decision\":\"approve\",\"reason\":\"$reason\"}" "$output"
}

update_policy() {
  local expected="$1" max_failed="$2" max_sessions="$3" require_mfa="$4" cidrs="$5" output="$6"
  local payload request_body approval_id approval_version prefix expected_target_name
  payload="$(identity_policy_payload "$expected" "$max_failed" "$max_sessions" "$require_mfa" "$cidrs")"
  prefix="policy-v$expected"
  expected_target_name="身份安全租户"
  [ "$expected" = "0" ] && expected_target_name="租户 981 身份安全策略"
  request_body="$(jq -cn --argjson payload "$payload" --arg key "identity-policy-v$expected" \
    '{actionType:"identity.policy.update",payload:$payload,reason:"复核租户身份安全策略变更",idempotencyKey:$key}')"
  api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$request_body" "$WORK_DIR/$prefix-request.json"
  approval_id="$(json_value "$WORK_DIR/$prefix-request.json" data.approval.id)"
  approval_version="$(json_value "$WORK_DIR/$prefix-request.json" data.approval.version)"
  test "$(json_value "$WORK_DIR/$prefix-request.json" data.approval.actionType)" = "identity.policy.update"
  test "$(json_value "$WORK_DIR/$prefix-request.json" data.approval.requiredApprovals)" = "2"
  test "$(json_value "$WORK_DIR/$prefix-request.json" data.approval.targetId)" = "981"
  test "$(json_value "$WORK_DIR/$prefix-request.json" data.approval.targetName)" = "$expected_target_name"
  test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.expectedVersion')) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id")" = "$expected"
  approve "$APPROVER1_TOKEN" "$approval_id" "$approval_version" "第一复核人确认身份策略变更" "$WORK_DIR/$prefix-vote-one.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-one.json" data.approval.version)"
  approve "$APPROVER2_TOKEN" "$approval_id" "$approval_version" "第二复核人确认身份策略变更" "$WORK_DIR/$prefix-vote-two.json"
  approval_version="$(json_value "$WORK_DIR/$prefix-vote-two.json" data.approval.version)"
  test "$(json_value "$WORK_DIR/$prefix-vote-two.json" data.approval.status)" = "approved"
  api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
    "{\"approvalId\":$approval_id,\"expectedVersion\":$approval_version}" "$output"
  test "$(json_value "$output" data.result.policy.version)" = "$((expected + 1))"
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $approval_id AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
  test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = (SELECT effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id = $approval_id) AND action = 'saas.admin.identity.policy.update'")" = "1"
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
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go
DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0076_saas_identity_policy_change_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0077_saas_tenant_disable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0079_saas_service_account_key_revoke_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0080_saas_service_account_update_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0081_saas_service_account_key_rotate_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0082_saas_service_account_create_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0083_saas_identity_mfa_reset_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies')" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'identity.policy.update' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'identity.mfa.reset' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.identity.read'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.identity.manage'")" = "1"

cat >"$WORK_DIR/tenants.csv" <<'CSV'
tenant_id,tenant_name,phone,password,user_name,role_name,package_code,package_name,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions,package_expires_at,config_copy_mode
1,身份安全平台,13800000001,secret001,平台超级管理员,SaaS平台超级管理员,platform,平台版,10,100,100000,2000,20,500,300,300,300,300,300,300,300,300,300,300,300,300,4096,300,300,300,300,300,50,100000,2037-01-01,missing
981,身份安全租户,13800000981,secret981,租户管理员,SaaS租户超级管理员,starter,基础版,1,5,500,20,2,10,10,10,10,10,10,10,10,10,10,10,10,10,256,10,10,10,10,10,1,500,2037-01-01,missing
CSV
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$JWT_SECRET" -batch-file "$WORK_DIR/tenants.csv" >"$WORK_DIR/bootstrap.out"
TENANT_USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000981' LIMIT 1")"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000002', password, '身份安全运营', 0, '安全部', '身份运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000003', password, '身份复核人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000004', password, '身份复核人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000001' LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000002'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000003'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000004'")"
OPERATOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $OPERATOR_ROLE_ID, 0, NOW()), ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());
SQL

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$JWT_SECRET" \
  MOCHAT_SIMPLE_JWT_TTL_SECONDS=7200 \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_MIGRATE_LOGOUT=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY=1 \
  MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS=1 \
  MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS=1 \
  MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS=127.0.0.1/32 \
  MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY" \
  MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID=identity-smoke \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200
test "$(curl -s -o "$WORK_DIR/security-login.html" -w '%{http_code}' "http://$GO_ADDR/security/login")" = "200"
grep -q 'id="password-form"' "$WORK_DIR/security-login.html" || { echo "unexpected /security/login response:" >&2; head -40 "$WORK_DIR/security-login.html" >&2; false; }
grep -q 'id="mfa-form"' "$WORK_DIR/security-login.html" || { echo "missing MFA form in /security/login response" >&2; false; }

login_request 13800000001 secret001 "$WORK_DIR/platform-login.json" 200
PLATFORM_TOKEN="$(json_value "$WORK_DIR/platform-login.json" data.token)"
login_request 13800000981 secret981 "$WORK_DIR/tenant-login.json" 200
TENANT_TOKEN="$(json_value "$WORK_DIR/tenant-login.json" data.token)"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_sessions WHERE status = 'active'")" = "2"

login_request 13800000002 secret001 "$WORK_DIR/operator-login.json" 200
OPERATOR_TOKEN="$(json_value "$WORK_DIR/operator-login.json" data.token)"
login_request 13800000003 secret001 "$WORK_DIR/approver1-login.json" 200
APPROVER1_TOKEN="$(json_value "$WORK_DIR/approver1-login.json" data.token)"
login_request 13800000004 secret001 "$WORK_DIR/approver2-login.json" 200
APPROVER2_TOKEN="$(json_value "$WORK_DIR/approver2-login.json" data.token)"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityOverview?tenantId=981&limit=20" "$WORK_DIR/overview-initial.json"
test "$(json_value "$WORK_DIR/overview-initial.json" data.config.sessionEnforced)" = "true"
api_get "$TENANT_TOKEN" "/dashboard/user/securityMFA" "$WORK_DIR/self-initial.json"

INITIAL_POLICY_PAYLOAD="$(identity_policy_payload 0 3 5 false '[]')"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityPolicy" "$INITIAL_POLICY_PAYLOAD" "$WORK_DIR/policy-direct-blocked.json" 428
test "$(json_value "$WORK_DIR/policy-direct-blocked.json" data.actionType)" = "identity.policy.update"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_policies WHERE tenant_id = 981")" = "0"
update_policy 0 3 5 false '[]' "$WORK_DIR/policy-lockout.json"
test "$(json_value "$WORK_DIR/policy-lockout.json" data.result.policy.version)" = "1"
login_request 13800000981 wrong-password "$WORK_DIR/bad-1.json" 401
login_request 13800000981 wrong-password "$WORK_DIR/bad-2.json" 401
login_request 13800000981 wrong-password "$WORK_DIR/bad-3.json" 401
login_request 13800000981 secret981 "$WORK_DIR/locked.json" 403
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityOverview?tenantId=981&limit=20" "$WORK_DIR/overview-locked.json"
LOCK_VERSION="$(json_value "$WORK_DIR/overview-locked.json" data.users.0.version)"
test "$(json_value "$WORK_DIR/overview-locked.json" data.summary.lockedUsers)" = "1"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityUser" "{\"userId\":$TENANT_USER_ID,\"expectedVersion\":$LOCK_VERSION,\"reason\":\"smoke unlock\"}" "$WORK_DIR/unlock.json"
login_request 13800000981 secret981 "$WORK_DIR/unlocked-login.json" 200
TENANT_TOKEN="$(json_value "$WORK_DIR/unlocked-login.json" data.token)"

update_policy 1 3 5 false '["10.10.0.0/16"]' "$WORK_DIR/policy-ip.json"
login_request 13800000981 secret981 "$WORK_DIR/ip-denied.json" 403 192.168.10.4
login_request 13800000981 secret981 "$WORK_DIR/ip-allowed.json" 200 10.10.8.9
TENANT_TOKEN="$(json_value "$WORK_DIR/ip-allowed.json" data.token)"
update_policy 2 3 5 false '[]' "$WORK_DIR/policy-open-ip.json"

api_post "$TENANT_TOKEN" "/dashboard/user/securityMFA" '{"action":"begin"}' "$WORK_DIR/mfa-begin.json" 201
MFA_SECRET="$(json_value "$WORK_DIR/mfa-begin.json" data.enrollment.secret)"
MFA_VERSION="$(json_value "$WORK_DIR/mfa-begin.json" data.enrollment.version)"
RECOVERY_1="$(json_value "$WORK_DIR/mfa-begin.json" data.enrollment.recoveryCodes.0)"
RECOVERY_2="$(json_value "$WORK_DIR/mfa-begin.json" data.enrollment.recoveryCodes.1)"
RECOVERY_3="$(json_value "$WORK_DIR/mfa-begin.json" data.enrollment.recoveryCodes.2)"
RECOVERY_4="$(json_value "$WORK_DIR/mfa-begin.json" data.enrollment.recoveryCodes.3)"
ENROLL_CODE="$(totp_code "$MFA_SECRET" 0)"
api_post "$TENANT_TOKEN" "/dashboard/user/securityMFA" "{\"action\":\"verify\",\"code\":\"$ENROLL_CODE\",\"expectedVersion\":$MFA_VERSION}" "$WORK_DIR/mfa-verify.json"
test "$(json_value "$WORK_DIR/mfa-verify.json" data.mfa.status)" = "active"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_mfa_credentials WHERE user_id = $TENANT_USER_ID AND status = 'active' AND secret_ciphertext NOT LIKE '%$MFA_SECRET%' AND recovery_codes_remaining = 8")" = "1"

login_request 13800000981 secret981 "$WORK_DIR/mfa-challenge-1.json" 202
CHALLENGE_1="$(json_value "$WORK_DIR/mfa-challenge-1.json" data.challengeToken)"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_1\",\"code\":\"000000\"}" "$WORK_DIR/mfa-invalid.json" 401
LOGIN_CODE="$(totp_code "$MFA_SECRET" 1)"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_1\",\"code\":\"$LOGIN_CODE\"}" "$WORK_DIR/mfa-login.json"
MFA_TOKEN="$(json_value "$WORK_DIR/mfa-login.json" data.token)"
test "$(json_value "$WORK_DIR/mfa-login.json" data.authMethod)" = "password+totp"

login_request 13800000981 secret981 "$WORK_DIR/mfa-challenge-2.json" 202
CHALLENGE_2="$(json_value "$WORK_DIR/mfa-challenge-2.json" data.challengeToken)"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_2\",\"code\":\"$LOGIN_CODE\"}" "$WORK_DIR/mfa-replay.json" 401
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_2\",\"code\":\"$RECOVERY_1\"}" "$WORK_DIR/recovery-login.json"
test "$(json_value "$WORK_DIR/recovery-login.json" data.authMethod)" = "password+recovery"
test "$(mysql_scalar "SELECT recovery_codes_remaining FROM mochat_go_saas_identity_mfa_credentials WHERE user_id = $TENANT_USER_ID")" = "7"

login_request 13800000981 secret981 "$WORK_DIR/mfa-challenge-3.json" 202
CHALLENGE_3="$(json_value "$WORK_DIR/mfa-challenge-3.json" data.challengeToken)"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_3\",\"code\":\"$RECOVERY_1\"}" "$WORK_DIR/recovery-replay.json" 401

update_policy 3 3 2 true '[]' "$WORK_DIR/policy-mfa-concurrency.json"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_3\",\"code\":\"$RECOVERY_2\"}" "$WORK_DIR/session-a.json"
TOKEN_A="$(json_value "$WORK_DIR/session-a.json" data.token)"
SESSION_A_ID="$(json_value "$WORK_DIR/session-a.json" data.session.id)"
SESSION_A_VERSION="$(json_value "$WORK_DIR/session-a.json" data.session.version)"
login_request 13800000981 secret981 "$WORK_DIR/mfa-challenge-4.json" 202
CHALLENGE_4="$(json_value "$WORK_DIR/mfa-challenge-4.json" data.challengeToken)"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$CHALLENGE_4\",\"code\":\"$RECOVERY_3\"}" "$WORK_DIR/session-b.json"
TOKEN_B="$(json_value "$WORK_DIR/session-b.json" data.token)"
api_get "$TENANT_TOKEN" "/dashboard/user/securityMFA" "$WORK_DIR/old-session-revoked.json" 401
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_sessions WHERE user_id = $TENANT_USER_ID AND status = 'active'")" = "2"

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identitySessions?tenantId=981&status=active&limit=20" "$WORK_DIR/sessions-active.json"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identitySession" "{\"sessionId\":$SESSION_A_ID,\"expectedVersion\":$SESSION_A_VERSION,\"reason\":\"smoke revoke\"}" "$WORK_DIR/revoke-a.json"
api_get "$TOKEN_A" "/dashboard/user/securityMFA" "$WORK_DIR/token-a-revoked.json" 401
api_put "$TOKEN_B" "/dashboard/user/logout" '{}' "$WORK_DIR/logout-b.json"
api_get "$TOKEN_B" "/dashboard/user/securityMFA" "$WORK_DIR/token-b-logout.json" 401

login_request 13800000981 secret981 "$WORK_DIR/mfa-reset-challenge.json" 202
RESET_CHALLENGE="$(json_value "$WORK_DIR/mfa-reset-challenge.json" data.challengeToken)"
api_post "" "/dashboard/user/authMFA" "{\"challengeToken\":\"$RESET_CHALLENGE\",\"code\":\"$RECOVERY_4\"}" "$WORK_DIR/mfa-reset-session.json"
RESET_TOKEN="$(json_value "$WORK_DIR/mfa-reset-session.json" data.token)"
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityOverview?tenantId=981&limit=20" "$WORK_DIR/overview-before-mfa-reset.json"
RESET_MFA_VERSION="$(json_value "$WORK_DIR/overview-before-mfa-reset.json" data.users.0.mfaVersion)"
RESET_PAYLOAD="$(jq -cn --argjson userId "$TENANT_USER_ID" --argjson expectedVersion "$RESET_MFA_VERSION" --arg reason " 用户更换认证设备 " '{userId:$userId,expectedVersion:$expectedVersion,reason:$reason}')"
api_put "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityMFA" "$RESET_PAYLOAD" "$WORK_DIR/mfa-reset-direct-blocked.json" 428
test "$(json_value "$WORK_DIR/mfa-reset-direct-blocked.json" data.actionType)" = "identity.mfa.reset"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_sessions WHERE user_id = $TENANT_USER_ID AND status = 'active'")" = "1"
RESET_APPROVAL_BODY="$(jq -cn --argjson payload "$RESET_PAYLOAD" '{actionType:"identity.mfa.reset",payload:$payload,reason:"双人复核 MFA 重置",idempotencyKey:"identity-mfa-reset-smoke"}')"
api_post "$OPERATOR_TOKEN" "/dashboard/saasAdmin/approvalRequest" "$RESET_APPROVAL_BODY" "$WORK_DIR/mfa-reset-request.json"
RESET_APPROVAL_ID="$(json_value "$WORK_DIR/mfa-reset-request.json" data.approval.id)"
RESET_APPROVAL_VERSION="$(json_value "$WORK_DIR/mfa-reset-request.json" data.approval.version)"
test "$(json_value "$WORK_DIR/mfa-reset-request.json" data.approval.requiredApprovals)" = "2"
test "$(json_value "$WORK_DIR/mfa-reset-request.json" data.approval.targetId)" = "$TENANT_USER_ID"
test "$(json_value "$WORK_DIR/mfa-reset-request.json" data.approval.targetName)" = "租户管理员 / 租户 981"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.tenantId')) FROM mochat_go_saas_admin_approvals WHERE id = $RESET_APPROVAL_ID")" = "981"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.reason')) FROM mochat_go_saas_admin_approvals WHERE id = $RESET_APPROVAL_ID")" = "用户更换认证设备"
approve "$APPROVER1_TOKEN" "$RESET_APPROVAL_ID" "$RESET_APPROVAL_VERSION" "第一复核人确认 MFA 重置" "$WORK_DIR/mfa-reset-vote-one.json"
RESET_APPROVAL_VERSION="$(json_value "$WORK_DIR/mfa-reset-vote-one.json" data.approval.version)"
approve "$APPROVER2_TOKEN" "$RESET_APPROVAL_ID" "$RESET_APPROVAL_VERSION" "第二复核人确认 MFA 重置" "$WORK_DIR/mfa-reset-vote-two.json"
RESET_APPROVAL_VERSION="$(json_value "$WORK_DIR/mfa-reset-vote-two.json" data.approval.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/approvalExecute" \
  "{\"approvalId\":$RESET_APPROVAL_ID,\"expectedVersion\":$RESET_APPROVAL_VERSION}" "$WORK_DIR/mfa-reset-execute.json"
test "$(json_value "$WORK_DIR/mfa-reset-execute.json" data.result.mfa.status)" = "disabled"
test "$(json_value "$WORK_DIR/mfa-reset-execute.json" data.result.revokedSessions)" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_sessions WHERE user_id = $TENANT_USER_ID AND status = 'active'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_auth_challenges WHERE user_id = $TENANT_USER_ID AND status = 'pending'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $RESET_APPROVAL_ID AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = (SELECT effect_operation_id FROM mochat_go_saas_admin_approvals WHERE id = $RESET_APPROVAL_ID) AND action = 'saas.admin.identity.mfa.reset'")" = "1"
api_get "$RESET_TOKEN" "/dashboard/user/securityMFA" "$WORK_DIR/mfa-reset-token-revoked.json" 401

api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityLoginEvents?tenantId=981&limit=100" "$WORK_DIR/events.json"
api_get "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityIncidents?tenantId=981&limit=20" "$WORK_DIR/incidents.json"
python3 - "$WORK_DIR/events.json" "$WORK_DIR/incidents.json" <<'PY'
import json, pathlib, sys
events = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))["data"]["events"]
incidents = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))["data"]["incidents"]
types = {item["eventType"] for item in events}
assert {"password_failed", "login_blocked", "mfa_failed", "login_succeeded", "session_revoked"}.issubset(types), types
assert any(item["incidentType"] == "brute_force" for item in incidents), incidents
assert incidents, incidents
PY
INCIDENT_ID="$(json_value "$WORK_DIR/incidents.json" data.incidents.0.id)"
INCIDENT_VERSION="$(json_value "$WORK_DIR/incidents.json" data.incidents.0.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityIncident" "{\"id\":$INCIDENT_ID,\"action\":\"acknowledge\",\"expectedVersion\":$INCIDENT_VERSION}" "$WORK_DIR/incident-ack.json"
INCIDENT_VERSION="$(json_value "$WORK_DIR/incident-ack.json" data.incident.version)"
api_post "$PLATFORM_TOKEN" "/dashboard/saasAdmin/identityIncident" "{\"id\":$INCIDENT_ID,\"action\":\"resolve\",\"resolution\":\"smoke verified\",\"expectedVersion\":$INCIDENT_VERSION}" "$WORK_DIR/incident-resolve.json"

mysql_root "$DATABASE" <<SQL
UPDATE mochat_go_saas_identity_auth_challenges SET status = 'consumed', active_slot = NULL, updated_at = DATE_SUB(NOW(), INTERVAL 10 DAY);
UPDATE mochat_go_saas_identity_sessions SET status = 'revoked', updated_at = DATE_SUB(NOW(), INTERVAL 10 DAY) WHERE tenant_id = 981;
INSERT INTO mochat_go_saas_identity_sessions
  (session_jti, token_sha256, user_id, tenant_id, status, auth_method, ip_address, user_agent,
   issued_at, expires_at, idle_expires_at, last_seen_at, version, created_at, updated_at)
VALUES
  ('expired-smoke-session', REPEAT('e', 64), $TENANT_USER_ID, 981, 'active', 'password', '127.0.0.1', 'smoke',
   DATE_SUB(NOW(), INTERVAL 2 DAY), DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_SUB(NOW(), INTERVAL 1 DAY),
   DATE_SUB(NOW(), INTERVAL 1 DAY), 1, DATE_SUB(NOW(), INTERVAL 2 DAY), DATE_SUB(NOW(), INTERVAL 2 DAY));
INSERT INTO mochat_go_saas_identity_login_events
  (user_id, tenant_id, phone_sha256, event_type, result, risk_level, reason_code, ip_address, user_agent, metadata_json, occurred_at, created_at)
VALUES ($TENANT_USER_ID, 981, REPEAT('f', 64), 'password_failed', 'failed', 'warning', 'retention-smoke', '', '', JSON_OBJECT(), DATE_SUB(NOW(), INTERVAL 40 DAY), NOW());
SQL

MOCHAT_MYSQL_DSN="$DSN" MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY" \
  "$MAINTENANCE_BIN" -action identity-cleanup -identity-cleanup-limit 1000 >"$WORK_DIR/maintenance.out"
grep -q $'action\tidentity-cleanup' "$WORK_DIR/maintenance.out"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_login_events WHERE reason_code = 'retention-smoke'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_identity_sessions WHERE session_jti = 'expired-smoke-session' AND status = 'expired'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action IN ('saas.admin.identity.policy.update', 'saas.admin.identity.session.revoke', 'saas.admin.identity.user.unlock', 'saas.admin.identity.mfa.reset', 'saas.admin.identity.incident.update')")" -ge "8"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE COALESCE(before_json, '') LIKE '%$MFA_SECRET%' OR COALESCE(after_json, '') LIKE '%$MFA_SECRET%' OR COALESCE(remark, '') LIKE '%$RECOVERY_1%'")" = "0"

echo "SaaS identity security smoke passed"
