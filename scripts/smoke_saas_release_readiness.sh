#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-release-readiness-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13417}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26467}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18191}"
EVIDENCE_PORT="${MOCHAT_RELEASE_EVIDENCE_PORT:-18443}"
DATABASE="mochat_saas_release_readiness"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-release-readiness.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
EVIDENCE_PID=""
EVIDENCE_DIR="$WORK_DIR/evidence"
EVIDENCE_CERT="$WORK_DIR/evidence-cert.pem"
EVIDENCE_KEY="$WORK_DIR/evidence-key.pem"
EVIDENCE_LOG="$WORK_DIR/evidence-server.log"
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-release-readiness-jwt-secret}"
FINGERPRINT="$(printf 'a%.0s' {1..64})"
OTHER_FINGERPRINT="$(printf 'b%.0s' {1..64})"
ARTIFACT_SHA256="$(printf 'c%.0s' {1..64})"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
	if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
	fi
	if [ -n "$EVIDENCE_PID" ] && kill -0 "$EVIDENCE_PID" 2>/dev/null; then
		kill "$EVIDENCE_PID" 2>/dev/null || true
		wait "$EVIDENCE_PID" 2>/dev/null || true
	fi
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "SaaS release readiness smoke failed at line $LINENO" >&2; tail -240 "$GO_LOG" >&2 || true' ERR
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
  local phone="$1" output="$2"
  curl -sS -f -H 'Content-Type: application/json' -d "{\"phone\":\"$phone\",\"password\":\"secret057\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

api_write() {
  local method="$1" token="$2" path="$3" body="$4" output="$5" expected="${6:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "$method $path returned $status, expected $expected" >&2; cat "$output" >&2; return 1; }
}

stop_go() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID"
    wait "$GO_PID" 2>/dev/null || true
  fi
  GO_PID=""
}

start_go() {
	local approval_required="$1"
	env -u GOROOT \
		MOCHAT_GO_STANDALONE=1 \
		MOCHAT_GO_ADDR="$GO_ADDR" \
		MOCHAT_MYSQL_DSN="$DSN" \
		MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
		MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
		MOCHAT_GO_MIGRATE_AUTH=1 \
		MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
		MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
		MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED="$approval_required" \
		MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
		MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT="$FINGERPRINT" \
		MOCHAT_GO_SAAS_RELEASE_EVIDENCE_ALLOWED_CIDRS=127.0.0.1/32 \
		MOCHAT_GO_SAAS_RELEASE_EVIDENCE_CA_FILE="$EVIDENCE_CERT" \
		MOCHAT_GO_SAAS_RELEASE_EVIDENCE_VERIFY_TIMEOUT_SECONDS=5 \
		MOCHAT_GO_SAAS_RELEASE_EVIDENCE_MAX_BYTES=1048576 \
		MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY=5757575757575757575757575757575757575757575757575757575757575757 \
		MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=release-smoke \
		MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY=5858585858585858585858585858585858585858585858585858585858585858 \
		MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID=release-smoke \
		MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY=5959595959595959595959595959595959595959595959595959595959595959 \
		"$GO_BIN" >"$GO_LOG" 2>&1 &
	GO_PID="$!"
	wait_url "http://$GO_ADDR/readyz" 200
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"
assert_port_free "$EVIDENCE_PORT"
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
grep -q $'0060_saas_service_account_usage_retention\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0062_saas_audit_integrity\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0063_saas_audit_anchor_signatures\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0065_saas_service_account_key_pepper_ring\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0066_saas_alert_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0069_saas_release_candidate_approval\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0070_saas_approval_policy_change_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0071_saas_backup_policy_change_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0072_saas_backup_cleanup_saga\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0073_saas_compliance_export_deletion_saga\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0074_saas_compliance_legal_hold_release_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0076_saas_identity_policy_change_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0077_saas_tenant_disable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0079_saas_service_account_key_revoke_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0080_saas_service_account_update_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0081_saas_service_account_key_rotate_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0082_saas_service_account_create_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0083_saas_identity_mfa_reset_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0084_saas_package_definition_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0086_saas_tenant_provision_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0087_saas_tenant_renewal_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0090_saas_payment_order_create_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0091_saas_payment_settlement_close_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0092_saas_payment_settlement_reopen_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0093_saas_payment_settlement_resolve_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0094_saas_tenant_domain_command_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0095_saas_tenant_domain_create_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0096_saas_tenant_enable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0098_scrm_lead_foundation\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type IN ('tenant.disable','tenant.enable','tenant.provision','tenant.renewal','tenant.subscription.transition','package.upsert','tenant.package.update','tenant.domain.create','tenant.domain.command','payment.order.create','payment.refund.create','payment.settlement.close','payment.settlement.reopen','payment.settlement.resolve','billing.invoice.issue','access.role.save','access.assignment.save','tenant.data.erase','release.candidate.gate','approval.policy.update','backup.policy.update','backup.retention.cleanup','compliance.policy.update','compliance.export.delete','compliance.legal_hold.release','identity.policy.update','identity.mfa.reset','service_account.create','service_account.update','service_account.key.rotate','service_account.key.revoke') AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals >= 2")" = "31"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'release.candidate.gate' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 6")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.legal_hold.release' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.policy.update' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'identity.policy.update' AND enabled = 1 AND required_approvals = 2 AND expiry_hours = 12")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.enable' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2 AND expiry_hours = 24")" = "1"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_saas_release_evidence WHERE required = 1')" = "6"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_saas_release_evidence_actions WHERE owner_user_id = 0 AND due_at IS NULL AND CHAR_LENGTH(next_action) > 0 AND version = 1')" = "6"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_release_evidence' AND column_name IN ('artifact_sha256', 'artifact_size_bytes')")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.release.read'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.release.manage'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_service_accounts' AND column_name IN ('usage_alert_enabled', 'usage_warning_percent', 'rejection_warning_count', 'usage_alert_cooldown_minutes', 'usage_alert_last_evaluated_at', 'usage_alert_last_notified_at', 'rejection_alert_last_notified_at')")" = "7"

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 1 -tenant-name '发布治理平台' -phone 13800000057 -password secret057 -user-name '平台超级管理员' -role-name '平台超级管理员' -package-code release-platform -package-name '平台版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-platform.out"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 2 -tenant-name '业务租户' -phone 13800000058 -password secret057 -user-name '业务租户管理员' -role-name '租户超级管理员' -package-code release-business -package-name '业务版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-business.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000059', password, '发布运营', 0, '发布部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000057' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000060', password, '发布审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000057' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000061', password, '发布审批甲', 0, '审批部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000057' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000062', password, '发布审批乙', 0, '审批部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000057' LIMIT 1;
SQL
OPERATIONS_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000059'")"
BUSINESS_USER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000058'")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000060'")"
APPROVER_A_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000061'")"
APPROVER_B_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000062'")"
OPERATIONS_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at) VALUES ($OPERATIONS_ID, 1, 1, NOW(), NOW()), ($AUDITOR_ID, 1, 1, NOW(), NOW()), ($APPROVER_A_ID, 1, 1, NOW(), NOW()), ($APPROVER_B_ID, 1, 1, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at) VALUES ($OPERATIONS_ID, $OPERATIONS_ROLE_ID, 1, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 1, NOW()), ($APPROVER_A_ID, $APPROVER_ROLE_ID, 1, NOW()), ($APPROVER_B_ID, $APPROVER_ROLE_ID, 1, NOW());
SQL

