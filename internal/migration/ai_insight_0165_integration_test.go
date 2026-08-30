package migration_test

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

	"jiyi/mochat-go/internal/integrationtestdb"
	. "jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
)

const aiInsight0165PublishedCRLFChecksum = "4575a0d89e59cf0b87059c0d60575be3e5cc566ee7338cc6fb6f616e8431520f"
const aiInsight0165PublishedLFChecksum = "eb6c469a0b46bccb44da6f51d10d517b600f6aa4e6bbfa10f9dbcfd7e1163d5b"

func isPublishedAIInsight0165Checksum(value string) bool {
	return value == aiInsight0165PublishedCRLFChecksum || value == aiInsight0165PublishedLFChecksum
}

func TestAIInsight0165IsRegisteredAsControlledWithoutChangingPublishedSQL(t *testing.T) {
	kind, metadata := MigrationMetadata(AIInsight0165Version)
	if kind != MigrationControlled || metadata == nil {
		t.Fatalf("0165 metadata = %q %#v, want controlled", kind, metadata)
	}
	if metadata.RequiredCLI != "preflight_0165_ai_daily_insight_unification" {
		t.Fatalf("0165 required CLI = %q", metadata.RequiredCLI)
	}

	root := filepath.Join("..", "..")
	inventory, err := DefaultInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range inventory {
		if item.Version != AIInsight0165Version {
			continue
		}
		if !isPublishedAIInsight0165Checksum(item.Checksum) {
			t.Fatalf("0165 checksum = %s, want a published LF/CRLF checksum", item.Checksum)
		}
		for _, migration := range DefaultMigrations(root) {
			if migration.Version != AIInsight0165Version {
				continue
			}
			other := aiInsight0165PublishedLFChecksum
			if item.Checksum == other {
				other = aiInsight0165PublishedCRLFChecksum
			}
			foundAlias := false
			for _, alias := range migration.ChecksumAliases {
				foundAlias = foundAlias || alias == other
			}
			if !foundAlias {
				t.Fatalf("0165 registered aliases = %v, missing published line-ending checksum %s", migration.ChecksumAliases, other)
			}
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
	compatible := AIInsight0165BodyForServerForTest(original, "5.7.44")
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
	if got := AIInsight0165BodyForServerForTest(original, "10.6.22-MariaDB"); got != original {
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
	if !isPublishedAIInsight0165Checksum(inventory.MigrationChecksum) || inventory.Applied {
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
	if ledgerChecksum != result.MigrationChecksum {
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
			db := openAIInsight0165IntegrationDB(t)
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("source snapshot changes after backup", func(t *testing.T) {
		db := openAIInsight0165IntegrationDB(t)
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
		db := openAIInsight0165IntegrationDB(t)
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("wrong source schema", func(t *testing.T) {
		db := openAIInsight0165IntegrationDB(t)
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
	})

	t.Run("missing pre-0165 unique index", func(t *testing.T) {
		db := openAIInsight0165IntegrationDB(t)
		resetAIInsight0165Schema(t, ctx, db)
		if _, err := db.ExecContext(ctx, `ALTER TABLE mochat_go_ai_conversation_insights DROP INDEX uq_ai_conversation_source`); err != nil {
			t.Fatal(err)
		}
		controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Inventory(ctx); !errors.Is(err, ErrAIInsight0165WrongSchema) {
			t.Fatalf("missing pre-0165 unique index error = %v", err)
		}
	})

	t.Run("already applied through controlled controller", func(t *testing.T) {
		db := openAIInsight0165IntegrationDB(t)
		resetAIInsight0165Schema(t, ctx, db)
		controller, err := NewAIInsight0165Controller(db, filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Backup(ctx, "already-applied"); err != nil {
			t.Fatal(err)
		}
		preflight, err := controller.Preflight(ctx, "already-applied")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
			RequestID: "already-applied", ApprovalToken: preflight.ApprovalToken,
			DestructiveApproval: preflight.DestructiveApproval, TrafficStopped: true,
		}); err != nil {
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
	})
}

func TestAIInsight0165AdoptsHistoricalAppliedEnvironmentWithoutClaimingVerified(t *testing.T) {
	db := openAIInsight0165IntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	resetAIInsight0165Schema(t, ctx, db)

	root := filepath.Join("..", "..")
	controller, err := NewAIInsight0165Controller(db, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Backup(ctx, "historical-source"); err != nil {
		t.Fatal(err)
	}
	preflight, err := controller.Preflight(ctx, "historical-source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Apply(ctx, AIInsight0165ApplyRequest{
		RequestID: "historical-source", ApprovalToken: preflight.ApprovalToken,
		DestructiveApproval: preflight.DestructiveApproval, TrafficStopped: true,
	}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{
		"mochat_go_controlled_migration_0165",
		"mochat_go_backup_0165_ai_conversation_insights",
		"mochat_go_backup_0165_ai_analysis",
	} {
		if _, err := db.ExecContext(ctx, `DROP TABLE `+table); err != nil {
			t.Fatal(err)
		}
	}
	latestEvidence, err := testharness.NewControlledEvidence("historical-adoption-latest")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.ApplyLatest(ctx, db, root, latestEvidence); err == nil || !strings.Contains(err.Error(), "0165") {
		t.Fatalf("historical environment was not blocked before adoption: %v", err)
	}
	backupSHA := strings.Repeat("a", 64)
	if _, err := controller.AdoptExisting(ctx, AIInsight0165AdoptExistingRequest{
		RequestID: "historical-adoption", ExternalBackupSHA256: backupSHA,
	}); !errors.Is(err, ErrAIInsight0165TrafficNotStopped) {
		t.Fatalf("missing traffic confirmation error = %v", err)
	}
	if _, err := controller.AdoptExisting(ctx, AIInsight0165AdoptExistingRequest{
		RequestID: "historical-adoption", ExternalBackupSHA256: "not-a-sha", TrafficStopped: true,
	}); err == nil || !strings.Contains(err.Error(), "valid external backup SHA-256") {
		t.Fatalf("invalid backup hash error = %v", err)
	}
	result, err := controller.AdoptExisting(ctx, AIInsight0165AdoptExistingRequest{
		RequestID: "historical-adoption", ExternalBackupSHA256: backupSHA, TrafficStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Adopted || result.Verified || result.AppliedMigrationChecksum != result.MigrationChecksum || result.ExternalBackupSHA256 != backupSHA || result.RecoveryBoundary == "" {
		t.Fatalf("adoption result = %+v", result)
	}
	repeated, err := controller.AdoptExisting(ctx, AIInsight0165AdoptExistingRequest{
		RequestID: "historical-adoption", ExternalBackupSHA256: backupSHA, TrafficStopped: true,
	})
	if err != nil || repeated != result {
		t.Fatalf("idempotent adoption result=%+v error=%v", repeated, err)
	}
	if _, err := controller.AdoptExisting(ctx, AIInsight0165AdoptExistingRequest{
		RequestID: "conflicting-adoption", ExternalBackupSHA256: strings.Repeat("b", 64), TrafficStopped: true,
	}); !errors.Is(err, ErrAIInsight0165AdoptionConflict) {
		t.Fatalf("conflicting adoption error = %v", err)
	}
	if err := testharness.ApplyLatest(ctx, db, root, latestEvidence); err != nil {
		t.Fatal(err)
	}
	var latestRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version = '0176_wework_callback_side_effect_reconciliation'`).Scan(&latestRows); err != nil {
		t.Fatal(err)
	}
	if latestRows != 1 {
		t.Fatalf("latest migration rows after adoption = %d", latestRows)
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
	database := integrationtestdb.NewIsolated(t, dsn)
	evidence, err := testharness.NewControlledEvidence("ai-insight-0165")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.ApplyThrough(context.Background(), database.DB, filepath.Join("..", ".."), "0164_saas_tenant_ai_provider", evidence); err != nil {
		t.Fatalf("apply production migration registry through 0164: %v", err)
	}
	return database.DB
}

const aiInsight0165InsertSQL = `INSERT INTO mochat_go_ai_conversation_insights
	(id, tenant_id, corp_id, analysis_type, rule_version_id, conversation_key, source_fingerprint, result_json, generated_at)
	VALUES (?, 1, 1, 'smart', 7, 'conversation-a', ?, '{}', ?)`

func resetAIInsight0165Schema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
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
