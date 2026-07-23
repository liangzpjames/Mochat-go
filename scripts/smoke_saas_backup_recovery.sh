#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-backup-recovery-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13409}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26459}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18183}"
MINIO_PORT="${MOCHAT_MINIO_PORT:-19183}"
MINIO_CONTAINER="${PROJECT_NAME}-minio"
MINIO_IMAGE="${MOCHAT_MINIO_IMAGE:-minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e}"
MINIO_MC_IMAGE="${MOCHAT_MINIO_MC_IMAGE:-minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727}"
MINIO_ACCESS_KEY="backup-smoke-access"
MINIO_SECRET_KEY="backup-smoke-secret-0051"
MINIO_BUCKET="mochat-backup-smoke"
DATABASE="mochat_saas_backup_recovery"
RESTORE_DATABASE="mochat_restore_backup_recovery"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-backup-recovery.XXXXXX")"
BACKUP_ROOT="$WORK_DIR/backups"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
MAINTENANCE_BIN="$WORK_DIR/mochat-saas-maintenance"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-backup-recovery-jwt-secret}"
OLD_BACKUP_KEY="0808080808080808080808080808080808080808080808080808080808080808"
BACKUP_KEY="${MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY:-0909090909090909090909090909090909090909090909090909090909090909}"
BACKUP_KEY_RING="{\"backup-smoke-q2\":\"$OLD_BACKUP_KEY\",\"backup-smoke-q3\":\"$BACKUP_KEY\"}"
WRONG_BACKUP_KEY="1010101010101010101010101010101010101010101010101010101010101010"
AUDIT_ANCHOR_KEY="abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
PLAINTEXT_MARKER="BACKUP-PLAINTEXT-MARKER-0051"

compose() {
  MOCHAT_MYSQL_PORT="$MYSQL_PORT" MOCHAT_REDIS_PORT="$REDIS_PORT" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
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
trap 'echo "SaaS backup recovery smoke failed at line $LINENO" >&2; tail -220 "$GO_LOG" >&2 || true' ERR
trap cleanup EXIT INT TERM

resolve_binary() {
  local configured="$1"
  shift
  if [ -n "$configured" ] && [ -x "$configured" ]; then
    printf '%s\n' "$configured"
    return 0
  fi
  local name candidate
  for name in "$@"; do
    if candidate="$(command -v "$name" 2>/dev/null)"; then
      printf '%s\n' "$candidate"
      return 0
    fi
    for candidate in \
      "/opt/homebrew/opt/mysql-client/bin/$name" \
      "/usr/local/opt/mysql-client/bin/$name" \
      "/opt/homebrew/bin/$name" \
      "/usr/local/bin/$name"; do
      if [ -x "$candidate" ]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done
  done
  echo "required MySQL client binary was not found: $*" >&2
  return 1
}

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
    if [ "$status" = "healthy" ]; then
      return 0
    fi
    sleep 2
  done
  compose logs --tail=120 "$service" >&2 || true
  return 1
}

wait_url() {
  local url="$1" expected="$2" deadline=$((SECONDS + 90))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local status
    status="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$status" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_mysql_at_least() {
  local query="$1" minimum="$2" deadline=$((SECONDS + 120)) value="0"
  while [ "$SECONDS" -lt "$deadline" ]; do
    value="$(mysql_scalar "$query" || true)"
    if [[ "$value" =~ ^[0-9]+$ ]] && [ "$value" -ge "$minimum" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for MySQL value >= $minimum; last value=$value" >&2
  echo "$query" >&2
  return 1
}

mysql_root() {
  compose exec -T mysql mariadb -uroot -pmochat_root "$@"
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$DATABASE" -e "$1" </dev/null | tr -d '\r'
}

restore_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B "$RESTORE_DATABASE" -e "$1" </dev/null | tr -d '\r'
}

login_token() {
  local phone="$1" password="$2" output="$3"
  curl -sS -f -H 'Content-Type: application/json' \
    -d "{\"phone\":\"$phone\",\"password\":\"$password\"}" \
    "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_write() {
  local method="$1" token="$2" path="$3" body="$4" output="$5" expected="${6:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" \
    -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "$method $path returned $status" >&2; cat "$output" >&2; return 1; }
}

minio_mc() {
  local command="$1"
  docker run --rm -v "$WORK_DIR:/work:ro" --entrypoint /bin/sh "$MINIO_MC_IMAGE" -c \
    "mc alias set smoke http://host.docker.internal:$MINIO_PORT '$MINIO_ACCESS_KEY' '$MINIO_SECRET_KEY' >/dev/null && mc $command"
}

