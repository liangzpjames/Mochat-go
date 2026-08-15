package store

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// P0-4: reminder crash-window fencing evidence that does not require a real
// database. The real integration harness covers the end-to-end flows; these
// tests lock the store-level fences: stable idempotency keys, stale lease
// rejection without writes, invalid totals rejection, terminal-status
// mismatches, and lease-expiry fencing on the final record update.

func reminderOperationRows(id int64, status, lease string, attempt int, targetTotal, successTotal, failureTotal int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "capability", "action", "credential_group", "credential_generation", "idempotency_key", "status", "provider_request_id", "provider_object_id", "actual_agent_id", "external_success", "callback_evidence", "target_total", "success_total", "failure_total", "error_code", "actor_user_id", "actor_source", "request_id", "lease_token", "lease_expires_at", "attempt", "requested_at", "started_at", "finished_at", "created_at", "updated_at"}).
		AddRow(id, 7, 9, wecomcapability.AgentMessage, wecomcapability.ActionSend, string(wecomcapability.CredentialGroupAgent), 1, "contact-remind:batch:1:employee:101:created:2026-08-15 00:00:00", status, "", "", "100001", false, false, targetTotal, successTotal, failureTotal, "", 11001, "user", "contact-batch:1", lease, nil, attempt, nil, nil, nil, nil, nil)
}

func reminderOperationSelectPattern() string {
	return regexp.QuoteMeta("SELECT id,tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status," +
		"\n\t\t       provider_request_id,provider_object_id,actual_agent_id,external_success,callback_evidence," +
		"\n\t\t       target_total,success_total,failure_total,error_code,actor_user_id,actor_source,request_id,lease_token," +
		"\n\t\t       lease_expires_at,attempt,requested_at,started_at,finished_at,created_at,updated_at")
}

func TestContactBatchReminderIdempotencyKeyIsStableAndRequestScoped(t *testing.T) {
	requestScoped := contactBatchReminderIdempotencyKey(1, 101, "req-abc", "2026-08-15 00:00:00")
	if requestScoped != "contact-remind:request:req-abc" {
		t.Fatalf("request-scoped key=%q", requestScoped)
	}
	createdScoped := contactBatchReminderIdempotencyKey(1, 101, "", "2026-08-15 00:00:00")
	if createdScoped != "contact-remind:batch:1:employee:101:created:2026-08-15 00:00:00" {
		t.Fatalf("created-scoped key=%q", createdScoped)
	}
	if contactBatchReminderIdempotencyKey(1, 101, "req-abc", "2026-08-15 00:00:00") != requestScoped {
		t.Fatal("request-scoped key must be stable")
	}
	if contactBatchReminderIdempotencyKey(2, 101, "", "2026-08-15 00:00:00") == createdScoped {
		t.Fatal("created-scoped key must differ across batches")
	}
}

// TestContactBatchReminderRecordRejectsStaleLeaseWithoutWriting locks that a
// reminder record with a lease that does not match the stored attempt fails
// closed as stale with zero writes (no UPDATE is ever issued).
func TestContactBatchReminderRecordRejectsStaleLeaseWithoutWriting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 11001, TenantID: 7, CorpID: 9, AuthVersion: 1}
	mock.ExpectBegin()
	mock.ExpectQuery(reminderOperationSelectPattern()).
		WithArgs(7, 9, 41).
		WillReturnRows(reminderOperationRows(41, wecomcapability.OperationSubmitting, "stored-lease", 1, 2, 0, 0))
	mock.ExpectRollback()
	err = store.RecordContactBatchDurableReminder(context.Background(), principal, dashboard.ContactBatchDurableReminder{
		OperationID: 41, LeaseToken: "other-lease", Attempt: 1,
	}, "", true, 2, 0)
	if err == nil || !errors.Is(err, ErrCapabilityOperationStale) {
		t.Fatalf("stale lease err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestContactBatchReminderRecordRejectsInvalidTotals locks that a success
// record carrying failures (or a failure record with zero failures) is an
// invalid request, not a persisted terminal state.
func TestContactBatchReminderRecordRejectsInvalidTotals(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 11001, TenantID: 7, CorpID: 9, AuthVersion: 1}
	mock.ExpectBegin()
	mock.ExpectQuery(reminderOperationSelectPattern()).
		WithArgs(7, 9, 41).
		WillReturnRows(reminderOperationRows(41, wecomcapability.OperationSubmitting, "lease-1", 1, 2, 0, 0))
	mock.ExpectRollback()
	err = store.RecordContactBatchDurableReminder(context.Background(), principal, dashboard.ContactBatchDurableReminder{
		OperationID: 41, LeaseToken: "lease-1", Attempt: 1,
	}, "", true, 1, 1)
	if err == nil || err != companyprofile.ErrInvalidRequest {
		t.Fatalf("invalid totals err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestContactBatchReminderRecordRejectsTerminalStatusMismatch locks that a
// terminal operation with totals different from the record is stale, never a
// silent overwrite.
func TestContactBatchReminderRecordRejectsTerminalStatusMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 11001, TenantID: 7, CorpID: 9, AuthVersion: 1}
	mock.ExpectBegin()
	mock.ExpectQuery(reminderOperationSelectPattern()).
		WithArgs(7, 9, 41).
		WillReturnRows(reminderOperationRows(41, wecomcapability.OperationFailed, "lease-1", 1, 2, 1, 1))
	mock.ExpectRollback()
	err = store.RecordContactBatchDurableReminder(context.Background(), principal, dashboard.ContactBatchDurableReminder{
		OperationID: 41, LeaseToken: "lease-1", Attempt: 1,
	}, "", true, 2, 0)
	if err == nil || !errors.Is(err, ErrCapabilityOperationStale) {
		t.Fatalf("terminal mismatch err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestContactBatchReminderRecordFencesByLeaseExpiry locks that the final
// record UPDATE is fenced by an unexpired lease: when the stored lease has
// expired, zero rows are affected and the record is stale.
func TestContactBatchReminderRecordFencesByLeaseExpiry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	principal := dashboardprincipal.DashboardPrincipal{UserID: 11001, TenantID: 7, CorpID: 9, AuthVersion: 1}
	mock.ExpectBegin()
	mock.ExpectQuery(reminderOperationSelectPattern()).
		WithArgs(7, 9, 41).
		WillReturnRows(reminderOperationRows(41, wecomcapability.OperationSubmitting, "lease-1", 1, 2, 0, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mochat_go_wecom_capability_operations")).
		WithArgs(wecomcapability.OperationSucceeded, true, 2, 0, "", 7, 9, 41, wecomcapability.OperationSubmitting, "lease-1", 1).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	err = store.RecordContactBatchDurableReminder(context.Background(), principal, dashboard.ContactBatchDurableReminder{
		OperationID: 41, LeaseToken: "lease-1", Attempt: 1,
	}, "", true, 2, 0)
	if err == nil || !errors.Is(err, ErrCapabilityOperationStale) {
		t.Fatalf("expired lease fence err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}