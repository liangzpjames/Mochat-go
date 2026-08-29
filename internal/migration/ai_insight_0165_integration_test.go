package migration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const aiInsight0165ExpectedChecksum = "4575a0d89e59cf0b87059c0d60575be3e5cc566ee7338cc6fb6f616e8431520f"

func TestAIInsight0165IsRegisteredAsControlledWithoutChangingPublishedSQL(t *testing.T) {
	kind, metadata := MigrationMetadata(AIInsight0165Version)
	if kind != MigrationControlled || metadata == nil {
		t.Fatalf("0165 metadata = %q %#v, want controlled", kind, metadata)
	}
	if metadata.RequiredCLI != "preflight_0165_ai_daily_insight_unification" {
		t.Fatalf("0165 required CLI = %q", metadata.RequiredCLI)
	}

	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	for _, item := range migrations {
		if item.Version != AIInsight0165Version {
			continue
		}
		_, checksum, err := migrationBodyAndChecksum(item)
		if err != nil {
			t.Fatal(err)
		}
		if checksum != aiInsight0165ExpectedChecksum {
			t.Fatalf("0165 checksum = %s, want immutable %s", checksum, aiInsight0165ExpectedChecksum)
		}
		return
	}
	t.Fatal("0165 migration missing")
}

func TestAIInsight0165MySQL57ExecutionPlanPreservesImmutableSourceAndSemantics(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "standalone", "migrations", AIInsight0165Version+".up.sql")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original := string(body)
	compatible := aiInsight0165BodyForServer(original, "5.7.44")
	for _, unsupported := range []string{
		"ADD COLUMN IF NOT EXISTS",
		"MODIFY COLUMN IF EXISTS",
		"DROP INDEX IF EXISTS",
		"ADD UNIQUE KEY IF NOT EXISTS",
		"ADD KEY IF NOT EXISTS",
	} {
		if strings.Contains(compatible, unsupported) {
			t.Fatalf("MySQL 5.7 execution plan retained unsupported clause %q", unsupported)
		}
	}
	if !strings.Contains(compatible, "DROP TABLE IF EXISTS `mochat_go_ai_analysis`") {
		t.Fatal("MySQL 5.7 execution plan lost idempotent optional legacy-table drop")
	}
	current, err := os.ReadFile(path)
	if err != nil || string(current) != original {
		t.Fatal("MySQL 5.7 compatibility rewrote the immutable migration source")
	}
	if got := aiInsight0165BodyForServer(original, "10.6.22-MariaDB"); got != original {
		t.Fatal("MariaDB execution plan unexpectedly changed the source SQL")
	}
}

