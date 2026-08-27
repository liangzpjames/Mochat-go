#!/bin/sh
set -eu

# Match the standalone compose baseline and install the ordinary migrations
# needed by the Dashboard acceptance runtime. Controlled 0130/0131 remain
# deliberately outside this local bootstrap and are never ledgered here.
# The inclusive range ends at 0104_scrm_opportunity_owner.up.sql.
mysql_exec() {
  mariadb --protocol=socket -uroot -p"${MARIADB_ROOT_PASSWORD}" "${MARIADB_DATABASE}" "$@"
}

mysql_exec <<'SQL'
CREATE TABLE IF NOT EXISTS mochat_go_schema_migrations (
  version varchar(64) NOT NULL,
  description varchar(255) NOT NULL DEFAULT '',
  checksum char(64) NOT NULL DEFAULT '',
  applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  execution_ms int(10) unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SQL

record_migration() {
  migration_path="$1"
  migration_version="$2"
  migration_checksum="$(sha256sum "${migration_path}" | awk '{print $1}')"
  mysql_exec -e "INSERT INTO mochat_go_schema_migrations (version,description,checksum,applied_at,execution_ms) VALUES ('${migration_version}','local acceptance: ${migration_version}','${migration_checksum}',NOW(),0) ON DUPLICATE KEY UPDATE version=VALUES(version)"
}

apply_migration() {
  migration_path="$1"
  migration_name="$(basename "${migration_path}")"
  migration_version="${migration_name%.up.sql}"
  mysql_exec < "${migration_path}"
  record_migration "${migration_path}" "${migration_version}"
}

record_migration /docker-entrypoint-initdb.d/001-mochat.sql 0001_initial_schema

for migration in /mochat-migrations/*.up.sql; do
  name="$(basename "${migration}")"
  version="${name%%_*}"
  if [ "${version}" -ge 0002 ] && [ "${version}" -le 0104 ]; then
    apply_migration "${migration}"
  fi
done

# Keep the post-baseline list explicit so a fresh acceptance volume has every
# provider table used by these pages. Reconciliation migrations already folded
# into mochat.sql are intentionally omitted because several are non-idempotent.
# Never add controlled 0130/0131 here.
for name in \
  0105_corp_data_realtime_indexes.up.sql \
  0106_scrm_lead_parity.up.sql \
  0107_scrm_contact_lifecycle_idempotency.up.sql \
  0108_scrm_public_pool_parity.up.sql \
  0109_scrm_customer_tag_parity.up.sql \
  0110_risk_behavior_provider.up.sql \
  0111_timeout_warning_provider.up.sql \
  0112_message_intercept_keyword_library_provider.up.sql \
  0113_silent_customer_refuse_archive_provider.up.sql \
  0114_friends_circle_provider.up.sql \
  0115_material_foundation.up.sql \
  0116_phase34_acquisition_provider.up.sql \
  0117_friends_circle_publish_audit.up.sql \
  0118_material_selector_references.up.sql \
  0119_phase35_orders_settings.up.sql \
  0120_saas_tenant_default_corp_reconcile.up.sql \
  0121_phase35_order_productization.up.sql \
  0122_phase35_acceptance_lifecycle.up.sql \
  0123_phase35_order_collation_align.up.sql \
  0124_ai_settings_tables.up.sql \
  0125_bootstrap_role_remark_cn.up.sql \
  0126_phase3_final_providers.up.sql \
  0127_dashboard_page_rbac.up.sql \
  0128_dashboard_page_rbac_legacy_scope_fix.up.sql \
  0129_identity_realms_single_corp_schema.up.sql \
  0132_company_settings_credentials.up.sql \
  0133_archive_simulation_registry.up.sql \
  0134_reconcile_phase3_provider_schema.up.sql \
  0135_reconcile_material_schema.up.sql \
  0136_reconcile_scrm_customer_schema.up.sql \
  0137_reconcile_ai_settings_schema.up.sql \
  0138_archive_source_sync.up.sql \
	0139_wecom_capability_ledger.up.sql \
  0140_global_message_focus_and_indexes.up.sql \
  0141_conversation_workspace_rbac.up.sql \
  0142_customer_conversation_workspace_rbac.up.sql \
  0143_group_conversation_workspace.up.sql \
  0144_work_message_export_tasks.up.sql \
  0145_chat_media_sync_metadata.up.sql \
  0146_risk_warning_scan_states.up.sql \
  0147_risk_sensitive_detail_rbac.up.sql \
  0148_ai_conversation_insights.up.sql \
  0149_message_intercept_silent_filters.up.sql \
  0150_repair_keyword_published_snapshots.up.sql \
  0151_ai_conversation_insight_api_resources.up.sql \
  0153_live_code_workspace.up.sql \
  0154_dashboard_permission_resource_reconciliation.up.sql \
  0155_ai_settings_integrity_audit.up.sql \
  0156_ai_settings_knowledge_runtime.up.sql \
  0157_ai_settings_document_count_reconcile.up.sql \
  0158_ai_assistant_default_smart_rule.up.sql \
  0159_dual_analysis_assistants.up.sql \
  0160_ai_settings_knowledge_collation.up.sql \
  0161_ai_insight_filter_options_rbac.up.sql \
  0162_company_profile_grantable.up.sql \
  0163_ai_insight_projection_resources.up.sql \
  0164_saas_tenant_ai_provider.up.sql \
  0165_ai_daily_insight_unification.up.sql \
  0166_wecom_integration_and_archive_media.up.sql \
  0167_tenant_wecom_mode.up.sql
do
	if [ "${name}" = "0139_wecom_capability_ledger.up.sql" ]; then
		mysql_exec < /local-acceptance-init/0139-parent-compat.sql
	fi
	apply_migration "/mochat-migrations/${name}"
done