file_mode() {
  local path="$1"
  if stat -f '%Lp' "$path" >/dev/null 2>&1; then
    stat -f '%Lp' "$path"
  else
    stat -c '%a' "$path"
  fi
}

DUMP_BINARY="$(resolve_binary "${MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY:-}" mysqldump mariadb-dump)"
RESTORE_BINARY="$(resolve_binary "${MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY:-}" mysql mariadb)"

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
  "mc alias set smoke http://host.docker.internal:$MINIO_PORT '$MINIO_ACCESS_KEY' '$MINIO_SECRET_KEY' >/dev/null && mc mb --ignore-existing smoke/$MINIO_BUCKET >/dev/null"

mysql_root <<SQL
DROP DATABASE IF EXISTS $DATABASE;
DROP DATABASE IF EXISTS $RESTORE_DATABASE;
CREATE DATABASE $DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE $RESTORE_DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON $DATABASE.* TO 'mochat'@'%';
GRANT ALL PRIVILEGES ON $RESTORE_DATABASE.* TO 'mochat'@'%';
DROP USER IF EXISTS 'restore_admin'@'%';
CREATE USER 'restore_admin'@'%' IDENTIFIED BY 'restore_admin_pass';
GRANT ALL PRIVILEGES ON *.* TO 'restore_admin'@'%';
FLUSH PRIVILEGES;
SQL

env -u GOROOT go build -o "$MIGRATE_BIN" ./cmd/mochat-migrate
env -u GOROOT go build -o "$BOOTSTRAP_BIN" ./cmd/mochat-bootstrap
env -u GOROOT go build -o "$MAINTENANCE_BIN" ./cmd/mochat-saas-maintenance
env -u GOROOT go build -o "$GO_BIN" ./cmd/mochat-go

DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$DATABASE?parseTime=true&loc=Local"
RESTORE_DSN="mochat:mochat_pass@tcp(127.0.0.1:$MYSQL_PORT)/$RESTORE_DATABASE?parseTime=true&loc=Local"
"$MIGRATE_BIN" -dsn "$DSN" -project-root "$PWD" -action apply >"$WORK_DIR/migrate.out"
grep -q $'0052_saas_compliance_lifecycle\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0068_wechat_open_credential_encryption\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0069_saas_release_candidate_approval\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0073_saas_compliance_export_deletion_saga\tapplied_now' "$WORK_DIR/migrate.out"
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
grep -q $'0090_saas_payment_order_create_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0091_saas_payment_settlement_close_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0092_saas_payment_settlement_reopen_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0093_saas_payment_settlement_resolve_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0094_saas_tenant_domain_command_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0096_saas_tenant_enable_approval_guard\tapplied_now' "$WORK_DIR/migrate.out"
grep -q $'0097_saas_release_evidence_action_tracking\tapplied_now' "$WORK_DIR/migrate.out"
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.backups.read'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.backups.manage'")" = "1"

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" \
  -tenant-id 1 -tenant-name '灾备验收平台' -phone 13800000060 -password secret060 \
  -user-name "$PLAINTEXT_MARKER" -role-name '平台超级管理员' \
  -package-code backup-platform -package-name '平台版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-platform.out"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" \
  -tenant-id 2 -tenant-name '灾备隔离租户' -phone 13800000061 -password secret061 \
  -user-name '业务租户管理员' -role-name '租户超级管理员' \
  -package-code backup-business -package-name '业务版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-business.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000062', password, '平台灾备运维', 0, '运维部', '灾备运维', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000060' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000063', password, '平台灾备审计', 0, '审计部', '灾备审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000060' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000064', password, '灾备审批人一', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000060' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000065', password, '灾备审批人二', 0, '内控部', '审批', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000060' LIMIT 1;
SQL

OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000062'")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000063'")"
APPROVER1_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000064'")"
APPROVER2_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000065'")"
OPERATOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"
APPROVER_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_approver'")"
mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($AUDITOR_ID, 1, 0, NOW(), NOW()),
       ($APPROVER1_ID, 1, 0, NOW(), NOW()), ($APPROVER2_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $OPERATOR_ROLE_ID, 0, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 0, NOW()),
       ($APPROVER1_ID, $APPROVER_ROLE_ID, 0, NOW()), ($APPROVER2_ID, $APPROVER_ROLE_ID, 0, NOW());
SQL

