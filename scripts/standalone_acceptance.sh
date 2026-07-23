#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
unset GOROOT

SUITE="${MOCHAT_ACCEPTANCE_SUITE:-all}"
INCLUDE_PHP="${MOCHAT_ACCEPTANCE_INCLUDE_PHP:-0}"
SKIP_INVENTORY_PARITY="${MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY:-0}"
TRANSIENT_RETRIES="${MOCHAT_ACCEPTANCE_TRANSIENT_RETRIES:-1}"
TRANSIENT_RETRY_DELAY_SECONDS="${MOCHAT_ACCEPTANCE_TRANSIENT_RETRY_DELAY_SECONDS:-3}"

case "$SUITE" in
  all|core|saas|workers|cron|frontend|mysql57|php)
    ;;
  *)
    echo "unknown MOCHAT_ACCEPTANCE_SUITE=$SUITE" >&2
    echo "valid suites: all, core, saas, workers, cron, frontend, mysql57, php" >&2
    exit 2
    ;;
esac

case "$SKIP_INVENTORY_PARITY" in
  0|1)
    ;;
  *)
    echo "unknown MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=$SKIP_INVENTORY_PARITY" >&2
    echo "valid values: 0, 1" >&2
    exit 2
    ;;
esac

case "$TRANSIENT_RETRIES" in
  ''|*[!0-9]*)
    echo "MOCHAT_ACCEPTANCE_TRANSIENT_RETRIES must be a non-negative integer" >&2
    exit 2
    ;;
esac

case "$TRANSIENT_RETRY_DELAY_SECONDS" in
  ''|*[!0-9]*)
    echo "MOCHAT_ACCEPTANCE_TRANSIENT_RETRY_DELAY_SECONDS must be a non-negative integer" >&2
    exit 2
    ;;
esac

run_step() {
  local name="$1"
  shift
  printf '\n==> %s\n' "$name"
  local attempt=0
  local attempt_log
  local rc
  while :; do
    attempt_log="$(mktemp "${TMPDIR:-/tmp}/mochat-go-acceptance-step.XXXXXX")"
    set +e
    "$@" 2>&1 | tee "$attempt_log"
    rc="${PIPESTATUS[0]}"
    set -e
    if [ "$rc" -eq 0 ]; then
      rm -f "$attempt_log"
      return 0
    fi
    if [ "$attempt" -ge "$TRANSIENT_RETRIES" ] || ! grep -Eq '(did not become healthy|timed out waiting for .+ to return)' "$attempt_log"; then
      rm -f "$attempt_log"
      return "$rc"
    fi
    attempt=$((attempt + 1))
    printf 'transient infrastructure failure; retrying %s (%s/%s)\n' "$name" "$attempt" "$TRANSIENT_RETRIES" >&2
    rm -f "$attempt_log"
    sleep "$TRANSIENT_RETRY_DELAY_SECONDS"
  done
}

run_core() {
  run_step "fast static/unit/build gate" ./scripts/test.sh
  if [ "$SKIP_INVENTORY_PARITY" = "1" ]; then
    echo "standalone inventory parity skipped by MOCHAT_ACCEPTANCE_SKIP_INVENTORY_PARITY=1; this is the independent delivery evidence mode" >&2
  elif [ -d "${MOCHAT_SOURCE_ROOT:-../mochat}/api-server" ]; then
    run_step "standalone inventory parity against PHP source checkout" ./scripts/standalone_inventory_parity.sh
  else
    echo "standalone inventory parity skipped; set MOCHAT_SOURCE_ROOT to a checkout that contains api-server/ to run the migration-period manifest gate" >&2
  fi
  run_step "schema migration apply/baseline/rollback" ./scripts/smoke_schema_migrate.sh
  run_step "standalone smoke without PHP/source/manifest" ./scripts/smoke_standalone.sh
  run_step "independent package smoke without original PHP checkout" ./scripts/smoke_independent_package.sh
  run_step "production evidence gate rule smoke" ./scripts/smoke_production_evidence_gate.sh
  run_step "standalone MySQL/Redis stack check" ./scripts/standalone_stack_check.sh
  run_step "standalone compose app check" ./scripts/smoke_standalone_compose_app.sh
  run_step "standalone route coverage must be complete" env MOCHAT_ROUTE_COVERAGE_MAX_MISSING=0 ./scripts/standalone_route_coverage.sh
  run_step "bootstrap standalone tenant" ./scripts/smoke_bootstrap_standalone.sh
  run_step "queue idempotency" ./scripts/smoke_queue_idempotency.sh
}

