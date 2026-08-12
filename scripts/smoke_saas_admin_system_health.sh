#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_STACK_PROJECT:-mochat-go-saas-admin-system-health-check}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13398}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26448}"
GO_ADDR="${MOCHAT_GO_ADDR:-127.0.0.1:18172}"
DATABASE="mochat_saas_admin_system_health"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-saas-admin-system-health.XXXXXX")"
MIGRATE_BIN="$WORK_DIR/mochat-migrate"
BOOTSTRAP_BIN="$WORK_DIR/mochat-bootstrap"
GO_BIN="$WORK_DIR/mochat-go"
GO_LOG="$WORK_DIR/go.log"
GO_PID=""
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-saas-admin-system-health-jwt-secret}"

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
trap 'echo "SaaS admin system health smoke failed at line $LINENO" >&2; tail -240 "$GO_LOG" >&2 || true' ERR
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
  exit 1
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

login_token() {
  local phone="$1" output="$2"
  curl -sS -f -H 'Content-Type: application/json' -d "{\"phone\":\"$phone\",\"password\":\"secret048\"}" "http://$GO_ADDR/dashboard/user/auth" >"$output"
  jq -er '.data.token' "$output"
}

api_get() {
  local token="$1" path="$2" output="$3" expected="${4:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -H "Authorization: Bearer $token" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "GET $path returned $status" >&2; cat "$output" >&2; return 1; }
}