mkdir -p "$BACKUP_ROOT"
env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$OLD_BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=backup-smoke-q2 \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT="http://127.0.0.1:$MINIO_PORT" \
  MOCHAT_GO_SAAS_BACKUP_S3_BUCKET="$MINIO_BUCKET" \
  MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID="$MINIO_ACCESS_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL=0 \
  MOCHAT_GO_SAAS_BACKUP_S3_PREFIX=smoke/database \
  "$MAINTENANCE_BIN" -action backup-create >"$WORK_DIR/old-key-backup.out"
OLD_KEY_RUN_ID="$(awk -F '\t' '$1 == "backup_run_id" {print $2}' "$WORK_DIR/old-key-backup.out")"
test -n "$OLD_KEY_RUN_ID"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_runs WHERE id = $OLD_KEY_RUN_ID AND encryption_key_id = 'backup-smoke-q2' AND replica_status = 'succeeded'")" = "1"
mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_backup_runs SET finished_at = DATE_SUB(NOW(), INTERVAL 2 DAY), verified_at = DATE_SUB(NOW(), INTERVAL 2 DAY), replicated_at = DATE_SUB(NOW(), INTERVAL 2 DAY), replica_verified_at = DATE_SUB(NOW(), INTERVAL 2 DAY) WHERE id = $OLD_KEY_RUN_ID"

env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ADDR="$GO_ADDR" \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_REDIS_ADDR="127.0.0.1:$REDIS_PORT" \
  MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
  MOCHAT_GO_MIGRATE_AUTH=1 \
  MOCHAT_GO_MIGRATE_LOGIN_SHOW=1 \
  MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD=1 \
  MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=1 \
  MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID=1 \
  MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT="$WORK_DIR/audit-anchors" \
  MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY="$AUDIT_ANCHOR_KEY" \
  MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID=backup-smoke-audit \
  MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON=1 \
  MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS=3600 \
  MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START=1 \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS="$BACKUP_KEY_RING" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=backup-smoke-q3 \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN="restore_admin:restore_admin_pass@tcp(127.0.0.1:$MYSQL_PORT)/?parseTime=true&loc=Local" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION=1 \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX=mochat_restore_ \
  MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT="http://127.0.0.1:$MINIO_PORT" \
  MOCHAT_GO_SAAS_BACKUP_S3_BUCKET="$MINIO_BUCKET" \
  MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID="$MINIO_ACCESS_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL=0 \
  MOCHAT_GO_SAAS_BACKUP_S3_PREFIX=smoke/database \
  "$GO_BIN" >"$GO_LOG" 2>&1 &
GO_PID="$!"
wait_url "http://$GO_ADDR/readyz" 200
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_saas_backup_runs WHERE trigger_type = 'cron' AND status = 'succeeded' AND verification_status = 'passed'" 1
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-backup' AND status = 'succeeded'" 1

CRON_RUN_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_backup_runs WHERE trigger_type = 'cron' ORDER BY id DESC LIMIT 1")"
CRON_ARTIFACT_NAME="$(mysql_scalar "SELECT artifact_name FROM mochat_go_saas_backup_runs WHERE id = $CRON_RUN_ID")"
CRON_OBJECT_KEY="$(mysql_scalar "SELECT replica_object_key FROM mochat_go_saas_backup_runs WHERE id = $CRON_RUN_ID")"
CRON_ARTIFACT="$BACKUP_ROOT/$CRON_ARTIFACT_NAME"
test -f "$CRON_ARTIFACT"
test "$(file_mode "$CRON_ARTIFACT")" = "600"
test -n "$CRON_OBJECT_KEY"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_runs WHERE id = $CRON_RUN_ID AND replica_status = 'succeeded'")" = "1"

SUPER_TOKEN="$(login_token 13800000060 secret060 "$WORK_DIR/super-auth.json")"
BUSINESS_TOKEN="$(login_token 13800000061 secret061 "$WORK_DIR/business-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000062 secret060 "$WORK_DIR/operator-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000063 secret060 "$WORK_DIR/auditor-auth.json")"
APPROVER1_TOKEN="$(login_token 13800000064 secret060 "$WORK_DIR/approver1-auth.json")"
APPROVER2_TOKEN="$(login_token 13800000065 secret060 "$WORK_DIR/approver2-auth.json")"

