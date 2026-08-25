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