mkdir -p "$EVIDENCE_DIR"
for key in mysql57_amd64 real_wecom real_wechat_open real_saas_tenants production_frontend stability; do
	printf 'mochat-go release evidence: %s\n' "$key" >"$EVIDENCE_DIR/$key"
done
openssl req -x509 -nodes -newkey rsa:2048 -sha256 -days 1 \
	-keyout "$EVIDENCE_KEY" -out "$EVIDENCE_CERT" -subj '/CN=127.0.0.1' \
	-addext 'subjectAltName=IP:127.0.0.1' -addext 'keyUsage=digitalSignature,keyEncipherment' \
	-addext 'extendedKeyUsage=serverAuth' >/dev/null 2>&1
python3 - "$EVIDENCE_DIR" "$EVIDENCE_PORT" "$EVIDENCE_CERT" "$EVIDENCE_KEY" >"$EVIDENCE_LOG" 2>&1 <<'PY' &
import http.server
import ssl
import sys

directory, port, certificate, key = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
handler = lambda *args, **kwargs: http.server.SimpleHTTPRequestHandler(*args, directory=directory, **kwargs)
server = http.server.ThreadingHTTPServer(("127.0.0.1", port), handler)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.minimum_version = ssl.TLSVersion.TLSv1_2
context.load_cert_chain(certificate, key)
server.socket = context.wrap_socket(server.socket, server_side=True)
server.serve_forever()
PY
EVIDENCE_PID="$!"
deadline=$((SECONDS + 30))
while [ "$SECONDS" -lt "$deadline" ]; do
	curl -sS -f --cacert "$EVIDENCE_CERT" "https://127.0.0.1:$EVIDENCE_PORT/mysql57_amd64" >/dev/null 2>&1 && break
	sleep 1
done
curl -sS -f --cacert "$EVIDENCE_CERT" "https://127.0.0.1:$EVIDENCE_PORT/mysql57_amd64" >/dev/null

start_go 0

SUPER_TOKEN="$(login_token 13800000057 "$WORK_DIR/super-auth.json")"
BUSINESS_TOKEN="$(login_token 13800000058 "$WORK_DIR/business-auth.json")"
OPERATIONS_TOKEN="$(login_token 13800000059 "$WORK_DIR/operations-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000060 "$WORK_DIR/auditor-auth.json")"
APPROVER_A_TOKEN="$(login_token 13800000061 "$WORK_DIR/approver-a-auth.json")"
APPROVER_B_TOKEN="$(login_token 13800000062 "$WORK_DIR/approver-b-auth.json")"