api_write() {
  local method="$1" token="$2" path="$3" body="$4" output="$5" expected="${6:-200}" status
  status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$body" "http://$GO_ADDR$path")"
  test "$status" = "$expected" || { echo "$method $path returned $status" >&2; cat "$output" >&2; return 1; }
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
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER=0707070707070707070707070707070707070707070707070707070707070707 \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID=health-smoke \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER=1 \
    MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON=1 \
    MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS=3600 \
    MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_RUN_ON_START=1 \
    MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS=24 \
    MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES=15 \
    MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS=3 \
	    MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY=0808080808080808080808080808080808080808080808080808080808080808 \
	    MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID=health-smoke \
	    MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON=1 \
	    MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS=3600 \
	    MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START=0 \
	    MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT="$WORK_DIR/audit-anchors" \
	    MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY=0909090909090909090909090909090909090909090909090909090909090909 \
	    MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID=health-smoke \
    "$GO_BIN" >"$GO_LOG" 2>&1 &
  GO_PID="$!"
  wait_url "http://$GO_ADDR/readyz" 200
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
test "$(mysql_scalar 'SELECT COUNT(*) FROM mochat_go_schema_migrations')" = "98"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.system.read'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.system.manage'")" = "1"

"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 1 -tenant-name '平台健康验收' -phone 13800000048 -password secret048 -user-name '平台超级管理员' -role-name '平台超级管理员' -package-code health-platform -package-name '平台版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-platform.out"
"$BOOTSTRAP_BIN" -dsn "$DSN" -secret "$SECRET" -tenant-id 2 -tenant-name '业务租户' -phone 13800000049 -password secret048 -user-name '业务租户管理员' -role-name '租户超级管理员' -package-code health-business -package-name '业务版' -package-expires-at 2037-01-01 >"$WORK_DIR/bootstrap-business.out"

mysql_root "$DATABASE" <<'SQL'
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000050', password, '平台运维', 0, '运维部', '运维', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000048' LIMIT 1;
INSERT INTO mc_user (phone, password, name, gender, department, position, status, tenant_id, created_at, updated_at, isSuperAdmin)
SELECT '13800000051', password, '平台审计', 0, '审计部', '审计', 1, 1, NOW(), NOW(), 0 FROM mc_user WHERE phone = '13800000048' LIMIT 1;
SQL

SUPER_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000048'")"
OPERATOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000050'")"
AUDITOR_ID="$(mysql_scalar "SELECT id FROM mc_user WHERE phone = '13800000051'")"
OPERATOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_operations'")"
AUDITOR_ROLE_ID="$(mysql_scalar "SELECT id FROM mochat_go_saas_admin_roles WHERE code = 'platform_auditor'")"

mysql_root "$DATABASE" <<SQL
INSERT INTO mochat_go_saas_admin_user_access (user_id, version, updated_by, created_at, updated_at)
VALUES ($OPERATOR_ID, 1, 0, NOW(), NOW()), ($AUDITOR_ID, 1, 0, NOW(), NOW());
INSERT INTO mochat_go_saas_admin_user_roles (user_id, role_id, assigned_by, created_at)
VALUES ($OPERATOR_ID, $OPERATOR_ROLE_ID, 0, NOW()), ($AUDITOR_ID, $AUDITOR_ROLE_ID, 0, NOW());

INSERT INTO mochat_go_background_tasks (name, status, started_at, stopped_at, last_error, start_count, failure_count, created_at, updated_at)
VALUES ('smoke-failed-worker', 'failed', DATE_SUB(NOW(), INTERVAL 2 HOUR), NOW(), 'smoke worker failed', 1, 1, NOW(), NOW());
INSERT INTO mochat_go_background_task_executions
  (tenant_id, execution_id, task_name, kind, status, started_at, stopped_at, duration_ms, last_error, created_at, updated_at)
VALUES (1, 'system-health-smoke-execution', 'smoke-failed-worker', 'periodic_tick', 'failed', DATE_SUB(NOW(), INTERVAL 1 HOUR), NOW(), 1000, 'smoke execution failed', NOW(), NOW());

INSERT INTO mochat_go_saas_alert_notifications
  (notification_key, alert_key, tenant_id, channel, status, attempts, max_attempts, alert_json, last_error, next_retry_at, delivered_at, created_at, updated_at, deleted_at)
VALUES
  ('health-smoke-dead', 'health-smoke-dead', 1, 'webhook', 'dead', 3, 3, JSON_OBJECT('type', 'smoke'), 'attempts exhausted', NULL, NULL, DATE_SUB(NOW(), INTERVAL 2 HOUR), NOW(), NULL),
  ('health-smoke-failed', 'health-smoke-failed', 1, 'webhook', 'failed', 1, 3, JSON_OBJECT('type', 'smoke'), 'retry due', DATE_SUB(NOW(), INTERVAL 1 MINUTE), NULL, NOW(), NOW(), NULL),
  ('health-smoke-stale', 'health-smoke-stale', 1, 'webhook', 'pending', 0, 3, JSON_OBJECT('type', 'smoke'), '', NULL, NULL, DATE_SUB(NOW(), INTERVAL 1 HOUR), DATE_SUB(NOW(), INTERVAL 1 HOUR), NULL);

INSERT INTO mochat_go_saas_admin_approvals
  (request_no, action_type, risk_level, status, required_permission, target_type, target_id, target_name,
   requester_user_id, requester_tenant_id, idempotency_key, request_sha256, request_json, reason,
   expires_at, sla_due_at, next_reminder_at, version, created_at, updated_at)
VALUES
  ('APR-HEALTH-SMOKE', 'access.role.save', 'critical', 'pending', 'platform.access.manage', 'role', 'smoke', '健康检查验收',
   $SUPER_ID, 1, 'system-health-smoke-approval', REPEAT('a', 64), JSON_OBJECT('smoke', true), '健康检查验收',
   DATE_ADD(NOW(), INTERVAL 1 DAY), DATE_SUB(NOW(), INTERVAL 10 MINUTE), DATE_SUB(NOW(), INTERVAL 5 MINUTE), 1, NOW(), NOW());

INSERT INTO mochat_go_saas_admin_tasks
  (task_type, status, tenant_id, package_code, actor_user_id, actor_tenant_id, request_json, remark, last_error, created_at, updated_at)
VALUES
  ('tenant_provision', 'failed', 2, 'health-business', $OPERATOR_ID, 1, JSON_OBJECT('smoke', true), '健康验收失败任务', '执行失败', NOW(), NOW()),
  ('package_sync', 'blocked', 2, 'health-business', $OPERATOR_ID, 1, JSON_OBJECT('smoke', true), '健康验收阻断任务', '超额阻断', NOW(), NOW());

INSERT INTO mochat_go_saas_payment_settlement_sync_states
  (provider, active_run_id, last_attempt_at, last_success_at, last_error, version, created_at, updated_at)
VALUES ('health-smoke', NULL, NOW(), NULL, '渠道同步失败', 1, NOW(), NOW());
INSERT INTO mochat_go_saas_payment_settlement_sync_runs
  (run_no, provider, source, status, dry_run, started_at, finished_at, error_message, actor_user_id, actor_tenant_id, created_at, updated_at)
VALUES ('SSR-HEALTH-SMOKE', 'health-smoke', 'cron', 'failed', 0, DATE_SUB(NOW(), INTERVAL 30 MINUTE), NOW(), '渠道同步失败', 0, 1, NOW(), NOW());

INSERT INTO mochat_go_saas_backup_runs
  (backup_no, trigger_type, status, active_slot, artifact_name, artifact_format, encrypted, encryption_key_id,
   sha256, size_bytes, database_name, migration_version, migration_count, table_count, verification_status,
   verified_at, verification_error, error_message, started_at, finished_at, created_by, actor_tenant_id,
   operation_id, version, created_at, updated_at, deleted_at)
VALUES
  ('BKP-HEALTH-SMOKE', 'manual', 'succeeded', NULL, 'health-smoke.sql.gz.mgbk', 'sql.gz.mgbk', 1, 'health-smoke',
	   REPEAT('b', 64), 1024, '$DATABASE', '0092_saas_payment_settlement_reopen_guard', 92, 187, 'passed', NOW(), '', '',
   DATE_SUB(NOW(), INTERVAL 2 MINUTE), NOW(), $OPERATOR_ID, 1, 0, 1, NOW(), NOW(), NULL);

INSERT INTO mochat_go_saas_restore_drills
  (drill_no, backup_run_id, trigger_type, status, active_slot, target_fingerprint, target_database,
   target_lifecycle, target_cleanup_status, target_cleanup_error,
   expected_migration_version, actual_migration_version, expected_migration_count, actual_migration_count,
   expected_table_count, actual_table_count, checks_json, error_message, started_at, finished_at,
   duration_ms, created_by, actor_tenant_id, operation_id, created_at)
SELECT
  'RDR-HEALTH-SMOKE', id, 'manual', 'succeeded', NULL, REPEAT('c', 64), 'mochat_restore_health_smoke',
  'ephemeral', 'failed', 'smoke drop database failed',
	  '0092_saas_payment_settlement_reopen_guard', '0092_saas_payment_settlement_reopen_guard', 92, 92, 187, 187,
  JSON_OBJECT('migrationVersion', true, 'migrationCount', true, 'tableCount', true), '',
  DATE_SUB(NOW(), INTERVAL 1 MINUTE), NOW(), 1000, $OPERATOR_ID, 1, 0, NOW()
FROM mochat_go_saas_backup_runs WHERE backup_no = 'BKP-HEALTH-SMOKE';
SQL

start_go
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_saas_admin_health_scans WHERE trigger_type = 'cron'" 1
wait_mysql_at_least "SELECT COUNT(*) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-admin-system-health' AND status = 'succeeded'" 1

SUPER_TOKEN="$(login_token 13800000048 "$WORK_DIR/super-auth.json")"
BUSINESS_TOKEN="$(login_token 13800000049 "$WORK_DIR/business-auth.json")"
OPERATOR_TOKEN="$(login_token 13800000050 "$WORK_DIR/operator-auth.json")"
AUDITOR_TOKEN="$(login_token 13800000051 "$WORK_DIR/auditor-auth.json")"

api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/operator-access.json"
jq -e '(.data.profile.permissions | index("platform.system.read")) != null and (.data.profile.permissions | index("platform.system.manage")) != null' "$WORK_DIR/operator-access.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/accessProfile' "$WORK_DIR/auditor-access.json"
jq -e '(.data.profile.permissions | index("platform.system.read")) != null and (.data.profile.permissions | index("platform.system.manage")) == null' "$WORK_DIR/auditor-access.json" >/dev/null

api_get "$BUSINESS_TOKEN" '/dashboard/saasAdmin/systemHealth' "$WORK_DIR/business-health.json" 403
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/systemHealth' "$WORK_DIR/auditor-health.json"
jq -e '.data.summary.healthState == "critical" and .data.summary.checkCount == 30 and .data.summary.issueCount == 10 and (.data.incidents | length) == 10 and any(.data.checks[]; .code == "saas_alert_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wecom_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wechat_open_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "backup_automation" and .status == "healthy") and any(.data.checks[]; .code == "backup_retention_cleanup" and .status == "healthy") and ([.data.checks[].code] | index("service_account_key_protection")) != null and ([.data.checks[].code] | index("backup_replica")) != null and ([.data.checks[].code] | index("backup_keyring")) != null and ([.data.checks[].code] | index("compliance_export_queue")) != null and ([.data.checks[].code] | index("compliance_erasure_queue")) != null and ([.data.checks[].code] | index("compliance_configuration")) != null and ([.data.checks[].code] | index("audit_anchor_configuration")) != null and ([.data.checks[].code] | index("domain_delivery_queue")) != null and ([.data.checks[].code] | index("domain_certificate_lifecycle")) != null and ([.data.incidents[].incidentKey] | index("restore_drill_freshness")) != null' "$WORK_DIR/auditor-health.json" >/dev/null
api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemIncidents?status=active&severity=all&limit=100' "$WORK_DIR/operator-incidents.json"
jq -e '(.data.items | length) == 10 and ([.data.items[].incidentKey] | index("admin_tasks_failed")) != null and ([.data.items[].incidentKey] | index("settlement_sync_failed")) != null and ([.data.items[].incidentKey] | index("restore_drill_freshness")) != null' "$WORK_DIR/operator-incidents.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE JSON_UNQUOTE(JSON_EXTRACT(alert_json, '$.AlertType')) = 'system_health_incident'")" = "10"