api_get "$BUSINESS_TOKEN" '/dashboard/saasAdmin/backupOverview' "$WORK_DIR/business-overview.json" 403
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/backupOverview' "$WORK_DIR/auditor-overview.json"
jq -e '.data.config.cronEnabled == true and .data.config.cronIntervalSeconds == 3600 and .data.config.cronRunOnStart == true' "$WORK_DIR/auditor-overview.json" >/dev/null
api_write POST "$AUDITOR_TOKEN" '/dashboard/saasAdmin/backupRun' '{"action":"create"}' "$WORK_DIR/auditor-create.json" 403
api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupOverview' "$WORK_DIR/operator-overview.json"
jq -e '.data.config.encryptionConfigured == true and .data.config.encryptionKeyCount == 2 and .data.config.restoreConfigured == true and .data.config.restoreAutoProvision == true and .data.config.restoreAdminConfigured == true and .data.config.replicaConfigured == true and .data.config.replicaProvider == "s3" and .data.config.replicaBucket == "mochat-backup-smoke" and .data.config.dumpToolReady == true and .data.config.restoreToolReady == true and .data.summary.successfulCount >= 2 and .data.summary.replicaSucceededCount >= 2' "$WORK_DIR/operator-overview.json" >/dev/null

api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"verify\",\"backupRunId\":$OLD_KEY_RUN_ID}" "$WORK_DIR/old-key-verify.json"
jq -e '.data.run.verificationStatus == "passed" and .data.run.encryptionKeyId == "backup-smoke-q2" and .data.run.encryptionKeyReady == true and .data.run.replicaStatus == "succeeded"' "$WORK_DIR/old-key-verify.json" >/dev/null
api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupOverview?limit=20' "$WORK_DIR/keyring-overview.json"
jq -e --argjson run_id "$OLD_KEY_RUN_ID" 'any(.data.runs[]; .id == $run_id and .encryptionKeyReady == true)' "$WORK_DIR/keyring-overview.json" >/dev/null
mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_backup_runs SET finished_at = NOW(), verified_at = NOW(), replicated_at = NOW(), replica_verified_at = NOW() WHERE id = $OLD_KEY_RUN_ID"

api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' '{"action":"create"}' "$WORK_DIR/manual-create.json" 201
jq -e '.data.run.status == "succeeded" and .data.run.encrypted == true and .data.run.encryptionKeyId == "backup-smoke-q3" and .data.run.encryptionKeyReady == true and .data.run.verificationStatus == "passed" and .data.run.replicaStatus == "succeeded" and .data.run.replicaProvider == "s3" and .data.run.replicaBucket == "mochat-backup-smoke" and (.data.run.replicaObjectKey | length) > 0 and .data.run.replicaSha256 == .data.run.sha256 and .data.run.replicaSizeBytes == .data.run.sizeBytes and .data.run.migrationVersion == "0097_saas_release_evidence_action_tracking" and .data.run.migrationCount == 97 and (.data.run.sha256 | length) == 64 and .data.run.sizeBytes > 0' "$WORK_DIR/manual-create.json" >/dev/null
MANUAL_RUN_ID="$(jq -er '.data.run.id' "$WORK_DIR/manual-create.json")"
MANUAL_ARTIFACT_NAME="$(jq -er '.data.run.artifactName' "$WORK_DIR/manual-create.json")"
MANUAL_OBJECT_KEY="$(jq -er '.data.run.replicaObjectKey' "$WORK_DIR/manual-create.json")"
MANUAL_SHA256="$(jq -er '.data.run.sha256' "$WORK_DIR/manual-create.json")"
MANUAL_ARTIFACT="$BACKUP_ROOT/$MANUAL_ARTIFACT_NAME"
test -f "$MANUAL_ARTIFACT"
test "$(file_mode "$MANUAL_ARTIFACT")" = "600"
test "$(xxd -p -l 8 "$MANUAL_ARTIFACT")" = "4d47424b01000000"
if grep -aFq "$PLAINTEXT_MARKER" "$MANUAL_ARTIFACT"; then
  echo "encrypted backup exposed a plaintext database marker" >&2
  exit 1
fi
test -z "$(find "$BACKUP_ROOT" -maxdepth 1 -name '*.partial' -print -quit)"
minio_mc "stat smoke/$MINIO_BUCKET/$MANUAL_OBJECT_KEY >/dev/null"

api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"verify\",\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/manual-verify.json"
jq -e '.data.run.verificationStatus == "passed"' "$WORK_DIR/manual-verify.json" >/dev/null

rm -f "$MANUAL_ARTIFACT"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"verify\",\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/hydrate-verify.json"
jq -e '.data.run.verificationStatus == "passed" and .data.run.replicaStatus == "succeeded"' "$WORK_DIR/hydrate-verify.json" >/dev/null
test -f "$MANUAL_ARTIFACT"
test "$(file_mode "$MANUAL_ARTIFACT")" = "600"
test "$(shasum -a 256 "$MANUAL_ARTIFACT" | awk '{print $1}')" = "$MANUAL_SHA256"
test -z "$(find "$BACKUP_ROOT" -maxdepth 1 -name '*.hydrate.partial' -print -quit)"

