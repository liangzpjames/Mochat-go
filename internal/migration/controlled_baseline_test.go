package migration

import (
	"context"
	"regexp"
	"strings"
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

func TestControlledMigrationBaselineEvidenceRequiresIdentityCutoverPostcondition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	metadata := controlledMigrationRegistry["0131_identity_realms_single_corp_cutover"]
	migration := Migration{Version: metadata.Version, Kind: MigrationControlled, Controlled: &metadata}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(migration.Controlled.LedgerName, migration.Controlled.SuccessPhase, migration.Controlled.SuccessStatus, "expected-checksum").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.columns")).
		WithArgs("mc_corp", "tenant_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if err := controlledMigrationBaselineEvidence(context.Background(), db, migration, "expected-checksum"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestControlledMigrationBaselineEvidenceRejectsNullableIdentityCutoverPostcondition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	metadata := controlledMigrationRegistry["0131_identity_realms_single_corp_cutover"]
	migration := Migration{Version: metadata.Version, Kind: MigrationControlled, Controlled: &metadata}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(migration.Controlled.LedgerName, migration.Controlled.SuccessPhase, migration.Controlled.SuccessStatus, "expected-checksum").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.columns")).
		WithArgs("mc_corp", "tenant_id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	err = controlledMigrationBaselineEvidence(context.Background(), db, migration, "expected-checksum")
	if err == nil {
		t.Fatal("nullable mc_corp.tenant_id was accepted after successful 0131 control evidence")
	}
	if !strings.Contains(err.Error(), "0131_identity_realms_single_corp_cutover") || !strings.Contains(err.Error(), "postcondition") {
		t.Fatalf("unexpected error: %v", err)
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
