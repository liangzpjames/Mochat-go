package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

func TestRecordCapabilityDispatchResultRollsBackWhenEventAppendFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{TenantID: 7, CorpID: 9, AuthVersion: 1, CorpStatus: dashboardprincipal.CorpBindingStatusActive}

	mock.ExpectBegin()
	expectCapabilityLedgerBinding(mock, 7, 9)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,"+"\n\t\t       provider_request_id,provider_message_id,provider_object_id,credential_generation,lease_token,"+"\n\t\t       lease_expires_at,attempt,next_poll_at,last_error_code,created_at,updated_at")).WithArgs(7, 9, 31).WillReturnRows(capabilityDispatchRows("submitted", "lease-1", 1, 4, 44))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches")).WithArgs(7, 9, 31, "lease-1", 1).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT operation_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE")).WithArgs(7, 9, 31).WillReturnRows(sqlmock.NewRows([]string{"operation_id"}).AddRow(44))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,"+"\n\t\t       provider_request_id,provider_object_id,actual_agent_id,external_success,callback_evidence,"+"\n\t\t       target_total,success_total,failure_total,error_code,actor_user_id,actor_source,request_id,lease_token,"+"\n\t\t       lease_expires_at,attempt,requested_at,started_at,finished_at,created_at,updated_at")).WithArgs(7, 9, 44).WillReturnRows(capabilityOperationRows(44, "pending", 4))
	expectCapabilityLedgerBinding(mock, 7, 9)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe,created_at,updated_at")).WithArgs(7, 9, 44, "external_user", "user-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_wecom_capability_operation_results")).WithArgs(7, 9, 44, "external_user", "user-1", wecomcapability.DispatchSucceeded, "provider-user-1", "", "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_wecom_capability_operation_audits")).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_wecom_capability_operation_events")).WillReturnError(errors.New("event append failed"))
	mock.ExpectRollback()

	_, err = store.RecordCapabilityDispatchResult(context.Background(), principal, CapabilityDispatchResultInput{
		DispatchID: 31, LeaseToken: "lease-1", Attempt: 1, TargetKind: "external_user", TargetID: "user-1",
		Status: wecomcapability.DispatchSucceeded, ProviderTargetID: "provider-user-1",
	})
	if err == nil || !regexp.MustCompile("event append failed").MatchString(err.Error()) {
		t.Fatalf("event failure was not returned with transaction rollback: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimCapabilityDispatchAtGenerationRejectsBeforeLeaseUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{TenantID: 7, CorpID: 9, AuthVersion: 1, CorpStatus: dashboardprincipal.CorpBindingStatusActive}

	mock.ExpectBegin()
	expectCapabilityLedgerBinding(mock, 7, 9)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT operation_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE")).WithArgs(7, 9, 31).WillReturnRows(sqlmock.NewRows([]string{"operation_id"}).AddRow(44))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,"+"\n\t\t       provider_request_id,provider_object_id,actual_agent_id,external_success,callback_evidence,"+"\n\t\t       target_total,success_total,failure_total,error_code,actor_user_id,actor_source,request_id,lease_token,"+"\n\t\t       lease_expires_at,attempt,requested_at,started_at,finished_at,created_at,updated_at")).WithArgs(7, 9, 44).WillReturnRows(capabilityOperationRows(44, "pending", 4))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,"+"\n\t\t       provider_request_id,provider_message_id,provider_object_id,credential_generation,lease_token,"+"\n\t\t       lease_expires_at,attempt,next_poll_at,last_error_code,created_at,updated_at")).WithArgs(7, 9, 31).WillReturnRows(capabilityDispatchRows("queued", "", 0, 4, 44))
	expectCapabilityLedgerBinding(mock, 7, 9)
	mock.ExpectRollback()

	_, err = store.ClaimCapabilityDispatchAtGeneration(context.Background(), principal, 31, timeMinuteForTest(), 3)
	if !errors.Is(err, ErrCapabilityOperationStale) {
		t.Fatalf("generation mismatch err=%v, want stale", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSafeCapabilityDiagnosticTextAllowsHumanTextButRejectsControl(t *testing.T) {
	if !validSafeCapabilityText("企微返回：任务暂未完成", 255) {
		t.Fatal("human-safe diagnostic text was rejected")
	}
	if validSafeCapabilityText("provider\nraw", 255) {
		t.Fatal("line-broken provider text was accepted")
	}
	if validSafeCapabilityText("provider\x00raw", 255) {
		t.Fatal("NUL-containing diagnostic text was accepted")
	}
	if validCapabilityMachineCode("企微返回：任务暂未完成", 255) {
		t.Fatal("human diagnostic text was incorrectly treated as a machine code")
	}
}

func TestRecordCapabilityDispatchResultRejectsQueuedTargetResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	_, err = store.RecordCapabilityDispatchResult(context.Background(), dashboardprincipal.DashboardPrincipal{
		TenantID: 7, CorpID: 9, AuthVersion: 1, CorpStatus: dashboardprincipal.CorpBindingStatusActive,
	}, CapabilityDispatchResultInput{
		DispatchID: 31, LeaseToken: "lease-1", Attempt: 1, TargetKind: "external_user", TargetID: "user-1",
		Status: wecomcapability.DispatchQueued,
	})
	if !errors.Is(err, companyprofile.ErrInvalidRequest) {
		t.Fatalf("queued target result was not rejected as invalid request: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("queued result touched the database: %v", err)
	}
}

func TestRecordCapabilityDispatchResultExactDuplicateIsNoOp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewMySQLStore(db)
	principal := dashboardprincipal.DashboardPrincipal{TenantID: 7, CorpID: 9, AuthVersion: 1, CorpStatus: dashboardprincipal.CorpBindingStatusActive}

	mock.ExpectBegin()
	expectCapabilityLedgerBinding(mock, 7, 9)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,operation_id,dispatch_kind,chunk_no,target_id,idempotency_key,status,"+"\n\t\t       provider_request_id,provider_message_id,provider_object_id,credential_generation,lease_token,"+"\n\t\t       lease_expires_at,attempt,next_poll_at,last_error_code,created_at,updated_at")).WithArgs(7, 9, 31).WillReturnRows(capabilityDispatchRows("submitted", "lease-1", 1, 4, 44))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM mochat_go_wecom_capability_dispatches")).WithArgs(7, 9, 31, "lease-1", 1).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT operation_id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND id=? FOR UPDATE")).WithArgs(7, 9, 31).WillReturnRows(sqlmock.NewRows([]string{"operation_id"}).AddRow(44))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,capability,action,credential_group,credential_generation,idempotency_key,status,"+"\n\t\t       provider_request_id,provider_object_id,actual_agent_id,external_success,callback_evidence,"+"\n\t\t       target_total,success_total,failure_total,error_code,actor_user_id,actor_source,request_id,lease_token,"+"\n\t\t       lease_expires_at,attempt,requested_at,started_at,finished_at,created_at,updated_at")).WithArgs(7, 9, 44).WillReturnRows(capabilityOperationRows(44, "pending", 4))
	expectCapabilityLedgerBinding(mock, 7, 9)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,tenant_id,corp_id,operation_id,target_kind,target_id,status,provider_target_id,error_code,error_message_safe,created_at,updated_at")).WithArgs(7, 9, 44, "external_user", "user-1").WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "operation_id", "target_kind", "target_id", "status", "provider_target_id", "error_code", "error_message_safe", "created_at", "updated_at"}).AddRow(71, 7, 9, 44, "external_user", "user-1", wecomcapability.DispatchFailed, "provider-user-1", "wecom.http_401", "安全错误", nil, nil))
	mock.ExpectCommit()

	result, err := store.RecordCapabilityDispatchResult(context.Background(), principal, CapabilityDispatchResultInput{
		DispatchID: 31, LeaseToken: "lease-1", Attempt: 1, TargetKind: "external_user", TargetID: "user-1",
		Status: wecomcapability.DispatchFailed, ProviderTargetID: "provider-user-1", ErrorCode: "wecom.http_401", ErrorMessageSafe: "安全错误",
	})
	if err != nil || result.ID != 71 || result.Status != wecomcapability.DispatchFailed {
		t.Fatalf("exact duplicate was not returned as a no-op: result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func timeMinuteForTest() time.Duration { return time.Minute }

func expectCapabilityLedgerBinding(mock sqlmock.Sqlmock, tenantID, corpID int) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT b.tenant_id, b.corp_id, b.status, b.version,"+"\n\t\t       b.employee_credential_generation, b.contact_credential_generation,"+"\n\t\t       b.agent_credential_generation, b.callback_credential_generation,"+"\n\t\t       COALESCE(b.verified_wx_corpid,''), COALESCE(b.verified_corp_name,''), b.verified_at,"+"\n\t\t       COALESCE(c.name,''), COALESCE(c.wx_corpid,''),"+"\n\t\t       COALESCE(c.wecom_credentials_ciphertext,''), COALESCE(c.wecom_credentials_key_id,''), c.updated_at")).WithArgs(tenantID, corpID).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "corp_id", "status", "version", "employee_generation", "contact_generation", "agent_generation", "callback_generation", "verified_wx_corpid", "verified_corp_name", "verified_at", "name", "wx_corpid", "ciphertext", "key_id", "updated_at"}).AddRow(tenantID, corpID, 2, 5, 1, 4, 1, 1, "wx-corp", "Corp", nil, "Corp", "wx-corp", "cipher", "key", nil))
}

func capabilityDispatchRows(status, lease string, attempt int, generation uint64, operationID int64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "operation_id", "dispatch_kind", "chunk_no", "target_id", "idempotency_key", "status", "provider_request_id", "provider_message_id", "provider_object_id", "credential_generation", "lease_token", "lease_expires_at", "attempt", "next_poll_at", "last_error_code", "created_at", "updated_at"}).
		AddRow(31, 7, 9, operationID, "contact", 0, "external-1", "dispatch-1", status, "", "", "", generation, lease, nil, attempt, nil, "", nil, nil)
}

func capabilityOperationRows(id int64, status string, generation uint64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "tenant_id", "corp_id", "capability", "action", "credential_group", "credential_generation", "idempotency_key", "status", "provider_request_id", "provider_object_id", "actual_agent_id", "external_success", "callback_evidence", "target_total", "success_total", "failure_total", "error_code", "actor_user_id", "actor_source", "request_id", "lease_token", "lease_expires_at", "attempt", "requested_at", "started_at", "finished_at", "created_at", "updated_at"}).
		AddRow(id, 7, 9, wecomcapability.ContactBatchSend, wecomcapability.ActionSend, string(wecomcapability.CredentialGroupContact), generation, "operation-1", status, "", "", "", false, false, 1, 0, 0, "", nil, "system", "", "", nil, 0, nil, nil, nil, nil, nil)
}
