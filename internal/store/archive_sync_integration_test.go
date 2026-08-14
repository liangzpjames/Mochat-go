package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

func TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	t.Cleanup(func() {
		executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	})

	store := NewMySQLStore(db)
	run, err := store.EnqueueArchiveSync(context.Background(), archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:contract-run", Namespace: "MOCHAT-SIM:contract-run", IdempotencyKey: "contract-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.ID == "" || run.Status != archiveprovider.SyncStatusQueued {
		t.Fatalf("queued run=%#v", run)
	}
	started := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	if err := store.MarkArchiveSyncRunning(context.Background(), run.ID, started); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveArchiveSyncCursor(context.Background(), run.ID, archiveprovider.Cursor{Sequence: 13, Token: "opaque"}, started.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	completed, err := store.CompleteArchiveSync(context.Background(), run.ID, archiveprovider.SyncCounts{Fetched: 13, Processed: 12, Skipped: 1}, archiveprovider.Cursor{Sequence: 13, Token: "opaque"}, started.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != archiveprovider.SyncStatusSucceeded || completed.Cursor.Sequence != 13 || completed.Counts.Processed != 12 {
		t.Fatalf("completed run=%#v", completed)
	}
	replay, err := store.EnqueueArchiveSync(context.Background(), archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:contract-run", Namespace: "MOCHAT-SIM:contract-run", IdempotencyKey: "contract-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Idempotent || replay.ID != run.ID {
		t.Fatalf("replay=%#v", replay)
	}

	var auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_audits WHERE run_id=? AND tenant_id=? AND corp_id=?`, runIDInt(run.ID), 11, 27).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount < 3 {
		t.Fatalf("audit rows=%d, want enqueue/start/complete at minimum", auditCount)
	}
	otherTenant, err := store.GetArchiveSourceStatus(context.Background(), dashboardprincipal.DashboardPrincipal{TenantID: 12, CorpID: 27})
	if err != nil {
		t.Fatal(err)
	}
	if otherTenant.Code != "" {
		t.Fatalf("cross-tenant status=%#v", otherTenant)
	}
}

func executeArchiveMigrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	path := filepath.Join("..", "..", "deploy", "standalone", "migrations", name)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(contents), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") && !strings.Contains(statement, "CREATE TABLE") && !strings.Contains(statement, "DROP TABLE") {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
}