printf 'tampered-offsite-replica\n' >"$WORK_DIR/tampered.bin"
minio_mc "cp /work/tampered.bin smoke/$MINIO_BUCKET/$MANUAL_OBJECT_KEY >/dev/null"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"verify\",\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/tampered-verify.json" 500
grep -q 'checksum mismatch' "$WORK_DIR/tampered-verify.json"
test "$(mysql_scalar "SELECT verification_status FROM mochat_go_saas_backup_runs WHERE id = $MANUAL_RUN_ID")" = "failed"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"replicate\",\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/replica-repair.json"
jq -e '.data.run.verificationStatus == "passed" and .data.run.replicaStatus == "succeeded" and .data.run.replicaSha256 == .data.run.sha256 and .data.run.replicaSizeBytes == .data.run.sizeBytes' "$WORK_DIR/replica-repair.json" >/dev/null
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"verify\",\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/repaired-verify.json"
jq -e '.data.run.verificationStatus == "passed" and .data.run.replicaStatus == "succeeded"' "$WORK_DIR/repaired-verify.json" >/dev/null

SOURCE_TABLE_COUNT="$(mysql_scalar "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DATABASE'")"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/restoreDrill' "{\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/restore-success.json"
jq -e '.data.drill.status == "succeeded" and .data.drill.actualMigrationVersion == "0097_saas_release_evidence_action_tracking" and .data.drill.actualMigrationCount == 97 and .data.drill.checks.passed == true and .data.drill.targetLifecycle == "ephemeral" and .data.drill.targetCleanupStatus == "succeeded" and .data.drill.checks.targetCleanupPassed == true and (.data.drill.targetCleanedAt | length) > 0 and (.data.drill.targetFingerprint | length) == 64' "$WORK_DIR/restore-success.json" >/dev/null
AUTO_RESTORE_DATABASE="$(jq -er '.data.drill.targetDatabase' "$WORK_DIR/restore-success.json")"
test "$(mysql_scalar "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = '$AUTO_RESTORE_DATABASE'")" = "0"

env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS="$BACKUP_KEY_RING" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=backup-smoke-q3 \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN="$RESTORE_DSN" \
  MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT="http://127.0.0.1:$MINIO_PORT" \
  MOCHAT_GO_SAAS_BACKUP_S3_BUCKET="$MINIO_BUCKET" \
  MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID="$MINIO_ACCESS_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL=0 \
  MOCHAT_GO_SAAS_BACKUP_S3_PREFIX=smoke/database \
  "$MAINTENANCE_BIN" -action backup-restore-drill -backup-run-id "$MANUAL_RUN_ID" >"$WORK_DIR/preconfigured-restore.out"
grep -q $'status\tsucceeded' "$WORK_DIR/preconfigured-restore.out"
grep -q $'target_lifecycle\tpreconfigured' "$WORK_DIR/preconfigured-restore.out"
grep -q $'target_cleanup_status\tnot_required' "$WORK_DIR/preconfigured-restore.out"
test "$(restore_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "97"
test "$(restore_scalar "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$RESTORE_DATABASE'")" = "$SOURCE_TABLE_COUNT"
test "$(restore_scalar "SELECT COUNT(*) FROM mc_user WHERE name = '$PLAINTEXT_MARKER'")" = "1"

if env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS="$BACKUP_KEY_RING" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=backup-smoke-q3 \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN="$RESTORE_DSN" \
  MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT="http://127.0.0.1:$MINIO_PORT" \
  MOCHAT_GO_SAAS_BACKUP_S3_BUCKET="$MINIO_BUCKET" \
  MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID="$MINIO_ACCESS_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL=0 \
  MOCHAT_GO_SAAS_BACKUP_S3_PREFIX=smoke/database \
  "$MAINTENANCE_BIN" -action backup-restore-drill -backup-run-id "$MANUAL_RUN_ID" >"$WORK_DIR/restore-nonempty.out" 2>"$WORK_DIR/restore-nonempty.err"; then
  echo "restore drill unexpectedly accepted a non-empty target database" >&2
  exit 1
fi
grep -q '恢复演练目标库必须为空' "$WORK_DIR/restore-nonempty.err"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_restore_drills WHERE status = 'failed' AND error_message LIKE '%目标库必须为空%'")" = "1"

if env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$WRONG_BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  "$MAINTENANCE_BIN" -action backup-verify -backup-run-id "$MANUAL_RUN_ID" >"$WORK_DIR/wrong-key.out" 2>"$WORK_DIR/wrong-key.err"; then
  echo "backup verification unexpectedly succeeded with the wrong key" >&2
  exit 1
