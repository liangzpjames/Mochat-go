package store

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestBeginAndCompleteWeWorkCallbackSideEffectAreFencedByActiveInboxLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	execution := dashboard.WeWorkCallbackExecution{TenantID: 21, CorpID: 7, EventKey: strings.Repeat("a", 64), LeaseToken: strings.Repeat("1", 64), LeaseFence: 9}
	payloadHash := strings.Repeat("b", 64)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT 1 FROM mochat_go_wework_callback_inbox").WithArgs(21, 7, execution.EventKey, execution.LeaseToken, uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery("SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects").WithArgs(21, 7, execution.EventKey, "fission.customer_push").WillReturnRows(sqlmock.NewRows([]string{"status", "payload_hash"}).AddRow("pending", payloadHash))
	mock.ExpectExec("UPDATE mochat_go_wework_callback_side_effects").WithArgs(21, 7, execution.EventKey, "fission.customer_push", payloadHash).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	execute, status, err := store.BeginWeWorkCallbackSideEffect(context.Background(), execution, "fission.customer_push", payloadHash)
	if err != nil || !execute || status != "unknown" {
		t.Fatalf("begin execute=%t status=%q err=%v", execute, status, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT 1 FROM mochat_go_wework_callback_inbox").WithArgs(21, 7, execution.EventKey, execution.LeaseToken, uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery("SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects").WithArgs(21, 7, execution.EventKey, "fission.customer_push").WillReturnRows(sqlmock.NewRows([]string{"status", "payload_hash"}).AddRow("unknown", payloadHash))
	mock.ExpectExec("UPDATE mochat_go_wework_callback_side_effects").WithArgs(21, 7, execution.EventKey, "fission.customer_push", payloadHash).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.CompleteWeWorkCallbackSideEffect(context.Background(), execution, "fission.customer_push", payloadHash); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBeginWeWorkCallbackSideEffectRejectsStaleWorkerLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	execution := dashboard.WeWorkCallbackExecution{TenantID: 21, CorpID: 7, EventKey: strings.Repeat("c", 64), LeaseToken: strings.Repeat("2", 64), LeaseFence: 3}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT 1 FROM mochat_go_wework_callback_inbox").WithArgs(21, 7, execution.EventKey, execution.LeaseToken, uint64(3)).WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectRollback()
	if _, _, err := store.BeginWeWorkCallbackSideEffect(context.Background(), execution, "fission.customer_push", strings.Repeat("d", 64)); err != dashboard.ErrWeWorkCallbackLeaseLost {
		t.Fatalf("stale lease error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