api_get "$BUSINESS_TOKEN" '/dashboard/saasAdmin/releaseReadiness' "$WORK_DIR/business-readiness.json" 403
api_get "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/operations-access.json"
jq -e '(.data.profile.permissions | index("platform.release.read")) != null and (.data.profile.permissions | index("platform.release.manage")) != null' "$WORK_DIR/operations-access.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/auditor-access.json"
jq -e '(.data.profile.permissions | index("platform.release.read")) != null and (.data.profile.permissions | index("platform.release.manage")) == null' "$WORK_DIR/auditor-access.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/releaseReadiness' "$WORK_DIR/initial.json"
jq -e --arg fingerprint "$FINGERPRINT" '.data.summary.requiredCount == 6 and .data.summary.passedCount == 0 and .data.summary.metadataReady == false and .data.summary.ready == false and .data.summary.targetSourceFingerprint == $fingerprint and .data.summary.sourceFingerprintAuthoritative == true and .data.summary.sourceFingerprintSource == "environment" and .data.summary.candidateGateEnabled == false and .data.summary.artifactVerifier.configured == true and .data.summary.artifactVerifier.verifyOnPass == true and .data.summary.artifactVerifier.verifyOnCandidateGate == true and .data.summary.artifactVerifier.explicitAllowedCidrCount == 1 and .data.summary.artifactVerifier.customRootCasConfigured == true and (.data.evidence | length) == 6 and (.data.actions | length) == 6 and (.data.owners | length) >= 5 and .data.actionSummary.totalCount == 6 and .data.actionSummary.unresolvedCount == 6 and .data.actionSummary.unassignedCount == 6 and .data.actionSummary.resolvedCount == 0' "$WORK_DIR/initial.json" >/dev/null
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint=$OTHER_FINGERPRINT" "$WORK_DIR/readiness-fingerprint-conflict.json" 409

api_write PUT "$AUDITOR_TOKEN" '/dashboard/saasAdmin/releaseEvidence' '{"key":"real_wecom","status":"in_progress","expectedVersion":1}' "$WORK_DIR/auditor-write.json" 403
api_write PUT "$AUDITOR_TOKEN" '/dashboard/saasAdmin/releaseEvidenceAction' '{"key":"real_wecom","ownerUserId":0,"dueAt":"","nextAction":"审计员不能分派","expectedVersion":1}' "$WORK_DIR/auditor-action-write.json" 403
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidenceAction' "{\"key\":\"real_wecom\",\"ownerUserId\":$BUSINESS_USER_ID,\"dueAt\":\"2037-01-01 09:30:00\",\"nextAction\":\"跨租户负责人应被拒绝\",\"expectedVersion\":1}" "$WORK_DIR/cross-tenant-owner.json" 400
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidenceAction' "{\"key\":\"real_wecom\",\"ownerUserId\":$OPERATIONS_ID,\"dueAt\":\"2037-01-01T09:30\",\"nextAction\":\"使用真实企业微信完成授权、回调和消息链路联调\",\"note\":\"发布运营负责\",\"expectedVersion\":1}" "$WORK_DIR/action-assigned.json"
jq -e --argjson owner "$OPERATIONS_ID" '.data.action.key == "real_wecom" and .data.action.ownerUserId == $owner and .data.action.ownerActive == true and .data.action.dueAt == "2037-01-01 09:30:00" and .data.action.version == 2 and .data.operationId > 0' "$WORK_DIR/action-assigned.json" >/dev/null
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidenceAction' "{\"key\":\"real_wecom\",\"ownerUserId\":$OPERATIONS_ID,\"dueAt\":\"2037-01-02 09:30:00\",\"nextAction\":\"旧版本不能覆盖\",\"expectedVersion\":1}" "$WORK_DIR/action-version-conflict.json" 409
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_release_evidence_actions WHERE evidence_key = 'real_wecom' AND owner_user_id = $OPERATIONS_ID AND due_at = '2037-01-01 09:30:00' AND version = 2")" = "1"
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_release_evidence WHERE evidence_key = 'real_wecom'")" = "1"
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/releaseReadiness' "$WORK_DIR/action-readiness.json"
jq -e --argjson owner "$OPERATIONS_ID" '.data.actionSummary.assignedCount == 1 and .data.actionSummary.unassignedCount == 5 and (.data.actions[] | select(.key == "real_wecom") | .ownerUserId) == $owner and (.data.actions[] | select(.key == "real_wecom") | .state) == "in_progress"' "$WORK_DIR/action-readiness.json" >/dev/null
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "{\"key\":\"real_wecom\",\"status\":\"passed\",\"evidenceUrl\":\"https://localhost/evidence\",\"environment\":\"production\",\"sourceFingerprint\":\"$FINGERPRINT\",\"expectedVersion\":1}" "$WORK_DIR/unsafe-url.json" 400
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "{\"key\":\"real_wecom\",\"status\":\"failed\",\"note\":\"Authorization: Bearer hidden-credential-value\",\"expectedVersion\":1}" "$WORK_DIR/unsafe-note.json" 400
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "{\"key\":\"real_wecom\",\"status\":\"passed\",\"evidenceUrl\":\"https://evidence.company.cn/releases/real_wecom\",\"environment\":\"production\",\"sourceFingerprint\":\"$FINGERPRINT\",\"expectedVersion\":1}" "$WORK_DIR/missing-artifact.json" 400
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "{\"key\":\"real_wecom\",\"status\":\"passed\",\"evidenceUrl\":\"https://evidence.company.cn/releases/real_wecom\",\"environment\":\"production\",\"sourceFingerprint\":\"$OTHER_FINGERPRINT\",\"artifactSha256\":\"$ARTIFACT_SHA256\",\"artifactSizeBytes\":4096,\"expectedVersion\":1}" "$WORK_DIR/evidence-fingerprint-conflict.json" 409
test "$(mysql_scalar "SELECT version FROM mochat_go_saas_release_evidence WHERE evidence_key = 'real_wecom'")" = "1"