fi
grep -Eq 'verify backup|authentication failed|backup artifact' "$WORK_DIR/wrong-key.err"
test "$(mysql_scalar "SELECT verification_status FROM mochat_go_saas_backup_runs WHERE id = $MANUAL_RUN_ID")" = "failed"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' "{\"action\":\"verify\",\"backupRunId\":$MANUAL_RUN_ID}" "$WORK_DIR/reverify.json"
jq -e '.data.run.verificationStatus == "passed"' "$WORK_DIR/reverify.json" >/dev/null

if env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN="$DSN" \
  "$MAINTENANCE_BIN" -action backup-restore-drill -backup-run-id "$MANUAL_RUN_ID" >"$WORK_DIR/same-db.out" 2>"$WORK_DIR/same-db.err"; then
  echo "restore drill unexpectedly accepted the source database as target" >&2
  exit 1
fi
grep -q '恢复目标数据库必须使用前缀 mochat_restore_' "$WORK_DIR/same-db.err"

if env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX=mochat_ \
  "$MAINTENANCE_BIN" -action backup-restore-drill -backup-run-id "$MANUAL_RUN_ID" >"$WORK_DIR/same-db-prefix.out" 2>"$WORK_DIR/same-db-prefix.err"; then
  echo "restore drill unexpectedly accepted the source database after the test prefix was widened" >&2
  exit 1
fi
grep -q '恢复目标不能与源数据库相同' "$WORK_DIR/same-db-prefix.err"

BACKUP_POLICY_BODY='{"status":"active","intervalMinutes":1440,"retentionDays":1,"minSuccessfulBackups":1,"maxBackupAgeMinutes":1800,"restoreDrillIntervalDays":30,"requireEncryption":true,"requireOffsiteReplica":true,"expectedVersion":1}'
api_write PUT "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupPolicy' "$BACKUP_POLICY_BODY" "$WORK_DIR/policy-direct.json" 428
jq -e '.data.actionType == "backup.policy.update" and .data.requiredApprovals == 2' "$WORK_DIR/policy-direct.json" >/dev/null
POLICY_APPROVAL_BODY="$(jq -cn --argjson payload "$BACKUP_POLICY_BODY" '{actionType:"backup.policy.update",payload:$payload,reason:"调整灾备保留策略",expiresInHours:12,idempotencyKey:"backup-recovery-policy"}')"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/approvalRequest' "$POLICY_APPROVAL_BODY" "$WORK_DIR/policy-request.json"
POLICY_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/policy-request.json")"
api_write POST "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$POLICY_APPROVAL_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"灾备策略第一票\"}" "$WORK_DIR/policy-vote-one.json"
api_write POST "$APPROVER2_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$POLICY_APPROVAL_ID,\"expectedVersion\":2,\"decision\":\"approve\",\"reason\":\"灾备策略第二票\"}" "$WORK_DIR/policy-vote-two.json"
api_write POST "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$POLICY_APPROVAL_ID,\"expectedVersion\":3}" "$WORK_DIR/policy-update.json"
jq -e '.data.approval.status == "executed" and .data.result.policy.version == 2 and .data.result.policy.retentionDays == 1 and .data.result.policy.minSuccessfulBackups == 1 and .data.result.policy.requireOffsiteReplica == true' "$WORK_DIR/policy-update.json" >/dev/null

