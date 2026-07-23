#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-audit-integrity-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13412}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26462}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18186}"
MINIO_PORT="${MOCHAT_MINIO_PORT:-19186}"
MINIO_CONTAINER="${PROJECT_NAME}-minio"
MINIO_IMAGE="${MOCHAT_MINIO_IMAGE:-minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e}"
MINIO_MC_IMAGE="${MOCHAT_MINIO_MC_IMAGE:-minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727}"
MINIO_ACCESS_KEY="audit-anchor-smoke-access"
MINIO_SECRET_KEY="audit-anchor-smoke-secret-0064"
MINIO_BUCKET="mochat-audit-anchor-smoke"
AUDIT_ANCHOR_PREFIX="smoke/audit-anchors"
DATABASE="mochat_saas_audit_integrity"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-audit-integrity.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-audit-integrity-jwt-secret}"
AUDIT_ANCHOR_ROOT="$WORK_DIR/audit-anchors"
AUDIT_ANCHOR_KEY="${MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY:-0707070707070707070707070707070707070707070707070707070707070707}"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  if [ -n "$GO_PID" ] && kill -0 "$GO_PID" 2>/dev/null; then
    kill "$GO_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
  fi
  docker rm -f "$MINIO_CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap 'echo "SaaS audit integrity smoke failed at line $LINENO" >&2; tail -240 "$GO_LOG" >&2 || true' ERR
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
    if [ "$status" = "healthy" ]; then return 0; fi
    sleep 2
  done
  compose logs --tail=120 "$service" >&2 || true
  return 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
    if [ "$code" = "$expected" ]; then return 0; fi
    sleep 1
  done
  return 1
}

wait_mysql_at_least() {
  local query="$1" minimum="$2" deadline=$((SECONDS + 60)) value
  while [ "$SECONDS" -lt "$deadline" ]; do
    value="$(mysql_scalar "$query" || true)"
    if [[ "$value" =~ ^[0-9]+$ ]] && [ "$value" -ge "$minimum" ]; then return 0; fi
    sleep 1
  done
  echo "timed out waiting for MySQL value >= $minimum: $query" >&2
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

minio_mc() {
  local command="$1"
  docker run --rm -v "$WORK_DIR:/work:ro" --entrypoint /bin/sh "$MINIO_MC_IMAGE" -c \
    "mc alias set smoke http://host.docker.internal:$MINIO_PORT '$MINIO_ACCESS_KEY' '$MINIO_SECRET_KEY' >/dev/null && mc $command"
}

login_token() {
  local phone="$1" output="$2"
  curl -sS -f -H 'Content-Type: application/json' -d "{\"phone\":\"$phone\",\"password\":\"secret062\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_write() {
  local token="$1" path="$2" body="$3" output="$4" expected="${5:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X POST -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "POST $path returned $status" >&2; cat "$output" >&2; return 1; }
}

start_go() {
  : >"$GO_LOG"
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
    MOCHAT_GO_ENABLE_SAAS_AUDIT_INTEGRITY_CRON=1 \
    MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_RUN_ON_START=1 \
    MOCHAT_GO_SAAS_AUDIT_INTEGRITY_LIMIT=100 \
    MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON=1 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS=86400 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START=1 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT=100 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT="$AUDIT_ANCHOR_ROOT" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY="$AUDIT_ANCHOR_KEY" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID=audit-smoke-q3 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE=1 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT="http://127.0.0.1:$MINIO_PORT" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_BUCKET="$MINIO_BUCKET" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ACCESS_KEY_ID="$MINIO_ACCESS_KEY" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_USE_SSL=0 \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX="$AUDIT_ANCHOR_PREFIX" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE=compliance \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS=7 \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
}

assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"
assert_port_free "${GO_ADDR##*:}"
assert_port_free "$MINIO_PORT"
compose up -d mysql redis
wait_service_healthy mysql
wait_service_healthy redis

docker run -d --name "$MINIO_CONTAINER" \
  -p "127.0.0.1:$MINIO_PORT:9000" \
  -e "MINIO_ROOT_USER=$MINIO_ACCESS_KEY" \
  -e "MINIO_ROOT_PASSWORD=$MINIO_SECRET_KEY" \
  "$MINIO_IMAGE" server /data >/dev/null
wait_url "http://127.0.0.1:$MINIO_PORT/minio/health/live" 200
docker run --rm --entrypoint /bin/sh "$MINIO_MC_IMAGE" -c \
  "mc alias set smoke http://host.docker.internal:$MINIO_PORT '$MINIO_ACCESS_KEY' '$MINIO_SECRET_KEY' >/dev/null && mc mb --with-lock --ignore-existing smoke/$MINIO_BUCKET >/dev/null"

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
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_admin_audit_anchor_checkpoints'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = '$DATABASE' AND table_name = 'mochat_go_saas_admin_audit_anchor_checkpoints' AND column_name IN ('remote_status', 'remote_object_key', 'remote_version_id', 'remote_retention_mode', 'remote_retain_until')")" = "5"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.audit.manage'")" = "1"

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 1 -tenant-name '审计治理平台' -phone 13800000062 -password secret062 -user-name '平台超级管理员' -role-name '平台超级管理员' -package-code audit-platform -package-name '平台版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-platform.out"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 2 -tenant-name '审计业务租户' -phone 13800000063 -password secret062 -user-name '业务租户管理员' -role-name '租户超级管理员' -package-code audit-business -package-name '业务版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-business.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000064', password, '平台运营', 0, '运营部', '运营', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000062' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000065', password, '平台审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000062' LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000064'")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000065'")"
OPERATOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($AUDITOR_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $OPERATOR_ROLE_ID, 0, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 0, NOW());