api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseCandidate' '{"releaseVersion":"v0.9.0"}' "$WORK_DIR/blocked-initial.json"
jq -e '.data.candidate.status == "blocked" and .data.candidate.requiredCount == 6 and .data.candidate.passedCount == 0 and .data.candidate.matchedCount == 0' "$WORK_DIR/blocked-initial.json" >/dev/null

for key in mysql57_amd64 real_wecom real_wechat_open real_saas_tenants production_frontend stability; do
  version="$(mysql_scalar "SELECT version FROM mochat_go_saas_release_evidence WHERE evidence_key = '$key'")"
  artifact_sha256="$(shasum -a 256 "$EVIDENCE_DIR/$key" | awk '{print $1}')"
  artifact_size="$(wc -c <"$EVIDENCE_DIR/$key" | tr -d ' ')"
  body="$(jq -cn --arg key "$key" --arg url "https://127.0.0.1:$EVIDENCE_PORT/$key" --arg fingerprint "$FINGERPRINT" --arg artifact "$artifact_sha256" --argjson size "$artifact_size" --argjson version "$version" '{key:$key,status:"passed",evidenceUrl:$url,environment:"production",sourceFingerprint:$fingerprint,artifactSha256:$artifact,artifactSizeBytes:$size,note:"验收通过",expectedVersion:$version}')"
  api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "$body" "$WORK_DIR/evidence-$key.json"
  jq -e '.data.artifactVerification.verified == true and .data.artifactVerification.actualSha256 == .data.artifactVerification.expectedSha256 and .data.artifactVerification.actualSizeBytes == .data.artifactVerification.expectedSizeBytes and .data.artifactVerification.httpStatus == 200' "$WORK_DIR/evidence-$key.json" >/dev/null
done

REAL_WECOM_SHA256="$(shasum -a 256 "$EVIDENCE_DIR/real_wecom" | awk '{print $1}')"
REAL_WECOM_SIZE="$(wc -c <"$EVIDENCE_DIR/real_wecom" | tr -d ' ')"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "{\"key\":\"real_wecom\",\"status\":\"passed\",\"evidenceUrl\":\"https://127.0.0.1:$EVIDENCE_PORT/real_wecom\",\"environment\":\"production\",\"sourceFingerprint\":\"$FINGERPRINT\",\"artifactSha256\":\"$REAL_WECOM_SHA256\",\"artifactSizeBytes\":$REAL_WECOM_SIZE,\"note\":\"旧版本写入\",\"expectedVersion\":1}" "$WORK_DIR/version-conflict.json" 409

api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint=$FINGERPRINT" "$WORK_DIR/ready.json"
jq -e '.data.summary.metadataReady == true and .data.summary.candidateGateEnabled == true and .data.summary.ready == false and .data.summary.remoteVerifiedCount == 0 and .data.summary.passedCount == 6 and .data.summary.fingerprintMatchedCount == 6 and .data.actionSummary.resolvedCount == 6 and .data.actionSummary.unresolvedCount == 0 and ([.data.actions[].state] | all(. == "resolved")) and ([.data.evidence[].checkedBy] | all(. > 0)) and ([.data.evidence[].artifactSha256 | length] | all(. == 64)) and ([.data.evidence[].artifactSizeBytes] | all(. > 0))' "$WORK_DIR/ready.json" >/dev/null
api_write POST "$SUPER_TOKEN" '/dashboard/saasAdmin/releaseCandidate' "{\"releaseVersion\":\"v1.0.0-wrong\",\"sourceFingerprint\":\"$OTHER_FINGERPRINT\"}" "$WORK_DIR/rejected-fingerprint.json" 409
printf 'tampered\n' >>"$EVIDENCE_DIR/stability"
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseCandidate' '{"releaseVersion":"v1.0.0-tampered"}' "$WORK_DIR/tampered-candidate.json"
jq -e '.data.candidate.status == "blocked" and .data.candidate.requiredCount == 6 and .data.candidate.passedCount == 5 and .data.candidate.matchedCount == 5' "$WORK_DIR/tampered-candidate.json" >/dev/null
printf 'mochat-go release evidence: stability\n' >"$EVIDENCE_DIR/stability"
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseCandidate' '{"releaseVersion":"v1.0.0"}' "$WORK_DIR/ready-candidate.json"
jq -e --arg fingerprint "$FINGERPRINT" '.data.candidate.status == "ready" and .data.candidate.sourceFingerprint == $fingerprint and .data.candidate.requiredCount == 6 and .data.candidate.passedCount == 6 and .data.candidate.matchedCount == 6 and .data.operationId > 0' "$WORK_DIR/ready-candidate.json" >/dev/null

api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint=$FINGERPRINT" "$WORK_DIR/final-ready.json"
jq -e '.data.summary.metadataReady == true and .data.summary.ready == true and .data.summary.remoteVerifiedCount == 6 and .data.summary.latestCandidateStatus == "ready" and .data.summary.latestCandidateEffectiveStatus == "ready" and .data.summary.latestCandidateSnapshotValid == true and .data.summary.latestCandidateEvidenceCurrent == true and (.data.summary.latestCandidateDriftedEvidenceKeys | length) == 0 and (.data.summary.latestCandidateNo | length) > 0 and .data.candidates[0].effectiveStatus == "ready" and .data.candidates[0].evidenceCurrent == true' "$WORK_DIR/final-ready.json" >/dev/null

