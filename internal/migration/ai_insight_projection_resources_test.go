package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIInsightProjectionResources0163MigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upBytes, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0163_ai_insight_projection_resources.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	downBytes, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0163_ai_insight_projection_resources.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, down := string(upBytes), string(downBytes)
	views := map[string]string{
		"dashboard.ai_insight.emotion":               "emotion",
		"dashboard.ai_insight.employee_score":        "employee-score",
		"dashboard.ai_insight.communication_keyword": "communication-keyword",
	}
	for permission, view := range views {
		if !strings.Contains(up, permission) {
			t.Errorf("0163 must target permission %q", permission)
		}
		for _, action := range []string{"records", "detail", "status", "filter-options", "export"} {
			path := "/dashboard/ai-insight/" + view + "/" + action
			if strings.Count(up, path) != 1 {
				t.Errorf("0163 up path %q count=%d, want 1", path, strings.Count(up, path))
			}
		}
	}
	for _, fragment := range []string{"'api'", "'GET'", "scope_required", "NOT EXISTS", "status", "version"} {
		if !strings.Contains(up, fragment) {
			t.Errorf("0163 up missing %q", fragment)
		}
	}
	upperDown := strings.ToUpper(down)
	if strings.Contains(upperDown, "DELETE") || strings.Contains(upperDown, "UPDATE") {
		t.Fatal("0163 down must fail closed and preserve resources whose pre-migration ownership is unknown")
	}
	if !strings.Contains(down, "ownership is unknown") || !strings.Contains(upperDown, "DO 0") {
		t.Fatal("0163 down must document and execute its conservative no-op rollback")
	}
}