api_write POST "$AUDITOR_TOKEN" '/dashboard/saasAdmin/systemHealthScan' '{"failureWindowHours":24,"notificationStaleMinutes":15,"notify":true}' "$WORK_DIR/auditor-scan.json" 403
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemHealthScan' '{"failureWindowHours":24,"notificationStaleMinutes":15,"notify":true}' "$WORK_DIR/manual-repeat.json"
jq -e '.data.scan.healthState == "critical" and .data.scan.issueCount == 10 and .data.openedCount == 0 and .data.reopenedCount == 0 and .data.notificationCount == 0' "$WORK_DIR/manual-repeat.json" >/dev/null

FAILED_INCIDENT_ID="$(jq -r '.data.incidents[] | select(.incidentKey == "admin_tasks_failed") | .id' "$WORK_DIR/manual-repeat.json")"
FAILED_INCIDENT_VERSION="$(jq -r '.data.incidents[] | select(.incidentKey == "admin_tasks_failed") | .version' "$WORK_DIR/manual-repeat.json")"
api_write PUT "$AUDITOR_TOKEN" '/dashboard/saasAdmin/systemIncident' "{\"incidentId\":$FAILED_INCIDENT_ID,\"action\":\"acknowledge\",\"expectedVersion\":$FAILED_INCIDENT_VERSION,\"owner\":\"审计越权\",\"note\":\"smoke\"}" "$WORK_DIR/auditor-incident.json" 403
api_write PUT "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemIncident' "{\"incidentId\":$FAILED_INCIDENT_ID,\"action\":\"acknowledge\",\"expectedVersion\":$FAILED_INCIDENT_VERSION,\"owner\":\"平台运维\",\"note\":\"已接手排查\"}" "$WORK_DIR/acknowledged.json"
jq -e '.data.incident.status == "acknowledged" and .data.incident.owner == "平台运维"' "$WORK_DIR/acknowledged.json" >/dev/null
api_write PUT "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemIncident' "{\"incidentId\":$FAILED_INCIDENT_ID,\"action\":\"assign\",\"expectedVersion\":$FAILED_INCIDENT_VERSION,\"owner\":\"过期版本\",\"note\":\"smoke\"}" "$WORK_DIR/stale-version.json" 409
ACK_VERSION="$(jq -r '.data.incident.version' "$WORK_DIR/acknowledged.json")"
api_write PUT "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemIncident' "{\"incidentId\":$FAILED_INCIDENT_ID,\"action\":\"resolve\",\"expectedVersion\":$ACK_VERSION,\"owner\":\"平台运维\",\"note\":\"人工标记已解决\"}" "$WORK_DIR/resolved.json"
jq -e '.data.incident.status == "resolved" and .data.incident.resolutionNote == "人工标记已解决"' "$WORK_DIR/resolved.json" >/dev/null

api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemHealthScan' '{"failureWindowHours":24,"notificationStaleMinutes":15,"notify":true}' "$WORK_DIR/reopened-scan.json"
jq -e '.data.scan.healthState == "critical" and .data.reopenedCount == 1 and .data.notificationCount == 1 and (.data.incidents[] | select(.incidentKey == "admin_tasks_failed") | .status) == "open"' "$WORK_DIR/reopened-scan.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE JSON_UNQUOTE(JSON_EXTRACT(alert_json, '$.AlertType')) = 'system_health_incident'")" = "11"
api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemHealthScan' '{"failureWindowHours":24,"notificationStaleMinutes":15,"notify":true}' "$WORK_DIR/idempotent-scan.json"
jq -e '.data.reopenedCount == 0 and .data.notificationCount == 0' "$WORK_DIR/idempotent-scan.json" >/dev/null
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_alert_notifications WHERE JSON_UNQUOTE(JSON_EXTRACT(alert_json, '$.AlertType')) = 'system_health_incident'")" = "11"

api_get "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemIncidents?status=active&severity=all&limit=100' "$WORK_DIR/incidents-before-assign.json"
BLOCKED_INCIDENT_ID="$(jq -r '.data.items[] | select(.incidentKey == "admin_tasks_blocked") | .id' "$WORK_DIR/incidents-before-assign.json")"
BLOCKED_INCIDENT_VERSION="$(jq -r '.data.items[] | select(.incidentKey == "admin_tasks_blocked") | .version' "$WORK_DIR/incidents-before-assign.json")"
api_write PUT "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemIncident' "{\"incidentId\":$BLOCKED_INCIDENT_ID,\"action\":\"assign\",\"expectedVersion\":$BLOCKED_INCIDENT_VERSION,\"owner\":\"运营支持组\",\"note\":\"分派超额任务\"}" "$WORK_DIR/assigned.json"
jq -e '.data.incident.status == "open" and .data.incident.owner == "运营支持组"' "$WORK_DIR/assigned.json" >/dev/null

mysql_root "$DATABASE" <<'SQL'
UPDATE mochat_go_background_tasks SET status = 'stopped', last_error = '', stopped_at = NOW() WHERE name = 'smoke-failed-worker';
UPDATE mochat_go_background_task_executions SET status = 'succeeded', last_error = '', stopped_at = NOW() WHERE execution_id = 'system-health-smoke-execution';
UPDATE mochat_go_saas_alert_notifications SET status = 'delivered', last_error = '', next_retry_at = NULL, delivered_at = NOW(), updated_at = NOW() WHERE status IN ('dead', 'failed', 'pending');
UPDATE mochat_go_saas_admin_approvals SET status = 'canceled', next_reminder_at = NULL, updated_at = NOW() WHERE request_no = 'APR-HEALTH-SMOKE';
UPDATE mochat_go_saas_admin_tasks SET status = 'applied', last_error = '', applied_at = NOW(), updated_at = NOW() WHERE remark LIKE '健康验收%';
UPDATE mochat_go_saas_payment_settlement_sync_states SET last_error = '', last_success_at = NOW(), updated_at = NOW() WHERE provider = 'health-smoke';
UPDATE mochat_go_saas_payment_settlement_sync_runs SET status = 'succeeded', error_message = '', finished_at = NOW(), updated_at = NOW() WHERE run_no = 'SSR-HEALTH-SMOKE';
UPDATE mochat_go_saas_restore_drills SET target_cleanup_status = 'succeeded', target_cleaned_at = NOW(), target_cleanup_error = '' WHERE drill_no = 'RDR-HEALTH-SMOKE';
SQL

api_write POST "$OPERATOR_TOKEN" '/dashboard/saasAdmin/systemHealthScan' '{"failureWindowHours":24,"notificationStaleMinutes":15,"notify":true}' "$WORK_DIR/recovered-scan.json"
jq -e '.data.scan.healthState == "healthy" and .data.scan.issueCount == 0 and .data.recoveredCount == 10 and .data.notificationCount == 0 and (.data.incidents | length) == 0' "$WORK_DIR/recovered-scan.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15' "$WORK_DIR/healthy.json"
jq -e '.data.summary.healthState == "healthy" and .data.summary.issueCount == 0 and .data.summary.healthyCount == 30 and (.data.incidents | length) == 0 and any(.data.checks[]; .code == "saas_alert_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wecom_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "wechat_open_credential_protection" and .status == "healthy") and any(.data.checks[]; .code == "backup_automation" and .status == "healthy") and any(.data.checks[]; .code == "backup_retention_cleanup" and .status == "healthy")' "$WORK_DIR/healthy.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/systemIncidents?status=resolved&severity=all&limit=100' "$WORK_DIR/resolved-incidents.json"
jq -e '(.data.items | length) == 10 and all(.data.items[]; .status == "resolved")' "$WORK_DIR/resolved-incidents.json" >/dev/null
api_get "$AUDITOR_TOKEN" '/dashboard/saasAdmin/systemHealthScans?limit=30' "$WORK_DIR/scans.json"
jq -e '(.data.items | length) >= 5 and (.data.items[0].healthState == "healthy")' "$WORK_DIR/scans.json" >/dev/null

test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.system_health.scan'")" = "5"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_operation_logs WHERE action = 'saas.admin.system_incident.update'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_system_incidents WHERE status = 'resolved'")" = "10"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_health_scans WHERE health_state = 'healthy'")" = "1"

curl -sS -f "http://$GO_ADDR/dashboard/saasAdmin/page" >"$WORK_DIR/page.html"
grep -q '平台健康中心' "$WORK_DIR/page.html"
grep -q '/dashboard/saasAdmin/systemHealthScan' "$WORK_DIR/page.html"
grep -q 'go cron enabled: SaaS system health interval=1h0m0s run_on_start=true failure_window_hours=24 notification_stale_minutes=15' "$GO_LOG"
grep -q 'SaaS system health cron finished: state=critical checks=30 issues=10 opened=10 reopened=0 recovered=0 notifications=10' "$GO_LOG"
grep -q 'GET /dashboard/saasAdmin/systemHealth' "$GO_LOG"
grep -q 'POST/PUT /dashboard/saasAdmin/systemIncident' "$GO_LOG"

echo "SaaS admin system health smoke passed"
