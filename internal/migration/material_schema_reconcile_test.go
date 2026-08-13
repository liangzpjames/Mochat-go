package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialSchemaReconcileMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0135_reconcile_material_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0135_reconcile_material_schema.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(up)
	for _, marker := range []string{
		"mc_medium.scope_type",
		"mc_medium.scope_id",
		"mc_medium.sidebar_visible",
		"mc_medium.status",
		"mc_medium.idx_medium_scope",
		"mc_medium.idx_medium_selector",
	} {
		if !strings.Contains(body, "-- reconcile: "+marker) {
			t.Errorf("0135 missing conditional change marker %s", marker)
		}
	}
	if strings.Contains(strings.ToUpper(body), "ADD COLUMN IF NOT EXISTS") {
		t.Fatal("0135 must remain compatible with MySQL 5.7")
	}
	if strings.Contains(strings.ToUpper(body), "DROP ") || strings.Contains(strings.ToUpper(body), "DELETE ") || strings.Contains(strings.ToUpper(string(down)), "DROP ") {
		t.Fatal("0135 reconciliation must be non-destructive")
	}
	if _, err := SplitSQLStatements(body); err != nil {
		t.Fatalf("0135 must be executable by the production SQL splitter: %v", err)
	}
}