mysql_root "$DATABASE" -e "UPDATE mochat_go_saas_backup_runs SET finished_at = DATE_SUB(NOW(), INTERVAL 2 DAY), verified_at = DATE_SUB(NOW(), INTERVAL 2 DAY) WHERE id = $CRON_RUN_ID"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/backupRun' '{"action":"cleanup"}' "$WORK_DIR/cleanup-direct.json" 428
jq -e '.data.actionType == "backup.retention.cleanup" and .data.requiredApprovals == 2' "$WORK_DIR/cleanup-direct.json" >/dev/null
CLEANUP_APPROVAL_BODY='{"actionType":"backup.retention.cleanup","payload":{},"reason":"清理过期平台数据库备份","expiresInHours":12,"idempotencyKey":"backup-recovery-cleanup"}'
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/approvalRequest' "$CLEANUP_APPROVAL_BODY" "$WORK_DIR/cleanup-request.json"
CLEANUP_APPROVAL_ID="$(jq -r '.data.approval.id' "$WORK_DIR/cleanup-request.json")"
jq -e --argjson run_id "$CRON_RUN_ID" '.data.approval.actionType == "backup.retention.cleanup" and .data.approval.requiredApprovals == 2 and (.data.approval.request.items | length) == 1 and .data.approval.request.items[0].backupRunId == $run_id' "$WORK_DIR/cleanup-request.json" >/dev/null
api_write POST "$APPROVER1_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$CLEANUP_APPROVAL_ID,\"expectedVersion\":1,\"decision\":\"approve\",\"reason\":\"清理范围第一票\"}" "$WORK_DIR/cleanup-vote-one.json"
api_write POST "$APPROVER2_TOKEN" '/dashboard/saasAdmin/approvalDecision' "{\"approvalId\":$CLEANUP_APPROVAL_ID,\"expectedVersion\":2,\"decision\":\"approve\",\"reason\":\"清理范围第二票\"}" "$WORK_DIR/cleanup-vote-two.json"
api_write POST "$SUPER_TOKEN" '/dashboard/saasAdmin/approvalExecute' "{\"approvalId\":$CLEANUP_APPROVAL_ID,\"expectedVersion\":3}" "$WORK_DIR/cleanup.json"
jq -e '.data.approval.status == "executed" and .data.result.cleanupRun.status == "succeeded" and .data.result.cleanupRun.candidateCount == 1 and .data.result.cleanupRun.deletedCount == 1 and .data.result.cleanupRun.replicasDeletedCount == 1 and .data.result.cleanupRun.failedCount == 0' "$WORK_DIR/cleanup.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_cleanup_runs WHERE approval_id = $CLEANUP_APPROVAL_ID AND status = 'succeeded' AND candidate_count = 1 AND deleted_count = 1 AND failed_count = 0 AND active_slot IS NULL")" = "1"
CLEANUP_RUN_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_backup_cleanup_runs WHERE approval_id = $CLEANUP_APPROVAL_ID")"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_cleanup_items WHERE cleanup_run_id = $CLEANUP_RUN_ID AND backup_run_id = $CRON_RUN_ID AND status = 'succeeded' AND replica_status = 'deleted' AND record_status = 'deleted' AND operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approvals WHERE id = $CLEANUP_APPROVAL_ID AND status = 'executed' AND effect_applied_at IS NOT NULL AND effect_operation_id > 0")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_runs WHERE id = $CRON_RUN_ID AND status = 'deleted' AND cleanup_run_id IS NULL")" = "1"
test ! -e "$CRON_ARTIFACT"
test -f "$MANUAL_ARTIFACT"
if minio_mc "stat smoke/$MINIO_BUCKET/$CRON_OBJECT_KEY" >/dev/null 2>&1; then
  echo "retention cleanup left the expired offsite replica behind" >&2
  exit 1
fi

env -u GOROOT \
  MOCHAT_MYSQL_DSN="$DSN" \
  MOCHAT_GO_SAAS_BACKUP_ROOT="$BACKUP_ROOT" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS="$BACKUP_KEY_RING" \
  MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=backup-smoke-q3 \
  MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY="$DUMP_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY="$RESTORE_BINARY" \
  MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN="$RESTORE_DSN" \
  MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT="http://127.0.0.1:$MINIO_PORT" \
  MOCHAT_GO_SAAS_BACKUP_S3_BUCKET="$MINIO_BUCKET" \
  MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID="$MINIO_ACCESS_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY="$MINIO_SECRET_KEY" \
  MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL=0 \
  MOCHAT_GO_SAAS_BACKUP_S3_PREFIX=smoke/database \
  "$MAINTENANCE_BIN" -action backup-status >"$WORK_DIR/maintenance-status.out"
grep -q $'action\tbackup-status' "$WORK_DIR/maintenance-status.out"
grep -q $'encryption_configured\ttrue' "$WORK_DIR/maintenance-status.out"
grep -q $'encryption_key_count\t2' "$WORK_DIR/maintenance-status.out"
grep -q $'restore_configured\ttrue' "$WORK_DIR/maintenance-status.out"
grep -q $'replica_configured\ttrue' "$WORK_DIR/maintenance-status.out"
grep -q $'replica_provider\ts3' "$WORK_DIR/maintenance-status.out"

