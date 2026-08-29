package migration

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestControlledMigrationBaselineEvidenceRequiresExactlyOneIdentityCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	metadata := controlledMigrationRegistry["0130_identity_realms_single_corp_backfill"]
	migration := Migration{Version: metadata.Version, Kind: MigrationControlled, Controlled: &metadata}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(migration.Controlled.LedgerName, migration.Controlled.SuccessPhase, migration.Controlled.SuccessStatus, "expected-checksum").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if err := controlledMigrationBaselineEvidence(context.Background(), db, migration, "expected-checksum"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestControlledMigrationBaselineEvidenceRejectsMissingAIInsightVerification(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	metadata := controlledMigrationRegistry[AIInsight0165Version]
	migration := Migration{Version: metadata.Version, Kind: MigrationControlled, Controlled: &metadata}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs("expected-checksum", migration.Controlled.SuccessStatus).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	if err := controlledMigrationBaselineEvidence(context.Background(), db, migration, "expected-checksum"); err == nil {
		t.Fatal("missing verified 0165 evidence was accepted")
	}
}
