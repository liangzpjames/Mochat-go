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
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/sqlscript"
	archivesourcefixture "jiyi/mochat-go/internal/testfixtures/archivesource"
)

func TestArchiveSourceMigrationContainsLegacySimulationBackfillContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0138_archive_source_sync.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ToLower(string(body))
	for _, fragment := range []string{
		"insert into `mochat_go_archive_sync_runs`",
		"insert into `mochat_go_archive_message_sources`",
		"mochat_go_archive_simulation_batches",
		"mochat_go_archive_simulation_messages",
		"concat(''simulation:'', batch.`batch_key`)",
		"concat(''mochat-sim:'', batch.`batch_key`)",
		"on duplicate key update `id` = last_insert_id(`mochat_go_archive_sync_runs`.`id`)",
	} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("0138 legacy simulation backfill missing %q", fragment)
		}
	}
}

func TestArchiveSourceMigrationBackfillsLegacySimulationRowsOnTemporaryMariaDB(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0133_archive_simulation_registry.up.sql")
	defer executeArchiveMigrationFile(t, db, "0133_archive_simulation_registry.down.sql")
	if _, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_batches (corp_id,batch_key,status,message_count) VALUES (27,'legacy-backfill','complete',1)`); err != nil {
		t.Fatal(err)
	}
	var batchID int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_archive_simulation_batches WHERE corp_id=27 AND batch_key='legacy-backfill'`).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_messages (batch_id,corp_id,msgid,table_index) VALUES (?,?,?,1)`, batchID, 27, "legacy-backfill-msg"); err != nil {
		t.Fatal(err)
	}
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	defer executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
	var runs, sources int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:legacy-backfill' AND namespace='MOCHAT-SIM:legacy-backfill' AND status='succeeded'`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_message_sources WHERE tenant_id=11 AND corp_id=27 AND msgid='legacy-backfill-msg' AND source_kind='simulated' AND source_id='simulation:legacy-backfill' AND namespace='MOCHAT-SIM:legacy-backfill'`).Scan(&sources); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || sources != 1 {
		t.Fatalf("legacy backfill runs=%d sources=%d", runs, sources)
	}
}

func TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveSyncCorpFixture(t, db)

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
	run, err = store.MarkArchiveSyncRunning(context.Background(), run.ID, started)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveArchiveSyncCursor(context.Background(), run.ID, run.Attempt, run.LeaseToken, archiveprovider.Cursor{Sequence: 13, Token: "opaque"}, started.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	completed, err := store.CompleteArchiveSync(context.Background(), run.ID, run.Attempt, run.LeaseToken, archiveprovider.SyncCounts{Fetched: 13, Processed: 12, Skipped: 1}, archiveprovider.Cursor{Sequence: 13, Token: "opaque"}, started.Add(2*time.Minute))
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

func TestArchiveSourceStatusUsesCurrentCorpArchiveMode(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveSyncCorpFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mochat_go_archive_simulation_batches (corp_id,batch_key,status,message_count) VALUES (27,'status-mode','complete',1)`); err != nil {
		t.Fatal(err)
	}
	for _, run := range []struct {
		sourceKind, sourceID, namespace, idempotency, status, errorCode string
		updatedAt                                                       string
	}{
		{"simulated", "simulation:status-mode", "MOCHAT-SIM:status-mode", "status-simulated-old", "succeeded", "", "2026-08-14 09:00:00.000000"},
		{"external", "wecom", "wecom", "status-external", "succeeded", "", "2026-08-14 10:00:00.000000"},
		{"simulated", "simulation:status-mode-new", "MOCHAT-SIM:status-mode-new", "status-simulated", "failed", "archive.simulation_fixture_failed", "2026-08-14 11:00:00.000000"},
	} {
		if _, err := db.Exec(`
			INSERT INTO mochat_go_archive_sync_runs
			(tenant_id,corp_id,source_kind,source_id,namespace,idempotency_key,status,error_code,finished_at,updated_at)
			VALUES (11,27,?,?,?,?,?,?,?,?)
		`, run.sourceKind, run.sourceID, run.namespace, run.idempotency, run.status, run.errorCode, run.updatedAt, run.updatedAt); err != nil {
			t.Fatal(err)
		}
	}
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 27}
	ctx := context.Background()
	if status, err := store.GetArchiveSourceStatus(ctx, principal); err != nil {
		t.Fatal(err)
	} else if status.Source != providers.SourceExternal || status.Code != "archive.bridge_ready" || status.State != providers.StateReady {
		t.Fatalf("real mode status=%#v", status)
	}
	if _, err := db.Exec(`UPDATE mc_corp SET chat_status=0 WHERE id=27 AND tenant_id=11`); err != nil {
		t.Fatal(err)
	}
	if status, err := store.GetArchiveSourceStatus(ctx, principal); err != nil {
		t.Fatal(err)
	} else if status.Source != providers.SourceSimulated || status.Code != "archive.simulation_failed" || status.LastErrorCode != "archive.simulation_fixture_failed" {
		t.Fatalf("simulation mode status=%#v", status)
	}
}

