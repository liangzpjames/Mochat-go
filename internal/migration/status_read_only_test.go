package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStatusReadOnlyReturnsPendingWithoutCreatingMissingLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration := statusTestMigration(t)
	runner, err := NewRunner(db, []Migration{migration})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*information_schema\.tables`).
		WithArgs(VersionTable).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	items, err := runner.StatusReadOnly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != "pending" || items[0].Applied != nil {
		t.Fatalf("status = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("read-only status performed an unexpected write/query: %v", err)
	}
}

func TestStatusReadOnlyDetectsDatabaseAheadAndKeepsChecksumMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration := statusTestMigration(t)
	runner, err := NewRunner(db, []Migration{migration})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*information_schema\.tables`).
		WithArgs(VersionTable).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT version, description, checksum, applied_at, execution_ms.*mochat_go_schema_migrations`).
		WillReturnRows(sqlmock.NewRows([]string{"version", "description", "checksum", "applied_at", "execution_ms"}).
			AddRow(migration.Version, migration.Description, "wrong-checksum", time.Now(), 1).
			AddRow("9999_future_release", "future", statusTestChecksum(t, migration.Path), time.Now(), 1))

	items, err := runner.StatusReadOnly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].State != "checksum_mismatch" {
		t.Fatalf("known status = %+v", items)
	}
	if items[1].State != "database_ahead" || items[1].Migration.Version != "9999_future_release" || items[1].Applied == nil {
		t.Fatalf("database-ahead status = %+v", items[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStatusReadOnlyMarksAuditedLiveCodeLegacyVersionSuperseded(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	current := statusTestMigration(t)
	current.Version = "0153_live_code_workspace"
	current.Description = "live code workspace"
	currentChecksum := statusTestChecksum(t, current.Path)
	runner, err := NewRunner(db, []Migration{current})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*information_schema\.tables`).
		WithArgs(VersionTable).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT version, description, checksum, applied_at, execution_ms.*mochat_go_schema_migrations`).
		WillReturnRows(sqlmock.NewRows([]string{"version", "description", "checksum", "applied_at", "execution_ms"}).
			AddRow(current.Version, current.Description, currentChecksum, time.Now(), 1).
			AddRow("0150_live_code_workspace", "live code workspace", "f89678394dea6164312152f9bbb5298111150d9dea8489252789ef7ed67117ed", time.Now(), 1))

	items, err := runner.StatusReadOnly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[1].State != "superseded" || items[1].Migration.Version != "0150_live_code_workspace" {
		t.Fatalf("legacy live-code status = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func statusTestMigration(t *testing.T) Migration {
	t.Helper()
	path := t.TempDir() + "/0001_status_test.sql"
	if err := os.WriteFile(path, []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Migration{Version: "0001_status_test", Description: "status test", Path: path}
}

func statusTestChecksum(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
