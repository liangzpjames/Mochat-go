package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDualAnalysisAssistantsMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	var target Migration
	for _, migration := range DefaultMigrations(root) {
		if migration.Version == "0159_dual_analysis_assistants" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0159 dual analysis assistants migration was not discovered")
	}

	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	down := string(downBody)

	for _, required := range []string{
		"ADD COLUMN `customer_analysis_prompt` TEXT NULL",
		"ADD COLUMN `employee_qa_prompt` TEXT NULL",
		"mochat_go_ai_agents",
		"'smart-analysis'",
		"'智能分析助手'",
		"session_agent.`knowledge_base_ids`",
		"'session-analysis'",
		"'会话分析规则'",
		"客户购买意向",
		"流失风险",
		"核心需求",
		"员工服务质检",
		"客户异议",
		"改进建议",
		"mochat_go_ai_analysis_rule_versions",
		"'default-smart-analysis'",
		"'智能分析规则'",
		"AND NOT EXISTS",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0159 up migration missing %q", required)
		}
	}
	if strings.Contains(up, "UPDATE `mochat_go_ai_analysis_rules`\nSET `system_key`") {
		t.Fatal("0159 up must preserve the default-smart-analysis persistence key")
	}

	for _, required := range []string{
		"rule_version_id",
		"LEFT JOIN `mochat_go_ai_conversation_insights` insight",
		"insight.`id` IS NULL",
		"LEFT JOIN `mochat_go_ai_insight_runs` insight_run",
		"insight_run.`id` IS NULL",
		"`system_key` = 'session-analysis'",
		"`system_key` = 'smart-analysis'",
		"Prompt columns are deliberately retained",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0159 down migration missing safe rollback guard %q", required)
		}
	}
	for _, unsafe := range []string{
		"DELETE FROM `mochat_go_ai_conversation_insights`",
		"DROP COLUMN `customer_analysis_prompt`",
		"DROP COLUMN `employee_qa_prompt`",
	} {
		if strings.Contains(down, unsafe) {
			t.Fatalf("0159 down migration must preserve historical insight safety; found %q", unsafe)
		}
	}
	if _, err := SplitSQLStatements(up); err != nil {
		t.Fatalf("0159 up migration is not executable by production splitter: %v", err)
	}
	if _, err := SplitSQLStatements(down); err != nil {
		t.Fatalf("0159 down migration is not executable by production splitter: %v", err)
	}
}

func TestDualAnalysisPromptDataContract(t *testing.T) {
	path := filepath.Join("..", "modules", "ai-settings", "ports", "repository.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contract := string(body)
	for _, required := range []string{
		"SmartAnalysisSystemKey",
		"SmartAnalysisAssistantName",
		"SessionAnalysisRuleName",
		"CustomerAnalysisPrompt",
		"EmployeeQAPrompt",
		"MaxAnalysisPromptRunes",
	} {
		if !strings.Contains(contract, required) {
			t.Fatalf("AI settings prompt contract missing %q", required)
		}
	}
}