func TestAIInsight0165ControlledLifecycleMariaDB(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	resetAIInsight0165Schema(t, ctx, db)

	controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	inventory, err := controller.Inventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.InsightRows != 2 || inventory.DuplicateRows != 1 || inventory.LegacyRows != 1 {
		t.Fatalf("inventory = %+v", inventory)
	}
	if inventory.MigrationChecksum != aiInsight0165ExpectedChecksum || inventory.Applied {
		t.Fatalf("inventory checksum/applied = %+v", inventory)
	}

	if _, err := controller.Preflight(ctx, "task8-request"); !errors.Is(err, ErrAIInsight0165BackupMissing) {
		t.Fatalf("preflight without backup error = %v", err)
	}
	backup, err := controller.Backup(ctx, "task8-request")
	if err != nil {
		t.Fatal(err)
	}
	if backup.BackupInsightRows != 2 || backup.BackupLegacyRows != 1 {
		t.Fatalf("backup = %+v", backup)
	}
	preflight, err := controller.Preflight(ctx, "task8-request")
	if err != nil {
		t.Fatal(err)
	}
	if preflight.ApprovalToken == "" || preflight.DestructiveApproval != "duplicates=1,legacy=1" {
		t.Fatalf("preflight = %+v", preflight)
	}

	if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
		RequestID:           "task8-request",
		ApprovalToken:       "wrong-token",
		DestructiveApproval: preflight.DestructiveApproval,
		TrafficStopped:      true,
	}); !errors.Is(err, ErrAIInsight0165ApprovalMismatch) {
		t.Fatalf("wrong approval token error = %v", err)
	}
	if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
		RequestID:           "task8-request",
		ApprovalToken:       preflight.ApprovalToken,
		DestructiveApproval: "duplicates=0,legacy=0",
		TrafficStopped:      true,
	}); !errors.Is(err, ErrAIInsight0165ApprovalMismatch) {
		t.Fatalf("wrong destructive approval error = %v", err)
	}
	if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
		RequestID:           "task8-request",
		ApprovalToken:       preflight.ApprovalToken,
		DestructiveApproval: preflight.DestructiveApproval,
	}); !errors.Is(err, ErrAIInsight0165TrafficNotStopped) {
		t.Fatalf("missing traffic confirmation error = %v", err)
	}

	result, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
		RequestID:           "task8-request",
		ApprovalToken:       preflight.ApprovalToken,
		DestructiveApproval: preflight.DestructiveApproval,
		TrafficStopped:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RetainedInsightRows != 1 || result.RemovedDuplicateRows != 1 || result.RemovedLegacyRows != 1 {
		t.Fatalf("apply result = %+v", result)
	}
	verified, err := controller.Verify(ctx, "task8-request")
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Applied || !verified.Verified || verified.BackupInsightRows != 2 || verified.BackupLegacyRows != 1 {
		t.Fatalf("verify = %+v", verified)
	}

	var ledgerChecksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM mochat_go_schema_migrations WHERE version = ?`, AIInsight0165Version).Scan(&ledgerChecksum); err != nil {
		t.Fatal(err)
	}
	if ledgerChecksum != aiInsight0165ExpectedChecksum {
		t.Fatalf("ledger checksum = %s", ledgerChecksum)
	}
	if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{RequestID: "task8-request", ApprovalToken: preflight.ApprovalToken, DestructiveApproval: preflight.DestructiveApproval, TrafficStopped: true}); !errors.Is(err, ErrAIInsight0165AlreadyApplied) {
		t.Fatalf("repeat apply error = %v", err)
	}
}

func TestAIInsight0165LedgerAndVerifiedStatusCommitAtomicallyMariaDB(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	resetAIInsight0165Schema(t, ctx, db)

	controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Backup(ctx, "atomic-ledger"); err != nil {
		t.Fatal(err)
	}
	preflight, err := controller.Preflight(ctx, "atomic-ledger")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER mochat_0165_test_fail_verified BEFORE UPDATE ON mochat_go_controlled_migration_0165
		FOR EACH ROW BEGIN IF NEW.status = 'verified' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected verified failure'; END IF; END`); err != nil {
		t.Fatal(err)
	}
	_, applyErr := controller.Apply(ctx, AIInsight0165ApplyRequest{
		RequestID:           "atomic-ledger",
		ApprovalToken:       preflight.ApprovalToken,
		DestructiveApproval: preflight.DestructiveApproval,
		TrafficStopped:      true,
	})
	if applyErr == nil {
		t.Fatal("injected control verification failure was accepted")
	}
	var ledgerRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version = ?`, AIInsight0165Version).Scan(&ledgerRows); err != nil {
		t.Fatal(err)
	}
	if ledgerRows != 0 {
		t.Fatalf("schema ledger escaped failed control verification: rows=%d", ledgerRows)
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM mochat_go_controlled_migration_0165 WHERE request_id = 'atomic-ledger'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "applied_unverified" {
		t.Fatalf("control status after injected failure = %q", status)
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER mochat_0165_test_fail_verified`); err != nil {
		t.Fatal(err)
	}
	verified, err := controller.Verify(ctx, "atomic-ledger")
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Applied || !verified.Verified {
		t.Fatalf("recovered verification = %+v", verified)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version = ?`, AIInsight0165Version).Scan(&ledgerRows); err != nil {
		t.Fatal(err)
	}
	if ledgerRows != 1 {
		t.Fatalf("recovered schema ledger rows = %d", ledgerRows)
	}
}

func TestAIInsight0165VerifyRejectsWrongOrChangedSurvivorMariaDB(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	for _, test := range []struct {
		name    string
		request string
		mutate  string
	}{
		{name: "non-max id", request: "survivor-id", mutate: `UPDATE mochat_go_ai_conversation_insights SET id = 1 WHERE id = 2`},
		{name: "content hash drift", request: "survivor-content", mutate: `UPDATE mochat_go_ai_conversation_insights SET summary = 'tampered' WHERE id = 2`},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetAIInsight0165Schema(t, ctx, db)
			controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := controller.Backup(ctx, test.request); err != nil {
				t.Fatal(err)
			}
			preflight, err := controller.Preflight(ctx, test.request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
				RequestID:           test.request,
				ApprovalToken:       preflight.ApprovalToken,
				DestructiveApproval: preflight.DestructiveApproval,
				TrafficStopped:      true,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, test.mutate); err != nil {
				t.Fatal(err)
			}
			if _, err := controller.Verify(ctx, test.request); err == nil || !strings.Contains(err.Error(), "survivor") {
				t.Fatalf("survivor drift was accepted: %v", err)
			}
		})
	}
}

func TestAIInsight0165RejectsSnapshotAndBackupDrift(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("source snapshot changes after backup", func(t *testing.T) {
		resetAIInsight0165Schema(t, ctx, db)
		controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Backup(ctx, "source-drift"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, aiInsight0165InsertSQL, 3, "different", "2026-08-28 04:00:00.000000"); err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Preflight(ctx, "source-drift"); !errors.Is(err, ErrAIInsight0165SnapshotDrift) {
			t.Fatalf("source drift error = %v", err)
		}
	})

	t.Run("backup rows change", func(t *testing.T) {
		resetAIInsight0165Schema(t, ctx, db)
		controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Backup(ctx, "backup-drift"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM mochat_go_backup_0165_ai_conversation_insights WHERE id = 1`); err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Preflight(ctx, "backup-drift"); !errors.Is(err, ErrAIInsight0165BackupDrift) {
			t.Fatalf("backup drift error = %v", err)
		}
	})
}