run_saas() {
  run_step "SaaS provisioning" ./scripts/smoke_saas_provisioning.sh
  run_step "SaaS tenant isolation" ./scripts/smoke_saas_tenant_isolation.sh
  run_step "SaaS quota enforcement" ./scripts/smoke_saas_quota_enforcement.sh
  run_step "SaaS storage reconcile" ./scripts/smoke_saas_storage_reconcile.sh
  run_step "SaaS storage reclaim" ./scripts/smoke_saas_storage_reclaim.sh
  run_step "SaaS usage refresh" ./scripts/smoke_saas_usage_refresh.sh
  run_step "SaaS alert tenant setting dispatch" ./scripts/smoke_saas_alert_setting_dispatch.sh
  run_step "WeCom credential encryption, rotation, and admin governance" ./scripts/smoke_wecom_credential_encryption.sh
  run_step "SaaS admin dashboard overview" ./scripts/smoke_saas_admin_dashboard.sh
  run_step "SaaS admin platform access RBAC" ./scripts/smoke_saas_admin_access_rbac.sh
  run_step "SaaS tenant branding and standalone frontend metadata" ./scripts/smoke_saas_branding.sh
  run_step "SaaS tenant custom domains, Host isolation, and route/TLS delivery" ./scripts/smoke_saas_tenant_domains.sh
  run_step "SaaS production evidence and release candidate gate" ./scripts/smoke_saas_release_readiness.sh
  run_step "SaaS admin high-risk approvals" ./scripts/smoke_saas_admin_approvals.sh
  run_step "SaaS admin approval governance" ./scripts/smoke_saas_admin_approval_governance.sh
  run_step "SaaS package definition dual approval and optimistic locking" ./scripts/smoke_saas_package_definition_approval.sh
  run_step "SaaS tenant package assignment dual approval and optimistic locking" ./scripts/smoke_saas_tenant_package_assignment_approval.sh
  run_step "SaaS tenant provision dual approval, credential redaction, and atomic execution" ./scripts/smoke_saas_tenant_provision_approval.sh
  run_step "SaaS tenant renewal dual approval, frozen state, and atomic execution" ./scripts/smoke_saas_tenant_renewal_approval.sh
  run_step "SaaS subscription transition dual approval, frozen state, and atomic execution" ./scripts/smoke_saas_subscription_transition_approval.sh
  run_step "SaaS invoice issue dual approval, frozen ledger, and atomic execution" ./scripts/smoke_saas_invoice_issue_approval.sh
  run_step "SaaS payment order create dual approval, frozen package, and atomic execution" ./scripts/smoke_saas_payment_order_create_approval.sh
	  run_step "SaaS payment settlement close dual approval, frozen ledger, and atomic execution" ./scripts/smoke_saas_payment_settlement_close_approval.sh
	  run_step "SaaS payment settlement reopen dual approval, frozen close audit, and atomic execution" ./scripts/smoke_saas_payment_settlement_reopen_approval.sh
	  run_step "SaaS payment settlement resolve dual approval, frozen entry and batch, and atomic execution" ./scripts/smoke_saas_payment_settlement_resolve_approval.sh
	  run_step "SaaS tenant domain lifecycle dual approval, frozen routing, and atomic execution" ./scripts/smoke_saas_tenant_domain_approval.sh
  run_step "SaaS identity security, MFA, and persistent sessions" ./scripts/smoke_saas_identity_security.sh
  run_step "SaaS admin system health and incident governance" ./scripts/smoke_saas_admin_system_health.sh
  # The signed-anchor smoke delegates the legacy-chain cases to ./scripts/smoke_saas_audit_integrity.sh.
  run_step "SaaS signed audit anchors, tamper detection, and rollback evidence" ./scripts/smoke_saas_audit_anchor.sh
  run_step "SaaS service accounts and API key lifecycle" ./scripts/smoke_saas_service_accounts.sh
  run_step "SaaS encrypted backup, offsite replica, and isolated restore recovery" ./scripts/smoke_saas_backup_recovery.sh
  run_step "SaaS tenant data export, legal hold, approval, and erasure lifecycle" ./scripts/smoke_saas_compliance_lifecycle.sh
  run_step "SaaS notification SLO trends" ./scripts/smoke_saas_notification_slo.sh
  run_step "SaaS subscription lifecycle" ./scripts/smoke_saas_subscription_lifecycle.sh
  run_step "SaaS payment collection" ./scripts/smoke_saas_payment_collection.sh
  run_step "SaaS payment refunds" ./scripts/smoke_saas_payment_refunds.sh
  run_step "SaaS billing invoices and credit notes" ./scripts/smoke_saas_billing_invoices.sh
  run_step "SaaS payment provider settlements" ./scripts/smoke_saas_payment_settlements.sh
  run_step "SaaS payment settlement automatic sync" ./scripts/smoke_saas_payment_settlement_sync.sh
  run_step "official account component ticket" ./scripts/smoke_official_account_ticket.sh
}

