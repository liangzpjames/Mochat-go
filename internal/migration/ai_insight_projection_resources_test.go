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
		if !strings.Contains(up, permission) || !strings.Contains(down, permission) {
			t.Errorf("0163 must target permission %q in both directions", permission)
		}
		for _, action := range []string{"records", "detail", "status", "filter-options", "export"} {
			path := "/dashboard/ai-insight/" + view + "/" + action
			if strings.Count(up, path) != 1 {
				t.Errorf("0163 up path %q count=%d, want 1", path, strings.Count(up, path))
			}
			if strings.Count(down, path) != 1 {
				t.Errorf("0163 down path %q count=%d, want 1", path, strings.Count(down, path))
			}
		}
	}
	for _, fragment := range []string{"'api'", "'GET'", "scope_required", "NOT EXISTS", "status", "version"} {
		if !strings.Contains(up, fragment) {
			t.Errorf("0163 up missing %q", fragment)
		}
	}
	if !strings.Contains(down, "DELETE resource") || !strings.Contains(down, "resource.`http_method` = 'GET'") {
		t.Fatal("0163 down must delete only exact GET resources")
	}
	for _, fragment := range []string{
		"seed.`permission_code` = permission.`code`",
		"seed.`http_method` = resource.`http_method`",
		"seed.`path_pattern` = resource.`path_pattern`",
	} {
		if !strings.Contains(down, fragment) {
			t.Errorf("0163 down must pair every permission and route through %q", fragment)
		}
	}
	if strings.Contains(down, "permission.`code` IN") || strings.Contains(down, "resource.`path_pattern` IN") {
		t.Fatal("0163 down must not delete the permission/path cross product")
	}
	if strings.Contains(strings.ToUpper(down), "DELETE FROM `MOCHAT_GO_DASHBOARD_PERMISSIONS`") {
		t.Fatal("0163 down must preserve page permissions")
	}
}
