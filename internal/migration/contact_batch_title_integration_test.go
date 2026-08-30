package migration_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationtestdb"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
)

func TestContactBatchTitle0175UpDownReapplyLifecycle(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for 0175 lifecycle integration")
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	root := filepath.Join("..", "..")
	evidence, err := testharness.NewControlledEvidence("contact-0175")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.ApplyThrough(context.Background(), database.DB, root, "0174_wework_callback_side_effects", evidence); err != nil {
		t.Fatal(err)
	}
	migrations := migrationsThrough(t, root, "0175_contact_batch_title")
	runner, err := migration.NewRunner(database.DB, migrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContactBatchTitleColumn(t, database.DB, true)
	assertContact0175Ledger(t, database.DB, root, true)
	rolledBack, err := runner.RollbackLast(context.Background())
	if err != nil || rolledBack != "0175_contact_batch_title" {
		t.Fatalf("rollback=%q err=%v", rolledBack, err)
	}
	assertContactBatchTitleColumn(t, database.DB, false)
	assertContact0175Ledger(t, database.DB, root, false)
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContactBatchTitleColumn(t, database.DB, true)
	assertContact0175Ledger(t, database.DB, root, true)
}

func migrationsThrough(t *testing.T, root, version string) []migration.Migration {
	t.Helper()
	all := migration.DefaultMigrations(root)
	for index, item := range all {
		if item.Version == version {
			return all[:index+1]
		}
	}
	t.Fatalf("migration %s not found", version)
	return nil
}

func assertContactBatchTitleColumn(t *testing.T, db interface{ QueryRow(string, ...any) *sql.Row }, present bool) {
	t.Helper()
	var dataType, nullable string
	var length int
	var collation, defaultValue sql.NullString
	err := db.QueryRow(`SELECT data_type,character_maximum_length,collation_name,is_nullable,column_default FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_contact_message_batch_send' AND column_name='batch_title'`).Scan(&dataType, &length, &collation, &nullable, &defaultValue)
	if !present {
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("0175 batch_title remained after down: type=%q length=%d collation=%q nullable=%q default=%q err=%v", dataType, length, collation.String, nullable, defaultValue.String, err)
		}
		return
	}
	if err != nil || dataType != "varchar" || length != 100 || collation.String != "utf8mb4_unicode_ci" || nullable != "NO" || !defaultValue.Valid || (defaultValue.String != "" && defaultValue.String != "''") {
		t.Fatalf("0175 batch_title metadata type=%q length=%d collation=%q nullable=%q default=%q valid=%t err=%v", dataType, length, collation.String, nullable, defaultValue.String, defaultValue.Valid, err)
	}
}

func assertContact0175Ledger(t *testing.T, db interface{ QueryRow(string, ...any) *sql.Row }, root string, present bool) {
	t.Helper()
	inventory, err := migration.DefaultInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	var checksum string
	for _, item := range inventory {
		if item.Version == "0175_contact_batch_title" {
			checksum = item.Checksum
		}
	}
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0175_contact_batch_title' AND checksum=?`, checksum).Scan(&count)
	if err != nil || (count == 1) != present {
		t.Fatalf("0175 ledger count=%d present=%t err=%v", count, present, err)
	}
}