run_workers() {
  run_step "async file upload worker" ./scripts/smoke_async_file_upload_worker.sh
  run_step "async file upload SaaS usage" ./scripts/smoke_async_file_upload_saas_usage.sh
  run_step "mark tags worker" ./scripts/smoke_mark_tags_worker.sh
  run_step "auto tag keyword task" ./scripts/smoke_auto_tag_keyword_task.sh
  run_step "message remind worker" ./scripts/smoke_message_remind_worker.sh
  run_step "work room sync worker" ./scripts/smoke_work_room_sync_worker.sh
  run_step "work contact sync worker" ./scripts/smoke_work_contact_sync_worker.sh
  run_step "work department list worker" ./scripts/smoke_work_department_list_worker.sh
  run_step "media id update worker" ./scripts/smoke_media_id_update_worker.sh
  run_step "employee statistic worker" ./scripts/smoke_employee_statistic_worker.sh
  run_step "work contact tag remote write" ./scripts/smoke_work_contact_tag_remote_write.sh
  run_step "WeWork callback worker" ./scripts/smoke_wework_callback_worker.sh
  run_step "employee apply worker and SaaS alerts" ./scripts/smoke_employee_apply_worker.sh
}

run_cron() {
  run_step "pull agent cron" ./scripts/smoke_pull_agent_cron.sh
  run_step "employee statistic cron" ./scripts/smoke_employee_statistic_cron.sh
  run_step "channel code cron" ./scripts/smoke_channel_code_cron.sh
  run_step "contact batch send cron" ./scripts/smoke_contact_batch_send_cron.sh
  run_step "room batch send cron" ./scripts/smoke_room_batch_send_cron.sh
  run_step "contact sync send result cron" ./scripts/smoke_contact_sync_send_result_cron.sh
  run_step "room sync send result cron" ./scripts/smoke_room_sync_send_result_cron.sh
  run_step "room tag pull cron" ./scripts/smoke_room_tag_pull_cron.sh
  run_step "corp data cron" ./scripts/smoke_corp_data_cron.sh
  run_step "media id update cron" ./scripts/smoke_media_id_update_cron.sh
  run_step "transfer state refresh cron" ./scripts/smoke_transfer_state_refresh_cron.sh
  run_step "work message archive sync cron" ./scripts/smoke_work_message_archive_sync_cron.sh
  run_step "sensitive words monitor cron" ./scripts/smoke_sensitive_word_monitor_cron.sh
  run_step "SaaS alert notification dispatch cron" ./scripts/smoke_saas_alert_notification_cron.sh
  run_step "SaaS operation queue assignment reminder cron" ./scripts/smoke_saas_operation_queue_assignment_reminder_cron.sh
  run_step "SaaS notification health recovery cron" ./scripts/smoke_saas_notification_health_recovery_cron.sh
}

