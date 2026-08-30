package store

import (
	"context"
	"fmt"
	"testing"

	"jiyi/mochat-go/internal/dashboard"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestSaaSMigrationHealthCheckReportsMissingLedgerWithoutFailingPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT version, checksum FROM mochat_go_schema_migrations").WillReturnError(&mysqlDriver.MySQLError{Number: 1146, Message: "table does not exist"})
	check, err := saasMigrationHealthCheck(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != dashboard.SaaSAdminSystemHealthStateCritical || check.Code != "schema_migration" || check.Metadata["ledgerAvailable"] != false {
		t.Fatalf("check=%+v", check)
	}
}

func TestSaaSMigrationHealthCheckAcceptsAuditedSupersededLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := sqlmock.NewRows([]string{"version", "checksum"})
	for index := 1; index <= dashboard.SaaSAdminExpectedMigrationCount-2; index++ {
		rows.AddRow(fmt.Sprintf("%04d_test", index), "current")
	}
	rows.AddRow("0153_live_code_workspace", "f17df230c78b79ed0e23d77b87057a939fa8ef5d1ac97fa1db43b5aa34f7344c")
	rows.AddRow(dashboard.SaaSAdminExpectedMigrationVersion, "current")
	rows.AddRow("0150_live_code_workspace", "f89678394dea6164312152f9bbb5298111150d9dea8489252789ef7ed67117ed")
	mock.ExpectQuery("SELECT version, checksum FROM mochat_go_schema_migrations").WillReturnRows(rows)

	check, err := saasMigrationHealthCheck(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != dashboard.SaaSAdminSystemHealthStateHealthy || check.Current != dashboard.SaaSAdminExpectedMigrationCount {
		t.Fatalf("check=%+v", check)
	}
	if check.Metadata["rawMigrationCount"] != int64(dashboard.SaaSAdminExpectedMigrationCount+1) || check.Metadata["supersededMigrationCount"] != int64(1) {
		t.Fatalf("metadata=%+v", check.Metadata)
	}
}