func TestArchiveSyncStaleRunningRunIsTakenOverWithAudit(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveSyncCorpFixture(t, db)
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
	if _, err := store.MarkArchiveSyncRunning(context.Background(), run.ID, started); err != nil {
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
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveSyncCorpFixture(t, db)
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
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveSyncCorpFixture(t, db)
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

func TestArchiveSyncMigrationRejectsSingleFactorIdempotencyKeyTypeMismatch(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	if _, err := db.Exec(`ALTER TABLE mochat_go_archive_sync_runs MODIFY idempotency_key VARCHAR(1) NOT NULL`); err != nil {
		t.Fatal(err)
	}
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive sync runs table") {
		t.Fatalf("idempotency_key varchar(1) residual table unexpectedly passed: %v", err)
	}
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
}

func assertArchiveSyncRunsResidualGuardPasses(t *testing.T, db *sql.DB) {
	t.Helper()
	dumpArchiveSyncRunsGuardMetadata(t, db)
	var invalid int
	err := db.QueryRow(`
		SELECT CASE WHEN
			(SELECT COUNT(DISTINCT column_name)
			 FROM information_schema.columns
			 WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
			   AND column_name IN ('id','tenant_id','corp_id','source_kind','source_id','namespace','idempotency_key','status','cursor_sequence','cursor_token','fetched_count','processed_count','skipped_count','failed_count','error_code','attempt','lease_token','started_at','finished_at','lease_expires_at','heartbeat_at','created_at','updated_at')) <> 23
			OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
			            FROM information_schema.statistics
			            WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
			              AND index_name='uk_archive_sync_run_idempotency' AND non_unique=0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,source_kind,source_id,idempotency_key'
			OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
			            FROM information_schema.statistics
			            WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
			              AND index_name='uk_archive_sync_run_scope_id' AND non_unique=0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,id'
			OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
			            FROM information_schema.statistics
			            WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
			              AND index_name='uk_archive_sync_run_identity' AND non_unique=0 AND sub_part IS NULL), '') <> 'tenant_id,corp_id,id,source_kind,source_id,namespace'
			OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',')
			            FROM information_schema.key_column_usage
			            WHERE constraint_schema=DATABASE() AND table_schema=DATABASE()
			              AND table_name='mochat_go_archive_sync_runs' AND constraint_name='fk_archive_sync_run_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
			OR (SELECT COUNT(*) FROM information_schema.key_column_usage
			    WHERE constraint_schema=DATABASE() AND table_schema=DATABASE()
			      AND table_name='mochat_go_archive_sync_runs' AND constraint_name='fk_archive_sync_run_corp') <> 2
			THEN 1 ELSE 0 END
	`).Scan(&invalid)
	if err != nil {
		t.Fatalf("inspect residual runs guard: %v", err)
	}
	if invalid != 0 {
		t.Fatalf("target fixture contaminated the residual archive sync runs guard")
	}
}

func dumpArchiveSyncRunsGuardMetadata(t *testing.T, db *sql.DB) {
	t.Helper()
	mismatches := archiveSyncRunsGuardMismatches(t, db)
	if len(mismatches) > 0 {
		t.Logf("archive runs guard mismatches: %s", strings.Join(mismatches, "; "))
	}
	columns, err := db.Query(`
		SELECT ordinal_position, column_name, data_type, numeric_precision, character_maximum_length,
		       datetime_precision, column_type, is_nullable, column_default, extra
		FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
		ORDER BY ordinal_position
	`)
	if err != nil {
		t.Fatalf("inspect archive sync runs columns: %v", err)
	}
	for columns.Next() {
		var ordinal int
		var name, dataType, columnType, nullable, extra string
		var numericPrecision, charLength, datetimePrecision sql.NullInt64
		var columnDefault sql.NullString
		if err := columns.Scan(&ordinal, &name, &dataType, &numericPrecision, &charLength, &datetimePrecision, &columnType, &nullable, &columnDefault, &extra); err != nil {
			columns.Close()
			t.Fatalf("scan archive sync runs column metadata: %v", err)
		}
		t.Logf("archive runs guard column ordinal=%d name=%s data_type=%s numeric_precision=%v char_length=%v datetime_precision=%v column_type=%s nullable=%s default=%v default_kind=%s extra=%s", ordinal, name, dataType, numericPrecision, charLength, datetimePrecision, columnType, nullable, columnDefault, archiveInformationSchemaDefaultKind(columnDefault), extra)
	}
	if err := columns.Err(); err != nil {
		columns.Close()
		t.Fatalf("read archive sync runs columns: %v", err)
	}
	columns.Close()

	indexes, err := db.Query(`
		SELECT index_name, non_unique, seq_in_index, column_name, sub_part
		FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
		ORDER BY index_name, seq_in_index
	`)
	if err != nil {
		t.Fatalf("inspect archive sync runs indexes: %v", err)
	}
	for indexes.Next() {
		var indexName, columnName string
		var nonUnique, sequence int
		var subPart sql.NullInt64
		if err := indexes.Scan(&indexName, &nonUnique, &sequence, &columnName, &subPart); err != nil {
			indexes.Close()
			t.Fatalf("scan archive sync runs index metadata: %v", err)
		}
		t.Logf("archive runs guard index name=%s non_unique=%d seq=%d column=%s sub_part=%v", indexName, nonUnique, sequence, columnName, subPart)
	}
	if err := indexes.Err(); err != nil {
		indexes.Close()
		t.Fatalf("read archive sync runs indexes: %v", err)
	}
	indexes.Close()

	foreignKeys, err := db.Query(`
		SELECT constraint_name, ordinal_position, column_name, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE constraint_schema=DATABASE() AND table_schema=DATABASE()
		  AND table_name='mochat_go_archive_sync_runs'
		  AND referenced_table_name IS NOT NULL
		ORDER BY constraint_name, ordinal_position
	`)
	if err != nil {
		t.Fatalf("inspect archive sync runs foreign keys: %v", err)
	}
	for foreignKeys.Next() {
		var constraintName, columnName, referencedTable, referencedColumn string
		var ordinal int
		if err := foreignKeys.Scan(&constraintName, &ordinal, &columnName, &referencedTable, &referencedColumn); err != nil {
			foreignKeys.Close()
			t.Fatalf("scan archive sync runs foreign-key metadata: %v", err)
		}
		t.Logf("archive runs guard fk name=%s ordinal=%d column=%s references=%s.%s", constraintName, ordinal, columnName, referencedTable, referencedColumn)
	}
	if err := foreignKeys.Err(); err != nil {
		foreignKeys.Close()
		t.Fatalf("read archive sync runs foreign keys: %v", err)
	}
	foreignKeys.Close()
}

type archiveRunsColumnMetadata struct {
	dataType, columnType, nullable, extra                string
	numericPrecision, characterLength, datetimePrecision sql.NullInt64
	columnDefault                                        sql.NullString
}

type archiveRunsIndexMetadata struct {
	nonUnique, sequence int
	columnName          string
	subPart             sql.NullInt64
}

type archiveRunsForeignKeyMetadata struct {
	ordinal                                   int
	column, referencedTable, referencedColumn string
}

func archiveSyncRunsGuardMismatches(t *testing.T, db *sql.DB) []string {
	t.Helper()
	mismatches := make([]string, 0)
	columns, err := db.Query(`
		SELECT column_name, data_type, numeric_precision, character_maximum_length, datetime_precision,
		       column_type, is_nullable, column_default, extra
		FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
	`)
	if err != nil {
		t.Fatalf("query complete archive sync runs column guard metadata: %v", err)
	}
	columnMetadata := map[string]archiveRunsColumnMetadata{}
	for columns.Next() {
		var name string
		var metadata archiveRunsColumnMetadata
		if err := columns.Scan(&name, &metadata.dataType, &metadata.numericPrecision, &metadata.characterLength, &metadata.datetimePrecision, &metadata.columnType, &metadata.nullable, &metadata.columnDefault, &metadata.extra); err != nil {
			columns.Close()
			t.Fatalf("scan complete archive sync runs column guard metadata: %v", err)
		}
		columnMetadata[name] = metadata
	}
	if err := columns.Err(); err != nil {
		columns.Close()
		t.Fatalf("read complete archive sync runs column guard metadata: %v", err)
	}
	columns.Close()
	wantColumns := []string{"id", "tenant_id", "corp_id", "source_kind", "source_id", "namespace", "idempotency_key", "status", "cursor_sequence", "cursor_token", "fetched_count", "processed_count", "skipped_count", "failed_count", "error_code", "attempt", "lease_token", "started_at", "finished_at", "lease_expires_at", "heartbeat_at", "created_at", "updated_at"}
	if len(columnMetadata) != len(wantColumns) {
		mismatches = append(mismatches, fmt.Sprintf("column_count=%d want=%d", len(columnMetadata), len(wantColumns)))
	}
	for _, name := range wantColumns {
		metadata, ok := columnMetadata[name]
		if !ok {
			mismatches = append(mismatches, "missing column "+name)
			continue
		}
		if mismatch := validateArchiveRunsColumn(name, metadata); mismatch != "" {
			mismatches = append(mismatches, mismatch)
		}
	}

	indexes, err := db.Query(`
		SELECT index_name, non_unique, seq_in_index, column_name, sub_part
		FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name='mochat_go_archive_sync_runs'
		ORDER BY index_name, seq_in_index
	`)
	if err != nil {
		t.Fatalf("query complete archive sync runs index guard metadata: %v", err)
	}
	indexMetadata := map[string][]archiveRunsIndexMetadata{}
	for indexes.Next() {
		var indexName string
		var metadata archiveRunsIndexMetadata
		if err := indexes.Scan(&indexName, &metadata.nonUnique, &metadata.sequence, &metadata.columnName, &metadata.subPart); err != nil {
			indexes.Close()
			t.Fatalf("scan complete archive sync runs index guard metadata: %v", err)
		}
		indexMetadata[indexName] = append(indexMetadata[indexName], metadata)
	}
	if err := indexes.Err(); err != nil {
		indexes.Close()
		t.Fatalf("read complete archive sync runs index guard metadata: %v", err)
	}
	indexes.Close()
	wantIndexes := map[string]struct {
		nonUnique int
		columns   []string
	}{
		"PRIMARY":                           {nonUnique: 0, columns: []string{"id"}},
		"uk_archive_sync_run_idempotency":   {nonUnique: 0, columns: []string{"tenant_id", "corp_id", "source_kind", "source_id", "idempotency_key"}},
		"uk_archive_sync_run_scope_id":      {nonUnique: 0, columns: []string{"tenant_id", "corp_id", "id"}},
		"uk_archive_sync_run_identity":      {nonUnique: 0, columns: []string{"tenant_id", "corp_id", "id", "source_kind", "source_id", "namespace"}},
		"idx_archive_sync_run_scope_status": {nonUnique: 1, columns: []string{"tenant_id", "corp_id", "status", "updated_at"}},
		"idx_archive_sync_run_source":       {nonUnique: 1, columns: []string{"tenant_id", "corp_id", "source_kind", "source_id", "updated_at"}},
	}
	for name, want := range wantIndexes {
		got, ok := indexMetadata[name]
		if !ok {
			mismatches = append(mismatches, "missing index "+name)
			continue
		}
		if len(got) != len(want.columns) {
			mismatches = append(mismatches, fmt.Sprintf("index %s entries=%d want=%d", name, len(got), len(want.columns)))
			continue
		}
		for index, expectedColumn := range want.columns {
			row := got[index]
			if row.nonUnique != want.nonUnique || row.sequence != index+1 || row.columnName != expectedColumn || row.subPart.Valid {
				mismatches = append(mismatches, fmt.Sprintf("index %s[%d]=non_unique:%d seq:%d column:%s sub_part:%v want non_unique:%d seq:%d column:%s sub_part:NULL", name, index+1, row.nonUnique, row.sequence, row.columnName, row.subPart, want.nonUnique, index+1, expectedColumn))
			}
		}
	}

	foreignKeys, err := db.Query(`
		SELECT constraint_name, ordinal_position, column_name, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE constraint_schema=DATABASE() AND table_schema=DATABASE()
		  AND table_name='mochat_go_archive_sync_runs' AND constraint_name='fk_archive_sync_run_corp'
		ORDER BY ordinal_position
	`)
	if err != nil {
		t.Fatalf("query complete archive sync runs FK guard metadata: %v", err)
	}
	foreignKeyMetadata := make([]archiveRunsForeignKeyMetadata, 0, 2)
	for foreignKeys.Next() {
		var constraintName string
		var metadata archiveRunsForeignKeyMetadata
		if err := foreignKeys.Scan(&constraintName, &metadata.ordinal, &metadata.column, &metadata.referencedTable, &metadata.referencedColumn); err != nil {
			foreignKeys.Close()
			t.Fatalf("scan complete archive sync runs FK guard metadata: %v", err)
		}
		foreignKeyMetadata = append(foreignKeyMetadata, metadata)
	}
	if err := foreignKeys.Err(); err != nil {
		foreignKeys.Close()
		t.Fatalf("read complete archive sync runs FK guard metadata: %v", err)
	}
	foreignKeys.Close()
	wantForeignKeys := []archiveRunsForeignKeyMetadata{
		{ordinal: 1, column: "tenant_id", referencedTable: "mc_corp", referencedColumn: "tenant_id"},
		{ordinal: 2, column: "corp_id", referencedTable: "mc_corp", referencedColumn: "id"},
	}
	if len(foreignKeyMetadata) != len(wantForeignKeys) {
		mismatches = append(mismatches, fmt.Sprintf("fk entries=%d want=%d", len(foreignKeyMetadata), len(wantForeignKeys)))
	} else {
		for index, want := range wantForeignKeys {
			got := foreignKeyMetadata[index]
			if got != want {
				mismatches = append(mismatches, fmt.Sprintf("fk[%d]=%#v want=%#v", index+1, got, want))
			}
		}
	}
	return mismatches
}

func validateArchiveRunsColumn(name string, metadata archiveRunsColumnMetadata) string {
	dataType := strings.ToLower(metadata.dataType)
	columnType := strings.ToLower(metadata.columnType)
	normalizedExtra := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(metadata.extra), " ", ""), "default_generated", "")
	defaultKind := archiveInformationSchemaDefaultKind(metadata.columnDefault)
	defaultValue := strings.ToLower(metadata.columnDefault.String)
	if !metadata.columnDefault.Valid {
		defaultValue = "<null>"
	}
	valid := func(condition bool, want string) string {
		if condition {
			return ""
		}
		return fmt.Sprintf("column %s got data_type=%s numeric_precision=%v char_length=%v datetime_precision=%v column_type=%s nullable=%s default=%s extra=%s want %s", name, metadata.dataType, metadata.numericPrecision, metadata.characterLength, metadata.datetimePrecision, metadata.columnType, metadata.nullable, defaultValue, metadata.extra, want)
	}
	switch name {
	case "id":
		return valid(dataType == "bigint" && metadata.numericPrecision.Valid && metadata.numericPrecision.Int64 == 20 && strings.Contains(columnType, "unsigned") && metadata.nullable == "NO" && !metadata.columnDefault.Valid && strings.Contains(strings.ToLower(metadata.extra), "auto_increment"), "bigint unsigned NOT NULL AUTO_INCREMENT")
	case "tenant_id", "corp_id":
		return valid(dataType == "int" && metadata.numericPrecision.Valid && metadata.numericPrecision.Int64 == 10 && strings.Contains(columnType, "unsigned") && metadata.nullable == "NO" && !metadata.columnDefault.Valid && normalizedExtra == "", "int unsigned NOT NULL")
	case "source_kind":
		return valid(dataType == "varchar" && metadata.characterLength.Valid && metadata.characterLength.Int64 == 16 && metadata.nullable == "NO" && !metadata.columnDefault.Valid && normalizedExtra == "", "varchar(16) NOT NULL")
	case "source_id", "namespace", "idempotency_key", "lease_token":
		return valid(dataType == "varchar" && metadata.characterLength.Valid && metadata.characterLength.Int64 == 128 && metadata.nullable == "NO" && (name != "lease_token" && !metadata.columnDefault.Valid || name == "lease_token" && defaultKind == "empty-string") && normalizedExtra == "", "varchar(128) NOT NULL with migration default contract")
	case "status":
		return valid(dataType == "varchar" && metadata.characterLength.Valid && metadata.characterLength.Int64 == 16 && metadata.nullable == "NO" && !metadata.columnDefault.Valid && normalizedExtra == "", "varchar(16) NOT NULL")
	case "cursor_sequence":
		return valid(dataType == "bigint" && metadata.numericPrecision.Valid && metadata.numericPrecision.Int64 == 19 && !strings.Contains(columnType, "unsigned") && metadata.nullable == "NO" && metadata.columnDefault.Valid && metadata.columnDefault.String == "0" && normalizedExtra == "", "signed bigint NOT NULL DEFAULT 0")
	case "cursor_token":
		return valid(dataType == "varchar" && metadata.characterLength.Valid && metadata.characterLength.Int64 == 255 && metadata.nullable == "NO" && defaultKind == "empty-string" && normalizedExtra == "", "varchar(255) NOT NULL DEFAULT ''")
	case "fetched_count", "processed_count", "skipped_count", "failed_count":
		return valid(dataType == "int" && metadata.numericPrecision.Valid && metadata.numericPrecision.Int64 == 10 && strings.Contains(columnType, "unsigned") && metadata.nullable == "NO" && metadata.columnDefault.Valid && metadata.columnDefault.String == "0" && normalizedExtra == "", "int unsigned NOT NULL DEFAULT 0")
	case "error_code":
		return valid(dataType == "varchar" && metadata.characterLength.Valid && metadata.characterLength.Int64 == 96 && metadata.nullable == "NO" && defaultKind == "empty-string" && normalizedExtra == "", "varchar(96) NOT NULL DEFAULT ''")
	case "attempt":
		return valid(dataType == "int" && metadata.numericPrecision.Valid && metadata.numericPrecision.Int64 == 10 && strings.Contains(columnType, "unsigned") && metadata.nullable == "NO" && metadata.columnDefault.Valid && metadata.columnDefault.String == "1" && normalizedExtra == "", "int unsigned NOT NULL DEFAULT 1")
	case "started_at", "finished_at", "lease_expires_at", "heartbeat_at":
		return valid(dataType == "datetime" && metadata.datetimePrecision.Valid && metadata.datetimePrecision.Int64 == 6 && metadata.nullable == "YES" && defaultKind == "sql-null" && normalizedExtra == "", "datetime(6) NULL")
	case "created_at":
		return valid(dataType == "datetime" && metadata.datetimePrecision.Valid && metadata.datetimePrecision.Int64 == 6 && metadata.nullable == "NO" && defaultValue == "current_timestamp(6)" && normalizedExtra == "", "datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)")
	case "updated_at":
		return valid(dataType == "datetime" && metadata.datetimePrecision.Valid && metadata.datetimePrecision.Int64 == 6 && metadata.nullable == "NO" && defaultValue == "current_timestamp(6)" && normalizedExtra == "onupdatecurrent_timestamp(6)", "datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)")
	default:
		return "unknown column " + name
	}
}