api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15' "$WORK_DIR/system-health.json"
jq -e '.data.summary.healthState == "healthy" and .data.summary.checkCount == 31 and .data.summary.issueCount == 0 and any(.data.checks[]; .code == "saas_alert_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wecom_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wechat_open_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "backup_automation" and .status == "healthy") and any(.data.checks[]; .code == "backup_retention_cleanup" and .status == "healthy") and ([.data.checks[].code] | index("service_account_key_protection")) != null and ([.data.checks[].code] | index("backup_freshness")) != null and ([.data.checks[].code] | index("backup_replica")) != null and ([.data.checks[].code] | index("backup_keyring")) != null and ([.data.checks[].code] | index("backup_replica_store")) != null and ([.data.checks[].code] | index("restore_drill_freshness")) != null and ([.data.checks[].code] | index("compliance_export_queue")) != null and ([.data.checks[].code] | index("compliance_erasure_queue")) != null and ([.data.checks[].code] | index("compliance_configuration")) != null and ([.data.checks[].code] | index("domain_delivery_queue")) != null and ([.data.checks[].code] | index("domain_certificate_lifecycle")) != null' "$WORK_DIR/system-health.json" >/dev/null
api_get "$SUPER_TOKEN" '/dashboard/saasAdmin/backupOverview?limit=20' "$WORK_DIR/final-overview.json"
jq -e --argjson run_id "$MANUAL_RUN_ID" --argjson old_run_id "$OLD_KEY_RUN_ID" --argjson cleanup_id "$CLEANUP_RUN_ID" '.data.summary.successfulCount == 2 and .data.summary.replicaSucceededCount == 2 and .data.summary.successfulDrillCount == 2 and .data.summary.cleanupCount == 1 and .data.summary.cleanupFailedCount == 0 and any(.data.runs[]; .id == $run_id and .verificationStatus == "passed" and .replicaStatus == "succeeded" and .encryptionKeyReady == true) and any(.data.runs[]; .id == $old_run_id and .encryptionKeyReady == true) and any(.data.cleanupRuns[]; .id == $cleanup_id and .status == "succeeded" and .deletedCount == 1)' "$WORK_DIR/final-overview.json" >/dev/null

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.create'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.policy.update'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.replicate'")" -ge "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.restore_drill'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.delete'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.cleanup.schedule'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.cleanup.finish'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.backup.verify'")" -ge "8"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_runs WHERE active_slot IS NOT NULL")" = "0"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_restore_drills WHERE active_slot IS NOT NULL")" = "0"

mysql_root "$DATABASE" -N -B -e "
SELECT CONCAT_WS('|', backup_no, artifact_name, encryption_key_id, sha256, verification_error, error_message) FROM mochat_go_saas_backup_runs;
SELECT CONCAT_WS('|', before_json, after_json, remark) FROM mochat_go_saas_admin_operation_logs WHERE action LIKE 'saas.admin.backup.%';
SELECT CONCAT_WS('|', drill_no, target_fingerprint, target_database, checks_json, error_message) FROM mochat_go_saas_restore_drills;
" >"$WORK_DIR/backup-ledger.txt"
for secret_value in "$OLD_BACKUP_KEY" "$BACKUP_KEY" "$MINIO_SECRET_KEY" restore_admin_pass; do
  if grep -Fq "$secret_value" "$WORK_DIR/backup-ledger.txt"; then
    echo "backup or object-storage secret leaked into database audit records" >&2
    exit 1
  fi
  if grep -Fq "$secret_value" "$GO_LOG"; then
    echo "backup or object-storage secret leaked into application logs" >&2
    exit 1
  fi
done

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q 'id="backupCenter"' "$WORK_DIR/page.html"
grep -q '自动调度' "$WORK_DIR/page.html"
grep -q 'id="backupRequireOffsiteReplica"' "$WORK_DIR/page.html"
grep -q '复制异地' "$WORK_DIR/page.html"
grep -q '自动创建临时隔离库，校验后立即销毁' "$WORK_DIR/page.html"
grep -q '/dashboard/saasAdmin/restoreDrill' "$WORK_DIR/page.html"
curl -sS -f "http://$GO_ADDR/compat/routes" >"$WORK_DIR/routes.json"
grep -q 'GET /dashboard/saasAdmin/backupOverview' "$WORK_DIR/routes.json"
grep -q 'PUT /dashboard/saasAdmin/restoreDrill' "$WORK_DIR/routes.json"
grep -q 'SaaS backup manager enabled: encryption_configured=true key_count=2 cron_enabled=true cron_interval=1h0m0s cron_run_on_start=true restore_configured=true restore_auto=true replica_configured=true replica_provider=s3' "$GO_LOG"
grep -q 'go cron enabled: SaaS encrypted database backup interval=1h0m0s run_on_start=true key_id=backup-smoke-q3' "$GO_LOG"
grep -q 'SaaS backup cron finished: backup_no=' "$GO_LOG"

echo "SaaS backup recovery smoke passed"
