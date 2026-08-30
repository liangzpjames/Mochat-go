//go:build integration

package migration

import (
	"context"
	"database/sql"
	"testing"
)

func TestPublicPoolParityMigrationApplyAndRollbackOnIsolatedMariaDB(t *testing.T) {
	db := newMigrationIntegrationDBThrough(t, "0107_scrm_contact_lifecycle_idempotency")
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES('contact-1',1,2,'Historical contact','13800000000',1,NOW(6),NOW(6))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_leads(id,tenant_id,corp_id,business_key,name,phone,source,status,owner_id,converted_contact_id,discard_reason,version,created_at,updated_at) VALUES('lead-1',1,2,'public-pool-lead','Historical lead','13800000000','wecom','converted',NULL,'contact-1','',1,NOW(6),NOW(6))`); err != nil {
		t.Fatal(err)
	}

	runner := newMigrationRunnerThrough(t, db, "0108_scrm_public_pool_parity")
	items, err := runner.Apply(context.Background())
	if err != nil || len(items) == 0 || items[len(items)-1].Migration.Version != "0108_scrm_public_pool_parity" || items[len(items)-1].State != "applied_now" {
		t.Fatalf("apply items=%#v err=%v", items, err)
	}
	for _, column := range []string{"source", "business_type", "region"} {
		if !columnExists(t, db, "mochat_go_scrm_contacts", column) {
			t.Fatalf("column %s was not created by 0108", column)
		}
	}
	if !tableExists(t, db, "mochat_go_scrm_assignment_history") {
		t.Fatal("assignment history table was not created by 0108")
	}
	for _, expected := range []struct{ table, index string }{
		{"mochat_go_scrm_contacts", "idx_scrm_contacts_public_pool_filters"},
		{"mochat_go_scrm_assignments", "idx_scrm_assignments_public_pool"},
		{"mochat_go_scrm_assignment_history", "idx_scrm_pool_history_contact"},
		{"mochat_go_scrm_assignment_history", "idx_scrm_pool_history_filter"},
	} {
		if !indexExists(t, db, expected.table, expected.index) {
			t.Fatalf("index %s on %s was not created by 0108", expected.index, expected.table)
		}
	}
	var source string
	if err := db.QueryRow(`SELECT source FROM mochat_go_scrm_contacts WHERE id='contact-1'`).Scan(&source); err != nil || source != "wecom" {
		t.Fatalf("source backfill=%q err=%v", source, err)
	}
	if rolledBack, err := runner.RollbackLast(context.Background()); err != nil || rolledBack != "0108_scrm_public_pool_parity" {
		t.Fatalf("rollback=%q err=%v", rolledBack, err)
	}
	if tableExists(t, db, "mochat_go_scrm_assignment_history") {
		t.Fatal("assignment history table still exists after 0108 down")
	}
	for _, column := range []string{"source", "business_type", "region"} {
		if columnExists(t, db, "mochat_go_scrm_contacts", column) {
			t.Fatalf("column %s still exists after 0108 down", column)
		}
	}
	if indexExists(t, db, "mochat_go_scrm_assignments", "idx_scrm_assignments_public_pool") {
		t.Fatal("assignment public-pool index still exists after 0108 down")
	}
	var migrationRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version=?`, "0108_scrm_public_pool_parity").Scan(&migrationRows); err != nil || migrationRows != 0 {
		t.Fatalf("migration rows=%d err=%v", migrationRows, err)
	}
}

func tableExists(t *testing.T, db interface{ QueryRow(string, ...any) *sql.Row }, table string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}
