package migration_test

import (
	"context"
	"database/sql"
	"testing"
)

func TestArchiveSourceMigrationRunnerApplyDownApplyPinsOneConnection(t *testing.T) {
	db, root := newExternalMigrationIntegrationDBThrough(t, "0137_reconcile_ai_settings_schema", "archive-runner-0137")
	if _, err := db.Exec(`INSERT INTO mc_tenant (id,name,status) VALUES (11,'Archive runner tenant',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_corp (id,tenant_id,name) VALUES (27,11,'Archive runner corp')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id,corp_id,status,version) VALUES (11,27,1,1)`); err != nil {
		t.Fatal(err)
	}
	runner := newExternalMigrationRunnerThrough(t, db, root, "0138_archive_source_sync")
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertArchiveRunnerTables(t, db, true)
	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0138_archive_source_sync'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("applied ledger rows=%d", applied)
	}
	if _, err := runner.RollbackLast(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertArchiveRunnerTables(t, db, false)
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_sync_runs (id BIGINT UNSIGNED NOT NULL PRIMARY KEY) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err == nil {
		t.Fatal("incompatible residual archive table unexpectedly applied")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0138_archive_source_sync'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Fatalf("failed apply recorded ledger rows=%d", applied)
	}
	if _, err := db.Exec(`DROP TABLE mochat_go_archive_sync_runs`); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertArchiveRunnerTables(t, db, true)
	if statuses, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	} else {
		found := false
		for _, status := range statuses {
			if status.Migration.Version == "0138_archive_source_sync" && status.State == "applied" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("repeat apply missing applied 0138 status: %#v", statuses)
		}
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0138_archive_source_sync'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("repeat apply ledger rows=%d", applied)
	}
}

func assertArchiveRunnerTables(t *testing.T, db *sql.DB, want bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_archive_sync_runs','mochat_go_archive_sync_audits','mochat_go_archive_message_sources')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if (count == 3) != want {
		t.Fatalf("archive tables=%d wantPresent=%v", count, want)
	}
}