run_frontend() {
  run_step "frontend static browser smoke" ./scripts/smoke_frontend_static_browser.sh
  run_step "admin core dashboard API smoke" ./scripts/smoke_admin_core_dashboard.sh
  run_step "sensitive word dashboard API smoke" ./scripts/smoke_sensitive_word_dashboard.sh
  run_step "channel code dashboard API smoke" ./scripts/smoke_channel_code_dashboard.sh
  run_step "shop code dashboard API smoke" ./scripts/smoke_shop_code_dashboard.sh
  run_step "radar dashboard API smoke" ./scripts/smoke_radar_dashboard.sh
  run_step "lottery dashboard API smoke" ./scripts/smoke_lottery_dashboard.sh
  run_step "room fission dashboard API smoke" ./scripts/smoke_room_fission_dashboard.sh
  run_step "room infinite pull dashboard API smoke" ./scripts/smoke_room_infinite_pull_dashboard.sh
  run_step "room clock in dashboard API smoke" ./scripts/smoke_room_clock_in_dashboard.sh
  run_step "room quality dashboard API smoke" ./scripts/smoke_room_quality_dashboard.sh
  run_step "room calendar dashboard API smoke" ./scripts/smoke_room_calendar_dashboard.sh
  run_step "room remind dashboard API smoke" ./scripts/smoke_room_remind_dashboard.sh
  run_step "SOP dashboard API smoke" ./scripts/smoke_sop_dashboard.sh
  run_step "auto tag dashboard API smoke" ./scripts/smoke_auto_tag_dashboard.sh
  run_step "greeting dashboard API smoke" ./scripts/smoke_greeting_dashboard.sh
  run_step "room welcome dashboard API smoke" ./scripts/smoke_room_welcome_dashboard.sh
  run_step "work room auto pull dashboard API smoke" ./scripts/smoke_work_room_auto_pull_dashboard.sh
  run_step "room tag pull dashboard API smoke" ./scripts/smoke_room_tag_pull_dashboard.sh
  run_step "contact message batch send dashboard API smoke" ./scripts/smoke_contact_message_batch_send_dashboard.sh
  run_step "room message batch send dashboard API smoke" ./scripts/smoke_room_message_batch_send_dashboard.sh
  run_step "contact batch add dashboard API smoke" ./scripts/smoke_contact_batch_add_dashboard.sh
  run_step "contact transfer dashboard API smoke" ./scripts/smoke_contact_transfer_dashboard.sh
  run_step "work fission dashboard API smoke" ./scripts/smoke_work_fission_dashboard.sh
  run_step "dashboard browser login smoke" ./scripts/smoke_dashboard_frontend_login.sh
  run_step "sidebar contact browser smoke" ./scripts/smoke_sidebar_frontend_contact.sh
  run_step "operation workFission browser smoke" ./scripts/smoke_operation_frontend_work_fission.sh
}

run_mysql57() {
  run_step "MySQL 5.7 schema lint" ./scripts/lint_mysql57_schema.sh
  run_step "MySQL 5.7 migration smoke" ./scripts/smoke_mysql57_schema_migrate.sh
}

run_php_comparison() {
  if [ "$INCLUDE_PHP" != "1" ]; then
    echo "PHP comparison suite skipped; set MOCHAT_ACCEPTANCE_INCLUDE_PHP=1 to run migration-period comparison checks" >&2
    return 0
  fi
  run_step "fallback compatibility smoke" ./scripts/smoke_fallback.sh
  run_step "local compatibility stack check" ./scripts/local_stack_check.sh
  run_step "real PHP auth chain comparison" ./scripts/smoke_real_php_auth_chain.sh
}

STARTED_AT="$(date '+%Y-%m-%d %H:%M:%S')"
printf 'MoChat Go standalone acceptance started at %s, suite=%s\n' "$STARTED_AT" "$SUITE"

case "$SUITE" in
  core)
    run_core
    ;;
  saas)
    run_saas
    ;;
  workers)
    run_workers
    ;;
  cron)
    run_cron
    ;;
  frontend)
    run_frontend
    ;;
  mysql57)
    run_mysql57
    ;;
  php)
    run_php_comparison
    ;;
  all)
    run_core
    run_saas
    run_workers
    run_cron
    run_frontend
    run_mysql57
    run_php_comparison
    ;;
esac

FINISHED_AT="$(date '+%Y-%m-%d %H:%M:%S')"
printf '\nMoChat Go standalone acceptance finished at %s, suite=%s\n' "$FINISHED_AT" "$SUITE"
