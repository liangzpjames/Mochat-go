package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAISettingsSchemaReconcileMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0137_reconcile_ai_settings_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0137_reconcile_ai_settings_schema.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(up)
	for _, table := range []string{"mochat_go_ai_knowledge_bases", "mochat_go_ai_agents"} {
		if !strings.Contains(body, "CREATE TABLE IF NOT EXISTS `"+table+"`") {
			t.Errorf("0137 must reconcile table %s", table)
		}
	}
	if strings.Contains(strings.ToUpper(body), "DROP ") || strings.Contains(strings.ToUpper(body), "DELETE ") || strings.Contains(strings.ToUpper(string(down)), "DROP ") {
		t.Fatal("0137 reconciliation must be non-destructive")
	}
	if _, err := SplitSQLStatements(body); err != nil {
		t.Fatalf("0137 must be executable by the production SQL splitter: %v", err)
	}
}
