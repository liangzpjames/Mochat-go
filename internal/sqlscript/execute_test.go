package sqlscript

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestExecuteStatementConvertsMySQL57PreparedSignalOnPinnedConnection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mock.ExpectExec("PREPARE guard_stmt FROM @guard_sql").
		WillReturnError(&mysqldriver.MySQLError{Number: 1295, Message: "unsupported"})
	mock.ExpectQuery("SELECT @guard_sql").
		WillReturnRows(sqlmock.NewRows([]string{"@guard_sql"}).AddRow("SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'guard didn''t pass'"))

	err = ExecuteStatement(context.Background(), conn, "PREPARE guard_stmt FROM @guard_sql")
	var mysqlErr *mysqldriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		t.Fatalf("error=%v, want MySQLError", err)
	}
	if mysqlErr.Number != 1644 || string(mysqlErr.SQLState[:]) != "45000" || mysqlErr.Message != "guard didn't pass" {
		t.Fatalf("error number=%d state=%q message=%q", mysqlErr.Number, string(mysqlErr.SQLState[:]), mysqlErr.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteStatementDoesNotRewriteUnsupportedNonSignalPrepare(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	original := &mysqldriver.MySQLError{Number: 1295, Message: "unsupported"}
	mock.ExpectExec("PREPARE guard_stmt FROM @guard_sql").WillReturnError(original)
	mock.ExpectQuery("SELECT @guard_sql").
		WillReturnRows(sqlmock.NewRows([]string{"@guard_sql"}).AddRow("DROP TABLE important_data"))

	err = ExecuteStatement(context.Background(), db, "PREPARE guard_stmt FROM @guard_sql")
	if !errors.Is(err, original) {
		t.Fatalf("error=%v, want original unsupported error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
