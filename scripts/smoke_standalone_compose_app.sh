#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

COMPOSE_FILE="deploy/standalone/docker-compose.yml"
PROJECT_NAME="${MOCHAT_COMPOSE_APP_PROJECT:-mochat-go-compose-app-check}"
GO_PORT="${MOCHAT_GO_PORT:-18090}"
SIDEBAR_PORT="${MOCHAT_SIDEBAR_PORT:-18091}"
OPERATION_PORT="${MOCHAT_OPERATION_PORT:-18092}"
MYSQL_PORT="${MOCHAT_MYSQL_PORT:-13318}"
REDIS_PORT="${MOCHAT_REDIS_PORT:-26391}"
SECRET="${MOCHAT_SIMPLE_JWT_SECRET:-compose-app-secret}"
PHONE="${MOCHAT_BOOTSTRAP_PHONE:-13800000090}"
PASSWORD="${MOCHAT_BOOTSTRAP_PASSWORD:-secret090}"
BACKUP_KEY="${MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY:-0707070707070707070707070707070707070707070707070707070707070707}"
AUDIT_ANCHOR_KEY="${MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY:-0808080808080808080808080808080808080808080808080808080808080808}"
SERVICE_ACCOUNT_PEPPER="${MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER:-0909090909090909090909090909090909090909090909090909090909090909}"
SOURCE_FINGERPRINT="$(python3 scripts/source_fingerprint.py | python3 -c 'import json,sys; print(json.load(sys.stdin)["fingerprint"])')"
SMOKE_CORP_ID="${MOCHAT_COMPOSE_APP_SMOKE_CORP_ID:-900090}"
SMOKE_EMPLOYEE_ID="${MOCHAT_COMPOSE_APP_SMOKE_EMPLOYEE_ID:-900090}"
SMOKE_DEPARTMENT_ID="${MOCHAT_COMPOSE_APP_SMOKE_DEPARTMENT_ID:-900090}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mochat-go-compose-app.XXXXXX")"
STACK_STARTED=0

compose() {
  MOCHAT_GO_PORT="$GO_PORT" \
    MOCHAT_SIDEBAR_PORT="$SIDEBAR_PORT" \
    MOCHAT_OPERATION_PORT="$OPERATION_PORT" \
    MOCHAT_MYSQL_PORT="$MYSQL_PORT" \
    MOCHAT_REDIS_PORT="$REDIS_PORT" \
    MOCHAT_SIMPLE_JWT_SECRET="$SECRET" \
    MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD="${MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD:-1}" \
    MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED="${MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED:-1}" \
    MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID="${MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID:-1}" \
    MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON="${MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON:-1}" \
    MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS="${MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS:-3600}" \
    MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START="${MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START:-1}" \
    MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT="${MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT:-100}" \
    MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY="$BACKUP_KEY" \
    MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID="${MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID:-compose-smoke}" \
    MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON="${MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON:-1}" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS="${MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS:-3600}" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START="${MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START:-1}" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT="${MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT:-100}" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY="$AUDIT_ANCHOR_KEY" \
    MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID="${MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID:-compose-smoke}" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER="$SERVICE_ACCOUNT_PEPPER" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID="${MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID:-compose-smoke}" \
    MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER=1 \
    MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON="${MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON:-1}" \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS="${MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS:-3600}" \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START="${MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START:-1}" \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS="${MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS:-24}" \
    MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES="${MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES:-15}" \
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" --profile app "$@"
}

cleanup() {
  local rc=$?
  rm -rf "$WORK_DIR"
  if [ "$STACK_STARTED" = "1" ] && [ "${KEEP_COMPOSE_APP_STACK:-0}" != "1" ]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  exit "$rc"
}
trap cleanup EXIT INT TERM

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port $port is already in use" >&2
    exit 1
  fi
}