STABILITY_SHA256="$(shasum -a 256 "$EVIDENCE_DIR/stability" | awk '{print $1}')"
STABILITY_SIZE="$(wc -c <"$EVIDENCE_DIR/stability" | tr -d ' ')"
STABILITY_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_release_evidence WHERE evidence_key = 'stability'")"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "{\"key\":\"stability\",\"status\":\"failed\",\"note\":\"目标环境回归失败\",\"expectedVersion\":$STABILITY_VERSION}" "$WORK_DIR/stability-failed.json"
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint=$FINGERPRINT" "$WORK_DIR/stale-after-failure.json"
jq -e '.data.summary.metadataReady == false and .data.summary.ready == false and .data.summary.latestCandidateStatus == "ready" and .data.summary.latestCandidateEffectiveStatus == "stale" and .data.summary.latestCandidateSnapshotValid == true and .data.summary.latestCandidateEvidenceCurrent == false and (.data.summary.latestCandidateDriftedEvidenceKeys | index("stability")) != null and .data.candidates[0].status == "ready" and .data.candidates[0].effectiveStatus == "stale" and .data.candidates[0].evidenceCurrent == false' "$WORK_DIR/stale-after-failure.json" >/dev/null

STABILITY_VERSION="$(mysql_scalar "SELECT version FROM mochat_go_saas_release_evidence WHERE evidence_key = 'stability'")"
body="$(jq -cn --arg url "https://127.0.0.1:$EVIDENCE_PORT/stability" --arg fingerprint "$FINGERPRINT" --arg artifact "$STABILITY_SHA256" --argjson size "$STABILITY_SIZE" --argjson version "$STABILITY_VERSION" '{key:"stability",status:"passed",evidenceUrl:$url,environment:"production",sourceFingerprint:$fingerprint,artifactSha256:$artifact,artifactSizeBytes:$size,note:"重新验收通过",expectedVersion:$version}')"
api_write PUT "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseEvidence' "$body" "$WORK_DIR/stability-restored.json"
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint=$FINGERPRINT" "$WORK_DIR/stale-after-restore.json"
jq -e '.data.summary.metadataReady == true and .data.summary.ready == false and .data.summary.latestCandidateEffectiveStatus == "stale" and .data.summary.latestCandidateEvidenceCurrent == false and (.data.summary.latestCandidateDriftedEvidenceKeys | index("stability")) != null' "$WORK_DIR/stale-after-restore.json" >/dev/null

api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseCandidate' '{"releaseVersion":"v1.0.1"}' "$WORK_DIR/recertified-candidate.json"
jq -e '.data.candidate.status == "ready" and .data.candidate.requiredCount == 6 and .data.candidate.passedCount == 6 and .data.candidate.matchedCount == 6' "$WORK_DIR/recertified-candidate.json" >/dev/null
api_get "$AUDITOR_TOKEN" "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint=$FINGERPRINT" "$WORK_DIR/recertified-ready.json"
jq -e '.data.summary.metadataReady == true and .data.summary.ready == true and .data.summary.latestCandidateEffectiveStatus == "ready" and .data.summary.latestCandidateSnapshotValid == true and .data.summary.latestCandidateEvidenceCurrent == true and (.data.summary.latestCandidateDriftedEvidenceKeys | length) == 0 and .data.candidates[0].releaseVersion == "v1.0.1" and .data.candidates[0].effectiveStatus == "ready" and .data.candidates[1].effectiveStatus == "stale"' "$WORK_DIR/recertified-ready.json" >/dev/null