INSERT INTO mochat_go_saas_admin_operation_logs
  (tenant_id, actor_user_id, actor_tenant_id, action, target_type, target_id, target_name, before_json, after_json, remark, created_at, updated_at, deleted_at)
VALUES
  (0, 0, 0, 'legacy.platform.audit', 'platform', '0', '平台历史审计', NULL, NULL, 'legacy anchor smoke', DATE_SUB(NOW(), INTERVAL 400 DAY), DATE_SUB(NOW(), INTERVAL 400 DAY), NULL),
  (2, 0, 1, 'legacy.tenant.audit', 'tenant', '2', '租户历史审计', NULL, NULL, 'legacy anchor smoke', DATE_SUB(NOW(), INTERVAL 2 DAY), DATE_SUB(NOW(), INTERVAL 2 DAY), NULL);
SQL

start_go
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-admin-audit-integrity' AND status = 'succeeded'" 1
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_saas_admin_audit_verifications WHERE source = 'cron' AND status = 'healthy'" 2
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-admin-audit-anchor' AND status = 'succeeded'" 1
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_saas_admin_audit_anchor_checkpoints WHERE artifact_status = 'exported' AND verification_status = 'passed'" 2
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_saas_admin_audit_anchor_checkpoints WHERE remote_status = 'exported' AND remote_provider = 's3-object-lock' AND remote_retention_mode = 'compliance' AND remote_version_id <> ''" 2

SUPER_TOKEN="$(login_token 13800000062 "$WORK_DIR/super-auth.json")"
BUSINESS_TOKEN="$(login_token 13800000063 "$WORK_DIR/business-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000064 "$WORK_DIR/operator-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000065 "$WORK_DIR/auditor-auth.json")"

api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/operator-access.json"
jq -e '(.data.profile.permissions | index("platform.audit.read")) != null and (.data.profile.permissions | index("platform.audit.manage")) != null' "$WORK_DIR/operator-access.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/auditor-access.json"
jq -e '(.data.profile.permissions | index("platform.audit.read")) != null and (.data.profile.permissions | index("platform.audit.manage")) == null' "$WORK_DIR/auditor-access.json" >/dev/null
api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/complianceOverview' "$WORK_DIR/compliance-overview.json"
jq -e '.data.config.inventoryUnknownTables == [] and .data.config.inventoryTableCount == 158' "$WORK_DIR/compliance-overview.json" >/dev/null