wait_url() {
  local url="$1"
  local expected="$2"
  local deadline=$((SECONDS + 120))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url to return $expected" >&2
  compose ps >&2 || true
  compose logs --tail=160 app >&2 || true
  exit 1
}

mysql_scalar() {
  compose exec -T mysql mariadb -umochat -pmochat_pass -N -B mochat -e "$1" | tr -d '\r'
}

wait_mysql_scalar() {
  local query="$1"
  local expected="$2"
  local deadline=$((SECONDS + 60))
  while [ "$SECONDS" -lt "$deadline" ]; do
    local value
    value="$(mysql_scalar "$query" || true)"
    if [ "$value" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for MySQL query to return $expected" >&2
  echo "$query" >&2
  echo "last value: $(mysql_scalar "$query" || true)" >&2
  compose logs --tail=160 app >&2 || true
  exit 1
}

assert_port_free "$GO_PORT"
assert_port_free "$SIDEBAR_PORT"
assert_port_free "$OPERATION_PORT"
assert_port_free "$MYSQL_PORT"
assert_port_free "$REDIS_PORT"

STACK_STARTED=1
compose up -d --build app

wait_url "http://127.0.0.1:$GO_PORT/readyz" 200
wait_url "http://127.0.0.1:$SIDEBAR_PORT/contact" 200
wait_url "http://127.0.0.1:$OPERATION_PORT/workFission" 200
wait_url "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/page" 307
wait_url "http://127.0.0.1:$GO_PORT/saas-admin/" 200
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'cron-saas-notification-health-recovery' AND status = 'running'" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-notification-health-recovery' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"
wait_mysql_scalar "SELECT COUNT(*) FROM mochat_go_background_tasks WHERE name = 'cron-saas-admin-approval-reminder' AND status = 'running'" "1"
wait_mysql_scalar "SELECT IF(COUNT(*) >= 1, 1, 0) FROM mochat_go_background_task_executions WHERE task_name = 'cron-saas-admin-approval-reminder' AND kind = 'periodic_tick' AND status = 'succeeded'" "1"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_operation_logs'")" = "mochat_go_saas_admin_operation_logs"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_audit_anchor_checkpoints'")" = "mochat_go_saas_admin_audit_anchor_checkpoints"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_alert_notifications' AND INDEX_NAME = 'idx_mochat_go_saas_alert_notifications_health_window'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_alert_notifications' AND INDEX_NAME = 'idx_mochat_go_saas_alert_notifications_slo_window'")" = "5"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_subscriptions'")" = "mochat_go_saas_subscriptions"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_subscription_events'")" = "mochat_go_saas_subscription_events"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_orders'")" = "mochat_go_saas_payment_orders"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_webhook_events'")" = "mochat_go_saas_payment_webhook_events"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_refunds'")" = "mochat_go_saas_payment_refunds"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND COLUMN_NAME IN ('refund_pending_amount_cents', 'refunded_amount_cents', 'latest_refund_id')")" = "3"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_billing_profiles'")" = "mochat_go_saas_billing_profiles"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_invoice_documents'")" = "mochat_go_saas_invoice_documents"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND COLUMN_NAME IN ('invoice_pending_amount_cents', 'invoiced_amount_cents', 'credit_pending_amount_cents', 'credited_amount_cents', 'latest_invoice_document_id')")" = "5"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_batches'")" = "mochat_go_saas_payment_settlement_batches"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_entries'")" = "mochat_go_saas_payment_settlement_entries"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_sync_states'")" = "mochat_go_saas_payment_settlement_sync_states"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_payment_settlement_sync_runs'")" = "mochat_go_saas_payment_settlement_sync_runs"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_roles'")" = "mochat_go_saas_admin_roles"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_user_roles'")" = "mochat_go_saas_admin_user_roles"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_approvals'")" = "mochat_go_saas_admin_approvals"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_approval_events'")" = "mochat_go_saas_admin_approval_events"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_approval_policies'")" = "mochat_go_saas_admin_approval_policies"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_approval_decisions'")" = "mochat_go_saas_admin_approval_decisions"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_approval_delegations'")" = "mochat_go_saas_admin_approval_delegations"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_health_scans'")" = "mochat_go_saas_admin_health_scans"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_service_accounts'")" = "mochat_go_saas_service_accounts"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_service_account_keys'")" = "mochat_go_saas_service_account_keys"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_backup_policies'")" = "mochat_go_saas_backup_policies"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_backup_runs'")" = "mochat_go_saas_backup_runs"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_restore_drills'")" = "mochat_go_saas_restore_drills"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_compliance_policies'")" = "mochat_go_saas_compliance_policies"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_legal_holds'")" = "mochat_go_saas_legal_holds"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_data_exports'")" = "mochat_go_saas_data_exports"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_erasure_requests'")" = "mochat_go_saas_erasure_requests"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_erasure_steps'")" = "mochat_go_saas_erasure_steps"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_tenant_tombstones'")" = "mochat_go_saas_tenant_tombstones"
test "$(mysql_scalar "SHOW TABLES LIKE 'mochat_go_saas_admin_system_incidents'")" = "mochat_go_saas_admin_system_incidents"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.system.read'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.system.manage'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.backups.read'")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.backups.manage'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.compliance.read'")" = "4"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions WHERE permission_code = 'platform.compliance.manage'")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type = 'tenant.data.erase' AND required_approvals = 2")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_payment_orders' AND COLUMN_NAME IN ('package_version', 'package_limits_json')")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_approval_policies WHERE action_type IN ('payment.order.create', 'payment.settlement.close', 'payment.settlement.reopen') AND enabled = 1 AND amount_threshold_cents = 0 AND required_approvals = 2")" = "3"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_policies WHERE id = 1 AND status = 'active' AND require_encryption = 1")" = "1"
test "$(mysql_scalar "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'mochat' AND TABLE_NAME = 'mochat_go_saas_admin_approvals' AND COLUMN_NAME IN ('effect_applied_at', 'effect_operation_id')")" = "2"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_admin_roles WHERE is_system = 1 AND status = 1")" = "6"

