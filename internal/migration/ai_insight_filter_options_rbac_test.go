package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIInsightFilterOptionsRBACMigrationCoversBothPages(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0161_ai_insight_filter_options_rbac.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0161_ai_insight_filter_options_rbac.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"dashboard.ai_insight.session_analysis",
		"/dashboard/ai-insight/session-analysis/filter-options",
		"dashboard.ai_insight.smart_analysis",
		"/dashboard/ai-insight/smart-analysis/filter-options",
		"scope_required`, `status`, `version`",
	} {
		if !strings.Contains(string(up), fragment) {
			t.Fatalf("up migration missing %q", fragment)
		}
	}
	if !strings.Contains(string(down), "filter-options") {
		t.Fatal("down migration must remove only filter-options resources")
	}
	if _, err := SplitSQLStatements(string(up)); err != nil {
		t.Fatalf("up migration is not executable by production splitter: %v", err)
	}
	if _, err := SplitSQLStatements(string(down)); err != nil {
		t.Fatalf("down migration is not executable by production splitter: %v", err)
	}
}