func archiveInformationSchemaDefaultKind(value sql.NullString) string {
	if !value.Valid {
		return "sql-null"
	}
	normalized := strings.ToLower(strings.TrimSpace(value.String))
	if normalized == "null" {
		return "sql-null"
	}
	switch value.String {
	case "", "''", `""`:
		return "empty-string"
	}
	return strings.ToLower(value.String)
}

func TestArchiveSyncColumnDefaultNormalization(t *testing.T) {
	for _, test := range []struct {
		name  string
		input sql.NullString
		want  string
	}{
		{name: "sql null", input: sql.NullString{}, want: "sql-null"},
		{name: "literal null", input: sql.NullString{String: "NULL", Valid: true}, want: "sql-null"},
		{name: "empty string", input: sql.NullString{String: "", Valid: true}, want: "empty-string"},
		{name: "quoted empty string", input: sql.NullString{String: "''", Valid: true}, want: "empty-string"},
		{name: "quoted double empty string", input: sql.NullString{String: `""`, Valid: true}, want: "empty-string"},
		{name: "single quote character", input: sql.NullString{String: "'", Valid: true}, want: "'"},
		{name: "three single quotes", input: sql.NullString{String: "'''", Valid: true}, want: "'''"},
		{name: "mixed quotes", input: sql.NullString{String: `"'`, Valid: true}, want: `"'`},
		{name: "padded empty string", input: sql.NullString{String: " '' ", Valid: true}, want: " '' "},
		{name: "nonempty default", input: sql.NullString{String: "'bad'", Valid: true}, want: "'bad'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := archiveInformationSchemaDefaultKind(test.input); got != test.want {
				t.Fatalf("archiveInformationSchemaDefaultKind(%#v)=%q want %q", test.input, got, test.want)
			}
		})
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
		run_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
		UNIQUE KEY uk_archive_message_source_scope_msg (tenant_id, corp_id, msgid),
		KEY idx_archive_message_source_filter (tenant_id, corp_id, source_kind, source_id, created_at),
		KEY idx_archive_message_source_run (run_id),
		CONSTRAINT fk_archive_message_source_run FOREIGN KEY (run_id) REFERENCES mochat_go_archive_sync_runs(id)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	assertArchiveSyncRunsResidualGuardPasses(t, db)
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive message sources table") {
		t.Fatalf("wrong composite source foreign key unexpectedly passed migration guard: err=%v", err)
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
		run_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
		KEY uk_archive_message_source_scope_msg (tenant_id, corp_id, msgid),
		KEY idx_archive_message_source_filter (tenant_id, corp_id, source_kind, source_id, created_at),
		KEY idx_archive_message_source_run (run_id),
		CONSTRAINT fk_archive_message_source_run FOREIGN KEY (tenant_id,corp_id,run_id,source_kind,source_id,namespace)
			REFERENCES mochat_go_archive_sync_runs (tenant_id,corp_id,id,source_kind,source_id,namespace)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	assertArchiveSyncRunsResidualGuardPasses(t, db)
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive message sources table") {
		t.Fatalf("non-unique residual scope index unexpectedly passed migration guard: err=%v", err)
	}
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
}

func TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex(t *testing.T) {
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
		run_id BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
		UNIQUE KEY uk_archive_message_source_scope_msg (tenant_id, corp_id, msgid(10)),
		KEY idx_archive_message_source_filter (tenant_id, corp_id, source_kind, source_id, created_at),
		KEY idx_archive_message_source_run (run_id),
		CONSTRAINT fk_archive_message_source_run FOREIGN KEY (tenant_id,corp_id,run_id,source_kind,source_id,namespace)
			REFERENCES mochat_go_archive_sync_runs (tenant_id,corp_id,id,source_kind,source_id,namespace)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	assertArchiveSyncRunsResidualGuardPasses(t, db)
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive message sources table") {
		t.Fatalf("prefixed residual scope index unexpectedly passed migration guard: %v", err)
	}
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.down.sql")
}

func TestArchiveSyncMigrationRejectsWrongAuditScopeIndex(t *testing.T) {
	db := newDashboardAdminProvisioningDB(t)
	createArchiveSyncCorpFixture(t, db)
	executeArchiveMigrationFile(t, db, "0138_archive_source_sync.up.sql")
	if _, err := db.Exec("DROP TABLE mochat_go_archive_sync_audits"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_sync_audits (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		run_id BIGINT UNSIGNED NOT NULL, tenant_id INT UNSIGNED NOT NULL, corp_id INT UNSIGNED NOT NULL,
		source_kind VARCHAR(16) NOT NULL, source_id VARCHAR(128) NOT NULL, namespace VARCHAR(128) NOT NULL,
		action VARCHAR(16) NOT NULL, status VARCHAR(16) NOT NULL, error_code VARCHAR(96) NOT NULL DEFAULT '',
		cursor_sequence BIGINT NOT NULL DEFAULT 0, fetched_count INT UNSIGNED NOT NULL DEFAULT 0,
		processed_count INT UNSIGNED NOT NULL DEFAULT 0, skipped_count INT UNSIGNED NOT NULL DEFAULT 0,
		failed_count INT UNSIGNED NOT NULL DEFAULT 0, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
		PRIMARY KEY (id),
		KEY idx_archive_sync_audit_scope (tenant_id, corp_id),
		KEY idx_archive_sync_audit_run (run_id, created_at),
		CONSTRAINT fk_archive_sync_audit_run FOREIGN KEY (tenant_id,corp_id,run_id,source_kind,source_id,namespace)
			REFERENCES mochat_go_archive_sync_runs (tenant_id,corp_id,id,source_kind,source_id,namespace)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	assertArchiveSyncRunsResidualGuardPasses(t, db)
	err := executeArchiveMigrationFileErr(db, "0138_archive_source_sync.up.sql")
	if err == nil || !strings.Contains(err.Error(), "0138 incompatible archive sync audits table") {
		t.Fatalf("wrong audit scope index unexpectedly passed migration guard: %v", err)
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
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveMessageFixture(t, db)

	ctx := context.Background()
	store := NewMySQLStore(db)
	run, err := store.EnqueueArchiveSync(ctx, archiveprovider.SyncRun{
		Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated,
		SourceID: "simulation:atomic", Namespace: "MOCHAT-SIM:atomic", IdempotencyKey: "atomic-1",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	run, err = store.MarkArchiveSyncRunning(ctx, run.ID, time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	message := archiveprovider.Message{
		Source: providers.SourceSimulated, SourceID: "simulation:atomic", Namespace: "MOCHAT-SIM:atomic",
		MsgID: "MOCHAT-SIM:atomic:001", Seq: 1, From: "employee-atomic", ToList: []string{"contact-atomic"},
		MsgType: "text", ContentRaw: `{"content":"atomic"}`, ContentText: "atomic",
	}
	if _, err := store.UpsertArchiveMessage(ctx, run.ID, run.Attempt, run.LeaseToken, archiveprovider.Scope{TenantID: 12, CorpID: 27}, message); err == nil {
		t.Fatal("wrong-tenant archive upsert unexpectedly succeeded")
	}
	if _, err := store.UpsertArchiveMessage(ctx, run.ID, run.Attempt, run.LeaseToken, archiveprovider.Scope{TenantID: 11, CorpID: 27}, archiveprovider.Message{
		Source: providers.SourceExternal, SourceID: "wecom:27", Namespace: "wecom", MsgID: message.MsgID, Seq: message.Seq,
	}); err == nil {
		t.Fatal("wrong-source archive upsert unexpectedly succeeded")
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 0, 0)

	if _, err := db.Exec(`CREATE TRIGGER archive_source_atomic_fault BEFORE INSERT ON mochat_go_archive_message_sources
		FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'archive_source_atomic_fault'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertArchiveMessage(ctx, run.ID, run.Attempt, run.LeaseToken, archiveprovider.Scope{TenantID: 11, CorpID: 27}, message); err == nil {
		t.Fatal("source fault unexpectedly allowed archive upsert")
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 0, 0)
	if _, err := db.Exec("DROP TRIGGER archive_source_atomic_fault"); err != nil {
		t.Fatal(err)
	}
	result, err := store.UpsertArchiveMessage(ctx, run.ID, run.Attempt, run.LeaseToken, archiveprovider.Scope{TenantID: 11, CorpID: 27}, message)
	if err != nil || !result.Inserted {
		t.Fatalf("successful atomic upsert result=%#v err=%v", result, err)
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 1, 1)
}

func TestArchiveSyncLeaseFenceRejectsStaleWorkerMutations(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveMessageFixture(t, db)
	store := NewMySQLStore(db)
	ctx := context.Background()
	template := archiveprovider.SyncRun{Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated, SourceID: "simulation:fence", Namespace: "MOCHAT-SIM:fence", IdempotencyKey: "fence-1"}
	run, err := store.EnqueueArchiveSync(ctx, template, false)
	if err != nil {
		t.Fatal(err)
	}
	old, err := store.MarkArchiveSyncRunning(ctx, run.ID, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_archive_sync_runs SET lease_expires_at=? WHERE id=?`, time.Now().Add(-time.Minute), runIDInt(run.ID)); err != nil {
		t.Fatal(err)
	}
	taken, err := store.EnqueueArchiveSync(ctx, template, false)
	if err != nil || taken.Attempt != 2 || taken.Status != archiveprovider.SyncStatusQueued || taken.ID != old.ID {
		t.Fatalf("takeover=%#v err=%v", taken, err)
	}
	newRun, err := store.MarkArchiveSyncRunning(ctx, taken.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if newRun.LeaseToken == old.LeaseToken || newRun.Attempt != old.Attempt+1 {
		t.Fatalf("old=%#v new=%#v", old, newRun)
	}
	message := archiveprovider.Message{Source: providers.SourceSimulated, SourceID: template.SourceID, Namespace: template.Namespace, MsgID: "MOCHAT-SIM:fence:001", Seq: 1, From: "employee-atomic", ToList: []string{"contact-atomic"}, MsgType: "text", ContentRaw: `{"content":"fence"}`, ContentText: "fence"}
	if _, err := store.UpsertArchiveMessage(ctx, old.ID, old.Attempt, old.LeaseToken, template.Scope, message); err == nil {
		t.Fatal("stale worker upsert unexpectedly succeeded")
	}
	if err := store.HeartbeatArchiveSync(ctx, old.ID, old.Attempt, old.LeaseToken, time.Now()); err == nil {
		t.Fatal("stale worker heartbeat unexpectedly succeeded")
	}
	if err := store.SaveArchiveSyncCursor(ctx, old.ID, old.Attempt, old.LeaseToken, archiveprovider.Cursor{Sequence: 1}, time.Now()); err == nil {
		t.Fatal("stale worker cursor unexpectedly succeeded")
	}
	if _, err := store.CompleteArchiveSync(ctx, old.ID, old.Attempt, old.LeaseToken, archiveprovider.SyncCounts{Processed: 1}, archiveprovider.Cursor{Sequence: 1}, time.Now()); err == nil {
		t.Fatal("stale worker complete unexpectedly succeeded")
	}
	if _, err := store.FailArchiveSync(ctx, old.ID, old.Attempt, old.LeaseToken, archiveprovider.SyncCounts{Failed: 1}, archiveprovider.Cursor{Sequence: 1}, "archive.stale", time.Now()); err == nil {
		t.Fatal("stale worker fail unexpectedly succeeded")
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 0, 0)
	if _, err := store.UpsertArchiveMessage(ctx, newRun.ID, newRun.Attempt, newRun.LeaseToken, template.Scope, message); err != nil {
		t.Fatal(err)
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 1, 1)
}

func TestArchiveSyncConcurrentDifferentRunsClaimOneMessageIdentity(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	seedCurrentArchiveMessageFixture(t, db)
	store := NewMySQLStore(db)
	ctx := context.Background()
	base := archiveprovider.SyncRun{Scope: archiveprovider.Scope{TenantID: 11, CorpID: 27}, Source: providers.SourceSimulated, SourceID: "simulation:claim", Namespace: "MOCHAT-SIM:claim"}
	first, err := store.EnqueueArchiveSync(ctx, archiveprovider.SyncRun{Scope: base.Scope, Source: base.Source, SourceID: base.SourceID, Namespace: base.Namespace, IdempotencyKey: "claim-1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.EnqueueArchiveSync(ctx, archiveprovider.SyncRun{Scope: base.Scope, Source: base.Source, SourceID: base.SourceID, Namespace: base.Namespace, IdempotencyKey: "claim-2"}, false)
	if err != nil {
		t.Fatal(err)
	}
	first, err = store.MarkArchiveSyncRunning(ctx, first.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err = store.MarkArchiveSyncRunning(ctx, second.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	message := archiveprovider.Message{Source: base.Source, SourceID: base.SourceID, Namespace: base.Namespace, MsgID: "MOCHAT-SIM:claim:001", Seq: 1, From: "employee-atomic", ToList: []string{"contact-atomic"}, MsgType: "text", ContentRaw: `{"content":"claim"}`, ContentText: "claim"}
	type result struct {
		runID    string
		inserted bool
		skipped  bool
		err      error
	}
	results := make(chan result, 2)
	go func() {
		value, callErr := store.UpsertArchiveMessage(ctx, first.ID, first.Attempt, first.LeaseToken, base.Scope, message)
		results <- result{runID: first.ID, inserted: value.Inserted, skipped: value.Skipped, err: callErr}
	}()
	go func() {
		value, callErr := store.UpsertArchiveMessage(ctx, second.ID, second.Attempt, second.LeaseToken, base.Scope, message)
		results <- result{runID: second.ID, inserted: value.Inserted, skipped: value.Skipped, err: callErr}
	}()
	seenInserted, seenSkipped, seenFailed := 0, 0, 0
	winnerRunID := ""
	for index := 0; index < 2; index++ {
		value := <-results
		if value.err != nil {
			seenFailed++
		} else if value.inserted {
			seenInserted++
			winnerRunID = value.runID
		} else if value.skipped {
			seenSkipped++
		}
	}
	if seenInserted != 1 || seenSkipped != 1 || seenFailed != 0 {
		t.Fatalf("claim results inserted=%d skipped=%d failed=%d", seenInserted, seenSkipped, seenFailed)
	}
	assertArchiveMessageWriteCounts(t, db, message.MsgID, 1, 1)
	var runID int64
	if err := db.QueryRow(`SELECT run_id FROM mochat_go_archive_message_sources WHERE tenant_id=11 AND corp_id=27 AND msgid=?`, message.MsgID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if runID != runIDInt(winnerRunID) {
		t.Fatalf("source run_id=%d was overwritten; first claim=%s", runID, winnerRunID)
	}
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

func seedCurrentArchiveMessageFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	seedCurrentArchiveSyncCorpFixture(t, db)
	for _, statement := range []string{
		`INSERT INTO mc_work_employee (id,corp_id,wx_user_id,name) VALUES (1001,27,'employee-atomic','Atomic employee')`,
		`INSERT INTO mc_work_contact (id,corp_id,wx_external_userid,name) VALUES (2001,27,'contact-atomic','Atomic contact')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func seedCurrentArchiveSyncCorpFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO mc_tenant (id,name,status) VALUES (11,'Archive sync tenant',1)`,
		`INSERT INTO mc_corp (id,tenant_id,name,chat_status) VALUES (27,11,'Archive sync corp',1)`,
		`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id,corp_id,status,version) VALUES (11,27,1,1)`,
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
	if err := archivesourcefixture.PrepareDashboardPermissionDependencies(context.Background(), db); err != nil {
		t.Fatalf("0127 dashboard permission fixture: %v", err)
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
	statements, err := migration.SplitSQLStatements(string(contents))
	if err != nil {
		return err
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, statement := range statements {
		if err := sqlscript.ExecuteStatement(context.Background(), conn, statement); err != nil {
			return err
		}
	}
	return nil
}