curl -sS -f "http://127.0.0.1:$GO_PORT/readyz" >"$WORK_DIR/readyz.json"
curl -sS -f "http://127.0.0.1:$GO_PORT/compat/routes" >"$WORK_DIR/routes.json"
curl -sS -f "http://127.0.0.1:$GO_PORT/login" >"$WORK_DIR/dashboard-login.html"
curl -sS -f "http://127.0.0.1:$GO_PORT/saas-admin/" >"$WORK_DIR/saas-admin.html"
curl -sS -f "http://127.0.0.1:$SIDEBAR_PORT/contact" >"$WORK_DIR/sidebar-contact.html"
curl -sS -f "http://127.0.0.1:$OPERATION_PORT/workFission" >"$WORK_DIR/operation-work-fission.html"

compose exec -T app mochat-migrate -action baseline -project-root /app >"$WORK_DIR/migrate-baseline.out"
grep -q $'0001_initial_schema\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0002_seed_core_data\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0003_saas_provisioning\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0004_saas_storage_objects\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0005_background_tasks\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0006_saas_alerts\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0033_saas_admin_operation_logs\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0037_saas_notification_health_index\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0038_saas_notification_slo_index\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0039_saas_subscription_lifecycle\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0040_saas_payment_collection\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0041_saas_payment_refunds\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0042_saas_billing_invoices\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0043_saas_payment_settlements\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0044_saas_payment_settlement_sync\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0045_saas_admin_rbac\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0046_saas_admin_approvals\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0047_saas_admin_approval_governance\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0048_saas_admin_system_health\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0049_saas_service_accounts\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0052_saas_compliance_lifecycle\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0055_saas_tenant_domains\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0060_saas_service_account_usage_retention\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0061_saas_service_account_usage_alerts\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0062_saas_audit_integrity\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0064_saas_audit_anchor_remote_immutability\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0068_wechat_open_credential_encryption\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0073_saas_compliance_export_deletion_saga\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0076_saas_identity_policy_change_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0077_saas_tenant_disable_approval_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0079_saas_service_account_key_revoke_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0080_saas_service_account_update_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0081_saas_service_account_key_rotate_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0082_saas_service_account_create_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0083_saas_identity_mfa_reset_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0085_saas_tenant_package_assignment_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0088_saas_subscription_transition_approval_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0089_saas_invoice_issue_approval_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0090_saas_payment_order_create_approval_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0091_saas_payment_settlement_close_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0092_saas_payment_settlement_reopen_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0093_saas_payment_settlement_resolve_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0094_saas_tenant_domain_command_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0096_saas_tenant_enable_approval_guard\tbaselined' "$WORK_DIR/migrate-baseline.out"
grep -q $'0097_saas_release_evidence_action_tracking\tbaselined' "$WORK_DIR/migrate-baseline.out"

