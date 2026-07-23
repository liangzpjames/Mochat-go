package saascompliance

import "sort"

func Inventory() []DatasetSpec {
	specs := make([]DatasetSpec, 0, 161)
	add := func(key, table, predicate, action, retention string, order int) {
		orderBy := "id"
		switch table {
		case "mochat_go_saas_identity_policies":
			orderBy = "tenant_id"
		case "mochat_go_saas_admin_audit_chains":
			orderBy = "tenant_id"
		case "mochat_go_saas_identity_user_states":
			orderBy = "user_id"
		case "mochat_go_saas_branding_profiles":
			orderBy = "tenant_id"
		}
		specs = append(specs, DatasetSpec{Key: key, Table: table, Predicate: predicate, OrderBy: orderBy, Action: action, RetentionClass: retention, DeleteOrder: order, Export: true})
	}

	// Child rows are removed before their owning parents. Predicates are static and
	// only receive the target tenant ID as arguments.
	children := []struct{ table, predicate string }{
		{"mc_contact_batch_add_allot", "import_id IN (SELECT id FROM mc_contact_batch_add_import WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_contact_field_pivot", "contact_id IN (SELECT id FROM mc_work_contact WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_contact_message_batch_send_employee", "batch_id IN (SELECT id FROM mc_contact_message_batch_send WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_contact_message_batch_send_result", "batch_id IN (SELECT id FROM mc_contact_message_batch_send WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_lottery_contact_record", "lottery_id IN (SELECT id FROM mc_lottery WHERE tenant_id = ?)"},
		{"mc_lottery_contact", "lottery_id IN (SELECT id FROM mc_lottery WHERE tenant_id = ?)"},
		{"mc_lottery_prize", "lottery_id IN (SELECT id FROM mc_lottery WHERE tenant_id = ?)"},
		{"mc_rbac_role_menu", "role_id IN (SELECT id FROM mc_rbac_role WHERE tenant_id = ?)"},
		{"mc_rbac_user_role", "user_id IN (SELECT id FROM mc_user WHERE tenant_id = ?)"},
		{"mc_room_calendar_record", "room_calendar_id IN (SELECT id FROM mc_room_calendar WHERE tenant_id = ?)"},
		{"mc_room_calendar_push", "room_calendar_id IN (SELECT id FROM mc_room_calendar WHERE tenant_id = ?)"},
		{"mc_room_clock_in_contact", "clock_in_id IN (SELECT id FROM mc_room_clock_in WHERE tenant_id = ?)"},
		{"mc_room_clock_in_record", "clock_in_id IN (SELECT id FROM mc_room_clock_in WHERE tenant_id = ?)"},
		{"mc_room_fission_contact", "fission_id IN (SELECT id FROM mc_room_fission WHERE tenant_id = ?)"},
		{"mc_room_fission_invite", "fission_id IN (SELECT id FROM mc_room_fission WHERE tenant_id = ?)"},
		{"mc_room_fission_poster", "fission_id IN (SELECT id FROM mc_room_fission WHERE tenant_id = ?)"},
		{"mc_room_fission_room", "fission_id IN (SELECT id FROM mc_room_fission WHERE tenant_id = ?)"},
		{"mc_room_fission_welcome", "fission_id IN (SELECT id FROM mc_room_fission WHERE tenant_id = ?)"},
		{"mc_room_message_batch_send_employee", "batch_id IN (SELECT id FROM mc_room_message_batch_send WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_room_message_batch_send_result", "batch_id IN (SELECT id FROM mc_room_message_batch_send WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_room_quality_contact", "quality_id IN (SELECT id FROM mc_room_quality WHERE tenant_id = ?)"},
		{"mc_room_tag_pull_contact", "room_tag_pull_id IN (SELECT id FROM mc_room_tag_pull WHERE tenant_id = ?)"},
		{"mc_system_config_value", "config_id IN (SELECT id FROM mc_system_config WHERE tenant_id = ?)"},
		{"mc_work_contact_room", "contact_id IN (SELECT id FROM mc_work_contact WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?)) OR room_id IN (SELECT id FROM mc_work_room WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_contact_tag_pivot", "contact_id IN (SELECT id FROM mc_work_contact WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_employee_department", "employee_id IN (SELECT id FROM mc_work_employee WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_employee_tag_pivot", "employee_id IN (SELECT id FROM mc_work_employee WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_fission_contact", "fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_fission_invite", "fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_fission_poster", "fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_fission_push", "fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mc_work_fission_welcome", "fission_id IN (SELECT id FROM mc_work_fission WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?))"},
		{"mochat_go_saas_service_account_usage_daily", "service_account_id IN (SELECT id FROM mochat_go_saas_service_accounts WHERE tenant_id = ?)"},
		{"mochat_go_saas_service_account_keys", "service_account_id IN (SELECT id FROM mochat_go_saas_service_accounts WHERE tenant_id = ?)"},
		{"mochat_go_saas_payment_webhook_events", "order_id IN (SELECT id FROM mochat_go_saas_payment_orders WHERE tenant_id = ?) OR refund_id IN (SELECT id FROM mochat_go_saas_payment_refunds WHERE tenant_id = ?)"},
		{"mochat_go_saas_payment_settlement_entries", "matched_tenant_id = ?"},
		{"mochat_go_saas_admin_user_roles", "user_id IN (SELECT id FROM mc_user WHERE tenant_id = ?)"},
		{"mochat_go_saas_admin_user_access", "user_id IN (SELECT id FROM mc_user WHERE tenant_id = ?)"},
		{"mochat_go_saas_tenant_domain_delivery_events", "tenant_id = ?"},
		{"mochat_go_saas_tenant_domain_delivery_jobs", "tenant_id = ?"},
	}
	for index, item := range children {
		retention := RetentionOperational
		action := DatasetActionDelete
		if item.table == "mochat_go_saas_payment_webhook_events" || item.table == "mochat_go_saas_payment_settlement_entries" {
			retention = RetentionFinancial
		}
		if item.table == "mochat_go_saas_payment_settlement_entries" {
			action = DatasetActionRedact
		}
		add(item.table, item.table, item.predicate, action, retention, 100+index)
	}

	tenantTables := []string{
		"mc_auto_tag", "mc_lottery", "mc_official_account", "mc_official_account_set", "mc_radar", "mc_radar_channel",
		"mc_radar_channel_link", "mc_rbac_role", "mc_room_calendar", "mc_room_clock_in", "mc_room_fission", "mc_room_infinite",
		"mc_room_quality", "mc_room_remind", "mc_room_tag_pull", "mc_shop_code", "mc_shop_code_page", "mc_system_config",
		"mochat_go_background_task_executions", "mochat_go_saas_admin_tasks", "mochat_go_saas_alerts", "mochat_go_saas_alert_notifications",
		"mochat_go_saas_identity_auth_challenges", "mochat_go_saas_identity_mfa_credentials", "mochat_go_saas_identity_policies",
		"mochat_go_saas_identity_sessions", "mochat_go_saas_identity_user_states",
		"mochat_go_saas_alert_settings", "mochat_go_saas_branding_profiles", "mochat_go_saas_service_accounts", "mochat_go_saas_storage_objects", "mochat_go_saas_tenant_domain_deliveries", "mochat_go_saas_tenant_domains", "mochat_go_saas_tenant_packages",
		"mochat_go_saas_usage_counters", "mochat_go_tenant_provision_runs",
	}
	for index, table := range tenantTables {
		add(table, table, "tenant_id = ?", DatasetActionDelete, RetentionOperational, 300+index)
	}

	financialTables := []string{
		"mochat_go_saas_billing_events", "mochat_go_saas_billing_profiles", "mochat_go_saas_invoice_documents",
		"mochat_go_saas_payment_refunds", "mochat_go_saas_payment_orders", "mochat_go_saas_subscription_events", "mochat_go_saas_subscriptions",
	}
	for index, table := range financialTables {
		add(table, table, "tenant_id = ?", DatasetActionDelete, RetentionFinancial, 400+index)
	}

	corpTables := []string{
		"mc_auto_tag_record", "mc_channel_code", "mc_channel_code_group", "mc_contact_batch_add_config", "mc_contact_batch_add_import",
		"mc_contact_batch_add_import_record", "mc_contact_employee_process", "mc_contact_employee_track", "mc_contact_message_batch_send",
		"mc_contact_process", "mc_contact_sop", "mc_contact_sop_log", "mc_corp_day_data", "mc_greeting", "mc_medium", "mc_medium_group",
		"mc_plugin", "mc_radar_record", "mc_room_message_batch_send", "mc_room_remind_record", "mc_room_sop", "mc_room_sop_log",
		"mc_room_welcome_template", "mc_sensitive_word", "mc_sensitive_words_monitor", "mc_sensitive_word_group", "mc_shop_code_record",
		"mc_work_agent", "mc_work_contact", "mc_work_contact_employee", "mc_work_contact_tag", "mc_work_contact_tag_group",
		"mc_work_department", "mc_work_employee", "mc_work_employee_statistic", "mc_work_employee_tag", "mc_work_fission",
		"mc_work_message_1", "mc_work_message_2", "mc_work_message_3", "mc_work_message_4", "mc_work_message_5",
		"mc_work_message_6", "mc_work_message_7", "mc_work_message_8", "mc_work_message_9", "mc_work_message_10",
		"mc_work_message_id", "mc_work_room", "mc_work_room_auto_pull", "mc_work_room_group", "mc_work_transfer_log", "mc_work_unassigned",
		"mc_work_unionid_external_userid_mapping_0", "mc_work_unionid_external_userid_mapping_1", "mc_work_unionid_external_userid_mapping_2",
		"mc_work_unionid_external_userid_mapping_3", "mc_work_unionid_external_userid_mapping_4", "mc_work_unionid_external_userid_mapping_5",
		"mc_work_unionid_external_userid_mapping_6", "mc_work_unionid_external_userid_mapping_7", "mc_work_unionid_external_userid_mapping_8",
		"mc_work_unionid_external_userid_mapping_9", "mc_work_update_time",
	}
	for index, table := range corpTables {
		add(table, table, "corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = ?)", DatasetActionDelete, RetentionOperational, 500+index)
	}

	add("mc_sys_log", "mc_sys_log", "operate_id IN (SELECT id FROM mc_user WHERE tenant_id = ?)", DatasetActionDelete, RetentionAudit, 700)
	add("mochat_go_seed_versions", "mochat_go_seed_versions", "scope = 'tenant' AND target_id = ?", DatasetActionDelete, RetentionOperational, 701)
	add("mochat_go_saas_admin_operation_logs", "mochat_go_saas_admin_operation_logs", "tenant_id = ?", DatasetActionRedact, RetentionAudit, 800)
	add("mochat_go_saas_identity_login_events", "mochat_go_saas_identity_login_events", "tenant_id = ?", DatasetActionRedact, RetentionAudit, 801)
	add("mochat_go_saas_identity_security_incidents", "mochat_go_saas_identity_security_incidents", "tenant_id = ?", DatasetActionRedact, RetentionAudit, 802)
	add("mochat_go_saas_admin_audit_chains", "mochat_go_saas_admin_audit_chains", "tenant_id = ?", DatasetActionRetain, RetentionAudit, 803)
	add("mochat_go_saas_admin_audit_verifications", "mochat_go_saas_admin_audit_verifications", "tenant_id = ?", DatasetActionRetain, RetentionAudit, 804)
	add("mochat_go_saas_admin_audit_anchor_checkpoints", "mochat_go_saas_admin_audit_anchor_checkpoints", "tenant_id = ?", DatasetActionRetain, RetentionAudit, 805)
	add("mc_user", "mc_user", "tenant_id = ?", DatasetActionDelete, RetentionOperational, 900)
	add("mc_corp", "mc_corp", "tenant_id = ?", DatasetActionDelete, RetentionOperational, 901)
	add("mc_tenant", "mc_tenant", "id = ?", DatasetActionTombstone, RetentionAudit, 1000)

	sort.SliceStable(specs, func(i, j int) bool {
		if specs[i].DeleteOrder == specs[j].DeleteOrder {
			return specs[i].Key < specs[j].Key
		}
		return specs[i].DeleteOrder < specs[j].DeleteOrder
	})
	return specs
}

func InventoryCoveredTables() map[string]struct{} {
	covered := make(map[string]struct{})
	for _, spec := range Inventory() {
		covered[spec.Table] = struct{}{}
	}
	for _, table := range []string{
		"mochat_go_saas_compliance_policies", "mochat_go_saas_legal_holds", "mochat_go_saas_data_exports",
		"mochat_go_saas_erasure_requests", "mochat_go_saas_erasure_steps", "mochat_go_saas_tenant_tombstones",
		"mochat_go_saas_release_evidence", "mochat_go_saas_release_candidates",
	} {
		covered[table] = struct{}{}
	}
	return covered
}