func TestAIInsight0165RejectsWrongSchemaAndAlreadyAppliedEnvironment(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	resetAIInsight0165Schema(t, ctx, db)

	if _, err := db.ExecContext(ctx, `ALTER TABLE mochat_go_ai_conversation_insights DROP INDEX uq_ai_conversation_source, DROP COLUMN source_fingerprint`); err != nil {
		t.Fatal(err)
	}
	controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Inventory(ctx); !errors.Is(err, ErrAIInsight0165WrongSchema) {
		t.Fatalf("wrong schema error = %v", err)
	}

	resetAIInsight0165Schema(t, ctx, db)
	if _, err := db.ExecContext(ctx, `ALTER TABLE mochat_go_ai_conversation_insights DROP INDEX uq_ai_conversation_source`); err != nil {
		t.Fatal(err)
	}
	controller, err = NewAIInsight0165Controller(db, filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Inventory(ctx); !errors.Is(err, ErrAIInsight0165WrongSchema) {
		t.Fatalf("missing pre-0165 unique index error = %v", err)
	}

	resetAIInsight0165Schema(t, ctx, db)
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_schema_migrations (version, description, checksum, applied_at, execution_ms) VALUES (?, 'historical apply', ?, NOW(), 0)`, AIInsight0165Version, aiInsight0165ExpectedChecksum); err != nil {
		t.Fatal(err)
	}
	controller, err = NewAIInsight0165Controller(db, filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := controller.Inventory(ctx)
	if !errors.Is(err, ErrAIInsight0165AlreadyApplied) || !inventory.Applied || inventory.RecoveryBoundary == "" {
		t.Fatalf("already applied inventory=%+v error=%v", inventory, err)
	}
}

func TestAIInsight0165ConcurrentApplyCommitsExactlyOnce(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	resetAIInsight0165Schema(t, ctx, db)
	controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Backup(ctx, "concurrent"); err != nil {
		t.Fatal(err)
	}
	preflight, err := controller.Preflight(ctx, "concurrent")
	if err != nil {
		t.Fatal(err)
	}
	req := AIInsight0165ApplyRequest{RequestID: "concurrent", ApprovalToken: preflight.ApprovalToken, DestructiveApproval: preflight.DestructiveApproval, TrafficStopped: true}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, applyErr := controller.Apply(ctx, req)
			errs <- applyErr
		}()
	}
	wg.Wait()
	close(errs)
	success, rejected := 0, 0
	for applyErr := range errs {
		switch {
		case applyErr == nil:
			success++
		case errors.Is(applyErr, ErrAIInsight0165ConcurrentRun), errors.Is(applyErr, ErrAIInsight0165AlreadyApplied):
			rejected++
		default:
			t.Fatalf("unexpected concurrent apply error: %v", applyErr)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("concurrent outcomes success=%d rejected=%d", success, rejected)
	}
}

func openAIInsight0165IntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_0165_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_0165_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

const aiInsight0165InsertSQL = `INSERT INTO mochat_go_ai_conversation_insights
	(id, tenant_id, corp_id, analysis_type, rule_version_id, conversation_key, source_fingerprint, result_json, generated_at)
	VALUES (?, 1, 1, 'smart', 7, 'conversation-a', ?, '{}', ?)`

func resetAIInsight0165Schema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`DROP TRIGGER IF EXISTS mochat_0165_test_fail_verified`,
		`DROP TRIGGER IF EXISTS mochat_0165_guard_insight_insert`,
		`DROP TRIGGER IF EXISTS mochat_0165_guard_insight_update`,
		`DROP TRIGGER IF EXISTS mochat_0165_guard_insight_delete`,
		`DROP TRIGGER IF EXISTS mochat_0165_guard_legacy_insert`,
		`DROP TRIGGER IF EXISTS mochat_0165_guard_legacy_update`,
		`DROP TRIGGER IF EXISTS mochat_0165_guard_legacy_delete`,
		`DROP TABLE IF EXISTS mochat_go_controlled_migration_0165`,
		`DROP TABLE IF EXISTS mochat_go_backup_0165_ai_conversation_insights`,
		`DROP TABLE IF EXISTS mochat_go_backup_0165_ai_analysis`,
		`DROP TABLE IF EXISTS mochat_go_ai_conversation_insights`,
		`DROP TABLE IF EXISTS mochat_go_ai_analysis`,
		`DROP TABLE IF EXISTS mochat_go_schema_migrations`,
		`CREATE TABLE mochat_go_schema_migrations (version varchar(191) NOT NULL PRIMARY KEY, description varchar(255) NOT NULL, checksum char(64) NOT NULL, applied_at datetime NOT NULL, execution_ms int NOT NULL DEFAULT 0) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_ai_conversation_insights (
			id bigint unsigned NOT NULL AUTO_INCREMENT, tenant_id int unsigned NOT NULL, corp_id int unsigned NOT NULL,
			analysis_type varchar(24) NOT NULL, rule_id bigint unsigned NOT NULL DEFAULT 0, rule_version_id bigint unsigned NOT NULL DEFAULT 0,
			conversation_key varchar(191) NOT NULL, employee_id bigint unsigned NOT NULL DEFAULT 0, employee_name varchar(120) NOT NULL DEFAULT '', employee_avatar varchar(512) NOT NULL DEFAULT '',
			target_type varchar(24) NOT NULL DEFAULT '', target_id varchar(191) NOT NULL DEFAULT '', target_name varchar(191) NOT NULL DEFAULT '', target_avatar varchar(512) NOT NULL DEFAULT '',
			source_started_at datetime(6) NULL, source_ended_at datetime(6) NULL, source_message_count int unsigned NOT NULL DEFAULT 0, source_fingerprint char(64) NOT NULL,
			status varchar(16) NOT NULL DEFAULT 'pending', summary varchar(1200) NOT NULL DEFAULT '', result_json json NOT NULL, error_summary varchar(500) NOT NULL DEFAULT '',
			provider varchar(64) NOT NULL DEFAULT '', model varchar(128) NOT NULL DEFAULT '', prompt_version varchar(32) NOT NULL DEFAULT '', generated_at datetime(6) NULL,
			created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id), UNIQUE KEY uq_ai_conversation_source (tenant_id,corp_id,analysis_type,rule_version_id,conversation_key,source_fingerprint)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE mochat_go_ai_analysis (id bigint unsigned NOT NULL AUTO_INCREMENT, tenant_id int unsigned NOT NULL DEFAULT 0, corp_id int unsigned NOT NULL DEFAULT 0, page varchar(64) NOT NULL DEFAULT '', status varchar(16) NOT NULL DEFAULT 'succeeded', payload json DEFAULT NULL, error varchar(1024) NOT NULL DEFAULT '', created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id)) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("reset statement %q: %v", statement, err)
		}
	}
	for _, row := range []struct {
		id          int
		fingerprint string
		generated   string
	}{{1, "first", "2026-08-28 01:00:00.000000"}, {2, "second", "2026-08-28 02:00:00.000000"}} {
		if _, err := db.ExecContext(ctx, aiInsight0165InsertSQL, row.id, row.fingerprint, row.generated); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mochat_go_ai_analysis (tenant_id, corp_id, page, payload) VALUES (1, 1, 'legacy', '{"value":1}')`); err != nil {
		t.Fatal(err)
	}
}
