package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"jiyi/mochat-go/internal/dashboard"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInsertWorkMessageExportTaskUsesExecAndLastInsertID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	input := dashboard.WorkMessageExportTaskInput{
		TenantID: 1, CorpID: 2, UserID: 3, EmployeeIDs: []int{1001},
		Request: dashboard.WorkMessageExportCreateRequest{
			ExportType: "employee", ObjectIDs: []int{1001}, IdempotencyKey: "export-key",
			ConversationScopes: []string{"customer_direct", "external_group"},
			StartAt:            time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
			EndAt:              time.Date(2026, 8, 21, 4, 0, 0, 0, time.UTC), FileMode: "split", Format: "zip",
		},
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_work_message_export_tasks")).
		WillReturnResult(sqlmock.NewResult(42, 1))

	id, err := store.insertWorkMessageExportTask(context.Background(), input, 3, time.Date(2026, 8, 28, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Fatalf("id=%d, want 42", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
