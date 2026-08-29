package store

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBeginAndCompleteWeWorkCallbackSideEffectTransitionsPendingUnknownSent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	eventKey := strings.Repeat("a", 64)
	payloadHash := strings.Repeat("b", 64)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_wework_callback_side_effects SET status='unknown',updated_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND payload_hash=? AND status='pending'")).
		WithArgs(21, 7, eventKey, "fission.customer_push", payloadHash).
		WillReturnResult(sqlmock.NewResult(0, 1))
	execute, status, err := store.BeginWeWorkCallbackSideEffect(context.Background(), 21, 7, eventKey, "fission.customer_push", payloadHash)
	if err != nil || !execute || status != "unknown" {
		t.Fatalf("begin execute=%t status=%q err=%v", execute, status, err)
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_wework_callback_side_effects SET status='sent',sent_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND payload_hash=? AND status='unknown'")).
		WithArgs(21, 7, eventKey, "fission.customer_push", payloadHash).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.CompleteWeWorkCallbackSideEffect(context.Background(), 21, 7, eventKey, "fission.customer_push", payloadHash); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBeginWeWorkCallbackSideEffectReplaysSentWithoutExecuting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	eventKey := strings.Repeat("c", 64)
	payloadHash := strings.Repeat("d", 64)

	mock.ExpectExec("UPDATE mochat_go_wework_callback_side_effects").
		WithArgs(21, 7, eventKey, "fission.customer_push", payloadHash).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects").
		WithArgs(21, 7, eventKey, "fission.customer_push").
		WillReturnRows(sqlmock.NewRows([]string{"status", "payload_hash"}).AddRow("sent", payloadHash))
	execute, status, err := store.BeginWeWorkCallbackSideEffect(context.Background(), 21, 7, eventKey, "fission.customer_push", payloadHash)
	if err != nil || execute || status != "sent" {
		t.Fatalf("replay execute=%t status=%q err=%v", execute, status, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