api_get "$BUSINESS_TOKEN" '/dashboard/saasAdmin/auditIntegrity?tenantId=2&limit=50&verificationLimit=20' "$WORK_DIR/business-overview.json" 403
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/auditIntegrity?tenantId=2&limit=50&verificationLimit=20' "$WORK_DIR/auditor-overview.json"
jq -e '.data.summary.chainCount == 1 and .data.summary.healthyChainCount == 1 and .data.summary.legacyLogCount == 1 and .data.chains[0].tenantId == 2 and .data.chains[0].status == "healthy"' "$WORK_DIR/auditor-overview.json" >/dev/null
api_get "$SUPER_TOKEN" '/dashboard/saasAdmin/auditIntegrity?tenantId=0&limit=50&verificationLimit=20' "$WORK_DIR/all-overview.json"
jq -e '.data.summary.chainCount >= 2 and .data.summary.healthyChainCount >= 2 and .data.summary.legacyLogCount >= 2 and .data.summary.retentionEligibleLogCount >= 1 and ([.data.chains[].tenantId] | index(0)) != null and ([.data.chains[].tenantId] | index(2)) != null' "$WORK_DIR/all-overview.json" >/dev/null

api_get "$BUSINESS_TOKEN" '/dashboard/saasAdmin/auditAnchors?tenantId=2&limit=50' "$WORK_DIR/business-anchors.json" 403
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/auditAnchors?tenantId=2&limit=50' "$WORK_DIR/auditor-anchors.json"
jq -e --arg bucket "$MINIO_BUCKET" --arg prefix "$AUDIT_ANCHOR_PREFIX" '.data.config.hmacConfigured == true and .data.config.hmacKeyId == "audit-smoke-q3" and .data.config.missingKeyIds == [] and .data.config.remoteConfigured == true and .data.config.remoteRequired == true and .data.config.remoteProvider == "s3-object-lock" and .data.config.remoteBucket == $bucket and .data.config.remotePrefix == $prefix and .data.config.remoteRetentionMode == "compliance" and .data.config.remoteRetentionDays == 7 and .data.summary.checkpointCount >= 1 and .data.summary.remoteExportedCount == .data.summary.checkpointCount and .data.summary.remoteFailedCount == 0 and .data.summary.orphanArtifactCount == 0 and .data.summary.orphanRemoteCount == 0' "$WORK_DIR/auditor-anchors.json" >/dev/null
api_write "$AUDITOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"create","tenantId":2,"limit":50}' "$WORK_DIR/auditor-anchor-create.json" 403
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"create","tenantId":2,"limit":50}' "$WORK_DIR/operator-anchor-create.json" 201
jq -e '.data.action == "create" and .data.result.scannedChains == 1 and .data.result.failedArtifacts == 0 and .data.result.exportedArtifacts == 1 and .data.result.remoteExportedArtifacts == 1 and .data.result.remoteFailedArtifacts == 0 and .data.result.operationId > 0' "$WORK_DIR/operator-anchor-create.json" >/dev/null
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/operator-anchor-verify.json"
jq -e '.data.action == "verify" and .data.result.scannedCheckpoints >= 1 and .data.result.failedCheckpoints == 0 and .data.result.passedCheckpoints == .data.result.scannedCheckpoints and .data.result.missingRemoteCount == 0 and .data.result.orphanArtifactCount == 0 and .data.result.orphanRemoteCount == 0 and .data.result.operationId > 0' "$WORK_DIR/operator-anchor-verify.json" >/dev/null

