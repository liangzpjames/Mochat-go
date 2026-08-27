package store

import (
	"context"
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
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM mochat_go_schema_migrations").WillReturnError(&mysqlDriver.MySQLError{Number: 1146, Message: "table does not exist"})
	check, err := saasMigrationHealthCheck(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != dashboard.SaaSAdminSystemHealthStateCritical || check.Code != "schema_migration" || check.Metadata["ledgerAvailable"] != false {
		t.Fatalf("check=%+v", check)
	}
}
