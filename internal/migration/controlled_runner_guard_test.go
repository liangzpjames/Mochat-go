package migration

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRunnerApplyRejectsAppliedControlledMigrationWithoutVerifiedEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	path := filepath.Join(t.TempDir(), AIInsight0165Version+".up.sql")
	if err := os.WriteFile(path, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata := controlledMigrationRegistry[AIInsight0165Version]
	migration := Migration{
		Version:     AIInsight0165Version,
		Description: "controlled test",
		Path:        path,
		Kind:        MigrationControlled,
		Controlled:  &metadata,
	}
	_, checksum, err := migrationBodyAndChecksum(migration)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{db: db, migrations: []Migration{migration}}

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS " + VersionTable)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version, description, checksum, applied_at, execution_ms")).
		WillReturnRows(sqlmock.NewRows([]string{"version", "description", "checksum", "applied_at", "execution_ms"}).
			AddRow(migration.Version, migration.Description, checksum, time.Now(), 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(checksum, metadata.SuccessStatus).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, err = runner.Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "baseline requires exactly one verified completion") {
		t.Fatalf("runner accepted half-recorded controlled migration: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