compose exec -T app sh -c 'command -v mysqldump >/dev/null && command -v mysql >/dev/null'

compose logs app >"$WORK_DIR/app.log"
grep -q "go cron enabled: SaaS notification health recovery interval=1h0m0s run_on_start=true window_hours=24 stale_minutes=15" "$WORK_DIR/app.log"
grep -q "SaaS notification health recovery cron finished: matched=0 active=0 recovered=0 closed=0 already_closed=0 unhealthy=0 no_data=0 no_delivery_evidence=0 missing_tenant=0" "$WORK_DIR/app.log"
grep -q "go cron enabled: SaaS approval reminder interval=1h0m0s run_on_start=true limit=100" "$WORK_DIR/app.log"
grep -q "SaaS approval reminder cron finished: scanned=0 enqueued=0 skipped=0" "$WORK_DIR/app.log"
grep -q "GET /dashboard/saasAdmin/approvalPolicies approval_required=true" "$WORK_DIR/app.log"
grep -q "GET /dashboard/saasAdmin/systemHealth" "$WORK_DIR/app.log"
grep -q "GET /dashboard/saasAdmin/serviceAccounts" "$WORK_DIR/app.log"
grep -q "GET /dashboard/saasAdmin/backupOverview" "$WORK_DIR/app.log"
grep -q "POST/PUT /dashboard/saasAdmin/backupPolicy" "$WORK_DIR/app.log"
grep -q "POST/PUT /dashboard/saasAdmin/backupRun" "$WORK_DIR/app.log"
grep -q "POST/PUT /dashboard/saasAdmin/restoreDrill" "$WORK_DIR/app.log"
grep -q "SaaS backup manager enabled: encryption_configured=true" "$WORK_DIR/app.log"
grep -q "SaaS audit anchor manager enabled: artifact_root=/app/audit-anchors hmac_configured=true key_id=compose-smoke" "$WORK_DIR/app.log"
grep -q "go cron enabled: SaaS audit anchor interval=1h0m0s run_on_start=true limit=100 key_id=compose-smoke" "$WORK_DIR/app.log"
grep -q "source_fingerprint_configured=true source=build fingerprint=$SOURCE_FINGERPRINT" "$WORK_DIR/app.log"
grep -q "GET /api/saas/v1/whoami" "$WORK_DIR/app.log"

compose exec -T app mochat-bootstrap \
  -tenant-id 1 \
  -tenant-name "容器验收租户" \
  -phone "$PHONE" \
  -password "$PASSWORD" \
  -user-name "容器管理员" \
  -role-name "容器超级管理员" \
  -package-code "compose-standard" \
  -package-name "Compose标准版" \
  -max-corps 3 \
  -max-users 25 \
  -max-contacts 5000 \
  -max-rooms 200 \
  -max-agents 5 \
  -channel-codes 120 \
  -shop-codes 45 \
  -radars 33 \
  -lotteries 22 \
  -room-infinite-pulls 24 \
  -room-fissions 26 \
  -room-clock-ins 28 \
  -room-qualities 30 \
  -room-calendars 32 \
  -room-reminds 34 \
  -contact-sops 36 \
  -room-sops 38 \
  -sensitive-words 39 \
  -storage-mb 1024 \
  -contact-message-batches 300 \
  -room-message-batches 150 \
  -room-tag-pulls 80 \
  -work-room-auto-pulls 60 \
  -work-fissions 40 \
  -official-accounts 10 \
  -async-executions 10000 >"$WORK_DIR/bootstrap.out"