ANCHOR_NAME="$(jq -er '.data.result.checkpoints[0].artifactName' "$WORK_DIR/operator-anchor-create.json")"
ANCHOR_SHA="$(jq -er '.data.result.checkpoints[0].artifactSha256' "$WORK_DIR/operator-anchor-create.json")"
ANCHOR_PATH="$AUDIT_ANCHOR_ROOT/$ANCHOR_NAME"
REMOTE_OBJECT_KEY="$(jq -er '.data.result.checkpoints[0].remoteObjectKey' "$WORK_DIR/operator-anchor-create.json")"
REMOTE_VERSION_ID="$(jq -er '.data.result.checkpoints[0].remoteVersionId' "$WORK_DIR/operator-anchor-create.json")"
REMOTE_SHA="$(jq -er '.data.result.checkpoints[0].remoteSha256' "$WORK_DIR/operator-anchor-create.json")"
test -f "$ANCHOR_PATH"
test "$(shasum -a 256 "$ANCHOR_PATH" | awk '{print $1}')" = "$ANCHOR_SHA"
test "$REMOTE_SHA" = "$ANCHOR_SHA"
test -n "$REMOTE_OBJECT_KEY"
test -n "$REMOTE_VERSION_ID"
minio_mc "stat --version-id '$REMOTE_VERSION_ID' smoke/$MINIO_BUCKET/$REMOTE_OBJECT_KEY" >"$WORK_DIR/remote-stat.out"
if minio_mc "rm --version-id '$REMOTE_VERSION_ID' smoke/$MINIO_BUCKET/$REMOTE_OBJECT_KEY" >"$WORK_DIR/remote-delete.out" 2>&1; then
  echo "compliance-locked audit anchor version was unexpectedly deleted" >&2
  exit 1
fi
minio_mc "stat --version-id '$REMOTE_VERSION_ID' smoke/$MINIO_BUCKET/$REMOTE_OBJECT_KEY" >/dev/null
cp "$ANCHOR_PATH" "$WORK_DIR/anchor-original.json"
printf 'tampered\n' >"$ANCHOR_PATH"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/tampered-anchor-verify.json"
jq -e '.data.result.failedCheckpoints >= 1 and ([.data.result.checkpoints[].verificationStatus] | index("failed")) != null' "$WORK_DIR/tampered-anchor-verify.json" >/dev/null
cp "$WORK_DIR/anchor-original.json" "$ANCHOR_PATH"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/restored-anchor-verify.json"
jq -e '.data.result.failedCheckpoints == 0 and .data.result.passedCheckpoints == .data.result.scannedCheckpoints' "$WORK_DIR/restored-anchor-verify.json" >/dev/null

cp "$ANCHOR_PATH" "$AUDIT_ANCHOR_ROOT/AAN-ORPHAN-ROLLBACK-EVIDENCE.json"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/orphan-anchor-verify.json"
jq -e '.data.result.orphanArtifactCount == 1 and .data.result.failedCheckpoints >= 1' "$WORK_DIR/orphan-anchor-verify.json" >/dev/null
rm "$AUDIT_ANCHOR_ROOT/AAN-ORPHAN-ROLLBACK-EVIDENCE.json"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/orphan-restored-verify.json"
jq -e '.data.result.orphanArtifactCount == 0 and .data.result.failedCheckpoints == 0' "$WORK_DIR/orphan-restored-verify.json" >/dev/null

mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_audit_anchor_checkpoints SET remote_object_key = CONCAT(remote_object_key, '-missing') WHERE checkpoint_no = '${ANCHOR_NAME%.json}';"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/missing-remote-verify.json"
jq -e '.data.result.missingRemoteCount == 1 and .data.result.failedCheckpoints >= 1 and ([.data.result.checkpoints[].remoteStatus] | index("failed")) != null' "$WORK_DIR/missing-remote-verify.json" >/dev/null
mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_audit_anchor_checkpoints SET remote_object_key = '$REMOTE_OBJECT_KEY', remote_version_id = '$REMOTE_VERSION_ID' WHERE checkpoint_no = '${ANCHOR_NAME%.json}';"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/remote-restored-verify.json"
jq -e '.data.result.missingRemoteCount == 0 and .data.result.orphanRemoteCount == 0 and .data.result.failedCheckpoints == 0' "$WORK_DIR/remote-restored-verify.json" >/dev/null

