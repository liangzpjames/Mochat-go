package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderSchemaReconcileMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0134_reconcile_phase3_provider_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0134_reconcile_phase3_provider_schema.down.sql"))
	if err != nil {
		t.Fatal(err)
	}

	body := string(up)
	requiredTables := []string{
		"mochat_go_saas_alerts",
		"mochat_go_saas_service_accounts",
		"mochat_go_saas_service_account_keys",
		"mochat_go_scrm_contacts",
		"mochat_go_scrm_tag_groups",
		"mochat_go_risk_rules",
		"mochat_go_risk_rule_strategies",
		"mochat_go_risk_records",
		"mochat_go_risk_record_audits",
		"mochat_go_timeout_rules",
		"mochat_go_timeout_rule_strategies",
		"mochat_go_timeout_rule_quiet_periods",
		"mochat_go_timeout_rule_notify_targets",
		"mochat_go_timeout_settings",
		"mochat_go_timeout_records",
		"mochat_go_timeout_record_audits",
		"mochat_go_timeout_notification_intents",
		"mochat_go_keyword_libraries",
		"mochat_go_keyword_entries",
		"mochat_go_keyword_versions",
		"mochat_go_keyword_version_entries",
		"mochat_go_message_intercept_rules",
		"mochat_go_message_intercept_records",
		"mochat_go_message_intercept_audits",
		"mochat_go_silent_customer_rules",
		"mochat_go_silent_customer_records",
		"mochat_go_refuse_archive_records",
		"mc_friends_circle_tasks",
		"mc_friends_circle_materials",
		"mc_friends_circle_task_results",
		"mc_phase34_acquisition_links",
		"mc_phase34_customer_services",
		"mc_phase34_short_links",
		"mc_phase34_short_link_visits",
		"mochat_go_audio_objects",
		"mochat_go_ai_analysis",
	}
	for _, table := range requiredTables {
		if !strings.Contains(body, "CREATE TABLE IF NOT EXISTS `"+table+"`") {
			t.Errorf("0134 must reconcile table %s", table)
		}
	}

	requiredConditionalChanges := []string{
		"mc_friends_circle_tasks.publish_attempts",
		"mc_friends_circle_tasks.last_callback_at",
		"mc_friends_circle_tasks.medium_id",
		"mc_work_room_auto_pull.medium_id",
		"mc_contact_message_batch_send.medium_id",
		"mc_room_message_batch_send.medium_id",
		"mochat_go_scrm_tags.group_id",
		"mochat_go_scrm_tags.active_group_name",
	}
	for _, marker := range requiredConditionalChanges {
		if !strings.Contains(body, "-- reconcile: "+marker) {
			t.Errorf("0134 missing conditional change marker %s", marker)
		}
	}
	if strings.Contains(strings.ToUpper(body), "ADD COLUMN IF NOT EXISTS") {
		t.Fatal("0134 must remain compatible with MySQL 5.7; use information_schema + PREPARE")
	}
	if strings.Contains(strings.ToUpper(body), "DROP TABLE") || strings.Contains(strings.ToUpper(body), "DELETE FROM") {
		t.Fatal("0134 reconciliation must not remove business data")
	}
	if strings.Contains(strings.ToUpper(string(down)), "DROP ") || strings.Contains(strings.ToUpper(string(down)), "DELETE ") {
		t.Fatal("0134 down migration must be intentionally non-destructive")
	}
	if _, err := SplitSQLStatements(body); err != nil {
		t.Fatalf("0134 must be executable by the production SQL splitter: %v", err)
	}
}