grep -q $'tenant_id\t1' "$WORK_DIR/bootstrap.out"
grep -q $'package_code\tcompose-standard' "$WORK_DIR/bootstrap.out"
grep -q $'usage_metric_count\t26' "$WORK_DIR/bootstrap.out"
grep -q $'seed_version_count\t3' "$WORK_DIR/bootstrap.out"

USER_ID="$(awk -F '\t' '$1 == "user_id" { print $2 }' "$WORK_DIR/bootstrap.out")"
test -n "$USER_ID"

compose exec -T mysql mariadb -umochat -pmochat_pass mochat <<SQL
SET NAMES utf8mb4;
INSERT INTO mc_corp
  (id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
VALUES
  ($SMOKE_CORP_ID, '容器验收企业', 'ww-compose-smoke', 'employee-secret-compose', 'contact-secret-compose', 'callback-token', 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  tenant_id = VALUES(tenant_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_employee
  (id, wx_user_id, corp_id, name, mobile, gender, status, log_user_id, created_at, updated_at, deleted_at)
VALUES
  ($SMOKE_EMPLOYEE_ID, 'compose-smoke-user', $SMOKE_CORP_ID, '容器验收员工', '$PHONE', 1, 1, $USER_ID, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  name = VALUES(name),
  mobile = VALUES(mobile),
  status = VALUES(status),
  log_user_id = VALUES(log_user_id),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_department
  (id, wx_department_id, corp_id, name, parent_id, wx_parentid, \`order\`, level, path, created_at, updated_at, deleted_at)
VALUES
  ($SMOKE_DEPARTMENT_ID, 1, $SMOKE_CORP_ID, '容器验收总部', 0, 0, 100, 1, '#$SMOKE_DEPARTMENT_ID#', NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  wx_department_id = VALUES(wx_department_id),
  corp_id = VALUES(corp_id),
  name = VALUES(name),
  parent_id = VALUES(parent_id),
  wx_parentid = VALUES(wx_parentid),
  \`order\` = VALUES(\`order\`),
  level = VALUES(level),
  path = VALUES(path),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_work_employee_department
  (id, employee_id, department_id, is_leader_in_dept, \`order\`, created_at, updated_at, deleted_at)
VALUES
  ($SMOKE_EMPLOYEE_ID, $SMOKE_EMPLOYEE_ID, $SMOKE_DEPARTMENT_ID, 0, 1, NOW(), NOW(), NULL)
ON DUPLICATE KEY UPDATE
  employee_id = VALUES(employee_id),
  department_id = VALUES(department_id),
  is_leader_in_dept = VALUES(is_leader_in_dept),
  \`order\` = VALUES(\`order\`),
  updated_at = NOW(),
  deleted_at = NULL;

INSERT INTO mc_corp_day_data
  (id, corp_id, add_contact_num, add_room_num, add_into_room_num, loss_contact_num, quit_room_num, date, created_at, updated_at)
VALUES
  ($SMOKE_CORP_ID, $SMOKE_CORP_ID, 5, 2, 3, 1, 1, CURDATE(), NOW(), NOW()),
  ($((SMOKE_CORP_ID + 1)), $SMOKE_CORP_ID, 4, 1, 2, 0, 0, DATE_SUB(CURDATE(), INTERVAL 1 DAY), NOW(), NOW())
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
  ($SMOKE_CORP_ID, $SMOKE_CORP_ID, 6, '2026-07-02 12:00:00', NOW(), NOW()),
  ($((SMOKE_CORP_ID + 1)), $SMOKE_CORP_ID, 1, '2026-07-02 09:00:00', NOW(), NOW())
ON DUPLICATE KEY UPDATE
  corp_id = VALUES(corp_id),
  type = VALUES(type),
  last_update_time = VALUES(last_update_time),
  updated_at = NOW();
SQL

curl -sS -f \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"password\":\"$PASSWORD\"}" \
  "http://127.0.0.1:$GO_PORT/dashboard/user/auth" >"$WORK_DIR/auth.json"

TOKEN="$(python3 - "$WORK_DIR/auth.json" <<'PY'
import json
import pathlib
import sys

auth = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if auth.get("code") != 200:
    raise SystemExit("auth failed: " + json.dumps(auth, ensure_ascii=False))
token = auth.get("data", {}).get("token", "")
if not token:
    raise SystemExit("auth token is empty")
print(token)
PY
)"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/releaseReadiness" >"$WORK_DIR/release-readiness.json"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"action":"create"}' \
  "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/backupRun" >"$WORK_DIR/backup-create.json"
test "$(mysql_scalar "SELECT COUNT(*) FROM mochat_go_saas_backup_runs WHERE status = 'succeeded' AND encrypted = 1 AND verification_status = 'passed' AND migration_version = '0097_saas_release_evidence_action_tracking' AND migration_count = 97")" = "1"

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenantId":1,"limit":100,"verificationLimit":20}' \
  "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/auditIntegrityVerify" >"$WORK_DIR/audit-integrity-verify.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"action":"create","tenantId":1,"limit":100}' \
  "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/auditAnchor" >"$WORK_DIR/audit-anchor-create.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"action":"verify","tenantId":1,"limit":100}' \
  "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/auditAnchor" >"$WORK_DIR/audit-anchor-verify.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/saasAdmin/auditAnchors?tenantId=1&limit=100" >"$WORK_DIR/audit-anchors.json"
compose exec -T app sh -c '
  set -eu
  test "$(stat -c %u /app/audit-anchors)" = "10001"
  set -- /app/audit-anchors/AAN-*.json
  test -f "$1"
  for file in "$@"; do
    test "$(stat -c %a "$file")" = "600"
    test "$(stat -c %u "$file")" = "10001"
  done
'

curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/user/loginShow" >"$WORK_DIR/login-show-before-bind.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/role/permissionByUser" >"$WORK_DIR/permission-by-user.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/corp/select" >"$WORK_DIR/corp-select.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"corpId\":$SMOKE_CORP_ID}" \
  "http://127.0.0.1:$GO_PORT/dashboard/corp/bind" >"$WORK_DIR/corp-bind.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/user/loginShow" >"$WORK_DIR/login-show-after-bind.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/corpData/index" >"$WORK_DIR/corp-data-index.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/corpData/lineChat" >"$WORK_DIR/corp-data-line-chat.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/workEmployee/searchCondition" >"$WORK_DIR/work-employee-search-condition.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/workDepartment/index" >"$WORK_DIR/work-department-index.json"
curl -sS -f \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:$GO_PORT/dashboard/workEmployee/index?page=1&perPage=10" >"$WORK_DIR/work-employee-index.json"

USER_CORP_CACHE="$(compose exec -T redis redis-cli GET "mc:user.$USER_ID" | tr -d '\r')"

python3 - "$WORK_DIR" "$USER_ID" "$SMOKE_CORP_ID" "$SMOKE_EMPLOYEE_ID" "$USER_CORP_CACHE" "$PHONE" "$SOURCE_FINGERPRINT" <<'PY'
import json
import pathlib
import sys

work = pathlib.Path(sys.argv[1])
user_id = int(sys.argv[2])
corp_id = int(sys.argv[3])
employee_id = int(sys.argv[4])
user_corp_cache = sys.argv[5]
phone = sys.argv[6]
source_fingerprint = sys.argv[7]
readyz = json.loads((work / "readyz.json").read_text(encoding="utf-8"))
routes = json.loads((work / "routes.json").read_text(encoding="utf-8"))
auth = json.loads((work / "auth.json").read_text(encoding="utf-8"))
backup_create = json.loads((work / "backup-create.json").read_text(encoding="utf-8"))
audit_integrity_verify = json.loads((work / "audit-integrity-verify.json").read_text(encoding="utf-8"))
audit_anchor_create = json.loads((work / "audit-anchor-create.json").read_text(encoding="utf-8"))
audit_anchor_verify = json.loads((work / "audit-anchor-verify.json").read_text(encoding="utf-8"))
audit_anchors = json.loads((work / "audit-anchors.json").read_text(encoding="utf-8"))
release_readiness = json.loads((work / "release-readiness.json").read_text(encoding="utf-8"))
login_show_before_bind = json.loads((work / "login-show-before-bind.json").read_text(encoding="utf-8"))
permission_by_user = json.loads((work / "permission-by-user.json").read_text(encoding="utf-8"))
corp_select = json.loads((work / "corp-select.json").read_text(encoding="utf-8"))
corp_bind = json.loads((work / "corp-bind.json").read_text(encoding="utf-8"))
login_show_after_bind = json.loads((work / "login-show-after-bind.json").read_text(encoding="utf-8"))
corp_data_index = json.loads((work / "corp-data-index.json").read_text(encoding="utf-8"))
corp_data_line_chat = json.loads((work / "corp-data-line-chat.json").read_text(encoding="utf-8"))
work_employee_search_condition = json.loads((work / "work-employee-search-condition.json").read_text(encoding="utf-8"))
work_department_index = json.loads((work / "work-department-index.json").read_text(encoding="utf-8"))
work_employee_index = json.loads((work / "work-employee-index.json").read_text(encoding="utf-8"))
dashboard_login = (work / "dashboard-login.html").read_text(encoding="utf-8")
sidebar_contact = (work / "sidebar-contact.html").read_text(encoding="utf-8")
operation_work_fission = (work / "operation-work-fission.html").read_text(encoding="utf-8")
saas_admin = (work / "saas-admin.html").read_text(encoding="utf-8")

assert readyz["standalone"] is True, readyz
assert readyz["mode"] == "standalone-go", readyz
assert readyz["source_root"] == "", readyz
assert readyz["manifest_path"] == "", readyz
assert readyz["proxy_fallback_enabled"] is False, readyz
assert readyz["php_upstream"] == "", readyz
assert len(routes["routes"]) >= 200, len(routes["routes"])
assert routes["migrated_route_count"] >= len(routes["routes"]), routes["migrated_route_count"]
assert auth["code"] == 200, auth
assert auth["data"]["token"], auth
assert auth["data"]["expire"] > 0, auth
assert backup_create["code"] == 201, backup_create
assert backup_create["data"]["run"]["status"] == "succeeded", backup_create
assert backup_create["data"]["run"]["encrypted"] is True, backup_create
assert backup_create["data"]["run"]["verificationStatus"] == "passed", backup_create
assert backup_create["data"]["run"]["migrationVersion"] == "0097_saas_release_evidence_action_tracking", backup_create
assert backup_create["data"]["run"]["migrationCount"] == 97, backup_create
assert audit_integrity_verify["code"] == 200, audit_integrity_verify
assert audit_integrity_verify["data"]["scannedChains"] == 1, audit_integrity_verify
assert audit_integrity_verify["data"]["healthyChains"] == 1, audit_integrity_verify
assert audit_integrity_verify["data"]["failedChains"] == 0, audit_integrity_verify
assert audit_anchor_create["code"] == 201, audit_anchor_create
assert audit_anchor_create["data"]["result"]["scannedChains"] == 1, audit_anchor_create
assert audit_anchor_create["data"]["result"]["failedArtifacts"] == 0, audit_anchor_create
assert audit_anchor_verify["code"] == 200, audit_anchor_verify
assert audit_anchor_verify["data"]["result"]["failedCheckpoints"] == 0, audit_anchor_verify
assert audit_anchor_verify["data"]["result"]["passedCheckpoints"] == audit_anchor_verify["data"]["result"]["scannedCheckpoints"], audit_anchor_verify
assert audit_anchors["code"] == 200, audit_anchors
assert audit_anchors["data"]["config"]["hmacConfigured"] is True, audit_anchors
assert audit_anchors["data"]["config"]["hmacKeyId"] == "compose-smoke", audit_anchors
assert audit_anchors["data"]["summary"]["orphanArtifactCount"] == 0, audit_anchors
assert audit_anchors["data"]["summary"]["verificationFailedCount"] == 0, audit_anchors
assert audit_anchors["data"]["summary"]["checkpointCount"] >= 1, audit_anchors
assert release_readiness["code"] == 200, release_readiness
release_summary = release_readiness["data"]["summary"]
assert release_summary["targetSourceFingerprint"] == source_fingerprint, release_summary
assert release_summary["sourceFingerprintAuthoritative"] is True, release_summary
assert release_summary["sourceFingerprintSource"] == "build", release_summary
assert release_summary["artifactVerifier"]["configured"] is True, release_summary
assert release_summary["requiredCount"] == 6, release_summary
assert release_summary["missingCount"] == 6, release_summary
assert release_summary["metadataReady"] is False, release_summary
assert release_summary["candidateGateEnabled"] is False, release_summary
assert login_show_before_bind["code"] == 200, login_show_before_bind
assert login_show_before_bind["data"]["userId"] == user_id, login_show_before_bind
assert login_show_before_bind["data"]["userPhone"] == phone, login_show_before_bind
assert permission_by_user["code"] == 200, permission_by_user
assert isinstance(permission_by_user["data"], list) and permission_by_user["data"], permission_by_user
assert corp_select["code"] == 200, corp_select
assert corp_select["data"] == [{"corpId": corp_id, "corpName": "容器验收企业"}], corp_select
assert corp_bind["code"] == 200, corp_bind
assert login_show_after_bind["code"] == 200, login_show_after_bind
assert login_show_after_bind["data"]["corpId"] == corp_id, login_show_after_bind
assert login_show_after_bind["data"]["corpName"] == "容器验收企业", login_show_after_bind
assert user_corp_cache == f"{corp_id}-0", user_corp_cache
assert corp_data_index["code"] == 200, corp_data_index
assert corp_data_index["data"]["addContactNum"] == 5, corp_data_index
assert corp_data_index["data"]["lastAddContactNum"] == 4, corp_data_index
assert corp_data_index["data"]["updateTime"] == "2026-07-02 12:00:00", corp_data_index
assert corp_data_line_chat["code"] == 200, corp_data_line_chat
assert isinstance(corp_data_line_chat["data"], list) and corp_data_line_chat["data"], corp_data_line_chat
assert work_employee_search_condition["code"] == 200, work_employee_search_condition
assert work_employee_search_condition["data"]["syncTime"] == "2026-07-02 09:00:00", work_employee_search_condition
assert work_department_index["code"] == 200, work_department_index
assert work_department_index["data"]["department"][0]["name"] == "容器验收总部", work_department_index
assert work_department_index["data"]["employee"][0]["employeeId"] == employee_id, work_department_index
assert work_employee_index["code"] == 200, work_employee_index
assert work_employee_index["data"]["page"]["total"] == 1, work_employee_index
assert work_employee_index["data"]["list"][0]["name"] == "容器验收员工", work_employee_index
assert 'id="app"' in dashboard_login and "/js/app." in dashboard_login, dashboard_login[:200]
assert 'id="app"' in sidebar_contact and "/js/app." in sidebar_contact, sidebar_contact[:200]
assert 'id="app"' in operation_work_fission and "/js/app." in operation_work_fission, operation_work_fission[:200]
assert 'id="root"' in saas_admin, saas_admin[:500]
assert "/saas-admin/assets/" in saas_admin, saas_admin[:500]
print("standalone compose app smoke passed")
PY