ORPHAN_REMOTE_KEY="$AUDIT_ANCHOR_PREFIX/2026/07/13/AAN-FFFFFFFFFFFFFFFFFFFFFFFF.json"
minio_mc "cp /work/anchor-original.json smoke/$MINIO_BUCKET/$ORPHAN_REMOTE_KEY" >/dev/null
ORPHAN_REMOTE_VERSION="$(minio_mc "stat --json smoke/$MINIO_BUCKET/$ORPHAN_REMOTE_KEY" | jq -er '.versionID')"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/orphan-remote-verify.json"
jq -e '.data.result.orphanRemoteCount == 1 and .data.result.failedCheckpoints >= 1' "$WORK_DIR/orphan-remote-verify.json" >/dev/null
minio_mc "rm --version-id '$ORPHAN_REMOTE_VERSION' smoke/$MINIO_BUCKET/$ORPHAN_REMOTE_KEY" >/dev/null
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditAnchor' '{"action":"verify","tenantId":2,"limit":50}' "$WORK_DIR/orphan-remote-restored-verify.json"
jq -e '.data.result.orphanRemoteCount == 0 and .data.result.failedCheckpoints == 0' "$WORK_DIR/orphan-remote-restored-verify.json" >/dev/null

api_write "$AUDITOR_TOKEN" '/dashboard/saasAdmin/auditIntegrityVerify' '{"tenantId":2,"limit":50,"verificationLimit":20}' "$WORK_DIR/auditor-verify.json" 403
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditIntegrityVerify' '{"tenantId":2,"limit":50,"verificationLimit":20}' "$WORK_DIR/operator-verify.json"
jq -e '.data.scannedChains == 1 and .data.healthyChains == 1 and .data.failedChains == 0 and .data.operationId > 0 and .data.chains[0].signedLogCount >= 1' "$WORK_DIR/operator-verify.json" >/dev/null
SIGNED_OPERATION_ID="$(jq -er '.data.operationId' "$WORK_DIR/operator-verify.json")"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE id = $SIGNED_OPERATION_ID AND integrity_version = 1 AND CHAR_LENGTH(integrity_prev_hash) = 64 AND CHAR_LENGTH(integrity_hash) = 64")" = "1"

mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_operation_logs SET action = 'tampered.audit.action' WHERE id = $SIGNED_OPERATION_ID;"
api_write "$OPERATOR_TOKEN" '/dashboard/saasAdmin/auditIntegrityVerify' '{"tenantId":2,"limit":50,"verificationLimit":20}' "$WORK_DIR/tampered-verify.json"
jq -e --argjson id "$SIGNED_OPERATION_ID" '.data.scannedChains == 1 and .data.failedChains == 1 and .data.healthyChains == 0 and .data.chains[0].status == "failed" and .data.chains[0].lastFailedLogId == $id' "$WORK_DIR/tampered-verify.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/auditIntegrity?tenantId=2&limit=50&verificationLimit=20' "$WORK_DIR/failed-overview.json"
jq -e --argjson id "$SIGNED_OPERATION_ID" '.data.summary.failedChainCount == 1 and .data.chains[0].lastFailedLogId == $id and (.data.chains[0].lastVerificationError | length) > 0' "$WORK_DIR/failed-overview.json" >/dev/null

mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_admin_operation_logs SET action = 'saas.admin.audit.integrity.verify', target_name = '已脱敏', before_json = NULL, after_json = NULL, remark = '已按合规策略脱敏' WHERE id = $SIGNED_OPERATION_ID;"
api_write "$SUPER_TOKEN" '/dashboard/saasAdmin/auditIntegrityVerify' '{"tenantId":2,"limit":50,"verificationLimit":20}' "$WORK_DIR/restored-verify.json"
jq -e '.data.scannedChains == 1 and .data.healthyChains == 1 and .data.failedChains == 0 and .data.chains[0].status == "healthy"' "$WORK_DIR/restored-verify.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_audit_verifications WHERE tenant_id = 2 AND status = 'failed'")" -ge 1
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_audit_verifications WHERE tenant_id = 2 AND status = 'healthy'")" -ge 2

