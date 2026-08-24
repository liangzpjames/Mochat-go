package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIAssistantDefaultSmartRuleMigration(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0158_ai_assistant_default_smart_rule.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0158_ai_assistant_default_smart_rule.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"ADD COLUMN `system_key`",
		"uq_ai_rules_system_key",
		"default-smart-analysis",
		"默认智能分析规则",
		"mochat_go_ai_analysis_rule_versions",
		"'分析助手'",
		") deactivation_seed",
		"resource.`status` = 0",
		"'/dashboard/ai-insight/smart-analysis/rules/status'",
	} {
		if !strings.Contains(string(up), fragment) {
			t.Fatalf("up migration missing %q", fragment)
		}
	}
	for _, fragment := range []string{
		"SIGNAL SQLSTATE '45000'",
		"immutable smart-analysis history",
	} {
		if !strings.Contains(string(down), fragment) {
			t.Fatalf("down migration missing %q", fragment)
		}
	}
	for _, unsafeFragment := range []string{
		") restoration_seed",
		"SET resource.`status` = 1",
		"resource.`deleted_at` = NULL",
		"DELETE ",
		"DROP COLUMN",
	} {
		if strings.Contains(string(down), unsafeFragment) {
			t.Fatalf("down migration may resurrect a pre-0158 disabled/deleted RBAC resource via %q", unsafeFragment)
		}
	}
	if _, err := SplitSQLStatements(string(up)); err != nil {
		t.Fatalf("up migration is not executable by production splitter: %v", err)
	}
	if _, err := SplitSQLStatements(string(down)); err != nil {
		t.Fatalf("down migration is not executable by production splitter: %v", err)
	}
}