stop_go
start_go 1
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/releaseCandidate' '{"releaseVersion":"v1.0.2"}' "$WORK_DIR/direct-release-rejected.json" 428
jq -e '.data.actionType == "release.candidate.gate" and .data.required == true and .data.requiredApprovals == 2' "$WORK_DIR/direct-release-rejected.json" >/dev/null
approval_body="$(jq -cn --arg fingerprint "$FINGERPRINT" '{actionType:"release.candidate.gate",payload:{releaseVersion:"v1.0.2",sourceFingerprint:$fingerprint},reason:"生产发布双人复核",expiresInHours:6,idempotencyKey:"release-smoke-v1.0.2"}')"
api_write POST "$OPERATIONS_TOKEN" '/dashboard/saasAdmin/approvalRequest' "$approval_body" "$WORK_DIR/release-approval.json"
jq -e --arg fingerprint "$FINGERPRINT" '.data.approval.actionType == "release.candidate.gate" and .data.approval.status == "pending" and .data.approval.requiredApprovals == 2 and .data.approval.targetId == $fingerprint' "$WORK_DIR/release-approval.json" >/dev/null
APPROVAL_ID="$(jq -er '.data.approval.id' "$WORK_DIR/release-approval.json")"
APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/release-approval.json")"
api_write POST "$APPROVER_A_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$APPROVAL_ID,\"expectedVersion\":$APPROVAL_VERSION,\"decision\":\"approve\",\"reason\":\"证据与版本已复核\"}" "$WORK_DIR/release-approval-a.json"
jq -e '.data.approval.status == "pending" and .data.approval.approvalCount == 1 and .data.approval.requiredApprovals == 2' "$WORK_DIR/release-approval-a.json" >/dev/null
APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/release-approval-a.json")"
api_write POST "$APPROVER_B_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$APPROVAL_ID,\"expectedVersion\":$APPROVAL_VERSION,\"decision\":\"approve\",\"reason\":\"批准进入发布门禁执行\"}" "$WORK_DIR/release-approval-b.json"
jq -e '.data.approval.status == "approved" and .data.approval.approvalCount == 2 and .data.approval.requiredApprovals == 2' "$WORK_DIR/release-approval-b.json" >/dev/null
APPROVAL_VERSION="$(jq -er '.data.approval.version' "$WORK_DIR/release-approval-b.json")"
api_write POST "$APPROVER_B_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$APPROVAL_ID,\"expectedVersion\":$APPROVAL_VERSION}" "$WORK_DIR/release-approval-executed.json"
jq -e --arg fingerprint "$FINGERPRINT" '.data.approval.status == "executed" and .data.result.candidate.status == "ready" and .data.result.candidate.releaseVersion == "v1.0.2" and .data.result.candidate.sourceFingerprint == $fingerprint and .data.result.operationId > 0' "$WORK_DIR/release-approval-executed.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $APPROVAL_ID AND status = 'executed' AND approval_count = 2 AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_release_candidates c INNER JOIN mochat_go_saas_admin_approvals a ON a.effect_operation_id = c.operation_id WHERE a.id = $APPROVAL_ID AND c.release_version = 'v1.0.2' AND c.status = 'ready'")" = "1"

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_release_candidates WHERE status = 'blocked'")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_release_candidates WHERE status = 'ready'")" = "3"
test "$(mysql_scalar "SELECT JSON_LENGTH(snapshot_json) FROM mochat_go_saas_release_candidates WHERE status = 'ready' ORDER BY id DESC LIMIT 1")" = "6"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(snapshot_json, '\$[0].artifactVerification.verified')) FROM mochat_go_saas_release_candidates WHERE status = 'ready' ORDER BY id DESC LIMIT 1")" = "true"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(snapshot_json, '\$[0].artifactVerification.actualSha256')) = JSON_UNQUOTE(JSON_EXTRACT(snapshot_json, '\$[0].artifactVerification.expectedSha256')) FROM mochat_go_saas_release_candidates WHERE status = 'ready' ORDER BY id DESC LIMIT 1")" = "1"
test "$(mysql_scalar "SELECT JSON_EXTRACT(snapshot_json, '\$[0].artifactVerification.actualSizeBytes') = JSON_EXTRACT(snapshot_json, '\$[0].artifactVerification.expectedSizeBytes') FROM mochat_go_saas_release_candidates WHERE status = 'ready' ORDER BY id DESC LIMIT 1")" = "1"
test "$(mysql_scalar "SELECT JSON_UNQUOTE(JSON_EXTRACT(snapshot_json, '\$[5].artifactVerification.verified')) FROM mochat_go_saas_release_candidates WHERE status = 'blocked' AND release_version = 'v1.0.0-tampered' LIMIT 1")" = "false"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.release.evidence.save'")" = "8"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.release.evidence.action.save'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.release.candidate.gate'")" = "5"
grep -q "source_fingerprint_configured=true source=environment fingerprint=$FINGERPRINT" "$GO_LOG"

api_get "$SUPER_TOKEN" '/dashboard/saasAdmin/page' "$WORK_DIR/page.html"
grep -q 'id="releaseReadinessCenter"' "$WORK_DIR/page.html"
grep -q 'id="runReleaseGate"' "$WORK_DIR/page.html"
grep -q 'id="releaseEvidenceArtifactFingerprint"' "$WORK_DIR/page.html"
grep -q 'id="releaseEvidenceArtifactSize"' "$WORK_DIR/page.html"
grep -q 'id="releaseEvidenceActions"' "$WORK_DIR/page.html"
grep -q 'id="releaseEvidenceActionOwner"' "$WORK_DIR/page.html"
grep -q '生产补证行动清单' "$WORK_DIR/page.html"
grep -q '证据已变化' "$WORK_DIR/page.html"
grep -q '现行证据已变化' "$WORK_DIR/page.html"
api_get "$SUPER_TOKEN" '/compat/routes' "$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/releaseReadiness' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/releaseEvidence' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/releaseEvidenceAction' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/releaseCandidate' "$WORK_DIR/routes.json"