"$MAINTENANCE_BIN" -dsn "$DSN" -action verify-audit-integrity -tenant-id 2 -audit-integrity-limit 50 >"$WORK_DIR/maintenance.out"
grep -q $'action\tverify-audit-integrity' "$WORK_DIR/maintenance.out"
grep -q $'failed_chains\t0' "$WORK_DIR/maintenance.out"
grep -q $'healthy_chains\t1' "$WORK_DIR/maintenance.out"
"$MAINTENANCE_BIN" -dsn "$DSN" -action create-audit-anchor -tenant-id 2 -audit-anchor-limit 50 -audit-anchor-artifact-root "$AUDIT_ANCHOR_ROOT" -audit-anchor-hmac-key "$AUDIT_ANCHOR_KEY" -audit-anchor-hmac-key-id audit-smoke-q3 -audit-anchor-require-remote -audit-anchor-s3-endpoint "http://127.0.0.1:$MINIO_PORT" -audit-anchor-s3-bucket "$MINIO_BUCKET" -audit-anchor-s3-access-key-id "$MINIO_ACCESS_KEY" -audit-anchor-s3-secret-access-key "$MINIO_SECRET_KEY" -audit-anchor-s3-use-ssl=false -audit-anchor-s3-prefix "$AUDIT_ANCHOR_PREFIX" -audit-anchor-s3-retention-mode compliance -audit-anchor-s3-retention-days 7 >"$WORK_DIR/anchor-maintenance-create.out"
grep -q $'action\tcreate-audit-anchor' "$WORK_DIR/anchor-maintenance-create.out"
grep -q $'failed_artifacts\t0' "$WORK_DIR/anchor-maintenance-create.out"
grep -q $'remote_exported_artifacts\t1' "$WORK_DIR/anchor-maintenance-create.out"
grep -q $'remote_failed_artifacts\t0' "$WORK_DIR/anchor-maintenance-create.out"
"$MAINTENANCE_BIN" -dsn "$DSN" -action verify-audit-anchor -tenant-id 2 -audit-anchor-limit 50 -audit-anchor-artifact-root "$AUDIT_ANCHOR_ROOT" -audit-anchor-hmac-key "$AUDIT_ANCHOR_KEY" -audit-anchor-hmac-key-id audit-smoke-q3 -audit-anchor-require-remote -audit-anchor-s3-endpoint "http://127.0.0.1:$MINIO_PORT" -audit-anchor-s3-bucket "$MINIO_BUCKET" -audit-anchor-s3-access-key-id "$MINIO_ACCESS_KEY" -audit-anchor-s3-secret-access-key "$MINIO_SECRET_KEY" -audit-anchor-s3-use-ssl=false -audit-anchor-s3-prefix "$AUDIT_ANCHOR_PREFIX" -audit-anchor-s3-retention-mode compliance -audit-anchor-s3-retention-days 7 >"$WORK_DIR/anchor-maintenance-verify.out"
grep -q $'action\tverify-audit-anchor' "$WORK_DIR/anchor-maintenance-verify.out"
grep -q $'failed_checkpoints\t0' "$WORK_DIR/anchor-maintenance-verify.out"
grep -q $'missing_remote_artifacts\t0' "$WORK_DIR/anchor-maintenance-verify.out"
grep -q $'orphan_remote_artifacts\t0' "$WORK_DIR/anchor-maintenance-verify.out"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'aria-label="审计完整性治理"' "$WORK_DIR/page.html"
grep -q 'id="verifyAuditIntegrity"' "$WORK_DIR/page.html"
grep -q 'id="createAuditAnchor"' "$WORK_DIR/page.html"
grep -q 'id="verifyAuditAnchor"' "$WORK_DIR/page.html"
grep -q 'id="auditAnchorRemoteSummary"' "$WORK_DIR/page.html"
grep -q 'id="auditAnchorRetentionSummary"' "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/auditIntegrity' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/auditIntegrityVerify' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/auditIntegrityVerify' "$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/auditAnchors' "$WORK_DIR/routes.json"
grep -q 'POST /dashboard/saasAdmin/auditAnchor' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/auditAnchor' "$WORK_DIR/routes.json"
grep -q 'SaaS audit integrity verification completed' "$GO_LOG"
grep -q 'SaaS audit anchor completed' "$GO_LOG"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.audit.integrity.verify' AND integrity_version = 1")" -ge 3

echo "SaaS audit integrity and remote immutable signed anchor smoke passed"
