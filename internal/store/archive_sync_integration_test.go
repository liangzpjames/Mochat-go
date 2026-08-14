package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

func TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
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

func TestArchiveSyncStaleRunningRunIsTakenOverWithAudit(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	store := NewMySQLStore(db)
	template := archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:stale", Namespace: "MOCHAT-SIM:stale", IdempotencyKey: "stale-1",
	}
	run, err := store.EnqueueArchiveSync(context.Background(), template, false)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-10 * time.Minute)
	if err := store.MarkArchiveSyncRunning(context.Background(), run.ID, started); err != nil {
		t.Fatal(err)
	}
	taken, err := store.EnqueueArchiveSync(context.Background(), template, false)
	if err != nil {
		t.Fatal(err)
	}
	if taken.ID != run.ID || taken.Status != archiveprovider.SyncStatusQueued || taken.Attempt != 2 || taken.ErrorCode != "archive.stale_takeover" {
		t.Fatalf("stale takeover=%#v", taken)
	}
	var audits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_audits WHERE run_id=? AND action='stale_takeover' AND error_code='archive.stale_takeover'`, runIDInt(run.ID)).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("stale takeover audits=%d", audits)
	}
}

func TestArchiveSyncConcurrentFirstEnqueueRereadsDuplicateRun(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	store := NewMySQLStore(db)
	template := archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:concurrent", Namespace: "MOCHAT-SIM:concurrent", IdempotencyKey: "concurrent-1",
	}
	type result struct {
		run archiveprovider.SyncRun
		err error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			run, err := store.EnqueueArchiveSync(context.Background(), template, false)
			results <- result{run: run, err: err}
		}()
	}
	wait.Wait()
	close(results)
	var firstID string
	for item := range results {
		if item.err != nil {
			t.Fatal(item.err)
		}
		if firstID == "" {
			firstID = item.run.ID
		} else if item.run.ID != firstID {
			t.Fatalf("duplicate enqueue IDs=%s,%s", firstID, item.run.ID)
		}
	}
	var runs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:concurrent' AND idempotency_key='concurrent-1'`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("concurrent duplicate runs=%d", runs)
	}
}

func TestArchiveSyncEnqueueRejectsNamespaceMismatchWithoutMutation(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	store := NewMySQLStore(db)
	base := archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:namespace", Namespace: "MOCHAT-SIM:namespace-a", IdempotencyKey: "namespace-1",
	}
	run, err := store.EnqueueArchiveSync(context.Background(), base, false)
	if err != nil {
		t.Fatal(err)
	}
	conflict := base
	conflict.Namespace = "MOCHAT-SIM:namespace-b"
	if _, err := store.EnqueueArchiveSync(context.Background(), conflict, false); err == nil {
		t.Fatal("namespace mismatch unexpectedly reused existing run")
	}
	var runs, audits, attempt int
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(attempt),0) FROM mochat_go_archive_sync_runs WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:namespace' AND idempotency_key='namespace-1'`).Scan(&runs, &attempt); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_audits WHERE run_id=?`, runIDInt(run.ID)).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || attempt != 1 || audits != 1 {
		t.Fatalf("namespace mismatch mutated runs=%d attempt=%d audits=%d", runs, attempt, audits)
	}
}

func TestArchiveSyncMigrationRejectsIncompleteResidualTable(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_sync_runs (id BIGINT UNSIGNED NOT NULL PRIMARY KEY) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil {
		t.Fatal("incomplete residual archive runs table unexpectedly passed migration guard")
	}
	if _, dropErr := db.Exec("DROP TABLE mochat_go_archive_sync_runs"); dropErr != nil {
		t.Fatal(dropErr)
	}
}

func TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	if _, err := db.Exec("DROP TABLE mochat_go_archive_message_sources"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_message_sources (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		tenant_id INT UNSIGNED NOT NULL, corp_id INT UNSIGNED NOT NULL, msgid VARCHAR(255) NOT NULL,
		source_kind VARCHAR(16) NOT NULL, source_id VARCHAR(128) NOT NULL, namespace VARCHAR(128) NOT NULL,
		run_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
		UNIQUE KEY uk_archive_message_source_scope_msg (tenant_id, corp_id, msgid),
		CONSTRAINT fk_archive_message_source_run FOREIGN KEY (run_id) REFERENCES mochat_go_archive_sync_runs(id)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive message sources table") {
		t.Fatal("wrong composite source foreign key unexpectedly passed migration guard")
	}
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
}

func TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	if _, err := db.Exec("DROP TABLE mochat_go_archive_message_sources"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_message_sources (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		tenant_id INT UNSIGNED NOT NULL, corp_id INT UNSIGNED NOT NULL, msgid VARCHAR(255) NOT NULL,
		source_kind VARCHAR(16) NOT NULL, source_id VARCHAR(128) NOT NULL, namespace VARCHAR(128) NOT NULL,
		run_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
		KEY uk_archive_message_source_scope_msg (tenant_id, corp_id, msgid),
		CONSTRAINT fk_archive_message_source_run FOREIGN KEY (tenant_id,corp_id,run_id,source_kind,source_id,namespace)
			REFERENCES mochat_go_archive_sync_runs (tenant_id,corp_id,id,source_kind,source_id,namespace)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive message sources table") {
		t.Fatal("non-unique residual scope index unexpectedly passed migration guard")
	}
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
}

func TestArchiveSyncMigrationApplyDownApplyAndRejectsCrossTenantRun(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")

	_, err := db.Exec(`INSERT INTO mochat_go_archive_sync_runs
		(tenant_id, corp_id, source_kind, source_id, namespace, idempotency_key, status)
		VALUES (12, 27, 'simulated', 'simulation:cross-tenant', 'MOCHAT-SIM:cross-tenant', 'cross-tenant', 'queued')`)
	if err == nil {
		t.Fatal("cross-tenant archive run unexpectedly inserted")
	}
	result, err := db.Exec(`INSERT INTO mochat_go_archive_sync_runs
		(tenant_id, corp_id, source_kind, source_id, namespace, idempotency_key, status)
		VALUES (11, 27, 'simulated', 'simulation:parent', 'MOCHAT-SIM:parent', 'parent-1', 'queued')`)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO mochat_go_archive_message_sources
		(tenant_id, corp_id, msgid, source_kind, source_id, namespace, run_id)
		VALUES (11, 27, 'MOCHAT-SIM:parent:001', 'simulated', 'simulation:other', 'MOCHAT-SIM:parent', ?)`, runID)
	if err == nil {
		t.Fatal("source identity mismatch unexpectedly bypassed composite child foreign key")
	}
}

func TestArchiveSyncUpsertValidatesRunScopeAndRollsBackSourceFailure(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	createArchiveMessageUpsertFixture(t, db)

	ctx := context.Background()
	store := NewMySQLStore(db)
	run, err := store.EnqueueArchiveSync(ctx, archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:atomic", Namespace: "MOCHAT-SIM:atomic", IdempotencyKey: "atomic-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkArchiveSyncRunning(ctx, run.ID, time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	message := archiveprovider.Message{
		Source: providers.SourceSimulated, SourceID: "simulation:atomic", Namespace: "MOCHAT-SIM:atomic",
		MsgID: "MOCHAT-SIM:atomic:001", Seq: 1, From: "employee-atomic", ToList: []string{"contact-atomic"},
		MsgType: "text", ContentRaw: `{"content":"atomic"}`, ContentText: "atomic",
	}
	if _, err := store.UpsertArchiveMessage(ctx, run.ID, archiveprovider.Scope{TenantID: 12, CorpID: 27}, message); err == nil {
		t.Fatal("wrong-tenant archive upsert unexpectedly succeeded")
	}
	if _, err := store.UpsertArchiveMessage(ctx, run.ID, archiveprovider.Scope{TenantID: 11, CorpID: 27}, archiveprovider.Message{
		Source: providers.SourceExternal, SourceID: "wecom:27", Namespace: "wecom", MsgID: message.MsgID, Seq: message.Seq,
	}); err == nil {
		t.Fatal("wrong-source archive upsert unexpectedly succeeded")
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 0, 0)

	if _, err := db.Exec(`CREATE TRIGGER archive_source_atomic_fault BEFORE INSERT ON mochat_go_archive_message_sources
		FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'archive_source_atomic_fault'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertArchiveMessage(ctx, run.ID, archiveprovider.Scope{TenantID: 11, CorpID: 27}, message); err == nil {
		t.Fatal("source fault unexpectedly allowed archive upsert")
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 0, 0)
	if _, err := db.Exec("DROP TRIGGER archive_source_atomic_fault"); err != nil {
		t.Fatal(err)
	}
	result, err := store.UpsertArchiveMessage(ctx, run.ID, archiveprovider.Scope{TenantID: 11, CorpID: 27}, message)
	if err != nil || !result.Inserted {
		t.Fatalf("successful atomic upsert result=%#v err=%v", result, err)
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 1, 1)
}

func createArchiveMessageUpsertFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for index := 1; index <= 10; index++ {
		if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE mc_work_message_%d (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL, msgid VARCHAR(255) NOT NULL,
			seq BIGINT NOT NULL, work_employee_id INT NOT NULL, to_user_type INT NOT NULL, to_user_id INT NOT NULL,
			sender_type INT NOT NULL, action INT NOT NULL, type INT NOT NULL, msg_type INT NOT NULL,
			content TEXT NOT NULL, content_text TEXT NOT NULL, room_id INT NOT NULL DEFAULT 0, status INT NOT NULL DEFAULT 0,
			msg_data_time DATETIME NULL, created_at DATETIME NULL, updated_at DATETIME NULL, deleted_at DATETIME NULL,
			PRIMARY KEY (id), KEY idx_archive_fixture_msg (corp_id, msgid)
		) ENGINE=InnoDB`, index)); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`CREATE TABLE mc_work_employee (id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL, wx_user_id VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', deleted_at DATETIME NULL, PRIMARY KEY (id), KEY idx_archive_fixture_employee (corp_id, wx_user_id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_contact (id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL, wx_external_userid VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', deleted_at DATETIME NULL, PRIMARY KEY (id), KEY idx_archive_fixture_contact (corp_id, wx_external_userid)) ENGINE=InnoDB`,
		`INSERT INTO mc_work_employee (id, corp_id, wx_user_id, name) VALUES (1001, 27, 'employee-atomic', 'Atomic employee')`,
		`INSERT INTO mc_work_contact (id, corp_id, wx_external_userid, name) VALUES (2001, 27, 'contact-atomic', 'Atomic contact')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func assertArchiveMessageWriteCounts(t *testing.T, db *sql.DB, msgID string, wantMessage, wantSource int) {
	t.Helper()
	var messageCount, sourceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_work_message_1 WHERE corp_id=27 AND msgid=?`, msgID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_message_sources WHERE tenant_id=11 AND corp_id=27 AND msgid=?`, msgID).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != wantMessage || sourceCount != wantSource {
		t.Fatalf("message rows=%d source rows=%d, want %d/%d", messageCount, sourceCount, wantMessage, wantSource)
	}
}

func createArchiveSyncCorpFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mc_corp (id INT(10) UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT(10) UNSIGNED NOT NULL, chat_status TINYINT NOT NULL DEFAULT 1, deleted_at DATETIME NULL, PRIMARY KEY (id), UNIQUE KEY uni_mc_corp_tenant_id_id (tenant_id, id)) ENGINE=InnoDB`,
		`INSERT INTO mc_corp (id, tenant_id, deleted_at) VALUES (27, 11, NULL)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func executeArchiveMigrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	if err := executeArchiveMigrationFileErr(db, name); err != nil {
		t.Fatalf("migration %s: %v", name, err)
	}
}

func executeArchiveMigrationFileErr(db *sql.DB, name string) error {
	path := filepath.Join("..", "..", "deploy", "standalone", "migrations", name)
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, statement := range strings.Split(string(contents), ";") {
		lines := strings.Split(statement, "\n")
		withoutComments := make([]string, 0, len(lines))
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				withoutComments = append(withoutComments, line)
			}
		}
		statement = strings.Join(withoutComments, "\n")
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") && !strings.Contains(statement, "CREATE TABLE") && !strings.Contains(statement, "DROP TABLE") {
			continue
		}
		if _, err := conn.ExecContext(context.Background(), statement); err != nil {
			return err
		}
	}
	return nil
}