stop_go
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-scrm-lead-foundation.out"
grep -q $'0098_scrm_lead_foundation\trolled_back' "$WORK_DIR/rollback-scrm-lead-foundation.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_scrm_leads'")"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-release-evidence-action.out"
grep -q $'0097_saas_release_evidence_action_tracking\trolled_back' "$WORK_DIR/rollback-release-evidence-action.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "96"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_release_evidence_actions'")"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-enable-guard.out"
grep -q $'0096_saas_tenant_enable_approval_guard\trolled_back' "$WORK_DIR/rollback-tenant-enable-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "95"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.enable'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-domain-create-guard.out"
grep -q $'0095_saas_tenant_domain_create_guard\trolled_back' "$WORK_DIR/rollback-tenant-domain-create-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "94"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.domain.create'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-domain-command-guard.out"
grep -q $'0094_saas_tenant_domain_command_guard\trolled_back' "$WORK_DIR/rollback-tenant-domain-command-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "93"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.domain.command'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-payment-settlement-resolve-guard.out"
grep -q $'0093_saas_payment_settlement_resolve_guard\trolled_back' "$WORK_DIR/rollback-payment-settlement-resolve-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "92"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.settlement.resolve'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-payment-settlement-reopen-guard.out"
grep -q $'0092_saas_payment_settlement_reopen_guard\trolled_back' "$WORK_DIR/rollback-payment-settlement-reopen-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "91"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.settlement.reopen'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-payment-settlement-close-guard.out"
grep -q $'0091_saas_payment_settlement_close_guard\trolled_back' "$WORK_DIR/rollback-payment-settlement-close-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "90"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.settlement.close' AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals >= 2")" = "1"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-payment-order-create-approval-guard.out"
grep -q $'0090_saas_payment_order_create_approval_guard\trolled_back' "$WORK_DIR/rollback-payment-order-create-approval-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "89"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.order.create'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_payment_orders' AND column_name IN ('package_version', 'package_limits_json')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-invoice-issue-approval-guard.out"
grep -q $'0089_saas_invoice_issue_approval_guard\trolled_back' "$WORK_DIR/rollback-invoice-issue-approval-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "88"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'billing.invoice.issue'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-subscription-transition-approval-guard.out"
grep -q $'0088_saas_subscription_transition_approval_guard\trolled_back' "$WORK_DIR/rollback-subscription-transition-approval-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "87"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.subscription.transition'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-renewal-approval-guard.out"
grep -q $'0087_saas_tenant_renewal_approval_guard\trolled_back' "$WORK_DIR/rollback-tenant-renewal-approval-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "86"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.renewal'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-provision-approval-guard.out"
grep -q $'0086_saas_tenant_provision_approval_guard\trolled_back' "$WORK_DIR/rollback-tenant-provision-approval-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "85"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.provision'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_admin_tasks' AND column_name = 'version'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-package-assignment-guard.out"
grep -q $'0085_saas_tenant_package_assignment_guard\trolled_back' "$WORK_DIR/rollback-tenant-package-assignment-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "84"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.package.update'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_tenant_packages' AND column_name = 'version'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-package-definition-guard.out"
grep -q $'0084_saas_package_definition_guard\trolled_back' "$WORK_DIR/rollback-package-definition-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "83"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'package.upsert'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_packages' AND column_name = 'version'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-identity-mfa-reset-guard.out"
grep -q $'0083_saas_identity_mfa_reset_guard\trolled_back' "$WORK_DIR/rollback-identity-mfa-reset-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "82"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'identity.mfa.reset'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-service-account-create-guard.out"
grep -q $'0082_saas_service_account_create_guard\trolled_back' "$WORK_DIR/rollback-service-account-create-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "81"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.create'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-service-account-key-rotate-guard.out"
grep -q $'0081_saas_service_account_key_rotate_guard\trolled_back' "$WORK_DIR/rollback-service-account-key-rotate-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "80"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.key.rotate'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-service-account-update-guard.out"
grep -q $'0080_saas_service_account_update_guard\trolled_back' "$WORK_DIR/rollback-service-account-update-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "79"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.update'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-service-account-key-revoke-guard.out"
grep -q $'0079_saas_service_account_key_revoke_guard\trolled_back' "$WORK_DIR/rollback-service-account-key-revoke-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "78"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'service_account.key.revoke'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-critical-approval-policy.out"
grep -q $'0078_saas_critical_approval_policy_guard\trolled_back' "$WORK_DIR/rollback-critical-approval-policy.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "77"
test "$(mysql_scalar "SELECT required_approvals FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'payment.refund.create'")" = "1"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-tenant-disable-approval.out"
grep -q $'0077_saas_tenant_disable_approval_guard\trolled_back' "$WORK_DIR/rollback-tenant-disable-approval.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "76"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.disable' AND required_approvals = 1")" = "1"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-identity-policy-update.out"
grep -q $'0076_saas_identity_policy_change_guard\trolled_back' "$WORK_DIR/rollback-identity-policy-update.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "75"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'identity.policy.update'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-compliance-policy-update.out"
grep -q $'0075_saas_compliance_policy_change_guard\trolled_back' "$WORK_DIR/rollback-compliance-policy-update.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "74"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.policy.update'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-compliance-hold-release.out"
grep -q $'0074_saas_compliance_legal_hold_release_guard\trolled_back' "$WORK_DIR/rollback-compliance-hold-release.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "73"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.legal_hold.release'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-compliance-export-deletion.out"
grep -q $'0073_saas_compliance_export_deletion_saga\trolled_back' "$WORK_DIR/rollback-compliance-export-deletion.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "72"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'compliance.export.delete'")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_data_exports' AND column_name LIKE 'deletion_%'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-backup-cleanup-saga.out"
grep -q $'0072_saas_backup_cleanup_saga\trolled_back' "$WORK_DIR/rollback-backup-cleanup-saga.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "71"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'backup.retention.cleanup'")" = "0"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_backup_cleanup_runs'")"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_backup_cleanup_items'")"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_backup_runs' AND column_name = 'cleanup_run_id'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-backup-policy-guard.out"
grep -q $'0071_saas_backup_policy_change_guard\trolled_back' "$WORK_DIR/rollback-backup-policy-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "70"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'backup.policy.update'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-approval-policy-guard.out"
grep -q $'0070_saas_approval_policy_change_guard\trolled_back' "$WORK_DIR/rollback-approval-policy-guard.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "69"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'approval.policy.update'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-release-approval.out"
grep -q $'0069_saas_release_candidate_approval\trolled_back' "$WORK_DIR/rollback-release-approval.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "68"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'release.candidate.gate'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-wechat-open-credentials.out"
grep -q $'0068_wechat_open_credential_encryption\trolled_back' "$WORK_DIR/rollback-wechat-open-credentials.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "67"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_wechat_component_tickets' AND column_name IN ('component_verify_ticket_ciphertext', 'credential_key_id')")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mc_official_account' AND column_name IN ('wechat_credentials_ciphertext', 'wechat_credentials_key_id')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-wecom-credentials.out"
grep -q $'0067_wecom_credential_encryption\trolled_back' "$WORK_DIR/rollback-wecom-credentials.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "66"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mc_corp' AND column_name IN ('wecom_credentials_ciphertext', 'wecom_credentials_key_id')")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mc_work_agent' AND column_name IN ('wecom_credentials_ciphertext', 'wecom_credentials_key_id')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-alert-credentials.out"
grep -q $'0066_saas_alert_credential_encryption\trolled_back' "$WORK_DIR/rollback-alert-credentials.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "65"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_alert_settings' AND column_name IN ('webhook_credentials_ciphertext', 'webhook_credentials_key_id')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-service-account-pepper.out"
grep -q $'0065_saas_service_account_key_pepper_ring\trolled_back' "$WORK_DIR/rollback-service-account-pepper.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "64"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_service_account_keys' AND column_name = 'hash_key_id'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-audit-anchor-remote.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\trolled_back' "$WORK_DIR/rollback-audit-anchor-remote.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "63"
test -n "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_audit_anchor_checkpoints'")"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_admin_audit_anchor_checkpoints' AND column_name LIKE 'remote_%'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-audit-anchor.out"
grep -q $'0063_saas_audit_anchor_signatures\trolled_back' "$WORK_DIR/rollback-audit-anchor.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "62"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_audit_anchor_checkpoints'")"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-audit-integrity.out"
grep -q $'0062_saas_audit_integrity\trolled_back' "$WORK_DIR/rollback-audit-integrity.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "61"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_audit_chains'")"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_admin_operation_logs' AND column_name IN ('integrity_prev_hash', 'integrity_hash', 'integrity_version')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-usage-alerts.out"
grep -q $'0061_saas_service_account_usage_alerts\trolled_back' "$WORK_DIR/rollback-usage-alerts.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "60"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_service_accounts' AND column_name IN ('usage_alert_enabled', 'usage_warning_percent', 'rejection_warning_count', 'usage_alert_cooldown_minutes', 'usage_alert_last_evaluated_at', 'usage_alert_last_notified_at', 'rejection_alert_last_notified_at')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-usage-retention.out"
grep -q $'0060_saas_service_account_usage_retention\trolled_back' "$WORK_DIR/rollback-usage-retention.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "59"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_compliance_policies' AND column_name = 'service_account_usage_retention_days'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-rate-limits.out"
grep -q $'0059_saas_service_account_rate_limits\trolled_back' "$WORK_DIR/rollback-rate-limits.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "58"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_service_account_usage_daily'")"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback.out"
grep -q $'0058_saas_release_evidence_integrity\trolled_back' "$WORK_DIR/rollback.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "57"
test -n "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_release_evidence'")"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_release_evidence' AND column_name IN ('artifact_sha256', 'artifact_size_bytes')")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action rollback >"$WORK_DIR/rollback-release.out"
grep -q $'0057_saas_release_readiness\trolled_back' "$WORK_DIR/rollback-release.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "56"
test -z "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_release_evidence'")"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code LIKE 'platform.release.%'")" = "0"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/reapply.out"
grep -q $'0057_saas_release_readiness\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0058_saas_release_evidence_integrity\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0059_saas_service_account_rate_limits\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0060_saas_service_account_usage_retention\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0061_saas_service_account_usage_alerts\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0062_saas_audit_integrity\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0063_saas_audit_anchor_signatures\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0065_saas_service_account_key_pepper_ring\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0066_saas_alert_credential_encryption\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0067_wecom_credential_encryption\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0069_saas_release_candidate_approval\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0070_saas_approval_policy_change_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0071_saas_backup_policy_change_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0072_saas_backup_cleanup_saga\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0073_saas_compliance_export_deletion_saga\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0074_saas_compliance_legal_hold_release_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0075_saas_compliance_policy_change_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0076_saas_identity_policy_change_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0077_saas_tenant_disable_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0078_saas_critical_approval_policy_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0079_saas_service_account_key_revoke_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0080_saas_service_account_update_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0081_saas_service_account_key_rotate_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0082_saas_service_account_create_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0083_saas_identity_mfa_reset_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0084_saas_package_definition_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0085_saas_tenant_package_assignment_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0086_saas_tenant_provision_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0087_saas_tenant_renewal_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0090_saas_payment_order_create_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0091_saas_payment_settlement_close_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0092_saas_payment_settlement_reopen_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0093_saas_payment_settlement_resolve_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0094_saas_tenant_domain_command_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0095_saas_tenant_domain_create_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0096_saas_tenant_enable_approval_guard\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0097_saas_release_evidence_action_tracking\tapplied_now' "$WORK_DIR/reapply.out"
grep -q $'0098_scrm_lead_foundation\tapplied_now' "$WORK_DIR/reapply.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_payment_orders' AND column_name IN ('package_version', 'package_limits_json')")" = "2"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_saas_release_evidence')" = "6"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_saas_release_evidence_actions')" = "6"

echo "SaaS release readiness smoke passed"
