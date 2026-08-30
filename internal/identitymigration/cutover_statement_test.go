package identitymigration

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestExecuteCutoverStatementsReportsMiddleStatementIndex(t *testing.T) {
	cause := errors.New("injected middle statement failure")
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT 2").WillReturnError(cause)
	err = executeCutoverStatements(context.Background(), db, []string{"SELECT 1", "SELECT 2", "SELECT 3"})
	var phaseErr *PhaseError
	if !errors.As(err, &phaseErr) {
		t.Fatalf("executeCutoverStatements() error=%v, want PhaseError", err)
	}
	if phaseErr.StatementIndex != 1 {
		t.Fatalf("statement index=%d, want 1", phaseErr.StatementIndex)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("statement failure lost cause: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
