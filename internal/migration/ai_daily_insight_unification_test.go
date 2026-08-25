package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIDailyInsightUnification0165MigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0165_ai_daily_insight_unification."+name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	up := read("up")
	down := read("down")
	for _, fragment := range []string{
		"ADD COLUMN IF NOT EXISTS `analysis_date` date",
		"ADD COLUMN IF NOT EXISTS `previous_insight_id` bigint unsigned",
		"ADD COLUMN IF NOT EXISTS `previous_score` decimal(6,2)",
		"ADD COLUMN IF NOT EXISTS `previous_summary` varchar(1200)",
		"ADD COLUMN IF NOT EXISTS `previous_generated_at` datetime(6)",
		"UPDATE `mochat_go_ai_conversation_insights`",
		"DELETE duplicate_row",
		"DROP INDEX IF EXISTS `uq_ai_conversation_source`",
		"UNIQUE KEY IF NOT EXISTS `uq_ai_conversation_daily` (`tenant_id`,`corp_id`,`analysis_type`,`rule_version_id`,`conversation_key`,`analysis_date`)",
		"DROP TABLE IF EXISTS `mochat_go_ai_analysis`",
	} {
		if !strings.Contains(up, fragment) {
			t.Errorf("up migration missing %q", fragment)
		}
	}
	if strings.Contains(up, "DROP TABLE `mochat_go_ai_analysis_rules`") ||
		strings.Contains(up, "DROP TABLE `mochat_go_ai_analysis_rule_versions`") ||
		strings.Contains(up, "DROP TABLE `mochat_go_ai_conversation_insights`") ||
		strings.Contains(up, "DROP TABLE `mochat_go_ai_insight_runs`") {
		t.Fatal("up migration drops a retained AI insight table")
	}

	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS `mochat_go_ai_analysis`",
		"DROP INDEX IF EXISTS `uq_ai_conversation_daily`",
		"UNIQUE KEY IF NOT EXISTS `uq_ai_conversation_source` (`tenant_id`,`corp_id`,`analysis_type`,`rule_version_id`,`conversation_key`,`source_fingerprint`)",
		"DROP COLUMN IF EXISTS `previous_generated_at`",
		"DROP COLUMN IF EXISTS `previous_summary`",
		"DROP COLUMN IF EXISTS `previous_score`",
		"DROP COLUMN IF EXISTS `previous_insight_id`",
		"DROP COLUMN IF EXISTS `analysis_date`",
	} {
		if !strings.Contains(down, fragment) {
			t.Errorf("down migration missing %q", fragment)
		}
	}
	if strings.Contains(down, "INSERT INTO `mochat_go_ai_analysis`") {
		t.Fatal("down migration must recreate an empty legacy table without fabricating data")
	}
}

func TestIsAIDailyMigrationIntegrationSchemaStrictlyDedicated(t *testing.T) {
	for _, name := range []string{
		"mochat_go_ai_insight_test",
		"mochat_go_ai_insight_test_0165",
		"mochat_go_ai_insight_integration",
		"mochat_go_ai_insight_integration_ci_2",
	} {
		if !isAIDailyMigrationIntegrationSchema(name) {
			t.Fatalf("dedicated schema %q was rejected", name)
		}
	}
	for _, name := range []string{
		"mochat_go_ai_insight_testproduction",
		"mochat_go_ai_insight_integration-prod",
		"mochat_go_ai_insight_test_",
		"mochat_go_ai_insight_test_UPPER",
		"mochat_go_ai_insight_tests",
		"production",
	} {
		if isAIDailyMigrationIntegrationSchema(name) {
			t.Fatalf("unsafe schema %q was accepted", name)
		}
	}
}

func isAIDailyMigrationIntegrationSchema(name string) bool {
	name = strings.TrimSpace(name)
	for _, base := range []string{"mochat_go_ai_insight_test", "mochat_go_ai_insight_integration"} {
		if name == base {
			return true
		}
		prefix := base + "_"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if suffix == "" {
			return false
		}
		for _, part := range strings.Split(suffix, "_") {
			if part == "" {
				return false
			}
			for _, char := range part {
				if (char < 'a' || char > 'z') && (char < '0' || char > '9') {
					return false
				}
			}
		}
		return true
	}
	return false
}
