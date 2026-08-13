package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCRMCustomerSchemaReconcileMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	up, err := os.ReadFile(filepath.Join(root, "0136_reconcile_scrm_customer_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "0136_reconcile_scrm_customer_schema.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(up)
	for _, marker := range []string{
		"mochat_go_scrm_contacts.source",
		"mochat_go_scrm_contacts.business_type",
		"mochat_go_scrm_contacts.region",
		"mochat_go_scrm_contacts.idx_scrm_contacts_public_pool_filters",
		"mochat_go_scrm_assignments.idx_scrm_assignments_public_pool",
	} {
		if !strings.Contains(body, "-- reconcile: "+marker) {
			t.Errorf("0136 missing conditional change marker %s", marker)
		}
	}
	for _, table := range []string{"mochat_go_scrm_idempotency_keys", "mochat_go_scrm_assignment_history"} {
		if !strings.Contains(body, "CREATE TABLE IF NOT EXISTS `"+table+"`") {
			t.Errorf("0136 must reconcile table %s", table)
		}
	}
	if strings.Contains(strings.ToUpper(body), "ADD COLUMN IF NOT EXISTS") {
		t.Fatal("0136 must remain compatible with MySQL 5.7")
	}
	if strings.Contains(strings.ToUpper(body), "DROP ") || strings.Contains(strings.ToUpper(body), "DELETE ") || strings.Contains(strings.ToUpper(string(down)), "DROP ") {
		t.Fatal("0136 reconciliation must be non-destructive")
	}
	if _, err := SplitSQLStatements(body); err != nil {
		t.Fatalf("0136 must be executable by the production SQL splitter: %v", err)
	}
}
